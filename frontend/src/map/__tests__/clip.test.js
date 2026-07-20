// #190: AOI clipping of the DWD weather-warnings polygons. A dissolved warning
// region can span half of Germany; clipping it to the tenant AOI rectangle keeps
// only the in-sector part so the map no longer shows a "riesiges gelbes Feld".
import { describe, it, expect } from 'vitest'
import { clipFeatureCollectionToBBox, aoiMaskFeature } from '../clip.js'

const AOI = { minLon: 0, minLat: 0, maxLon: 10, maxLat: 10 }

function poly(coords) {
  return { type: 'Feature', properties: {}, geometry: { type: 'Polygon', coordinates: [coords] } }
}

describe('clipFeatureCollectionToBBox', () => {
  it('returns the collection unchanged when no AOI is given', () => {
    const fc = { type: 'FeatureCollection', features: [poly([[0, 0], [100, 0], [100, 100], [0, 0]])] }
    expect(clipFeatureCollectionToBBox(fc, null)).toBe(fc)
  })

  it('clips a polygon that extends far beyond the AOI to the AOI rectangle', () => {
    // A huge triangle covering way past the 10×10 AOI.
    const fc = { type: 'FeatureCollection', features: [poly([[-50, -50], [50, -50], [50, 50], [-50, 50], [-50, -50]])] }
    const out = clipFeatureCollectionToBBox(fc, AOI)
    expect(out.features).toHaveLength(1)
    const ring = out.features[0].geometry.coordinates[0]
    // Every clipped vertex must lie within the AOI bounds.
    for (const [lon, lat] of ring) {
      expect(lon).toBeGreaterThanOrEqual(AOI.minLon - 1e-9)
      expect(lon).toBeLessThanOrEqual(AOI.maxLon + 1e-9)
      expect(lat).toBeGreaterThanOrEqual(AOI.minLat - 1e-9)
      expect(lat).toBeLessThanOrEqual(AOI.maxLat + 1e-9)
    }
  })

  it('drops a polygon entirely outside the AOI', () => {
    const fc = { type: 'FeatureCollection', features: [poly([[20, 20], [30, 20], [30, 30], [20, 20]])] }
    const out = clipFeatureCollectionToBBox(fc, AOI)
    expect(out.features).toHaveLength(0)
  })

  it('keeps a polygon fully inside the AOI (closed ring)', () => {
    const inside = [[2, 2], [8, 2], [8, 8], [2, 8], [2, 2]]
    const fc = { type: 'FeatureCollection', features: [poly(inside)] }
    const out = clipFeatureCollectionToBBox(fc, AOI)
    expect(out.features).toHaveLength(1)
    const ring = out.features[0].geometry.coordinates[0]
    expect(ring[0]).toEqual(ring[ring.length - 1]) // ring stays closed
  })

  it('clips each polygon of a MultiPolygon and drops empty ones', () => {
    const fc = {
      type: 'FeatureCollection',
      features: [{
        type: 'Feature',
        properties: {},
        geometry: {
          type: 'MultiPolygon',
          coordinates: [
            [[[2, 2], [8, 2], [8, 8], [2, 2]]],       // inside → kept
            [[[50, 50], [60, 50], [60, 60], [50, 50]]], // outside → dropped
          ],
        },
      }],
    }
    const out = clipFeatureCollectionToBBox(fc, AOI)
    expect(out.features).toHaveLength(1)
    expect(out.features[0].geometry.coordinates).toHaveLength(1)
  })
})

// #289: the base-map AOI mask geometry — a world-spanning fill with a rectangular
// hole at the tenant AOI, so the official BKG base map is limited to the sector.
describe('aoiMaskFeature (#289 base-map AOI mask)', () => {
  it('returns null when no AOI is configured (→ full map, no clip)', () => {
    expect(aoiMaskFeature(null)).toBeNull()
    expect(aoiMaskFeature(undefined)).toBeNull()
  })

  it('returns null when a bound is non-finite (never a broken polygon)', () => {
    expect(aoiMaskFeature({ minLat: 0, minLon: 0, maxLat: NaN, maxLon: 10 })).toBeNull()
  })

  it('builds a Polygon: outer world ring + AOI hole', () => {
    const f = aoiMaskFeature({ minLat: 48, minLon: 8, maxLat: 50, maxLon: 12 })
    expect(f.type).toBe('Feature')
    expect(f.geometry.type).toBe('Polygon')
    const [outer, hole] = f.geometry.coordinates
    // outer ring spans the renderable world (covers everything to be masked)
    expect(outer).toContainEqual([-180, -85])
    expect(outer).toContainEqual([180, 85])
    // the hole is the AOI rectangle, closed (first === last)
    expect(hole[0]).toEqual([8, 48])
    expect(hole).toContainEqual([12, 50])
    expect(hole[0]).toEqual(hole[hole.length - 1])
  })
})
