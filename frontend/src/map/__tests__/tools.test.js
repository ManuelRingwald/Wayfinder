import { describe, it, expect } from 'vitest'
import { haversineNM, bearingDeg, formatNM, formatBearing, measureText } from '@/map/tools.js'

describe('haversineNM', () => {
  it('is ~0 for coincident points', () => {
    expect(haversineNM({ lng: 8, lat: 50 }, { lng: 8, lat: 50 })).toBeCloseTo(0, 6)
  })

  it('one degree of latitude is ~60 NM', () => {
    // A degree of latitude ≈ 60 nautical miles by definition of the NM.
    const d = haversineNM({ lng: 0, lat: 0 }, { lng: 0, lat: 1 })
    expect(d).toBeGreaterThan(59.9)
    expect(d).toBeLessThan(60.1)
  })

  it('is symmetric', () => {
    const a = { lng: 8.5, lat: 50.0 }
    const b = { lng: 9.2, lat: 49.4 }
    expect(haversineNM(a, b)).toBeCloseTo(haversineNM(b, a), 9)
  })
})

describe('bearingDeg', () => {
  it('due north is 0°', () => {
    expect(bearingDeg({ lng: 0, lat: 0 }, { lng: 0, lat: 1 })).toBeCloseTo(0, 6)
  })

  it('due east is ~90° at the equator', () => {
    expect(bearingDeg({ lng: 0, lat: 0 }, { lng: 1, lat: 0 })).toBeCloseTo(90, 3)
  })

  it('due south is 180°', () => {
    expect(bearingDeg({ lng: 0, lat: 1 }, { lng: 0, lat: 0 })).toBeCloseTo(180, 6)
  })

  it('due west is ~270° at the equator', () => {
    expect(bearingDeg({ lng: 1, lat: 0 }, { lng: 0, lat: 0 })).toBeCloseTo(270, 3)
  })
})

describe('formatNM / formatBearing', () => {
  it('formats distance to one decimal + unit', () => {
    expect(formatNM(12.34)).toBe('12.3 NM')
    expect(formatNM(0)).toBe('0.0 NM')
  })

  it('zero-pads bearings to three digits', () => {
    expect(formatBearing(5)).toBe('005°')
    expect(formatBearing(90)).toBe('090°')
    expect(formatBearing(359.6)).toBe('000°') // 360 wraps to 000
  })
})

describe('measureText', () => {
  it('combines distance and bearing', () => {
    const t = measureText({ lng: 0, lat: 0 }, { lng: 0, lat: 1 })
    expect(t).toMatch(/^\d+\.\d NM · \d{3}°$/)
    expect(t).toContain('· 000°') // due north
  })
})
