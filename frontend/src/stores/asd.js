import { defineStore } from 'pinia'
import { ref, reactive, computed } from 'vue'
import { DEFAULT_RANGE_RING_SPACING_NM, DEFAULT_RANGE_RING_COUNT } from '@/map/constants.js'

// The broadcast FeedStatusMessage carries a per-feed traffic-light *color*
// (green/yellow/red, pkg/broadcast); the chip speaks in states. Mapping the
// vocabulary here (not in the chip) keeps the wire contract in one place (#117).
const FEED_COLOR_TO_STATE = { green: 'ok', yellow: 'degraded', red: 'stale' }
const FEED_STATE_RANK = { ok: 0, degraded: 1, stale: 2 }

export const useAsdStore = defineStore('asd', () => {
  // Map/app state
  const mapLoaded = ref(false)
  const palette = ref('dark') // 'dark' | 'osm'

  // Feed health per feed (#117): feedId → 'ok' | 'degraded' | 'stale'. A tenant
  // can be subscribed to several feeds; the chip shows the WORST state so a dead
  // feed is never masked by a healthy one. 'unknown' until the first status.
  const feedHealth = ref(new Map())
  const feedStatus = computed(() => {
    let worst = null
    for (const state of feedHealth.value.values()) {
      if (worst === null || FEED_STATE_RANK[state] > FEED_STATE_RANK[worst]) worst = state
    }
    return worst ?? 'unknown'
  })

  // #114: whether server-side coverage sensors are configured at all. The
  // sidebar disables the "Radarabdeckung" toggle when there is no data — a
  // switch that visibly does nothing reads as a bug. Set by the engine from
  // /api/map-config (coverage_sensor_count).
  const coverageAvailable = ref(false)
  function setCoverageAvailable(v) { coverageAvailable.value = !!v }

  // Layer visibility
  const layerVisibility = reactive({
    airspace: true,
    navaids: true,
    waypoints: true,
    coverageRings: true,
    rangeRings: false, // ASD-012: off by default (declutter); operator opts in
    historyDots: true, // AP2: on by default; hidden by feature gate when tenant lacks history_dots
  })

  // ASD-012: operator-tunable range-ring configuration, applied live. The engine
  // regenerates the overlay from the configured centre whenever this changes.
  const rangeRingConfig = reactive({
    spacingNM: DEFAULT_RANGE_RING_SPACING_NM,
    count: DEFAULT_RANGE_RING_COUNT,
  })
  function setRangeRingConfig(updates) { Object.assign(rangeRingConfig, updates) }

  // FL filter
  const flFilter = reactive({
    minFL: null,
    maxFL: null,
    hide: false,
  })

  // ASD-010: live track counts per status category.
  // Updated by the engine after every WS frame; consumed by TrackFilterChips.
  const trackCounts = reactive({
    confirmed: 0,
    coasting: 0,
    tentative: 0,
  })

  // ASD-010: categories currently hidden via the filter chips.
  // Engine's renderSources skips features in this set.
  const hiddenCategories = reactive(new Set())

  // ASD-011: per-group visibility for airspace category filter.
  // All groups visible by default; MapCanvas watches this and calls
  // mapEngine.updateAirspaceFilter() to apply MapLibre setFilter.
  const airspaceGroupVisibility = reactive({
    ctr: true,
    tma: true,
    restricted: true,
    info: true,
  })

  function toggleAirspaceGroup(id) {
    airspaceGroupVisibility[id] = !airspaceGroupVisibility[id]
  }

  // Selected track for detail panel (null = no selection)
  const selectedTrack = ref(null)

  // Label pins: Map<track_num, {dx, dy}>
  const labelPins = ref(new Map())

  // setFeedHealth records one feed's health from a WS feed_status message. An
  // unknown color is ignored (fail-safe: never corrupt the chip on a newer
  // server vocabulary). resetFeedHealth clears all entries — called on WS
  // (re)connect so statuses from a previous scope never linger.
  function setFeedHealth(feedId, color) {
    const state = FEED_COLOR_TO_STATE[color]
    if (!state) return
    const m = new Map(feedHealth.value)
    m.set(feedId ?? 0, state)
    feedHealth.value = m
  }
  function resetFeedHealth() { feedHealth.value = new Map() }
  function setMapLoaded(val) { mapLoaded.value = val }
  function setPalette(p) { palette.value = p }
  function setLayerVisibility(layer, val) { layerVisibility[layer] = val }
  function setFlFilter(updates) { Object.assign(flFilter, updates) }
  function selectTrack(track) { selectedTrack.value = track }
  function clearTrackSelection() { selectedTrack.value = null }

  // ASD-010: update live counts (called by engine after each WS frame).
  function setTrackCounts(counts) {
    trackCounts.confirmed = counts.confirmed ?? 0
    trackCounts.coasting = counts.coasting ?? 0
    trackCounts.tentative = counts.tentative ?? 0
  }

  // ASD-010: toggle a category in/out of the hidden set.
  function toggleCategoryFilter(category) {
    if (hiddenCategories.has(category)) {
      hiddenCategories.delete(category)
    } else {
      hiddenCategories.add(category)
    }
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
    mapLoaded, palette, feedStatus, feedHealth, layerVisibility, flFilter,
    coverageAvailable, setCoverageAvailable,
    trackCounts, hiddenCategories,
    airspaceGroupVisibility,
    rangeRingConfig, setRangeRingConfig,
    selectedTrack, labelPins,
    setFeedHealth, resetFeedHealth, setMapLoaded, setPalette, setLayerVisibility,
    setFlFilter, setTrackCounts, toggleCategoryFilter,
    toggleAirspaceGroup,
    selectTrack, clearTrackSelection, setLabelPin, deleteLabelPin,
  }
})
