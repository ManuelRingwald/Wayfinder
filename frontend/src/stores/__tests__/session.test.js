import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSessionStore } from '@/stores/session.js'

// installFetch stubs global fetch with a table keyed by "METHOD path" and records
// every call (same harness as the admin store test).
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

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('session store — ASD map auth gate', () => {
  it('probe resolves to authed and exposes the identity on 200', async () => {
    installFetch({
      'GET /api/whoami': { status: 200, body: { subject: 'edlv-lotse', tenant_id: 2, role: 'user' } },
    })
    const s = useSessionStore()
    const st = await s.probe()
    expect(st).toBe('authed')
    expect(s.status).toBe('authed')
    expect(s.subject).toBe('edlv-lotse')
    expect(s.role).toBe('user')
    expect(s.isAdmin).toBe(false)
  })

  it('exposes tenant features and sensor classes from whoami (Issues #106/#107)', async () => {
    installFetch({
      'GET /api/whoami': {
        status: 200,
        body: {
          subject: 'lotse', tenant_id: 2, role: 'user',
          features: { airspaces: true, range_rings: false },
          sensor_classes: ['ADS-B', 'FLARM'],
        },
      },
    })
    const s = useSessionStore()
    await s.probe()
    expect(s.hasFeature('airspaces')).toBe(true)
    expect(s.hasFeature('range_rings')).toBe(false)
    expect(s.hasFeature('waypoints')).toBe(false) // absent ⇒ denied
    expect(s.sensorClasses).toEqual(['ADS-B', 'FLARM'])
  })

  it('defaults features/sensorClasses to empty when whoami omits them', async () => {
    installFetch({
      'GET /api/whoami': { status: 200, body: { subject: 'lotse', tenant_id: 2, role: 'user' } },
    })
    const s = useSessionStore()
    await s.probe()
    expect(s.hasFeature('airspaces')).toBe(false)
    expect(s.sensorClasses).toEqual([])
  })

  it('exposes the effective AoR airspace ids from whoami (ASD-014)', async () => {
    installFetch({
      'GET /api/whoami': {
        status: 200,
        body: { subject: 'lotse', tenant_id: 2, role: 'user', aor_airspace_ids: ['62a1', '62b2'] },
      },
    })
    const s = useSessionStore()
    await s.probe()
    expect(s.aorAirspaceIds).toEqual(['62a1', '62b2'])
  })

  it('defaults aorAirspaceIds to [] when whoami omits it (no AoR ⇒ nothing highlighted)', async () => {
    installFetch({
      'GET /api/whoami': { status: 200, body: { subject: 'lotse', tenant_id: 2, role: 'user' } },
    })
    const s = useSessionStore()
    await s.probe()
    expect(s.aorAirspaceIds).toEqual([])
  })

  it('exposes the tenant view centre from whoami so the map opens on its sector (FR-UI-013)', async () => {
    installFetch({
      'GET /api/whoami': {
        status: 200,
        body: { subject: 'lotse', tenant_id: 2, role: 'user', center_lat: 53.63, center_lon: 9.988, zoom: 8 },
      },
    })
    const s = useSessionStore()
    await s.probe()
    expect(s.viewCenter).toEqual({ lat: 53.63, lon: 9.988, zoom: 8 })
  })

  it('viewCenter is null when whoami omits the centre ⇒ map keeps the env default', async () => {
    installFetch({
      'GET /api/whoami': { status: 200, body: { subject: 'lotse', tenant_id: 2, role: 'user' } },
    })
    const s = useSessionStore()
    await s.probe()
    expect(s.viewCenter).toBeNull()
  })

  it('viewCenter honours the equator (center_lat === 0 is valid, not "unset")', async () => {
    installFetch({
      'GET /api/whoami': {
        status: 200,
        body: { subject: 'lotse', tenant_id: 2, role: 'user', center_lat: 0, center_lon: 0, zoom: 4 },
      },
    })
    const s = useSessionStore()
    await s.probe()
    expect(s.viewCenter).toEqual({ lat: 0, lon: 0, zoom: 4 })
  })

  it('probe falls closed to anon on 401 (no identity ⇒ login screen, never a map)', async () => {
    installFetch({ 'GET /api/whoami': { status: 401, body: { error: 'unauthorized' } } })
    const s = useSessionStore()
    const st = await s.probe()
    expect(st).toBe('anon')
    expect(s.identity).toBeNull()
  })

  it('login posts credentials, then re-probes and flips to authed', async () => {
    const calls = installFetch({
      'POST /api/login': { status: 204 },
      'GET /api/whoami': { status: 200, body: { subject: 'edlv-lotse', tenant_id: 2, role: 'user' } },
    })
    const s = useSessionStore()
    const ok = await s.login('edlv-lotse', 'Weeze-30nm!')
    expect(ok).toBe(true)
    expect(s.status).toBe('authed')
    const loginCall = calls.find((c) => c.url === '/api/login')
    expect(loginCall.method).toBe('POST')
    expect(JSON.parse(loginCall.body)).toEqual({ subject: 'edlv-lotse', password: 'Weeze-30nm!' })
  })

  it('login failure surfaces the error and stays anon (no whoami follow-up)', async () => {
    const calls = installFetch({ 'POST /api/login': { status: 401, body: { error: 'unauthorized' } } })
    const s = useSessionStore()
    const ok = await s.login('edlv-lotse', 'wrong')
    expect(ok).toBe(false)
    expect(s.status).not.toBe('authed')
    expect(s.error).toBeTruthy()
    expect(calls.some((c) => c.url === '/api/whoami')).toBe(false)
  })

  it('logout calls /api/logout and returns to anon', async () => {
    const calls = installFetch({ 'POST /api/logout': { status: 204 } })
    const s = useSessionStore()
    await s.logout()
    expect(s.status).toBe('anon')
    expect(s.identity).toBeNull()
    expect(calls.some((c) => c.url === '/api/logout' && c.method === 'POST')).toBe(true)
  })
})

