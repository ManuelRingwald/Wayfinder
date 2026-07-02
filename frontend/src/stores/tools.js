import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// Controller measurement tools (Häppchen 4). activeTool drives the map's measure
// controller (map/measure.js) and the toolbar's active state; readout carries the
// live distance/bearing text the controller reports back, and readoutAt its
// screen anchor ({x, y} in map-canvas pixels) so the label floats at the line
// (map/measure.js reprojects it on drag and on map move).
// PROBE is intentionally NOT here yet — its content is undefined (see plan).
export const useToolsStore = defineStore('tools', () => {
  const activeTool = ref(null) // null | 'rbl' | 'dist' | 'qdm'
  const readout = ref(null) // e.g. "12.3 NM · 087°", or null
  const readoutAt = ref(null) // { x, y } screen anchor for the floating label, or null

  // selectTool toggles: picking the active tool again turns measuring off.
  function selectTool(kind) {
    activeTool.value = activeTool.value === kind ? null : kind
    readout.value = null
    readoutAt.value = null
  }
  function clearTool() {
    activeTool.value = null
    readout.value = null
    readoutAt.value = null
  }
  function setReadout(text, at = null) {
    readout.value = text
    readoutAt.value = at
  }

  // hint: the one-line instruction shown while a tool is active.
  const hint = computed(() => {
    switch (activeTool.value) {
      case 'rbl': return 'RBL — auf der Karte ziehen · Esc beendet'
      case 'dist': return 'DIST — zwei Tracks nacheinander anklicken · Esc beendet'
      case 'qdm': return 'QDM — erst Track, dann Zielpunkt anklicken · Esc beendet'
      default: return null
    }
  })

  return { activeTool, readout, readoutAt, hint, selectTool, clearTool, setReadout }
})
