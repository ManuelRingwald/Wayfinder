// Wayfinder ASD frontend. Loads the configured map style and centers the
// view on the configured position. Connects to the WebSocket server (/ws)
// to receive live track updates and renders them as map symbols.

const TRACKS_SOURCE_ID = "tracks";
const TRACKS_LAYER_ID = "tracks-points";
const VECTORS_SOURCE_ID = "track-vectors";
const VECTORS_LAYER_ID = "track-vectors-lines";
const TRAILS_SOURCE_ID = "track-trails";
const TRAILS_LAYER_ID = "track-trails-lines";
// ASD-004a: individual position-dot layer, rendered above the trail line.
const HISTORY_DOTS_SOURCE_ID = "track-history-dots";
const HISTORY_DOTS_LAYER_ID = "track-history-dots-circles";

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

// ASD-004c: duration of the TSE graceful fade-out animation in milliseconds.
const FADE_DURATION_MS = 1500;

// ASD-002: Anti-Garbling — separate GeoJSON sources for deconflicted labels
// and leader lines (lines from symbol to data-block anchor).
const LABELS_SOURCE_ID = "track-labels";
const LABELS_LAYER_ID = "track-labels-text";
const LEADER_LINES_SOURCE_ID = "track-leader-lines";
const LEADER_LINES_LAYER_ID = "track-leader-lines-lines";

// ASD-002: Deconfliction geometry constants (all values in screen pixels).
// LABEL_SLOT_RADIUS_PX : distance from symbol centre to label anchor candidate.
// LABEL_W/H_PX         : conservative bounding box for a 3-line data block at text-size 11.
// SYMBOL_BBOX_R_PX     : symbol footprint reserved so OTHER tracks' labels avoid this dot.
// LEADER_THRESHOLD_PX  : minimum symbol→label distance before a leader line is drawn.
const LABEL_SLOT_RADIUS_PX = 20;
const LABEL_W_PX = 62;
const LABEL_H_PX = 46;
const SYMBOL_BBOX_R_PX = 8;
const LEADER_THRESHOLD_PX = 10;

