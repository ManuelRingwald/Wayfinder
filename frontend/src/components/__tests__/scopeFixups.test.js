// Regression guards for the E2E scope fix-ups:
//  - RBL/DIST/QDM were dead because the measure controller was created before the
//    map style loaded (addSource threw); it must be deferred to 'load';
//  - the OSM/CARTO attribution is compact (collapsed) so it no longer overlaps
//    the bottom-right readout;
//  - the rail carries a top brand glyph and horizontal group dividers.
// Source-level assertions (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import mapCanvas from '../MapCanvas.vue?raw'
import engine from '../../map/engine.js?raw'
import rail from '../NavigationRail.vue?raw'

describe('measure tools are created after the style loads (RBL fix)', () => {
  it('MapCanvas defers createMeasure to the map load event', () => {
    expect(mapCanvas).toContain('createMeasure')
    expect(mapCanvas).toContain('map.loaded()')
    expect(mapCanvas).toContain("map.once('load'")
    // a tool selected before load is still applied once the controller exists
    expect(mapCanvas).toContain('measure.setTool(tools.activeTool)')
  })
})

describe('map attribution is compact (does not overlap the readout)', () => {
  it('the default expanded attribution is off; a compact one is added', () => {
    expect(engine).toContain('attributionControl: false')
    expect(engine).toContain('AttributionControl({ compact: true })')
  })
})

describe('rail brand glyph + group dividers', () => {
  it('the rail has a top brand glyph', () => {
    expect(rail).toContain('nav-rail__brand')
    expect(rail).toContain('mdi-radar')
  })

  it('the rail separates its groups with horizontal dividers', () => {
    const dividers = rail.match(/nav-rail__divider/g) || []
    // one class def + at least three separators (after brand, tools, sections)
    expect(dividers.length).toBeGreaterThanOrEqual(4)
  })
})
