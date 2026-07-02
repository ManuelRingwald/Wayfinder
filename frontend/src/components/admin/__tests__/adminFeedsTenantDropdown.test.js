// Regression guard: the "adopt from tenant" dropdown in the feed-sources dialog
// must reflect tenants created after the list was first fetched. The dropdown
// reads admin.tenants (the cross-tenant list), which — unlike admin.overview —
// does not refresh on tenant creation; the earlier lazy guard
// (`if (!admin.tenants.length) admin.loadTenants()`) therefore showed a stale set
// that was missing newly-created tenants. openSources now reloads unconditionally.
// Source-level assertions (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import sfc from '../AdminFeeds.vue?raw'

describe('feed-sources tenant dropdown refresh', () => {
  it('drops the stale lazy guard on the tenant reload', () => {
    expect(sfc).not.toContain('if (!admin.tenants.length) admin.loadTenants()')
  })

  it('still reloads the tenant list when the dialog opens', () => {
    expect(sfc).toContain('admin.loadTenants()')
  })
})
