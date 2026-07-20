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
  })

  it('embeds the existing global-OpenAIP panel in the Aeronautik tab (no duplication)', () => {
    expect(mapData).toContain("import AdminGlobalOpenAIP from '@/components/admin/AdminGlobalOpenAIP.vue'")
    expect(mapData).toContain('<AdminGlobalOpenAIP />')
  })

})

describe('AdminMapData: Basiskarte live editing (K2 #310)', () => {
  it('loads + saves the base-map settings via the mapconfig admin endpoints', () => {
    expect(mapData).toContain("apiFetch('/api/admin/mapdata/basemap/theme')")
    expect(mapData).toContain("apiFetch('/api/admin/mapdata/basemap/style-url')")
    expect(mapData).toContain("method: 'PUT'")
    expect(mapData).toContain('saveTheme')
    expect(mapData).toContain('saveStyle')
    expect(mapData).toContain('resetStyle') // empty value resets to env default
  })

  it('surfaces a reload error honestly (stored but not applied)', () => {
    expect(mapData).toContain('reload_error')
    expect(mapData).toContain('basemap.value.reloadError')
  })
})