describe('session store — sliding refresh (WF2-12.5)', () => {
  it('renewNow POSTs to /api/session/renew', async () => {
    const calls = installFetch({ 'POST /api/session/renew': { status: 204 } })
    const s = useSessionStore()
    const ok = await s.renewNow()
    expect(ok).toBe(true)
    expect(calls.some((c) => c.url === '/api/session/renew' && c.method === 'POST')).toBe(true)
  })

  it('renewNow on 401 re-probes and flips to anon + expired', async () => {
    installFetch({ 'GET /api/whoami': { status: 200, body: { subject: 'x', tenant_id: 1, role: 'user' } } })
    const s = useSessionStore()
    await s.probe()
    expect(s.status).toBe('authed')

    installFetch({ 'POST /api/session/renew': { status: 401 }, 'GET /api/whoami': { status: 401 } })
    const ok = await s.renewNow()
    expect(ok).toBe(false)
    expect(s.status).toBe('anon')
    expect(s.expired).toBe(true)
  })

  it('probe marks expired only on an authed→anon transition', async () => {
    installFetch({ 'GET /api/whoami': { status: 401 } })
    const s = useSessionStore()
    await s.probe() // never authed
    expect(s.status).toBe('anon')
    expect(s.expired).toBe(false)

    installFetch({ 'GET /api/whoami': { status: 200, body: { subject: 'x', tenant_id: 1, role: 'user' } } })
    await s.probe() // now authed
    expect(s.expired).toBe(false)

    installFetch({ 'GET /api/whoami': { status: 401 } })
    await s.probe() // dropped → expired
    expect(s.status).toBe('anon')
    expect(s.expired).toBe(true)
  })

  it('startRenew fires renewNow on the interval; stopRenew halts it', async () => {
    vi.useFakeTimers()
    try {
      const calls = installFetch({ 'POST /api/session/renew': { status: 204 } })
      const s = useSessionStore()
      s.startRenew(1000)
      await vi.advanceTimersByTimeAsync(2500)
      const fired = calls.filter((c) => c.url === '/api/session/renew').length
      expect(fired).toBeGreaterThanOrEqual(2)

      s.stopRenew()
      await vi.advanceTimersByTimeAsync(3000)
      expect(calls.filter((c) => c.url === '/api/session/renew').length).toBe(fired)
    } finally {
      vi.useRealTimers()
    }
  })
})
