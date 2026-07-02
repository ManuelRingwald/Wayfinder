import { describe, it, expect } from 'vitest'
import { validateView } from '@/admin/validateView.js'

// A well-formed config used as the baseline; each test perturbs one field.
function valid() {
  return {
    center_lat: 50,
    center_lon: 9,
    zoom: 8,
    aoi: { min_lat: 49, min_lon: 7, max_lat: 51, max_lon: 11 },
    fl_min: 0,
    fl_max: 400,
    layers: { airspace: true },
  }
}

describe('validateView (parity with server pkg/adminapi.validateView)', () => {
  it('accepts a well-formed config', () => {
    expect(validateView(valid())).toEqual([])
  })

  it('accepts the minimal config (no AOI, no FL band)', () => {
    expect(validateView({ center_lat: 0, center_lon: 0, zoom: 0 })).toEqual([])
  })

  it.each([
    ['center_lat too high', { center_lat: 91 }, /center_lat/],
    ['center_lat too low', { center_lat: -91 }, /center_lat/],
    ['center_lat missing', { center_lat: undefined }, /center_lat/],
    ['center_lon too high', { center_lon: 181 }, /center_lon/],
    ['center_lon too low', { center_lon: -181 }, /center_lon/],
    ['zoom too high', { zoom: 25 }, /zoom/],
    ['zoom negative', { zoom: -1 }, /zoom/],
  ])('rejects %s', (_name, patch, re) => {
    const errs = validateView({ ...valid(), ...patch })
    expect(errs.some((e) => re.test(e))).toBe(true)
  })

  it('rejects an AOI out of range', () => {
    const errs = validateView({ ...valid(), aoi: { min_lat: 49, min_lon: 7, max_lat: 95, max_lon: 11 } })
    expect(errs).toContain('aoi out of range')
  })

  it('rejects an inverted AOI (min > max)', () => {
    const errs = validateView({ ...valid(), aoi: { min_lat: 51, min_lon: 7, max_lat: 49, max_lon: 11 } })
    expect(errs).toContain('aoi min must be <= max')
  })

  it('rejects a negative FL bound', () => {
    expect(validateView({ ...valid(), fl_min: -1 })).toContain('fl_min must be >= 0')
    expect(validateView({ ...valid(), fl_max: -1 })).toContain('fl_max must be >= 0')
  })

  it('rejects an inverted FL band (min > max)', () => {
    expect(validateView({ ...valid(), fl_min: 400, fl_max: 100 })).toContain('fl_min must be <= fl_max')
  })

  it('does not flag FL bounds when the band is absent', () => {
    const d = valid()
    delete d.fl_min
    delete d.fl_max
    expect(validateView(d)).toEqual([])
  })

  it('accepts a short ICAO label', () => {
    expect(validateView({ ...valid(), icao: 'EDGG·KTG' })).toEqual([])
  })

  it('rejects an over-long ICAO label', () => {
    expect(validateView({ ...valid(), icao: 'ABCDEFGHIJKLM' })).toContain('icao label too long')
  })
})
