// Map engine: initialises MapLibre, wires all layers, manages the WebSocket
// connection and the ASD rendering loop.
//
// The engine owns a local `state` object that mirrors the original app.js
// `state` — all mutable ASD runtime state lives here. The Pinia store is used
// only for UI-facing state (feedStatus, mapLoaded, palette key).
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { PALETTES, TRACKS_LAYER_ID, AIRSPACE_GROUPS } from './constants.js'
import { haversineNM } from './tools.js'
import {
  addAeronauticalIcons,
  addAirspaceLayers,
  addNavaidLayers,
  addWaypointLayers,
  addTracksLayer,
  addLeaderLinesLayer,
  addSelectionLayer,
  addLabelsLayer,
  addTrailsLayer,
  addHistoryDotsLayer,
  addVectorsLayer,
  addCoverageLayer,
  updateCoverageSource,
  addRangeRingsLayer,
  addWeatherRadarLayer,
  addWeatherWarningsLayer,
  updateWeatherWarnings,
} from './layers.js'
import { rangeRingsGeoJSON } from './rangerings.js'
import { updateTracksLayer } from './tracks.js'
import { renderSources, tickFade } from './render.js'
import { setupLabelDrag } from './drag.js'
import { startAeronauticalRefresh } from './aeronautical.js'
import {
  AIRSPACE_FILL_LAYER_ID,
  AIRSPACE_LINE_LAYER_ID,
  AIRSPACE_LABEL_LAYER_ID,
  NAVAIDS_LAYER_ID,
  WAYPOINTS_LAYER_ID,
  COVERAGE_RINGS_LAYER_ID,
  COVERAGE_CENTER_LAYER_ID,
  RANGE_RINGS_SOURCE_ID,
  RANGE_RINGS_LAYER_ID,
  RANGE_RINGS_LABEL_LAYER_ID,
  HISTORY_DOTS_LAYER_ID,
  WEATHER_RADAR_LAYER_ID,
  WEATHER_WARNINGS_FILL_LAYER_ID,
  WEATHER_WARNINGS_LINE_LAYER_ID,
  WEATHER_WARNINGS_URL,
  WEATHER_WARNINGS_REFRESH_MS,
} from './constants.js'

