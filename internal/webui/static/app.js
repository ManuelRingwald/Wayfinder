// Wayfinder ASD frontend. Loads the configured map style and centers the
// view on the configured position. Connects to the WebSocket server (/ws)
// to receive live track updates and renders them as map symbols.

const TRACKS_SOURCE_ID = "tracks";
const TRACKS_LAYER_ID = "tracks-points";
const TRACKS_LABEL_LAYER_ID = "tracks-labels";
const VECTORS_SOURCE_ID = "track-vectors";
const VECTORS_LAYER_ID = "track-vectors-lines";
const TRAILS_SOURCE_ID = "track-trails";
const TRAILS_LAYER_ID = "track-trails-lines";

// Aeronautical overlay layers (ASD-003, fed by the OpenAIP backend via
// /api/airspace, /api/navaids, /api/waypoints). They render beneath the track
// layers so tracks always dominate the scope.
const AIRSPACE_SOURCE_ID = "airspace";
const AIRSPACE_FILL_LAYER_ID = "airspace-fill";
const AIRSPACE_LINE_LAYER_ID = "airspace-line";
const AIRSPACE_LABEL_LAYER_ID = "airspace-label";
const NAVAIDS_SOURCE_ID = "navaids";
const NAVAIDS_LAYER_ID = "navaids-symbols";
const WAYPOINTS_SOURCE_ID = "waypoints";
const WAYPOINTS_LAYER_ID = "waypoints-symbols";

// How often the frontend re-pulls the aeronautical GeoJSON. The backend itself
// refreshes from OpenAIP on the AIRAC-paced interval; this only needs to be
// frequent enough to pick up a backend cache update, not to hit OpenAIP.
const AERO_REFRESH_MS = 5 * 60 * 1000;

// Speed-vector look-ahead: how many seconds of travel the vector line
// represents (standard ASD-style speed vector line, SVL).
const VECTOR_LOOKAHEAD_S = 60;

// Maximum number of past positions kept per track for the trail display.
const TRAIL_MAX_POINTS = 20;

// Mean Earth radius (m), used for the local meters-to-degrees conversion of
// the vector endpoint. Sufficient accuracy for display purposes.
const EARTH_RADIUS_M = 6371000;

// Foreground palettes per base-map theme (ASD-003 Häppchen 3a). On the dark
// "Radar Dark Mode" base, labels are light with a dark halo so they stay
// legible; on the bright OSM base the original dark-on-white palette is used.
// Track-status colours (confirmed/coasting/tentative) read well on both bases.
const PALETTES = {
  dark: {
    label: "#e8eef5",
    labelHalo: "#000000",
    vector: "#cfd8dc",
    trail: "#607d8b",
    symbolStroke: "#000000",
    airspaceLine: "#5b8fd6",
    airspaceText: "#9fc0e8",
    aeroHalo: "#000000",
  },
  osm: {
    label: "#212121",
    labelHalo: "#ffffff",
    vector: "#212121",
    trail: "#90a4ae",
    symbolStroke: "#000000",
    airspaceLine: "#1f4ea8",
    airspaceText: "#22305a",
    aeroHalo: "#ffffff",
  },
};

const state = {
  map: null,
  mapLoaded: false,
  ws: null,
  reconnectTimer: null,
  reconnectDelay: 2000,
  pendingTracks: null,
  // Per-track history of past positions ([lon, lat]), for the trail display.
  trackHistory: new Map(),
  // Active foreground palette, selected from the configured map theme.
  palette: PALETTES.dark,
};

async function main() {
  const res = await fetch("/api/map-config");
  const cfg = await res.json();

  // Select the foreground palette to match the base-map theme (dark by
  // default). An unknown theme falls back to the dark palette.
  state.palette = PALETTES[cfg.theme] || PALETTES.dark;

  const map = new maplibregl.Map({
    container: "map",
    style: cfg.style,
    center: [cfg.center_lon, cfg.center_lat],
    zoom: cfg.zoom,
  });
  state.map = map;

  map.on("load", () => {
    // Aeronautical overlays first, so they sit beneath the track layers.
    addAeronauticalIcons(map);
    addAirspaceLayers(map);
    addNavaidLayers(map);
    addWaypointLayers(map);
    addTrailsLayer(map);
    addVectorsLayer(map);
    addTracksLayer(map);
    state.mapLoaded = true;
    if (state.pendingTracks) {
      updateTracksLayer(state.pendingTracks);
      state.pendingTracks = null;
    }

    // Load aeronautical data and refresh it periodically.
    loadAeronautical(map);
    setInterval(() => loadAeronautical(map), AERO_REFRESH_MS);

    setupLayerControl(map);
  });

  connectWebSocket();
}

