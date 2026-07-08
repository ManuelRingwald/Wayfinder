// Map layer registration functions for the ASD scope.
// Each function adds a GeoJSON source and one or more MapLibre layers.
// All functions take `map` and `palette` as explicit parameters so there is
// no global state dependency.
import {
  TRACKS_SOURCE_ID,
  TRACKS_LAYER_ID,
  VECTORS_SOURCE_ID,
  VECTORS_LAYER_ID,
  TRAILS_SOURCE_ID,
  TRAILS_LAYER_ID,
  HISTORY_DOTS_SOURCE_ID,
  HISTORY_DOTS_LAYER_ID,
  AIRSPACE_SOURCE_ID,
  AIRSPACE_FILL_LAYER_ID,
  AIRSPACE_LINE_LAYER_ID,
  AIRSPACE_LABEL_LAYER_ID,
  AIRSPACE_AOR_LAYER_ID,
  AIRSPACE_AOR_COLOR,
  NAVAIDS_SOURCE_ID,
  NAVAIDS_LAYER_ID,
  WAYPOINTS_SOURCE_ID,
  WAYPOINTS_LAYER_ID,
  AIRPORT_SOURCE_ID,
  AIRPORT_LAYER_ID,
  AIRPORT_LABEL_LAYER_ID,
  RUNWAY_SOURCE_ID,
  RUNWAY_LAYER_ID,
  LABELS_SOURCE_ID,
  LABELS_LAYER_ID,
  LEADER_LINES_SOURCE_ID,
  LEADER_LINES_LAYER_ID,
  SELECTION_SOURCE_ID,
  SELECTION_LAYER_ID,
  SELECTION_ICON_ID,
  SELECTION_LABEL_SOURCE_ID,
  SELECTION_LABEL_LAYER_ID,
  SELECTION_LABEL_COLOR,
  SELECTION_LABEL_WIDTH_PX,
  LABEL_TEXT_SIZE,
  TRACK_STATE_COLORS,
  AIRSPACE_GROUPS,
  COVERAGE_SOURCE_ID,
  COVERAGE_RINGS_LAYER_ID,
  COVERAGE_CENTER_LAYER_ID,
  RANGE_RINGS_SOURCE_ID,
  RANGE_RINGS_LAYER_ID,
  RANGE_RINGS_LABEL_LAYER_ID,
  WEATHER_RADAR_SOURCE_ID,
  WEATHER_RADAR_LAYER_ID,
  WEATHER_RADAR_TILES_URL,
  WEATHER_RADAR_OPACITY,
  DWD_ATTRIBUTION,
  WEATHER_WARNINGS_SOURCE_ID,
  WEATHER_WARNINGS_FILL_LAYER_ID,
  WEATHER_WARNINGS_LINE_LAYER_ID,
  WEATHER_WARNINGS_COLORS,
} from './constants.js'

// severityColorExpr builds a MapLibre 'match' on the normalised wf_level
// (1..4), falling back to the moderate colour for any unexpected value.
function severityColorExpr() {
  return [
    'match',
    ['get', 'wf_level'],
    1, WEATHER_WARNINGS_COLORS[1],
    2, WEATHER_WARNINGS_COLORS[2],
    3, WEATHER_WARNINGS_COLORS[3],
    4, WEATHER_WARNINGS_COLORS[4],
    WEATHER_WARNINGS_COLORS[2],
  ]
}

// Build a MapLibre 'match' expression keyed on the OpenAIP numeric type field.
// Each AIRSPACE_GROUP contributes one arm (array label → single value).
// Unknown types fall back to `fallback`.
function airspaceMatchExpr(prop, fallback) {
  const expr = ['match', ['get', 'type']]
  for (const g of AIRSPACE_GROUPS) {
    expr.push(g.types, g[prop])
  }
  expr.push(fallback)
  return expr
}

// makeIconImage renders a small icon onto an offscreen canvas and returns its
// ImageData, so we need no external sprite assets (keeps Wayfinder a single
// self-contained binary). draw(ctx, size) paints into a size×size square.
// Icons are registered at pixelRatio 2 (addImage below), so an N-px canvas lays
// out at N/2 CSS px and every canvas coordinate/stroke halves on screen. Track
// symbols pass size=32 (a 12-CSS-px diamond needs ±12 canvas coords plus stroke
// headroom); the smaller aeronautical marks keep the 24-px default.
function makeIconImage(draw, size = 24) {
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d')
  draw(ctx, size)
  return ctx.getImageData(0, 0, size, size)
}

