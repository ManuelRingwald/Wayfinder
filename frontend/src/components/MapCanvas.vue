<template>
  <div style="position: absolute; inset: 0">
    <div ref="mapEl" style="width: 100%; height: 100%" />
    <MapControls
      @zoom-in="mapEngine?.zoomIn()"
      @zoom-out="mapEngine?.zoomOut()"
      @recenter="mapEngine?.recenter()"
    />
    <!-- ASD-013: FeedStatusChip moved here after app bar removal -->
    <FeedStatusChip class="feed-chip" />
    <!-- ASD-010: category filter chips top-centre -->
    <TrackFilterChips />
    <!-- WF2-34: super_admin read-only impersonation banner/switcher (ADR 0008) -->
    <ImpersonationBar />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { useImpersonationStore } from '@/stores/impersonation.js'
import { initMap } from '@/map/engine.js'
import MapControls from './MapControls.vue'
import TrackFilterChips from './TrackFilterChips.vue'
import FeedStatusChip from './FeedStatusChip.vue'
import ImpersonationBar from './ImpersonationBar.vue'

const emit = defineEmits(['track-click'])
const store = useAsdStore()
const imp = useImpersonationStore()
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

// ASD-011: apply airspace group filter once after map load (so the initial
// state is correctly reflected) and whenever the store changes.
watch(() => store.mapLoaded, (loaded) => {
  if (loaded) mapEngine?.updateAirspaceFilter()
})
watch(() => ({ ...store.airspaceGroupVisibility }), () => {
  mapEngine?.updateAirspaceFilter()
}, { deep: true })

// ASD-012: regenerate the range-ring overlay when the operator changes spacing
// or count (visibility itself is handled by the layerVisibility watcher above).
watch(() => ({ ...store.rangeRingConfig }), (cfg) => {
  mapEngine?.updateRangeRings(cfg.spacingNM, cfg.count)
}, { deep: true })

// WF2-34: when the super_admin starts/switches/exits read-only impersonation
// (ADR 0008), reconnect the WebSocket so the new grant cookie — and thus the new
// tenant scope — takes effect immediately. loadStatus does not bump the nonce, so
// a normal page load reconnects only via the engine's own connect.
watch(() => imp.reconnectNonce, () => {
  mapEngine?.reconnect()
})

defineExpose({
  setLayerVisibility: (layer, val) => mapEngine?.setLayerVisibility({ [layer]: val }),
  updateFlFilter: () => mapEngine?.updateFlFilter(),
})
</script>

<style scoped>
/* ASD-013: Feed chip anchored top-right, beside map controls. */
.feed-chip {
  position: absolute;
  top: 12px;
  right: 60px;
  z-index: 10;
}
</style>
