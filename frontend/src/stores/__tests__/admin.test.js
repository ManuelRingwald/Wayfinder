import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAdminStore } from '@/stores/admin.js'

// installFetch stubs global fetch with a table keyed by "METHOD path". Each entry
// is { status, body }. It records every call so tests can assert URL/method/body.
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

describe('admin store — identity & role gating', () => {
  it('loadIdentity stores the identity and exposes the role', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 200, body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'admin' } },
    })
    const s = useAdminStore()
    const ok = await s.loadIdentity()
    expect(ok).toBe(true)
    expect(s.isAuthorized).toBe(true)
    expect(s.role).toBe('admin')
    expect(s.isAdmin).toBe(true)
  })

  it('isAdmin gates the rail Admin entry to the admin role', async () => {
    const cases = [
      { role: 'admin', status: 200, want: true },
      { role: null, status: 403, want: false }, // user/non-admin → whoami 403
    ]
    for (const c of cases) {
      setActivePinia(createPinia())
      installFetch({
        'GET /api/admin/whoami':
          c.status === 200
            ? { status: 200, body: { subject: 'x', tenant_id: 1, user_id: 1, role: c.role } }
            : { status: c.status, body: { error: 'forbidden' } },
      })
      const s = useAdminStore()
      await s.loadIdentity()
      expect(s.isAdmin, `role=${c.role} status=${c.status}`).toBe(c.want)
    }
  })

  it('exposes effective feature flags from whoami (WF2-50)', async () => {
    installFetch({
      'GET /api/admin/whoami': {
        status: 200,
        body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'admin',
          features: { stca: true, multi_feed: false, premium_layers: true } },
      },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.hasFeature('stca')).toBe(true)
    expect(s.hasFeature('multi_feed')).toBe(false)
    expect(s.hasFeature('premium_layers')).toBe(true)
    expect(s.hasFeature('nonexistent')).toBe(false)
  })

  it('hasFeature is false when whoami omits features (fail-safe default)', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 200, body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'admin' } },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.hasFeature('stca')).toBe(false)
  })

  it('marks admin so the provisioning panel is shown', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 200, body: { subject: 'root', tenant_id: 1, user_id: 1, role: 'admin' } },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.isAdmin).toBe(true)
  })

  it('records an access error and stays unauthorized on 403', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 403, body: { error: 'forbidden' } },
    })
    const s = useAdminStore()
    const ok = await s.loadIdentity()
    expect(ok).toBe(false)
    expect(s.isAuthorized).toBe(false)
    expect(s.accessError).toBe('forbidden')
    expect(s.accessStatus).toBe(403)
  })

  it('sets accessStatus to 401 when not logged in', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 401, body: { error: 'unauthorized' } },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.accessStatus).toBe(401)
    expect(s.isAuthorized).toBe(false)
  })

  it('clears accessStatus and accessError after successful login probe', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 200, body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'admin' } },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.accessStatus).toBeNull()
    expect(s.accessError).toBeNull()
    expect(s.isAuthorized).toBe(true)
  })
})

describe('admin store — login', () => {
  it('login POSTs subject and password to /api/login', async () => {
    const calls = installFetch({ 'POST /api/login': { status: 204 } })
    const s = useAdminStore()
    const r = await s.login('alice', 's3cr3t')
    expect(r.ok).toBe(true)
    expect(r.status).toBe(204)
    const post = calls.find((c) => c.method === 'POST')
    expect(post.url).toBe('/api/login')
    expect(JSON.parse(post.body)).toEqual({ subject: 'alice', password: 's3cr3t' })
  })

  it('login returns ok:false with 401 on wrong credentials', async () => {
    installFetch({ 'POST /api/login': { status: 401, body: { error: 'invalid credentials' } } })
    const s = useAdminStore()
    const r = await s.login('alice', 'wrong')
    expect(r.ok).toBe(false)
    expect(r.status).toBe(401)
  })
})

