// Track processing: WebSocket message → GeoJSON features for the map.
// All state (trackHistory, trackFlHistory, trackCoasting, fadingTracks,
// liveTrackFeatures, liveVectorFeatures) is carried in a single `state`
// object passed by the caller (engine.js), keeping this module pure.
import { HISTORY_HARD_CAP, DEFAULT_HISTORY_DURATION_S, FADE_DURATION_MS, VECTOR_LOOKAHEAD_S, EARTH_RADIUS_M } from './constants.js'
import { buildLabel } from './label.js'
import { trackProvenance } from './provenance.js'

// vectorEndpoint computes the geographic point reached after
// VECTOR_LOOKAHEAD_S seconds of travel at constant velocity (vx/vy in m/s,
// East/North), starting from (lat, lon). Uses a local flat-Earth
// approximation, which is sufficient for the short look-ahead distances
// involved.
function vectorEndpoint(lat, lon, vx, vy) {
  const dEast = vx * VECTOR_LOOKAHEAD_S
  const dNorth = vy * VECTOR_LOOKAHEAD_S

  const dLat = (dNorth / EARTH_RADIUS_M) * (180 / Math.PI)
  const dLon =
    (dEast / (EARTH_RADIUS_M * Math.cos((lat * Math.PI) / 180))) *
    (180 / Math.PI)

  return [lon + dLon, lat + dLat]
}

// isFlFiltered returns true when a known flight level falls outside the active
// FL filter window (ASD-005). Tracks with unknown FL always pass through the
// filter — hiding unknown-altitude traffic would be operationally unsafe.
export function isFlFiltered(flightLevelFt, flFilter) {
  if (typeof flightLevelFt !== 'number') return false
  const fl = Math.round(flightLevelFt / 100)
  const { minFL, maxFL } = flFilter
  if (minFL !== null && fl < minFL) return true
  if (maxFL !== null && fl > maxFL) return true
  return false
}

// flOpacity returns the fl_opacity value to attach to a filtered feature, or
// undefined when the feature passes the filter. hide=true → 0 (invisible);
// hide=false → 0.15 (entsättigt / heavily dimmed).
export function flOpacity(flightLevelFt, flFilter) {
  if (!isFlFiltered(flightLevelFt, flFilter)) return undefined
  return flFilter.hide ? 0.0 : 0.15
}

// updateTrackHistory appends each track's current position to its trail history
// and drops history for tracks that are no longer present — but keeps history
// alive for tracks currently fading out (ASD-004c), so their trail and dots
// remain visible during the fade.
//
// #191: each point is stored as { c: [lon, lat], t } where t is the message
// arrival time (ms). Retention is by DURATION (retentionMs) rather than a fixed
// point count, so "last N minutes" is well-defined regardless of the per-sensor
// scan period. HISTORY_HARD_CAP still bounds memory for pathological rates.
export function updateTrackHistory(tracks, state, nowMs = Date.now(), retentionMs = DEFAULT_HISTORY_DURATION_S * 1000) {
  const seen = new Set()

  tracks.forEach((track) => {
    seen.add(track.track_num)
    let hist = state.trackHistory.get(track.track_num)
    if (!hist) {
      hist = []
      state.trackHistory.set(track.track_num, hist)
    }
    hist.push({ c: [track.longitude, track.latitude], t: nowMs })
    // Drop points older than the retention window (measured from this update).
    const cutoff = nowMs - retentionMs
    while (hist.length > 0 && hist[0].t < cutoff) hist.shift()
    // Absolute safety cap on point count (memory bound), independent of duration.
    if (hist.length > HISTORY_HARD_CAP) hist.splice(0, hist.length - HISTORY_HARD_CAP)
  })

  for (const trackNum of state.trackHistory.keys()) {
    if (!seen.has(trackNum) && !state.fadingTracks.has(trackNum)) {
      state.trackHistory.delete(trackNum)
    }
  }
}

