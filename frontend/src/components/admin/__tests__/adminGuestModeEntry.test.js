// #209 (WF2-34, ADR 0008): the read-only guest mode is entered ONLY from the tenant
// overview — an eye icon in the "Gastmodus" column mints the grant and jumps to the
// map. The old entry points (a button on the detail page, the ImpersonationBar's
// start menu) were removed so there is exactly one way in. Source-level guards
// (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import tenants from '../AdminTenants.vue?raw'
import detail from '../AdminTenantDetail.vue?raw'
import bar from '../../ImpersonationBar.vue?raw'

describe('guest-mode entry via the overview eye column (#209)', () => {
  it('the overview has a "Gastmodus" column with an eye icon', () => {
    expect(tenants).toContain('<th class="text-center">Gastmodus</th>')
    expect(tenants).toContain('icon="mdi-eye-outline"')
    expect(tenants).toContain('@click="viewAsTenant(t)"')
  })

  it('the eye mints the grant and jumps to the map', () => {
    expect(tenants).toContain('useImpersonationStore')
    expect(tenants).toContain('imp.start(t.id)')
    expect(tenants).toContain("router.push('/')")
  })

  it('surfaces an error inline when minting fails', () => {
    expect(tenants).toContain('impError')
    expect(tenants).toContain('Ansehen als Mandant fehlgeschlagen')
  })

  it('the detail page no longer offers a view-as entry', () => {
    expect(detail).not.toContain('Als Mandant ansehen')
    expect(detail).not.toContain('viewAsTenant')
  })

  it('the ImpersonationBar renders only as the active read-only banner, no start menu', () => {
    // Only shows while a grant is active; the inactive "Als Mandant ansehen" menu is gone.
    expect(bar).toContain('v-if="admin.isAdmin && imp.active"')
    expect(bar).not.toContain('Als Mandant ansehen')
  })
})
