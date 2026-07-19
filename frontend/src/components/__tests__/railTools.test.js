// Regression guard for Häppchen 3: the measurement tools (RBL/DIST/QDM) and zoom
// moved from the floating map toolbar into the navigation rail (design mockup).
// These are wiring facts across several SFCs; the repo has no component-mount
// infra, so we assert against the raw source (same approach as the other UI tests).
import { describe, it, expect } from 'vitest'
import rail from '../NavigationRail.vue?raw'
import mapControls from '../MapControls.vue?raw'
import mapCanvas from '../MapCanvas.vue?raw'
import measureStatus from '../MeasureStatus.vue?raw'
import asdView from '../../views/AsdView.vue?raw'

describe('measurement tools live in the navigation rail', () => {
  it('the rail lists RBL/DIST/QDM and toggles them via the tools store', () => {
    expect(rail).toContain('useToolsStore')
    for (const id of ["'rbl'", "'dist'", "'qdm'"]) expect(rail).toContain(id)
    expect(rail).toContain('tools.selectTool(t.id)')
  })

  it('the rail hosts zoom and emits it (delegated to the map engine)', () => {
    expect(rail).toContain("emit('zoom-in')")
    expect(rail).toContain("emit('zoom-out')")
    expect(rail).toContain("'zoom-in'")
    expect(rail).toContain("'zoom-out'")
  })
})

describe('the floating measure toolbar is gone; status stays over the map', () => {
  it('MapCanvas mounts MeasureStatus, not the old MeasureToolbar', () => {
    expect(mapCanvas).toContain('MeasureStatus')
    expect(mapCanvas).not.toContain('MeasureToolbar')
  })

  it('MeasureStatus keeps the hint/readout and the R/D/Q/Esc shortcuts', () => {
    expect(measureStatus).toContain('measure-hint')
    expect(measureStatus).toContain("store.selectTool('rbl')")
    expect(measureStatus).toContain('escape')
  })
})

describe('zoom delegation is wired end to end', () => {
  // #194: the rail hosts zoom on desktop; on phones/tablet-portrait the rail is
  // not rendered, so MapControls carries zoom (gated to !mdAndUp) and MapCanvas
  // wires it to the engine — the rail path stays intact for desktop.
  it('MapControls hosts zoom on mobile only (gated by !mdAndUp in MapCanvas)', () => {
    expect(mapControls).toContain("defineEmits(['recenter', 'zoom-in', 'zoom-out'])")
    expect(mapControls).toContain("$emit('zoom-in')")
    // ASD-018: the mobile gate moved to MapCanvas — it renders MapControls only
    // for !mdAndUp; on desktop the viewport controls live in AsdView's rail.
    expect(mapCanvas).toContain('useDisplay')
    expect(mapCanvas).toMatch(/<MapControls\s+v-if="!mdAndUp"/)
  })

  it('MapCanvas exposes zoomIn/zoomOut and wires both the rail and the map controls', () => {
    expect(mapCanvas).toContain('zoomIn:')
    expect(mapCanvas).toContain('zoomOut:')
    // Rail path (desktop) delegated from AsdView.
    expect(asdView).toContain('@zoom-in="mapCanvas?.zoomIn()"')
    expect(asdView).toContain('@zoom-out="mapCanvas?.zoomOut()"')
    // Map-controls path (mobile) delegated from MapCanvas.
    expect(mapCanvas).toContain('@zoom-in="mapEngine?.zoomIn()"')
    expect(mapCanvas).toContain('@zoom-out="mapEngine?.zoomOut()"')
  })
})
