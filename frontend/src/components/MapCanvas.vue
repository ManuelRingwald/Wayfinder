<template>
  <div style="position: absolute; inset: 0">
    <div ref="mapEl" style="width: 100%; height: 100%" />
    <!-- ASD-019 (ADR 0030): the bottom-right map-control stack renders on BOTH
         desktop and mobile now — zoom moved off the navigation rail onto the
         scope. MapControls itself gates the viewport actions to mobile (on desktop
         recenter/fullscreen stay in AsdView's top-right cluster, ADR 0029), so it
         shows only zoom on desktop and zoom + viewport actions on mobile. -->
    <MapControls
      @recenter="mapEngine?.recenter()"
      @zoom-in="mapEngine?.zoomIn()"
      @zoom-out="mapEngine?.zoomOut()"
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

const emit = defineEmits(['track-click', 'connection-change', 'empty-click'])
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
    // #189/#190: clip the DWD weather overlays (radar/warnings) to the tenant AOI.
    session.aoi,
    // #273: click on free map area (no track/label hit) → parent decides
    // whether to deselect (tool guard lives there, mirroring track-click).
    () => emit('empty-click'),
    // #297: feed each track batch to the measure controller so a track-anchored
    // measure endpoint follows the moving track. No-op until the controller is
    // set up on map 'load' (measure is null before then).
    (features) => measure?.refreshTracks(features),
  )
  // #219: initMap is async (it awaits /api/map-config), so session.viewCenter /
  // session.aoi can resolve DURING that await — most notably when an admin enters
  // read-only guest mode: the session store still holds the stale, non-impersonated
  // view (empty centre) when the map mounts, and the impersonation-aware whoami
  // lands a moment later. The viewCenter/aoi watchers below fire against a still-null
  // mapEngine in that window, so their re-aim is silently lost and the map stays on
  // — and "Ansicht zurücksetzen" resets to — the global Frankfurt default instead of
  // the impersonated tenant's sector. Reconcile once the engine exists so the opening
  // centre + AOI always reflect the CURRENT effective view (no-op when unchanged).
  mapEngine.applyViewCenter(session.viewCenter)
  mapEngine.applyWeatherAOI(session.aoi)
  // ASD-014: same #219 race — apply the AoR highlight once the engine exists, so
  // a late-resolving (or impersonation-switched) whoami is reflected on mount.
  mapEngine.updateAoR(session.aorAirspaceIds)
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

// E2 (#293): re-apply the base map when a per-element switch changes (Gewässer/
// Verkehr/…). The master (layerVisibility.basemap) flows through the watcher
// above; applyBasemap combines both, so an element toggle takes effect at once.
watch(() => ({ ...store.basemapElementVisibility }), () => {
  mapEngine?.applyBasemap()
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

// ASD-011: re-apply the airspace group filter whenever the store changes. The
// INITIAL application is done by the engine itself in its load handler (#179),
// so it runs on every mount regardless of the store's false→true edge — this
// watcher is a belt-and-suspenders re-sync for the first-mount edge and does
// not carry correctness on a remount (store.mapLoaded is a write-once latch).
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

// #191: re-render the history dots when the operator changes the retention
// window (duration), so the trail length and age fade update immediately.
watch(() => ({ ...store.historyConfig }), () => {
  mapEngine?.updateHistoryConfig()
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

// #189/#190: re-clip the DWD weather overlays when the effective AOI changes
// (whoami resolves after mount, or an admin switches the impersonation target).
watch(() => session.aoi, (a) => {
  mapEngine?.applyWeatherAOI(a)
}, { deep: true })

// ASD-014: re-apply the AoR highlight when the effective Area of Responsibility
// changes (whoami resolves after mount, or an admin switches the impersonation
// target to a tenant with a different AoR). Empty list highlights nothing.
watch(() => session.aorAirspaceIds, (ids) => {
  mapEngine?.updateAoR(ids)
}, { deep: true })

defineExpose({
  // ASD-019 (ADR 0030): zoom is driven by the bottom-right MapControls, whose
  // zoom-in/zoom-out are wired straight to the engine in this component's template
  // — so no zoomIn/zoomOut needs to be exposed here any more.
  // ASD-018 (ADR 0029): recenter is driven from the desktop top-right cluster's
  // ViewportControls (AsdView), so expose it.
  recenter: () => mapEngine?.recenter(),
  setLayerVisibility: (layer, val) => mapEngine?.setLayerVisibility({ [layer]: val }),
  updateFlFilter: () => mapEngine?.updateFlFilter(),
  // #121: MapLibre must be told when its container changes size (drawer/panel
  // collapse), otherwise a dead, unpainted strip is left where the panel was.
  resize: () => mapEngine?.map?.resize(),
  // ASD-013: select a track by number from the Ereignis-Panel. Returns false if
  // the track is no longer live so AsdView can leave the panel open.
  selectTrackByNum: (trackNum) => mapEngine?.selectTrackByNum(trackNum) ?? false,
  // #277: sector search — drop/clear the result marker (MapSearch via AsdView).
  showSearchMarker: (lon, lat, name) => mapEngine?.showSearchMarker(lon, lat, name),
  clearSearchMarker: () => mapEngine?.clearSearchMarker(),
})
</script>

