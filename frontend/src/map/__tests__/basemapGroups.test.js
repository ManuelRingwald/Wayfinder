// E0 (#291): the schema-agnostic bucketing of BKG base-map layers into element
// groups. Unit-tested against a fixture of REALISTIC layer names from both the
// basemap.de (German stems: Gewaesser/Verkehr/Vegetation/…) and basemap.world
// (OSM stems: water/transportation/landuse/…) styles, since the live style
// cannot be fetched here. The point is robustness: known stems land in the right
// group, and nothing is ever silently dropped.
import { describe, it, expect } from 'vitest'
import { classifyBasemapLayer, bucketBasemapLayers, BASEMAP_GROUPS } from '../basemapGroups.js'

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
