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
