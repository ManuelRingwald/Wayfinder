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
  it('MapControls no longer owns zoom (only recenter)', () => {
    expect(mapControls).toContain("defineEmits(['recenter'])")
    expect(mapControls).not.toContain('zoom-in')
  })

  it('MapCanvas exposes zoomIn/zoomOut and AsdView delegates the rail events to it', () => {
    expect(mapCanvas).toContain('zoomIn:')
    expect(mapCanvas).toContain('zoomOut:')
    expect(asdView).toContain('@zoom-in="mapCanvas?.zoomIn()"')
    expect(asdView).toContain('@zoom-out="mapCanvas?.zoomOut()"')
  })
})
