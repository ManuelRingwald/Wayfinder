import { describe, it, expect } from 'vitest'
import { radiusNmToBbox, bboxToRadius } from '@/admin/geo.js'

describe('admin geo — radiusNmToBbox', () => {
  it('produces a latitude half-height of radius/60 degrees', () => {
    // 60 NM radius → 1° of latitude on each side.
    const b = radiusNmToBbox(50, 8, 60)
    expect(b.maxLat - 50).toBeCloseTo(1, 6)
    expect(50 - b.minLat).toBeCloseTo(1, 6)
  })

  it('widens longitude by 1/cos(lat) relative to latitude', () => {
    const lat = 50
    const b = radiusNmToBbox(lat, 8, 60)
    const lonHalf = (b.maxLon - b.minLon) / 2
    const latHalf = (b.maxLat - b.minLat) / 2
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
    expect(b.maxLat).toBeLessThanOrEqual(90)
    expect(b.minLon).toBeGreaterThanOrEqual(-180)
    expect(b.maxLon).toBeLessThanOrEqual(180)
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

  it('returns null for a missing or degenerate box', () => {
    expect(bboxToRadius(null)).toBeNull()
    expect(bboxToRadius({ minLat: 50, maxLat: 50, minLon: 8, maxLon: 8 })).toBeNull()
  })

  it('returns null when a corner is non-finite', () => {
    expect(bboxToRadius({ minLat: 50, maxLat: NaN, minLon: 8, maxLon: 9 })).toBeNull()
  })
})
