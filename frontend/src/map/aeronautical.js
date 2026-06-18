// loadAeronautical pulls the cached GeoJSON for each overlay and pushes it into
// the matching source. Failures are non-fatal: an empty/unreachable endpoint
// simply leaves that overlay unchanged (graceful degradation, ADR 0004).
import {
  AIRSPACE_SOURCE_ID,
  NAVAIDS_SOURCE_ID,
  WAYPOINTS_SOURCE_ID,
  AERO_REFRESH_MS,
} from './constants.js'

export async function loadAeronautical(map) {
  const sources = [
    ['/api/airspace', AIRSPACE_SOURCE_ID],
    ['/api/navaids', NAVAIDS_SOURCE_ID],
    ['/api/waypoints', WAYPOINTS_SOURCE_ID],
  ]
  await Promise.all(
    sources.map(async ([url, sourceId]) => {
      try {
        const res = await fetch(url)
        if (!res.ok) {
          return
        }
        const data = await res.json()
        const src = map.getSource(sourceId)
        if (src) {
          src.setData(data)
        }
      } catch (err) {
        console.warn('aeronautical load failed for', url, err)
      }
    }),
  )
}

// startAeronauticalRefresh loads aeronautical data immediately and then
// schedules a periodic refresh. Returns the interval handle so the caller
// can clear it on destroy.
export function startAeronauticalRefresh(map) {
  loadAeronautical(map)
  return setInterval(() => loadAeronautical(map), AERO_REFRESH_MS)
}
