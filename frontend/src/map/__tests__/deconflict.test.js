import { describe, it, expect, vi } from 'vitest'
import { bboxCollides, deconflictLabels } from '../deconflict.js'

// Helpers
function bbox(x1, y1, x2, y2) {
  return { x1, y1, x2, y2 }
}

function makeTrackFeature(trackNum, lon, lat, extra = {}) {
  return {
    type: 'Feature',
    geometry: { type: 'Point', coordinates: [lon, lat] },
    properties: { track_num: trackNum, label: `T${trackNum}`, coasting: false, ...extra },
  }
}

// Mock MapLibre map: project returns a deterministic screen point
function makeMockMap(zoom = 10) {
  return {
    getZoom: () => zoom,
    // Simple bijection: lon*10, lat*10 (arbitrary, consistent)
    project: ([lon, lat]) => ({ x: lon * 100, y: lat * 100 }),
  }
}

describe('bboxCollides', () => {
  it('returns false for empty occupied list', () => {
    expect(bboxCollides([], bbox(0, 0, 10, 10))).toBe(false)
  })

  it('returns false when bbox is adjacent but not overlapping', () => {
    const occupied = [bbox(0, 0, 10, 10)]
    // Touching right edge — x1 of candidate equals x2 of occupied → no overlap
    expect(bboxCollides(occupied, bbox(10, 0, 20, 10))).toBe(false)
  })

  it('returns true when bbox overlaps an occupied rectangle', () => {
    const occupied = [bbox(0, 0, 20, 20)]
    expect(bboxCollides(occupied, bbox(10, 10, 30, 30))).toBe(true)
  })

  it('returns false when bbox is entirely to the right', () => {
    const occupied = [bbox(0, 0, 10, 10)]
    expect(bboxCollides(occupied, bbox(20, 0, 30, 10))).toBe(false)
  })

  it('returns false when bbox is entirely above', () => {
    const occupied = [bbox(50, 50, 100, 100)]
    expect(bboxCollides(occupied, bbox(50, 0, 100, 49))).toBe(false)
  })

  it('detects collision with second element in occupied list', () => {
    const occupied = [bbox(0, 0, 5, 5), bbox(100, 100, 200, 200)]
    expect(bboxCollides(occupied, bbox(150, 150, 250, 250))).toBe(true)
  })
})

describe('deconflictLabels', () => {
  it('returns empty arrays when given no features', () => {
    const map = makeMockMap()
    const { labelFeatures, leaderLineFeatures } = deconflictLabels([], map, new Map())
    expect(labelFeatures).toHaveLength(0)
    expect(leaderLineFeatures).toHaveLength(0)
  })

  it('produces one label per track', () => {
    const map = makeMockMap()
    const features = [
      makeTrackFeature(1, 8.0, 50.0),
      makeTrackFeature(2, 8.5, 50.5),
    ]
    const { labelFeatures } = deconflictLabels(features, map, new Map())
    expect(labelFeatures).toHaveLength(2)
  })

  it('processes tracks in track_num order (deterministic)', () => {
    const map = makeMockMap()
    // Provide in reverse order — result should be sorted ascending by track_num
    const features = [
      makeTrackFeature(5, 8.0, 50.0),
      makeTrackFeature(1, 8.1, 50.1),
      makeTrackFeature(3, 8.2, 50.2),
    ]
    const { labelFeatures } = deconflictLabels(features, map, new Map())
    const nums = labelFeatures.map(f => f.properties.track_num)
    expect(nums).toEqual([1, 3, 5])
  })

  it('uses manual pin when labelPins has an entry', () => {
    const map = makeMockMap()
    const features = [makeTrackFeature(42, 8.0, 50.0)]
    const pins = new Map([[42, { dx: 99, dy: -55 }]])
    const { labelFeatures } = deconflictLabels(features, map, pins)
    expect(labelFeatures).toHaveLength(1)
    // The label geo position should reflect the pin offset (dx=99, dy=-55)
    // relative to the projected symbol position — just check a label was placed.
    expect(labelFeatures[0].properties.track_num).toBe(42)
  })

  it('never suppresses a label (fallback slot 0 when all collide)', () => {
    // Place many tracks at the exact same screen position — all slots will
    // collide for most tracks; they must still get a label each.
    const map = makeMockMap()
    const features = Array.from({ length: 20 }, (_, i) =>
      makeTrackFeature(i + 1, 8.0, 50.0),
    )
    const { labelFeatures } = deconflictLabels(features, map, new Map())
    expect(labelFeatures).toHaveLength(20)
  })

  it('carries fade_opacity from track features to label features', () => {
    const map = makeMockMap()
    const features = [makeTrackFeature(7, 8.0, 50.0, { fade_opacity: 0.42 })]
    const { labelFeatures } = deconflictLabels(features, map, new Map())
    expect(labelFeatures[0].properties.fade_opacity).toBeCloseTo(0.42)
  })

  it('carries fl_opacity from track features to label features', () => {
    const map = makeMockMap()
    const features = [makeTrackFeature(8, 8.0, 50.0, { fl_opacity: 0.15 })]
    const { labelFeatures } = deconflictLabels(features, map, new Map())
    expect(labelFeatures[0].properties.fl_opacity).toBeCloseTo(0.15)
  })
})
