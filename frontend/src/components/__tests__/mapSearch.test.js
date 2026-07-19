// #277 (ADR 0028): sector search — the Lotse finds a street/place inside the
// tenant's AOI ("eine Drohne startet aus der Friedrichstraße"). The server
// builds a lazy per-AOI index from the base map's vector tiles and answers
// /api/basemap/search; the UI polls through the 202 build phase and drops a
// marker on the picked hit. MapLibre wiring is pinned with source-guards
// (house pattern); the MapSearch component itself is exercised with a mounted
// instance against a stubbed fetch.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import MapSearch from '../MapSearch.vue'
import {
  SEARCH_MARKER_SOURCE_ID,
  SEARCH_MARKER_LAYER_ID,
  SEARCH_MARKER_LABEL_LAYER_ID,
} from '@/map/constants.js'

const src = (rel) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

describe('search marker constants (#277)', () => {
  it('defines the single-point marker source and its two layers', () => {
    expect(SEARCH_MARKER_SOURCE_ID).toBe('search-marker')
    expect(SEARCH_MARKER_LAYER_ID).toBeTruthy()
    expect(SEARCH_MARKER_LABEL_LAYER_ID).toBeTruthy()
  })
})

describe('engine wiring (source-guard)', () => {
  const engine = src('../../map/engine.js')

  it('adds the marker layer LAST so a found place is never buried under tracks', () => {
    const addMarker = engine.indexOf('addSearchMarkerLayer(map, palette)')
    const addLabels = engine.indexOf('addLabelsLayer(map, palette)')
    expect(addMarker).toBeGreaterThan(addLabels)
  })

  it('exposes showSearchMarker (marker + camera) and clearSearchMarker', () => {
    expect(engine).toMatch(/function showSearchMarker\(lon, lat, name\)/)
    expect(engine).toMatch(/map\.easeTo\(\{ center: \[lon, lat\]/)
    expect(engine).toMatch(/function clearSearchMarker\(\)/)
    expect(engine).toMatch(/showSearchMarker, clearSearchMarker \}/)
  })
})

describe('MapCanvas / AsdView wiring (source-guard)', () => {
  it('MapCanvas exposes the marker calls to the view layer', () => {
    const canvas = src('../MapCanvas.vue')
    expect(canvas).toMatch(/showSearchMarker: \(lon, lat, name\)/)
    expect(canvas).toContain('clearSearchMarker: () => mapEngine?.clearSearchMarker()')
  })

  it('AsdView mounts MapSearch behind the basemap entitlement gate', () => {
    const view = src('../../views/AsdView.vue')
    expect(view).toMatch(/<MapSearch\s/)
    expect(view).toMatch(/v-if="showSearch"/)
    expect(view).toMatch(/session\.isAdmin \|\| session\.hasFeature\('basemap'\)/)
    expect(view).toMatch(/mapCanvas\.value\?\.showSearchMarker\(hit\.lon, hit\.lat, hit\.name\)/)
  })
})

// ---- MapSearch component behaviour ------------------------------------------

function jsonResponse(status, body) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  })
}

describe('MapSearch component (#277)', () => {
  // jsdom has no ResizeObserver; Vuetify's field components expect one.
  if (typeof globalThis.ResizeObserver === 'undefined') {
    globalThis.ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
  }
  const vuetify = createVuetify({ components, directives })
  const mountSearch = () => mount(MapSearch, { global: { plugins: [vuetify] } })

  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  async function typeQuery(wrapper, text) {
    await wrapper.find('input').setValue(text)
    await vi.advanceTimersByTimeAsync(350) // past the 300 ms debounce
  }

  it('debounces, queries the endpoint and lists ready hits; picking emits select', async () => {
    const hits = [
      { name: 'Friedrichstraße', category: 'verkehrslinie', lat: 50.04, lon: 8.56 },
    ]
    global.fetch = vi.fn(() => jsonResponse(200, { status: 'ready', results: hits }))
    const wrapper = mountSearch()

    await typeQuery(wrapper, 'friedrich')
    expect(global.fetch).toHaveBeenCalledTimes(1)
    expect(global.fetch.mock.calls[0][0]).toBe('/api/basemap/search?q=friedrich')

    const rows = wrapper.findAll('.v-list-item')
    expect(rows).toHaveLength(1)
    expect(wrapper.text()).toContain('Friedrichstraße')
    expect(wrapper.text()).toContain('Straße / Weg') // category label mapping

    await rows[0].trigger('click')
    expect(wrapper.emitted('select')[0][0]).toEqual(hits[0])
  })

  it('does not query below two characters', async () => {
    global.fetch = vi.fn()
    const wrapper = mountSearch()
    await typeQuery(wrapper, 'f')
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('shows the building hint on 202 and polls until ready', async () => {
    let call = 0
    global.fetch = vi.fn(() => {
      call++
      return call === 1
        ? jsonResponse(202, { status: 'building' })
        : jsonResponse(200, { status: 'ready', results: [] })
    })
    const wrapper = mountSearch()

    await typeQuery(wrapper, 'friedrich')
    expect(wrapper.text()).toContain('Suchindex wird aufgebaut')

    await vi.advanceTimersByTimeAsync(1600) // past the 1500 ms building retry
    expect(global.fetch).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('Keine Treffer')
  })

  it('clearing emits clear and stops the building poll', async () => {
    global.fetch = vi.fn(() => jsonResponse(202, { status: 'building' }))
    const wrapper = mountSearch()
    await typeQuery(wrapper, 'friedrich')
    expect(global.fetch).toHaveBeenCalledTimes(1)

    await wrapper.find('input').trigger('keydown', { key: 'Escape' })
    expect(wrapper.emitted('clear')).toBeTruthy()

    await vi.advanceTimersByTimeAsync(5000)
    expect(global.fetch).toHaveBeenCalledTimes(1) // poll cancelled
  })
})
