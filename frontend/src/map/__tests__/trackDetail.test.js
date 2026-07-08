import { describe, it, expect } from 'vitest'
import {
  formatLatLon,
  formatHeading,
  formatIcao,
  formatAccuracy,
  formatAge,
  verticalTrendLabel,
  sensorAgeList,
} from '../trackDetail.js'

describe('formatLatLon', () => {
  it('formats northern/eastern hemisphere with 4 decimals', () => {
    expect(formatLatLon(53.6304, 9.9882)).toBe('53.6304° N, 9.9882° E')
  })
  it('uses S/W for negative components and drops the sign', () => {
    expect(formatLatLon(-33.9425, -18.4231)).toBe('33.9425° S, 18.4231° W')
  })
  it('treats the equator/prime meridian (0) as N/E', () => {
    expect(formatLatLon(0, 0)).toBe('0.0000° N, 0.0000° E')
  })
  it('returns empty string when a component is missing', () => {
    expect(formatLatLon(50, undefined)).toBe('')
    expect(formatLatLon(null, 8)).toBe('')
  })
})

describe('formatHeading', () => {
  it('north (Vx=0, Vy>0) is 000°', () => {
    expect(formatHeading(0, 100)).toBe('000°')
  })
  it('east (Vx>0, Vy=0) is 090°', () => {
    expect(formatHeading(100, 0)).toBe('090°')
  })
  it('south (Vx=0, Vy<0) is 180°', () => {
    expect(formatHeading(0, -100)).toBe('180°')
  })
  it('west (Vx<0, Vy=0) is 270°', () => {
    expect(formatHeading(-100, 0)).toBe('270°')
  })
  it('north-east is 045°', () => {
    expect(formatHeading(50, 50)).toBe('045°')
  })
  it('normalises a value that rounds up to 360 back to 000°', () => {
    // atan2 gives ~359.7° here → rounds to 360 → must wrap to 0.
    expect(formatHeading(-0.5, 100)).toBe('000°')
  })
  it('returns empty string for a stationary track', () => {
    expect(formatHeading(0, 0)).toBe('')
  })
  it('returns empty string when velocity is missing', () => {
    expect(formatHeading(undefined, undefined)).toBe('')
  })
})

describe('formatIcao', () => {
  it('renders a 6-digit uppercase hex address', () => {
    expect(formatIcao(0x3c6dd2)).toBe('3C6DD2')
  })
  it('zero-pads a small address to 6 digits', () => {
    expect(formatIcao(0xff)).toBe('0000FF')
  })
  it('returns empty string when absent', () => {
    expect(formatIcao(null)).toBe('')
    expect(formatIcao(undefined)).toBe('')
  })
})

describe('formatAccuracy', () => {
  it('renders metres with a ± sign, rounded', () => {
    expect(formatAccuracy(42.6)).toBe('±43 m')
  })
  it('returns empty string for missing/non-positive values', () => {
    expect(formatAccuracy(0)).toBe('')
    expect(formatAccuracy(-1)).toBe('')
    expect(formatAccuracy(null)).toBe('')
    expect(formatAccuracy(Infinity)).toBe('')
  })
})

describe('formatAge', () => {
  it('uses one decimal below 10 s', () => {
    expect(formatAge(2.34)).toBe('2.3 s')
  })
  it('uses whole seconds at/above 10 s', () => {
    expect(formatAge(12.7)).toBe('13 s')
  })
  it('returns empty string for missing values', () => {
    expect(formatAge(null)).toBe('')
    expect(formatAge(undefined)).toBe('')
  })
})

describe('verticalTrendLabel', () => {
  it('maps the climb/descent glyphs to German words', () => {
    expect(verticalTrendLabel('▲')).toBe('Steigend')
    expect(verticalTrendLabel('▼')).toBe('Sinkend')
  })
  it('treats empty/unknown as level flight', () => {
    expect(verticalTrendLabel('')).toBe('Gleichbleibend')
    expect(verticalTrendLabel(undefined)).toBe('Gleichbleibend')
  })
})

describe('sensorAgeList', () => {
  it('lists only technologies whose age is present, in display order', () => {
    const list = sensorAgeList({ adsb_age_s: 2, ssr_age_s: 40, mode_3a: 1234 })
    expect(list.map((s) => s.key)).toEqual(['adsb_age_s', 'ssr_age_s'])
    expect(list[0]).toMatchObject({ label: 'ADS-B', ageS: 2, fresh: true })
    expect(list[1]).toMatchObject({ label: 'SSR (Mode A/C)', ageS: 40, fresh: false })
  })
  it('returns an empty list for a primary-only track (no per-tech ages)', () => {
    expect(sensorAgeList({ psr_age: 5 })).toEqual([])
  })
  it('handles a null track', () => {
    expect(sensorAgeList(null)).toEqual([])
  })
})
