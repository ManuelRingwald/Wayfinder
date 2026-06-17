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

    // If already pinned, use existing offset as the drag start point.
    const currentPin = state.labelPins.get(trackNum) ?? {
      dx: e.point.x - sym.x,
      dy: e.point.y - sym.y,
    }

    drag = {
      trackNum,
      sym,
      startMouse: { x: e.point.x, y: e.point.y },
      startPin: currentPin,
    }
    map.dragPan.disable()

    const onMove = (moveE) => {
      const dx = drag.startPin.dx + (moveE.point.x - drag.startMouse.x)
      const dy = drag.startPin.dy + (moveE.point.y - drag.startMouse.y)
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
