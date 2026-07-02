import { describe, it, expect } from 'vitest'
import { radiusNmToBbox, bboxToRadius } from '@/admin/geo.js'

describe('admin geo — radiusNmToBbox', () => {
  it('produces a latitude half-height of radius/60 degrees', () => {
    // 60 NM radius → 1° of latitude on each side.
    const b = radiusNmToBbox(50, 8, 60)
    expect(b.max_lat - 50).toBeCloseTo(1, 6)
    expect(50 - b.min_lat).toBeCloseTo(1, 6)
  })

  it('emits the backend wire shape (snake_case min/max_lat/lon)', () => {
    // Guards the regression where camelCase output silently dropped the radius on
    // the round-trip through the backend (which speaks snake_case).
    const b = radiusNmToBbox(50, 8, 60)
    expect(Object.keys(b).sort()).toEqual(['max_lat', 'max_lon', 'min_lat', 'min_lon'])
  })

  it('widens longitude by 1/cos(lat) relative to latitude', () => {
    const lat = 50
    const b = radiusNmToBbox(lat, 8, 60)
    const lonHalf = (b.max_lon - b.min_lon) / 2
    const latHalf = (b.max_lat - b.min_lat) / 2
    expect(lonHalf).toBeCloseTo(latHalf / Math.cos((lat * Math.PI) / 180), 6)
  })

  it('returns null for a non-positive radius', () => {
    expect(radiusNmToBbox(50, 8, 0)).toBeNull()
    expect(radiusNmToBbox(50, 8, -10)).toBeNull()
  })

  it('returns null for non-finite inputs', () => {
    expect(radiusNmToBbox(NaN, 8, 60)).toBeNull()
    expect(radiusNmToBbox(50, 8, Infinity)).toBeNull()
  })

  it('clamps to valid WGS84 ranges near the pole', () => {
    const b = radiusNmToBbox(89.95, 0, 600)
    expect(b.max_lat).toBeLessThanOrEqual(90)
    expect(b.min_lon).toBeGreaterThanOrEqual(-180)
    expect(b.max_lon).toBeLessThanOrEqual(180)
  })
})

describe('admin geo — bboxToRadius', () => {
  it('round-trips a box produced by radiusNmToBbox', () => {
    const b = radiusNmToBbox(50, 8, 120)
    const r = bboxToRadius(b)
    expect(r.centerLat).toBeCloseTo(50, 6)
    expect(r.centerLon).toBeCloseTo(8, 6)
    expect(r.radiusNm).toBeCloseTo(120, 6)
  })

  it('reads a backend-shaped AOI (snake_case) as stored on the wire', () => {
    // The exact shape store.BBox serialises and the admin API returns; the load
    // path must derive the radius from it, not reset to 0 (WF-radius-bug).
    const r = bboxToRadius({ min_lat: 49, max_lat: 51, min_lon: 6, max_lon: 10 })
    expect(r.centerLat).toBeCloseTo(50, 6)
    expect(r.radiusNm).toBeCloseTo(60, 6)
  })

  it('returns null for a missing or degenerate box', () => {
    expect(bboxToRadius(null)).toBeNull()
    expect(bboxToRadius({ min_lat: 50, max_lat: 50, min_lon: 8, max_lon: 8 })).toBeNull()
  })

  it('returns null when a corner is non-finite', () => {
    expect(bboxToRadius({ min_lat: 50, max_lat: NaN, min_lon: 8, max_lon: 9 })).toBeNull()
  })
})
