// E0 (#291): the schema-agnostic bucketing of BKG base-map layers into element
// groups. Unit-tested against a fixture of REALISTIC layer names from both the
// basemap.de (German stems: Gewaesser/Verkehr/Vegetation/…) and basemap.world
// (OSM stems: water/transportation/landuse/…) styles, since the live style
// cannot be fetched here. The point is robustness: known stems land in the right
// group, and nothing is ever silently dropped.
import { describe, it, expect } from 'vitest'
import { classifyBasemapLayer, bucketBasemapLayers, BASEMAP_GROUPS, BASEMAP_ELEMENTS, BASEMAP_PRESETS, matchPreset } from '../basemapGroups.js'

// basemap.de (German source-layer stems) — the primary target.
const bmDe = [
  ['gewaesserflaeche_see', 'gewaesserflaeche', 'fill', 'water'],
  ['gewaesserlinie_fluss', 'gewaesserlinie', 'line', 'water'],
  ['verkehrsflaeche', 'verkehrsflaeche', 'fill', 'traffic'],
  ['verkehr_strasse_autobahn', 'verkehrslinie', 'line', 'traffic'],
  ['bahnverkehr', 'bahnverkehr', 'line', 'traffic'],
  ['vegetationsflaeche_wald', 'vegetationsflaeche', 'fill', 'vegetation'],
  ['siedlungsflaeche', 'siedlungsflaeche', 'fill', 'settlement'],
  ['gebaeude', 'gebaeudeflaeche', 'fill', 'building'],
  ['verwaltungsgrenze_kreis', 'verwaltungseinheit', 'line', 'boundary'],
  ['beschriftung_ort', 'beschriftung', 'symbol', 'label'],
  ['hintergrund', '', 'background', 'background'],
]

// basemap.world (OSM-derived source-layer stems) — the cross-border style.
const bmWorld = [
  ['water', 'water', 'fill', 'water'],
  ['waterway_river', 'waterway', 'line', 'water'],
  ['transportation_motorway', 'transportation', 'line', 'traffic'],
  ['building', 'building', 'fill', 'building'],
  ['boundary_country', 'boundary', 'line', 'boundary'],
  ['landuse_residential', 'landuse', 'fill', 'settlement'],
  ['park_national', 'park', 'fill', 'vegetation'],
  ['place_city', 'place', 'symbol', 'label'],
  ['water_name', 'water_name', 'symbol', 'label'], // a symbol IS a label, even over water
]

const asLayer = ([id, sl, type]) => ({ id, 'source-layer': sl, type })

describe('classifyBasemapLayer — basemap.de stems', () => {
  it.each(bmDe)('%s → %s', (id, sl, type, want) => {
    expect(classifyBasemapLayer(asLayer([id, sl, type]))).toBe(want)
  })
})

describe('classifyBasemapLayer — basemap.world stems', () => {
  it.each(bmWorld)('%s → %s', (id, sl, type, want) => {
    expect(classifyBasemapLayer(asLayer([id, sl, type]))).toBe(want)
  })
})

describe('classifyBasemapLayer — edge cases', () => {
  it('does not mistake a "chaussee" road for water (no bare "see" match)', () => {
    expect(classifyBasemapLayer({ id: 'verkehr_chaussee', 'source-layer': 'verkehrslinie', type: 'line' })).toBe('traffic')
  })

  it('classifies a building before the generic settlement land-use bucket', () => {
    expect(classifyBasemapLayer({ id: 'x', 'source-layer': 'gebaeude', type: 'fill' })).toBe('building')
  })

  it('classifies forest land-cover as vegetation, not settlement', () => {
    expect(classifyBasemapLayer({ id: 'landcover_wood', 'source-layer': 'landcover', type: 'fill' })).toBe('vegetation')
  })

  it('falls back to "other" for an unrecognised layer (never dropped)', () => {
    expect(classifyBasemapLayer({ id: 'mystery', 'source-layer': 'weird', type: 'fill' })).toBe('other')
  })

  it('is safe on a null/empty layer', () => {
    expect(classifyBasemapLayer(null)).toBe('other')
    expect(classifyBasemapLayer({})).toBe('other')
  })
})

