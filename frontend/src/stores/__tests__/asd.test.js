import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAsdStore } from '@/stores/asd.js'

beforeEach(() => {
  setActivePinia(createPinia())
})

// installFetch stubs global fetch with a table keyed by "METHOD path". Each entry
// is { status, body }. It records every call so tests can assert URL/method/body.
// (Mirrors the helper in admin.test.js — both drive apiFetch.)
function installFetch(table) {
  const calls = []
  globalThis.fetch = vi.fn(async (url, opts = {}) => {
    const method = (opts.method || 'GET').toUpperCase()
    calls.push({ url, method, body: opts.body })
    const entry = table[`${method} ${url}`]
    if (!entry) {
      return { ok: false, status: 404, text: async () => JSON.stringify({ error: 'not found' }) }
    }
    return {
      ok: entry.status >= 200 && entry.status < 300,
      status: entry.status,
      text: async () => (entry.body !== undefined ? JSON.stringify(entry.body) : ''),
    }
  })
  return calls
}

// #176: the standalone "Lufträume" parent toggle was removed; the airspace layer
// visibility is derived from the four group toggles (visible iff any is on).
describe('asd store — airspace layer visibility derived from groups (#176)', () => {
  it('stays visible while any group is on, hides when all are off', () => {
    const s = useAsdStore()
    expect(s.layerVisibility.airspace).toBe(true) // all groups on by default
    s.setAirspaceGroup('ctr', false)
    s.setAirspaceGroup('tma', false)
    s.setAirspaceGroup('restricted', false)
    expect(s.layerVisibility.airspace).toBe(true) // info still on
    s.setAirspaceGroup('info', false)
    expect(s.layerVisibility.airspace).toBe(false) // all off → layer hidden
    s.setAirspaceGroup('tma', true)
    expect(s.layerVisibility.airspace).toBe(true) // one back on → visible
    expect(s.airspaceGroupVisibility.tma).toBe(true)
  })

  it('toggleAirspaceGroup flips a group and re-derives visibility', () => {
    const s = useAsdStore()
    ;['ctr', 'tma', 'restricted', 'info'].forEach((g) => s.setAirspaceGroup(g, false))
    expect(s.layerVisibility.airspace).toBe(false)
    s.toggleAirspaceGroup('ctr')
    expect(s.airspaceGroupVisibility.ctr).toBe(true)
    expect(s.layerVisibility.airspace).toBe(true)
  })
})