// addAeronauticalIcons registers the navaid/waypoint marker icons: a triangle
// for waypoints, a compass-rose ring for VOR-family navaids, and a
// dashed/dotted ring for NDBs. Colours are chosen to read on the dark scope.
export function addAeronauticalIcons(map) {
  const add = (id, image) => {
    if (!map.hasImage(id)) {
      map.addImage(id, image, { pixelRatio: 2 })
    }
  }

  add(
    'wf-waypoint',
    makeIconImage((ctx, s) => {
      const c = s / 2
      ctx.strokeStyle = '#4dd0e1'
      ctx.lineWidth = 2
      ctx.beginPath()
      ctx.moveTo(c, c - 7)
      ctx.lineTo(c + 6, c + 5)
      ctx.lineTo(c - 6, c + 5)
      ctx.closePath()
      ctx.stroke()
    }),
  )

  add(
    'wf-vor',
    makeIconImage((ctx, s) => {
      const c = s / 2
      ctx.strokeStyle = '#80cbc4'
      ctx.lineWidth = 2
      ctx.beginPath()
      ctx.arc(c, c, 7, 0, 2 * Math.PI)
      ctx.stroke()
      // compass-rose ticks
      for (let i = 0; i < 8; i++) {
        const a = (i * Math.PI) / 4
        ctx.beginPath()
        ctx.moveTo(c + Math.cos(a) * 7, c + Math.sin(a) * 7)
        ctx.lineTo(c + Math.cos(a) * 10, c + Math.sin(a) * 10)
        ctx.stroke()
      }
    }),
  )

  add(
    'wf-ndb',
    makeIconImage((ctx, s) => {
      const c = s / 2
      ctx.strokeStyle = '#ffb74d'
      ctx.lineWidth = 2
      ctx.setLineDash([2, 2])
      ctx.beginPath()
      ctx.arc(c, c, 7, 0, 2 * Math.PI)
      ctx.stroke()
      ctx.setLineDash([])
      ctx.fillStyle = '#ffb74d'
      ctx.beginPath()
      ctx.arc(c, c, 1.6, 0, 2 * Math.PI)
      ctx.fill()
    }),
  )

  add(
    'wf-navaid',
    makeIconImage((ctx, s) => {
      const c = s / 2
      ctx.strokeStyle = '#b0bec5'
      ctx.lineWidth = 2
      ctx.beginPath()
      ctx.arc(c, c, 6, 0, 2 * Math.PI)
      ctx.stroke()
    }),
  )
}

// WF2-40/#119: Provenance track symbols. The GLYPH encodes the surveillance
// source (A = ADS-B, F = FLARM, ▢ SSR/Mode S, ○ primary/PSR); the fill (or
// ring/letter) COLOUR encodes the track state — the same colours the old
// circle layer used, so no state information is lost. Letter glyphs (#119)
// make the cooperative self-report sources directly readable on the scope.
// Icons are pre-rendered per (shape × state) combination and selected at
// runtime by a data-driven icon-image expression, which avoids the
// antialiasing pitfalls of tinting a single SDF icon.
const TRACK_ICON_STROKE = '#000000' // dark edge for legibility on both bases

// makeTrackIcon paints one provenance symbol in the given state colour. Shape
// encodes the surveillance source per the design legend: ADS-B a diamond ◆, SSR
// a filled square ■ (cooperative reply, carries identity), PSR a HOLLOW ring ○
// (raw skin paint, no ID), FLARM an upward triangle ▲ (#185). Combined stays a
// letter glyph (K) — the multi-sensor superset source beyond the design legend.
//
// Geometry mirrors the design template (scope-tracks.jsx symbolNode, s=5):
// diamond 12 CSS px point-to-point, square 8 CSS px side, circle 9 CSS px dia,
// filled edge 1 CSS px, hollow outline 1.7 CSS px. Because the icon is
// registered at pixelRatio 2 on a 32-px canvas, every value here is the template
// CSS pixel × 2 (see makeIconImage). Earlier icons were drawn on a 24-px canvas,
// which capped the footprint at 12 CSS px and rendered the symbols ~40% too
// small; the 32-px canvas gives the enlarged shapes their stroke headroom.
//
// When `hollow` (the coasting state), non-PSR symbols are stroked as an OUTLINE
// with no fill, so a coasting track is readable from its SHAPE, not the colour
// alone (design legend "Coasting (hohl)"). PSR is a special case: it is ALWAYS a
// hollow ring — its state reads from the ring COLOUR, never from a fill.
function makeTrackIcon(shape, color, hollow) {
  return makeIconImage((ctx, s) => {
    const c = s / 2
    ctx.lineJoin = 'round'
    // strokeOrFill paints the current path either hollow (coloured outline, no
    // fill — the coasting look) or solid (colour fill + dark edge for legibility).
    const strokeOrFill = () => {
      if (hollow) {
        ctx.strokeStyle = color
        ctx.lineWidth = 3.4 // 1.7 CSS px
        ctx.stroke()
      } else {
        ctx.fillStyle = color
        ctx.fill()
        ctx.strokeStyle = TRACK_ICON_STROKE
        ctx.lineWidth = 2 // 1 CSS px
        ctx.stroke()
      }
    }
    if (shape === 'psr') {
      // PSR is always an open ring (design template: the PSR branch ignores the
      // fill channel). r = 4.5 CSS px, stroke 2 CSS px, in every track state.
      ctx.beginPath()
      ctx.arc(c, c, 9, 0, 2 * Math.PI)
      ctx.strokeStyle = color
      ctx.lineWidth = 4 // 2 CSS px
      ctx.stroke()
      return
    }
    if (shape === 'ssr') {
      ctx.beginPath()
      ctx.rect(c - 8, c - 8, 16, 16) // 8 CSS px side
      strokeOrFill()
      return
    }
    if (shape === 'adsb') {
      // Diamond (rotated square): ADS-B per the design legend. 12 CSS px
      // point-to-point (vertices at c ± 12 canvas).
      ctx.beginPath()
      ctx.moveTo(c, c - 12)
      ctx.lineTo(c + 12, c)
      ctx.lineTo(c, c + 12)
      ctx.lineTo(c - 12, c)
      ctx.closePath()
      strokeOrFill()
      return
    }
    if (shape === 'flarm') {
      // FLARM (#185): an upward triangle — the remaining basic geometric mark,
      // font-independent and consistent with "shape = provenance" (replacing the
      // earlier letter "F", which broke the geometric systematics). Sized to the
      // diamond's footprint (apex at c-12, base at c+10). The vertical-tendency
      // arrows ▲/▼ (ASD-001b) live in the data block, not on the symbol, so there
      // is no positional clash. Coasting => outline only (hollow convention).
      ctx.beginPath()
      ctx.moveTo(c, c - 12)
      ctx.lineTo(c + 11, c + 10)
      ctx.lineTo(c - 11, c + 10)
      ctx.closePath()
      strokeOrFill()
      return
    }
    // combined: letter glyph in the state colour (K = kombiniert/Mehr-Sensor
    // #125) — the multi-sensor superset source outside the 3-way design legend.
    // Coasting => outline the letter (no fill) to match the hollow convention.
    const letter = { combined: 'K' }[shape] ?? '?'
    ctx.font = 'bold 22px sans-serif'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    if (hollow) {
      ctx.strokeStyle = color
      ctx.lineWidth = 3
      ctx.strokeText(letter, c, c + 1)
    } else {
      ctx.strokeStyle = TRACK_ICON_STROKE
      ctx.lineWidth = 4
      ctx.strokeText(letter, c, c + 1)
      ctx.fillStyle = color
      ctx.fillText(letter, c, c + 1)
    }
  }, 32)
}

