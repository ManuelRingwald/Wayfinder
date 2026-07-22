// T3 (ADR 0035): the per-tenant config screen is grouped into tabs (Sicht |
// Freigaben | Kartendaten); the Kartendaten tab edits the tenant's own base map
// (theme + style URL) via the T2 admin endpoints. Source-level guards (house
// convention — no Vuetify mount for admin panels).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const src = (rel) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
const detail = src('../AdminTenantDetail.vue')

describe('AdminTenantDetail: tab layout (T3)', () => {
  it('groups the config into Sicht / Freigaben / Kartendaten tabs', () => {
    expect(detail).toContain('<v-tabs')
    expect(detail).toContain('v-model="tab"')
    for (const t of ['Sicht', 'Freigaben', 'Kartendaten']) {
      expect(detail).toContain(`>${t}</v-tab>`)
    }
    // Three window items wrap the existing + new sections.
    expect(detail).toContain('value="view"')
    expect(detail).toContain('value="entitlements"')
    expect(detail).toContain('value="mapdata"')
  })

  it('keeps the single global save for Sicht+Freigaben but hides it on Kartendaten', () => {
    // The global save (saveAll) is not shown on the mapdata tab, which saves itself.
    expect(detail).toMatch(/v-if="tab !== 'mapdata'"[\s\S]*saveAll/)
  })
})

describe('AdminTenantDetail: per-tenant base map editor (T3)', () => {
  it('loads + saves the tenant base map via the T2 admin endpoints', () => {
    expect(detail).toContain('/api/admin/tenants/${props.tenantId}/mapdata/basemap')
    expect(detail).toContain('loadTenantBasemap')
    expect(detail).toContain('saveTenantTheme')
    expect(detail).toContain('saveTenantStyle')
    expect(detail).toContain('resetTenantBasemap') // empty value → back to global
    expect(detail).toContain("method: 'PUT'")
  })

  it('binds the theme select + style field and shows the override state', () => {
    expect(detail).toContain('v-model="basemap.themeInput"')
    expect(detail).toContain('v-model="basemap.styleInput"')
    expect(detail).toContain('basemap.overridden')
    // The reset targets both settings so the tenant fully returns to the global map.
    expect(detail).toMatch(/resetTenantBasemap[\s\S]*theme[\s\S]*style-url/)
  })
})