// #117: the broadcast FeedStatusMessage speaks per-feed colors
// (green/yellow/red); the store maps them to chip states and aggregates
// worst-wins across all subscribed feeds.
describe('asd store — per-feed health aggregation (#117)', () => {
  it('starts unknown until the first feed status arrives', () => {
    const s = useAsdStore()
    expect(s.feedStatus).toBe('unknown')
  })

  it('maps the wire colors to chip states', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'green')
    expect(s.feedStatus).toBe('ok')
    s.setFeedHealth(1, 'yellow')
    expect(s.feedStatus).toBe('degraded')
    s.setFeedHealth(1, 'red')
    expect(s.feedStatus).toBe('stale')
  })

  it('aggregates worst-wins across feeds (a dead feed is never masked)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'green')
    s.setFeedHealth(2, 'red')
    expect(s.feedStatus).toBe('stale')
    s.setFeedHealth(2, 'green')
    expect(s.feedStatus).toBe('ok')
  })

  it('ignores an unknown color instead of corrupting the chip', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'green')
    s.setFeedHealth(1, 'chartreuse')
    expect(s.feedStatus).toBe('ok')
  })

  it('resetFeedHealth returns to unknown (WS reconnect drops stale scope)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'red')
    s.resetFeedHealth()
    expect(s.feedStatus).toBe('unknown')
  })

  it('carries per-sensor detail and flattens it into sensorDetails (#237)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'yellow', 'unreachable', [
      { sac: 0, sic: 1, operational: true, range_bias_m: 145, azimuth_bias_deg: 0.3 },
      { sac: 0, sic: 2, operational: false, degraded_reason: 'unreachable' },
    ])
    s.setFeedHealth(2, 'green', '', [{ sac: 0, sic: 5, operational: true }])
    const byFeed = s.sensorDetails.reduce((m, x) => { (m[x.feedId] ??= []).push(x); return m }, {})
    expect(byFeed[1]).toHaveLength(2)
    expect(byFeed[1][0].range_bias_m).toBe(145)
    expect(byFeed[1][0].feedId).toBe(1)
    expect(byFeed[2]).toHaveLength(1)
    // resetFeedHealth also clears the per-sensor detail.
    s.resetFeedHealth()
    expect(s.sensorDetails).toHaveLength(0)
  })

  it('surfaces the degraded reason from the CAT063 RE field (#197)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'yellow', 'auth')
    expect(s.feedStatus).toBe('degraded')
    expect(s.feedDegradedReason).toBe('auth')
  })

  it('shows no reason unless the aggregate state is degraded', () => {
    const s = useAsdStore()
    // A healthy feed with a (stale) reason must not leak a reason onto the chip.
    s.setFeedHealth(1, 'green', '')
    expect(s.feedDegradedReason).toBe('')
    // A dead (stale) feed outranks degraded, so no degraded reason is shown.
    s.setFeedHealth(2, 'red', '')
    s.setFeedHealth(3, 'yellow', 'unreachable')
    expect(s.feedStatus).toBe('stale')
    expect(s.feedDegradedReason).toBe('')
  })

  it('picks the most actionable reason across degraded feeds (auth > rate_limited > unreachable)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'yellow', 'unreachable')
    s.setFeedHealth(2, 'yellow', 'auth')
    s.setFeedHealth(3, 'yellow', 'rate_limited')
    expect(s.feedDegradedReason).toBe('auth')
  })

  it('resetFeedHealth also clears reasons', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'yellow', 'auth')
    s.resetFeedHealth()
    expect(s.feedDegradedReason).toBe('')
  })
})

// WX-A: DWD weather-radar overlay is off by default and gated by an
// availability flag set from /api/map-config (weather_radar_available).
describe('asd store — weather radar overlay (WX-A)', () => {
  it('is off by default and reports unavailable until configured', () => {
    const s = useAsdStore()
    expect(s.layerVisibility.weatherRadar).toBe(false)
    expect(s.weatherRadarAvailable).toBe(false)
  })

  it('setWeatherRadarAvailable coerces to a boolean', () => {
    const s = useAsdStore()
    s.setWeatherRadarAvailable(true)
    expect(s.weatherRadarAvailable).toBe(true)
    s.setWeatherRadarAvailable(0)
    expect(s.weatherRadarAvailable).toBe(false)
  })

  it('setLayerVisibility toggles the weatherRadar layer', () => {
    const s = useAsdStore()
    s.setLayerVisibility('weatherRadar', true)
    expect(s.layerVisibility.weatherRadar).toBe(true)
  })
})

// WX-C: DWD weather-warnings overlay — same off-by-default + availability gate.
describe('asd store — weather warnings overlay (WX-C)', () => {
  it('is off by default and reports unavailable until configured', () => {
    const s = useAsdStore()
    expect(s.layerVisibility.weatherWarnings).toBe(false)
    expect(s.weatherWarningsAvailable).toBe(false)
  })

  it('setWeatherWarningsAvailable coerces to a boolean', () => {
    const s = useAsdStore()
    s.setWeatherWarningsAvailable(1)
    expect(s.weatherWarningsAvailable).toBe(true)
    s.setWeatherWarningsAvailable('')
    expect(s.weatherWarningsAvailable).toBe(false)
  })
})