// addTrackIcons registers the 20 provenance×state track symbols (idempotent).
// Names follow `wf-trk-<provenance>-<stateKey>`, matched by the track layer's
// icon-image expression in addTracksLayer.
export function addTrackIcons(map) {
  for (const shape of ['adsb', 'flarm', 'combined', 'ssr', 'psr']) {
    for (const [stateKey, color] of Object.entries(TRACK_STATE_COLORS)) {
      const id = `wf-trk-${shape}-${stateKey}`
      if (!map.hasImage(id)) {
        // Coasting is drawn hollow (outline) so the state reads from the shape.
        map.addImage(id, makeTrackIcon(shape, color, stateKey === 'coasting'), { pixelRatio: 2 })
      }
    }
  }
}

// addWeatherRadarLayer registers the DWD weather-radar raster overlay (WX-A,
// ADR 0016). Tiles are proxied same-origin by Wayfinder from the DWD GeoServer
// WMS. It is added FIRST (before the aeronautical/track layers) so it sits above
// the base map but below every operational overlay; it starts hidden and is
// toggled via the sidebar (store.layerVisibility.weatherRadar). A raster source
// self-fetches, so there is no setData/refresh helper — the backend proxy caps
// the tile freshness to the DWD radar cadence (~5 min).
// #189: when the tenant has an AOI, the raster source is bounded to it so the
// radar is only fetched/rendered inside the sector (no country-wide extent). The
// bounds are a rectangle [west, south, east, north]; a null AOI leaves the layer
// unbounded. bboxToBounds converts the whoami AOI to that tuple.
function bboxToBounds(aoi) {
  if (!aoi) return undefined
  return [aoi.minLon, aoi.minLat, aoi.maxLon, aoi.maxLat]
}

export function addWeatherRadarLayer(map, aoi = null) {
  const bounds = bboxToBounds(aoi)
  map.addSource(WEATHER_RADAR_SOURCE_ID, {
    type: 'raster',
    tiles: [WEATHER_RADAR_TILES_URL],
    tileSize: 256,
    attribution: DWD_ATTRIBUTION,
    ...(bounds ? { bounds } : {}),
  })
  map.addLayer({
    id: WEATHER_RADAR_LAYER_ID,
    type: 'raster',
    source: WEATHER_RADAR_SOURCE_ID,
    layout: { visibility: 'none' },
    paint: { 'raster-opacity': WEATHER_RADAR_OPACITY },
  })
}

