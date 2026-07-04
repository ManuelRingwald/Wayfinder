// B2 (WF2-34, ADR 0008): the read-only "View as Tenant" must be reachable from
// the ADMIN UI, not only via the bar hidden on the map — the E2E test showed the
// function was effectively undiscoverable there. These checks pin the entry
// point on the tenant detail page (button → mint grant → jump to the map) at
// the source level, consistent with the house style for template guards.
import { describe, it, expect } from 'vitest'
import sfc from '../AdminTenantDetail.vue?raw'

describe('AdminTenantDetail "Als Mandant ansehen" entry (B2, ADR 0008)', () => {
  it('offers the read-only view-as-tenant button in the header row', () => {
    expect(sfc).toContain('Als Mandant ansehen')
    expect(sfc).toContain('mdi-account-eye-outline')
  })

  it('mints the grant via the impersonation store and jumps to the map', () => {
    expect(sfc).toContain("useImpersonationStore")
    expect(sfc).toContain('imp.start(props.tenantId)')
    expect(sfc).toContain("router.push('/')")
  })

  it('stays on the page and surfaces the error when minting fails', () => {
    expect(sfc).toContain('impError')
    expect(sfc).toContain('Ansehen als Mandant fehlgeschlagen')
  })
})
