<template>
  <!-- Wrapper with position:relative so MapControls and TrackFilterChips
       can be positioned absolutely over the MapLibre canvas. -->
  <div style="position: relative; width: 100%; height: 100%">
    <div ref="mapEl" style="width: 100%; height: 100%" />
    <MapControls
      @zoom-in="mapEngine?.zoomIn()"
      @zoom-out="mapEngine?.zoomOut()"
      @recenter="mapEngine?.recenter()"
      @reset-north="mapEngine?.resetNorth()"
    />
    <!-- ASD-010: category filter chips top-centre -->
    <TrackFilterChips />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { initMap } from '@/map/engine.js'
import MapControls from './MapControls.vue'
import TrackFilterChips from './TrackFilterChips.vue'

const emit = defineEmits(['track-click'])
const store = useAsdStore()
const mapEl = ref(null)
let mapEngine = null

onMounted(async () => {
  mapEngine = await initMap(mapEl.value, store, (track) => emit('track-click', track))
})

onUnmounted(() => {
  mapEngine?.destroy()
})

// Layer visibility reactive sync
watch(() => ({ ...store.layerVisibility }), (vis) => {
  mapEngine?.setLayerVisibility(vis)
}, { deep: true })

// FL filter reactive sync
watch(() => ({ ...store.flFilter }), () => {
  mapEngine?.updateFlFilter()
}, { deep: true })

// ASD-010: re-render when category filter changes (hiddenCategories is a
// reactive Set; we watch its size as a proxy for any add/delete).
watch(() => store.hiddenCategories.size, () => {
  mapEngine?.updateFlFilter()
})

defineExpose({
  setLayerVisibility: (layer, val) => mapEngine?.setLayerVisibility({ [layer]: val }),
  updateFlFilter: () => mapEngine?.updateFlFilter(),
})
</script>
