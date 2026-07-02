<template>
  <div style="position: absolute; inset: 0">
    <div ref="mapEl" style="width: 100%; height: 100%" />
    <MapControls
      @zoom-in="mapEngine?.zoomIn()"
      @zoom-out="mapEngine?.zoomOut()"
      @recenter="mapEngine?.recenter()"
    />
    <!-- ASD-010: category filter chips top-centre -->
    <TrackFilterChips />
    <!-- Häppchen 4: controller measurement tools (RBL/DIST/QDM) -->
    <MeasureToolbar />
    <!-- WF2-34: admin read-only impersonation banner/switcher (ADR 0008) -->
    <ImpersonationBar />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { useImpersonationStore } from '@/stores/impersonation.js'
import { useToolsStore } from '@/stores/tools.js'
import { initMap } from '@/map/engine.js'
import { createMeasure } from '@/map/measure.js'
import MapControls from './MapControls.vue'
import TrackFilterChips from './TrackFilterChips.vue'
import MeasureToolbar from './MeasureToolbar.vue'
import ImpersonationBar from './ImpersonationBar.vue'

const emit = defineEmits(['track-click', 'connection-change'])
const store = useAsdStore()
const imp = useImpersonationStore()
const tools = useToolsStore()
const mapEl = ref(null)
let mapEngine = null
let measure = null

onMounted(async () => {
  mapEngine = await initMap(
    mapEl.value,
    store,
    (track) => emit('track-click', track),
    (state) => emit('connection-change', state),
  )
  // Häppchen 4: attach the measurement controller to the ready map and let the
  // tools store drive it. The controller reports the live readout back to the store.
  measure = createMeasure(mapEngine.map, { onReadout: (t) => tools.setReadout(t) })
  watch(() => tools.activeTool, (kind) => measure?.setTool(kind))
})

onUnmounted(() => {
  measure?.destroy()
  tools.clearTool()
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

// WF2-34: when the admin starts/switches/exits read-only impersonation
// (ADR 0008), reconnect the WebSocket so the new grant cookie — and thus the new
// tenant scope — takes effect immediately. loadStatus does not bump the nonce, so
// a normal page load reconnects only via the engine's own connect.
watch(() => imp.reconnectNonce, () => {
  mapEngine?.reconnect()
})

defineExpose({
  setLayerVisibility: (layer, val) => mapEngine?.setLayerVisibility({ [layer]: val }),
  updateFlFilter: () => mapEngine?.updateFlFilter(),
  // #121: MapLibre must be told when its container changes size (drawer/panel
  // collapse), otherwise a dead, unpainted strip is left where the panel was.
  resize: () => mapEngine?.map?.resize(),
})
</script>

