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

describe('admin store — forced password change (ONB-1)', () => {
  it('exposes mustChangePassword from whoami', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 200, body: { subject: 'admin', tenant_id: 1, user_id: 1, role: 'admin', must_change_password: true } },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.isAuthorized).toBe(true)
    expect(s.mustChangePassword).toBe(true)
  })

  it('mustChangePassword is false when the flag is absent or false', async () => {
    installFetch({
      'GET /api/admin/whoami': { status: 200, body: { subject: 'admin', tenant_id: 1, user_id: 1, role: 'admin', must_change_password: false } },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.mustChangePassword).toBe(false)
  })

  it('changeOwnPassword PUTs current+new password and reloads identity on success', async () => {
    const calls = installFetch({
      'PUT /api/admin/me/password': { status: 204 },
      // after the change the flag is cleared
      'GET /api/admin/whoami': { status: 200, body: { subject: 'admin', tenant_id: 1, user_id: 1, role: 'admin', must_change_password: false } },
    })
    const s = useAdminStore()
    const r = await s.changeOwnPassword('admin', 'newsecret123')
    expect(r.ok).toBe(true)
    const put = calls.find((c) => c.method === 'PUT')
    expect(JSON.parse(put.body)).toEqual({ current_password: 'admin', new_password: 'newsecret123' })
    // identity reloaded → flag flipped off
    expect(s.mustChangePassword).toBe(false)
  })

  it('changeOwnPassword surfaces a 401 (wrong current password)', async () => {
    installFetch({
      'PUT /api/admin/me/password': { status: 401, body: { error: 'current password is incorrect' } },
    })
    const s = useAdminStore()
    const r = await s.changeOwnPassword('wrong', 'newsecret123')
    expect(r.ok).toBe(false)
    expect(r.status).toBe(401)
    expect(s.error).toBeTruthy()
  })
})

describe('admin store — self-management (ONB-2)', () => {
  it('deleteOwnAccount DELETEs /api/admin/me and clears identity on success', async () => {
    const calls = installFetch({
      'DELETE /api/admin/me': { status: 204 },
    })
    const s = useAdminStore()
    // pre-seed identity so there is something to clear
    s.identity = { subject: 'admin', tenant_id: 1, user_id: 1, role: 'admin' }
    const r = await s.deleteOwnAccount()
    expect(r.ok).toBe(true)
    expect(s.identity).toBeNull()
    expect(s.accessStatus).toBe(401)
    expect(s.isAuthorized).toBe(false)
    expect(calls[0].url).toBe('/api/admin/me')
    expect(calls[0].method).toBe('DELETE')
  })

  it('deleteOwnAccount surfaces 409 when last active admin tries to delete', async () => {
    installFetch({
      'DELETE /api/admin/me': { status: 409, body: { error: 'last active admin' } },
    })
    const s = useAdminStore()
    s.identity = { subject: 'admin', tenant_id: 1, user_id: 1, role: 'admin' }
    const r = await s.deleteOwnAccount()
    expect(r.ok).toBe(false)
    expect(r.status).toBe(409)
    expect(s.error).toMatch(/last active admin/)
    // identity remains — the account was NOT deleted
    expect(s.identity).not.toBeNull()
  })

  it('deleteOwnAccount leaves identity intact on generic server error', async () => {
    installFetch({
      'DELETE /api/admin/me': { status: 500, body: { error: 'internal error' } },
    })
    const s = useAdminStore()
    s.identity = { subject: 'admin', tenant_id: 1, user_id: 1, role: 'admin' }
    const r = await s.deleteOwnAccount()
    expect(r.ok).toBe(false)
    expect(s.identity).not.toBeNull()
  })
})

