// Regression guard: granting/revoking a feed in the embedded provisioning table
// must keep the overview's feed chips in sync. Those chips derive from
// admin.overview, so the provisioning component has to signal a change and its host
// has to reload the overview — otherwise the chips drift out of sync with the
// assignment table (observed in the E2E run: the header still showed the old feed
// while the table already showed the new one). Since #210 the provisioning table is
// hosted by the Feeds dialog in the tenant overview (AdminTenants), not the detail
// page, so the @changed → loadOverview wiring lives there.
//
// The repo has no component-mount test infrastructure (see adminTenantDetailCaptions
// for the same rationale), so we assert the wiring against the single-file-component
// source served raw by Vite.
import { describe, it, expect } from 'vitest'
import provisioning from '../AdminProvisioning.vue?raw'
import tenants from '../AdminTenants.vue?raw'

describe('feed provisioning ↔ overview refresh', () => {
  it('AdminProvisioning declares a `changed` event', () => {
    expect(provisioning).toContain("defineEmits(['changed'])")
  })

  it("emits `changed` after a successful grant and revoke", () => {
    // Both mutation handlers must emit inside their success branch, next to the
    // local refreshTenantSubs(), so the host learns the feed set moved.
    const emits = provisioning.match(/emit\('changed'\)/g) || []
    expect(emits.length).toBeGreaterThanOrEqual(2)
  })

  it('AdminTenants wires @changed on the feeds dialog provisioning table (#210)', () => {
    expect(tenants).toContain('@changed="onFeedsChanged"')
  })

  it('reloads the overview (feed chips) when feeds change', () => {
    // onFeedsChanged must refresh admin.overview — the source of tenant.feeds the
    // overview chips render — so a grant/revoke is reflected immediately.
    expect(tenants).toContain('async function onFeedsChanged')
    expect(tenants).toContain('admin.loadOverview()')
  })
})