// setWeatherRadarAOI re-creates the radar raster source with new AOI bounds
// (#189). Called when the tenant's AOI resolves after mount or changes (e.g. an
// admin switching the impersonation target). Preserves the layer's current
// visibility. A raster source's bounds cannot be mutated in place, so the
// source+layer are removed and re-added.
export function setWeatherRadarAOI(map, aoi) {
  if (!map.getLayer(WEATHER_RADAR_LAYER_ID)) return
  const visibility = map.getLayoutProperty(WEATHER_RADAR_LAYER_ID, 'visibility') || 'none'
  map.removeLayer(WEATHER_RADAR_LAYER_ID)
  map.removeSource(WEATHER_RADAR_SOURCE_ID)
  addWeatherRadarLayer(map, aoi)
  map.setLayoutProperty(WEATHER_RADAR_LAYER_ID, 'visibility', visibility)
}

// addWeatherWarningsLayer registers the DWD weather-warnings overlay (WX-C,
// ADR 0016): a GeoJSON source (populated via updateWeatherWarnings from the
// backend-proxied WFS) with a translucent severity-coloured fill and a matching
// outline. Starts hidden; toggled via the sidebar (weather_warnings entitlement).
// Sits above the radar raster but below the aeronautical/track layers.
export function addWeatherWarningsLayer(map) {
  map.addSource(WEATHER_WARNINGS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
    attribution: DWD_ATTRIBUTION,
  })
  map.addLayer({
    id: WEATHER_WARNINGS_FILL_LAYER_ID,
    type: 'fill',
    source: WEATHER_WARNINGS_SOURCE_ID,
    layout: { visibility: 'none' },
    paint: { 'fill-color': severityColorExpr(), 'fill-opacity': 0.18 },
  })
  map.addLayer({
    id: WEATHER_WARNINGS_LINE_LAYER_ID,
    type: 'line',
    source: WEATHER_WARNINGS_SOURCE_ID,
    layout: { visibility: 'none' },
    paint: { 'line-color': severityColorExpr(), 'line-width': 1.2, 'line-opacity': 0.7 },
  })
}

// updateWeatherWarnings pushes a fetched warnings FeatureCollection into the
// source. A no-op if the source isn't present yet (map still loading).
export function updateWeatherWarnings(map, geojson) {
  const src = map.getSource(WEATHER_WARNINGS_SOURCE_ID)
  if (src) {
    src.setData(geojson || { type: 'FeatureCollection', features: [] })
  }
}

// addAirspaceLayers registers the airspace source and its fill/outline/label
// layers. ASD-011: paint expressions are type-driven via AIRSPACE_GROUPS so
// each category gets a distinct colour; the group filter is applied separately
// via updateAirspaceFilter() in engine.js.
export function addAirspaceLayers(map, palette) {
  map.addSource(AIRSPACE_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: AIRSPACE_FILL_LAYER_ID,
    type: 'fill',
    source: AIRSPACE_SOURCE_ID,
    filter: ['==', ['geometry-type'], 'Polygon'],
    paint: {
      'fill-color': airspaceMatchExpr('color', palette.airspaceLine),
      'fill-opacity': airspaceMatchExpr('fillOpacity', 0.06),
    },
  })

  map.addLayer({
    id: AIRSPACE_LINE_LAYER_ID,
    type: 'line',
    source: AIRSPACE_SOURCE_ID,
    paint: {
      'line-color': airspaceMatchExpr('color', palette.airspaceLine),
      'line-width': airspaceMatchExpr('lineWidth', 1.0),
      'line-opacity': 0.8,
    },
  })

  map.addLayer({
    id: AIRSPACE_LABEL_LAYER_ID,
    type: 'symbol',
    source: AIRSPACE_SOURCE_ID,
    minzoom: 6,
    layout: {
      'text-field': ['coalesce', ['get', 'name'], ''],
      'text-font': ['Roboto Mono Medium'],
      'text-size': 10,
      'symbol-placement': 'line',
    },
    paint: {
      'text-color': airspaceMatchExpr('color', palette.airspaceText),
      'text-halo-color': palette.aeroHalo,
      'text-halo-width': 1,
    },
  })
}

// addAirspaceAoRLayer registers the Area-of-Responsibility highlight (ASD-014,
// ADR 0021): a bright, emphasised outline over the airspace source that draws
// only the tenant's controlled volumes (CTR/TMA). It shares the airspace source
// (so it needs no separate fetch) and is filtered by feature id via updateAoR()
// in engine.js. The initial filter matches nothing (empty id list) until whoami
// supplies aor_airspace_ids. Added after the airspace line so it sits on top.
export function addAirspaceAoRLayer(map) {
  map.addLayer({
    id: AIRSPACE_AOR_LAYER_ID,
    type: 'line',
    source: AIRSPACE_SOURCE_ID,
    filter: aorFilter([]),
    paint: {
      'line-color': AIRSPACE_AOR_COLOR,
      'line-width': 3,
      'line-opacity': 0.95,
    },
  })
}

// aorFilter builds the MapLibre filter selecting airspace polygons whose stable
// OpenAIP `id` is in the given list. An empty list yields a filter that matches
// nothing (so no AoR configured → nothing highlighted).
export function aorFilter(ids) {
  return ['all', ['==', ['geometry-type'], 'Polygon'], ['in', ['get', 'id'], ['literal', ids ?? []]]]
}