describe('admin store — tenant lifecycle (ONB-4)', () => {
  it('createTenant POSTs the payload and sets a success notice', async () => {
    const calls = installFetch({
      'POST /api/admin/tenants': { status: 201, body: { id: 5, slug: 'acme', name: 'ACME', status: 'active' } },
    })
    const s = useAdminStore()
    const r = await s.createTenant({ slug: 'acme', name: 'ACME' })
    expect(r.ok).toBe(true)
    const post = calls.find((c) => c.method === 'POST')
    expect(post.url).toBe('/api/admin/tenants')
    expect(JSON.parse(post.body)).toEqual({ slug: 'acme', name: 'ACME' })
    expect(s.notice).toMatch(/angelegt/)
  })

  it('createTenant surfaces a 409 duplicate slug', async () => {
    installFetch({ 'POST /api/admin/tenants': { status: 409, body: { error: 'slug already exists' } } })
    const s = useAdminStore()
    const r = await s.createTenant({ slug: 'acme' })
    expect(r.ok).toBe(false)
    expect(s.error).toBe('slug already exists')
  })

  it('createTenant surfaces a 400 invalid slug', async () => {
    installFetch({ 'POST /api/admin/tenants': { status: 400, body: { error: 'invalid slug' } } })
    const s = useAdminStore()
    const r = await s.createTenant({ slug: 'BAD SLUG' })
    expect(r.ok).toBe(false)
    expect(s.error).toMatch(/invalid slug/)
  })

  it('deleteTenant DELETEs the tenant and sets a success notice', async () => {
    const calls = installFetch({ 'DELETE /api/admin/tenants/5': { status: 204 } })
    const s = useAdminStore()
    const r = await s.deleteTenant(5)
    expect(r.ok).toBe(true)
    expect(calls[0].url).toBe('/api/admin/tenants/5')
    expect(calls[0].method).toBe('DELETE')
    expect(s.notice).toMatch(/gelöscht/)
  })

  it('deleteTenant shows a friendly message on the 409 not-empty guard', async () => {
    installFetch({ 'DELETE /api/admin/tenants/5': { status: 409, body: { error: 'tenant still has accounts' } } })
    const s = useAdminStore()
    const r = await s.deleteTenant(5)
    expect(r.ok).toBe(false)
    expect(r.status).toBe(409)
    expect(s.error).toMatch(/noch Zugänge/)
  })
})

describe('admin store — feed lifecycle (ONB-5)', () => {
  it('createFeed POSTs the payload and sets a success notice', async () => {
    const calls = installFetch({
      'POST /api/admin/feeds': {
        status: 201,
        body: { id: 5, name: 'north', multicast_group: '239.255.0.70', port: 8600, sensor_mix: ['PSR'] },
      },
    })
    const s = useAdminStore()
    const r = await s.createFeed({ name: 'north', multicast_group: '239.255.0.70', port: 8600, sensor_mix: ['PSR'] })
    expect(r.ok).toBe(true)
    const post = calls.find((c) => c.method === 'POST')
    expect(post.url).toBe('/api/admin/feeds')
    expect(JSON.parse(post.body)).toEqual({ name: 'north', multicast_group: '239.255.0.70', port: 8600, sensor_mix: ['PSR'] })
    expect(s.notice).toMatch(/angelegt/)
  })

  it('createFeed shows a friendly message on the 409 duplicate name', async () => {
    installFetch({ 'POST /api/admin/feeds': { status: 409, body: { error: 'a feed with this name already exists' } } })
    const s = useAdminStore()
    const r = await s.createFeed({ name: 'north', multicast_group: '239.255.0.70', port: 8600 })
    expect(r.ok).toBe(false)
    expect(r.status).toBe(409)
    expect(s.error).toMatch(/bereits/)
  })

  it('createFeed auto-allocates when group/port are omitted (ORCH-4)', async () => {
    const calls = installFetch({
      'POST /api/admin/feeds': {
        status: 201,
        body: { id: 6, name: 'auto', multicast_group: '239.255.0.1', port: 8600, sensor_mix: [] },
      },
    })
    const s = useAdminStore()
    const r = await s.createFeed({ name: 'auto' })
    expect(r.ok).toBe(true)
    const post = calls.find((c) => c.method === 'POST')
    expect(JSON.parse(post.body)).toEqual({ name: 'auto' }) // no endpoint sent
    expect(s.notice).toMatch(/angelegt/)
  })

  it('createFeed maps a 409 taken endpoint to a distinct message (ORCH-4)', async () => {
    installFetch({ 'POST /api/admin/feeds': { status: 409, body: { error: 'multicast endpoint already in use' } } })
    const s = useAdminStore()
    const r = await s.createFeed({ name: 'x', multicast_group: '239.255.0.1', port: 8600 })
    expect(r.ok).toBe(false)
    expect(s.error).toMatch(/Endpoint/)
  })

  it('createFeed maps a 507 exhausted pool to a friendly message (ORCH-4)', async () => {
    installFetch({ 'POST /api/admin/feeds': { status: 507, body: { error: 'no free multicast endpoint available (pool exhausted)' } } })
    const s = useAdminStore()
    const r = await s.createFeed({ name: 'x' })
    expect(r.ok).toBe(false)
    expect(s.error).toMatch(/Pool erschöpft/)
  })

  it('createFeed surfaces a 400 invalid multicast group', async () => {
    installFetch({ 'POST /api/admin/feeds': { status: 400, body: { error: 'multicast_group must be an IPv4 address' } } })
    const s = useAdminStore()
    const r = await s.createFeed({ name: 'north', multicast_group: 'nope', port: 8600 })
    expect(r.ok).toBe(false)
    expect(s.error).toMatch(/IPv4/)
  })

  it('deleteFeed DELETEs the feed and sets a success notice', async () => {
    const calls = installFetch({ 'DELETE /api/admin/feeds/3': { status: 204 } })
    const s = useAdminStore()
    const r = await s.deleteFeed(3)
    expect(r.ok).toBe(true)
    expect(calls[0].url).toBe('/api/admin/feeds/3')
    expect(calls[0].method).toBe('DELETE')
    expect(s.notice).toMatch(/gelöscht/)
  })
})