// makeIconImage renders a small icon onto an offscreen canvas and returns its
// ImageData, so we need no external sprite assets (keeps Wayfinder a single
// self-contained binary). draw(ctx, size) paints into a size×size square.
function makeIconImage(draw) {
  const size = 24;
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d");
  draw(ctx, size);
  return ctx.getImageData(0, 0, size, size);
}

// addAeronauticalIcons registers the navaid/waypoint marker icons: a triangle
// for waypoints, a compass-rose ring for VOR-family navaids, and a
// dashed/dotted ring for NDBs. Colours are chosen to read on the dark scope.
function addAeronauticalIcons(map) {
  const add = (id, image) => {
    if (!map.hasImage(id)) {
      map.addImage(id, image, { pixelRatio: 2 });
    }
  };

  add(
    "wf-waypoint",
    makeIconImage((ctx, s) => {
      const c = s / 2;
      ctx.strokeStyle = "#4dd0e1";
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(c, c - 7);
      ctx.lineTo(c + 6, c + 5);
      ctx.lineTo(c - 6, c + 5);
      ctx.closePath();
      ctx.stroke();
    }),
  );

  add(
    "wf-vor",
    makeIconImage((ctx, s) => {
      const c = s / 2;
      ctx.strokeStyle = "#80cbc4";
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.arc(c, c, 7, 0, 2 * Math.PI);
      ctx.stroke();
      // compass-rose ticks
      for (let i = 0; i < 8; i++) {
        const a = (i * Math.PI) / 4;
        ctx.beginPath();
        ctx.moveTo(c + Math.cos(a) * 7, c + Math.sin(a) * 7);
        ctx.lineTo(c + Math.cos(a) * 10, c + Math.sin(a) * 10);
        ctx.stroke();
      }
    }),
  );

  add(
    "wf-ndb",
    makeIconImage((ctx, s) => {
      const c = s / 2;
      ctx.strokeStyle = "#ffb74d";
      ctx.lineWidth = 2;
      ctx.setLineDash([2, 2]);
      ctx.beginPath();
      ctx.arc(c, c, 7, 0, 2 * Math.PI);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.fillStyle = "#ffb74d";
      ctx.beginPath();
      ctx.arc(c, c, 1.6, 0, 2 * Math.PI);
      ctx.fill();
    }),
  );

  add(
    "wf-navaid",
    makeIconImage((ctx, s) => {
      const c = s / 2;
      ctx.strokeStyle = "#b0bec5";
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.arc(c, c, 6, 0, 2 * Math.PI);
      ctx.stroke();
    }),
  );
}

// addAirspaceLayers registers the airspace source and its fill/outline/label
// layers (sector and FIR boundaries). Polygons get a faint fill so overlapping
// sectors stay readable on the dark base.
function addAirspaceLayers(map) {
  map.addSource(AIRSPACE_SOURCE_ID, {
    type: "geojson",
    data: { type: "FeatureCollection", features: [] },
  });

  map.addLayer({
    id: AIRSPACE_FILL_LAYER_ID,
    type: "fill",
    source: AIRSPACE_SOURCE_ID,
    filter: ["==", ["geometry-type"], "Polygon"],
    paint: {
      "fill-color": state.palette.airspaceLine,
      "fill-opacity": 0.06,
    },
  });

  map.addLayer({
    id: AIRSPACE_LINE_LAYER_ID,
    type: "line",
    source: AIRSPACE_SOURCE_ID,
    paint: {
      "line-color": state.palette.airspaceLine,
      "line-width": 1,
      "line-opacity": 0.8,
    },
  });

  map.addLayer({
    id: AIRSPACE_LABEL_LAYER_ID,
    type: "symbol",
    source: AIRSPACE_SOURCE_ID,
    minzoom: 6,
    layout: {
      "text-field": ["coalesce", ["get", "name"], ""],
      "text-size": 10,
      "symbol-placement": "line",
    },
    paint: {
      "text-color": state.palette.airspaceText,
      "text-halo-color": state.palette.aeroHalo,
      "text-halo-width": 1,
    },
  });
}

