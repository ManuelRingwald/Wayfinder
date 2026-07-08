// Regression guard for the sidebar cleanup (#176): the standalone "Lufträume"
// parent toggle is gone, the four airspace groups are first-class toggles wired
// to the store's derived visibility, the per-group colours/dots are removed
// (uniform primary), every section header is underlined, and the rail↔panel
// divider is made visible. Source-level assertions (project convention — no
// Vuetify mount).
import { describe, it, expect } from 'vitest'
import lfc from '../LayerFilterContent.vue?raw'
import rail from '../NavigationRail.vue?raw'

describe('sidebar cleanup (#176)', () => {
  it('drops the standalone "Lufträume" parent toggle', () => {
    expect(lfc).not.toContain('label="Lufträume"')
    expect(lfc).not.toContain('v-model="store.layerVisibility.airspace"')
  })

  it('wires the airspace groups to the store-derived visibility', () => {
    expect(lfc).toContain('onAirspaceGroup(group.id, $event)')
    expect(lfc).toContain('store.setAirspaceGroup')
  })

  it('removes the per-group colours and dots (uniform primary toggles)', () => {
    expect(lfc).not.toContain('airspace-dot')
    expect(lfc).not.toContain(':color="group.color"')
  })

  it('underlines every section header (not just the spaced variant)', () => {
    expect(lfc).toMatch(/\.filter-section-header\s*\{[\s\S]*?border-bottom/)
  })

  it('makes the rail↔panel divider a reliably-rendered, visible hairline', () => {
    expect(rail).toContain('nav-panel__divider')
    // A plain full-height strip (not the vertical v-divider that failed to
    // stretch) using the border token → a subtle but clearly visible line.
    expect(rail).toMatch(/\.nav-panel__divider\s*\{[\s\S]*?align-self:\s*stretch/)
    expect(rail).toMatch(/\.nav-panel__divider\s*\{[\s\S]*?background:\s*var\(--wf-border-strong\)/)
  })
})

// Operator request 2026-07-08: opening/closing the sidebar must not reflow the
// panel text ("Schrift baut sich auf / wird zusammengedrückt") nor flash a
// scrollbar. Fix: the panel has a FIXED width (open drawer width − rail) so its
// content is laid out at final width and the drawer just clips/reveals it.
describe('sidebar open/close does not reflow the panel (2026-07-08)', () => {
  it('gives the nav panel a fixed width instead of flex:1', () => {
    // Fixed width = open drawer width (248 desktop) minus the rail token.
    expect(rail).toMatch(/\.nav-panel\s*\{[\s\S]*?width:\s*calc\(248px - var\(--wf-nav-rail-width/)
    expect(rail).toMatch(/\.nav-panel\s*\{[\s\S]*?flex-shrink:\s*0/)
    // The tablet-landscape band uses the wider 304px open drawer.
    expect(rail).toMatch(/width:\s*calc\(304px - var\(--wf-nav-rail-width, 76px\)\)/)
  })
  it('suppresses the transient horizontal scrollbar on the panel body', () => {
    expect(rail).toMatch(/\.nav-panel__body\s*\{[\s\S]*?overflow-x:\s*hidden/)
  })
})
