import { describe, it, expect } from 'vitest'
import {
  trackProvenance,
  isAdsbFresh,
  ADSB_FRESH_THRESHOLD_S,
  PROVENANCE_ADSB,
  PROVENANCE_FLARM,
  PROVENANCE_SSR,
  PROVENANCE_PSR,
  PROVENANCE_COMBINED,
  PROVENANCE_LABELS,
  PROVENANCE_LEGEND,
  COMBINED_LEGEND,
  filterProvenanceLegend,
} from '../provenance.js'

describe('filterProvenanceLegend (shared sidebar + scope legend)', () => {
  it('returns the full legend plus Kombiniert (K) when no sensor classes are known', () => {
    expect(filterProvenanceLegend([])).toEqual([...PROVENANCE_LEGEND, COMBINED_LEGEND])
    expect(filterProvenanceLegend(undefined)).toEqual([...PROVENANCE_LEGEND, COMBINED_LEGEND])
  })

  it('narrows to the entries a tenant\'s feeds can produce and appends K when ≥2 (#125)', () => {
    const got = filterProvenanceLegend(['ADS-B', 'PSR'])
    expect(got.map((e) => e.glyph)).toEqual(['A', '○', 'K'])
  })

  it('maps Mode S / MLAT classes to the SSR entry (single source → no K)', () => {
    expect(filterProvenanceLegend(['MODE_S']).map((e) => e.glyph)).toEqual(['■'])
    expect(filterProvenanceLegend(['MLAT']).map((e) => e.glyph)).toEqual(['■'])
  })
})

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
    for (const p of [PROVENANCE_COMBINED, PROVENANCE_ADSB, PROVENANCE_FLARM, PROVENANCE_SSR, PROVENANCE_PSR]) {
      expect(typeof PROVENANCE_LABELS[p]).toBe('string')
      expect(PROVENANCE_LABELS[p].length).toBeGreaterThan(0)
    }
  })
})

// #118 / ICD 2.6.0: FLARM carries its own age subfield (flarm_age_s) — a FLARM
// track is no longer misclassified as ADS-B or SSR.
describe('trackProvenance — FLARM (#118)', () => {
  it('returns flarm for a fresh FLARM age', () => {
    expect(trackProvenance({ flarm_age_s: 0 })).toBe(PROVENANCE_FLARM)
    expect(trackProvenance({ flarm_age_s: 12 })).toBe(PROVENANCE_FLARM)
  })

  it('prefers flarm over a cooperative id when only FLARM is fresh', () => {
    // OGN targets often carry a pseudo ICAO address — FLARM still wins.
    expect(trackProvenance({ flarm_age_s: 3, icao_addr: 0x3c6dd2 })).toBe(PROVENANCE_FLARM)
  })

  it('falls back from stale FLARM to ssr/psr like ADS-B does', () => {
    expect(trackProvenance({ flarm_age_s: 90, icao_addr: 0x3c6dd2 })).toBe(PROVENANCE_SSR)
    expect(trackProvenance({ flarm_age_s: 90 })).toBe(PROVENANCE_PSR)
  })
})

// #125 (from #90): ≥2 distinct surveillance technologies currently fresh →
// "combined" (a multi-sensor fused track), which outranks any single source.
describe('trackProvenance — combined (#125)', () => {
  it('returns combined when two technologies are fresh (ES + FLARM)', () => {
    expect(trackProvenance({ adsb_age_s: 2, flarm_age_s: 1 })).toBe(PROVENANCE_COMBINED)
  })

  it('returns combined for ES + Mode S both fresh', () => {
    expect(trackProvenance({ adsb_age_s: 3, mds_age_s: 4 })).toBe(PROVENANCE_COMBINED)
  })

  it('returns combined for SSR + Mode S both fresh', () => {
    expect(trackProvenance({ ssr_age_s: 5, mds_age_s: 6 })).toBe(PROVENANCE_COMBINED)
  })

  it('needs at least two FRESH ages — one fresh + one stale is not combined', () => {
    expect(trackProvenance({ adsb_age_s: 2, mds_age_s: 90 })).toBe(PROVENANCE_ADSB)
  })

  it('a single fresh SSR/Mode S age (no other) is ssr, not combined', () => {
    expect(trackProvenance({ ssr_age_s: 4 })).toBe(PROVENANCE_SSR)
    expect(trackProvenance({ mds_age_s: 4 })).toBe(PROVENANCE_SSR)
  })
})
