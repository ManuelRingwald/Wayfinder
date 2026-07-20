// #297/#298: the measure tools anchor a measurement to a TRACK (not a frozen
// coordinate). A track can be picked by its symbol OR its data-block label
// (#298), the picked track is ringed, and the endpoint FOLLOWS the moving track
// on every batch (#297). measure.js injects its MapLibre map, so the behaviour is
// exercised directly through a minimal fake map; the engine/MapCanvas wiring that
// feeds batches in is guarded at the source level.
import { describe, it, expect } from 'vitest'
import { createMeasure } from '../measure.js'
import engine from '../engine.js?raw'
import mapCanvas from '../../components/MapCanvas.vue?raw'

// makeMap is the smallest MapLibre surface createMeasure touches. It captures the
// GeoJSON pushed to the measure source and the registered event handlers so a
// test can drive clicks and read back the rendered features.
function makeMap() {
  const state = { sources: new Set(), layers: new Set(), handlers: {}, lastData: null, qrf: [] }
  const map = {
    getSource(id) { return state.sources.has(id) ? { setData: (d) => { state.lastData = d } } : undefined },
    addSource(id) { state.sources.add(id) },
    addLayer(l) { state.layers.add(l.id) },
    getLayer(id) {
      // The track/label layers exist in the real map; measure layers exist once added.
      return state.layers.has(id) || id === 'tracks-points' || id === 'track-labels' ? { id } : undefined
    },
    removeLayer(id) { state.layers.delete(id) },
    removeSource(id) { state.sources.delete(id) },
    on(ev, fn) { (state.handlers[ev] ||= []).push(fn) },
    off() {},
    getCanvas() { return { style: {} } },
    dragPan: { enable() {}, disable() {} },
    project([lng, lat]) { return { x: lng, y: lat } },
    queryRenderedFeatures() { return state.qrf },
    loaded() { return true },
  }
  return { map, state }
}

const trackHit = (layerId, trackNum, lng, lat) => ({
  layer: { id: layerId },
  properties: { track_num: trackNum },
  geometry: { coordinates: [lng, lat] },
})

const lineOf = (state) => state.lastData.features.find((f) => f.geometry.type === 'LineString')
const pointsOf = (state) => state.lastData.features.filter((f) => f.geometry.type === 'Point')
const click = (state, point = { x: 1, y: 1 }) => state.handlers.click[0]({ point })

describe('#298: pick a track by symbol or label + highlight', () => {
  it('DIST picks the track under the SYMBOL and rings it (role=track)', () => {
    const { map, state } = makeMap()
    const m = createMeasure(map, { onReadout: () => {} })
    m.setTool('dist')

    state.qrf = [trackHit('tracks-points', 42, 10, 50)]
    click(state)
    state.qrf = [trackHit('tracks-points', 43, 11, 51)]
    click(state)

    expect(lineOf(state).geometry.coordinates).toEqual([[10, 50], [11, 51]])
    // Both endpoints are tracks → both carry the highlight role.
    expect(pointsOf(state).every((p) => p.properties.role === 'track')).toBe(true)
  })

  it('QDM resolves a LABEL click to the same track (data-block is a valid target)', () => {
    const { map, state } = makeMap()
    const m = createMeasure(map, { onReadout: () => {} })
    m.setTool('qdm')

    // Only the label layer is under the cursor (not the small symbol).
    state.qrf = [trackHit('track-labels', 7, 8, 48)]
    click(state)

    const pts = pointsOf(state)
    expect(pts).toHaveLength(1) // QDM point A only, B not set yet
    expect(pts[0].properties.role).toBe('track') // a track, not a free point
    expect(pts[0].geometry.coordinates).toEqual([8, 48])
  })

  it('the QDM target point is a FREE point (no highlight ring)', () => {
    const { map, state } = makeMap()
    const m = createMeasure(map, { onReadout: () => {} })
    m.setTool('qdm')
    state.qrf = [trackHit('tracks-points', 7, 8, 48)]
    click(state) // A = track
    state.qrf = []
    state.handlers.click[0]({ point: { x: 2, y: 2 }, lngLat: { lng: 9, lat: 49 } }) // B = free point

    const roles = pointsOf(state).map((p) => p.properties.role).sort()
    expect(roles).toEqual(['free', 'track'])
  })
})

describe('#297: a track-anchored endpoint follows the moving track', () => {
  it('re-anchors the endpoint to the live position on the next batch', () => {
    const { map, state } = makeMap()
    const m = createMeasure(map, { onReadout: () => {} })
    m.setTool('dist')
    state.qrf = [trackHit('tracks-points', 42, 10, 50)]
    click(state)
    state.qrf = [trackHit('tracks-points', 43, 11, 51)]
    click(state)
    expect(lineOf(state).geometry.coordinates).toEqual([[10, 50], [11, 51]])

    // Track 42 moves; the A endpoint must follow it, B stays put.
    m.refreshTracks([
      trackHit('tracks-points', 42, 10.5, 50.5),
      trackHit('tracks-points', 43, 11, 51),
    ])
    expect(lineOf(state).geometry.coordinates).toEqual([[10.5, 50.5], [11, 51]])
  })

  it('freezes the endpoint at its last position when the track leaves the set', () => {
    const { map, state } = makeMap()
    const m = createMeasure(map, { onReadout: () => {} })
    m.setTool('dist')
    state.qrf = [trackHit('tracks-points', 42, 10, 50)]
    click(state)
    state.qrf = [trackHit('tracks-points', 43, 11, 51)]
    click(state)

    // Track 42 is gone from the batch (TSE / out of scope) → A keeps last position.
    m.refreshTracks([trackHit('tracks-points', 43, 12, 52)])
    expect(lineOf(state).geometry.coordinates).toEqual([[10, 50], [12, 52]])
  })
})

describe('#297/#298 wiring is fed from the track batch loop', () => {
  it('measure.js queries both the track and label layers and exposes refreshTracks', async () => {
    const measureSrc = (await import('../measure.js?raw')).default
    expect(measureSrc).toContain('LABELS_LAYER_ID')
    expect(measureSrc).toContain('queryRenderedFeatures(point, { layers })')
    expect(measureSrc).toContain('refreshTracks')
  })

  it('the engine feeds every batch to onTracks; MapCanvas forwards it to the controller', () => {
    expect(engine).toContain('onTracks = null')
    expect(engine).toContain('onTracks?.(state.liveTrackFeatures)')
    expect(mapCanvas).toContain('measure?.refreshTracks(features)')
  })
})
