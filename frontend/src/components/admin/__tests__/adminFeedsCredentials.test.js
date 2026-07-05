// Regression guard for UX-4: the source-credential UI is source-type aware.
//  - Radar (radar_asterix) has NO credential UI (network endpoint, no auth);
//  - ADS-B shows OpenSky-labelled fields, FLARM shows APRS-IS-labelled fields;
//  - the credential reference is auto-managed (no hand-invented handle);
//  - when the secret store is off, a clear hint replaces the dead field.
// Source-level assertions (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import sfc from '../AdminFeeds.vue?raw'

describe('feed source credentials are source-type aware (UX-4)', () => {
  it('only ADS-B/FLARM carry a credential block (radar excluded)', () => {
    // The block is gated on credInfo(type); the map keys are adsb/flarm only.
    expect(sfc).toContain('credInfo(s.type)')
    expect(sfc).toContain('adsb_opensky:')
    expect(sfc).toContain('flarm_aprs:')
    expect(sfc).not.toContain('radar_asterix:') // radar is not a CREDENTIAL key
  })

  it('labels the fields per source type', () => {
    expect(sfc).toContain('OpenSky Client-ID')
    expect(sfc).toContain('OpenSky Client-Secret')
    expect(sfc).toContain('APRS-IS Rufzeichen')
    expect(sfc).toContain('APRS-IS Passcode')
  })

  it('drops the hand-invented reference field and auto-manages the ref', () => {
    expect(sfc).not.toContain('Credential-Referenz (optional)')
    expect(sfc).toContain('function ensureCredRef')
    expect(sfc).toContain('sources.value.forEach(ensureCredRef)')
  })

  it('explains the disabled secret store instead of showing a dead field', () => {
    expect(sfc).toContain('Secret-Store deaktiviert')
    expect(sfc).toContain('WAYFINDER_SECRET_KEY')
  })
})

describe('cred_ref is regenerated on source-type change (#198)', () => {
  it('ensureCredRef re-derives unconditionally (no "only if empty" guard)', () => {
    // The stale guard that kept a mismatched "…-adsb_opensky" ref after switching
    // to FLARM is gone; the ref is now always derived from (feed_id, type).
    expect(sfc).not.toContain("if (!(s.cred_ref || '').trim())")
    expect(sfc).toContain('secret/feed-${sourcesTarget.value?.id ?? \'new\'}-${s.type}')
  })

  it('an anonymous optional source sends no dangling cred_ref', () => {
    // buildSourcesPayload only emits cred_ref when required OR a secret is
    // stored/typed — so anonymous FLARM carries none (no "secret unresolved" log).
    expect(sfc).toContain('info.required || isSecretConfigured(ref) || secretTyped(i)')
  })
})

describe('one "Speichern" persists the source AND its credential (#199)', () => {
  it('submitSources blocks a required credential that is neither stored nor typed', () => {
    expect(sfc).toContain('Zugang erforderlich, aber weder hinterlegt noch eingegeben')
    expect(sfc).toContain('info.required && !configured && !secretTyped(i)')
  })

  it('submitSources saves typed secrets in the same action', () => {
    expect(sfc).toContain('if (secretTyped(i) && !(await saveSecret(i)))')
  })

  it('reports an honest result when a credential fails to persist', () => {
    expect(sfc).toContain('Quellen gespeichert, aber ein Zugang konnte nicht hinterlegt werden')
    // saveSecret returns a boolean so the overall result can be judged.
    expect(sfc).toContain('return true')
    expect(sfc).toContain('return false')
  })
})
