// renderSources and tickFade — push the current air picture into all GeoJSON
// map sources and drive the TSE fade-out animation loop (ASD-004c).
import {
  TRACKS_SOURCE_ID,
  VECTORS_SOURCE_ID,
  HISTORY_DOTS_SOURCE_ID,
  TRAILS_SOURCE_ID,
  LABELS_SOURCE_ID,
  LEADER_LINES_SOURCE_ID,
  SELECTION_SOURCE_ID,
  SELECTION_LABEL_SOURCE_ID,
  FADE_DURATION_MS,
  VECTOR_LOOKAHEAD_S,
  EARTH_RADIUS_M,
  DEFAULT_HISTORY_DURATION_S,
} from './constants.js'
import { buildLabel } from './label.js'
import { isFlFiltered, flOpacity } from './tracks.js'
import { trackProvenance } from './provenance.js'
import { deconflictLabels } from './deconflict.js'

// vectorEndpoint: local flat-Earth approximation for speed-vector tip.
function vectorEndpoint(lat, lon, vx, vy) {
  const dEast = vx * VECTOR_LOOKAHEAD_S
  const dNorth = vy * VECTOR_LOOKAHEAD_S
  const dLat = (dNorth / EARTH_RADIUS_M) * (180 / Math.PI)
  const dLon =
    (dEast / (EARTH_RADIUS_M * Math.cos((lat * Math.PI) / 180))) *
    (180 / Math.PI)
  return [lon + dLon, lat + dLat]
}