// ASD-002: Eight candidate placement slots as normalised screen-space direction
// vectors, ordered right-first following ATC scope convention. Each vector is
// scaled by LABEL_SLOT_RADIUS_PX to get the candidate label centre in pixels.
const LABEL_SLOTS = [
  [ 1.2,  0.3],  // right (ATC default)
  [ 0,    1.4],  // below
  [-1.2,  0.3],  // left
  [ 0,   -1.4],  // above
  [ 1.2, -0.5],  // upper-right
  [-1.2, -0.5],  // upper-left
  [ 1.2,  1.0],  // lower-right
  [-1.2,  1.0],  // lower-left
];

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
  // Per-track history of past positions ([lon, lat]), for trail and dot display.
  trackHistory: new Map(),
  // Per-track last-known flight level in feet, for the vertical-tendency
  // indicator (ASD-001b).
  trackFlHistory: new Map(),
  // ASD-004b: per-track coasting flag, needed by trail/dot features for the
  // dimming paint expression (trackHistory doesn't carry track-level metadata).
  trackCoasting: new Map(),
  // ASD-004c: tracks fading out after TSE; Map<track_num, {deadline, track}>.
  // History and coasting state are kept alive until the deadline expires.
  fadingTracks: new Map(),
  // setInterval handle for the fade-out animation loop (null when idle).
  fadeInterval: null,
  // Precomputed GeoJSON features for the current live-track frame; reused by
  // renderSources() on each fade-loop tick to avoid redundant computation.
  // flight_level_ft is stored alongside so renderSources() can re-evaluate
  // the FL filter whenever the user moves the slider (ASD-005).
  liveTrackFeatures: [],
  liveVectorFeatures: [],
  // ASD-005: active FL filter. minFL/maxFL are in FL units (e.g. 100 = FL100);
  // null means no limit. hide=true makes filtered tracks invisible; false dims them.
  flFilter: { minFL: null, maxFL: null, hide: false },
  // ASD-002 B2: per-track manual label-position overrides set by Drag&Drop.
  // Map<track_num, {dx, dy}> in screen pixels relative to the symbol centre.
  labelPins: new Map(),
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
    // Track layers from bottom to top: trail line → history dots → speed
    // vectors → leader lines → track symbols → deconflicted labels (ASD-002).
    addTrailsLayer(map);
    addHistoryDotsLayer(map);
    addVectorsLayer(map);
    addLeaderLinesLayer(map); // ASD-002: under track circles
    addTracksLayer(map);
    addLabelsLayer(map);      // ASD-002: above track circles
    state.mapLoaded = true;
    if (state.pendingTracks) {
      updateTracksLayer(state.pendingTracks);
      state.pendingTracks = null;
    }

    // Load aeronautical data and refresh it periodically.
    loadAeronautical(map);
    setInterval(() => loadAeronautical(map), AERO_REFRESH_MS);

    setupLayerControl(map);
    setupFlFilter();
    setupLabelDrag(map); // ASD-002 B2: Drag&Drop label pinning

    // ASD-002: re-deconflict label positions whenever the viewport moves, so
    // labels correctly track their symbols during pan and zoom. Throttled to
    // one deconfliction per animation frame to avoid redundant work.
    let deconflictFrame = null;
    map.on("move", () => {
      if (deconflictFrame) return;
      deconflictFrame = requestAnimationFrame(() => {
        deconflictFrame = null;
        if (state.mapLoaded) renderSources();
      });
    });
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
// ASD-004b/4c: circle-opacity and text-opacity use data-driven expressions to
// dim coasting tracks (status uncertain) and fade TSE tracks to transparency.
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
        ["get", "filtered"],
        "#455a64", // blue-grey: outside FL filter range (ASD-005)
        ["get", "coasting"],
        "#ff9800", // orange: coasting (no recent update)
        ["get", "confirmed"],
        "#4caf50", // green: confirmed track
        "#9e9e9e", // grey: tentative track
      ],
      "circle-stroke-width": 1,
      "circle-stroke-color": state.palette.symbolStroke,
      "circle-opacity": [
        "case",
        ["has", "fade_opacity"], ["get", "fade_opacity"],
        ["has", "fl_opacity"],   ["get", "fl_opacity"],
        ["get", "coasting"], 0.5,
        1.0,
      ],
    },
  });

}

// addLeaderLinesLayer registers the GeoJSON source and line layer for ASD-002
// leader lines — thin lines from each track symbol to its deconflicted data-block
// anchor. Registered before addTracksLayer so lines render behind the dots.
function addLeaderLinesLayer(map) {
  map.addSource(LEADER_LINES_SOURCE_ID, {
    type: "geojson",
    data: { type: "FeatureCollection", features: [] },
  });
  map.addLayer({
    id: LEADER_LINES_LAYER_ID,
    type: "line",
    source: LEADER_LINES_SOURCE_ID,
    paint: {
      "line-color": state.palette.label,
      "line-width": 0.7,
      "line-opacity": [
        "case",
        ["has", "fade_opacity"], ["get", "fade_opacity"],
        ["has", "fl_opacity"],   ["get", "fl_opacity"],
        ["get", "coasting"], 0.3,
        0.55,
      ],
    },
  });
}

// addLabelsLayer registers the GeoJSON source and symbol layer for ASD-002
// deconflicted data-block labels. Label geo positions are computed in screen
// space by deconflictLabels() and pushed here on every render. Setting
// text-allow-overlap:true means MapLibre's placement engine never hides a
// label — our deconfliction engine is solely responsible for placement quality.
function addLabelsLayer(map) {
  map.addSource(LABELS_SOURCE_ID, {
    type: "geojson",
    data: { type: "FeatureCollection", features: [] },
  });
  map.addLayer({
    id: LABELS_LAYER_ID,
    type: "symbol",
    source: LABELS_SOURCE_ID,
    layout: {
      "text-field": ["get", "label"],
      "text-size": 11,
      "text-anchor": "center",
      "text-allow-overlap": true,
      "text-ignore-placement": true,
    },
    paint: {
      "text-color": state.palette.label,
      "text-halo-color": state.palette.labelHalo,
      "text-halo-width": 1,
      "text-opacity": [
        "case",
        ["has", "fade_opacity"], ["get", "fade_opacity"],
        ["has", "fl_opacity"],   ["get", "fl_opacity"],
        ["get", "coasting"], 0.35,
        1.0,
      ],
    },
  });
}