// addNavaidLayers registers the navaids source and a symbol layer that picks an
// icon by navaid kind (VOR family / NDB / generic). A zoom floor keeps the
// scope uncluttered when zoomed far out.
export function addNavaidLayers(map, palette) {
  map.addSource(NAVAIDS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: NAVAIDS_LAYER_ID,
    type: 'symbol',
    source: NAVAIDS_SOURCE_ID,
    minzoom: 6,
    layout: {
      'icon-image': [
        'match',
        ['get', 'navaid_kind'],
        ['VOR', 'VOR-DME', 'VORTAC', 'DVOR', 'DVOR-DME', 'DVORTAC'],
        'wf-vor',
        'NDB',
        'wf-ndb',
        'wf-navaid',
      ],
      'icon-size': 1,
      'icon-allow-overlap': true,
      'text-field': ['coalesce', ['get', 'ident'], ['get', 'name'], ''],
      'text-font': ['Roboto Mono Medium'],
      'text-size': 10,
      'text-offset': [0, 1.1],
      'text-anchor': 'top',
    },
    paint: {
      'text-color': palette.airspaceText,
      'text-halo-color': palette.aeroHalo,
      'text-halo-width': 1,
    },
  })
}

// addWaypointLayers registers the waypoints source and its triangle-marker
// symbol layer, with a higher zoom floor (waypoints are denser than navaids).
export function addWaypointLayers(map, palette) {
  map.addSource(WAYPOINTS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: WAYPOINTS_LAYER_ID,
    type: 'symbol',
    source: WAYPOINTS_SOURCE_ID,
    minzoom: 7,
    layout: {
      'icon-image': 'wf-waypoint',
      'icon-size': 1,
      'icon-allow-overlap': false,
      'text-field': ['coalesce', ['get', 'name'], ''],
      'text-font': ['Roboto Mono Medium'],
      'text-size': 9,
      'text-offset': [0, 1.0],
      'text-anchor': 'top',
    },
    paint: {
      'text-color': palette.airspaceText,
      'text-halo-color': palette.aeroHalo,
      'text-halo-width': 1,
    },
  })
}

// addAirportLayers registers the #192 airport reference-point overlay: a small
// circle marker per aerodrome plus an ICAO label. Data is served AOI-scoped by
// /api/airports.geojson (offline OurAirports directory). Starts hidden; toggled
// via the sidebar (airport entitlement). A zoom floor keeps it uncluttered when
// zoomed far out.
export function addAirportLayers(map, palette) {
  map.addSource(AIRPORT_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: AIRPORT_LAYER_ID,
    type: 'circle',
    source: AIRPORT_SOURCE_ID,
    minzoom: 5,
    layout: { visibility: 'none' },
    paint: {
      'circle-radius': 3,
      'circle-color': palette.airspaceText,
      'circle-stroke-color': palette.aeroHalo,
      'circle-stroke-width': 1,
      'circle-opacity': 0.9,
    },
  })

  map.addLayer({
    id: AIRPORT_LABEL_LAYER_ID,
    type: 'symbol',
    source: AIRPORT_SOURCE_ID,
    minzoom: 6,
    layout: {
      visibility: 'none',
      'text-field': ['coalesce', ['get', 'icao'], ['get', 'name'], ''],
      'text-font': ['Roboto Mono Medium'],
      'text-size': 10,
      'text-offset': [0, 1.0],
      'text-anchor': 'top',
    },
    paint: {
      'text-color': palette.airspaceText,
      'text-halo-color': palette.aeroHalo,
      'text-halo-width': 1,
    },
  })
}

// updateAirportSource pushes a fetched airports FeatureCollection into the
// source. A no-op if the source isn't present yet (map still loading).
export function updateAirportSource(map, geojson) {
  const src = map.getSource(AIRPORT_SOURCE_ID)
  if (src) src.setData(geojson || { type: 'FeatureCollection', features: [] })
}

// addRunwayLayers registers the #192 runway-centreline overlay: one line per
// runway (LE→HE threshold), served AOI-scoped by /api/runways.geojson. Starts
// hidden; toggled via the sidebar (runways entitlement). A zoom floor keeps it
// off the far-out scope, where runways are sub-pixel anyway.
export function addRunwayLayers(map, palette) {
  map.addSource(RUNWAY_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: RUNWAY_LAYER_ID,
    type: 'line',
    source: RUNWAY_SOURCE_ID,
    minzoom: 8,
    layout: { visibility: 'none', 'line-cap': 'butt' },
    paint: {
      'line-color': palette.airspaceText,
      // Scale the runway line with zoom so it reads as a strip when zoomed in.
      'line-width': ['interpolate', ['linear'], ['zoom'], 8, 1.2, 12, 3, 15, 6],
      'line-opacity': 0.85,
    },
  })
}

// updateRunwaySource pushes a fetched runways FeatureCollection into the source.
// A no-op if the source isn't present yet (map still loading).
export function updateRunwaySource(map, geojson) {
  const src = map.getSource(RUNWAY_SOURCE_ID)
  if (src) src.setData(geojson || { type: 'FeatureCollection', features: [] })
}

