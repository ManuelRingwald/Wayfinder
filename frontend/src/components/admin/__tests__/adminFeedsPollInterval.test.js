// Regression guard for the poll-interval field (ADR 0029/0031, Wayfinder #3/#201):
//  - the field + info box render only for the POLLED sources (OpenSky and the
//    community aggregator — FLARM is a push stream, radar has its own scan);
//  - the payload carries poll_interval_secs only for polled types and only when set;
//  - the bounds mirror the server's write-boundary range.
// Source-level assertions (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import sfc from '../AdminFeeds.vue?raw'

describe('poll interval for polled sources (ADR 0029/0031)', () => {
  it('the poll field is gated to the polled types', () => {
    // The whole poll block (field + info alert) sits behind the polled-type check.
    expect(sfc).toContain('v-if="isPolledType(s.type)"')
    expect(sfc).toContain('Poll-Intervall')
    expect(sfc).toContain("POLLED_TYPES = new Set(['adsb_opensky', 'adsb_aggregator'])")
  })

  it('shows an OpenSky rate-limit info box and an aggregator politeness box', () => {
    expect(sfc).toContain('ratenbegrenzt')
    expect(sfc).toContain('429')
    // The aggregator variant explains the community service's public limit.
    expect(sfc).toContain('1 Abfrage/s')
  })

  it('sends poll_interval_secs only for polled types and only when set', () => {
    // The payload builder guards on the polled type AND a non-empty value.
    expect(sfc).toContain('out.poll_interval_secs = Number(s.poll_interval_secs)')
    expect(sfc).toContain('isPolledType(s.type) && s.poll_interval_secs != null')
  })

  it('carries the field through the form round-trip helpers', () => {
    // blankSource + toFormSource must both know the field, else it is dropped on
    // add / reload.
    expect(sfc).toContain('poll_interval_secs: s.poll_interval_secs ?? null')
  })

  it('prefills a fresh polled source with the visible default (#172)', () => {
    // Option A: a new polled source shows "10" rather than an empty field, so
    // the operator sees which interval applies at a glance.
    expect(sfc).toContain('poll_interval_secs: isPolledType(type) ? DEFAULT_POLL_SECS : null')
  })

  it('mirrors the server bounds (5..3600, default 10)', () => {
    expect(sfc).toContain('DEFAULT_POLL_SECS = 10')
    expect(sfc).toContain('MIN_POLL_SECS = 5')
    expect(sfc).toContain('MAX_POLL_SECS = 3600')
  })
})
