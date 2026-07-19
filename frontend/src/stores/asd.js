import { defineStore } from 'pinia'
import { ref, reactive, computed } from 'vue'
import { apiFetch } from '@/api.js'
import { DEFAULT_RANGE_RING_SPACING_NM, DEFAULT_RANGE_RING_COUNT, DEFAULT_HISTORY_DURATION_S } from '@/map/constants.js'

// #245 Teil B: the correlation endpoint (pkg/correlationapi) answers with HTTP
// statuses; this table turns them into short German controller-facing lines. The
// server also returns an {"error":...} body, but it is English and generic — a
// status-keyed table gives a clear, localized message without leaking backend
// phrasing. Any unlisted status falls back to the raw error.
const CORRELATION_ERROR_MESSAGES = {
  400: 'Ungültige Eingabe für die Korrelation.',
  401: 'Nicht angemeldet.',
  403: 'Für diesen Feed nicht berechtigt.',
  409: 'Der Tracker dieses Feeds hat keine Flugpläne konfiguriert.',
  422: 'Kein Flugplan mit dieser Kennung gefunden.',
  502: 'Tracker nicht erreichbar.',
  503: 'Manuelle Korrelation ist nicht aktiviert.',
}

// The broadcast FeedStatusMessage carries a per-feed traffic-light *color*
// (green/yellow/red, pkg/broadcast); the chip speaks in states. Mapping the
// vocabulary here (not in the chip) keeps the wire contract in one place (#117).
const FEED_COLOR_TO_STATE = { green: 'ok', yellow: 'degraded', red: 'stale' }
const FEED_STATE_RANK = { ok: 0, degraded: 1, stale: 2 }
// Per-source failure reason ranking (CAT063 I063/RE SRC-REASON, Firefly ADR 0033):
// when several degraded feeds disagree, the chip shows the most operator-actionable
// reason. Mirrors the backend's cat063.reasonPriority so the two never diverge.
const FEED_REASON_RANK = { auth: 3, rate_limited: 2, unreachable: 1 }

