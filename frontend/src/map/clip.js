// AOI clipping for the DWD weather overlays (#189/#190). The DWD warnings WFS
// returns dissolved warning polygons that can span half of Germany; without
// clipping, one such polygon covers the whole map far beyond the tenant's
// sector ("riesiges gelbes Feld"). We clip every polygon ring to the tenant's
// AOI rectangle so only the in-sector part is drawn. Display-only: a rectangular
// (convex) clip is sufficient and avoids a heavyweight geometry dependency.
//
// The AOI is the WGS84 bbox { minLat, minLon, maxLat, maxLon } from whoami. The
// radar RASTER is clipped separately via the raster source `bounds` (see
// layers.js); this module handles the GeoJSON warnings polygons.

// MASK_WORLD_RING is the outer ring of the base-map AOI mask: the whole Web-
// Mercator-renderable world (±180 lon, ±85 lat). The AOI rectangle is punched
// out as a hole, so the fill covers everything OUTSIDE the sector.
const MASK_WORLD_RING = [
  [-180, -85], [180, -85], [180, 85], [-180, 85], [-180, -85],
]

// aoiMaskFeature (#289) builds the GeoJSON Polygon for the base-map mask: a
// world-spanning fill with a rectangular HOLE at the tenant AOI, so the map is
// visible only inside the sector and covered (by the scope backdrop colour)
// outside it. Returns null when no AOI is configured or a bound is non-finite —
// the caller then draws nothing (full map, no clip). Kept pure/testable here
// next to the other AOI geometry; a future circular variant (issue #289 radius)
// only swaps this one function's hole ring.
export function aoiMaskFeature(bbox) {
  if (!bbox) return null
  const { minLat, minLon, maxLat, maxLon } = bbox
  if (![minLat, minLon, maxLat, maxLon].every(Number.isFinite)) return null
  const hole = [
    [minLon, minLat], [maxLon, minLat], [maxLon, maxLat], [minLon, maxLat], [minLon, minLat],
  ]
  return {
    type: 'Feature',
    properties: {},
    geometry: { type: 'Polygon', coordinates: [MASK_WORLD_RING, hole] },
  }
}

// clipRingToBBox clips a single linear ring (array of [lon, lat]) to the AOI
// rectangle using the Sutherland–Hodgman algorithm (convex clip window). Returns
// the clipped ring, or an empty array when the ring falls entirely outside.
function clipRingToBBox(ring, bbox) {
  const { minLon, minLat, maxLon, maxLat } = bbox
  // Each edge: keep points on the inside half-plane, insert intersections.
  // inside(p) and intersect(a,b) are specialised per rectangle edge.
  const edges = [
    { inside: (p) => p[0] >= minLon, x: minLon, axis: 0 }, // left
    { inside: (p) => p[0] <= maxLon, x: maxLon, axis: 0 }, // right
    { inside: (p) => p[1] >= minLat, y: minLat, axis: 1 }, // bottom
    { inside: (p) => p[1] <= maxLat, y: maxLat, axis: 1 }, // top
  ]

  let output = ring
  for (const edge of edges) {
    if (output.length === 0) break
    const input = output
    output = []
    for (let i = 0; i < input.length; i++) {
      const cur = input[i]
      const prev = input[(i + input.length - 1) % input.length]
      const curIn = edge.inside(cur)
      const prevIn = edge.inside(prev)
      if (curIn) {
        if (!prevIn) output.push(intersect(prev, cur, edge))
        output.push(cur)
      } else if (prevIn) {
        output.push(intersect(prev, cur, edge))
      }
    }
  }
  return output
}

// intersect returns the point where segment a→b crosses the given rectangle edge.
function intersect(a, b, edge) {
  if (edge.axis === 0) {
    const t = (edge.x - a[0]) / (b[0] - a[0])
    return [edge.x, a[1] + t * (b[1] - a[1])]
  }
  const t = (edge.y - a[1]) / (b[1] - a[1])
  return [a[0] + t * (b[0] - a[0]), edge.y]
}

// clipPolygon clips a GeoJSON Polygon coordinate array (outer ring + holes) to
// the AOI. Each ring is clipped independently; empty rings are dropped. Returns
// null when the outer ring is fully clipped away.
function clipPolygon(rings, bbox) {
  const out = []
  for (let r = 0; r < rings.length; r++) {
    const clipped = clipRingToBBox(rings[r], bbox)
    if (r === 0) {
      if (clipped.length < 3) return null // outer ring gone → drop polygon
      out.push(closeRing(clipped))
    } else if (clipped.length >= 3) {
      out.push(closeRing(clipped))
    }
  }
  return out
}

// closeRing ensures the ring's first and last positions coincide (GeoJSON rule).
function closeRing(ring) {
  const first = ring[0]
  const last = ring[ring.length - 1]
  if (first[0] !== last[0] || first[1] !== last[1]) return [...ring, first]
  return ring
}

// clipFeatureCollectionToBBox returns a new FeatureCollection with every Polygon
// / MultiPolygon feature clipped to the AOI bbox. Non-polygon geometries and
// features that fall entirely outside are dropped. A null/undefined bbox returns
// the collection unchanged (no AOI configured → show everything).
export function clipFeatureCollectionToBBox(fc, bbox) {
  if (!bbox || !fc || !Array.isArray(fc.features)) return fc
  const features = []
  for (const f of fc.features) {
    const g = f.geometry
    if (!g) continue
    if (g.type === 'Polygon') {
      const rings = clipPolygon(g.coordinates, bbox)
      if (rings) features.push({ ...f, geometry: { type: 'Polygon', coordinates: rings } })
    } else if (g.type === 'MultiPolygon') {
      const polys = []
      for (const poly of g.coordinates) {
        const rings = clipPolygon(poly, bbox)
        if (rings) polys.push(rings)
      }
      if (polys.length) features.push({ ...f, geometry: { type: 'MultiPolygon', coordinates: polys } })
    }
    // Other geometry types (points/lines) are not part of the warnings overlay.
  }
  return { type: 'FeatureCollection', features }
}