// addNavaidLayers registers the navaids source and a symbol layer that picks an
// icon by navaid kind (VOR family / NDB / generic). A zoom floor keeps the
// scope uncluttered when zoomed far out.
function addNavaidLayers(map) {
  map.addSource(NAVAIDS_SOURCE_ID, {
    type: "geojson",
    data: { type: "FeatureCollection", features: [] },
  });

  map.addLayer({
    id: NAVAIDS_LAYER_ID,
    type: "symbol",
    source: NAVAIDS_SOURCE_ID,
    minzoom: 6,
    layout: {
      "icon-image": [
        "match",
        ["get", "navaid_kind"],
        ["VOR", "VOR-DME", "VORTAC", "DVOR", "DVOR-DME", "DVORTAC"],
        "wf-vor",
        "NDB",
        "wf-ndb",
        "wf-navaid",
      ],
      "icon-size": 1,
      "icon-allow-overlap": true,
      "text-field": ["coalesce", ["get", "ident"], ["get", "name"], ""],
      "text-size": 10,
      "text-offset": [0, 1.1],
      "text-anchor": "top",
    },
    paint: {
      "text-color": state.palette.airspaceText,
      "text-halo-color": state.palette.aeroHalo,
      "text-halo-width": 1,
    },
  });
}

// addWaypointLayers registers the waypoints source and its triangle-marker
// symbol layer, with a higher zoom floor (waypoints are denser than navaids).
function addWaypointLayers(map) {
  map.addSource(WAYPOINTS_SOURCE_ID, {
    type: "geojson",
    data: { type: "FeatureCollection", features: [] },
  });

  map.addLayer({
    id: WAYPOINTS_LAYER_ID,
    type: "symbol",
    source: WAYPOINTS_SOURCE_ID,
    minzoom: 7,
    layout: {
      "icon-image": "wf-waypoint",
      "icon-size": 1,
      "icon-allow-overlap": false,
      "text-field": ["coalesce", ["get", "name"], ""],
      "text-size": 9,
      "text-offset": [0, 1.0],
      "text-anchor": "top",
    },
    paint: {
      "text-color": state.palette.airspaceText,
      "text-halo-color": state.palette.aeroHalo,
      "text-halo-width": 1,
    },
  });
}

// loadAeronautical pulls the cached GeoJSON for each overlay and pushes it into
// the matching source. Failures are non-fatal: an empty/unreachable endpoint
// simply leaves that overlay unchanged (graceful degradation, ADR 0004).
async function loadAeronautical(map) {
  const sources = [
    ["/api/airspace", AIRSPACE_SOURCE_ID],
    ["/api/navaids", NAVAIDS_SOURCE_ID],
    ["/api/waypoints", WAYPOINTS_SOURCE_ID],
  ];
  await Promise.all(
    sources.map(async ([url, sourceId]) => {
      try {
        const res = await fetch(url);
        if (!res.ok) {
          return;
        }
        const data = await res.json();
        const src = map.getSource(sourceId);
        if (src) {
          src.setData(data);
        }
      } catch (err) {
        console.warn("aeronautical load failed for", url, err);
      }
    }),
  );
}

// setupLayerControl wires the overlay checkboxes to MapLibre layer visibility,
// so the controller can declutter the scope (airspace / navaids / waypoints).
function setupLayerControl(map) {
  const groups = [
    ["toggle-airspace", [AIRSPACE_FILL_LAYER_ID, AIRSPACE_LINE_LAYER_ID, AIRSPACE_LABEL_LAYER_ID]],
    ["toggle-navaids", [NAVAIDS_LAYER_ID]],
    ["toggle-waypoints", [WAYPOINTS_LAYER_ID]],
  ];
  groups.forEach(([checkboxId, layerIds]) => {
    const cb = document.getElementById(checkboxId);
    if (!cb) {
      return;
    }
    const apply = () => {
      const visibility = cb.checked ? "visible" : "none";
      layerIds.forEach((id) => {
        if (map.getLayer(id)) {
          map.setLayoutProperty(id, "visibility", visibility);
        }
      });
    };
    cb.addEventListener("change", apply);
    apply();
  });
}

