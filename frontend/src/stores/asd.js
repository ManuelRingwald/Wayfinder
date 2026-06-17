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
  })

  // FL filter
  const flFilter = reactive({
    minFL: null,
    maxFL: null,
    hide: false,
  })

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
    selectedTrack, labelPins,
    setFeedStatus, setMapLoaded, setPalette, setLayerVisibility,
    setFlFilter, selectTrack, clearTrackSelection, setLabelPin, deleteLabelPin,
  }
})
