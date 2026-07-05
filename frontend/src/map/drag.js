// setupLabelDrag wires ASD-002 B2 Drag&Drop label pinning.
//   mousedown on a label → disable map pan, begin drag
//   mousemove            → update labelPins and re-render in real time
//   mouseup              → commit pin, re-enable map pan
//   dblclick on label    → delete pin, revert to auto-deconflicted placement
//
// Parameters:
//   map        — MapLibre Map instance
//   state      — engine runtime state (liveTrackFeatures, fadingTracks, labelPins)
//   onRender   — callback to trigger renderSources() on pin change
import { LABELS_LAYER_ID } from './constants.js'

export function setupLabelDrag(map, state, onRender) {
  let drag = null

  map.on('mouseenter', LABELS_LAYER_ID, () => {
    if (!drag) map.getCanvas().style.cursor = 'move'
  })
  map.on('mouseleave', LABELS_LAYER_ID, () => {
    if (!drag) map.getCanvas().style.cursor = ''
  })

  map.on('mousedown', LABELS_LAYER_ID, (e) => {
    e.preventDefault()
    const feat = (map.queryRenderedFeatures(e.point, { layers: [LABELS_LAYER_ID] }) || [])[0]
    if (!feat) return
    const trackNum = feat.properties.track_num

    // Find the track's SYMBOL position (geo), not the label's position.
    const trackFeature =
      state.liveTrackFeatures.find((f) => f.properties.track_num === trackNum) ||
      (() => {
        const fd = state.fadingTracks.get(trackNum)
        return fd ? { geometry: { coordinates: [fd.track.longitude, fd.track.latitude] } } : null
      })()
    if (!trackFeature) return

    const [lon, lat] = trackFeature.geometry.coordinates
    const sym = map.project([lon, lat])

    // Current label screen position — the label feature carries the position
    // it was actually rendered at (auto-placed or previously pinned), set by
    // deconflictLabels(). Using this (instead of the symbol position) is what
    // keeps the grabbed point under the cursor for not-yet-pinned labels.
    const labelPos = map.project(feat.geometry.coordinates)

    // Offset from the label's origin to the exact point grabbed by the mouse,
    // so that point — not the label's origin — tracks the cursor while dragging.
    const grab = { x: e.point.x - labelPos.x, y: e.point.y - labelPos.y }

    // If already pinned, keep the existing offset as the drag start point;
    // otherwise derive it from the current auto-placed label position.
    const startPin = state.labelPins.get(trackNum) ?? {
      dx: labelPos.x - sym.x,
      dy: labelPos.y - sym.y,
    }

    drag = {
      trackNum,
      sym,
      grab,
      startPin,
    }
    map.dragPan.disable()

    const onMove = (moveE) => {
      // Re-derive the pin offset so the grabbed point (not the label origin)
      // stays exactly under the cursor — no snap on the first move.
      const dx = (moveE.point.x - drag.grab.x) - drag.sym.x
      const dy = (moveE.point.y - drag.grab.y) - drag.sym.y
      state.labelPins.set(drag.trackNum, { dx, dy })
      onRender()
    }

    const onUp = () => {
      drag = null
      map.dragPan.enable()
      map.getCanvas().style.cursor = ''
      map.off('mousemove', onMove)
      map.off('mouseup', onUp)
    }

    map.on('mousemove', onMove)
    map.on('mouseup', onUp)
  })

  // Double-click clears the pin and returns the label to auto-placement.
  map.on('dblclick', LABELS_LAYER_ID, (e) => {
    e.preventDefault()
    const feat = (map.queryRenderedFeatures(e.point, { layers: [LABELS_LAYER_ID] }) || [])[0]
    if (!feat) return
    state.labelPins.delete(feat.properties.track_num)
    onRender()
  })
}
