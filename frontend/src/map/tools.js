// Great-circle geometry for the controller measurement tools (Häppchen 4):
//   RBL  — range/bearing line dragged on the map (A → B)
//   DIST — distance/bearing between two picked tracks
//   QDM  — bearing from a picked track to a picked point
//
// Inputs are {lng, lat} in degrees (MapLibre LngLat-shaped). Distances are
// nautical miles, bearings degrees true in [0,360). Pure functions — no MapLibre
// dependency, so they are unit-tested directly (map/__tests__/tools.test.js).

// Mean Earth radius in nautical miles (6371 km / 1.852).
export const EARTH_RADIUS_NM = 3440.065

const toRad = (d) => (d * Math.PI) / 180
const toDeg = (r) => (r * 180) / Math.PI

// haversineNM returns the great-circle distance between two lng/lat points in
// nautical miles. Adequate for display measurement at ASD ranges.
export function haversineNM(a, b) {
  const dLat = toRad(b.lat - a.lat)
  const dLon = toRad(b.lng - a.lng)
  const la1 = toRad(a.lat)
  const la2 = toRad(b.lat)
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(la1) * Math.cos(la2) * Math.sin(dLon / 2) ** 2
  return 2 * EARTH_RADIUS_NM * Math.asin(Math.min(1, Math.sqrt(h)))
}

// bearingDeg returns the initial true bearing from `from` to `to`, in [0,360).
export function bearingDeg(from, to) {
  const la1 = toRad(from.lat)
  const la2 = toRad(to.lat)
  const dLon = toRad(to.lng - from.lng)
  const y = Math.sin(dLon) * Math.cos(la2)
  const x = Math.cos(la1) * Math.sin(la2) - Math.sin(la1) * Math.cos(la2) * Math.cos(dLon)
  return (toDeg(Math.atan2(y, x)) + 360) % 360
}

// formatNM renders a distance as "12.3 NM".
export function formatNM(nm) {
  return `${nm.toFixed(1)} NM`
}

// formatBearing renders a true bearing as a zero-padded "087°" (360 wraps to 000).
export function formatBearing(deg) {
  return `${String(Math.round(deg) % 360).padStart(3, '0')}°`
}

// measureText is the readout for a two-point measurement (RBL/DIST): distance and
// the true bearing from a to b.
export function measureText(a, b) {
  return `${formatNM(haversineNM(a, b))} · ${formatBearing(bearingDeg(a, b))}`
}