// bboxCollides returns true when bbox overlaps any rectangle in occupied.
function bboxCollides(occupied, bbox) {
  for (const o of occupied) {
    if (bbox.x1 < o.x2 && bbox.x2 > o.x1 && bbox.y1 < o.y2 && bbox.y2 > o.y1) {
      return true;
    }
  }
  return false;
}

// deconflictLabels computes deconflicted screen-space positions for all track
// data-block labels (ASD-002 B1) and returns GeoJSON features for both the
// labels and their leader lines.
//
// Algorithm — greedy in track_num order (deterministic):
//   For each track, try LABEL_SLOTS in order and pick the first slot whose
//   bounding box does not overlap already-placed labels or OTHER tracks' symbols.
//   The current track's own symbol is intentionally excluded from the collision
//   check so a label can sit right next to its dot — the leader line makes the
//   symbol↔block association explicit. If all 8 slots collide, slot 0 is used
//   as a guaranteed fallback: no label is ever hidden.
//
// Manual pins from state.labelPins (ASD-002 B2) override auto-placement.
function deconflictLabels(allTrackFeatures) {
  const map = state.map;
  const symbolOccupied = []; // circle footprints of already-processed tracks
  const labelOccupied = [];  // bounding boxes of already-placed labels

  const sorted = [...allTrackFeatures].sort(
    (a, b) => a.properties.track_num - b.properties.track_num,
  );

  const labelFeatures = [];
  const leaderLineFeatures = [];

  for (const feature of sorted) {
    const [lon, lat] = feature.geometry.coordinates;
    const trackNum = feature.properties.track_num;
    const sym = map.project([lon, lat]);

    let lx, ly;

    if (state.labelPins.has(trackNum)) {
      // B2: manual pin overrides auto-placement.
      const pin = state.labelPins.get(trackNum);
      lx = sym.x + pin.dx;
      ly = sym.y + pin.dy;
    } else {
      // B1: greedy slot search, excluding own symbol from collision set.
      lx = null;
      for (const [ux, uy] of LABEL_SLOTS) {
        const cx = sym.x + ux * LABEL_SLOT_RADIUS_PX;
        const cy = sym.y + uy * LABEL_SLOT_RADIUS_PX;
        const bbox = {
          x1: cx - LABEL_W_PX / 2,
          y1: cy - LABEL_H_PX / 2,
          x2: cx + LABEL_W_PX / 2,
          y2: cy + LABEL_H_PX / 2,
        };
        if (!bboxCollides(symbolOccupied, bbox) && !bboxCollides(labelOccupied, bbox)) {
          lx = cx;
          ly = cy;
          break;
        }
      }
      // Fallback: slot 0, even if colliding — a label is never suppressed.
      if (lx === null) {
        lx = sym.x + LABEL_SLOTS[0][0] * LABEL_SLOT_RADIUS_PX;
        ly = sym.y + LABEL_SLOTS[0][1] * LABEL_SLOT_RADIUS_PX;
      }
    }

    // Register this track's symbol and placed label for subsequent iterations.
    symbolOccupied.push({
      x1: sym.x - SYMBOL_BBOX_R_PX,
      y1: sym.y - SYMBOL_BBOX_R_PX,
      x2: sym.x + SYMBOL_BBOX_R_PX,
      y2: sym.y + SYMBOL_BBOX_R_PX,
    });
    labelOccupied.push({
      x1: lx - LABEL_W_PX / 2,
      y1: ly - LABEL_H_PX / 2,
      x2: lx + LABEL_W_PX / 2,
      y2: ly + LABEL_H_PX / 2,
    });

    const labelLngLat = map.unproject([lx, ly]);

    // Carry opacity side-car properties so label paint expressions work.
    const opProps = {};
    if (feature.properties.fade_opacity !== undefined) opProps.fade_opacity = feature.properties.fade_opacity;
    if (feature.properties.fl_opacity !== undefined) opProps.fl_opacity = feature.properties.fl_opacity;

    labelFeatures.push({
      type: "Feature",
      geometry: { type: "Point", coordinates: [labelLngLat.lng, labelLngLat.lat] },
      properties: {
        track_num: trackNum,
        label: feature.properties.label,
        coasting: feature.properties.coasting,
        ...opProps,
      },
    });

    // Leader line: always drawn when label is offset from its symbol, to make
    // the symbol↔block association unambiguous in dense traffic (ATC convention).
    if (Math.hypot(lx - sym.x, ly - sym.y) > LEADER_THRESHOLD_PX) {
      leaderLineFeatures.push({
        type: "Feature",
        geometry: {
          type: "LineString",
          coordinates: [
            [lon, lat],
            [labelLngLat.lng, labelLngLat.lat],
          ],
        },
        properties: {
          track_num: trackNum,
          coasting: feature.properties.coasting,
          ...opProps,
        },
      });
    }
  }

  return { labelFeatures, leaderLineFeatures };
}

