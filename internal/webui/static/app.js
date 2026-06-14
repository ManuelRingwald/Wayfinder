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

// Speed-vector look-ahead: how many seconds of travel the vector line
// represents (standard ASD-style speed vector line, SVL).
const VECTOR_LOOKAHEAD_S = 60;

// Maximum number of past positions kept per track for the trail display.
const TRAIL_MAX_POINTS = 20;

// Mean Earth radius (m), used for the local meters-to-degrees conversion of
// the vector endpoint. Sufficient accuracy for display purposes.
const EARTH_RADIUS_M = 6371000;

const state = {
  map: null,
  mapLoaded: false,
  ws: null,
  reconnectTimer: null,
  reconnectDelay: 2000,
  pendingTracks: null,
  // Per-track history of past positions ([lon, lat]), for the trail display.
  trackHistory: new Map(),
};

async function main() {
  const res = await fetch("/api/map-config");
  const cfg = await res.json();

  const map = new maplibregl.Map({
    container: "map",
    style: cfg.style,
    center: [cfg.center_lon, cfg.center_lat],
    zoom: cfg.zoom,
  });
  state.map = map;

  map.on("load", () => {
    addTrailsLayer(map);
    addVectorsLayer(map);
    addTracksLayer(map);
    state.mapLoaded = true;
    if (state.pendingTracks) {
      updateTracksLayer(state.pendingTracks);
      state.pendingTracks = null;
    }
  });

  connectWebSocket();
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
      "circle-stroke-color": "#000000",
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
      "text-color": "#212121",
      "text-halo-color": "#ffffff",
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
      "line-color": "#90a4ae",
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
      "line-color": "#212121",
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

// buildLabel produces the track's data-block label: the track number, and —
// when the track carries a measured flight level (I062/136) — a second line
// "FLnnn" (flight level in hundreds of feet, ASD convention).
function buildLabel(track) {
  const num = String(track.track_num);
  if (typeof track.flight_level_ft === "number") {
    const fl = Math.round(track.flight_level_ft / 100);
    return `${num}\nFL${String(fl).padStart(3, "0")}`;
  }
  return num;
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
function updateTracksLayer(msg) {
  const tracks = msg.tracks || [];

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
