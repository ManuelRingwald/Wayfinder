// Label deconfliction for the ASD scope (ASD-002 B1).
// map and labelPins are passed explicitly — no global state access.
import {
  LABEL_SLOTS,
  LABEL_SLOT_RADIUS_PX,
  LABEL_W_PX,
  LABEL_H_PX,
  SYMBOL_BBOX_R_PX,
  LEADER_THRESHOLD_PX,
} from './constants.js'

// bboxCollides returns true when bbox overlaps any rectangle in occupied.
export function bboxCollides(occupied, bbox) {
  for (const o of occupied) {
    if (bbox.x1 < o.x2 && bbox.x2 > o.x1 && bbox.y1 < o.y2 && bbox.y2 > o.y1) {
      return true
    }
  }
  return false
}

// deconflictLabels computes deconflicted screen-space positions for all track
// data-block labels (ASD-002 B1) and returns GeoJSON features for both the
// labels and their leader lines.
//
// Algorithm — greedy in track_num order (deterministic):
//   For each track, try LABEL_SLOTS in order and pick the first slot whose
//   bounding box does not overlap already-placed labels or OTHER tracks' symbols.
//   The current track's own symbol is intentionally excluded from the collision
//   check so a label can sit right next to its dot — the leader line makes the
//   symbol↔block association explicit. If all 8 slots collide, slot 0 is used
//   as a guaranteed fallback: no label is ever hidden.
//
// Manual pins from labelPins (ASD-002 B2) override auto-placement.
//
// Label positioning: labels are kept at the track's geo-position and the
// screen-space pixel offset is converted to em units stored in the
// "text_offset" property. MapLibre's "text-offset" layout property picks this
// up data-driven (["get","text_offset"]). This avoids a map.unproject() call
// in the hot path, which was found to produce silent errors in certain
// MapLibre GL JS v4 build/camera combinations. Leader line end-points use a
// Mercator approximation for the same reason.
export function deconflictLabels(allTrackFeatures, map, labelPins) {
  const symbolOccupied = [] // circle footprints of already-processed tracks
  const labelOccupied = []  // bounding boxes of already-placed labels

  const sorted = [...allTrackFeatures].sort(
    (a, b) => a.properties.track_num - b.properties.track_num,
  )

  const labelFeatures = []
  const leaderLineFeatures = []

  // Mercator scale: pixels per geographic degree at this zoom level.
  // Used to convert pixel offsets to lng/lat deltas for leader-line endpoints.
  // Formula: pixelsPerDeg = (tileSize * 2^zoom) / 360 — valid for Web Mercator.
  const zoom = map.getZoom()
  const pixelsPerDeg = (256 * Math.pow(2, zoom)) / 360

  for (const feature of sorted) {
    const [lon, lat] = feature.geometry.coordinates
    const trackNum = feature.properties.track_num
    const sym = map.project([lon, lat])

    // sym may be undefined / lack .x/.y for tracks outside the valid projection
    // range. Skip the track rather than propagating NaN into GeoJSON features.
    if (!sym || typeof sym.x !== 'number' || isNaN(sym.x)) continue

    let dx, dy // pixel offsets from symbol centre to label anchor

    if (labelPins && labelPins.has(trackNum)) {
      // B2: manual pin overrides auto-placement.
      const pin = labelPins.get(trackNum)
      dx = pin.dx
      dy = pin.dy
    } else {
      // B1: greedy slot search, excluding own symbol from collision set.
      dx = null
      for (const [ux, uy] of LABEL_SLOTS) {
        const cx = sym.x + ux * LABEL_SLOT_RADIUS_PX
        const cy = sym.y + uy * LABEL_SLOT_RADIUS_PX
        const bbox = {
          x1: cx - LABEL_W_PX / 2,
          y1: cy - LABEL_H_PX / 2,
          x2: cx + LABEL_W_PX / 2,
          y2: cy + LABEL_H_PX / 2,
        }
        if (!bboxCollides(symbolOccupied, bbox) && !bboxCollides(labelOccupied, bbox)) {
          dx = ux * LABEL_SLOT_RADIUS_PX
          dy = uy * LABEL_SLOT_RADIUS_PX
          break
        }
      }
      // Fallback: slot 0, even if colliding — a label is never suppressed.
      if (dx === null) {
        dx = LABEL_SLOTS[0][0] * LABEL_SLOT_RADIUS_PX
        dy = LABEL_SLOTS[0][1] * LABEL_SLOT_RADIUS_PX
      }
    }

    const lx = sym.x + dx
    const ly = sym.y + dy

    // Register this track's symbol and placed label for subsequent iterations.
    symbolOccupied.push({
      x1: sym.x - SYMBOL_BBOX_R_PX,
      y1: sym.y - SYMBOL_BBOX_R_PX,
      x2: sym.x + SYMBOL_BBOX_R_PX,
      y2: sym.y + SYMBOL_BBOX_R_PX,
    })
    labelOccupied.push({
      x1: lx - LABEL_W_PX / 2,
      y1: ly - LABEL_H_PX / 2,
      x2: lx + LABEL_W_PX / 2,
      y2: ly + LABEL_H_PX / 2,
    })

    // Convert the screen-space pixel offset (dx, dy) to a geo-position via a
    // Web-Mercator approximation, then place the label point THERE. This keeps
    // the label at a normal centred anchor (which renders reliably) instead of
    // relying on a data-driven text-offset, which MapLibre GL JS v4 did not
    // apply (labels stayed invisible while leader lines drew). The same
    // labelLon/labelLat is reused as the leader-line endpoint, so symbol,
    // line and block stay perfectly consistent. Error at <30 px offset is
    // sub-metre — imperceptible at ASD zoom levels.
    const latRad = lat * Math.PI / 180
    const labelLon = lon + dx / pixelsPerDeg
    // Mercator: dy_pixel → dlat uses cos(lat) scaling (north-up, y positive down).
    const labelLat = lat - dy * Math.cos(latRad) / pixelsPerDeg

    // Carry opacity side-car properties so label paint expressions work.
    const opProps = {}
    if (feature.properties.fade_opacity !== undefined) opProps.fade_opacity = feature.properties.fade_opacity
    if (feature.properties.fl_opacity !== undefined) opProps.fl_opacity = feature.properties.fl_opacity

    labelFeatures.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [labelLon, labelLat] },
      properties: {
        track_num: trackNum,
        label: feature.properties.label,
        coasting: feature.properties.coasting,
        ...opProps,
      },
    })

    // Leader line: drawn when label is visibly offset from its symbol, to make
    // the symbol↔block association unambiguous in dense traffic (ATC convention).
    if (Math.hypot(dx, dy) > LEADER_THRESHOLD_PX) {
      leaderLineFeatures.push({
        type: 'Feature',
        geometry: {
          type: 'LineString',
          coordinates: [
            [lon, lat],
            [labelLon, labelLat],
          ],
        },
        properties: {
          track_num: trackNum,
          coasting: feature.properties.coasting,
          ...opProps,
        },
      })
    }
  }

  return { labelFeatures, leaderLineFeatures }
}
