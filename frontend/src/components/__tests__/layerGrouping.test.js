// ASD-020 (sidebar information architecture, ADR 0031): the flat Layer switch
// list is reorganised into four collapsible groups, each a LayerGroup with a
// tri-state master. Source-level guards (project convention — no Vuetify mount):
// they pin the STRUCTURE (which groups exist, that they carry a master wired to
// the group's members, that the master logic stays schema-agnostic) so a later
// edit cannot silently regress the grouping back to a flat list.
import { describe, it, expect } from 'vitest'
import lfc from '../LayerFilterContent.vue?raw'
import group from '../LayerGroup.vue?raw'

describe('LayerGroup component (the binding "Rahmen")', () => {
  it('is collapsible and carries a tri-state master, position-neutral', () => {
    // Chevron collapse + a controlled tri-state checkbox master (indeterminate
    // = mixed). The click is delegated to the parent so the derived state wins.
    expect(group).toMatch(/mdi-chevron-(down|right)/)
    expect(group).toContain('v-checkbox-btn')
    expect(group).toContain(':indeterminate="master === \'mixed\'"')
    expect(group).toContain("$emit('toggle-master')")
  })

  it('hides the master when the group has nothing to control', () => {
    expect(group).toContain("v-if=\"master !== 'empty'\"")
  })
})

describe('Layer section is organised into the four ASD-020 groups', () => {
  const titles = ['Aeronautik', 'Karte', 'Radar & Reichweite', 'Wetter']
  it('renders exactly the four groups, each with a master', () => {
    for (const t of titles) {
      expect(lfc).toContain(`title="${t}"`)
    }
    // Four LayerGroup blocks, each wiring :master and @toggle-master.
    const masters = lfc.match(/:master="\w+State"/g) || []
    expect(masters.length).toBe(4)
    const toggles = lfc.match(/@toggle-master="onGroupMaster\(/g) || []
    expect(toggles.length).toBe(4)
  })

  it('imports LayerGroup and the schema-agnostic tri-state helpers', () => {
    expect(lfc).toContain("import LayerGroup from './LayerGroup.vue'")
    expect(lfc).toContain("import { masterState, nextMaster } from '@/map/layerGroups.js'")
  })

  it('the base map lives in the Karte group (foundation for the BKG element split #290)', () => {
    // The basemap switch sits between the Karte group open tag and the next group.
    const karte = lfc.indexOf('title="Karte"')
    const radar = lfc.indexOf('title="Radar & Reichweite"')
    const basemap = lfc.indexOf("label=\"Basiskarte (BKG)\"")
    expect(karte).toBeGreaterThan(-1)
    expect(basemap).toBeGreaterThan(karte)
    expect(basemap).toBeLessThan(radar)
  })

  it('the master bulk action routes through the same store paths as the rows', () => {
    // onGroupMaster sets each enabled member via its set() — which for airspace
    // members is setAirspaceGroup and for the rest onLayerToggle: no dead toggle.
    expect(lfc).toMatch(/function onGroupMaster\([\s\S]*?m\.set\(target\)/)
    // A disabled toggle (unavailable source) is excluded from the master state
    // and the bulk action, so it never pins the master to "mixed".
    expect(lfc).toContain('enabled: store.coverageAvailable')
    expect(lfc).toContain('enabled: store.weatherRadarAvailable')
    expect(lfc).toContain('enabled: store.weatherWarningsAvailable')
  })
})
