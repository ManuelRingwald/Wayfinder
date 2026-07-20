// K1 (#309, Epic #307): the "Kartendaten" admin area groups the four map-data
// sources with a read-only status view; the Aeronautik tab embeds the existing
// global-OpenAIP panel (which used to be its own admin section). Source-level
// guards (house convention — no Vuetify mount for admin panels).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const src = (rel) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
const mapData = src('../admin/AdminMapData.vue')
const adminView = src('../../views/AdminView.vue')

describe('AdminView wires the new "Kartendaten" section (K1)', () => {
  it('replaces the standalone OpenAIP section with Kartendaten → AdminMapData', () => {
    // The section value + label changed; the component is swapped.
    expect(adminView).toContain('value="mapdata"')
    expect(adminView).toContain('Kartendaten')
    expect(adminView).toContain("section === 'mapdata'")
    expect(adminView).toContain('import AdminMapData from')
    // The old top-level OpenAIP section is gone (it moved into AdminMapData).
    expect(adminView).not.toContain("section === 'openaip'")
    expect(adminView).not.toContain('value="openaip"')
  })

  it('keeps the section in the mobile select list', () => {
    expect(adminView).toMatch(/value: 'mapdata', title: 'Kartendaten'/)
  })
})

describe('AdminMapData: four sources, status view, OpenAIP embedded', () => {
  it('has the four map-data tabs', () => {
    for (const t of ['Basiskarte', 'Wetter', 'Radar-Abdeckung', 'Aeronautik']) {
      expect(mapData).toContain(`>${t}</v-tab>`)
    }
  })

  it('reads the SAME /api/map-config the ASD reads (single source of truth)', () => {
    expect(mapData).toContain("apiFetch('/api/map-config')")
    // status is derived from the availability flags the backend already exposes
    expect(mapData).toContain('weather_radar_available')
    expect(mapData).toContain('weather_warnings_available')
    expect(mapData).toContain('qnh_available')
    expect(mapData).toContain('coverage_sensor_count')
    expect(mapData).toContain('cfg.theme')
    expect(mapData).toContain('cfg.style')
  })

  it('embeds the existing global-OpenAIP panel in the Aeronautik tab (no duplication)', () => {
    expect(mapData).toContain("import AdminGlobalOpenAIP from '@/components/admin/AdminGlobalOpenAIP.vue'")
    expect(mapData).toContain('<AdminGlobalOpenAIP />')
  })

  it('is read-only status in K1 (editing arrives in K2–K5)', () => {
    // No PUT/save wiring yet — the editing endpoints come per subsystem.
    expect(mapData).not.toContain("method: 'PUT'")
  })
})
