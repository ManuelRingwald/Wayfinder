// Regression guard: granting/revoking a feed in the embedded provisioning table
// must keep the tenant detail page's header feed chips in sync. Those chips derive
// from admin.overview (loaded once by the parent), so the provisioning component
// has to signal a change and the parent has to reload the overview — otherwise the
// chips drift out of sync with the assignment table (observed in the E2E run: the
// header still showed the old feed while the table already showed the new one).
//
// The repo has no component-mount test infrastructure (see adminTenantDetailCaptions
// for the same rationale), so we assert the wiring against the single-file-component
// source served raw by Vite.
import { describe, it, expect } from 'vitest'
import provisioning from '../AdminProvisioning.vue?raw'
import detail from '../AdminTenantDetail.vue?raw'

describe('feed provisioning ↔ tenant header refresh', () => {
  it('AdminProvisioning declares a `changed` event', () => {
    expect(provisioning).toContain("defineEmits(['changed'])")
  })

  it("emits `changed` after a successful grant and revoke", () => {
    // Both mutation handlers must emit inside their success branch, next to the
    // local refreshTenantSubs(), so the parent learns the feed set moved.
    const emits = provisioning.match(/emit\('changed'\)/g) || []
    expect(emits.length).toBeGreaterThanOrEqual(2)
  })

  it('AdminTenantDetail wires @changed on the embedded provisioning table', () => {
    expect(detail).toContain('@changed="onFeedsChanged"')
  })

  it('reloads the overview (header chips) when feeds change', () => {
    // onFeedsChanged must refresh admin.overview — the source of tenant.feeds the
    // header chips render — so a grant/revoke is reflected immediately.
    expect(detail).toContain('async function onFeedsChanged')
    expect(detail).toContain('admin.loadOverview()')
  })
})
