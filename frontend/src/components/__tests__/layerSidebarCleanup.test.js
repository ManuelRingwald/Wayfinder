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