describe('admin store — feed source configuration (ORCH-1c)', () => {
  it('loadFeedSources GETs the feed sources', async () => {
    const calls = installFetch({
      'GET /api/admin/feeds/3/sources': {
        status: 200,
        body: { sources: [{ type: 'adsb_opensky', bbox: { min_lat: 48, min_lon: 7, max_lat: 50, max_lon: 9 } }], coverage_bbox: null },
      },
    })
    const s = useAdminStore()
    const r = await s.loadFeedSources(3)
    expect(r.ok).toBe(true)
    expect(r.data.sources).toHaveLength(1)
    expect(calls[0].url).toBe('/api/admin/feeds/3/sources')
    expect(calls[0].method).toBe('GET')
  })

  it('saveFeedSources PUTs the payload and sets a success notice', async () => {
    const calls = installFetch({
      'PUT /api/admin/feeds/3/sources': {
        status: 200,
        body: { sources: [{ type: 'radar_asterix', sac: 1, sic: 4 }], coverage_bbox: null },
      },
    })
    const s = useAdminStore()
    const payload = { sources: [{ type: 'radar_asterix', sac: 1, sic: 4 }] }
    const r = await s.saveFeedSources(3, payload)
    expect(r.ok).toBe(true)
    const put = calls.find((c) => c.method === 'PUT')
    expect(put.url).toBe('/api/admin/feeds/3/sources')
    expect(JSON.parse(put.body)).toEqual(payload)
    expect(s.notice).toMatch(/gespeichert/)
  })

  it('saveFeedSources surfaces a 400 with the offending source index', async () => {
    installFetch({
      'PUT /api/admin/feeds/3/sources': { status: 400, body: { error: 'invalid sources: store: source[0]: adsb_opensky requires a bbox' } },
    })
    const s = useAdminStore()
    const r = await s.saveFeedSources(3, { sources: [{ type: 'adsb_opensky' }] })
    expect(r.ok).toBe(false)
    expect(r.status).toBe(400)
    expect(s.error).toMatch(/source\[0\]/)
  })
})

