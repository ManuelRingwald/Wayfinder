// Map engine: initialises MapLibre, wires all layers, manages the WebSocket
// connection and the ASD rendering loop.
//
// The engine owns a local `state` object that mirrors the original app.js
// `state` — all mutable ASD runtime state lives here. The Pinia store is used
// only for UI-facing state (feedStatus, mapLoaded, palette key).
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { PALETTES, TRACKS_LAYER_ID } from './constants.js'
import {
  addAeronauticalIcons,
  addAirspaceLayers,
  addNavaidLayers,
  addWaypointLayers,
  addTracksLayer,
  addLeaderLinesLayer,
  addLabelsLayer,
  addTrailsLayer,
  addHistoryDotsLayer,
  addVectorsLayer,
} from './layers.js'
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
} from './constants.js'

// initMap creates a MapLibre instance on the given container element, wires
// all ASD layers and WebSocket, and returns a { destroy } handle.
//
// Parameters:
//   container    — DOM element to mount the map into
//   store        — Pinia ASD store (setFeedStatus, setMapLoaded, palette,
//                  flFilter, layerVisibility, labelPins)
//   onTrackClick — callback(track) fired when the user clicks a track symbol
export async function initMap(container, store, onTrackClick) {
  // Fetch map config from the backend.
  const res = await fetch('/api/map-config')
  const cfg = await res.json()

  // Select the foreground palette to match the base-map theme (dark by
  // default). An unknown theme falls back to the dark palette.
  const palette = PALETTES[cfg.theme] || PALETTES.dark
  store.setPalette(cfg.theme || 'dark')

  const map = new maplibregl.Map({
    container,
    style: cfg.style,
    center: [cfg.center_lon, cfg.center_lat],
    zoom: cfg.zoom,
  })

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
    renderSources(map, state, store.flFilter, state.labelPins, palette, store.hiddenCategories)
  }

  // ASD-010: derive per-category track counts from live features and push to
  // the store so TrackFilterChips can display them reactively.
  function updateTrackCounts() {
    let confirmed = 0, coasting = 0, tentative = 0
    for (const f of state.liveTrackFeatures) {
      const p = f.properties
      if (p.coasting) coasting++
      else if (p.confirmed) confirmed++
      else tentative++
    }
    store.setTrackCounts({ confirmed, coasting, tentative })
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
    ws = new WebSocket(wsURL)

    ws.addEventListener('open', () => {
      console.log('WebSocket connected')
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
    })

    ws.addEventListener('message', (event) => {
      try {
        const msg = JSON.parse(event.data)
        // Feed-health updates (CAT065 heartbeat) are separate from the track
        // stream; route them to the store and never through the track layer,
        // so a heartbeat message doesn't clear the air picture.
        if (msg.feed_status) {
          store.setFeedStatus(msg.feed_status.state)
          return
        }
        if (state.mapLoaded) {
          updateTracksLayer(msg, state, doRender, startFadeLoop)
          updateTrackCounts()
        } else {
          state.pendingTracks = msg
        }
      } catch (err) {
        console.error('Failed to parse message:', err, event.data)
      }
    })

    ws.addEventListener('close', () => {
      console.warn('WebSocket disconnected, reconnecting in', reconnectDelay, 'ms')
      ws = null
      reconnectTimer = setTimeout(connectWebSocket, reconnectDelay)
    })

    ws.addEventListener('error', (err) => {
      console.error('WebSocket error:', err)
    })
  }

  // Wire everything once the MapLibre style is fully loaded.
  map.on('load', () => {
    // Aeronautical overlays first, so they sit beneath the track layers.
    addAeronauticalIcons(map)
    addAirspaceLayers(map, palette)
    addNavaidLayers(map, palette)
    addWaypointLayers(map, palette)
    // Track layers from bottom to top: trail line → history dots → speed
    // vectors → leader lines → track symbols → deconflicted labels (ASD-002).
    addTrailsLayer(map, palette)
    addHistoryDotsLayer(map, palette)
    addVectorsLayer(map, palette)
    addLeaderLinesLayer(map, palette) // ASD-002: under track circles
    addTracksLayer(map, palette)
    addLabelsLayer(map, palette)      // ASD-002: above track circles
    state.mapLoaded = true
    store.setMapLoaded(true)

    if (state.pendingTracks) {
      updateTracksLayer(state.pendingTracks, state, doRender, startFadeLoop)
      state.pendingTracks = null
    }

    // Load aeronautical data and start periodic refresh.
    const aeroInterval = startAeronauticalRefresh(map)

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

  // Destroy: close WS, clear intervals, remove map.
  function destroy() {
    if (reconnectTimer) clearTimeout(reconnectTimer)
    if (ws) ws.close()
    if (state.fadeInterval) clearInterval(state.fadeInterval)
    if (map._aeroInterval) clearInterval(map._aeroInterval)
    map.remove()
  }

  // ASD-009: Map control helpers exposed to the Vue chrome layer.
  // They are intentionally thin wrappers — the map object owns the state,
  // and the chrome layer never needs to reach into it directly.
  function zoomIn()    { map.zoomIn() }
  function zoomOut()   { map.zoomOut() }
  function resetNorth() { map.easeTo({ bearing: 0, pitch: 0 }) }
  function recenter()  { map.flyTo({ center: [cfg.center_lon, cfg.center_lat], zoom: cfg.zoom }) }

  return { map, destroy, setLayerVisibility, updateFlFilter, zoomIn, zoomOut, resetNorth, recenter }
}