// setupLabelDrag wires ASD-002 B2 Drag&Drop label pinning.
//   mousedown on a label → disable map pan, begin drag
//   mousemove            → update state.labelPins and re-render in real time
//   mouseup              → commit pin, re-enable map pan
//   dblclick on label    → delete pin, revert to auto-deconflicted placement
function setupLabelDrag(map) {
  let drag = null;

  map.on("mouseenter", LABELS_LAYER_ID, () => {
    if (!drag) map.getCanvas().style.cursor = "move";
  });
  map.on("mouseleave", LABELS_LAYER_ID, () => {
    if (!drag) map.getCanvas().style.cursor = "";
  });

  map.on("mousedown", LABELS_LAYER_ID, (e) => {
    e.preventDefault();
    const feat = (map.queryRenderedFeatures(e.point, { layers: [LABELS_LAYER_ID] }) || [])[0];
    if (!feat) return;
    const trackNum = feat.properties.track_num;

    // Find the track's SYMBOL position (geo), not the label's position.
    const trackFeature =
      state.liveTrackFeatures.find((f) => f.properties.track_num === trackNum) ||
      (() => {
        const fd = state.fadingTracks.get(trackNum);
        return fd ? { geometry: { coordinates: [fd.track.longitude, fd.track.latitude] } } : null;
      })();
    if (!trackFeature) return;

    const [lon, lat] = trackFeature.geometry.coordinates;
    const sym = map.project([lon, lat]);

    // If already pinned, use existing offset as the drag start point.
    const currentPin = state.labelPins.get(trackNum) ?? {
      dx: e.point.x - sym.x,
      dy: e.point.y - sym.y,
    };

    drag = {
      trackNum,
      sym,
      startMouse: { x: e.point.x, y: e.point.y },
      startPin: currentPin,
    };
    map.dragPan.disable();

    const onMove = (moveE) => {
      const dx = drag.startPin.dx + (moveE.point.x - drag.startMouse.x);
      const dy = drag.startPin.dy + (moveE.point.y - drag.startMouse.y);
      state.labelPins.set(drag.trackNum, { dx, dy });
      renderSources();
    };

    const onUp = () => {
      drag = null;
      map.dragPan.enable();
      map.getCanvas().style.cursor = "";
      map.off("mousemove", onMove);
      map.off("mouseup", onUp);
    };

    map.on("mousemove", onMove);
    map.on("mouseup", onUp);
  });

  // Double-click clears the pin and returns the label to auto-placement.
  map.on("dblclick", LABELS_LAYER_ID, (e) => {
    e.preventDefault();
    const feat = (map.queryRenderedFeatures(e.point, { layers: [LABELS_LAYER_ID] }) || [])[0];
    if (!feat) return;
    state.labelPins.delete(feat.properties.track_num);
    renderSources();
  });
}

