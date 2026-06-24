import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useImpersonationStore } from '@/stores/impersonation.js'

// installFetch stubs global fetch with a table keyed by "METHOD path"; each entry
// is { status, body }. The impersonation store reads JSON via res.json().
function installFetch(table) {
  const calls = []
  globalThis.fetch = vi.fn(async (url, opts = {}) => {
    const method = (opts.method || 'GET').toUpperCase()
    calls.push({ url, method, body: opts.body })
    const entry = table[`${method} ${url}`]
    if (!entry) return { ok: false, status: 404, json: async () => ({}) }
    return {
      ok: entry.status >= 200 && entry.status < 300,
      status: entry.status,
      json: async () => entry.body ?? {},
    }
  })
  return calls
}

beforeEach(() => setActivePinia(createPinia()))

describe('impersonation store (ADR 0008)', () => {
  it('loadStatus mirrors an active grant WITHOUT triggering a reconnect', async () => {
    installFetch({ 'GET /api/admin/impersonation': { status: 200, body: { active: true, tenant_id: 9 } } })
    const s = useImpersonationStore()
    await s.loadStatus()
    expect(s.active).toBe(true)
    expect(s.tenantId).toBe(9)
    expect(s.reconnectNonce).toBe(0) // a freshly loaded page already opens /ws with the cookie
  })

  it('loadStatus reports inactive when there is no grant', async () => {
    installFetch({ 'GET /api/admin/impersonation': { status: 200, body: { active: false } } })
    const s = useImpersonationStore()
    await s.loadStatus()
    expect(s.active).toBe(false)
    expect(s.tenantId).toBeNull()
  })

  it('start posts tenant_id, activates, and bumps reconnectNonce', async () => {
    const calls = installFetch({ 'POST /api/admin/impersonation': { status: 204 } })
    const s = useImpersonationStore()
    const ok = await s.start(42)
    expect(ok).toBe(true)
    expect(s.active).toBe(true)
    expect(s.tenantId).toBe(42)
    expect(s.reconnectNonce).toBe(1)
    const post = calls.find((c) => c.method === 'POST')
    expect(JSON.parse(post.body)).toEqual({ tenant_id: 42 })
  })

  it('start failure keeps the user OUT of impersonation and sets an error', async () => {
    installFetch({ 'POST /api/admin/impersonation': { status: 403 } })
    const s = useImpersonationStore()
    const ok = await s.start(42)
    expect(ok).toBe(false)
    expect(s.active).toBe(false)
    expect(s.reconnectNonce).toBe(0)
    expect(s.error).toMatch(/42/)
  })

  it('stop clears state and bumps reconnectNonce', async () => {
    installFetch({ 'DELETE /api/admin/impersonation': { status: 204 } })
    const s = useImpersonationStore()
    s.active = true
    s.tenantId = 9
    await s.stop()
    expect(s.active).toBe(false)
    expect(s.tenantId).toBeNull()
    expect(s.reconnectNonce).toBe(1)
  })

  it('stop never traps the operator: clears local state even on a server error', async () => {
    installFetch({ 'DELETE /api/admin/impersonation': { status: 500 } })
    const s = useImpersonationStore()
    s.active = true
    s.tenantId = 9
    const ok = await s.stop()
    expect(ok).toBe(false)
    expect(s.active).toBe(false)
    expect(s.reconnectNonce).toBe(1)
  })
})