export const useAsdStore = defineStore('asd', () => {
  // Map/app state
  const mapLoaded = ref(false)
  const palette = ref('bkg-dark') // 'bkg-dark' | 'bkg' (ADR 0026)

  // Feed health per feed (#117): feedId → 'ok' | 'degraded' | 'stale'. A tenant
  // can be subscribed to several feeds; the chip shows the WORST state so a dead
  // feed is never masked by a healthy one. 'unknown' until the first status.
  const feedHealth = ref(new Map())
  // feedId → per-source failure reason string ('' when none). Parallel to
  // feedHealth so the existing state map keeps its simple shape (#117).
  const feedReasons = ref(new Map())
  // feedId → per-sensor breakdown array from the last CAT063 block (#237): each
  // { sac, sic, operational, degraded_reason?, range_bias_m?, azimuth_bias_deg? }.
  // Drives the feed-health chip's expandable per-sensor bias view.
  const feedSensors = ref(new Map())
  const feedStatus = computed(() => {
    let worst = null
    for (const state of feedHealth.value.values()) {
      if (worst === null || FEED_STATE_RANK[state] > FEED_STATE_RANK[worst]) worst = state
    }
    return worst ?? 'unknown'
  })
  // feedDegradedReason is the reason shown on the chip when the aggregate state
  // is 'degraded': the most operator-actionable reason among the degraded feeds
  // (CAT063 I063/RE, Firefly ADR 0033). '' when not degraded or no known reason.
  const feedDegradedReason = computed(() => {
    if (feedStatus.value !== 'degraded') return ''
    let best = ''
    for (const [feedId, state] of feedHealth.value) {
      if (state !== 'degraded') continue
      const reason = feedReasons.value.get(feedId) || ''
      if ((FEED_REASON_RANK[reason] || 0) > (FEED_REASON_RANK[best] || 0)) best = reason
    }
    return best
  })

  // #114: whether server-side coverage sensors are configured at all. The
  // sidebar disables the "Radarabdeckung" toggle when there is no data — a
  // switch that visibly does nothing reads as a bug. Set by the engine from
  // /api/map-config (coverage_sensor_count).
  const coverageAvailable = ref(false)
  function setCoverageAvailable(v) { coverageAvailable.value = !!v }

  // WX-A: whether the DWD weather-radar overlay is configured on the backend.
  // Set by the engine from /api/map-config (weather_radar_available); gates the
  // sidebar toggle so a switch that would do nothing is disabled.
  const weatherRadarAvailable = ref(false)
  function setWeatherRadarAvailable(v) { weatherRadarAvailable.value = !!v }

  // WX-C: whether the DWD weather-warnings overlay is configured on the backend.
  const weatherWarningsAvailable = ref(false)
  function setWeatherWarningsAvailable(v) { weatherWarningsAvailable.value = !!v }

  // #245 Teil B: whether manual flight-plan correlation is enabled on the backend
  // (a command token is configured, WAYFINDER_FIREFLY_COMMAND_TOKEN). Set by the
  // engine from /api/map-config (correlation_available). Gates the correlation
  // controls in the detail panel so controls that could only ever answer 503 are
  // never shown — the server still enforces the feature edge independently.
  const correlationAvailable = ref(false)
  function setCorrelationAvailable(v) { correlationAvailable.value = !!v }

  // Layer visibility
  const layerVisibility = reactive({
    airspace: true,
    aor: true, // ASD-014: AoR highlight on by default (only shows when configured)
    navaids: true,
    waypoints: true,
    coverageRings: true,
    rangeRings: false, // ASD-012: off by default (declutter); operator opts in
    historyDots: true, // AP2: on by default; hidden by feature gate when tenant lacks history_dots
    weatherRadar: false, // WX-A: off by default (weather is opt-in context)
    weatherWarnings: false, // WX-C: off by default (opt-in context)
    airport: false, // #192: airport markers off by default (opt-in context)
    runways: false, // #192: runway centrelines off by default (opt-in context)
  })

  // ASD-012: operator-tunable range-ring configuration, applied live. The engine
  // regenerates the overlay from the configured centre whenever this changes.
  const rangeRingConfig = reactive({
    spacingNM: DEFAULT_RANGE_RING_SPACING_NM,
    count: DEFAULT_RANGE_RING_COUNT,
  })
  function setRangeRingConfig(updates) { Object.assign(rangeRingConfig, updates) }

  // #191: history-dots retention window (seconds). The engine prunes/fades the
  // dots to this duration; MapCanvas watches it and re-renders on change.
  const historyConfig = reactive({
    durationS: DEFAULT_HISTORY_DURATION_S,
  })
  function setHistoryConfig(updates) { Object.assign(historyConfig, updates) }

  // FL filter
  const flFilter = reactive({
    minFL: null,
    maxFL: null,
    hide: false,
  })

  // ASD-011: per-group visibility for airspace category filter.
  // All groups visible by default; MapCanvas watches this and calls
  // mapEngine.updateAirspaceFilter() to apply MapLibre setFilter.
  const airspaceGroupVisibility = reactive({
    ctr: true,
    tma: true,
    restricted: true,
    info: true,
  })

  // #176: the standalone "Lufträume" parent toggle was removed. The airspace
  // LAYER is now visible iff at least one group is on — derived here so both the
  // group filter (airspaceGroupVisibility) and the layer visibility
  // (layerVisibility.airspace) update together, and MapCanvas's two watchers
  // (updateAirspaceFilter + setLayerVisibility) keep the map in sync.
  function setAirspaceGroup(id, val) {
    airspaceGroupVisibility[id] = val
    layerVisibility.airspace = Object.values(airspaceGroupVisibility).some(Boolean)
  }
  function toggleAirspaceGroup(id) {
    setAirspaceGroup(id, !airspaceGroupVisibility[id])
  }

  // Selected track for detail panel (null = no selection)
  const selectedTrack = ref(null)

  // ASD-013: the set of track numbers currently on the scope (live + coasting,
  // i.e. everything in the engine's liveTrackFeatures). Kept here so the
  // Ereignis-Panel can tell whether a "Track N erschienen" event still refers to
  // a selectable track — only those rows are made clickable (operator request
  // 2026-07-08). The engine pushes this from every track batch; an ended/faded
  // track drops out and its event row goes inert.
  const liveTrackNums = ref(new Set())

  // Label pins: Map<track_num, {dx, dy}>
  const labelPins = ref(new Map())

  // setFeedHealth records one feed's health from a WS feed_status message. An
  // unknown color is ignored (fail-safe: never corrupt the chip on a newer
  // server vocabulary). resetFeedHealth clears all entries — called on WS
  // (re)connect so statuses from a previous scope never linger.
  function setFeedHealth(feedId, color, reason = '', sensors = []) {
    const state = FEED_COLOR_TO_STATE[color]
    if (!state) return
    const id = feedId ?? 0
    const m = new Map(feedHealth.value)
    m.set(id, state)
    feedHealth.value = m
    const rm = new Map(feedReasons.value)
    rm.set(id, reason || '')
    feedReasons.value = rm
    const sm = new Map(feedSensors.value)
    sm.set(id, Array.isArray(sensors) ? sensors : [])
    feedSensors.value = sm
  }
  function resetFeedHealth() {
    feedHealth.value = new Map()
    feedReasons.value = new Map()
    feedSensors.value = new Map()
  }

  // sensorDetails flattens the per-feed sensor breakdown (#237) into a single
  // list for the feed-health chip's expandable detail; each entry keeps its
  // feedId so a multi-feed operator can tell the sensors apart.
  const sensorDetails = computed(() => {
    const out = []
    for (const [feedId, list] of feedSensors.value) {
      for (const s of list) out.push({ feedId, ...s })
    }
    return out
  })
  function setMapLoaded(val) { mapLoaded.value = val }
  function setPalette(p) { palette.value = p }
  function setLayerVisibility(layer, val) { layerVisibility[layer] = val }
  function setFlFilter(updates) { Object.assign(flFilter, updates) }
  function selectTrack(track) { selectedTrack.value = track }
  function clearTrackSelection() { selectedTrack.value = null }
  // #272: refresh the selected track's snapshot from the displayed feature set
  // so the open detail panel follows every WS update instead of freezing at
  // selection time. A fresh object is assigned (Vue reactivity keys off
  // identity). A track that vanished (TSE / out of scope) keeps its last
  // snapshot — the panel stays open with the final values, matching the
  // established selectTrackByNum no-op behaviour (FR-UI-029).
  function refreshSelectedTrack(liveFeatures) {
    const current = selectedTrack.value
    if (!current) return
    const feature = (liveFeatures || []).find(
      (f) => f.properties.track_num === current.track_num,
    )
    if (feature) selectedTrack.value = { ...feature.properties }
  }

  // #245 Teil B: manual flight-plan correlation commands (ADR 0024). Each is the
  // browser half of the first Wayfinder→Firefly WRITE path: it posts to the
  // backend correlation endpoint (pkg/correlationapi), which authorises the
  // caller (must be subscribed to feedId) and relays the command to the feed's
  // Firefly instance. Every action returns a uniform { ok, message } so the
  // detail panel can show one synchronous result line without status branching.
  // feedId must be a real catalogue feed (> 0); the caller guards on the track's
  // feed_id being present (the ENV fallback feed has no command channel).
  function correlationResult(r, successMsg) {
    if (r.ok) return { ok: true, message: successMsg }
    return { ok: false, message: CORRELATION_ERROR_MESSAGES[r.status] || r.error || 'Korrelation fehlgeschlagen.' }
  }
  // correlate pins trackNum to the filed flight plan identified by callsign.
  async function correlate(feedId, trackNum, callsign) {
    const r = await apiFetch('/api/correlation', {
      method: 'POST',
      body: JSON.stringify({ feed_id: feedId, track_number: trackNum, callsign }),
    })
    return correlationResult(r, `Track ${trackNum} mit ${callsign} korreliert.`)
  }
  // setUncorrelated pins trackNum explicitly uncorrelated (suppress the automatic
  // match). A null callsign on the wire is the "uncorrelate" signal.
  async function setUncorrelated(feedId, trackNum) {
    const r = await apiFetch('/api/correlation', {
      method: 'POST',
      body: JSON.stringify({ feed_id: feedId, track_number: trackNum, callsign: null }),
    })
    return correlationResult(r, `Track ${trackNum} als unkorreliert markiert.`)
  }
  // clearOverride removes any manual override for trackNum so Firefly's automatic
  // correlation resumes. Idempotent on the server.
  async function clearOverride(feedId, trackNum) {
    const r = await apiFetch(`/api/correlation/${feedId}/${trackNum}`, { method: 'DELETE' })
    return correlationResult(r, `Manuelle Korrelation für Track ${trackNum} aufgehoben.`)
  }
  // setLiveTrackNums replaces the live-track set (accepts an array or a Set). A
  // fresh Set instance is stored so the reactive read in the Ereignis-Panel
  // re-evaluates. Cheap: called once per track batch (every scan, ~4–12 s).
  function setLiveTrackNums(nums) {
    liveTrackNums.value = nums instanceof Set ? nums : new Set(nums)
  }

  function setLabelPin(trackNum, pin) {
    const m = new Map(labelPins.value)
    m.set(trackNum, pin)
    labelPins.value = m
  }
  function deleteLabelPin(trackNum) {
    const m = new Map(labelPins.value)
    m.delete(trackNum)
    labelPins.value = m
  }

  return {
    mapLoaded, palette, feedStatus, feedHealth, feedDegradedReason, feedSensors, sensorDetails, layerVisibility, flFilter,
    coverageAvailable, setCoverageAvailable,
    weatherRadarAvailable, setWeatherRadarAvailable,
    weatherWarningsAvailable, setWeatherWarningsAvailable,
    correlationAvailable, setCorrelationAvailable,
    airspaceGroupVisibility,
    rangeRingConfig, setRangeRingConfig,
    historyConfig, setHistoryConfig,
    selectedTrack, labelPins, liveTrackNums,
    setFeedHealth, resetFeedHealth, setMapLoaded, setPalette, setLayerVisibility,
    setFlFilter,
    toggleAirspaceGroup, setAirspaceGroup,
    selectTrack, clearTrackSelection, refreshSelectedTrack, setLabelPin, deleteLabelPin, setLiveTrackNums,
    correlate, setUncorrelated, clearOverride,
  }
})
