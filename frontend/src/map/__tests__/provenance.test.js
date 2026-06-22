import { describe, it, expect } from 'vitest'
import {
  trackProvenance,
  isAdsbFresh,
  ADSB_FRESH_THRESHOLD_S,
  PROVENANCE_ADSB,
  PROVENANCE_SSR,
  PROVENANCE_PSR,
  PROVENANCE_LABELS,
} from '../provenance.js'

describe('isAdsbFresh', () => {
  it('is false for absent age (undefined/null)', () => {
    expect(isAdsbFresh(undefined)).toBe(false)
    expect(isAdsbFresh(null)).toBe(false)
  })

  it('is true for a fresh age, including zero', () => {
    expect(isAdsbFresh(0)).toBe(true)
    expect(isAdsbFresh(5.5)).toBe(true)
  })

  it('is true exactly at the threshold and false just past it', () => {
    expect(isAdsbFresh(ADSB_FRESH_THRESHOLD_S)).toBe(true)
    expect(isAdsbFresh(ADSB_FRESH_THRESHOLD_S + 0.1)).toBe(false)
  })
})

describe('trackProvenance', () => {
  it('returns adsb when a fresh ADS-B age is present (presence + freshness)', () => {
    expect(trackProvenance({ adsb_age_s: 0 })).toBe(PROVENANCE_ADSB)
    expect(trackProvenance({ adsb_age_s: 12 })).toBe(PROVENANCE_ADSB)
  })

  it('prefers adsb over cooperative fields when ADS-B is fresh', () => {
    // A real ADS-B track is also Mode S (carries icao_addr/callsign); ADS-B wins.
    const t = { adsb_age_s: 3, icao_addr: 0x3c6dd2, callsign: 'DLH123', mode_3a: 0o1000 }
    expect(trackProvenance(t)).toBe(PROVENANCE_ADSB)
  })

  it('falls back from stale ADS-B to ssr when a cooperative id remains', () => {
    const t = { adsb_age_s: 90, icao_addr: 0x3c6dd2 }
    expect(trackProvenance(t)).toBe(PROVENANCE_SSR)
  })

  it('falls back from stale ADS-B to psr when nothing else identifies it', () => {
    expect(trackProvenance({ adsb_age_s: 90 })).toBe(PROVENANCE_PSR)
  })

  it('returns ssr for a Mode S address with no ADS-B', () => {
    expect(trackProvenance({ icao_addr: 0x3c6dd2 })).toBe(PROVENANCE_SSR)
  })

  it('returns ssr for a Mode 3/A code with no ADS-B', () => {
    expect(trackProvenance({ mode_3a: 0o7000 })).toBe(PROVENANCE_SSR)
  })

  it('returns ssr for a non-empty callsign with no ADS-B', () => {
    expect(trackProvenance({ callsign: 'BAW456' })).toBe(PROVENANCE_SSR)
  })

  it('treats an empty callsign as no identification', () => {
    expect(trackProvenance({ callsign: '' })).toBe(PROVENANCE_PSR)
  })

  it('returns psr for a primary-only track (no cooperative fields)', () => {
    expect(trackProvenance({ track_num: 42, latitude: 50, longitude: 8 })).toBe(PROVENANCE_PSR)
  })

  it('treats a zero Mode 3/A code as present (0o0000 is a valid squawk)', () => {
    expect(trackProvenance({ mode_3a: 0 })).toBe(PROVENANCE_SSR)
  })

  it('does not throw on null/undefined input', () => {
    expect(trackProvenance(null)).toBe(PROVENANCE_PSR)
    expect(trackProvenance(undefined)).toBe(PROVENANCE_PSR)
  })

  it('maps every provenance value to a human-readable label', () => {
    for (const p of [PROVENANCE_ADSB, PROVENANCE_SSR, PROVENANCE_PSR]) {
      expect(typeof PROVENANCE_LABELS[p]).toBe('string')
      expect(PROVENANCE_LABELS[p].length).toBeGreaterThan(0)
    }
  })
})
