// Map engine: initialises MapLibre, wires all layers, manages the WebSocket
// connection and the ASD rendering loop.
//
// The engine owns a local `state` object that mirrors the original app.js
// `state` — all mutable ASD runtime state lives here. The Pinia store is used
// only for UI-facing state (feedStatus, mapLoaded, palette key).
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import {
  PALETTES, TRACKS_LAYER_ID, LABELS_LAYER_ID, AIRSPACE_GROUPS,
  SYNTHETIC_BACKGROUND_LAYER_ID, SYNTHETIC_BACKGROUND_COLOR, SYNTHETIC_SCOPE_STYLE,
  SEARCH_RESULT_ZOOM,
} from './constants.js'
import { bucketBasemapLayers } from './basemapGroups.js'
import {
  addAeronauticalIcons,
  addAirspaceLayers,
  addAirspaceAoRLayer,
  aorFilter,
  addNavaidLayers,
  addWaypointLayers,
  addAirportLayers,
  updateAirportSource,
  addRunwayLayers,
  updateRunwaySource,
  addTracksLayer,
  addSpiHighlightLayer,
  addLeaderLinesLayer,
  addSelectionLayer,
  addLabelsLayer,
  addSelectionLabelLayer,
  addTrailsLayer,
  addHistoryDotsLayer,
  addVectorsLayer,
  addCoverageLayer,
  updateCoverageSource,
  addRangeRingsLayer,
  addWeatherRadarLayer,
  setWeatherRadarAOI,
  addBasemapMaskLayer,
  setBasemapMaskAOI,
  addWeatherWarningsLayer,
  updateWeatherWarnings,
  addSearchMarkerLayer,
  updateSearchMarker,
} from './layers.js'
import { clipFeatureCollectionToBBox } from './clip.js'
import { rangeRingsGeoJSON } from './rangerings.js'
import { updateTracksLayer } from './tracks.js'
import { feedStatusEvent, connectionEvent, trackLifecycleEvents } from './events.js'
import { useEventsStore } from '@/stores/events.js'
import { renderSources, tickFade } from './render.js'
import { setupLabelDrag } from './drag.js'
import { startAeronauticalRefresh } from './aeronautical.js'
import {
  AIRSPACE_FILL_LAYER_ID,
  AIRSPACE_LINE_LAYER_ID,
  AIRSPACE_LABEL_LAYER_ID,
  AIRSPACE_AOR_LAYER_ID,
  NAVAIDS_LAYER_ID,
  WAYPOINTS_LAYER_ID,
  COVERAGE_RINGS_LAYER_ID,
  COVERAGE_CENTER_LAYER_ID,
  RANGE_RINGS_SOURCE_ID,
  RANGE_RINGS_LAYER_ID,
  RANGE_RINGS_LABEL_LAYER_ID,
  HISTORY_DOTS_LAYER_ID,
  AIRPORT_LAYER_ID,
  AIRPORT_LABEL_LAYER_ID,
  AIRPORT_URL,
  RUNWAY_LAYER_ID,
  RUNWAY_URL,
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
//   onTracks     — optional callback(liveTrackFeatures) fired after every track
//                  batch (and the pending flush on load). The measure controller
//                  (#297) uses it to re-anchor track-referenced endpoints to the
//                  moving track; it is decoupled from the engine like onTrackClick.
export async function initMap(container, store, onTrackClick, onConnectionChange, initialCenter = null, initialAOI = null, onEmptyClick = null, onTracks = null) {
  // #189/#190: the tenant's AOI (whoami) clips the DWD weather overlays to the
  // sector. Held in a closure so loadWarnings and applyWeatherAOI can re-clip.
  let weatherAOI = initialAOI
  // Last raw (unclipped) warnings FeatureCollection, kept so an AOI change can
  // re-clip without re-fetching.
  let lastWarningsRaw = null
  // ASD-014: the tenant's Area-of-Responsibility airspace ids (whoami), held in a
  // closure so updateAoR can re-apply the highlight filter and the load handler
  // can initialise it on every mount.
  let aorIds = []
  // Fetch map config from the backend.
  const res = await fetch('/api/map-config')
  const cfg = await res.json()

  // Select the foreground palette to match the base-map theme (the bkg-dark
  // scope by default). An unknown theme falls back to the dark palette.
  const palette = PALETTES[cfg.theme] || PALETTES['bkg-dark']
  store.setPalette(PALETTES[cfg.theme] ? cfg.theme : 'bkg-dark')

  // #114: the coverage-ring layer only ever has data when coverage sensors are
  // configured server-side; expose that so the sidebar can disable the toggle
  // (a switch that visibly does nothing reads as a bug).
  store.setCoverageAvailable((cfg.coverage_sensor_count ?? 0) > 0)

  // WX-A: only offer the DWD radar toggle when the backend has a WMS source
  // configured — a switch that visibly does nothing reads as a bug.
  store.setWeatherRadarAvailable(cfg.weather_radar_available === true)
  // WX-C: same for the DWD warnings overlay.
  store.setWeatherWarningsAvailable(cfg.weather_warnings_available === true)

  // #245 Teil B: only offer the manual-correlation controls when the backend has
  // a Firefly command token configured — otherwise every command would 503.
  store.setCorrelationAvailable(cfg.correlation_available === true)

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

  // #274 (W1=b): resolve a URL-style ourselves so a failing base-map upstream
  // degrades to the synthetic scope instead of a dead map — the ASD works
  // without the map layer by design, so a BKG outage must never cost the air
  // picture (track labels keep rendering: the fallback keeps the local glyphs).
  let mapStyle = cfg.style
  if (typeof mapStyle === 'string') {
    try {
      const styleRes = await fetch(mapStyle)
      if (!styleRes.ok) throw new Error(`HTTP ${styleRes.status}`)
      mapStyle = await styleRes.json()
    } catch (err) {
      console.warn('Base-map style unavailable — starting the synthetic scope:', err)
      mapStyle = SYNTHETIC_SCOPE_STYLE
    }
  }

  const map = new maplibregl.Map({
    container,
    style: mapStyle,
    center: [effectiveCenter.lon, effectiveCenter.lat],
    zoom: effectiveCenter.zoom,
    // Track-label flicker fix: MapLibre's symbol machinery is built for
    // cartographic labels and FADES new symbols in (default 300 ms). Our track
    // data blocks are telemetry — every WS batch replaces the label source, and
    // a label whose text changed (new FL) counts as a NEW symbol, so it blanked
    // for the fade window on every update. The label layer already opts out of
    // collision placement (text-allow-overlap/-ignore-placement, ASD-002);
    // fadeDuration: 0 completes that opt-out on the time axis: symbol opacity
    // snaps instead of animating, so the swapped labels stand in the same frame.
    // Global option (no per-layer lever in the style spec) — base-map labels
    // pop instead of fading too, which suits a radar scope.
    fadeDuration: 0,
    // Suppress the default expanded attribution: it printed "© OpenStreetMap …"
    // bottom-right, right under our distance/vector readout. We add a compact
    // attribution below (collapses to an ⓘ, expands on click) — the credit stays
    // (basemap.de/BKG terms) but no longer overlaps the readout.
    attributionControl: false,
  })
  map.addControl(new maplibregl.AttributionControl({ compact: true }), 'bottom-right')

  // Native MapLibre compass control. It shows the current bearing and resets to
  // north on click (replacing the old hand-rolled reset-north button). Zoom lives
  // in the bottom-right map controls (MapControls, ASD-019); showZoom is off here
  // to avoid duplicate buttons. The absolute distance reference is the range-ring
  // overlay (constant-ground-distance circles around the display centre); there is
  // no scale bar.
  map.addControl(
    new maplibregl.NavigationControl({ showZoom: false, showCompass: true, visualizePitch: false }),
    'top-left',
  )

  // Engine-local runtime state — mirrors the original app.js `state`.
  // All mutable ASD data lives here so modules receive it as a parameter.
  const state = {
    mapLoaded: false,
    pendingTracks: null,
    // #274: layer ids of the (toggleable) base map, snapshotted at style load.
    basemapLayerIds: [],
    // E0 (#291): the base-map layers bucketed by element group ({group: id[]}),
    // so the sidebar can later toggle "only rivers"/"only roads" (E2/#293). The
    // union of all buckets equals basemapLayerIds; the #274 master keeps using
    // that flat set. Populated at style load; empty until then.
    basemapGroups: {},
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
    // ASD-013: previous live track-number set, for deriving appeared/disappeared
    // events. `trackEventsPrimed` gates the very first frame after a (re)connect
    // so the initial air picture does not flood the log with "appeared".
    prevTrackNums: new Set(),
    trackEventsPrimed: false,
  }

  // ASD-013: the operator event log (Alarm-/Ereignis-Panel). Fed from the WS
  // stream below; the log is tenant-scoped because the stream already is.
  const events = useEventsStore()
  // Previous WebSocket connection status, for connection-lost/restored events.
  let prevConnStatus = null

  // recordTrackEvents derives track appeared/disappeared events (ASD-013) from a
  // raw WS track batch and pushes them to the event log. The first frame after a
  // (re)connect only primes the baseline (no "appeared" flood), but still logs
  // any genuine TSE-ended tracks it carries.
  function recordTrackEvents(msg) {
    const batch = msg.tracks || []
    const liveNums = batch.filter((t) => !t.ended).map((t) => t.track_num)
    const endedNums = batch.filter((t) => t.ended).map((t) => t.track_num)
    if (!state.trackEventsPrimed) {
      state.prevTrackNums = new Set(liveNums)
      state.trackEventsPrimed = true
      events.addMany(trackLifecycleEvents(state.prevTrackNums, [], endedNums))
      return
    }
    events.addMany(trackLifecycleEvents(state.prevTrackNums, liveNums, endedNums))
    state.prevTrackNums = new Set(liveNums)
  }

  // ASD-013: mirror the currently-displayed track numbers into the store so the
  // Ereignis-Panel can tell which "Track N erschienen" events still refer to a
  // selectable track (operator request 2026-07-08). Sourced from
  // liveTrackFeatures (live + coasting), which is the authoritative displayed
  // set — not the raw batch, which may omit coasting tracks.
  const syncLiveTrackNums = () => {
    store.setLiveTrackNums(state.liveTrackFeatures.map((f) => f.properties.track_num))
  }

  // Helper: build a bound renderSources call with the current store slices.
  const doRender = () => {
    if (!state.mapLoaded) return
    // Pass the selected track number so renderSources keeps the selection halo
    // (ASD-007) pinned to the moving symbol; undefined clears the ring.
    // #191: pass the configured history retention (ms) so the dot age fade uses
    // the operator's chosen window.
    renderSources(
      map, state, store.flFilter, state.labelPins, palette,
      store.selectedTrack?.track_num,
      store.historyConfig.durationS * 1000,
    )
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
      // ASD-013: a (re)connect re-scopes the stream and rebuilds the air picture
      // server-side; re-prime the track baseline so the fresh picture does not
      // flood the log with "appeared". Log the recovery if we were disconnected.
      state.trackEventsPrimed = false
      const connEvt = connectionEvent(prevConnStatus, 'open')
      if (connEvt) events.add(connEvt)
      prevConnStatus = 'open'
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
          // degraded_reason (CAT063 I063/RE SRC-REASON, Firefly ADR 0033) is
          // present only on a degraded feed with a known cause; the chip shows it.
          // ASD-013: log a change in the aggregate feed health as an event
          // (compare the worst-across-feeds status before and after the update).
          const prevFeed = store.feedStatus
          store.setFeedHealth(msg.feed_status.feed_id, msg.feed_status.color, msg.feed_status.degraded_reason, msg.feed_status.sensors)
          const feedEvt = feedStatusEvent(prevFeed, store.feedStatus)
          if (feedEvt) events.add(feedEvt)
          return
        }
        // ASD-013: derive track lifecycle events from every batch at receive time
        // (independent of map-load state, so none are lost while the style loads).
        recordTrackEvents(msg)
        if (state.mapLoaded) {
          updateTracksLayer(msg, state, doRender, startFadeLoop, store.historyConfig.durationS * 1000)
          syncLiveTrackNums()
          // #272: keep the open detail panel live — refresh the selected
          // track's snapshot from the just-updated displayed set.
          store.refreshSelectedTrack(state.liveTrackFeatures)
          // #297: feed the same displayed set to the measure controller so a
          // track-anchored measure endpoint follows the moving track.
          onTracks?.(state.liveTrackFeatures)
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
      // ASD-013: log the drop (once per transition).
      const connEvt = connectionEvent(prevConnStatus, 'closed')
      if (connEvt) events.add(connEvt)
      prevConnStatus = 'closed'
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
    // #274: snapshot the BASE style's layer ids before any overlay is added —
    // that set IS the "Basiskarte" the sidebar toggle shows/hides. Then lay an
    // always-visible near-black floor underneath (the base style's own
    // background hides with the rest, and a transparent canvas is not a scope).
    // Default per W2 is OFF: the map starts hidden unless the store (a view
    // profile, or the user's earlier toggle) says otherwise.
    const baseStyleLayers = map.getStyle().layers
    state.basemapLayerIds = baseStyleLayers
      .map((l) => l.id)
      .filter((id) => id !== SYNTHETIC_BACKGROUND_LAYER_ID)
    // E0 (#291): bucket the same snapshotted layers by element group. Uses the
    // full layer objects (source-layer/type) for schema-agnostic classification;
    // the synthetic floor is excluded (it is not part of the official base map).
    state.basemapGroups = bucketBasemapLayers(baseStyleLayers, SYNTHETIC_BACKGROUND_LAYER_ID)
    if (!map.getLayer(SYNTHETIC_BACKGROUND_LAYER_ID)) {
      map.addLayer(
        {
          id: SYNTHETIC_BACKGROUND_LAYER_ID,
          type: 'background',
          paint: { 'background-color': SYNTHETIC_BACKGROUND_COLOR },
        },
        state.basemapLayerIds[0],
      )
    }
    // #274 + E2 (#293): apply the base map respecting BOTH the master (is the
    // map shown at all) AND the per-element switches (which parts show when it
    // is). Default per W2 is master OFF, so this hides everything unless the
    // store (a view profile, or an earlier toggle) says otherwise.
    applyBasemap()

    // #289: the base-map AOI mask — covers the map OUTSIDE the tenant AOI with the
    // scope backdrop colour, so the official BKG base map is limited to the
    // sector. Added here (directly above the base map, before every operational
    // overlay below) so it hides only the map, never the tracks/weather/
    // aeronautical layers. Empty when no AOI is configured (full map, no clip).
    addBasemapMaskLayer(map, weatherAOI)

    // WX-A: DWD weather-radar overlay first of all, so it sits directly above the
    // base map and beneath every operational overlay. Starts hidden; toggled via
    // the sidebar (gated by the weather_radar entitlement + availability).
    addWeatherRadarLayer(map, weatherAOI)
    // WX-C: DWD weather-warnings polygons above the radar raster but below the
    // aeronautical/track layers. Starts hidden; toggled via the sidebar
    // (weather_warnings entitlement + availability).
    addWeatherWarningsLayer(map)
    // Aeronautical overlays next, so they sit beneath the track layers.
    addAeronauticalIcons(map)
    addAirspaceLayers(map, palette)
    // ASD-014: the AoR highlight sits directly above the airspace line so the
    // tenant's controlled volumes stand out from the context airspace.
    addAirspaceAoRLayer(map)
    addNavaidLayers(map, palette)
    addWaypointLayers(map, palette)
    // #192: airport reference-point markers (offline OurAirports, AOI-scoped by
    // the backend). Fetched once; the data is static context. Best-effort — a
    // failed/empty fetch leaves the overlay empty.
    addAirportLayers(map, palette)
    fetch(AIRPORT_URL)
      .then((r) => (r.ok ? r.json() : null))
      .then((geojson) => { if (geojson) updateAirportSource(map, geojson) })
      .catch((err) => console.warn('airports fetch failed:', err))
    // #192: runway centrelines (offline OurAirports, AOI-scoped by the backend).
    addRunwayLayers(map, palette)
    fetch(RUNWAY_URL)
      .then((r) => (r.ok ? r.json() : null))
      .then((geojson) => { if (geojson) updateRunwaySource(map, geojson) })
      .catch((err) => console.warn('runways fetch failed:', err))
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
    addSpiHighlightLayer(map)         // #236: SPI ident ring, framing the symbol
    addLabelsLayer(map, palette)      // ASD-002: above track circles
    addSelectionLabelLayer(map)       // ASD-011b: selected-label outline, above labels
    addSearchMarkerLayer(map, palette) // #277: search result pin, topmost
    state.mapLoaded = true
    store.setMapLoaded(true)
    // ASD-011 (#179): apply the airspace type filter directly on load, so the
    // engine initialises its own layer filters on EVERY mount — not only on the
    // first one. The MapCanvas watcher on store.mapLoaded fires only on the
    // false→true edge, but store.mapLoaded is a write-once-true latch on the
    // singleton Pinia store: on a second mount (logout→login, tenant switch,
    // re-login without a full reload) it is already true, so the edge — and thus
    // the initial filter — never fires. Calling it here makes correctness
    // independent of the store edge; the non-mapped, country-wide airspace types
    // (UIR/FIR/ADIZ/TRA …) are filtered out immediately instead of only after
    // the first group toggle.
    updateAirspaceFilter()
    // ASD-014: apply any AoR highlight already known at load (mirrors the
    // updateAirspaceFilter call above — initialise on every mount).
    updateAoR(aorIds)

    if (state.pendingTracks) {
      updateTracksLayer(state.pendingTracks, state, doRender, startFadeLoop, store.historyConfig.durationS * 1000)
      state.pendingTracks = null
      syncLiveTrackNums()
      store.refreshSelectedTrack(state.liveTrackFeatures) // #272
      onTracks?.(state.liveTrackFeatures) // #297: initial anchor for measure endpoints
    }

    // Load aeronautical data and start periodic refresh.
    const aeroInterval = startAeronauticalRefresh(map)

    // WX-C: load DWD warnings and refresh on the warn cadence. Best-effort — a
    // failed/absent fetch simply leaves the (empty) overlay unchanged.
    const loadWarnings = () => {
      fetch(WEATHER_WARNINGS_URL)
        .then((r) => (r.ok ? r.json() : null))
        .then((geojson) => {
          if (!geojson) return
          lastWarningsRaw = geojson
          // #190: clip the warnings to the tenant AOI so a huge dissolved warning
          // region is cut to the sector instead of covering the whole map.
          updateWeatherWarnings(map, clipFeatureCollectionToBBox(geojson, weatherAOI))
        })
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
      })
    })

    // Track click → emit to Vue component. #271: a click on the data-block
    // LABEL selects the same track as a click on the symbol — the label is the
    // larger, natural click target. Label DRAGS (ASD-002 pinning) stay
    // distinct for free: MapLibre suppresses the click event once the pointer
    // moves beyond clickTolerance, so only true clicks arrive here.
    const emitTrackClick = (layerId) => (e) => {
      const features = map.queryRenderedFeatures(e.point, { layers: [layerId] })
      if (!features || features.length === 0) return
      const props = features[0].properties
      // Find the full track data from liveTrackFeatures.
      const liveFeature = state.liveTrackFeatures.find(
        (f) => f.properties.track_num === props.track_num,
      )
      if (liveFeature && onTrackClick) {
        onTrackClick(liveFeature.properties)
      }
    }
    map.on('click', TRACKS_LAYER_ID, emitTrackClick(TRACKS_LAYER_ID))
    map.on('click', LABELS_LAYER_ID, emitTrackClick(LABELS_LAYER_ID))

    // #273: a click on FREE map area (no track symbol/label under the cursor)
    // deselects — the standard map-UI convention. The layer-specific handlers
    // above fire alongside this general one; when they hit a track the query
    // here is non-empty and nothing happens. Camera pans never arrive (MapLibre
    // suppresses click after a drag) and the measure-tool guard lives with the
    // callback owner (AsdView), mirroring onTrackClick.
    map.on('click', (e) => {
      if (!onEmptyClick) return
      const hits = map.queryRenderedFeatures(e.point, {
        layers: [TRACKS_LAYER_ID, LABELS_LAYER_ID],
      })
      if (!hits || hits.length === 0) onEmptyClick()
    })

    // Store cleanup ref for aeroInterval.
    map._aeroInterval = aeroInterval
  })

  connectWebSocket()

  // Layer visibility control: called by MapCanvas when store changes.
  function setLayerVisibility(vis) {
    if (!state.mapLoaded) return
    // #274 + E2 (#293): the base map is NOT a plain group here — its visibility
    // is the master AND the per-element switches combined (applyBasemap), so it
    // is handled below instead of in this flat show/hide loop.
    const groups = {
      airspace: [AIRSPACE_FILL_LAYER_ID, AIRSPACE_LINE_LAYER_ID, AIRSPACE_LABEL_LAYER_ID],
      aor: [AIRSPACE_AOR_LAYER_ID], // ASD-014: AoR highlight toggle
      navaids: [NAVAIDS_LAYER_ID],
      waypoints: [WAYPOINTS_LAYER_ID],
      coverageRings: [COVERAGE_RINGS_LAYER_ID, COVERAGE_RINGS_LAYER_ID + '-inner', COVERAGE_CENTER_LAYER_ID],
      rangeRings: [RANGE_RINGS_LAYER_ID, RANGE_RINGS_LABEL_LAYER_ID],
      historyDots: [HISTORY_DOTS_LAYER_ID],
      weatherRadar: [WEATHER_RADAR_LAYER_ID],
      weatherWarnings: [WEATHER_WARNINGS_FILL_LAYER_ID, WEATHER_WARNINGS_LINE_LAYER_ID],
      airport: [AIRPORT_LAYER_ID, AIRPORT_LABEL_LAYER_ID], // #192
      runways: [RUNWAY_LAYER_ID], // #192
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
    // #274 + E2 (#293): apply the base map (master × per-element) when its master
    // changed. Element-only changes come through the dedicated watcher → applyBasemap.
    if ('basemap' in vis) applyBasemap()
  }

  // #274 + E2 (#293): apply the base map's visibility as the combination of the
  // MASTER (store.layerVisibility.basemap — is the map shown at all, #274) and
  // the PER-ELEMENT switches (store.basemapElementVisibility — which parts show
  // when it is). A layer is visible iff the master is on AND its element group is
  // on; an unclassified group ('other', absent from the element map) defaults to
  // on, so it simply follows the master. All hidden when the master is off.
  function applyBasemap() {
    // No mapLoaded guard: this also runs from the load handler BEFORE mapLoaded
    // is set, to apply the initial base-map visibility. Safe before load anyway —
    // state.basemapGroups is empty until the style loads, so the loop is a no-op.
    const on = store.layerVisibility.basemap
    const elements = store.basemapElementVisibility
    for (const [group, ids] of Object.entries(state.basemapGroups)) {
      const el = elements[group]
      const visible = on && (el === undefined ? true : el)
      const visibility = visible ? 'visible' : 'none'
      ids.forEach((id) => {
        if (map.getLayer(id)) {
          map.setLayoutProperty(id, 'visibility', visibility)
        }
      })
    }
  }

  // E0 (#291): show/hide ONE base-map element group (water, traffic, …). The
  // group→ids map was bucketed at style load (state.basemapGroups). No UI calls
  // this yet — it is the capability E2/#293 wires the per-element sidebar
  // switches to. A no-op for an unknown group or before the map has loaded.
  function setBasemapGroupVisibility(group, visible) {
    if (!state.mapLoaded) return
    const ids = state.basemapGroups[group]
    if (!ids) return
    const visibility = visible ? 'visible' : 'none'
    ids.forEach((id) => {
      if (map.getLayer(id)) {
        map.setLayoutProperty(id, 'visibility', visibility)
      }
    })
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

  // ASD-013: select a track by its number, driven from the Ereignis-Panel
  // (clicking "Track N erschienen" for a still-live track). Looks the track up in
  // the current displayed set, opens the detail panel via the store (the
  // selectedTrack watcher then paints the ASD-007 halo) and gently eases the
  // camera onto it so the highlighted symbol is actually in view. Returns false
  // when the track is no longer live, so the caller can leave the UI untouched.
  function selectTrackByNum(trackNum) {
    const feature = state.liveTrackFeatures.find(
      (f) => f.properties.track_num === trackNum,
    )
    if (!feature) return false
    store.selectTrack(feature.properties)
    const c = feature.geometry?.coordinates
    if (Array.isArray(c) && Number.isFinite(c[0]) && Number.isFinite(c[1])) {
      map.easeTo({ center: c, duration: 400 })
    }
    return true
  }

  // #277 (ADR 0028): drop the sector-search result marker on the picked place
  // and fly the camera onto it. The marker is a transient navigation aid, not
  // scope state — it lives outside the render loop and is cleared explicitly
  // (next selection replaces it; clear/Esc in MapSearch removes it).
  function showSearchMarker(lon, lat, name) {
    if (!state.mapLoaded) return
    if (!Number.isFinite(lon) || !Number.isFinite(lat)) return
    updateSearchMarker(map, {
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [lon, lat] },
      properties: { name: name || '' },
    })
    // #277 Nachtrag: fly to the place at a fixed focus zoom, not just centre.
    // The zoom is ABSOLUTE — it pulls the scope IN when it was far out and OUT
    // when it was zoomed in too close, so the street is always readable at one
    // consistent distance regardless of where the Lotse was looking before.
    map.flyTo({ center: [lon, lat], zoom: SEARCH_RESULT_ZOOM, duration: 900 })
  }

  // #277: remove the search result marker (search cleared / Esc).
  function clearSearchMarker() {
    if (!state.mapLoaded) return
    updateSearchMarker(map, null)
  }

  // #191: history retention/fade changed — re-render immediately so the new
  // window takes effect without waiting for the next WS update. (Points already
  // stored are only pruned on the next updateTrackHistory, but the age fade and
  // any future pruning use the new value at once.)
  function updateHistoryConfig() {
    doRender()
  }

  // #189/#190 + #289: the tenant's AOI resolved after mount or changed (e.g. an
  // admin switching the impersonation target). Re-bound the radar raster, re-clip
  // the warnings, and re-cut the base-map mask to the new sector. No-op before
  // the style has loaded. (Named for weather historically; it is the AOI hook.)
  function applyWeatherAOI(aoi) {
    weatherAOI = aoi
    if (!state.mapLoaded) return
    setWeatherRadarAOI(map, aoi)
    setBasemapMaskAOI(map, aoi) // #289: limit the base map to the AOI
    if (lastWarningsRaw) {
      updateWeatherWarnings(map, clipFeatureCollectionToBBox(lastWarningsRaw, aoi))
    }
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

  // ASD-014: highlight the tenant's Area-of-Responsibility airspaces
  // (session.aorAirspaceIds, whoami) by filtering the dedicated AoR line layer to
  // those feature ids. Mirrors applyWeatherAOI: it stores the ids in the closure
  // and re-applies; a no-op before the style has loaded (the load handler
  // re-applies on every mount). An empty/absent list highlights nothing.
  function updateAoR(ids) {
    aorIds = Array.isArray(ids) ? ids : []
    if (!state.mapLoaded) return
    if (map.getLayer(AIRSPACE_AOR_LAYER_ID)) map.setFilter(AIRSPACE_AOR_LAYER_ID, aorFilter(aorIds))
  }

  // Destroy: close WS, clear intervals, remove map.
  function destroy() {
    if (reconnectTimer) clearTimeout(reconnectTimer)
    if (ws) ws.close()
    if (state.fadeInterval) clearInterval(state.fadeInterval)
    if (map._aeroInterval) clearInterval(map._aeroInterval)
    if (map._warnInterval) clearInterval(map._warnInterval)
    map.remove()
    // #179 hygiene: clear the singleton store's map-loaded latch on teardown so
    // the false→true edge is restored for the next mount. This protects any
    // other effect keyed on the store.mapLoaded edge (not just the airspace
    // filter, which the load handler now applies directly and defensively).
    store.setMapLoaded(false)
  }

  // ASD-009: Map control helpers exposed to the Vue chrome layer.
  // They are intentionally thin wrappers — the map object owns the state,
  // and the chrome layer never needs to reach into it directly.
  function zoomIn()    { map.zoomIn() }
  function zoomOut()   { map.zoomOut() }
  // recenter restores the full start view in one click (#169): centre + zoom AND
  // bearing 0 (north-up) + pitch 0 (top-down), so a rotated/tilted scope snaps
  // back to exactly how it opened — not just re-centred.
  function recenter()  { map.flyTo({ center: [effectiveCenter.lon, effectiveCenter.lat], zoom: effectiveCenter.zoom, bearing: 0, pitch: 0 }) }

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

  return { map, destroy, reconnect, setLayerVisibility, setBasemapGroupVisibility, applyBasemap, updateFlFilter, updateAirspaceFilter, updateAoR, updateSelection, selectTrackByNum, updateHistoryConfig, applyWeatherAOI, zoomIn, zoomOut, recenter, applyViewCenter, updateRangeRings, showSearchMarker, clearSearchMarker }
}