describe('admin store — per-feed source credentials (ORCH-2c 3a)', () => {
  it('loadFeedSecrets GETs the configured refs', async () => {
    const calls = installFetch({
      'GET /api/admin/feeds/3/secrets': {
        status: 200,
        body: { secrets: [{ ref: 'secret/opensky', configured: true }] },
      },
    })
    const s = useAdminStore()
    const r = await s.loadFeedSecrets(3)
    expect(r.ok).toBe(true)
    expect(r.data.secrets).toHaveLength(1)
    expect(calls[0].method).toBe('GET')
    expect(calls[0].url).toBe('/api/admin/feeds/3/secrets')
  })

  it('loadFeedSecrets surfaces a 503 when the secret store is disabled', async () => {
    installFetch({
      'GET /api/admin/feeds/3/secrets': { status: 503, body: { error: 'secret store not configured' } },
    })
    const s = useAdminStore()
    const r = await s.loadFeedSecrets(3)
    expect(r.ok).toBe(false)
    expect(r.status).toBe(503)
  })

  it('setFeedSecret PUTs the value, encoding a slashed cred_ref', async () => {
    const calls = installFetch({
      'PUT /api/admin/feeds/3/secrets/secret/opensky': { status: 204 },
    })
    const s = useAdminStore()
    const r = await s.setFeedSecret(3, 'secret/opensky', 'sky-token')
    expect(r.ok).toBe(true)
    const put = calls.find((c) => c.method === 'PUT')
    expect(put.url).toBe('/api/admin/feeds/3/secrets/secret/opensky')
    expect(JSON.parse(put.body)).toEqual({ value: 'sky-token' })
    expect(s.notice).toMatch(/gespeichert/)
  })

  it('deleteFeedSecret DELETEs the ref and sets a notice', async () => {
    const calls = installFetch({
      'DELETE /api/admin/feeds/3/secrets/secret/opensky': { status: 204 },
    })
    const s = useAdminStore()
    const r = await s.deleteFeedSecret(3, 'secret/opensky')
    expect(r.ok).toBe(true)
    const del = calls.find((c) => c.method === 'DELETE')
    expect(del.url).toBe('/api/admin/feeds/3/secrets/secret/opensky')
    expect(s.notice).toMatch(/entfernt/)
  })
})

