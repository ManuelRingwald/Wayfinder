import { describe, it, expect } from 'vitest'
import { destinationPoint, ringPolygon, rangeRingsGeoJSON, NM_TO_M } from '../rangerings.js'

// haversineM mirrors the sphere (R = EARTH_RADIUS_M) used by destinationPoint,
// so we can verify that generated points sit at the intended ground distance.
const R = 6371000
function haversineM(lat1, lon1, lat2, lon2) {
  const toRad = (d) => (d * Math.PI) / 180
  const dLat = toRad(lat2 - lat1)
  const dLon = toRad(lon2 - lon1)
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) ** 2
  return 2 * R * Math.asin(Math.sqrt(a))
}

describe('destinationPoint', () => {
  it('lands at the requested ground distance for every bearing (constant distance)', () => {
    const d = 50 * NM_TO_M
    for (const brng of [0, 45, 90, 135, 180, 225, 270, 315]) {
      const [lon, lat] = destinationPoint(50, 8, d, brng)
      expect(haversineM(50, 8, lat, lon)).toBeCloseTo(d, 0) // within ~1 m
    }
  })

  it('north bearing raises latitude at constant longitude', () => {
    const [lon, lat] = destinationPoint(50, 8, 10 * NM_TO_M, 0)
    expect(lat).toBeGreaterThan(50)
    expect(lon).toBeCloseTo(8, 6)
  })

  it('east bearing raises longitude (latitude ~level, slight great-circle curve)', () => {
    const [lon, lat] = destinationPoint(50, 8, 10 * NM_TO_M, 90)
    expect(lon).toBeGreaterThan(8)
    expect(lat).toBeLessThan(50)
    expect(lat).toBeCloseTo(50, 1)
  })

  // The anti-squish guarantee: the SAME ground distance produces a LARGER
  // longitude delta than latitude delta in DEGREES (because 1° lon < 1° lat in
  // metres away from the equator). A naive equal-degree ring would make these
  // equal and render squashed.
  it('does not squash longitude: equal metres, unequal degrees', () => {
    const r = 50 * NM_TO_M
    const east = destinationPoint(50, 8, r, 90)
    const north = destinationPoint(50, 8, r, 0)
    expect(haversineM(50, 8, east[1], east[0])).toBeCloseTo(
      haversineM(50, 8, north[1], north[0]),
      0,
    )
    const dLonDeg = Math.abs(east[0] - 8)
    const dLatDeg = Math.abs(north[1] - 50)
    expect(dLonDeg).toBeGreaterThan(dLatDeg)
  })
})

describe('ringPolygon', () => {
  it('is closed with points+1 vertices', () => {
    const poly = ringPolygon(50, 8, 10 * NM_TO_M, 64)
    expect(poly).toHaveLength(65)
    expect(poly[0]).toEqual(poly[64])
  })

  it('places every vertex at the ring radius', () => {
    const r = 30 * NM_TO_M
    for (const [lon, lat] of ringPolygon(50, 8, r, 32)) {
      expect(haversineM(50, 8, lat, lon)).toBeCloseTo(r, 0)
    }
  })
})

describe('rangeRingsGeoJSON', () => {
  it('emits one ring + one label per count with nm = spacing*k', () => {
    const fc = rangeRingsGeoJSON(50, 8, 10, 3)
    const rings = fc.features.filter((f) => f.properties.kind === 'ring')
    const labels = fc.features.filter((f) => f.properties.kind === 'label')
    expect(rings.map((f) => f.properties.nm)).toEqual([10, 20, 30])
    expect(labels.map((f) => f.properties.label)).toEqual(['10 NM', '20 NM', '30 NM'])
  })

  it('clamps non-positive / fractional counts', () => {
    expect(rangeRingsGeoJSON(50, 8, 10, 0).features).toHaveLength(0)
    expect(rangeRingsGeoJSON(50, 8, 10, -2).features).toHaveLength(0)
    const partial = rangeRingsGeoJSON(50, 8, 10, 2.9).features.filter((f) => f.properties.kind === 'ring')
    expect(partial).toHaveLength(2)
  })

  it('staggers labels onto diagonal bearings, never due north', () => {
    // Four rings → labels cycle NE, SE, SW, NW. None sits on the north radial
    // (each is meaningfully offset in longitude), and consecutive rings never
    // share a radial. Guards the fix that moved labels off the 12-o'clock line.
    const labels = rangeRingsGeoJSON(50, 8, 10, 4)
      .features.filter((f) => f.properties.kind === 'label')
      .map((f) => f.geometry.coordinates)
    const quadrant = ([lon, lat]) => [lat > 50 ? 'N' : 'S', lon > 8 ? 'E' : 'W'].join('')
    expect(labels.map(quadrant)).toEqual(['NE', 'SE', 'SW', 'NW'])
    // Every label is off the north/south radial (clearly east or west of centre).
    for (const [lon] of labels) {
      expect(Math.abs(lon - 8)).toBeGreaterThan(0.01)
    }
  })
})