// addTracksLayer registers a GeoJSON source and a symbol layer for rendering
// tracks. WF2-40/#119: the icon GLYPH encodes provenance (A ADS-B / F FLARM /
// ▢ SSR / ○ PSR) while the baked-in colour encodes track state (the old
// circle-color semantics). ASD-004b/4c: icon-opacity uses data-driven
// expressions to dim coasting tracks and fade TSE tracks to transparency.
export function addTracksLayer(map) {
  addTrackIcons(map)

  map.addSource(TRACKS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: TRACKS_LAYER_ID,
    type: 'symbol',
    source: TRACKS_SOURCE_ID,
    layout: {
      // Select the pre-rendered provenance×state icon. Shape = provenance;
      // colour follows the same precedence the old circle-color used
      // (filtered > coasting > confirmed > tentative). coalesce guards a
      // missing provenance property (defaults to the data-poorest source).
      'icon-image': [
        'concat',
        'wf-trk-',
        ['coalesce', ['get', 'provenance'], 'psr'],
        '-',
        [
          'case',
          ['get', 'filtered'], 'filtered',
          ['get', 'coasting'], 'coasting',
          ['get', 'confirmed'], 'confirmed',
          'tentative',
        ],
      ],
      'icon-size': 1,
      // Tracks are the air picture — never let symbol collision drop them.
      'icon-allow-overlap': true,
      'icon-ignore-placement': true,
    },
    paint: {
      // Opacity priority: fade > FL filter > normal. Coasting is no longer dimmed
      // here — the hollow symbol (makeTrackIcon) now carries that state, so a
      // coasting track stays at full opacity and reads crisply.
      'icon-opacity': [
        'case',
        ['has', 'fade_opacity'], ['get', 'fade_opacity'],
        ['has', 'fl_opacity'],   ['get', 'fl_opacity'],
        1.0,
      ],
    },
  })
}

// makeSelectionBox draws the ASD-007 selection marker as a square frame of four
// L-shaped corner brackets (ATC-scope convention, design ref EWG84F, #183) in the
// given colour. Corner brackets rather than a full square keep the track symbol
// and its data block readable. Drawn on a 32-px canvas at pixelRatio 2, so every
// value is the template CSS pixel × 2 (see makeIconImage / makeTrackIcon).
function makeSelectionBox(color) {
  return makeIconImage((ctx, s) => {
    const c = s / 2
    const half = 13 // box half-size (canvas px) — just outside the symbol footprint
    const arm = 6    // corner-bracket arm length
    ctx.strokeStyle = color
    ctx.lineWidth = 2 // 1 CSS px at pixelRatio 2
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    // Each corner: two arms running inward from the corner point.
    for (const [sx, sy] of [[-1, -1], [1, -1], [1, 1], [-1, 1]]) {
      const x = c + sx * half
      const y = c + sy * half
      ctx.beginPath()
      ctx.moveTo(x - sx * arm, y) // horizontal arm inward
      ctx.lineTo(x, y)
      ctx.lineTo(x, y - sy * arm) // vertical arm inward
      ctx.stroke()
    }
  }, 32)
}

// addSelectionLayer registers the ASD-007 selection marker: a cyan corner-bracket
// BOX around the currently selected track (#183, replacing the earlier ring — the
// box matches the ATC-scope look, design ref EWG84F). The source holds at most one
// Point (the selected track's live position, set by renderSources); registered
// before addTracksLayer so the box sits UNDER the symbol and the symbol stays
// crisp on top. The box is a pre-rendered icon in the selection colour.
export function addSelectionLayer(map, palette) {
  map.addSource(SELECTION_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })
  if (!map.hasImage(SELECTION_ICON_ID)) {
    map.addImage(SELECTION_ICON_ID, makeSelectionBox(palette.selection), { pixelRatio: 2 })
  }
  map.addLayer({
    id: SELECTION_LAYER_ID,
    type: 'symbol',
    source: SELECTION_SOURCE_ID,
    layout: {
      'icon-image': SELECTION_ICON_ID,
      'icon-allow-overlap': true,
      'icon-ignore-placement': true,
      'icon-rotation-alignment': 'map',
    },
    paint: {
      'icon-opacity': 0.9,
    },
  })
}

// addLeaderLinesLayer registers the GeoJSON source and line layer for ASD-002
// leader lines — thin lines from each track symbol to its deconflicted data-block
// anchor. Registered before addTracksLayer so lines render behind the dots.
export function addLeaderLinesLayer(map, palette) {
  map.addSource(LEADER_LINES_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })
  map.addLayer({
    id: LEADER_LINES_LAYER_ID,
    type: 'line',
    source: LEADER_LINES_SOURCE_ID,
    paint: {
      'line-color': palette.label,
      'line-width': 0.7,
      'line-opacity': [
        'case',
        ['has', 'fade_opacity'], ['get', 'fade_opacity'],
        ['has', 'fl_opacity'],   ['get', 'fl_opacity'],
        ['get', 'coasting'], 0.3,
        0.55,
      ],
    },
  })
}