// updateTracksLayer processes a WebSocket message (see pkg/broadcast.Message):
// it routes TSE tracks into the fade-out map (ASD-004c), computes per-track
// vertical tendency and labels (ASD-001), builds live GeoJSON features, and
// kicks off the fade-animation loop when needed.
export function updateTracksLayer(msg, state, renderSources, startFadeLoop, retentionMs = DEFAULT_HISTORY_DURATION_S * 1000) {
  // TSE (Track-Service-End) tracks: register them for a graceful fade-out
  // (ASD-004c) instead of removing them instantly. Only the first TSE for a
  // given track_num sets the deadline; duplicates are ignored.
  ;(msg.tracks || [])
    .filter((t) => t.ended)
    .forEach((t) => {
      if (!state.fadingTracks.has(t.track_num)) {
        state.fadingTracks.set(t.track_num, {
          deadline: Date.now() + FADE_DURATION_MS,
          track: t,
        })
      }
    })

  const tracks = (msg.tracks || []).filter((t) => !t.ended)

  // A track_num reappearing in the live stream (resurrection) must be evicted
  // from the fading map so it does not render with a stale fade_opacity.
  tracks.forEach((t) => state.fadingTracks.delete(t.track_num))

  // Stamp history points with the message arrival time (time_ms; wall-clock,
  // monotonic). Falls back to Date.now() for messages without it (e.g. tests).
  const nowMs = typeof msg.time_ms === 'number' ? msg.time_ms : Date.now()
  updateTrackHistory(tracks, state, nowMs, retentionMs)

  // Build the set of track_nums that need ongoing state (live + fading).
  const liveNums = new Set(tracks.map((t) => t.track_num))
  for (const num of state.trackFlHistory.keys()) {
    if (!liveNums.has(num) && !state.fadingTracks.has(num)) {
      state.trackFlHistory.delete(num)
    }
  }
  for (const num of state.trackCoasting.keys()) {
    if (!liveNums.has(num) && !state.fadingTracks.has(num)) {
      state.trackCoasting.delete(num)
    }
  }

  // Precompute live track GeoJSON features. Vertical-tendency (ASD-001b) is
  // computed here — comparing current FL to the previously stored value — and
  // the result is baked into the label string so renderSources() can reuse it
  // without recalculating on every fade-loop tick.
  state.liveTrackFeatures = tracks.map((track) => {
    let vTrend = ''
    if (typeof track.flight_level_ft === 'number') {
      const prevFl = state.trackFlHistory.get(track.track_num)
      if (typeof prevFl === 'number') {
        const delta = track.flight_level_ft - prevFl
        if (delta > 50) vTrend = '▲'
        else if (delta < -50) vTrend = '▼'
      }
      state.trackFlHistory.set(track.track_num, track.flight_level_ft)
    }
    state.trackCoasting.set(track.track_num, track.coasting)
    return {
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [track.longitude, track.latitude] },
      properties: {
        track_num: track.track_num,
        confirmed: track.confirmed,
        coasting: track.coasting,
        vx: track.vx,
        vy: track.vy,
        label: buildLabel(track, vTrend),
        // WF2-40: surveillance source, drives the track symbol shape and the
        // detail panel. Derived from the contract fields (see provenance.js).
        provenance: trackProvenance(track),
        // Stored so renderSources() can re-evaluate the FL filter on UI change
        // (ASD-005) without waiting for a new WebSocket update.
        flight_level_ft: typeof track.flight_level_ft === 'number' ? track.flight_level_ft : null,
        // Bug #55: bake transponder identity into feature properties so the
        // TrackDetailCard can display them without re-parsing the raw WS frame.
        mode_3a: track.mode_3a != null ? track.mode_3a : null,
        callsign: track.callsign != null ? track.callsign : null,
        // ASD-011: extended detail fields for the TrackDetailCard. Baked here so
        // the panel reads them straight off store.selectedTrack (the selected
        // feature's properties) without holding on to the raw WS message.
        latitude: track.latitude,
        longitude: track.longitude,
        icao_addr: track.icao_addr != null ? track.icao_addr : null,
        accuracy: typeof track.accuracy === 'number' ? track.accuracy : null,
        sac: track.sac != null ? track.sac : null,
        sic: track.sic != null ? track.sic : null,
        // Per-technology update ages (I062/290, ICD 2.6.0) drive the
        // "Sensor-Aktualität" section (see trackDetail.js / provenance.js).
        adsb_age_s: track.adsb_age_s != null ? track.adsb_age_s : null,
        flarm_age_s: track.flarm_age_s != null ? track.flarm_age_s : null,
        ssr_age_s: track.ssr_age_s != null ? track.ssr_age_s : null,
        mds_age_s: track.mds_age_s != null ? track.mds_age_s : null,
        // Vertical tendency (ASD-001b), already computed above for the label —
        // exposed as a property so the panel can word it (Steigend/Sinkend).
        vertical_trend: vTrend,
      },
    }
  })

  state.liveVectorFeatures = tracks.map((track) => ({
    type: 'Feature',
    geometry: {
      type: 'LineString',
      coordinates: [
        [track.longitude, track.latitude],
        vectorEndpoint(track.latitude, track.longitude, track.vx, track.vy),
      ],
    },
    properties: {
      track_num: track.track_num,
      coasting: track.coasting,
    },
  }))

  renderSources()

  // Start the fade-animation loop if there are fading tracks and it is not
  // already running.
  if (state.fadingTracks.size > 0) {
    startFadeLoop()
  }
}