// addTracksLayer registers a GeoJSON source and two layers for rendering
// tracks: a coloured circle per track (status-dependent) and a text label
// with the track number.
function addTracksLayer(map) {
  map.addSource(TRACKS_SOURCE_ID, {
    type: "geojson",
    data: { type: "FeatureCollection", features: [] },
  });

  map.addLayer({
    id: TRACKS_LAYER_ID,
    type: "circle",
    source: TRACKS_SOURCE_ID,
    paint: {
      "circle-radius": 5,
      "circle-color": [
        "case",
        ["get", "coasting"],
        "#ff9800", // orange: coasting (no recent update)
        ["get", "confirmed"],
        "#4caf50", // green: confirmed track
        "#9e9e9e", // grey: tentative track
      ],
      "circle-stroke-width": 1,
      "circle-stroke-color": state.palette.symbolStroke,
    },
  });

  map.addLayer({
    id: TRACKS_LABEL_LAYER_ID,
    type: "symbol",
    source: TRACKS_SOURCE_ID,
    layout: {
      // Precomputed two-line label: track number, and flight level when known
      // (ASD-style data block). See buildLabel in updateTracksLayer.
      "text-field": ["get", "label"],
      "text-size": 11,
      "text-offset": [0, 1.2],
      "text-anchor": "top",
    },
    paint: {
      "text-color": state.palette.label,
      "text-halo-color": state.palette.labelHalo,
      "text-halo-width": 1,
    },
  });
}

// addTrailsLayer registers a GeoJSON source and a line layer for rendering
// each track's recent flight path (a fading trail of its last positions).
// Added first so trails draw beneath the speed vectors and track symbols.
function addTrailsLayer(map) {
  map.addSource(TRAILS_SOURCE_ID, {
    type: "geojson",
    data: { type: "FeatureCollection", features: [] },
  });

  map.addLayer({
    id: TRAILS_LAYER_ID,
    type: "line",
    source: TRAILS_SOURCE_ID,
    paint: {
      "line-color": state.palette.trail,
      "line-width": 1,
      "line-opacity": 0.6,
    },
  });
}

// addVectorsLayer registers a GeoJSON source and a line layer for rendering
// each track's speed vector (a short line from the current position towards
// where the track will be in VECTOR_LOOKAHEAD_S seconds, ASD-style SVL).
// Added before the tracks layer so the track symbols draw on top.
function addVectorsLayer(map) {
  map.addSource(VECTORS_SOURCE_ID, {
    type: "geojson",
    data: { type: "FeatureCollection", features: [] },
  });

  map.addLayer({
    id: VECTORS_LAYER_ID,
    type: "line",
    source: VECTORS_SOURCE_ID,
    paint: {
      "line-color": state.palette.vector,
      "line-width": 1.5,
    },
  });
}

// vectorEndpoint computes the geographic point reached after
// VECTOR_LOOKAHEAD_S seconds of travel at constant velocity (vx/vy in m/s,
// East/North), starting from (lat, lon). Uses a local flat-Earth
// approximation, which is sufficient for the short look-ahead distances
// involved.
function vectorEndpoint(lat, lon, vx, vy) {
  const dEast = vx * VECTOR_LOOKAHEAD_S;
  const dNorth = vy * VECTOR_LOOKAHEAD_S;

  const dLat = (dNorth / EARTH_RADIUS_M) * (180 / Math.PI);
  const dLon =
    (dEast / (EARTH_RADIUS_M * Math.cos((lat * Math.PI) / 180))) *
    (180 / Math.PI);

  return [lon + dLon, lat + dLat];
}

// buildLabel produces the track's data-block label: the callsign (I062/245),
// or the track number if no callsign is known, and — when the track carries a
// measured flight level (I062/136) — a second line "FLnnn" (flight level in
// hundreds of feet, ASD convention).
function buildLabel(track) {
  const line1 =
    typeof track.callsign === "string" && track.callsign !== ""
      ? track.callsign
      : String(track.track_num);
  if (typeof track.flight_level_ft === "number") {
    const fl = Math.round(track.flight_level_ft / 100);
    return `${line1}\nFL${String(fl).padStart(3, "0")}`;
  }
  return line1;
}

// updateTrackHistory appends each track's current position to its trail
// history (capped at TRAIL_MAX_POINTS) and drops history for tracks that are
// no longer present in the latest update.
function updateTrackHistory(tracks) {
  const seen = new Set();

  tracks.forEach((track) => {
    seen.add(track.track_num);
    let hist = state.trackHistory.get(track.track_num);
    if (!hist) {
      hist = [];
      state.trackHistory.set(track.track_num, hist);
    }
    hist.push([track.longitude, track.latitude]);
    if (hist.length > TRAIL_MAX_POINTS) {
      hist.shift();
    }
  });

  for (const trackNum of state.trackHistory.keys()) {
    if (!seen.has(trackNum)) {
      state.trackHistory.delete(trackNum);
    }
  }
}

