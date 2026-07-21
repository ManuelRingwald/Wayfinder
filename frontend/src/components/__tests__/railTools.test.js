// ASD-019 (ADR 0030): the navigation rail is GROUPED into a MEASURE and a MAP
// section with two colour-coded active states (amber for armed tools, cyan for
// open panels), and zoom moved OFF the rail onto the bottom-right of the scope
// (MapControls, now rendered on desktop + mobile). These are wiring facts across
// several SFCs; the repo has no component-mount infra, so we assert against the
// raw source (same approach as the other UI tests).
import { describe, it, expect } from 'vitest'
import rail from '../NavigationRail.vue?raw'
import mapControls from '../MapControls.vue?raw'
import zoomControls from '../ZoomControls.vue?raw'
import mapCanvas from '../MapCanvas.vue?raw'
import measureStatus from '../MeasureStatus.vue?raw'
import asdView from '../../views/AsdView.vue?raw'

describe('the navigation rail groups its tools (ASD-019)', () => {
  it('the rail lists RBL/DIST/QDM and toggles them via the tools store', () => {
    expect(rail).toContain('useToolsStore')
    for (const id of ["'rbl'", "'dist'", "'qdm'"]) expect(rail).toContain(id)
    expect(rail).toContain('tools.selectTool(t.id)')
  })

  it('tools sit under a MEASURE micro-label, panels under a MAP micro-label', () => {
    expect(rail).toContain('nav-rail__section')
    expect(rail).toContain('>MEASURE<')
    expect(rail).toContain('>MAP<')
    // The two families carry distinct group classes for the colour code.
    expect(rail).toContain('nav-rail__btn--tool')
    expect(rail).toContain('nav-rail__btn--panel')
  })

  it('the two active states are colour-coded amber (tools) / cyan (panels) with a glow', () => {
    // Tools override the active colour to warning-amber; panels keep primary cyan.
    expect(rail).toContain('.nav-rail__btn--tool.nav-rail__btn--active')
    expect(rail).toContain('var(--wf-warning)')
    expect(rail).toContain('var(--wf-state-armed)')
    // Soft halos behind the active icon ("aktiv leuchtende Symbole").
    expect(rail).toContain('var(--wf-glow-armed)')
    expect(rail).toContain('var(--wf-glow-selected)')
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

describe('zoom lives in the bottom-right map controls, not the rail (ASD-019)', () => {
  it('the rail no longer hosts or emits zoom', () => {
    expect(rail).not.toContain("emit('zoom-in')")
    expect(rail).not.toContain("emit('zoom-out')")
    expect(rail).not.toContain('mdi-magnify-plus-outline')
    expect(rail).not.toContain('mdi-magnify-minus-outline')
  })

  it('AsdView no longer delegates zoom to the rail', () => {
    expect(asdView).not.toContain('@zoom-in="mapCanvas?.zoomIn()"')
    expect(asdView).not.toContain('@zoom-out="mapCanvas?.zoomOut()"')
  })

  it('ZoomControls is a position-neutral +/- group that emits zoom-in/zoom-out', () => {
    expect(zoomControls).toContain("defineEmits(['zoom-in', 'zoom-out'])")
    expect(zoomControls).toContain("$emit('zoom-in')")
    expect(zoomControls).toContain("$emit('zoom-out')")
    // Position-neutral: no absolute offset of its own — its zone places it.
    expect(zoomControls).not.toMatch(/position:\s*absolute/)
  })

  it('MapControls hosts ZoomControls and renders on desktop + mobile', () => {
    expect(mapControls).toContain("import ZoomControls from './ZoomControls.vue'")
    expect(mapControls).toContain('<ZoomControls')
    // No longer mobile-gated in MapCanvas — it renders on both now.
    expect(mapCanvas).not.toMatch(/<MapControls\s+v-if="!mdAndUp"/)
  })

  it('MapCanvas wires the map controls zoom straight to the engine', () => {
    expect(mapCanvas).toContain('@zoom-in="mapEngine?.zoomIn()"')
    expect(mapCanvas).toContain('@zoom-out="mapEngine?.zoomOut()"')
  })
})

describe('#296: measurement tools carry an explaining tooltip', () => {
  it('each tool defines a one-line description', () => {
    expect(rail).toContain('description:')
    expect(rail).toContain('Range/Bearing Line') // RBL
    expect(rail).toContain('zwischen zwei Tracks') // DIST
    expect(rail).toContain('rechtweisend') // QDM
  })

  it('renders the description as a hover/focus tooltip on the tool buttons', () => {
    // House style: a parent-activated v-tooltip bound to the tool description.
    expect(rail).toContain('activator="parent"')
    expect(rail).toContain(':text="t.description"')
  })
})

describe('#318: a MAP rail icon glows blue when its section has active elements', () => {
  it('derives an "engaged" state per section from the ASD store', () => {
    expect(rail).toContain("import { useAsdStore } from '@/stores/asd.js'")
    expect(rail).toContain('hasActiveLayers')
    expect(rail).toContain('hasActiveFilter')
    expect(rail).toContain('function sectionEngaged(')
  })

  it('binds the engaged glow class on the panel buttons, independent of the open state', () => {
    // Both classes coexist: --active (panel open) and --engaged (section active).
    expect(rail).toContain("'nav-rail__btn--engaged': sectionEngaged(s.id)")
  })

  it('the engaged glow reuses the cyan selected-glow (blue), not the amber armed glow', () => {
    expect(rail).toContain('.nav-rail__btn--engaged .nav-rail__pill')
    expect(rail).toMatch(/nav-rail__btn--engaged[\s\S]*?var\(--wf-glow-selected\)/)
  })
})
