// Regression guard for the OpenSky poll-interval field (ADR 0029, Wayfinder #3):
//  - the field + info box render only for adsb_opensky (FLARM is a push stream);
//  - the payload carries poll_interval_secs only for OpenSky and only when set;
//  - the bounds mirror the server's write-boundary range.
// Source-level assertions (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import sfc from '../AdminFeeds.vue?raw'

describe('OpenSky poll interval (ADR 0029)', () => {
  it('the poll field is gated to adsb_opensky only', () => {
    // The whole poll block (field + info alert) sits behind an OpenSky type check.
    expect(sfc).toContain("s.type === 'adsb_opensky'")
    expect(sfc).toContain('Poll-Intervall')
  })

  it('shows an OpenSky rate-limit info box', () => {
    expect(sfc).toContain('ratenbegrenzt')
    expect(sfc).toContain('429')
  })

  it('sends poll_interval_secs only for OpenSky and only when set', () => {
    // The payload builder guards on the OpenSky type AND a non-empty value.
    expect(sfc).toContain('out.poll_interval_secs = Number(s.poll_interval_secs)')
    expect(sfc).toContain("s.type === 'adsb_opensky' && s.poll_interval_secs != null")
  })

  it('carries the field through the form round-trip helpers', () => {
    // blankSource + toFormSource must both know the field, else it is dropped on
    // add / reload.
    expect(sfc).toContain('poll_interval_secs: null')
    expect(sfc).toContain('poll_interval_secs: s.poll_interval_secs ?? null')
  })

  it('mirrors the server bounds (5..3600, default 10)', () => {
    expect(sfc).toContain('DEFAULT_POLL_SECS = 10')
    expect(sfc).toContain('MIN_POLL_SECS = 5')
    expect(sfc).toContain('MAX_POLL_SECS = 3600')
  })
})
