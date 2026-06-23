// Range rings (ASD-012): concentric, operator-configurable circles of CONSTANT
// GROUND DISTANCE around the configured display centre, so a controller reads
// slant/ground distance at a glance. Distinct from the Paket-6 sensor coverage
// rings (which show each radar's reach).
//
// Geodesic generation — why it matters on Web Mercator: each ring vertex is the
// destination point at a true distance d and bearing along the sphere
// (destination-point formula). This keeps every vertex at the SAME ground
// distance in all directions. The naive shortcut "add d/111320° to both lat and
// lon" would squash the ring in longitude away from the equator (1° lon < 1° lat
// in metres), drawing a wrong, egg-flat shape. A true constant-distance ring is
// the correct representation on the Mercator base — it marks where N NM actually
// is; it renders very slightly taller than wide at high latitude, which is
// faithful, not a defect.
import { EARTH_RADIUS_M } from './constants.js'

// Metres per nautical mile (exact, by definition).
export const NM_TO_M = 1852

const DEG = Math.PI / 180
const RAD = 180 / Math.PI

// destinationPoint returns the [lon, lat] reached from (lat, lon) after
// travelling distanceM metres along bearingDeg (0 = North, clockwise), on a
// sphere of radius EARTH_RADIUS_M. Longitude is normalised to [-180, 180].
export function destinationPoint(lat, lon, distanceM, bearingDeg) {
  const angDist = distanceM / EARTH_RADIUS_M // angular distance (radians)
  const brng = bearingDeg * DEG
  const lat1 = lat * DEG
  const lon1 = lon * DEG

  const sinLat2 =
    Math.sin(lat1) * Math.cos(angDist) +
    Math.cos(lat1) * Math.sin(angDist) * Math.cos(brng)
  const lat2 = Math.asin(Math.max(-1, Math.min(1, sinLat2)))
  const lon2 =
    lon1 +
    Math.atan2(
      Math.sin(brng) * Math.sin(angDist) * Math.cos(lat1),
      Math.cos(angDist) - Math.sin(lat1) * sinLat2,
    )

  let lonDeg = lon2 * RAD
  lonDeg = ((lonDeg + 540) % 360) - 180 // normalise to [-180, 180]
  return [lonDeg, lat2 * RAD]
}

// ringPolygon returns the closed [lon,lat] ring of radiusM around the centre,
// sampled with `points` segments (default 128 → smooth at display scale). The
// first and last vertex coincide so it renders as a closed LineString.
export function ringPolygon(centerLat, centerLon, radiusM, points = 128) {
  const coords = []
  for (let i = 0; i <= points; i++) {
    const bearing = (i / points) * 360
    coords.push(destinationPoint(centerLat, centerLon, radiusM, bearing))
  }
  return coords
}

// rangeRingsGeoJSON builds the overlay FeatureCollection: one LineString per
// ring (property `nm`) plus one label Point per ring (placed due north of the
// centre so labels stack along the 12-o'clock radius). count is clamped to a
// non-negative integer; spacingNM/count come from the reactive store (ASD-012).
export function rangeRingsGeoJSON(centerLat, centerLon, spacingNM, count, points = 128) {
  const features = []
  const n = Math.max(0, Math.floor(count))
  for (let k = 1; k <= n; k++) {
    const nm = spacingNM * k
    const radiusM = nm * NM_TO_M
    features.push({
      type: 'Feature',
      properties: { nm, kind: 'ring' },
      geometry: { type: 'LineString', coordinates: ringPolygon(centerLat, centerLon, radiusM, points) },
    })
    features.push({
      type: 'Feature',
      properties: { nm, kind: 'label', label: `${nm} NM` },
      geometry: { type: 'Point', coordinates: destinationPoint(centerLat, centerLon, radiusM, 0) },
    })
  }
  return { type: 'FeatureCollection', features }
}
