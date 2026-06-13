// Wayfinder ASD frontend. Loads the configured map style and centers the
// view on the configured position. Connects to the WebSocket server (/ws)
// to receive live track updates.

const state = {
  ws: null,
  reconnectTimer: null,
  reconnectDelay: 2000,
};

async function main() {
  const res = await fetch("/api/map-config");
  const cfg = await res.json();

  new maplibregl.Map({
    container: "map",
    style: cfg.style,
    center: [cfg.center_lon, cfg.center_lat],
    zoom: cfg.zoom,
  });

  connectWebSocket();
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
      console.log("Track update:", msg);
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
