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

  it('applies the base map at load via applyBasemap (master × elements, E2)', () => {
    // #274 + E2 (#293): the load handler no longer flat-shows every basemap layer;
    // it calls applyBasemap(), which combines the master with the per-element
    // switches. So basemap is NOT a plain entry in the flat show/hide group map.
    expect(engine).toMatch(/applyBasemap\(\)/)
    expect(engine).not.toMatch(/basemap: state\.basemapLayerIds/)
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

// E2 (#293): per-element base-map switches ("only rivers"/"only roads"). Store
// defaults are tested directly; the wiring (engine applyBasemap, MapCanvas
// watcher, sidebar sub-rows) is pinned with source-guards.
describe('base-map element switches (E2 #293)', () => {
  const engine = src('../../map/engine.js')
  const sidebar = src('../LayerFilterContent.vue')
  const canvas = src('../MapCanvas.vue')

  it('all element groups start visible — nothing changes until the operator hides one', () => {
    const store = useAsdStore()
    for (const v of Object.values(store.basemapElementVisibility)) expect(v).toBe(true)
    store.setBasemapElement('water', false)
    expect(store.basemapElementVisibility.water).toBe(false)
  })

  it('applyBasemap combines the master with the per-element visibility', () => {
    expect(engine).toMatch(/function applyBasemap\(\)/)
    expect(engine).toContain('store.layerVisibility.basemap')
    expect(engine).toContain('store.basemapElementVisibility')
    // visible iff master on AND element on; an unclassified group ('other',
    // absent from the map) defaults on, so it follows the master.
    expect(engine).toMatch(/on && \(el === undefined \? true : el\)/)
    expect(engine).toMatch(/return \{[^}]*applyBasemap/)
  })

  it('routes basemap through applyBasemap, not the flat show/hide loop', () => {
    expect(engine).toMatch(/if \('basemap' in vis\) applyBasemap\(\)/)
  })

  it('MapCanvas re-applies the base map when an element switch changes', () => {
    expect(canvas).toMatch(/watch\(\(\) => \(\{ \.\.\.store\.basemapElementVisibility \}\)/)
    expect(canvas).toContain('mapEngine?.applyBasemap()')
  })

  it('renders an element switch per BASEMAP_ELEMENTS, disabled while the map is off', () => {
    expect(sidebar).toContain("import { BASEMAP_ELEMENTS, BASEMAP_PRESETS, matchPreset } from '@/map/basemapGroups.js'")
    expect(sidebar).toMatch(/v-for="el in BASEMAP_ELEMENTS"/)
    expect(sidebar).toContain('store.basemapElementVisibility[el.id]')
    expect(sidebar).toContain('store.setBasemapElement(el.id, $event)')
    expect(sidebar).toMatch(/:disabled="!store\.layerVisibility\.basemap"/)
  })
})

// E3 (#294): one-click element presets (Minimal/Standard/Detailliert).
describe('base-map element presets (E3 #294)', () => {
  const sidebar = src('../LayerFilterContent.vue')

  it('the store applies a preset by id (unknown id is a no-op)', () => {
    const store = useAsdStore()
    store.applyBasemapPreset('minimal')
    // Minimal keeps water/boundary/label, drops building/traffic/…
    expect(store.basemapElementVisibility.building).toBe(false)
    expect(store.basemapElementVisibility.boundary).toBe(true)
    const before = { ...store.basemapElementVisibility }
    store.applyBasemapPreset('does-not-exist')
    expect({ ...store.basemapElementVisibility }).toEqual(before)
  })

  it('renders the preset buttons, shown only while the map is on, active one highlighted', () => {
    expect(sidebar).toMatch(/v-for="p in BASEMAP_PRESETS"/)
    expect(sidebar).toContain('store.applyBasemapPreset(p.id)')
    expect(sidebar).toContain('activeBasemapPreset')
    expect(sidebar).toContain('matchPreset(store.basemapElementVisibility)')
    // gated on the master being on (presets are meaningless when the map is off)
    expect(sidebar).toMatch(/v-if="showLayer\('basemap'\) && store\.layerVisibility\.basemap"/)
  })
})

// #289: limit the BKG base map to the tenant AOI via a mask layer (covers the
// map outside the sector). Source-guards over the engine + layers wiring.
describe('base-map AOI mask (#289)', () => {
  const engine = src('../../map/engine.js')
  const layers = src('../../map/layers.js')

  it('adds the mask ABOVE the weather overlays so weather is clipped to the AOI too (#324)', () => {
    // The mask is registered AFTER the weather radar + warnings, so the backdrop
    // covers everything outside the AOI including the weather (which the radar
    // raster would otherwise bleed past via tile-granular bounds). It still sits
    // below the aeronautical overlay, added afterwards.
    expect(engine).toMatch(/addWeatherRadarLayer\(map, weatherAOI\)[\s\S]*addWeatherWarningsLayer\(map\)[\s\S]*addBasemapMaskLayer\(map, weatherAOI\)[\s\S]*addAeronauticalIcons/)
    // Guard against a regression to the old order (mask before the radar).
    expect(engine).not.toMatch(/addBasemapMaskLayer\(map, weatherAOI\)[\s\S]*addWeatherRadarLayer/)
  })

  it('keeps the radar below the warnings (hence below the mask) on every (re-)add (#324)', () => {
    // addWeatherRadarLayer inserts with beforeId = warnings fill when it exists,
    // so setWeatherRadarAOI's remove+re-add never lifts the radar above the mask.
    expect(layers).toMatch(/map\.getLayer\(WEATHER_WARNINGS_FILL_LAYER_ID\) \? WEATHER_WARNINGS_FILL_LAYER_ID : undefined/)
    expect(layers).toMatch(/paint: \{ 'raster-opacity': WEATHER_RADAR_OPACITY \},\s*\}, beforeId\)/)
  })

  it('re-cuts the mask when the AOI resolves/changes (the AOI hook)', () => {
    expect(engine).toMatch(/function applyWeatherAOI\(aoi\)[\s\S]*setBasemapMaskAOI\(map, aoi\)/)
  })

  it('the mask fill uses the scope backdrop colour and clears on a null AOI', () => {
    expect(layers).toContain("'fill-color': SYNTHETIC_BACKGROUND_COLOR")
    expect(layers).toMatch(/aoiMaskFeature\(aoi\) \|\| EMPTY_FC/)
    expect(layers).toMatch(/function setBasemapMaskAOI\(map, aoi\)/)
  })
})
