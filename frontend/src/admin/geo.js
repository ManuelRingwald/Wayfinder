// AP3 (ADR 0009): client-side conversion between an operator-facing
// "center + radius (NM)" view input and the AOI bounding box the backend stores.
//
// The backend stays AOI-based (WF2-21.2 is untouched): this conversion is a UX
// convenience applied *before* PUT /api/admin/tenants/{id}/view and reversed when
// loading an existing view, so an admin reasons in nautical miles around a centre
// rather than in raw min/max lat/lon corners.
//
// Wire shape: the bounding box is expressed in the backend's snake_case field
// names (min_lat/min_lon/max_lat/max_lon, matching store.BBox's JSON tags) on
// both ends, so radiusNmToBbox's output goes straight onto the wire and
// bboxToRadius reads a stored AOI directly — no per-call-site key remapping (a
// camelCase/snake_case mismatch there silently dropped the radius, WF-radius-bug).
//
// Geometry (small-angle flat-earth approximation, adequate for an AOI a few
// hundred NM across):
//   1 NM ≈ 1 arc-minute of latitude  → lat_delta = R / 60
//   longitude degrees shrink with latitude → lon_delta = R / (60 · cos φ)

// EARTH_NM_PER_DEGREE_LAT is the nautical miles in one degree of latitude
// (60 arc-minutes × 1 NM/arc-minute).
const NM_PER_DEGREE_LAT = 60

// radiusNmToBbox converts a centre point and a radius in nautical miles into an
// AOI bounding box { min_lat, min_lon, max_lat, max_lon } (backend wire shape).
// The box is the square that circumscribes the radius (half-width = radius on
// each axis), clamped to valid WGS84 ranges. Returns null for a non-positive or
// non-finite radius (no AOI).
export function radiusNmToBbox(centerLat, centerLon, radiusNm) {
  if (!Number.isFinite(centerLat) || !Number.isFinite(centerLon)) return null
  if (!Number.isFinite(radiusNm) || radiusNm <= 0) return null

  const latDelta = radiusNm / NM_PER_DEGREE_LAT
  // Guard the pole singularity: cos(φ) → 0 makes lon_delta explode. Above ~89.9°
  // the AOI spans all longitudes anyway, so clamp to a full-width band.
  const cosLat = Math.cos((centerLat * Math.PI) / 180)
  const lonDelta = Math.abs(cosLat) < 1e-6 ? 180 : radiusNm / (NM_PER_DEGREE_LAT * cosLat)

  return {
    min_lat: clamp(centerLat - latDelta, -90, 90),
    max_lat: clamp(centerLat + latDelta, -90, 90),
    min_lon: clamp(centerLon - Math.abs(lonDelta), -180, 180),
    max_lon: clamp(centerLon + Math.abs(lonDelta), -180, 180),
  }
}

// bboxToRadius derives an approximate centre and radius (NM) from an AOI bounding
// box in the backend wire shape ({ min_lat, min_lon, max_lat, max_lon }), the
// inverse of radiusNmToBbox for round-tripping a stored view back into the
// operator input. The radius is taken from the latitude half-height (the axis
// independent of longitude convergence), so a box produced by radiusNmToBbox
// round-trips to its original radius. Returns { centerLat, centerLon, radiusNm }
// (centre+radius, not a bbox), or null for a missing/degenerate box.
export function bboxToRadius(bbox) {
  if (!bbox || !Number.isFinite(bbox.min_lat) || !Number.isFinite(bbox.max_lat)) return null
  if (!Number.isFinite(bbox.min_lon) || !Number.isFinite(bbox.max_lon)) return null

  const centerLat = (bbox.min_lat + bbox.max_lat) / 2
  const centerLon = (bbox.min_lon + bbox.max_lon) / 2
  const latHalfHeight = (bbox.max_lat - bbox.min_lat) / 2
  const radiusNm = latHalfHeight * NM_PER_DEGREE_LAT
  if (radiusNm <= 0) return null

  return { centerLat, centerLon, radiusNm }
}

function clamp(v, lo, hi) {
  return Math.min(hi, Math.max(lo, v))
}