// addTrailsLayer registers a GeoJSON source and a line layer for rendering
// each track's recent flight path (a fading trail of its last positions).
// Added first so trails draw beneath the history dots, speed vectors and track
// symbols. ASD-004b/4c: line-opacity dims coasting trails and fades TSE trails.
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
      "line-opacity": [
        "case",
        ["has", "fade_opacity"], ["*", 0.6, ["get", "fade_opacity"]],
        ["has", "fl_opacity"],   ["get", "fl_opacity"],
        ["get", "coasting"], 0.2,
        0.6,
      ],
    },
  });
}

// addHistoryDotsLayer registers a GeoJSON source and a circle layer for
// rendering each past position in a track's history as a discrete dot (ASD-004a).
// On a real radar scope, the spacing between dots encodes instantaneous speed
// and the curvature encodes turn rate — information lost in a continuous line.
// ASD-004b/4c: circle-opacity dims coasting dots and fades TSE dots.
function addHistoryDotsLayer(map) {
  map.addSource(HISTORY_DOTS_SOURCE_ID, {
    type: "geojson",
    data: { type: "FeatureCollection", features: [] },
  });

  map.addLayer({
    id: HISTORY_DOTS_LAYER_ID,
    type: "circle",
    source: HISTORY_DOTS_SOURCE_ID,
    paint: {
      "circle-radius": 2,
      "circle-color": state.palette.trail,
      "circle-opacity": [
        "case",
        ["has", "fade_opacity"], ["*", 0.6, ["get", "fade_opacity"]],
        ["has", "fl_opacity"],   ["get", "fl_opacity"],
        ["get", "coasting"], 0.2,
        0.6,
      ],
    },
  });
}

// addVectorsLayer registers a GeoJSON source and a line layer for rendering
// each track's speed vector (a short line from the current position towards
// where the track will be in VECTOR_LOOKAHEAD_S seconds, ASD-style SVL).
// Added before the tracks layer so the track symbols draw on top.
// ASD-004b/4c: line-opacity dims coasting vectors and fades TSE vectors.
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
      "line-opacity": [
        "case",
        ["has", "fade_opacity"], ["get", "fade_opacity"],
        ["has", "fl_opacity"],   ["get", "fl_opacity"],
        ["get", "coasting"], 0.35,
        1.0,
      ],
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

// buildLabel produces the track's ASD data-block label (ASD-001).
//   Line 1: callsign (I062/245) or track number as fallback.
//   Line 2: "FLnnn" (flight level, I062/136) + vertical-tendency indicator
//            (▲ climbing / ▼ descending / empty for level), when FL is known.
//   Line 3: ground speed in knots (from Vx/Vy, I062/185), when non-zero.
// vTrend is "▲", "▼", or "" — computed by updateTracksLayer (ASD-001b).
function buildLabel(track, vTrend) {
  const line1 =
    typeof track.callsign === "string" && track.callsign !== ""
      ? track.callsign
      : String(track.track_num);

  // Ground speed: sqrt(Vx²+Vy²) m/s → kt (1 m/s ≈ 1.9438 kt).
  const gs = Math.round(Math.hypot(track.vx, track.vy) * 1.9438);
  const gsLine = gs > 0 ? `\n${gs}` : "";

  if (typeof track.flight_level_ft === "number") {
    const fl = Math.round(track.flight_level_ft / 100);
    const trend = vTrend ? ` ${vTrend}` : "";
    return `${line1}\nFL${String(fl).padStart(3, "0")}${trend}${gsLine}`;
  }
  return `${line1}${gsLine}`;
}

// isFlFiltered returns true when a known flight level falls outside the active
// FL filter window (ASD-005). Tracks with unknown FL always pass through the
// filter — hiding unknown-altitude traffic would be operationally unsafe.
function isFlFiltered(flightLevelFt) {
  if (typeof flightLevelFt !== "number") return false;
  const fl = Math.round(flightLevelFt / 100);
  const { minFL, maxFL } = state.flFilter;
  if (minFL !== null && fl < minFL) return true;
  if (maxFL !== null && fl > maxFL) return true;
  return false;
}

// flOpacity returns the fl_opacity value to attach to a filtered feature, or
// undefined when the feature passes the filter. hide=true → 0 (invisible);
// hide=false → 0.15 (entsättigt / heavily dimmed).
function flOpacity(flightLevelFt) {
  if (!isFlFiltered(flightLevelFt)) return undefined;
  return state.flFilter.hide ? 0.0 : 0.15;
}