// ASD-013: the live-track set feeds the Ereignis-Panel's decision whether a
// "Track N erschienen" event still points at a selectable track.
describe('asd store — live-track set for the event panel (ASD-013)', () => {
  it('starts empty and mirrors the numbers pushed from the engine', () => {
    const s = useAsdStore()
    expect(s.liveTrackNums.size).toBe(0)
    s.setLiveTrackNums([12, 36, 7])
    expect(s.liveTrackNums.has(36)).toBe(true)
    expect(s.liveTrackNums.has(99)).toBe(false)
    expect(s.liveTrackNums.size).toBe(3)
  })

  it('accepts a Set directly and replaces the previous set', () => {
    const s = useAsdStore()
    s.setLiveTrackNums([1, 2])
    s.setLiveTrackNums(new Set([2, 3]))
    expect(s.liveTrackNums.has(1)).toBe(false) // replaced, not merged
    expect(s.liveTrackNums.has(3)).toBe(true)
  })
})

// #245 Teil B (ADR 0024): manual flight-plan correlation. The availability flag
// gates the UI (set from /api/map-config); the three actions post to the command
// endpoint and normalise the HTTP result into a friendly { ok, message }.
describe('asd store — manual correlation availability gate (#245 Teil B)', () => {
  it('is unavailable by default and coerces the setter to a boolean', () => {
    const s = useAsdStore()
    expect(s.correlationAvailable).toBe(false)
    s.setCorrelationAvailable(1)
    expect(s.correlationAvailable).toBe(true)
    s.setCorrelationAvailable('')
    expect(s.correlationAvailable).toBe(false)
  })
})

describe('asd store — manual correlation commands (#245 Teil B)', () => {
  it('correlate POSTs feed_id/track_number/callsign and reports success on 204', async () => {
    const calls = installFetch({ 'POST /api/correlation': { status: 204 } })
    const s = useAsdStore()
    const r = await s.correlate(42, 1234, 'DLH123')
    expect(r.ok).toBe(true)
    expect(r.message).toContain('DLH123')
    expect(calls).toHaveLength(1)
    expect(calls[0].method).toBe('POST')
    expect(calls[0].url).toBe('/api/correlation')
    expect(JSON.parse(calls[0].body)).toEqual({ feed_id: 42, track_number: 1234, callsign: 'DLH123' })
  })

  it('setUncorrelated POSTs a null callsign (the uncorrelate signal)', async () => {
    const calls = installFetch({ 'POST /api/correlation': { status: 204 } })
    const s = useAsdStore()
    const r = await s.setUncorrelated(42, 7)
    expect(r.ok).toBe(true)
    expect(JSON.parse(calls[0].body)).toEqual({ feed_id: 42, track_number: 7, callsign: null })
  })

  it('clearOverride DELETEs the feed/track path', async () => {
    const calls = installFetch({ 'DELETE /api/correlation/42/7': { status: 204 } })
    const s = useAsdStore()
    const r = await s.clearOverride(42, 7)
    expect(r.ok).toBe(true)
    expect(calls[0].method).toBe('DELETE')
    expect(calls[0].url).toBe('/api/correlation/42/7')
  })

  it('maps a 422 (no such plan) to the German controller message', async () => {
    installFetch({ 'POST /api/correlation': { status: 422, body: { error: 'no filed flight plan for that callsign' } } })
    const s = useAsdStore()
    const r = await s.correlate(42, 1, 'ZZZ999')
    expect(r.ok).toBe(false)
    expect(r.message).toBe('Kein Flugplan mit dieser Kennung gefunden.')
  })

  it('maps a 409 (no plans configured) and a 403 (not authorised) to their messages', async () => {
    installFetch({ 'POST /api/correlation': { status: 409 } })
    let r = await useAsdStore().correlate(42, 1, 'DLH1')
    expect(r.message).toContain('keine Flugpläne')

    installFetch({ 'POST /api/correlation': { status: 403 } })
    r = await useAsdStore().correlate(42, 1, 'DLH1')
    expect(r.message).toBe('Für diesen Feed nicht berechtigt.')
  })

  it('falls back to the raw error for an unmapped status', async () => {
    installFetch({ 'POST /api/correlation': { status: 418, body: { error: 'teapot' } } })
    const s = useAsdStore()
    const r = await s.correlate(42, 1, 'DLH1')
    expect(r.ok).toBe(false)
    expect(r.message).toBe('teapot')
  })
})