describe('admin store — view config', () => {
  it('saveView PUTs the exact DTO and updates state on success', async () => {
    const echo = { center_lat: 50, center_lon: 9, zoom: 8 }
    const calls = installFetch({ 'PUT /api/admin/view': { status: 200, body: echo } })
    const s = useAdminStore()
    const r = await s.saveView(echo)
    expect(r.ok).toBe(true)
    expect(s.view).toEqual(echo)
    expect(s.notice).toMatch(/gespeichert/)
    const put = calls.find((c) => c.method === 'PUT')
    expect(put.url).toBe('/api/admin/view')
    expect(JSON.parse(put.body)).toEqual(echo)
  })

  it('surfaces the server error message on a 400', async () => {
    installFetch({ 'PUT /api/admin/view': { status: 400, body: { error: 'zoom out of range [0,24]' } } })
    const s = useAdminStore()
    const r = await s.saveView({ center_lat: 0, center_lon: 0, zoom: 99 })
    expect(r.ok).toBe(false)
    expect(s.error).toMatch(/zoom out of range/)
    expect(s.view).toBeNull()
  })

  it('treats a 404 view as "no config yet" (null, no error banner)', async () => {
    installFetch({ 'GET /api/admin/view': { status: 404, body: { error: 'no view configured' } } })
    const s = useAdminStore()
    await s.loadView()
    expect(s.view).toBeNull()
    expect(s.error).toBeNull()
  })
})

describe('admin store — provisioning', () => {
  it('grant POSTs feed_id to the tenant subscriptions endpoint', async () => {
    const calls = installFetch({ 'POST /api/admin/tenants/42/subscriptions': { status: 204 } })
    const s = useAdminStore()
    const r = await s.grant(42, 9)
    expect(r.ok).toBe(true)
    const post = calls.find((c) => c.method === 'POST')
    expect(post.url).toBe('/api/admin/tenants/42/subscriptions')
    expect(JSON.parse(post.body)).toEqual({ feed_id: 9 })
    expect(s.notice).toMatch(/zugewiesen/)
  })

  it('revoke DELETEs the feed under the tenant', async () => {
    const calls = installFetch({ 'DELETE /api/admin/tenants/42/subscriptions/9': { status: 204 } })
    const s = useAdminStore()
    const r = await s.revoke(42, 9)
    expect(r.ok).toBe(true)
    const del = calls.find((c) => c.method === 'DELETE')
    expect(del.url).toBe('/api/admin/tenants/42/subscriptions/9')
  })

  it('reports a 403 from grant (user attempting cross-tenant write)', async () => {
    installFetch({ 'POST /api/admin/tenants/42/subscriptions': { status: 403, body: { error: 'admin required' } } })
    const s = useAdminStore()
    const r = await s.grant(42, 9)
    expect(r.ok).toBe(false)
    expect(s.error).toBe('admin required')
  })
})

