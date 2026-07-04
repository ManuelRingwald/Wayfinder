<template>
  <div style="position: absolute; inset: 0">
    <div ref="mapEl" style="width: 100%; height: 100%" />
    <!-- Häppchen 3: zoom moved to the navigation rail; MapControls keeps the
         viewport actions (recenter / fullscreen) on the right edge. -->
    <MapControls
      @recenter="mapEngine?.recenter()"
    />
    <!-- Häppchen 3: measuring status (hint + readout); the tool buttons now live
         in the navigation rail. -->
    <MeasureStatus />
    <!-- WF2-34: admin read-only impersonation banner/switcher (ADR 0008) -->
    <ImpersonationBar />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { useImpersonationStore } from '@/stores/impersonation.js'
import { useSessionStore } from '@/stores/session.js'
import { useToolsStore } from '@/stores/tools.js'
import { initMap } from '@/map/engine.js'
import { createMeasure } from '@/map/measure.js'
import MapControls from './MapControls.vue'
import MeasureStatus from './MeasureStatus.vue'
import ImpersonationBar from './ImpersonationBar.vue'

const emit = defineEmits(['track-click', 'connection-change'])
const store = useAsdStore()
const imp = useImpersonationStore()
const session = useSessionStore()
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
    // FR-UI-013: open on the tenant's own sector (whoami view centre) instead of
    // the global map-config default. Null (no view config) keeps the env centre.
    session.viewCenter,
  )
  // Häppchen 4: attach the measurement controller and let the tools store drive
  // it (reporting the live readout back to the store). createMeasure adds a
  // source + layers, so it MUST run after the style has loaded — initMap returns
  // before the map's 'load' event, so calling it eagerly threw "style not loaded"
  // and left the tools dead (RBL/DIST/QDM did nothing). Defer to 'load'.
  const map = mapEngine.map
  const setupMeasure = () => {
    measure = createMeasure(map, { onReadout: (t, at) => tools.setReadout(t, at) })
    measure.setTool(tools.activeTool) // honour a tool selected before load
  }
  if (map.loaded()) setupMeasure()
  else map.once('load', setupMeasure)
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

// ASD-007: selection halo — re-render when the selected track changes so the
// cyan ring appears/moves/clears immediately (not only on the next WS update).
watch(() => store.selectedTrack?.track_num, () => {
  mapEngine?.updateSelection()
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

// FR-UI-013: re-aim the camera when the effective view centre changes — either
// because whoami resolved after the map mounted, or an admin switched the
// impersonation target (ADR 0008) to a tenant with a different sector. The engine
// no-ops when the centre is unchanged, so this never fights the user's panning.
watch(() => session.viewCenter, (vc) => {
  mapEngine?.applyViewCenter(vc)
})

defineExpose({
  // Häppchen 3: zoom is driven from the navigation rail, which delegates here.
  zoomIn: () => mapEngine?.zoomIn(),
  zoomOut: () => mapEngine?.zoomOut(),
  setLayerVisibility: (layer, val) => mapEngine?.setLayerVisibility({ [layer]: val }),
  updateFlFilter: () => mapEngine?.updateFlFilter(),
  // #121: MapLibre must be told when its container changes size (drawer/panel
  // collapse), otherwise a dead, unpainted strip is left where the panel was.
  resize: () => mapEngine?.map?.resize(),
})
</script>