describe('bucketBasemapLayers', () => {
  const layers = [...bmDe, ...bmWorld].map(asLayer)

  it('returns an entry for every group (stable shape)', () => {
    const groups = bucketBasemapLayers(layers)
    for (const g of BASEMAP_GROUPS) expect(Array.isArray(groups[g])).toBe(true)
  })

  it('never drops a layer: the buckets partition the input exactly', () => {
    const groups = bucketBasemapLayers(layers)
    const total = Object.values(groups).reduce((n, ids) => n + ids.length, 0)
    expect(total).toBe(layers.length)
  })

  it('excludes the synthetic scope floor', () => {
    const withFloor = [...layers, { id: 'synthetic-scope-floor', type: 'background' }]
    const groups = bucketBasemapLayers(withFloor, 'synthetic-scope-floor')
    const all = Object.values(groups).flat()
    expect(all).not.toContain('synthetic-scope-floor')
    expect(all.length).toBe(layers.length)
  })

  it('groups the rivers together so "only rivers" is possible (E2/#293)', () => {
    const groups = bucketBasemapLayers(layers)
    expect(groups.water).toContain('gewaesserlinie_fluss')
    expect(groups.water).toContain('waterway_river')
    expect(groups.traffic).not.toContain('gewaesserlinie_fluss')
  })

  it('is safe on empty/undefined input', () => {
    expect(bucketBasemapLayers(undefined).water).toEqual([])
    expect(bucketBasemapLayers([]).other).toEqual([])
  })
})

describe('BASEMAP_ELEMENTS (E2 exposed sidebar switches)', () => {
  it('exposes the meaningful groups with labels, but NOT the "other" catch-all', () => {
    const ids = BASEMAP_ELEMENTS.map((e) => e.id)
    expect(ids).not.toContain('other') // unclassified layers follow the master
    for (const e of BASEMAP_ELEMENTS) {
      expect(BASEMAP_GROUPS).toContain(e.id) // every element is a real group
      expect(typeof e.label).toBe('string')
      expect(e.label.length).toBeGreaterThan(0)
    }
    expect(ids).toContain('water')
    expect(ids).toContain('traffic')
  })
})

describe('BASEMAP_PRESETS (E3 one-click element sets)', () => {
  it('every preset assigns a value to EVERY element (deterministic apply)', () => {
    const elementIds = BASEMAP_ELEMENTS.map((e) => e.id)
    for (const p of BASEMAP_PRESETS) {
      for (const id of elementIds) {
        expect(typeof p.elements[id]).toBe('boolean')
      }
      expect(typeof p.label).toBe('string')
    }
  })

  it('"Detailliert" turns every element on', () => {
    const detail = BASEMAP_PRESETS.find((p) => p.id === 'detail')
    for (const e of BASEMAP_ELEMENTS) expect(detail.elements[e.id]).toBe(true)
  })

  it('presets are distinct (Minimal reduces vs Standard vs Detailliert)', () => {
    const onCount = (p) => BASEMAP_ELEMENTS.filter((e) => p.elements[e.id]).length
    const [min, std, det] = ['minimal', 'standard', 'detail'].map((id) => BASEMAP_PRESETS.find((p) => p.id === id))
    expect(onCount(min)).toBeLessThan(onCount(std))
    expect(onCount(std)).toBeLessThan(onCount(det))
  })
})

describe('matchPreset', () => {
  it('matches a preset when the element set equals it', () => {
    const std = BASEMAP_PRESETS.find((p) => p.id === 'standard')
    expect(matchPreset(std.elements)).toBe('standard')
  })

  it('returns null ("Benutzerdefiniert") for a set matching no preset', () => {
    // all-off matches no preset (Minimal keeps water/boundary/label on)
    const allOff = Object.fromEntries(BASEMAP_ELEMENTS.map((e) => [e.id, false]))
    expect(matchPreset(allOff)).toBeNull()
  })

  it('does not falsely match when an extra key differs', () => {
    const det = BASEMAP_PRESETS.find((p) => p.id === 'detail')
    // detail is all-on; flip one element → custom
    expect(matchPreset({ ...det.elements, building: false })).toBeNull()
  })

  it('is null-safe', () => {
    expect(matchPreset(null)).toBeNull()
  })
})
