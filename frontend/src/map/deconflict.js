// Label deconfliction for the ASD scope (ASD-002 B1).
// map and labelPins are passed explicitly — no global state access.
import {
  LABEL_SLOTS,
  LABEL_SLOT_RADIUS_PX,
  LABEL_W_PX,
  LABEL_H_PX,
  SYMBOL_BBOX_R_PX,
  LEADER_THRESHOLD_PX,
  SELECTION_LABEL_PAD_PX,
  SELECTION_LABEL_RADIUS_PX,
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

// roundedRectRing returns the screen-space points of a rounded-rectangle outline
// centred on (cx, cy), half-extents (halfW, halfH), corner radius r. Each corner
// is approximated by `segsPerCorner` arc segments; the ring is closed (last point
// equals the first). Pure (no map), so it is unit-testable; the caller
// inverse-projects the points to geo. Screen convention: x right, y down.
export function roundedRectRing(cx, cy, halfW, halfH, r, segsPerCorner = 4) {
  const rr = Math.max(0, Math.min(r, halfW, halfH))
  // Corner arc centres and sweep, ordered TR → BR → BL → TL (clockwise, y-down).
  const corners = [
    { x: cx + halfW - rr, y: cy - halfH + rr, a0: -Math.PI / 2, a1: 0 },
    { x: cx + halfW - rr, y: cy + halfH - rr, a0: 0, a1: Math.PI / 2 },
    { x: cx - halfW + rr, y: cy + halfH - rr, a0: Math.PI / 2, a1: Math.PI },
    { x: cx - halfW + rr, y: cy - halfH + rr, a0: Math.PI, a1: (3 * Math.PI) / 2 },
  ]
  const pts = []
  for (const c of corners) {
    for (let i = 0; i <= segsPerCorner; i++) {
      const a = c.a0 + (c.a1 - c.a0) * (i / segsPerCorner)
      pts.push({ x: c.x + rr * Math.cos(a), y: c.y + rr * Math.sin(a) })
    }
  }
  pts.push({ ...pts[0] }) // close the ring
  return pts
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
// Label positioning: each label's screen-space anchor (symbol + pixel offset) is
// inverse-projected back to geo with map.unproject() and the label is placed
// THERE with a centred anchor. Using the map's own inverse guarantees the
// round-trip map.project(labelGeo) === (anchor px) for any tile size/zoom/
// latitude, which is what keeps the drag handler (drag.js) pixel-exact.
//
// selectedTrackNum (ASD-011b): when set, the selected track's label additionally
// gets a rounded-rectangle outline box (returned as selectionBoxFeatures), so the
// selection reads on the data block as well as the symbol.
export function deconflictLabels(allTrackFeatures, map, labelPins, selectedTrackNum = null) {
  const symbolOccupied = [] // circle footprints of already-processed tracks
  const labelOccupied = []  // bounding boxes of already-placed labels

  const sorted = [...allTrackFeatures].sort(
    (a, b) => a.properties.track_num - b.properties.track_num,
  )

  const labelFeatures = []
  const leaderLineFeatures = []
  const selectionBoxFeatures = [] // ASD-011b: 0 or 1 (the selected label's outline)

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

    // Place the label at the EXACT screen pixel (lx, ly) by inverse-projecting
    // it back to geo with map.unproject(). Using the map's own inverse (instead
    // of a hand-rolled Web-Mercator formula) guarantees the round-trip
    // map.project([labelLon, labelLat]) === (lx, ly) for any tile size, zoom and
    // latitude. This is what keeps the label at a normal centred anchor (which
    // renders reliably, unlike the data-driven text-offset MapLibre GL v4 did
    // not apply) AND lets the drag handler (drag.js) reason in exact pixels — a
    // hand-rolled 256-tile formula placed the label at ~2× the intended offset
    // against MapLibre's 512-px world, which made a grabbed label jump on the
    // first drag move and then track the cursor with a constant offset. The same
    // labelLon/labelLat is reused as the leader-line endpoint, so symbol, line
    // and block stay perfectly consistent.
    const labelLngLat = map.unproject([lx, ly])
    const labelLon = labelLngLat.lng
    const labelLat = labelLngLat.lat

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

    // ASD-011b: outline the SELECTED track's label with a rounded-rectangle box.
    // Built around the label's screen bbox (centre lx,ly) with padding, then each
    // ring point is inverse-projected so the box sits exactly around the label.
    if (selectedTrackNum != null && trackNum === selectedTrackNum) {
      const halfW = LABEL_W_PX / 2 + SELECTION_LABEL_PAD_PX
      const halfH = LABEL_H_PX / 2 + SELECTION_LABEL_PAD_PX
      const ring = roundedRectRing(lx, ly, halfW, halfH, SELECTION_LABEL_RADIUS_PX)
      selectionBoxFeatures.push({
        type: 'Feature',
        geometry: {
          type: 'LineString',
          coordinates: ring.map((p) => {
            const ll = map.unproject([p.x, p.y])
            return [ll.lng, ll.lat]
          }),
        },
        properties: { track_num: trackNum, ...opProps },
      })
    }
  }

  return { labelFeatures, leaderLineFeatures, selectionBoxFeatures }
}
