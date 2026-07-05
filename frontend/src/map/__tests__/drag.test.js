import { describe, it, expect, vi } from 'vitest'
import { setupLabelDrag } from '../drag.js'
import { LABELS_LAYER_ID } from '../constants.js'

// Minimal MapLibre map mock: records handlers registered via on()/off() and
// lets the test fire them directly, like MapLibre would on real mouse events.
// project() is a simple bijection (lon*100, lat*100) — deterministic and easy
// to reason about, matching the style used in deconflict.test.js.
function makeMockMap() {
  const handlers = {}
  return {
    handlers,
    on(event, layerOrHandler, maybeHandler) {
      // Layer-scoped form: on(event, layerId, handler)
      if (typeof layerOrHandler === 'function') {
        handlers[event] = layerOrHandler
      } else {
        handlers[event] = maybeHandler
      }
    },
    off(event) {
      delete handlers[event]
    },
    project: ([lon, lat]) => ({ x: lon * 100, y: lat * 100 }),
    queryRenderedFeatures: vi.fn(),
    dragPan: { enable: vi.fn(), disable: vi.fn() },
    getCanvas: () => ({ style: {} }),
  }
}

function makeTrackFeature(trackNum, lon, lat) {
  return {
    type: 'Feature',
    geometry: { type: 'Point', coordinates: [lon, lat] },
    properties: { track_num: trackNum },
  }
}

// A label feature as produced by deconflictLabels(): its geometry carries the
// current (auto-placed or pinned) screen position of the label, projected
// back to a lon/lat the mock map.project() can round-trip.
function makeLabelFeature(trackNum, lon, lat) {
  return {
    type: 'Feature',
    geometry: { type: 'Point', coordinates: [lon, lat] },
    properties: { track_num: trackNum },
  }
}

describe('setupLabelDrag', () => {
  it('does not snap the label to the cursor on the first move (no initial jump)', () => {
    const map = makeMockMap()
    // Symbol at (8.0, 50.0) -> projected (800, 5000).
    // Label auto-placed one slot away, at (8.01, 50.02) -> projected (801, 5002).
    const trackFeature = makeTrackFeature(7, 8.0, 50.0)
    const labelFeature = makeLabelFeature(7, 8.01, 50.02)
    const state = {
      liveTrackFeatures: [trackFeature],
      fadingTracks: new Map(),
      labelPins: new Map(),
    }
    map.queryRenderedFeatures.mockReturnValue([labelFeature])
    const onRender = vi.fn()

    setupLabelDrag(map, state, onRender)

    const labelPos = map.project(labelFeature.geometry.coordinates) // {x:801, y:5002}
    const sym = map.project(trackFeature.geometry.coordinates) // {x:800, y:5000}

    // Grab the label not at its exact origin, but a few pixels into it —
    // this is the case that used to snap-jump under the old (buggy) offset.
    const grabPoint = { x: labelPos.x + 5, y: labelPos.y + 3 }
    map.handlers.mousedown({ point: grabPoint, preventDefault: () => {} })

    // First mousemove with zero cursor delta (mouse hasn't actually moved yet).
    map.handlers.mousemove({ point: grabPoint })

    const pin = state.labelPins.get(7)
    const newLabelPos = { x: sym.x + pin.dx, y: sym.y + pin.dy }

    // Invariant: with zero mouse delta, the label must stay exactly where it
    // was auto-placed — no jump to the raw grab point.
    expect(newLabelPos.x).toBeCloseTo(labelPos.x, 9)
    expect(newLabelPos.y).toBeCloseTo(labelPos.y, 9)
  })

  it('keeps the exact grabbed point under the cursor as the mouse moves', () => {
    const map = makeMockMap()
    const trackFeature = makeTrackFeature(3, 8.0, 50.0)
    const labelFeature = makeLabelFeature(3, 8.01, 50.02)
    const state = {
      liveTrackFeatures: [trackFeature],
      fadingTracks: new Map(),
      labelPins: new Map(),
    }
    map.queryRenderedFeatures.mockReturnValue([labelFeature])
    const onRender = vi.fn()

    setupLabelDrag(map, state, onRender)

    const labelPos = map.project(labelFeature.geometry.coordinates)
    const sym = map.project(trackFeature.geometry.coordinates)

    const grabPoint = { x: labelPos.x + 5, y: labelPos.y + 3 }
    map.handlers.mousedown({ point: grabPoint, preventDefault: () => {} })

    // Move the cursor by an arbitrary delta.
    const delta = { x: 37, y: -21 }
    const movePoint = { x: grabPoint.x + delta.x, y: grabPoint.y + delta.y }
    map.handlers.mousemove({ point: movePoint })

    const pin = state.labelPins.get(3)
    const newLabelPos = { x: sym.x + pin.dx, y: sym.y + pin.dy }

    // The point originally grabbed (offset (5,3) into the label) must be
    // exactly under the new cursor position.
    const grabbedPointNow = { x: newLabelPos.x + 5, y: newLabelPos.y + 3 }
    expect(grabbedPointNow.x).toBeCloseTo(movePoint.x, 9)
    expect(grabbedPointNow.y).toBeCloseTo(movePoint.y, 9)

    // Equivalently, the label origin itself moved by exactly `delta`.
    expect(newLabelPos.x).toBeCloseTo(labelPos.x + delta.x, 9)
    expect(newLabelPos.y).toBeCloseTo(labelPos.y + delta.y, 9)
  })

  it('reuses the existing pin offset as the drag start when the label is already pinned', () => {
    const map = makeMockMap()
    const trackFeature = makeTrackFeature(9, 8.0, 50.0)
    const existingPin = { dx: 50, dy: -30 }
    const pinnedLabelLon = (map.project(trackFeature.geometry.coordinates).x + existingPin.dx) / 100
    const pinnedLabelLat = (map.project(trackFeature.geometry.coordinates).y + existingPin.dy) / 100
    const labelFeature = makeLabelFeature(9, pinnedLabelLon, pinnedLabelLat)
    const state = {
      liveTrackFeatures: [trackFeature],
      fadingTracks: new Map(),
      labelPins: new Map([[9, existingPin]]),
    }
    map.queryRenderedFeatures.mockReturnValue([labelFeature])
    const onRender = vi.fn()

    setupLabelDrag(map, state, onRender)

    const labelPos = map.project(labelFeature.geometry.coordinates)
    const grabPoint = { x: labelPos.x + 2, y: labelPos.y + 1 }
    map.handlers.mousedown({ point: grabPoint, preventDefault: () => {} })
    map.handlers.mousemove({ point: grabPoint })

    const pin = state.labelPins.get(9)
    const sym = map.project(trackFeature.geometry.coordinates)
    const newLabelPos = { x: sym.x + pin.dx, y: sym.y + pin.dy }

    // Zero mouse delta -> label stays exactly at its (pinned) position.
    expect(newLabelPos.x).toBeCloseTo(labelPos.x, 9)
    expect(newLabelPos.y).toBeCloseTo(labelPos.y, 9)
  })
})
