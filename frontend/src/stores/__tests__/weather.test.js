import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useWeatherStore } from '@/stores/weather.js'

// Mock the QNH endpoint by stubbing global fetch (apiFetch reads text()+JSON).
function stubFetch(payload, { ok = true, status = 200 } = {}) {
  globalThis.fetch = vi.fn(async () => ({
    ok,
    status,
    text: async () => JSON.stringify(payload),
  }))
}

beforeEach(() => {
  setActivePinia(createPinia())
})
afterEach(() => {
  vi.restoreAllMocks()
})

describe('weather store — QNH infobox (WX-B)', () => {
  it('starts empty and unavailable', () => {
    const s = useWeatherStore()
    expect(s.stations).toEqual([])
    expect(s.primary).toBe(null)
    expect(s.available).toBe(false)
  })

  it('applyPayload sets stations, primary and availability', () => {
    const s = useWeatherStore()
    s.applyPayload({
      stations: [{ icao: 'EDDF', qnh_hpa: 1013, obs_time: 1700000000, stale: false }],
      primary: { icao: 'EDDF', qnh_hpa: 1013, obs_time: 1700000000, stale: false },
    })
    expect(s.stations).toHaveLength(1)
    expect(s.primary.icao).toBe('EDDF')
    expect(s.available).toBe(true)
  })

  it('poll() fetches and applies the payload', async () => {
    stubFetch({
      stations: [{ icao: 'EDDL', qnh_hpa: 1008, obs_time: 1, stale: true }],
      primary: { icao: 'EDDL', qnh_hpa: 1008, obs_time: 1, stale: true },
    })
    const s = useWeatherStore()
    await s.poll()
    expect(s.primary.qnh_hpa).toBe(1008)
    expect(s.primary.stale).toBe(true)
  })

  it('poll() keeps the last-good value on an error response', async () => {
    const s = useWeatherStore()
    s.applyPayload({
      stations: [{ icao: 'EDDF', qnh_hpa: 1013, obs_time: 1, stale: false }],
      primary: { icao: 'EDDF', qnh_hpa: 1013, obs_time: 1, stale: false },
    })
    stubFetch({ error: 'boom' }, { ok: false, status: 500 })
    await s.poll()
    expect(s.primary.qnh_hpa).toBe(1013) // unchanged
  })

  it('handles an empty (disabled) payload gracefully', async () => {
    stubFetch({ stations: [] })
    const s = useWeatherStore()
    await s.poll()
    expect(s.stations).toEqual([])
    expect(s.available).toBe(false)
  })
})