describe('admin store — access management (AP6)', () => {
  it('loadTenantUsers GETs the tenant users endpoint without storing globally', async () => {
    const calls = installFetch({
      'GET /api/admin/tenants/42/users': {
        status: 200,
        body: [{ id: 1, subject: 'alice', role: 'user', status: 'active' }],
      },
    })
    const s = useAdminStore()
    const r = await s.loadTenantUsers(42)
    expect(r.ok).toBe(true)
    expect(r.data[0].subject).toBe('alice')
    expect(calls[0].url).toBe('/api/admin/tenants/42/users')
  })

  it('createUser POSTs the payload and sets a success notice', async () => {
    const calls = installFetch({
      'POST /api/admin/tenants/42/users': { status: 201, body: { id: 7, subject: 'bob', role: 'user', status: 'active' } },
    })
    const s = useAdminStore()
    const r = await s.createUser(42, { subject: 'bob', password: 'hunter2!!' })
    expect(r.ok).toBe(true)
    const post = calls.find((c) => c.method === 'POST')
    expect(post.url).toBe('/api/admin/tenants/42/users')
    expect(JSON.parse(post.body)).toEqual({ subject: 'bob', password: 'hunter2!!' })
    expect(s.notice).toMatch(/angelegt/)
  })

  it('createUser surfaces a 409 duplicate error', async () => {
    installFetch({ 'POST /api/admin/tenants/42/users': { status: 409, body: { error: 'subject already exists' } } })
    const s = useAdminStore()
    const r = await s.createUser(42, { subject: 'taken' })
    expect(r.ok).toBe(false)
    expect(s.error).toBe('subject already exists')
  })

  it('setUserStatus PATCHes the user with the new status', async () => {
    const calls = installFetch({ 'PATCH /api/admin/tenants/42/users/7': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setUserStatus(42, 7, 'paused')
    expect(r.ok).toBe(true)
    const patch = calls.find((c) => c.method === 'PATCH')
    expect(patch.url).toBe('/api/admin/tenants/42/users/7')
    expect(JSON.parse(patch.body)).toEqual({ status: 'paused' })
    expect(s.notice).toMatch(/pausiert/)
  })

  it('deleteUser DELETEs the user', async () => {
    const calls = installFetch({ 'DELETE /api/admin/tenants/42/users/7': { status: 204 } })
    const s = useAdminStore()
    const r = await s.deleteUser(42, 7)
    expect(r.ok).toBe(true)
    expect(calls[0].url).toBe('/api/admin/tenants/42/users/7')
    expect(calls[0].method).toBe('DELETE')
  })

  it('setUserPassword PUTs the new password', async () => {
    const calls = installFetch({ 'PUT /api/admin/tenants/42/users/7/password': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setUserPassword(42, 7, 'newsecret1')
    expect(r.ok).toBe(true)
    const put = calls.find((c) => c.method === 'PUT')
    expect(put.url).toBe('/api/admin/tenants/42/users/7/password')
    expect(JSON.parse(put.body)).toEqual({ password: 'newsecret1' })
  })

  it('setTenantStatus PATCHes the tenant and notes the mode', async () => {
    const calls = installFetch({ 'PATCH /api/admin/tenants/42': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setTenantStatus(42, 'paused')
    expect(r.ok).toBe(true)
    const patch = calls.find((c) => c.method === 'PATCH')
    expect(patch.url).toBe('/api/admin/tenants/42')
    expect(JSON.parse(patch.body)).toEqual({ status: 'paused' })
    expect(s.notice).toMatch(/Mandant pausiert/)
  })
})

describe('admin store — feature catalog (AP2)', () => {
  it('exposes airspace overlay keys from whoami features', async () => {
    installFetch({
      'GET /api/admin/whoami': {
        status: 200,
        body: {
          subject: 'alice', tenant_id: 7, user_id: 1, role: 'admin',
          features: { airspaces: true, vor_ndb: false, waypoints: true },
        },
      },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.hasFeature('airspaces')).toBe(true)
    expect(s.hasFeature('vor_ndb')).toBe(false)
    expect(s.hasFeature('waypoints')).toBe(true)
  })

  it('exposes display-layer keys (range_rings, history_dots) from whoami features', async () => {
    installFetch({
      'GET /api/admin/whoami': {
        status: 200,
        body: {
          subject: 'alice', tenant_id: 7, user_id: 1, role: 'admin',
          features: { range_rings: true, history_dots: false },
        },
      },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.hasFeature('range_rings')).toBe(true)
    expect(s.hasFeature('history_dots')).toBe(false)
  })

  it('all AP2 keys default to false when whoami omits them (fail-closed)', async () => {
    installFetch({
      'GET /api/admin/whoami': {
        status: 200,
        body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'admin', features: {} },
      },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    for (const key of ['airspaces', 'range_rings', 'history_dots', 'vor_ndb', 'waypoints']) {
      expect(s.hasFeature(key), `key=${key}`).toBe(false)
    }
  })

  it('isAuthorized is false on 403 — non-admin users see all layer controls (cosmetic gate)', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 403, body: { error: 'forbidden' } },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.isAuthorized).toBe(false)
    // When isAuthorized is false the UI formula (!isAuthorized || hasFeature(k))
    // evaluates to true for every key — all layer controls are shown.
    for (const key of ['airspaces', 'range_rings', 'history_dots', 'vor_ndb', 'waypoints']) {
      expect(!s.isAuthorized || s.hasFeature(key), `key=${key}`).toBe(true)
    }
  })
})

describe('admin store — tenant dashboard (AP3)', () => {
  it('loadOverview stores the aggregated rows', async () => {
    const rows = [
      { id: 5, slug: 'acme', name: 'ACME', status: 'active', features: ['stca'], feeds: [{ id: 3, name: 'FRA' }], user_count: 2 },
    ]
    const calls = installFetch({ 'GET /api/admin/overview': { status: 200, body: rows } })
    const s = useAdminStore()
    const r = await s.loadOverview()
    expect(r.ok).toBe(true)
    expect(s.overview).toEqual(rows)
    expect(calls[0].url).toBe('/api/admin/overview')
  })

  it('loadOverview surfaces the error and leaves overview untouched on failure', async () => {
    installFetch({ 'GET /api/admin/overview': { status: 500, body: { error: 'boom' } } })
    const s = useAdminStore()
    await s.loadOverview()
    expect(s.error).toMatch(/boom/)
    expect(s.overview).toEqual([])
  })

  it('loadTenantView GETs the per-tenant view without storing globally', async () => {
    const view = { center_lat: 50, center_lon: 9, zoom: 8 }
    const calls = installFetch({ 'GET /api/admin/tenants/5/view': { status: 200, body: view } })
    const s = useAdminStore()
    const r = await s.loadTenantView(5)
    expect(r.ok).toBe(true)
    expect(r.data).toEqual(view)
    expect(s.view).toBeNull() // not stored globally — caller owns the transient view
    expect(calls[0].url).toBe('/api/admin/tenants/5/view')
  })

  it('loadTenantView reports a 404 (no view yet) to the caller', async () => {
    installFetch({ 'GET /api/admin/tenants/5/view': { status: 404, body: { error: 'no view configured' } } })
    const s = useAdminStore()
    const r = await s.loadTenantView(5)
    expect(r.ok).toBe(false)
    expect(r.status).toBe(404)
  })

  it('saveTenantView PUTs the DTO to the per-tenant route', async () => {
    const dto = { center_lat: 48, center_lon: 11, zoom: 7, aoi: { min_lat: 47, min_lon: 10, max_lat: 49, max_lon: 12 } }
    const calls = installFetch({ 'PUT /api/admin/tenants/5/view': { status: 200, body: dto } })
    const s = useAdminStore()
    const r = await s.saveTenantView(5, dto)
    expect(r.ok).toBe(true)
    expect(s.notice).toMatch(/gespeichert/)
    const put = calls.find((c) => c.method === 'PUT')
    expect(put.url).toBe('/api/admin/tenants/5/view')
    expect(JSON.parse(put.body)).toEqual(dto)
  })

  it('saveTenantView surfaces a server validation error', async () => {
    installFetch({ 'PUT /api/admin/tenants/5/view': { status: 400, body: { error: 'zoom out of range [0,24]' } } })
    const s = useAdminStore()
    const r = await s.saveTenantView(5, { center_lat: 0, center_lon: 0, zoom: 99 })
    expect(r.ok).toBe(false)
    expect(s.error).toMatch(/zoom out of range/)
  })

  it('loadTenantEntitlements GETs the catalogue for a tenant', async () => {
    const ents = [{ key: 'stca', enabled: true, description: 'x' }, { key: 'airspaces', enabled: false, description: 'y' }]
    const calls = installFetch({ 'GET /api/admin/tenants/5/entitlements': { status: 200, body: ents } })
    const s = useAdminStore()
    const r = await s.loadTenantEntitlements(5)
    expect(r.ok).toBe(true)
    expect(r.data).toEqual(ents)
    expect(calls[0].url).toBe('/api/admin/tenants/5/entitlements')
  })

  it('setTenantEntitlement PUTs the flag and notes the change', async () => {
    const calls = installFetch({ 'PUT /api/admin/tenants/5/entitlements/stca': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setTenantEntitlement(5, 'stca', true)
    expect(r.ok).toBe(true)
    expect(s.notice).toMatch(/aktiviert/)
    const put = calls.find((c) => c.method === 'PUT')
    expect(put.url).toBe('/api/admin/tenants/5/entitlements/stca')
    expect(JSON.parse(put.body)).toEqual({ enabled: true })
  })
})

// --- AP4: feed health ---------------------------------------------------------

describe('admin store — feed health (AP4)', () => {
  it('loadFeedsHealth stores health by feed_id key', async () => {
    // Feed 2 is green with 0 tracks (empty sky) — no longer yellow.
    const payload = [
      { feed_id: 1, color: 'green', stale: false, ever_seen: true, last_heartbeat_ago_s: 0.5, track_count_recent: 3, sensors_active: 0, sensors_total: 0 },
      { feed_id: 2, color: 'green', stale: false, ever_seen: true, last_heartbeat_ago_s: 1.2, track_count_recent: 0, sensors_active: 0, sensors_total: 0 },
    ]
    installFetch({ 'GET /api/admin/feeds/health': { status: 200, body: payload } })
    const s = useAdminStore()
    const r = await s.loadFeedsHealth()
    expect(r.ok).toBe(true)
    expect(s.feedsHealth[1].color).toBe('green')
    expect(s.feedsHealth[2].color).toBe('green')
    expect(s.feedsHealth[1].track_count_recent).toBe(3)
  })

  it('loadFeedsHealth with degraded sensors stores yellow', async () => {
    // Yellow = sensor fusion degraded (CAT063 data, Firefly issue #32).
    const payload = [{ feed_id: 5, color: 'yellow', stale: false, ever_seen: true, last_heartbeat_ago_s: 0.8, track_count_recent: 2, sensors_active: 2, sensors_total: 3 }]
    installFetch({ 'GET /api/admin/feeds/health': { status: 200, body: payload } })
    const s = useAdminStore()
    await s.loadFeedsHealth()
    expect(s.feedsHealth[5].color).toBe('yellow')
    expect(s.feedsHealth[5].sensors_active).toBe(2)
    expect(s.feedsHealth[5].sensors_total).toBe(3)
  })

  it('loadFeedsHealth with stale feed stores red', async () => {
    const payload = [{ feed_id: 7, color: 'red', stale: true, ever_seen: true, last_heartbeat_ago_s: 10, track_count_recent: 0 }]
    installFetch({ 'GET /api/admin/feeds/health': { status: 200, body: payload } })
    const s = useAdminStore()
    await s.loadFeedsHealth()
    expect(s.feedsHealth[7].color).toBe('red')
    expect(s.feedsHealth[7].stale).toBe(true)
  })

  it('loadFeedsHealth with empty list clears map', async () => {
    installFetch({ 'GET /api/admin/feeds/health': { status: 200, body: [] } })
    const s = useAdminStore()
    // pre-seed stale data
    s.feedsHealth[1] = { color: 'green' }
    await s.loadFeedsHealth()
    expect(Object.keys(s.feedsHealth).length).toBe(0)
  })

  it('loadFeedsHealth on error does not clear existing data', async () => {
    installFetch({ 'GET /api/admin/feeds/health': { status: 503 } })
    const s = useAdminStore()
    s.feedsHealth[1] = { color: 'green' }
    const r = await s.loadFeedsHealth()
    expect(r.ok).toBe(false)
    // existing data untouched on error
    expect(s.feedsHealth[1]).toBeDefined()
  })
})
