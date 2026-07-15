import { describe, it, expect } from 'vitest'
import {
  formatLatLon,
  formatHeading,
  formatIcao,
  formatAccuracy,
  formatAge,
  verticalTrendLabel,
  sensorAgeList,
  formatSelectedAltitude,
  formatMagneticHeading,
  formatIas,
  formatMach,
  isLevelBust,
  formatGeometricAltitude,
  formatBarometricAltitude,
  formatRateOfClimb,
  formatCourseTrend,
  formatSpeedTrend,
  formatVerticalMotion,
  formatAcceleration,
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

// #238: Mode-S DAP formatters + level-bust logic.
describe('formatSelectedAltitude', () => {
  it('renders feet as a flight level', () => {
    expect(formatSelectedAltitude(35000)).toBe('FL350')
    expect(formatSelectedAltitude(500)).toBe('FL005')
  })
  it('returns "" for an absent value', () => {
    expect(formatSelectedAltitude(null)).toBe('')
    expect(formatSelectedAltitude(undefined)).toBe('')
  })
})

describe('isLevelBust', () => {
  it('flags a selected altitude that differs from the FL beyond the threshold', () => {
    expect(isLevelBust(35000, 20000)).toBe(true)   // climbing to FL350 from FL200
    expect(isLevelBust(35000, 35000)).toBe(false)  // level at the selected altitude
    expect(isLevelBust(35000, 34800)).toBe(false)  // within 300 ft
    expect(isLevelBust(35000, 34700)).toBe(true)   // exactly 300 ft
  })
  it('never flags when a value is missing (fail-safe: no invented alarm)', () => {
    expect(isLevelBust(35000, null)).toBe(false)
    expect(isLevelBust(undefined, 20000)).toBe(false)
  })
})

describe('formatMagneticHeading / formatIas / formatMach', () => {
  it('renders heading zero-padded and normalised', () => {
    expect(formatMagneticHeading(270)).toBe('270°')
    expect(formatMagneticHeading(5)).toBe('005°')
    expect(formatMagneticHeading(360)).toBe('000°')
  })
  it('renders IAS in knots and Mach to three decimals', () => {
    expect(formatIas(250)).toBe('250 kt')
    expect(formatMach(0.784)).toBe('M0.784')
  })
  it('returns "" for absent values', () => {
    expect(formatMagneticHeading(null)).toBe('')
    expect(formatIas(undefined)).toBe('')
    expect(formatMach(null)).toBe('')
  })
})

// Vertical chain (I062/130/135/220, ICD 3.5.0, #241).
describe('formatGeometricAltitude', () => {
  it('renders whole feet', () => {
    expect(formatGeometricAltitude(10000)).toBe('10000 ft')
    expect(formatGeometricAltitude(-625)).toBe('-625 ft')
  })
  it('returns "" for absent/non-finite values', () => {
    expect(formatGeometricAltitude(null)).toBe('')
    expect(formatGeometricAltitude(undefined)).toBe('')
    expect(formatGeometricAltitude(NaN)).toBe('')
  })
})

describe('formatBarometricAltitude', () => {
  it('renders a QNH-corrected value as feet with a QNH marker', () => {
    expect(formatBarometricAltitude(3000, true)).toBe('3000 ft (QNH)')
  })
  it('renders an uncorrected value as a standard flight level', () => {
    expect(formatBarometricAltitude(35000, false)).toBe('FL350 (Standard)')
  })
  it('returns "" for absent values', () => {
    expect(formatBarometricAltitude(null, true)).toBe('')
    expect(formatBarometricAltitude(undefined, false)).toBe('')
  })
})

describe('formatRateOfClimb', () => {
  it('renders a positive rate with a leading plus', () => {
    expect(formatRateOfClimb(3000)).toBe('+3000 ft/min')
  })
  it('renders a negative rate with its sign', () => {
    expect(formatRateOfClimb(-1200)).toBe('-1200 ft/min')
  })
  it('renders zero without a sign', () => {
    expect(formatRateOfClimb(0)).toBe('0 ft/min')
  })
  it('returns "" for absent values', () => {
    expect(formatRateOfClimb(null)).toBe('')
    expect(formatRateOfClimb(undefined)).toBe('')
  })
})

// Kinematics chain (I062/200/210, ICD 3.6.0, #242).
describe('formatCourseTrend / formatSpeedTrend / formatVerticalMotion', () => {
  it('words the course trend', () => {
    expect(formatCourseTrend('right')).toBe('Rechtskurve')
    expect(formatCourseTrend('left')).toBe('Linkskurve')
    expect(formatCourseTrend('constant')).toBe('Konstanter Kurs')
  })
  it('words the speed trend', () => {
    expect(formatSpeedTrend('increasing')).toBe('Zunehmend')
    expect(formatSpeedTrend('decreasing')).toBe('Abnehmend')
    expect(formatSpeedTrend('constant')).toBe('Konstant')
  })
  it('words the vertical motion', () => {
    expect(formatVerticalMotion('climb')).toBe('Steigen')
    expect(formatVerticalMotion('descent')).toBe('Sinken')
    expect(formatVerticalMotion('level')).toBe('Level')
  })
  it('returns "" for an absent/undetermined axis', () => {
    expect(formatCourseTrend(null)).toBe('')
    expect(formatSpeedTrend(undefined)).toBe('')
    expect(formatVerticalMotion(null)).toBe('')
  })
})

describe('formatAcceleration', () => {
  it('renders the magnitude of the Ax/Ay components to one decimal', () => {
    expect(formatAcceleration(1.0, -0.5)).toBe('1.1 m/s²') // hypot(1, 0.5) ≈ 1.118
    expect(formatAcceleration(3, 4)).toBe('5.0 m/s²')
  })
  it('returns "" when either component is missing', () => {
    expect(formatAcceleration(1.0, null)).toBe('')
    expect(formatAcceleration(undefined, 0.5)).toBe('')
  })
})
