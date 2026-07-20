// Measure controller (Häppchen 4): wires the RBL/DIST/QDM tools onto a MapLibre
// map. Kept out of engine.js so the tools are a self-contained overlay that
// operates purely on the public map instance (a source + a few thin layers and
// event handlers). The distance/bearing readout is reported via onReadout to the
// Vue layer (tools store), NOT drawn as a MapLibre symbol — no glyph dependency.
//
//   RBL  — press-drag on the map: A = mousedown, B follows the cursor.
//   DIST — click two tracks in turn; a third click restarts.
//   QDM  — click a track (A), then any point (B); a further track click restarts.
//
// Track picking (#298) accepts a click on the track SYMBOL or its data-block
// LABEL — the label is the larger, natural target (mirrors the normal-selection
// behaviour from #271, which deliberately excludes the tool mode). A picked track
// is a live REFERENCE, not a frozen coordinate: refreshTracks() re-resolves its
// position from every track batch so the measure line FOLLOWS the moving track
// (#297) and the readout stays correct; a free point (RBL, the QDM target) stays
// a fixed map coordinate. Picked tracks are ringed so the operator sees what the
// measurement is anchored to.

import { measureText } from './tools.js'
import { TRACKS_LAYER_ID, LABELS_LAYER_ID } from './constants.js'

const SRC = 'measure'
const LINE = 'measure-line'
const PTS = 'measure-points'
const HILITE = 'measure-track-highlight'
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
    // #298: a highlight ring around an endpoint that is a TRACK (role='track'),
    // so the operator sees which track the measurement is anchored to. A hollow
    // cyan ring — distinct in SHAPE from the corner-bracket selection box (a
    // measure pick is not a normal selection) and in COLOUR from the amber SPI /
    // magenta search rings. Added under the dot so the dot stays crisp on top.
    map.addLayer({
      id: HILITE,
      type: 'circle',
      source: SRC,
      filter: ['all', ['==', ['geometry-type'], 'Point'], ['==', ['get', 'role'], 'track']],
      paint: {
        'circle-radius': 11,
        'circle-color': 'rgba(0,0,0,0)',
        'circle-stroke-color': MEASURE_COLOR,
        'circle-stroke-width': 2,
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
  let a = null // endpoint: { lng, lat, trackNum } — trackNum null = free point
  let b = null
  let dragging = false
  // #297: latest live track positions, keyed by track_num, refreshed from every
  // WS batch via refreshTracks(). Used to (a) make picked endpoints follow their
  // track and (b) resolve a label-click to the track's real symbol position.
  let liveById = new Map()

  // freePt builds a fixed-coordinate endpoint (no track reference).
  const freePt = (lngLat) => ({ lng: lngLat.lng, lat: lngLat.lat, trackNum: null })

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
      if (p) {
        feats.push({
          type: 'Feature',
          geometry: { type: 'Point', coordinates: [p.lng, p.lat] },
          // role drives the #298 highlight ring: only track-anchored endpoints.
          properties: { role: p.trackNum != null ? 'track' : 'free' },
        })
      }
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

  // trackAt returns the track under a screen point as { lng, lat, trackNum }, or
  // null. It accepts a hit on the track SYMBOL or its data-block LABEL (#298);
  // both features carry the same track_num. The position is the track's
  // authoritative symbol position: the live position by track_num when known
  // (so a label click does not anchor to the offset data block), else the
  // symbol geometry, else — a label-only hit before the first batch — the label
  // position as a temporary anchor (self-corrects on the next refreshTracks).
  function trackAt(point) {
    const layers = [TRACKS_LAYER_ID, LABELS_LAYER_ID].filter((id) => map.getLayer(id))
    if (!layers.length) return null
    const feats = map.queryRenderedFeatures(point, { layers })
    if (!feats.length) return null
    const symHit = feats.find((f) => f.layer?.id === TRACKS_LAYER_ID)
    const hit = symHit || feats[0]
    const trackNum = hit.properties?.track_num
    if (trackNum == null) return null
    const live = liveById.get(trackNum)
    if (live) return { lng: live.lng, lat: live.lat, trackNum }
    const c = symHit?.geometry?.coordinates
    if (c) return { lng: c[0], lat: c[1], trackNum }
    const lc = hit.geometry?.coordinates
    return lc ? { lng: lc[0], lat: lc[1], trackNum } : null
  }

  // RBL — drag on the map.
  function onDown(e) {
    if (tool !== 'rbl') return
    dragging = true
    a = freePt(e.lngLat)
    b = freePt(e.lngLat)
    render()
  }
  function onMove(e) {
    if (tool !== 'rbl' || !dragging) return
    b = freePt(e.lngLat)
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
        b = freePt(e.lngLat)
        render()
      } else {
        a = trackAt(e.point)
        b = null
        render()
      }
    }
  }

  // #297: refresh live track positions from a track batch and re-anchor any
  // track-referenced endpoint to its current position, so the measure line
  // follows the moving track. A track that has left the displayed set (TSE /
  // out of scope) is simply absent here → its endpoint keeps its last known
  // position (frozen), matching the FR-UI-029/refreshSelectedTrack convention.
  function refreshTracks(features) {
    const next = new Map()
    for (const f of features || []) {
      const tn = f.properties?.track_num
      const c = f.geometry?.coordinates
      if (tn != null && c) next.set(tn, { lng: c[0], lat: c[1] })
    }
    liveById = next
    let changed = false
    for (const p of [a, b]) {
      if (p && p.trackNum != null) {
        const live = liveById.get(p.trackNum)
        if (live && (live.lng !== p.lng || live.lat !== p.lat)) {
          p.lng = live.lng
          p.lat = live.lat
          changed = true
        }
      }
    }
    if (changed) render()
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
    if (map.getLayer(HILITE)) map.removeLayer(HILITE)
    if (map.getSource(SRC)) map.removeSource(SRC)
  }

  return { setTool, clear: reset, refreshTracks, destroy }
}
