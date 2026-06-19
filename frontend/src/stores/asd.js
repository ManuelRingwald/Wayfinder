import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'

export const useAsdStore = defineStore('asd', () => {
  // Map/app state
  const mapLoaded = ref(false)
  const palette = ref('dark') // 'dark' | 'osm'

  // Feed health: 'unknown' | 'ok' | 'stale'
  const feedStatus = ref('unknown')

  // Layer visibility
  const layerVisibility = reactive({
    airspace: true,
    navaids: true,
    waypoints: true,
    coverageRings: true,
  })

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

  function setFeedStatus(status) { feedStatus.value = status }
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
    mapLoaded, palette, feedStatus, layerVisibility, flFilter,
    trackCounts, hiddenCategories,
    airspaceGroupVisibility,
    selectedTrack, labelPins,
    setFeedStatus, setMapLoaded, setPalette, setLayerVisibility,
    setFlFilter, setTrackCounts, toggleCategoryFilter,
    toggleAirspaceGroup,
    selectTrack, clearTrackSelection, setLabelPin, deleteLabelPin,
  }
})
