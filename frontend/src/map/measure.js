// Measure controller (Häppchen 4): wires the RBL/DIST/QDM tools onto a MapLibre
// map. Kept out of engine.js so the tools are a self-contained overlay that
// operates purely on the public map instance (a source + two thin layers + a few
// event handlers). The distance/bearing readout is reported via onReadout to the
// Vue layer (tools store), NOT drawn as a MapLibre symbol — no glyph dependency.
//
//   RBL  — press-drag on the map: A = mousedown, B follows the cursor.
//   DIST — click two tracks in turn; a third click restarts.
//   QDM  — click a track (A), then any point (B); a further track click restarts.
//
// Track picking uses queryRenderedFeatures against the tracks layer, so it needs
// no coupling to the engine's own click handling.

import { measureText } from './tools.js'
import { TRACKS_LAYER_ID } from './constants.js'

const SRC = 'measure'
const LINE = 'measure-line'
const PTS = 'measure-points'
const EMPTY = { type: 'FeatureCollection', features: [] }

// Primary cyan (matches --wf-primary); measure chrome is deliberately distinct
// from track/state colours so a measurement never reads as traffic.
const MEASURE_COLOR = '#23d3e6'

export function createMeasure(map, { onReadout } = {}) {
  const report = (t, at) => { if (onReadout) onReadout(t, at) }

  // labelAnchor projects the A–B midpoint to map-canvas pixels, so the readout
  // label can float at the line (screenshot request). Reprojected on drag and on
  // map move so it stays glued while the viewport changes.
  function labelAnchor() {
    if (!(a && b)) return null
    const p = map.project([(a.lng + b.lng) / 2, (a.lat + b.lat) / 2])
    return { x: p.x, y: p.y }
  }

  if (!map.getSource(SRC)) {
    map.addSource(SRC, { type: 'geojson', data: EMPTY })
    map.addLayer({
      id: LINE,
      type: 'line',
      source: SRC,
      filter: ['==', ['geometry-type'], 'LineString'],
      paint: {
        'line-color': MEASURE_COLOR,
        'line-width': 1.5,
        'line-dasharray': [2, 2],
      },
    })
    map.addLayer({
      id: PTS,
      type: 'circle',
      source: SRC,
      filter: ['==', ['geometry-type'], 'Point'],
      paint: {
        'circle-radius': 3,
        'circle-color': MEASURE_COLOR,
        'circle-stroke-color': '#04141a',
        'circle-stroke-width': 1,
      },
    })
  }

  let tool = null
  let a = null // {lng, lat}
  let b = null
  let dragging = false

  function render() {
    const feats = []
    if (a && b) {
      feats.push({
        type: 'Feature',
        geometry: { type: 'LineString', coordinates: [[a.lng, a.lat], [b.lng, b.lat]] },
        properties: {},
      })
    }
    for (const p of [a, b]) {
      if (p) feats.push({ type: 'Feature', geometry: { type: 'Point', coordinates: [p.lng, p.lat] }, properties: {} })
    }
    const src = map.getSource(SRC)
    if (src) src.setData({ type: 'FeatureCollection', features: feats })
    report(a && b ? measureText(a, b) : null, labelAnchor())
  }

  function reset() {
    a = null
    b = null
    dragging = false
    render()
  }

  // trackAt returns the lng/lat of the track rendered under a screen point, or null.
  function trackAt(point) {
    if (!map.getLayer(TRACKS_LAYER_ID)) return null
    const feats = map.queryRenderedFeatures(point, { layers: [TRACKS_LAYER_ID] })
    const c = feats[0]?.geometry?.coordinates
    return c ? { lng: c[0], lat: c[1] } : null
  }

  // RBL — drag on the map.
  function onDown(e) {
    if (tool !== 'rbl') return
    dragging = true
    a = { lng: e.lngLat.lng, lat: e.lngLat.lat }
    b = { ...a }
    render()
  }
  function onMove(e) {
    if (tool !== 'rbl' || !dragging) return
    b = { lng: e.lngLat.lng, lat: e.lngLat.lat }
    render()
  }
  function onUp() {
    if (tool === 'rbl') dragging = false
  }

  // DIST / QDM — clicks.
  function onClick(e) {
    if (tool === 'dist') {
      const t = trackAt(e.point)
      if (!t) return
      if (a && b) { a = t; b = null } // third click restarts
      else if (a) { b = t }
      else { a = t }
      render()
    } else if (tool === 'qdm') {
      if (!a) {
        const t = trackAt(e.point)
        if (t) { a = t; render() }
      } else if (!b) {
        b = { lng: e.lngLat.lng, lat: e.lngLat.lat }
        render()
      } else {
        a = trackAt(e.point)
        b = null
        render()
      }
    }
  }

  // Keep the floating readout label glued to the line while the map pans/zooms
  // (DIST/QDM leave panning enabled; RBL disables it during its own drag).
  function reproject() { if (a && b) report(measureText(a, b), labelAnchor()) }

  map.on('mousedown', onDown)
  map.on('mousemove', onMove)
  map.on('mouseup', onUp)
  map.on('click', onClick)
  map.on('move', reproject)

  function setTool(kind) {
    reset()
    tool = kind || null
    map.getCanvas().style.cursor = tool ? 'crosshair' : ''
    // RBL owns the drag gesture, so map panning must yield while it is active.
    if (tool === 'rbl') map.dragPan.disable()
    else map.dragPan.enable()
  }

  function destroy() {
    map.off('mousedown', onDown)
    map.off('mousemove', onMove)
    map.off('mouseup', onUp)
    map.off('click', onClick)
    map.off('move', reproject)
    map.dragPan.enable()
    map.getCanvas().style.cursor = ''
    if (map.getLayer(LINE)) map.removeLayer(LINE)
    if (map.getLayer(PTS)) map.removeLayer(PTS)
    if (map.getSource(SRC)) map.removeSource(SRC)
  }

  return { setTool, clear: reset, destroy }
}
