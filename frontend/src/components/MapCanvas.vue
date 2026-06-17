<template>
  <div ref="mapEl" style="width: 100%; height: 100%" />
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { initMap } from '@/map/engine.js'

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

defineExpose({
  setLayerVisibility: (layer, val) => mapEngine?.setLayerVisibility({ [layer]: val }),
  updateFlFilter: () => mapEngine?.updateFlFilter(),
})
</script>
