// Regression guard for the View-Config form's operative captions (Issue #57,
// FR-UI-013). A new administrator must be able to read off *what each setting
// does operationally* — the initial map view, the AOI being a hard server-side
// data-minimisation boundary (not a display preference), the FL unit (× 100 ft)
// and the fail-open behaviour. These are static template strings, so we assert
// directly against the single-file component source rather than mounting the
// full Vuetify/router/store stack for a copy check.
import { describe, it, expect } from 'vitest'
// Vite serves the raw single-file-component source via the `?raw` query, so the
// copy check needs no filesystem access or component mount.
import sfc from '../AdminTenantDetail.vue?raw'

describe('AdminTenantDetail view-config captions (#57)', () => {
  it('explains that center & zoom set the clients’ initial map view', () => {
    expect(sfc).toContain('Zentrum &amp; Zoom')
    expect(sfc).toContain('Start-Kartenausschnitt')
  })

  it('states the AOI is a hard server-side data-minimisation boundary', () => {
    expect(sfc).toContain('harte serverseitige Daten-Minimierungsgrenze')
    expect(sfc).toContain('keine reine')
    expect(sfc).toContain('Anzeigepräferenz')
  })

  it('labels the FL fields with their unit (× 100 ft)', () => {
    expect(sfc).toContain('FL min (× 100 ft)')
    expect(sfc).toContain('FL max (× 100 ft)')
  })

  it('explains the FL unit and the fail-open behaviour', () => {
    expect(sfc).toContain('FL100 =')
    expect(sfc).toContain('Fail-open:')
    expect(sfc).toContain('ohne gemeldete Flugfläche')
  })
})

describe('AdminTenantDetail airport centre search (ICAO)', () => {
  it('wires the airport autocomplete to the search + pick handlers', () => {
    expect(sfc).toContain('v-autocomplete')
    expect(sfc).toContain('onAirportSearch')
    expect(sfc).toContain('onAirportPick')
    expect(sfc).toContain('admin.searchAirports')
  })

  it('a picked airport fills the centre coordinates AND the ICAO fields', () => {
    expect(sfc).toContain('form.centerLat = hit.lat')
    expect(sfc).toContain('form.centerLon = hit.lon')
    expect(sfc).toContain('form.icao = hit.icao')
    expect(sfc).toContain('form.qnhIcao = hit.icao')
  })
})

describe('AdminTenantDetail feature entitlements', () => {
  it('shows the catalogue label (Fachbegriff), falling back to the raw key', () => {
    // The heading must be the operator-facing label from the feature catalogue,
    // not the raw snake_case key; e.label || e.key keeps older servers working.
    expect(sfc).toContain('e.label || e.key')
    expect(sfc).toContain('e.description')
  })
})
