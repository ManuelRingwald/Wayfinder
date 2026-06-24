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
      'GET /api/admin/whoami': { status: 200, body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'tenant_admin' } },
    })
    const s = useAdminStore()
    const ok = await s.loadIdentity()
    expect(ok).toBe(true)
    expect(s.isAuthorized).toBe(true)
    expect(s.role).toBe('tenant_admin')
    expect(s.isSuperAdmin).toBe(false)
  })

  it('isAdmin gates the rail Admin entry to tenant_admin / super_admin', async () => {
    const cases = [
      { role: 'tenant_admin', status: 200, want: true },
      { role: 'super_admin', status: 200, want: true },
      { role: null, status: 403, want: false }, // operator/non-admin → whoami 403
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
        body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'tenant_admin',
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
      'GET /api/admin/whoami': { status: 200, body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'tenant_admin' } },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.hasFeature('stca')).toBe(false)
  })

  it('marks super_admin so the provisioning panel is shown', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 200, body: { subject: 'root', tenant_id: 1, user_id: 1, role: 'super_admin' } },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.isSuperAdmin).toBe(true)
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
      'GET /api/admin/whoami': { status: 200, body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'tenant_admin' } },
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

describe('admin store — super_admin provisioning', () => {
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

  it('reports a 403 from grant (tenant_admin attempting cross-tenant write)', async () => {
    installFetch({ 'POST /api/admin/tenants/42/subscriptions': { status: 403, body: { error: 'super_admin required' } } })
    const s = useAdminStore()
    const r = await s.grant(42, 9)
    expect(r.ok).toBe(false)
    expect(s.error).toBe('super_admin required')
  })
})