// setupFlFilter wires the FL-filter panel (ASD-005) so slider/checkbox changes
// immediately re-render all map sources without waiting for a new WSS update.
function setupFlFilter() {
  const minInput = document.getElementById("fl-min");
  const maxInput = document.getElementById("fl-max");
  const hideCheck = document.getElementById("fl-hide");
  if (!minInput || !maxInput || !hideCheck) return;
  const apply = () => {
    state.flFilter.minFL = minInput.value !== "" ? parseInt(minInput.value, 10) : null;
    state.flFilter.maxFL = maxInput.value !== "" ? parseInt(maxInput.value, 10) : null;
    state.flFilter.hide = hideCheck.checked;
    if (state.mapLoaded) renderSources();
  };
  minInput.addEventListener("input", apply);
  maxInput.addEventListener("input", apply);
  hideCheck.addEventListener("change", apply);
}

// updateTrackHistory appends each track's current position to its trail
// history (capped at TRAIL_MAX_POINTS) and drops history for tracks that are
// no longer present — but keeps history alive for tracks currently fading out
// (ASD-004c), so their trail and dots remain visible during the fade.
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
    if (!seen.has(trackNum) && !state.fadingTracks.has(trackNum)) {
      state.trackHistory.delete(trackNum);
    }
  }
}

// updateFeedBanner reflects the CAT065 feed-health state (Firefly ADR 0018)
// in the top-right banner: green "FEED OK", red "FEED STALE" (heartbeat lost),
// or grey "FEED ?" until the first heartbeat arrives.
function updateFeedBanner(feedStatus) {
  const el = document.getElementById("feed-status");
  if (!el) {
    return;
  }
  const s = feedStatus.state;
  el.className = s;
  if (s === "ok") {
    el.textContent = "● FEED OK";
  } else if (s === "stale") {
    el.textContent = "▲ FEED STALE — kein Heartbeat";
  } else {
    el.textContent = "● FEED ?";
  }
}

// updateTracksLayer processes a WebSocket message (see pkg/broadcast.Message):
// it routes TSE tracks into the fade-out map (ASD-004c), computes per-track
// vertical tendency and labels (ASD-001), builds live GeoJSON features, and
// kicks off the fade-animation loop when needed.
function updateTracksLayer(msg) {
  // TSE (Track-Service-End) tracks: register them for a graceful fade-out
  // (ASD-004c) instead of removing them instantly. Only the first TSE for a
  // given track_num sets the deadline; duplicates are ignored.
  (msg.tracks || [])
    .filter((t) => t.ended)
    .forEach((t) => {
      if (!state.fadingTracks.has(t.track_num)) {
        state.fadingTracks.set(t.track_num, {
          deadline: Date.now() + FADE_DURATION_MS,
          track: t,
        });
      }
    });

  const tracks = (msg.tracks || []).filter((t) => !t.ended);

  // A track_num reappearing in the live stream (resurrection) must be evicted
  // from the fading map so it does not render with a stale fade_opacity.
  tracks.forEach((t) => state.fadingTracks.delete(t.track_num));

  updateTrackHistory(tracks);

  // Build the set of track_nums that need ongoing state (live + fading).
  const liveNums = new Set(tracks.map((t) => t.track_num));
  for (const num of state.trackFlHistory.keys()) {
    if (!liveNums.has(num) && !state.fadingTracks.has(num)) {
      state.trackFlHistory.delete(num);
    }
  }
  for (const num of state.trackCoasting.keys()) {
    if (!liveNums.has(num) && !state.fadingTracks.has(num)) {
      state.trackCoasting.delete(num);
    }
  }

  // Precompute live track GeoJSON features. Vertical-tendency (ASD-001b) is
  // computed here — comparing current FL to the previously stored value — and
  // the result is baked into the label string so renderSources() can reuse it
  // without recalculating on every fade-loop tick.
  state.liveTrackFeatures = tracks.map((track) => {
    let vTrend = "";
    if (typeof track.flight_level_ft === "number") {
      const prevFl = state.trackFlHistory.get(track.track_num);
      if (typeof prevFl === "number") {
        const delta = track.flight_level_ft - prevFl;
        if (delta > 50) vTrend = "▲";
        else if (delta < -50) vTrend = "▼";
      }
      state.trackFlHistory.set(track.track_num, track.flight_level_ft);
    }
    state.trackCoasting.set(track.track_num, track.coasting);
    return {
      type: "Feature",
      geometry: { type: "Point", coordinates: [track.longitude, track.latitude] },
      properties: {
        track_num: track.track_num,
        confirmed: track.confirmed,
        coasting: track.coasting,
        vx: track.vx,
        vy: track.vy,
        label: buildLabel(track, vTrend),
        // Stored so renderSources() can re-evaluate the FL filter on UI change
        // (ASD-005) without waiting for a new WebSocket update.
        flight_level_ft: typeof track.flight_level_ft === "number" ? track.flight_level_ft : null,
      },
    };
  });

  state.liveVectorFeatures = tracks.map((track) => ({
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
      coasting: track.coasting,
    },
  }));

  renderSources();

  // Start the fade-animation loop if there are fading tracks and it is not
  // already running. tickFade() stops the interval when all tracks expire.
  if (state.fadingTracks.size > 0 && !state.fadeInterval) {
    state.fadeInterval = setInterval(tickFade, 50);
  }
}