// updateTracksLayer converts a Message (see pkg/broadcast.Message) into
// GeoJSON FeatureCollections and pushes them into the map sources: track
// symbols/labels, their speed-vector lines, and their recent-position trails.
// updateFeedBanner reflects the CAT065 feed-health state (Firefly ADR 0018)
// in the top-right banner: green "FEED OK", red "FEED STALE" (heartbeat lost),
// or grey "FEED ?" until the first heartbeat arrives.
function updateFeedBanner(feedStatus) {
  const el = document.getElementById("feed-status");
  if (!el) {
    return;
  }
  const state = feedStatus.state;
  el.className = state;
  if (state === "ok") {
    el.textContent = "● FEED OK";
  } else if (state === "stale") {
    el.textContent = "▲ FEED STALE — kein Heartbeat";
  } else {
    el.textContent = "● FEED ?";
  }
}

function updateTracksLayer(msg) {
  // Drop tracks flagged `ended` (CAT062 I062/080 TSE): this is their final
  // report, signalling deletion. Excluding them here makes the symbol, label
  // and speed vector disappear at once, and — since they fall out of the
  // "seen" set in updateTrackHistory — their trail history is purged too.
  const tracks = (msg.tracks || []).filter((track) => !track.ended);

  updateTrackHistory(tracks);

  const features = tracks.map((track) => ({
    type: "Feature",
    geometry: {
      type: "Point",
      coordinates: [track.longitude, track.latitude],
    },
    properties: {
      track_num: track.track_num,
      confirmed: track.confirmed,
      coasting: track.coasting,
      vx: track.vx,
      vy: track.vy,
      label: buildLabel(track),
    },
  }));

  const vectorFeatures = tracks.map((track) => ({
    type: "Feature",
    geometry: {
      type: "LineString",
      coordinates: [
        [track.longitude, track.latitude],
        vectorEndpoint(track.latitude, track.longitude, track.vx, track.vy),
      ],
    },
    properties: {
      track_num: track.track_num,
    },
  }));

  state.map.getSource(TRACKS_SOURCE_ID).setData({
    type: "FeatureCollection",
    features,
  });

  state.map.getSource(VECTORS_SOURCE_ID).setData({
    type: "FeatureCollection",
    features: vectorFeatures,
  });

  const trailFeatures = [];
  for (const [trackNum, hist] of state.trackHistory) {
    if (hist.length >= 2) {
      trailFeatures.push({
        type: "Feature",
        geometry: {
          type: "LineString",
          coordinates: hist,
        },
        properties: {
          track_num: trackNum,
        },
      });
    }
  }

  state.map.getSource(TRAILS_SOURCE_ID).setData({
    type: "FeatureCollection",
    features: trailFeatures,
  });
}

function connectWebSocket() {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const wsURL = `${protocol}//${window.location.host}/ws`;

  console.log("Connecting to", wsURL);

  const ws = new WebSocket(wsURL);

  ws.addEventListener("open", () => {
    console.log("WebSocket connected");
    state.ws = ws;
    // Clear any pending reconnect timer.
    if (state.reconnectTimer) {
      clearTimeout(state.reconnectTimer);
      state.reconnectTimer = null;
    }
  });

  ws.addEventListener("message", (event) => {
    try {
      const msg = JSON.parse(event.data);
      // Feed-health updates (CAT065 heartbeat) are separate from the track
      // stream; route them to the banner and never through the track layer,
      // so a heartbeat message doesn't clear the air picture.
      if (msg.feed_status) {
        updateFeedBanner(msg.feed_status);
        return;
      }
      if (state.mapLoaded) {
        updateTracksLayer(msg);
      } else {
        state.pendingTracks = msg;
      }
    } catch (err) {
      console.error("Failed to parse message:", err, event.data);
    }
  });

  ws.addEventListener("close", () => {
    console.warn("WebSocket disconnected, reconnecting in", state.reconnectDelay, "ms");
    state.ws = null;
    state.reconnectTimer = setTimeout(connectWebSocket, state.reconnectDelay);
  });

  ws.addEventListener("error", (err) => {
    console.error("WebSocket error:", err);
  });
}

main();
