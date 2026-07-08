import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useProfilesStore, MAX_PROFILES } from '@/stores/profiles.js'
import { useAsdStore } from '@/stores/asd.js'

// installFetch stubs global fetch with a table keyed by "METHOD path"; each entry
// is { status, body }. Records calls so tests can assert URL/method/body.
function installFetch(table) {
  const calls = []
  globalThis.fetch = vi.fn(async (url, opts = {}) => {
    const method = (opts.method || 'GET').toUpperCase()
    calls.push({ url, method, body: opts.body })
    const entry = table[`${method} ${url}`]
    if (!entry) return { ok: false, status: 404, text: async () => JSON.stringify({ error: 'not found' }) }
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

const P = (over = {}) => ({ id: 1, name: 'Approach', settings: { v: 1, layers: {} }, is_default: false, updated_at: '', ...over })

describe('profiles store', () => {
  it('load GETs the profiles and fills the list', async () => {
    installFetch({ 'GET /api/view-profiles': { status: 200, body: [P(), P({ id: 2, name: 'Overview' })] } })
    const s = useProfilesStore()
    expect(await s.load()).toBe(true)
    expect(s.list.map((p) => p.name)).toEqual(['Approach', 'Overview'])
  })

  it('load surfaces the error and leaves the list on failure', async () => {
    installFetch({ 'GET /api/view-profiles': { status: 500, body: { error: 'boom' } } })
    const s = useProfilesStore()
    expect(await s.load()).toBe(false)
    expect(s.error).toBe('boom')
  })

  it('saveCurrent POSTs the captured settings and appends the created profile', async () => {
    const calls = installFetch({
      'POST /api/view-profiles': { status: 201, body: P({ id: 9, name: 'Tower', is_default: true }) },
    })
    const s = useProfilesStore()
    expect(await s.saveCurrent('Tower', true)).toBe(true)
    expect(s.list).toHaveLength(1)
    expect(s.list[0]).toMatchObject({ id: 9, name: 'Tower', is_default: true })
    // The POST body carries name, make_default and a captured settings object.
    const body = JSON.parse(calls[0].body)
    expect(body.name).toBe('Tower')
    expect(body.make_default).toBe(true)
    expect(body.settings).toHaveProperty('layers')
    expect(body.settings).toHaveProperty('rangeRings')
  })

  it('setDefault marks exactly one profile default locally', async () => {
    installFetch({
      'GET /api/view-profiles': { status: 200, body: [P({ id: 1, is_default: true }), P({ id: 2 }), P({ id: 3 })] },
      'POST /api/view-profiles/2/default': { status: 200, body: P({ id: 2, is_default: true }) },
    })
    const s = useProfilesStore()
    await s.load()
    expect(await s.setDefault(2)).toBe(true)
    expect(s.list.filter((p) => p.is_default).map((p) => p.id)).toEqual([2])
    expect(s.defaultProfile.id).toBe(2)
  })

  it('rename re-sends the stored settings via PUT', async () => {
    const calls = installFetch({
      'GET /api/view-profiles': { status: 200, body: [P({ id: 5, name: 'Old', settings: { v: 1, layers: { rangeRings: true } } })] },
      'PUT /api/view-profiles/5': { status: 200, body: P({ id: 5, name: 'New', settings: { v: 1, layers: { rangeRings: true } } }) },
    })
    const s = useProfilesStore()
    await s.load()
    expect(await s.rename(5, 'New')).toBe(true)
    expect(s.list[0].name).toBe('New')
    const putBody = JSON.parse(calls.find((c) => c.method === 'PUT').body)
    expect(putBody.name).toBe('New')
    expect(putBody.settings.layers.rangeRings).toBe(true) // preserved
  })

  it('remove DELETEs and drops it from the list', async () => {
    installFetch({
      'GET /api/view-profiles': { status: 200, body: [P({ id: 1 }), P({ id: 2 })] },
      'DELETE /api/view-profiles/1': { status: 204 },
    })
    const s = useProfilesStore()
    await s.load()
    expect(await s.remove(1)).toBe(true)
    expect(s.list.map((p) => p.id)).toEqual([2])
  })

  it('apply writes a profile’s settings onto the ASD store', async () => {
    installFetch({
      'GET /api/view-profiles': {
        status: 200,
        body: [P({ id: 7, settings: { v: 1, layers: { rangeRings: true }, flFilter: { minFL: 100, maxFL: null, hide: true } } })],
      },
    })
    const s = useProfilesStore()
    const asd = useAsdStore()
    expect(asd.layerVisibility.rangeRings).toBe(false) // default
    await s.load()
    expect(s.apply(7)).toBe(true)
    expect(asd.layerVisibility.rangeRings).toBe(true)
    expect(asd.flFilter.minFL).toBe(100)
    expect(asd.flFilter.hide).toBe(true)
    expect(s.activeId).toBe(7)
    // Unknown id is a safe false.
    expect(s.apply(999)).toBe(false)
  })

  it('canCreate is false once MAX_PROFILES exist', async () => {
    const body = Array.from({ length: MAX_PROFILES }, (_, i) => P({ id: i + 1 }))
    installFetch({ 'GET /api/view-profiles': { status: 200, body } })
    const s = useProfilesStore()
    await s.load()
    expect(s.canCreate).toBe(false)
  })

  // VP-5: apply-on-login.
  it('applyDefaultOnce applies the default profile exactly once', async () => {
    installFetch({
      'GET /api/view-profiles': {
        status: 200,
        body: [P({ id: 1 }), P({ id: 2, is_default: true, settings: { v: 1, layers: { rangeRings: true } } })],
      },
    })
    const s = useProfilesStore()
    const asd = useAsdStore()
    await s.load()
    expect(asd.layerVisibility.rangeRings).toBe(false)
    expect(s.applyDefaultOnce()).toBe(true)
    expect(asd.layerVisibility.rangeRings).toBe(true)
    expect(s.activeId).toBe(2)
    expect(s.defaultApplied).toBe(true)
    // A second call is a no-op (does not re-apply / override a later manual choice).
    asd.setLayerVisibility('rangeRings', false)
    expect(s.applyDefaultOnce()).toBe(false)
    expect(asd.layerVisibility.rangeRings).toBe(false)
  })

  it('applyDefaultOnce is a retryable no-op when there is no default yet', () => {
    installFetch({})
    const s = useProfilesStore()
    // No profiles loaded → no default → false, and the guard does NOT latch.
    expect(s.applyDefaultOnce()).toBe(false)
    expect(s.defaultApplied).toBe(false)
  })
})
