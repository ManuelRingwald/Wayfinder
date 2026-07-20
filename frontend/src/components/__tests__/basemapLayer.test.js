// #274 (W1=b / W2=aus): the official BKG base map is a grantable nice-to-have,
// not the foundation — the scope runs purely synthetic (near-black + overlays)
// unless an entitled user opts into the map via the sidebar. Store defaults are
// tested directly; the MapLibre wiring is pinned with source-guards (house
// pattern — the engine's load handler needs a real map to execute).
import { describe, it, expect, beforeEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { setActivePinia, createPinia } from 'pinia'
import { useAsdStore } from '@/stores/asd.js'
import {
  SYNTHETIC_SCOPE_STYLE,
  SYNTHETIC_BACKGROUND_LAYER_ID,
  SYNTHETIC_BACKGROUND_COLOR,
} from '@/map/constants.js'

const src = (rel) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('base-map layer default (#274 W2)', () => {
  it('starts OFF — the scope is synthetic until the user opts in', () => {
    const store = useAsdStore()
    expect(store.layerVisibility.basemap).toBe(false)
  })
})

describe('synthetic scope style (#274 W1=b fallback)', () => {
  it('keeps the LOCAL glyphs so track labels survive a BKG outage', () => {
    expect(SYNTHETIC_SCOPE_STYLE.glyphs).toBe('/glyphs/{fontstack}/{range}.pbf')
  })

  it('is a valid one-layer near-black scope', () => {
    expect(SYNTHETIC_SCOPE_STYLE.version).toBe(8)
    expect(SYNTHETIC_SCOPE_STYLE.layers).toEqual([
      {
        id: SYNTHETIC_BACKGROUND_LAYER_ID,
        type: 'background',
        paint: { 'background-color': SYNTHETIC_BACKGROUND_COLOR },
      },
    ])
  })
})

describe('engine wiring (source-guard)', () => {
  const engine = src('../../map/engine.js')

  it('falls back to the synthetic scope when the style URL fails', () => {
    expect(engine).toContain('mapStyle = SYNTHETIC_SCOPE_STYLE')
    expect(engine).toMatch(/if \(!styleRes\.ok\) throw/)
  })

  it('snapshots the base-style layers before overlays and floors them with the synthetic background', () => {
    expect(engine).toContain('const baseStyleLayers = map.getStyle().layers')
    expect(engine).toContain('state.basemapLayerIds = baseStyleLayers')
    expect(engine).toMatch(/filter\(\(id\) => id !== SYNTHETIC_BACKGROUND_LAYER_ID\)/)
    expect(engine).toMatch(/map\.addLayer\(\s*\{\s*id: SYNTHETIC_BACKGROUND_LAYER_ID/)
  })

  it('applies the store default at load and exposes the toggle group', () => {
    expect(engine).toMatch(/store\.layerVisibility\.basemap \? 'visible' : 'none'/)
    expect(engine).toMatch(/basemap: state\.basemapLayerIds/)
  })

  // E0 (#291): the base-map layers are also bucketed by element group at load,
  // and a per-group visibility helper is exposed for the future element switches
  // (E2/#293). No UI calls it yet — this only pins the capability.
  it('buckets the base-map layers by element group at load', () => {
    expect(engine).toContain("import { bucketBasemapLayers } from './basemapGroups.js'")
    expect(engine).toMatch(/state\.basemapGroups = bucketBasemapLayers\(baseStyleLayers, SYNTHETIC_BACKGROUND_LAYER_ID\)/)
  })

  it('exposes setBasemapGroupVisibility(group, visible) for the per-element switches', () => {
    expect(engine).toMatch(/function setBasemapGroupVisibility\(group, visible\)/)
    expect(engine).toMatch(/const ids = state\.basemapGroups\[group\]/)
    // and it is part of the engine's public API
    expect(engine).toMatch(/return \{[^}]*setBasemapGroupVisibility/)
  })
})

describe('sidebar wiring (source-guard)', () => {
  it('gates the toggle on the basemap entitlement and binds the store key', () => {
    const sidebar = src('../LayerFilterContent.vue')
    expect(sidebar).toMatch(/v-if="showLayer\('basemap'\)"/)
    expect(sidebar).toContain('v-model="store.layerVisibility.basemap"')
    expect(sidebar).toMatch(/onLayerToggle\('basemap', \$event\)/)
  })
})