// addLabelsLayer registers the GeoJSON source and symbol layer for ASD-002
// deconflicted data-block labels. Label geo positions are computed in screen
// space by deconflictLabels() and pushed here on every render. Setting
// text-allow-overlap:true means MapLibre's placement engine never hides a
// label — our deconfliction engine is solely responsible for placement quality.
export function addLabelsLayer(map, palette) {
  map.addSource(LABELS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })
  map.addLayer({
    id: LABELS_LAYER_ID,
    type: 'symbol',
    source: LABELS_SOURCE_ID,
    layout: {
      'text-field': ['get', 'label'],
      // Roboto Mono (design template data-block face), served from Wayfinder's
      // own self-hosted glyph endpoint (/glyphs, webui.GlyphsHandler) — a symbol
      // layer draws no text without a glyphs source AND a served font, and
      // self-hosting keeps the scope font off any runtime CDN (air-gap, ADR 0015).
      'text-font': ['Roboto Mono Medium'],
      'text-size': LABEL_TEXT_SIZE,
      // Design template data-block metrics (scope-tracks.jsx): 0.02em tracking
      // and 1.25 line-height. Both are expressible on a GL symbol layer (unlike
      // per-line weight, which the template gets from DOM data blocks).
      'text-letter-spacing': 0.02,
      'text-line-height': 1.25,
      // The label point is placed at its deconflicted geo-position by
      // deconflictLabels() (Mercator approximation of the screen-space offset),
      // so the anchor is centred with no further offset.
      'text-anchor': 'center',
      'text-allow-overlap': true,
      'text-ignore-placement': true,
    },
    paint: {
      'text-color': palette.label,
      'text-halo-color': palette.labelHalo,
      'text-halo-width': 1,
      'text-opacity': [
        'case',
        ['has', 'fade_opacity'], ['get', 'fade_opacity'],
        ['has', 'fl_opacity'],   ['get', 'fl_opacity'],
        ['get', 'coasting'], 0.35,
        1.0,
      ],
    },
  })
}

// addSelectionLabelLayer registers the source + line layer for the ASD-011b
// selected-label outline: a bright neutral rounded rectangle framing the selected
// track's data block. Registered AFTER addLabelsLayer so the box draws above (and
// thus frames) the label text. The source carries at most one closed-ring feature
// (see deconflictLabels); round joins/caps give the soft corners of the design.
export function addSelectionLabelLayer(map) {
  map.addSource(SELECTION_LABEL_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })
  map.addLayer({
    id: SELECTION_LABEL_LAYER_ID,
    type: 'line',
    source: SELECTION_LABEL_SOURCE_ID,
    layout: {
      'line-join': 'round',
      'line-cap': 'round',
    },
    paint: {
      'line-color': SELECTION_LABEL_COLOR,
      'line-width': SELECTION_LABEL_WIDTH_PX,
      // Fade with the track when it is a selected, TSE-fading track (edge case).
      'line-opacity': ['case', ['has', 'fade_opacity'], ['get', 'fade_opacity'], 1.0],
    },
  })
}

// addTrailsLayer registers a GeoJSON source and a line layer for rendering
// each track's recent flight path (a fading trail of its last positions).
// Added first so trails draw beneath the history dots, speed vectors and track
// symbols. ASD-004b/4c: line-opacity dims coasting trails and fades TSE trails.
export function addTrailsLayer(map, palette) {
  map.addSource(TRAILS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: TRAILS_LAYER_ID,
    type: 'line',
    source: TRAILS_SOURCE_ID,
    paint: {
      'line-color': palette.trail,
      'line-width': 1,
      'line-opacity': [
        'case',
        ['has', 'fade_opacity'], ['*', 0.6, ['get', 'fade_opacity']],
        ['has', 'fl_opacity'],   ['get', 'fl_opacity'],
        ['get', 'coasting'], 0.2,
        0.6,
      ],
    },
  })
}

// addHistoryDotsLayer registers a GeoJSON source and a circle layer for
// rendering each past position in a track's history as a discrete dot (ASD-004a).
// On a real radar scope, the spacing between dots encodes instantaneous speed
// and the curvature encodes turn rate — information lost in a continuous line.
// ASD-004b/4c: circle-opacity dims coasting dots and fades TSE dots.
export function addHistoryDotsLayer(map, palette) {
  map.addSource(HISTORY_DOTS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: HISTORY_DOTS_LAYER_ID,
    type: 'circle',
    source: HISTORY_DOTS_SOURCE_ID,
    paint: {
      'circle-radius': 1.6, // design template: history dots r=1.6 CSS px
      'circle-color': palette.trail,
      // #191: multiply the base state opacity by an AGE fade so dots grow fainter
      // toward the older end of the trail (age 0 = newest → 1.0; age 1 = oldest →
      // 0.12). The base 'case' keeps the ASD-004b/4c coasting/TSE/FL behaviour.
      'circle-opacity': [
        '*',
        ['interpolate', ['linear'], ['coalesce', ['get', 'age'], 0], 0, 1.0, 1, 0.12],
        [
          'case',
          ['has', 'fade_opacity'], ['*', 0.6, ['get', 'fade_opacity']],
          ['has', 'fl_opacity'],   ['get', 'fl_opacity'],
          ['get', 'coasting'], 0.2,
          0.6,
        ],
      ],
    },
  })
}