// renderSources pushes the current air picture into all GeoJSON map sources.
// It merges live features with any currently fading tracks, attaching a
// fade_opacity property (0–1) that the paint expressions use for opacity.
// Called on every WebSocket update and on every fade-loop tick.
// Parameters:
//   map              — MapLibre Map instance
//   state            — engine runtime state (liveTrackFeatures, fadingTracks, …)
//   flFilter         — { minFL, maxFL, hide } from Pinia store
//   labelPins        — Map<track_num, {dx, dy}> manual label overrides
//   palette          — active foreground colour palette
export function renderSources(map, state, flFilter, labelPins, palette, selectedTrackNum, historyRetentionMs = DEFAULT_HISTORY_DURATION_S * 1000) {
  const now = Date.now()

  // Live-track features: re-evaluate FL filter (ASD-005) each render call so
  // a slider change takes effect immediately, not only on the next WSS update.
  const liveTrackFeatures = state.liveTrackFeatures.flatMap((f) => {
    const flFt = f.properties.flight_level_ft
    const filtered = isFlFiltered(flFt, flFilter)
    const flOp = flOpacity(flFt, flFilter)
    const props = { ...f.properties, filtered }
    if (flOp !== undefined) props.fl_opacity = flOp
    return { ...f, properties: props }
  })

  // Fading-track features: same shape as live features but carry fade_opacity.
  // FL filter is also applied so that a filtering track fades out invisibly.
  const fadingTrackFeatures = []
  const fadingVectorFeatures = []
  for (const [, { deadline, track }] of state.fadingTracks) {
    const fadeOpacity = Math.max(0, (deadline - now) / FADE_DURATION_MS)
    const flOp = flOpacity(track.flight_level_ft, flFilter)
    const trackProps = {
      track_num: track.track_num,
      confirmed: track.confirmed,
      coasting: track.coasting,
      vx: track.vx,
      vy: track.vy,
      label: buildLabel(track, ''),
      provenance: trackProvenance(track), // WF2-40: keep symbol shape during fade
      filtered: isFlFiltered(track.flight_level_ft, flFilter),
      fade_opacity: fadeOpacity,
      // Bug #55: carry identity fields through the fade so the detail panel
      // remains accurate if the user selected this track before TSE.
      mode_3a: track.mode_3a != null ? track.mode_3a : null,
      callsign: track.callsign != null ? track.callsign : null,
    }
    if (flOp !== undefined) trackProps.fl_opacity = flOp
    fadingTrackFeatures.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [track.longitude, track.latitude] },
      properties: trackProps,
    })
    const vecProps = {
      track_num: track.track_num,
      coasting: track.coasting,
      fade_opacity: fadeOpacity,
    }
    if (flOp !== undefined) vecProps.fl_opacity = flOp
    fadingVectorFeatures.push({
      type: 'Feature',
      geometry: {
        type: 'LineString',
        coordinates: [
          [track.longitude, track.latitude],
          vectorEndpoint(track.latitude, track.longitude, track.vx, track.vy),
        ],
      },
      properties: vecProps,
    })
  }

  // Live vector features also need FL filter re-evaluation.
  const liveVectorFeatures = state.liveVectorFeatures.flatMap((f) => {
    const flFt = state.trackFlHistory.get(f.properties.track_num)
    const flOp = flOpacity(flFt, flFilter)
    if (flOp === undefined) return [f]
    return [{ ...f, properties: { ...f.properties, fl_opacity: flOp } }]
  })

  map.getSource(TRACKS_SOURCE_ID).setData({
    type: 'FeatureCollection',
    features: [...liveTrackFeatures, ...fadingTrackFeatures],
  })

  // ASD-007: selection halo — pin a single ring to the selected track's current
  // position so it follows the moving symbol. Cleared (empty collection) when no
  // track is selected or the selected track is no longer on the scope.
  const selSrc = map.getSource(SELECTION_SOURCE_ID)
  if (selSrc) {
    const selFeature =
      selectedTrackNum != null &&
      [...liveTrackFeatures, ...fadingTrackFeatures].find(
        (f) => f.properties.track_num === selectedTrackNum,
      )
    selSrc.setData({
      type: 'FeatureCollection',
      features: selFeature
        ? [{ type: 'Feature', geometry: selFeature.geometry, properties: {} }]
        : [],
    })
  }

  map.getSource(VECTORS_SOURCE_ID).setData({
    type: 'FeatureCollection',
    features: [...liveVectorFeatures, ...fadingVectorFeatures],
  })

  // History dots (ASD-004a): one Point per entry in trackHistory. The coasting
  // flag comes from trackCoasting (updated in updateTracksLayer). Fading tracks
  // keep their history alive and carry fade_opacity so dots fade with the track.
  // ASD-005: fl_opacity is derived from the last known FL for this track.
  const dotsFeatures = []
  for (const [trackNum, hist] of state.trackHistory) {
    const isCoasting = state.trackCoasting.get(trackNum) || false
    const fadingEntry = state.fadingTracks.get(trackNum)
    const fadeOpacity = fadingEntry
      ? Math.max(0, (fadingEntry.deadline - now) / FADE_DURATION_MS)
      : undefined
    const flFt = state.trackFlHistory.get(trackNum)
    const flOp = flOpacity(flFt, flFilter)
    // #191: dot age is measured against the NEWEST point of this track (not the
    // wall clock), so the trail fades from bright (newest) to faint (oldest)
    // even when a coasting track has stopped updating. 0 = newest … 1 = oldest.
    const newestT = hist.length ? hist[hist.length - 1].t : now
    for (const { c, t } of hist) {
      const age = historyRetentionMs > 0
        ? Math.min(1, Math.max(0, (newestT - t) / historyRetentionMs))
        : 0
      const props = { track_num: trackNum, coasting: isCoasting, age }
      if (fadeOpacity !== undefined) props.fade_opacity = fadeOpacity
      if (flOp !== undefined) props.fl_opacity = flOp
      dotsFeatures.push({
        type: 'Feature',
        geometry: { type: 'Point', coordinates: c },
        properties: props,
      })
    }
  }

  map.getSource(HISTORY_DOTS_SOURCE_ID).setData({
    type: 'FeatureCollection',
    features: dotsFeatures,
  })

  // Trails: one LineString per track, with coasting, fade_opacity and fl_opacity
  // for consistent dimming/fading/filtering across all layers (ASD-004/ASD-005).
  const trailFeatures = []
  for (const [trackNum, hist] of state.trackHistory) {
    if (hist.length >= 2) {
      const isCoasting = state.trackCoasting.get(trackNum) || false
      const fadingEntry = state.fadingTracks.get(trackNum)
      const flFt = state.trackFlHistory.get(trackNum)
      const flOp = flOpacity(flFt, flFilter)
      const props = { track_num: trackNum, coasting: isCoasting }
      if (fadingEntry) {
        props.fade_opacity = Math.max(0, (fadingEntry.deadline - now) / FADE_DURATION_MS)
      }
      if (flOp !== undefined) props.fl_opacity = flOp
      trailFeatures.push({
        type: 'Feature',
        // #191: history points are now { c, t }; the line uses the coordinates.
        geometry: { type: 'LineString', coordinates: hist.map((h) => h.c) },
        properties: props,
      })
    }
  }

  map.getSource(TRAILS_SOURCE_ID).setData({
    type: 'FeatureCollection',
    features: trailFeatures,
  })

  // ASD-002: deconflict label positions in screen space and push to the
  // dedicated label + leader-line sources. Labels never disappear — the
  // greedy algorithm always places every label in the least-colliding slot.
  // Wrapped in try/catch: a deconfliction failure must never abort the rest
  // of renderSources() (circles / vectors would disappear if it propagated).
  try {
    const { labelFeatures, leaderLineFeatures, selectionBoxFeatures } = deconflictLabels(
      [...liveTrackFeatures, ...fadingTrackFeatures],
      map,
      labelPins,
      selectedTrackNum,
    )
    const labSrc = map.getSource(LABELS_SOURCE_ID)
    const llSrc  = map.getSource(LEADER_LINES_SOURCE_ID)
    const selLabSrc = map.getSource(SELECTION_LABEL_SOURCE_ID)
    if (labSrc) labSrc.setData({ type: 'FeatureCollection', features: labelFeatures })
    if (llSrc)  llSrc.setData({ type: 'FeatureCollection', features: leaderLineFeatures })
    // ASD-011b: the selected label's outline box (0 or 1 feature).
    if (selLabSrc) selLabSrc.setData({ type: 'FeatureCollection', features: selectionBoxFeatures || [] })
  } catch (err) {
    console.error('[ASD-002] label deconfliction error:', err)
  }
}

// tickFade advances the TSE fade-out animation (ASD-004c). Runs every 50 ms
// while fadingTracks is non-empty. Expired tracks are evicted from all state
// maps; the interval clears itself when all fading tracks have disappeared.
// Returns true if the loop should continue, false when it can be stopped.
export function tickFade(state, renderFn) {
  const now = Date.now()
  for (const [num, { deadline }] of state.fadingTracks) {
    if (now >= deadline) {
      state.fadingTracks.delete(num)
      state.trackHistory.delete(num)
      state.trackCoasting.delete(num)
      state.labelPins.delete(num) // ASD-002: drop pin for expired track
    }
  }

  renderFn()

  return state.fadingTracks.size > 0
}
