// Regression guard for the community-aggregator source type (#201; Firefly
// contract v1.5.0, ADR 0031 there):
//  - the type appears in the closed vocabulary with a clean human-readable label;
//  - it is area-bounded (centre+radius editor) and polled (interval field);
//  - the provider select offers human-readable names, wire values stay internal,
//    and airplanes.live is deliberately absent (unverified radius unit);
//  - the payload sends `provider` only for this type;
//  - the type is auth-free: NO CREDENTIAL entry, so the credential block never
//    renders and no cred_ref is emitted (the #198 fix drops refs without info).
// Source-level assertions (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import sfc from '../AdminFeeds.vue?raw'

describe('community-aggregator source type (#201)', () => {
  it('is offered in the type vocabulary with a clean label', () => {
    expect(sfc).toContain("{ value: 'adsb_aggregator', label: 'ADS-B (Community-Aggregator)' }")
    // Area-bounded like the other internet sources.
    expect(sfc).toContain("AREA_TYPES = new Set(['adsb_opensky', 'adsb_aggregator', 'flarm_aprs'])")
  })

  it('offers the providers with human-readable labels, wire values internal', () => {
    expect(sfc).toContain("{ value: 'adsb_lol', label: 'adsb.lol' }")
    expect(sfc).toContain("{ value: 'adsb_fi', label: 'adsb.fi' }")
    expect(sfc).toContain("DEFAULT_AGG_PROVIDER = 'adsb_lol'")
    // airplanes.live is deferred until its radius unit is verified (ADR 0031).
    expect(sfc).not.toContain('airplanes_live')
  })

  it('gates the provider select to the aggregator type', () => {
    expect(sfc).toContain(`v-if="s.type === 'adsb_aggregator'"`)
    expect(sfc).toContain(':items="AGG_PROVIDERS"')
  })

  it('sends provider only for the aggregator type', () => {
    expect(sfc).toContain("s.type === 'adsb_aggregator' && s.provider")
    expect(sfc).toContain('out.provider = s.provider')
  })

  it('carries the provider through the form round-trip helpers', () => {
    // blankSource + toFormSource must both know the field, else it is dropped on
    // add / reload.
    expect(sfc).toContain('provider: DEFAULT_AGG_PROVIDER')
    expect(sfc).toContain('provider: s.provider ?? DEFAULT_AGG_PROVIDER')
  })

  it('is auth-free: no CREDENTIAL entry for the type', () => {
    // The CREDENTIAL map drives the credential UI and the cred_ref lifecycle
    // (ensureCredRef clears the ref for types without an entry) — the aggregator
    // must not appear in it.
    const credBlock = sfc.slice(sfc.indexOf('const CREDENTIAL = {'), sfc.indexOf('function credInfo'))
    expect(credBlock).toContain('adsb_opensky:')
    expect(credBlock).toContain('flarm_aprs:')
    expect(credBlock).not.toContain('adsb_aggregator')
  })
})
