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
  NAVAIDS_SOURCE_ID,
  NAVAIDS_LAYER_ID,
  WAYPOINTS_SOURCE_ID,
  WAYPOINTS_LAYER_ID,
  LABELS_SOURCE_ID,
  LABELS_LAYER_ID,
  LEADER_LINES_SOURCE_ID,
  LEADER_LINES_LAYER_ID,
  LABEL_TEXT_SIZE,
  AIRSPACE_GROUPS,
  COVERAGE_SOURCE_ID,
  COVERAGE_RINGS_LAYER_ID,
  COVERAGE_CENTER_LAYER_ID,
} from './constants.js'

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
function makeIconImage(draw) {
  const size = 24
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
      'text-font': ['Open Sans Regular'],
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
      'text-font': ['Open Sans Regular'],
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
      'text-font': ['Open Sans Regular'],
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

// addTracksLayer registers a GeoJSON source and a circle layer for rendering
// tracks (status-dependent colour). ASD-004b/4c: circle-opacity and
// text-opacity use data-driven expressions to dim coasting tracks and fade
// TSE tracks to transparency.
export function addTracksLayer(map, palette) {
  map.addSource(TRACKS_SOURCE_ID, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
  })

  map.addLayer({
    id: TRACKS_LAYER_ID,
    type: 'circle',
    source: TRACKS_SOURCE_ID,
    paint: {
      'circle-radius': 5,
      'circle-color': [
        'case',
        ['get', 'filtered'],
        '#455a64', // blue-grey: outside FL filter range (ASD-005)
        ['get', 'coasting'],
        '#ff9800', // orange: coasting (no recent update)
        ['get', 'confirmed'],
        '#4caf50', // green: confirmed track
        '#9e9e9e', // grey: tentative track
      ],
      'circle-stroke-width': 1,
      'circle-stroke-color': palette.symbolStroke,
      'circle-opacity': [
        'case',
        ['has', 'fade_opacity'], ['get', 'fade_opacity'],
        ['has', 'fl_opacity'],   ['get', 'fl_opacity'],
        ['get', 'coasting'], 0.5,
        1.0,
      ],
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
      // Explicit font from the style's glyphs endpoint (fonts.openmaptiles.org).
      // Without a glyphs source AND a served font, a symbol layer renders no text
      // at all — which is exactly why labels were invisible while the circle and
      // line layers (needing no glyphs) drew fine.
      'text-font': ['Open Sans Regular'],
      'text-size': LABEL_TEXT_SIZE,
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
      'circle-radius': 2,
      'circle-color': palette.trail,
      'circle-opacity': [
        'case',
        ['has', 'fade_opacity'], ['*', 0.6, ['get', 'fade_opacity']],
        ['has', 'fl_opacity'],   ['get', 'fl_opacity'],
        ['get', 'coasting'], 0.2,
        0.6,
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