// renderSources pushes the current air picture into all four GeoJSON map
// sources: track symbols/labels, speed vectors, history dots, and trails.
// It merges live features with any currently fading tracks, attaching a
// fade_opacity property (0–1) that the paint expressions use for opacity.
// Called on every WebSocket update and on every fade-loop tick.
function renderSources() {
  const now = Date.now();

  // Live-track features: re-evaluate FL filter (ASD-005) each render call so
  // a slider change takes effect immediately, not only on the next WSS update.
  const liveTrackFeatures = state.liveTrackFeatures.map((f) => {
    const flFt = f.properties.flight_level_ft;
    const filtered = isFlFiltered(flFt);
    const flOp = flOpacity(flFt);
    const props = { ...f.properties, filtered };
    if (flOp !== undefined) props.fl_opacity = flOp;
    return { ...f, properties: props };
  });

  // Fading-track features: same shape as live features but carry fade_opacity.
  // FL filter is also applied so that a filtering track fades out invisibly.
  const fadingTrackFeatures = [];
  const fadingVectorFeatures = [];
  for (const [, { deadline, track }] of state.fadingTracks) {
    const fadeOpacity = Math.max(0, (deadline - now) / FADE_DURATION_MS);
    const flOp = flOpacity(track.flight_level_ft);
    const trackProps = {
      track_num: track.track_num,
      confirmed: track.confirmed,
      coasting: track.coasting,
      vx: track.vx,
      vy: track.vy,
      label: buildLabel(track, ""),
      filtered: isFlFiltered(track.flight_level_ft),
      fade_opacity: fadeOpacity,
    };
    if (flOp !== undefined) trackProps.fl_opacity = flOp;
    fadingTrackFeatures.push({
      type: "Feature",
      geometry: { type: "Point", coordinates: [track.longitude, track.latitude] },
      properties: trackProps,
    });
    const vecProps = {
      track_num: track.track_num,
      coasting: track.coasting,
      fade_opacity: fadeOpacity,
    };
    if (flOp !== undefined) vecProps.fl_opacity = flOp;
    fadingVectorFeatures.push({
      type: "Feature",
      geometry: {
        type: "LineString",
        coordinates: [
          [track.longitude, track.latitude],
          vectorEndpoint(track.latitude, track.longitude, track.vx, track.vy),
        ],
      },
      properties: vecProps,
    });
  }

  // Live vector features also need FL filter re-evaluation.
  const liveVectorFeatures = state.liveVectorFeatures.map((f) => {
    const flFt = state.trackFlHistory.get(f.properties.track_num);
    const flOp = flOpacity(flFt);
    if (flOp === undefined) return f;
    return { ...f, properties: { ...f.properties, fl_opacity: flOp } };
  });

  state.map.getSource(TRACKS_SOURCE_ID).setData({
    type: "FeatureCollection",
    features: [...liveTrackFeatures, ...fadingTrackFeatures],
  });

  state.map.getSource(VECTORS_SOURCE_ID).setData({
    type: "FeatureCollection",
    features: [...liveVectorFeatures, ...fadingVectorFeatures],
  });

  // History dots (ASD-004a): one Point per entry in trackHistory. The coasting
  // flag comes from trackCoasting (updated in updateTracksLayer). Fading tracks
  // keep their history alive and carry fade_opacity so dots fade with the track.
  // ASD-005: fl_opacity is derived from the last known FL for this track.
  const dotsFeatures = [];
  for (const [trackNum, hist] of state.trackHistory) {
    const isCoasting = state.trackCoasting.get(trackNum) || false;
    const fadingEntry = state.fadingTracks.get(trackNum);
    const fadeOpacity = fadingEntry
      ? Math.max(0, (fadingEntry.deadline - now) / FADE_DURATION_MS)
      : undefined;
    const flFt = state.trackFlHistory.get(trackNum);
    const flOp = flOpacity(flFt);
    for (const coord of hist) {
      const props = { track_num: trackNum, coasting: isCoasting };
      if (fadeOpacity !== undefined) props.fade_opacity = fadeOpacity;
      if (flOp !== undefined) props.fl_opacity = flOp;
      dotsFeatures.push({
        type: "Feature",
        geometry: { type: "Point", coordinates: coord },
        properties: props,
      });
    }
  }

  state.map.getSource(HISTORY_DOTS_SOURCE_ID).setData({
    type: "FeatureCollection",
    features: dotsFeatures,
  });

  // Trails: one LineString per track, with coasting, fade_opacity and fl_opacity
  // for consistent dimming/fading/filtering across all layers (ASD-004/ASD-005).
  const trailFeatures = [];
  for (const [trackNum, hist] of state.trackHistory) {
    if (hist.length >= 2) {
      const isCoasting = state.trackCoasting.get(trackNum) || false;
      const fadingEntry = state.fadingTracks.get(trackNum);
      const flFt = state.trackFlHistory.get(trackNum);
      const flOp = flOpacity(flFt);
      const props = { track_num: trackNum, coasting: isCoasting };
      if (fadingEntry) {
        props.fade_opacity = Math.max(0, (fadingEntry.deadline - now) / FADE_DURATION_MS);
      }
      if (flOp !== undefined) props.fl_opacity = flOp;
      trailFeatures.push({
        type: "Feature",
        geometry: { type: "LineString", coordinates: hist },
        properties: props,
      });
    }
  }

  state.map.getSource(TRAILS_SOURCE_ID).setData({
    type: "FeatureCollection",
    features: trailFeatures,
  });

  // ASD-002: deconflict label positions in screen space and push to the
  // dedicated label + leader-line sources. Labels never disappear — the
  // greedy algorithm always places every label in the least-colliding slot.
  const { labelFeatures, leaderLineFeatures } = deconflictLabels([
    ...liveTrackFeatures,
    ...fadingTrackFeatures,
  ]);
  state.map.getSource(LABELS_SOURCE_ID).setData({
    type: "FeatureCollection",
    features: labelFeatures,
  });
  state.map.getSource(LEADER_LINES_SOURCE_ID).setData({
    type: "FeatureCollection",
    features: leaderLineFeatures,
  });
}

// tickFade advances the TSE fade-out animation (ASD-004c). Runs every 50 ms
// while fadingTracks is non-empty. Expired tracks are evicted from all state
// maps; the interval clears itself when all fading tracks have disappeared.
function tickFade() {
  const now = Date.now();
  for (const [num, { deadline }] of state.fadingTracks) {
    if (now >= deadline) {
      state.fadingTracks.delete(num);
      state.trackHistory.delete(num);
      state.trackCoasting.delete(num);
      state.labelPins.delete(num); // ASD-002: drop pin for expired track
    }
  }

  if (state.fadingTracks.size === 0) {
    clearInterval(state.fadeInterval);
    state.fadeInterval = null;
  }

  renderSources();
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