// addVectorsLayer registers a GeoJSON source and a line layer for rendering
// each track's speed vector (a short line from the current position towards
// where the track will be in VECTOR_LOOKAHEAD_S seconds, ASD-style SVL).
// Added before the tracks layer so the track symbols draw on top.
// ASD-004b/4c: line-opacity dims coasting vectors and fades TSE vectors.
export function addVectorsLayer(map, palette) {
  map.addSource(VECTORS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: VECTORS_LAYER_ID,
    type: 'line',
    source: VECTORS_SOURCE_ID,
    paint: {
      'line-color': palette.vector,
      'line-width': 1.5,
      'line-opacity': [
        'case',
        ['has', 'fade_opacity'], ['get', 'fade_opacity'],
        ['has', 'fl_opacity'],   ['get', 'fl_opacity'],
        ['get', 'coasting'], 0.35,
        1.0,
      ],
    },
  })
}

// addRangeRingsLayer registers the ASD-012 range-ring overlay: concentric
// constant-distance circles (line layer) plus NM labels (symbol layer) on one
// GeoJSON source. Both start hidden — the operator opts in via the sidebar — and
// the data is (re)generated from the configured centre + reactive spacing/count
// (engine.updateRangeRings). The `kind` property splits rings from labels.
export function addRangeRingsLayer(map, palette) {
  map.addSource(RANGE_RINGS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: RANGE_RINGS_LAYER_ID,
    type: 'line',
    source: RANGE_RINGS_SOURCE_ID,
    filter: ['==', ['get', 'kind'], 'ring'],
    layout: { visibility: 'none' }, // default off (ASD-012; operator enables)
    paint: {
      'line-color': palette.rangeRing,
      'line-width': 1,
      'line-opacity': 0.55,
      'line-dasharray': [2, 3],
    },
  })

  map.addLayer({
    id: RANGE_RINGS_LABEL_LAYER_ID,
    type: 'symbol',
    source: RANGE_RINGS_SOURCE_ID,
    filter: ['==', ['get', 'kind'], 'label'],
    layout: {
      visibility: 'none',
      'text-field': ['get', 'label'],
      'text-font': ['Roboto Mono Medium'],
      'text-size': 10,
      'text-offset': [0, -0.5],
      'text-allow-overlap': false,
    },
    paint: {
      'text-color': palette.rangeRing,
      'text-halo-color': palette.labelHalo,
      'text-halo-width': 1,
    },
  })
}

// addCoverageLayer registers the sensor coverage ring overlay.
//
// Two MapLibre layers are created on a single GeoJSON source:
//   - COVERAGE_RINGS_LAYER_ID       : outer ring LineStrings.
//   - COVERAGE_RINGS_LAYER_ID-inner : inner (dead-zone) ring LineStrings.
//   - COVERAGE_CENTER_LAYER_ID      : small dot at each sensor site.
//
// The source starts empty; call updateCoverageSource() after the map loads to
// populate it with data from /api/coverage/rings.
export function addCoverageLayer(map) {
  map.addSource(COVERAGE_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  // Outer ring: dashed line at max detection range.
  map.addLayer({
    id: COVERAGE_RINGS_LAYER_ID,
    type: 'line',
    source: COVERAGE_SOURCE_ID,
    filter: ['==', ['get', 'type'], 'outer'],
    paint: {
      'line-color': ['get', 'color'],
      'line-width': 1.2,
      'line-opacity': 0.65,
      'line-dasharray': [4, 3],
    },
  })

  // Inner (dead-zone) ring: finer dashes at lower opacity.
  map.addLayer({
    id: COVERAGE_RINGS_LAYER_ID + '-inner',
    type: 'line',
    source: COVERAGE_SOURCE_ID,
    filter: ['==', ['get', 'type'], 'inner'],
    paint: {
      'line-color': ['get', 'color'],
      'line-width': 0.8,
      'line-opacity': 0.40,
      'line-dasharray': [2, 3],
    },
  })

  // Sensor centre dot.
  map.addLayer({
    id: COVERAGE_CENTER_LAYER_ID,
    type: 'circle',
    source: COVERAGE_SOURCE_ID,
    filter: ['==', ['get', 'type'], 'center'],
    paint: {
      'circle-color': ['get', 'color'],
      'circle-radius': 4,
      'circle-opacity': 0.80,
      'circle-stroke-color': '#000',
      'circle-stroke-width': 1,
    },
  })
}

// updateCoverageSource replaces the GeoJSON data in the coverage ring source.
export function updateCoverageSource(map, geojson) {
  const src = map.getSource(COVERAGE_SOURCE_ID)
  if (src) src.setData(geojson)
}