// initMap creates a MapLibre instance on the given container element, wires
// all ASD layers and WebSocket, and returns a { destroy } handle.
//
// Parameters:
//   container    — DOM element to mount the map into
//   store        — Pinia ASD store (setFeedStatus, setMapLoaded, palette,
//                  flFilter, layerVisibility, labelPins)
//   onTrackClick — callback(track) fired when the user clicks a track symbol
//   onConnectionChange — optional callback(state) fired on WebSocket lifecycle:
//                  'open' when the /ws stream connects, 'closed' when it drops.
//                  The ASD uses it to slide the session on connect and to probe
//                  the session on a drop (auth loss → login overlay, WF2-12.5).
//   initialCenter — optional {lat, lon, zoom} from the tenant's effective view
//                  (session.viewCenter, whoami). When present the map opens on
//                  the tenant's own sector; when null it uses the global
//                  /api/map-config centre (FR-UI-013). Later changes flow through
//                  applyViewCenter (e.g. an admin switching impersonation target).
export async function initMap(container, store, onTrackClick, onConnectionChange, initialCenter = null) {
  // Fetch map config from the backend.
  const res = await fetch('/api/map-config')
  const cfg = await res.json()

  // Select the foreground palette to match the base-map theme (dark by
  // default). An unknown theme falls back to the dark palette.
  const palette = PALETTES[cfg.theme] || PALETTES.dark
  store.setPalette(cfg.theme || 'dark')

  // #114: the coverage-ring layer only ever has data when coverage sensors are
  // configured server-side; expose that so the sidebar can disable the toggle
  // (a switch that visibly does nothing reads as a bug).
  store.setCoverageAvailable((cfg.coverage_sensor_count ?? 0) > 0)

  // WX-A: only offer the DWD radar toggle when the backend has a WMS source
  // configured — a switch that visibly does nothing reads as a bug.
  store.setWeatherRadarAvailable(cfg.weather_radar_available === true)
  // WX-C: same for the DWD warnings overlay.
  store.setWeatherWarningsAvailable(cfg.weather_warnings_available === true)

  // Effective viewport: the tenant's view centre (whoami) when supplied, else the
  // global map-config env. recenter() and the range rings follow this, so the
  // "recentre" button and the ring geometry track the tenant's sector too — not
  // the global default (FR-UI-013). center_lat === 0 is a valid latitude, so
  // presence is tested with != null, not truthiness.
  const effectiveCenter = {
    lat: initialCenter?.lat != null ? initialCenter.lat : cfg.center_lat,
    lon: initialCenter?.lon != null ? initialCenter.lon : cfg.center_lon,
    zoom: initialCenter?.zoom != null ? initialCenter.zoom : cfg.zoom,
  }

  const map = new maplibregl.Map({
    container,
    style: cfg.style,
    center: [effectiveCenter.lon, effectiveCenter.lat],
    zoom: effectiveCenter.zoom,
    // Suppress the default expanded attribution: it printed "© OpenStreetMap …"
    // bottom-right, right under our distance/vector readout. We add a compact
    // attribution below (collapses to an ⓘ, expands on click) — the credit stays
    // (OSM/CARTO terms) but no longer overlaps the readout.
    attributionControl: false,
  })
  map.addControl(new maplibregl.AttributionControl({ compact: true }), 'bottom-right')

  // Native MapLibre compass control. It shows the current bearing and resets to
  // north on click (replacing the old hand-rolled reset-north button). Zoom lives
  // on the navigation rail; showZoom is off here to avoid duplicate buttons. The
  // absolute distance reference is the bottom-right "<width> NM Breite" readout
  // (reportViewportWidth below), which replaced the native scale bar (design).
  map.addControl(
    new maplibregl.NavigationControl({ showZoom: false, showCompass: true, visualizePitch: false }),
    'top-left',
  )

  // Engine-local runtime state — mirrors the original app.js `state`.
  // All mutable ASD data lives here so modules receive it as a parameter.
  const state = {
    mapLoaded: false,
    pendingTracks: null,
    // Per-track history of past positions ([lon, lat]), for trail and dot display.
    trackHistory: new Map(),
    // Per-track last-known flight level in feet, for the vertical-tendency
    // indicator (ASD-001b).
    trackFlHistory: new Map(),
    // ASD-004b: per-track coasting flag, needed by trail/dot features for the
    // dimming paint expression (trackHistory doesn't carry track-level metadata).
    trackCoasting: new Map(),
    // ASD-004c: tracks fading out after TSE; Map<track_num, {deadline, track}>.
    fadingTracks: new Map(),
    // setInterval handle for the fade-out animation loop (null when idle).
    fadeInterval: null,
    // Precomputed GeoJSON features for the current live-track frame.
    liveTrackFeatures: [],
    liveVectorFeatures: [],
    // ASD-002 B2: per-track manual label-position overrides.
    labelPins: new Map(),
  }

  // Helper: build a bound renderSources call with the current store slices.
  const doRender = () => {
    if (!state.mapLoaded) return
    // Pass the selected track number so renderSources keeps the selection halo
    // (ASD-007) pinned to the moving symbol; undefined clears the ring.
    renderSources(map, state, store.flFilter, state.labelPins, palette, store.selectedTrack?.track_num)
  }

  // Report the visible scope width in NM for the bottom-right "<width> NM Breite"
  // readout (replaces the native scale bar, per the design). Measured across the
  // viewport at the centre latitude; pushed to the store, throttled to one update
  // per animation frame via the existing move handler below.
  const reportViewportWidth = () => {
    const b = map.getBounds()
    const lat = map.getCenter().lat
    const widthNM = haversineNM({ lat, lng: b.getWest() }, { lat, lng: b.getEast() })
    store.setViewportWidth(Math.round(widthNM))
  }

  // Fade-loop management: start interval if not already running.
  const startFadeLoop = () => {
    if (state.fadeInterval) return
    state.fadeInterval = setInterval(() => {
      const shouldContinue = tickFade(state, doRender)
      if (!shouldContinue) {
        clearInterval(state.fadeInterval)
        state.fadeInterval = null
      }
    }, 50)
  }

  // WebSocket connection with auto-reconnect.
  let ws = null
  let reconnectTimer = null
  let reconnectDelay = 2000

  function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsURL = `${protocol}//${window.location.host}/ws`

    console.log('Connecting to', wsURL)
    const socket = new WebSocket(wsURL)
    ws = socket

    socket.addEventListener('open', () => {
      console.log('WebSocket connected')
      // A (re)connect may carry a different feed scope (e.g. impersonation,
      // ADR 0008) — drop stale per-feed health so the chip reflects only this
      // connection's feeds. Fresh statuses arrive with the next heartbeats.
      store.resetFeedHealth()
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
      onConnectionChange?.('open')
    })

    socket.addEventListener('message', (event) => {
      try {
        const msg = JSON.parse(event.data)
        // Feed-health updates (CAT065 heartbeat) are separate from the track
        // stream; route them to the store and never through the track layer,
        // so a heartbeat message doesn't clear the air picture. The wire field
        // is the per-feed `color` (green/yellow/red, pkg/broadcast); the store
        // maps it to chip states and aggregates across feeds (#117).
        if (msg.feed_status) {
          store.setFeedHealth(msg.feed_status.feed_id, msg.feed_status.color)
          return
        }
        if (state.mapLoaded) {
          updateTracksLayer(msg, state, doRender, startFadeLoop)
        } else {
          state.pendingTracks = msg
        }
      } catch (err) {
        console.error('Failed to parse message:', err, event.data)
      }
    })

    socket.addEventListener('close', () => {
      // Ignore the close of a socket we have already superseded — an explicit
      // reconnect() (e.g. impersonation start/exit, ADR 0008) or a newer
      // connection. Only the current socket drives the auto-reconnect timer.
      if (ws !== socket) return
      console.warn('WebSocket disconnected, reconnecting in', reconnectDelay, 'ms')
      ws = null
      onConnectionChange?.('closed')
      reconnectTimer = setTimeout(connectWebSocket, reconnectDelay)
    })

    socket.addEventListener('error', (err) => {
      console.error('WebSocket error:', err)
    })
  }

  // reconnect tears down the current socket and opens a fresh one immediately, so
  // a changed impersonation grant cookie (ADR 0008) takes effect now instead of on
  // the next natural reconnect. Detaching ws before closing makes the old socket's
  // close handler no-op (it sees ws !== its own socket), avoiding a double connect.
  function reconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    const old = ws
    ws = null
    if (old && old.readyState <= WebSocket.OPEN) old.close()
    connectWebSocket()
  }

  // Wire everything once the MapLibre style is fully loaded.
  map.on('load', () => {
    // WX-A: DWD weather-radar overlay first of all, so it sits directly above the
    // base map and beneath every operational overlay. Starts hidden; toggled via
    // the sidebar (gated by the weather_radar entitlement + availability).
    addWeatherRadarLayer(map)
    // WX-C: DWD weather-warnings polygons above the radar raster but below the
    // aeronautical/track layers. Starts hidden; toggled via the sidebar
    // (weather_warnings entitlement + availability).
    addWeatherWarningsLayer(map)
    // Aeronautical overlays next, so they sit beneath the track layers.
    addAeronauticalIcons(map)
    addAirspaceLayers(map, palette)
    addNavaidLayers(map, palette)
    addWaypointLayers(map, palette)
    // Coverage rings sit above aeronautical overlays but below track layers,
    // so they provide geographic context without obscuring the air picture.
    addCoverageLayer(map)
    // Fetch rings once; the data is static (derived from operator config).
    fetch('/api/coverage/rings')
      .then((r) => r.json())
      .then((geojson) => updateCoverageSource(map, geojson))
      .catch((err) => console.warn('coverage rings fetch failed:', err))
    // ASD-012: range-ring overlay beneath the track layers. Geometry + visibility
    // are driven by the reactive store (default off; operator opts in).
    addRangeRingsLayer(map, palette)
    updateRangeRings(store.rangeRingConfig.spacingNM, store.rangeRingConfig.count)
    // Track layers from bottom to top: trail line → history dots → speed
    // vectors → leader lines → track symbols → deconflicted labels (ASD-002).
    addTrailsLayer(map, palette)
    addHistoryDotsLayer(map, palette)
    addVectorsLayer(map, palette)
    addLeaderLinesLayer(map, palette) // ASD-002: under track circles
    addSelectionLayer(map, palette)   // ASD-007: selection halo, under symbols
    addTracksLayer(map)
    addLabelsLayer(map, palette)      // ASD-002: above track circles
    state.mapLoaded = true
    store.setMapLoaded(true)
    reportViewportWidth() // initial bottom-right "NM Breite" readout

    if (state.pendingTracks) {
      updateTracksLayer(state.pendingTracks, state, doRender, startFadeLoop)
      state.pendingTracks = null
    }

    // Load aeronautical data and start periodic refresh.
    const aeroInterval = startAeronauticalRefresh(map)

    // WX-C: load DWD warnings and refresh on the warn cadence. Best-effort — a
    // failed/absent fetch simply leaves the (empty) overlay unchanged.
    const loadWarnings = () => {
      fetch(WEATHER_WARNINGS_URL)
        .then((r) => (r.ok ? r.json() : null))
        .then((geojson) => { if (geojson) updateWeatherWarnings(map, geojson) })
        .catch((err) => console.warn('weather warnings fetch failed:', err))
    }
    loadWarnings()
    const warnInterval = setInterval(loadWarnings, WEATHER_WARNINGS_REFRESH_MS)
    map._warnInterval = warnInterval

    // ASD-002 B2: Drag&Drop label pinning.
    setupLabelDrag(map, state, doRender)

    // ASD-002: re-deconflict label positions whenever the viewport moves, so
    // labels correctly track their symbols during pan and zoom. Throttled to
    // one deconfliction per animation frame to avoid redundant work.
    let deconflictFrame = null
    map.on('move', () => {
      if (deconflictFrame) return
      deconflictFrame = requestAnimationFrame(() => {
        deconflictFrame = null
        if (state.mapLoaded) doRender()
        reportViewportWidth() // keep the bottom-right "NM Breite" readout live
      })
    })

    // Track click → emit to Vue component.
    map.on('click', TRACKS_LAYER_ID, (e) => {
      const features = map.queryRenderedFeatures(e.point, { layers: [TRACKS_LAYER_ID] })
      if (!features || features.length === 0) return
      const props = features[0].properties
      // Find the full track data from liveTrackFeatures.
      const liveFeature = state.liveTrackFeatures.find(
        (f) => f.properties.track_num === props.track_num,
      )
      if (liveFeature && onTrackClick) {
        onTrackClick(liveFeature.properties)
      }
    })

    // Store cleanup ref for aeroInterval.
    map._aeroInterval = aeroInterval
  })

  connectWebSocket()

  // Layer visibility control: called by MapCanvas when store changes.
  function setLayerVisibility(vis) {
    if (!state.mapLoaded) return
    const groups = {
      airspace: [AIRSPACE_FILL_LAYER_ID, AIRSPACE_LINE_LAYER_ID, AIRSPACE_LABEL_LAYER_ID],
      navaids: [NAVAIDS_LAYER_ID],
      waypoints: [WAYPOINTS_LAYER_ID],
      coverageRings: [COVERAGE_RINGS_LAYER_ID, COVERAGE_RINGS_LAYER_ID + '-inner', COVERAGE_CENTER_LAYER_ID],
      rangeRings: [RANGE_RINGS_LAYER_ID, RANGE_RINGS_LABEL_LAYER_ID],
      historyDots: [HISTORY_DOTS_LAYER_ID],
      weatherRadar: [WEATHER_RADAR_LAYER_ID],
      weatherWarnings: [WEATHER_WARNINGS_FILL_LAYER_ID, WEATHER_WARNINGS_LINE_LAYER_ID],
    }
    for (const [key, layerIds] of Object.entries(groups)) {
      if (key in vis) {
        const visibility = vis[key] ? 'visible' : 'none'
        layerIds.forEach((id) => {
          if (map.getLayer(id)) {
            map.setLayoutProperty(id, 'visibility', visibility)
          }
        })
      }
    }
  }

  // FL filter update: re-render immediately without waiting for a WS update.
  function updateFlFilter() {
    doRender()
  }

  // ASD-007: selection changed (track picked/cleared in the UI) — re-render so
  // the selection halo appears/moves/clears without waiting for a WS update.
  function updateSelection() {
    doRender()
  }

  // ASD-011: update MapLibre filters on the airspace layers to reflect the
  // current airspaceGroupVisibility state. Called by MapCanvas whenever the
  // store changes (or after map load to apply the initial state).
  function updateAirspaceFilter() {
    if (!state.mapLoaded) return
    const vis = store.airspaceGroupVisibility

    const visibleTypes = []
    for (const g of AIRSPACE_GROUPS) {
      if (vis[g.id]) visibleTypes.push(...g.types)
    }

    const typeFilter = visibleTypes.length > 0
      ? ['in', ['get', 'type'], ['literal', visibleTypes]]
      : ['boolean', false]

    const polygonAndType = ['all', ['==', ['geometry-type'], 'Polygon'], typeFilter]

    if (map.getLayer(AIRSPACE_FILL_LAYER_ID))  map.setFilter(AIRSPACE_FILL_LAYER_ID, polygonAndType)
    if (map.getLayer(AIRSPACE_LINE_LAYER_ID))  map.setFilter(AIRSPACE_LINE_LAYER_ID, typeFilter)
    if (map.getLayer(AIRSPACE_LABEL_LAYER_ID)) map.setFilter(AIRSPACE_LABEL_LAYER_ID, typeFilter)
  }

  // Destroy: close WS, clear intervals, remove map.
  function destroy() {
    if (reconnectTimer) clearTimeout(reconnectTimer)
    if (ws) ws.close()
    if (state.fadeInterval) clearInterval(state.fadeInterval)
    if (map._aeroInterval) clearInterval(map._aeroInterval)
    if (map._warnInterval) clearInterval(map._warnInterval)
    map.remove()
  }

  // ASD-009: Map control helpers exposed to the Vue chrome layer.
  // They are intentionally thin wrappers — the map object owns the state,
  // and the chrome layer never needs to reach into it directly.
  function zoomIn()    { map.zoomIn() }
  function zoomOut()   { map.zoomOut() }
  function recenter()  { map.flyTo({ center: [effectiveCenter.lon, effectiveCenter.lat], zoom: effectiveCenter.zoom }) }

  // applyViewCenter aims the camera at the tenant's effective view centre
  // (session.viewCenter, whoami), keeping recenter()/range-rings in sync. Passing
  // null resets to the global map-config env centre. A no-op when the centre is
  // unchanged, so periodic session refreshes never yank the camera; a genuine
  // change (e.g. an admin switching the impersonation target) jumps to it.
  function applyViewCenter(vc) {
    const next = vc && vc.lat != null && vc.lon != null
      ? { lat: vc.lat, lon: vc.lon, zoom: vc.zoom != null ? vc.zoom : cfg.zoom }
      : { lat: cfg.center_lat, lon: cfg.center_lon, zoom: cfg.zoom }
    if (next.lat === effectiveCenter.lat && next.lon === effectiveCenter.lon && next.zoom === effectiveCenter.zoom) {
      return
    }
    effectiveCenter.lat = next.lat
    effectiveCenter.lon = next.lon
    effectiveCenter.zoom = next.zoom
    map.jumpTo({ center: [next.lon, next.lat], zoom: next.zoom })
  }

  // ASD-012: (re)generate the range-ring geometry from the configured centre and
  // the operator's spacing/count, then push it to the source. Called on load and
  // whenever the reactive store config changes (MapCanvas watcher).
  function updateRangeRings(spacingNM, count) {
    const src = map.getSource(RANGE_RINGS_SOURCE_ID)
    if (!src) return
    src.setData(rangeRingsGeoJSON(effectiveCenter.lat, effectiveCenter.lon, spacingNM, count))
  }

  return { map, destroy, reconnect, setLayerVisibility, updateFlFilter, updateAirspaceFilter, updateSelection, zoomIn, zoomOut, recenter, applyViewCenter, updateRangeRings }
}
