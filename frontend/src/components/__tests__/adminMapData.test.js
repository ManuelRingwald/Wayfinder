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

describe('AdminMapData: Aeronautik radius + base-URL (K5 #313)', () => {
  it('loads + saves the OpenAIP fetch radius and base URL via the mapconfig endpoints', () => {
    expect(mapData).toContain("'/api/admin/mapdata/aero'")
    expect(mapData).toContain("'radius-km'")
    expect(mapData).toContain("'base-url'")
    expect(mapData).toContain('loadAero')
    expect(mapData).toContain('saveAero')
    expect(mapData).toContain('resetAero') // empty value resets to env default
    expect(mapData).toContain('v-model.number="aero.radiusKM"')
    expect(mapData).toContain('v-model="aero.baseURL"')
  })

  it('keeps the API key sealed (managed by the embedded panel, not this form)', () => {
    // The radius/base-url form must not touch the key endpoint.
    expect(mapData).not.toContain('api-key')
    expect(mapData).toContain('versiegelt')
  })

  it('states honestly that radius/base-URL apply only at the next restart', () => {
    expect(mapData).toMatch(/Fetch-Radius[\s\S]*nächsten Neustart/)
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

describe('AdminMapData: Wetter live editing (K3 #311)', () => {
  it('loads + saves the seven weather settings via the mapconfig endpoints', () => {
    expect(mapData).toContain("'/api/admin/mapdata/weather'")
    expect(mapData).toContain('loadWeather')
    expect(mapData).toContain('saveWeather')
    for (const p of ['radar-enabled', 'radar-url', 'radar-layer', 'warn-enabled', 'warn-url', 'warn-layer', 'qnh-enabled']) {
      expect(mapData).toContain(`'${p}'`)
    }
  })

  it('exposes enable switches bound to the weather form', () => {
    expect(mapData).toContain('v-model="weather.radarEnabled"')
    expect(mapData).toContain('v-model="weather.warnEnabled"')
    expect(mapData).toContain('v-model="weather.qnhEnabled"')
    expect(mapData).toContain('v-model="weather.radarURL"')
  })

  it('states honestly that URL/layer changes apply only at the next restart', () => {
    expect(mapData).toContain('nächsten Neustart')
  })
})

describe('AdminMapData: Radar-Abdeckung sensor CRUD (K4 #312)', () => {
  it('loads, edits and saves the sensor list + ring colour', () => {
    expect(mapData).toContain("apiFetch('/api/admin/mapdata/coverage')")
    expect(mapData).toContain('v-for="(s, i) in sensors"')
    expect(mapData).toContain('addSensor')
    expect(mapData).toContain('saveCoverage')
    expect(mapData).toContain('ring_color: ringColor.value')
  })

  it('reset to env default uses DELETE (distinct from an empty-list override)', () => {
    expect(mapData).toMatch(/resetCoverage[\s\S]*method: 'DELETE'/)
  })
})