describe('admin store — OpenAIP per tenant (ONB-6)', () => {
  it('loadTenantOpenAIP reports the configured status', async () => {
    installFetch({ 'GET /api/admin/tenants/5/openaip': { status: 200, body: { configured: true } } })
    const s = useAdminStore()
    const r = await s.loadTenantOpenAIP(5)
    expect(r.ok).toBe(true)
    expect(r.data.configured).toBe(true)
  })

  it('setTenantOpenAIPKey PUTs the key and sets a success notice', async () => {
    const calls = installFetch({ 'PUT /api/admin/tenants/5/openaip': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setTenantOpenAIPKey(5, 'my-key')
    expect(r.ok).toBe(true)
    const put = calls.find((c) => c.method === 'PUT')
    expect(put.url).toBe('/api/admin/tenants/5/openaip')
    expect(JSON.parse(put.body)).toEqual({ api_key: 'my-key' })
    expect(s.notice).toMatch(/gespeichert/)
  })

  it('setTenantOpenAIPKey with null clears the key (sends api_key:null)', async () => {
    const calls = installFetch({ 'PUT /api/admin/tenants/5/openaip': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setTenantOpenAIPKey(5, null)
    expect(r.ok).toBe(true)
    expect(JSON.parse(calls[0].body)).toEqual({ api_key: null })
    expect(s.notice).toMatch(/entfernt/)
  })

  it('setTenantOpenAIPKey surfaces a backend error', async () => {
    installFetch({ 'PUT /api/admin/tenants/5/openaip': { status: 400, body: { error: 'api_key too long' } } })
    const s = useAdminStore()
    const r = await s.setTenantOpenAIPKey(5, 'x'.repeat(9999))
    expect(r.ok).toBe(false)
    expect(s.error).toMatch(/too long/)
  })

  it('loadTenantOpenAIP surfaces the AERO-1 cache freshness fields', async () => {
    installFetch({
      'GET /api/admin/tenants/5/openaip': {
        status: 200,
        body: { configured: false, fetched_at: '2026-07-03T10:00:00Z', feature_count: 12 },
      },
    })
    const s = useAdminStore()
    const r = await s.loadTenantOpenAIP(5)
    expect(r.data.fetched_at).toBe('2026-07-03T10:00:00Z')
    expect(r.data.feature_count).toBe(12)
  })

  it('refreshTenantOpenAIP POSTs the per-tenant refresh', async () => {
    const calls = installFetch({ 'POST /api/admin/tenants/5/openaip/refresh': { status: 202 } })
    const s = useAdminStore()
    const r = await s.refreshTenantOpenAIP(5)
    expect(r.ok).toBe(true)
    expect(calls[0].method).toBe('POST')
    expect(calls[0].url).toBe('/api/admin/tenants/5/openaip/refresh')
    expect(s.notice).toMatch(/angestoßen/)
  })
})

describe('admin store — global OpenAIP + fetch-all (AERO-2)', () => {
  it('loadGlobalOpenAIP reports configured + encryption availability', async () => {
    installFetch({
      'GET /api/admin/openaip': { status: 200, body: { configured: true, encryption_available: true } },
    })
    const s = useAdminStore()
    const r = await s.loadGlobalOpenAIP()
    expect(r.data).toEqual({ configured: true, encryption_available: true })
  })

  it('setGlobalOpenAIPKey PUTs the sealed key', async () => {
    const calls = installFetch({ 'PUT /api/admin/openaip': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setGlobalOpenAIPKey('glob-key')
    expect(r.ok).toBe(true)
    expect(calls[0].url).toBe('/api/admin/openaip')
    expect(JSON.parse(calls[0].body)).toEqual({ api_key: 'glob-key' })
    expect(s.notice).toMatch(/gespeichert/)
  })

  it('setGlobalOpenAIPKey surfaces the 503 when encryption is unavailable', async () => {
    installFetch({ 'PUT /api/admin/openaip': { status: 503, body: { error: 'encryption unavailable' } } })
    const s = useAdminStore()
    const r = await s.setGlobalOpenAIPKey('glob-key')
    expect(r.ok).toBe(false)
    expect(s.error).toMatch(/encryption unavailable/)
  })

  it('refreshAllOpenAIP POSTs the fetch-all', async () => {
    const calls = installFetch({ 'POST /api/admin/openaip/refresh': { status: 202 } })
    const s = useAdminStore()
    const r = await s.refreshAllOpenAIP()
    expect(r.ok).toBe(true)
    expect(calls[0].url).toBe('/api/admin/openaip/refresh')
    expect(s.notice).toMatch(/alle Mandanten/)
  })
})

describe('admin store — AIRAC + change-impact (AERO-3)', () => {
  it('loadAirac GETs the AIRAC cycle', async () => {
    installFetch({
      'GET /api/admin/airac': {
        status: 200,
        body: { ident: '2507', effective: '2025-06-26T00:00:00Z', next_ident: '2508', next_effective: '2025-07-24T00:00:00Z', days_until_next: 12 },
      },
    })
    const s = useAdminStore()
    const r = await s.loadAirac()
    expect(r.ok).toBe(true)
    expect(r.data.ident).toBe('2507')
    expect(r.data.days_until_next).toBe(12)
  })

  it('loadTenantOpenAIPChanges GETs the per-layer change-impact', async () => {
    installFetch({
      'GET /api/admin/tenants/5/openaip/changes': {
        status: 200,
        body: [{ kind: 'airspace', feature_count: 143, prev_feature_count: 140, added: 7, removed: 4 }],
      },
    })
    const s = useAdminStore()
    const r = await s.loadTenantOpenAIPChanges(5)
    expect(r.ok).toBe(true)
    expect(r.data[0].added).toBe(7)
    expect(r.data[0].removed).toBe(4)
  })
})

describe('admin store — platform-admin management (ONB-3)', () => {
  it('loadAdmins GETs /api/admin/admins without storing globally', async () => {
    const calls = installFetch({
      'GET /api/admin/admins': {
        status: 200,
        body: [{ id: 1, subject: 'root', status: 'active', must_change_password: false }],
      },
    })
    const s = useAdminStore()
    const r = await s.loadAdmins()
    expect(r.ok).toBe(true)
    expect(r.data[0].subject).toBe('root')
    expect(calls[0].url).toBe('/api/admin/admins')
    expect(calls[0].method).toBe('GET')
  })

  it('createAdmin POSTs the payload and sets a success notice', async () => {
    const calls = installFetch({
      'POST /api/admin/admins': { status: 201, body: { id: 7, subject: 'ops', status: 'active' } },
    })
    const s = useAdminStore()
    const r = await s.createAdmin({ subject: 'ops', password: 'hunter2!!' })
    expect(r.ok).toBe(true)
    const post = calls.find((c) => c.method === 'POST')
    expect(post.url).toBe('/api/admin/admins')
    expect(JSON.parse(post.body)).toEqual({ subject: 'ops', password: 'hunter2!!' })
    expect(s.notice).toMatch(/angelegt/)
  })

  it('createAdmin surfaces a 409 duplicate error', async () => {
    installFetch({ 'POST /api/admin/admins': { status: 409, body: { error: 'subject already exists' } } })
    const s = useAdminStore()
    const r = await s.createAdmin({ subject: 'taken' })
    expect(r.ok).toBe(false)
    expect(s.error).toBe('subject already exists')
  })

  it('setAdminStatus PATCHes the new status', async () => {
    const calls = installFetch({ 'PATCH /api/admin/admins/7': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setAdminStatus(7, 'paused')
    expect(r.ok).toBe(true)
    const patch = calls.find((c) => c.method === 'PATCH')
    expect(patch.url).toBe('/api/admin/admins/7')
    expect(JSON.parse(patch.body)).toEqual({ status: 'paused' })
    expect(s.notice).toMatch(/pausiert/)
  })

  it('setAdminStatus shows a friendly message on the 409 last-admin guard', async () => {
    installFetch({ 'PATCH /api/admin/admins/7': { status: 409, body: { error: 'cannot pause the last active admin' } } })
    const s = useAdminStore()
    const r = await s.setAdminStatus(7, 'paused')
    expect(r.ok).toBe(false)
    expect(r.status).toBe(409)
    expect(s.error).toMatch(/letzte aktive Administrator/)
  })

  it('deleteAdmin DELETEs the admin', async () => {
    const calls = installFetch({ 'DELETE /api/admin/admins/7': { status: 204 } })
    const s = useAdminStore()
    const r = await s.deleteAdmin(7)
    expect(r.ok).toBe(true)
    expect(calls[0].url).toBe('/api/admin/admins/7')
    expect(calls[0].method).toBe('DELETE')
    expect(s.notice).toMatch(/gelöscht/)
  })

  it('deleteAdmin shows a friendly message on the 409 last-admin guard', async () => {
    installFetch({ 'DELETE /api/admin/admins/7': { status: 409, body: { error: 'cannot delete the last active admin' } } })
    const s = useAdminStore()
    const r = await s.deleteAdmin(7)
    expect(r.ok).toBe(false)
    expect(s.error).toMatch(/letzte aktive Administrator/)
  })

  it('setAdminPassword PUTs the new password', async () => {
    const calls = installFetch({ 'PUT /api/admin/admins/7/password': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setAdminPassword(7, 'newsecret1')
    expect(r.ok).toBe(true)
    const put = calls.find((c) => c.method === 'PUT')
    expect(put.url).toBe('/api/admin/admins/7/password')
    expect(JSON.parse(put.body)).toEqual({ password: 'newsecret1' })
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

  it('logout POSTs to /api/logout and resets to the unauthenticated state', async () => {
    const calls = installFetch({
      'GET /api/admin/whoami': { status: 200, body: { subject: 'alice', tenant_id: 7, user_id: 1, role: 'admin' } },
      'POST /api/logout': { status: 204 },
    })
    const s = useAdminStore()
    await s.loadIdentity()
    expect(s.isAuthorized).toBe(true)
    await s.logout()
    expect(s.identity).toBeNull()
    expect(s.isAuthorized).toBe(false)
    expect(s.accessStatus).toBe(401)
    expect(calls.some((c) => c.url === '/api/logout' && c.method === 'POST')).toBe(true)
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

  it('setUserSessionLimit PUTs a numeric limit (AP7)', async () => {
    const calls = installFetch({ 'PUT /api/admin/tenants/42/users/7/session-limit': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setUserSessionLimit(42, 7, 3)
    expect(r.ok).toBe(true)
    const put = calls.find((c) => c.method === 'PUT')
    expect(put.url).toBe('/api/admin/tenants/42/users/7/session-limit')
    expect(JSON.parse(put.body)).toEqual({ limit: 3 })
    expect(s.notice).toMatch(/Sitzungslimit gesetzt/)
  })

  it('setUserSessionLimit PUTs null to reset to the default (AP7)', async () => {
    const calls = installFetch({ 'PUT /api/admin/tenants/42/users/7/session-limit': { status: 204 } })
    const s = useAdminStore()
    const r = await s.setUserSessionLimit(42, 7, null)
    expect(r.ok).toBe(true)
    const put = calls.find((c) => c.method === 'PUT')
    expect(JSON.parse(put.body)).toEqual({ limit: null })
    expect(s.notice).toMatch(/Standard/)
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
