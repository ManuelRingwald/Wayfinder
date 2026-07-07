// #210: the tenant overview owns the operational configuration that used to crowd
// the detail page. Feeds, OpenAIP and access accounts each get their own column
// with a classic config icon that opens a focused dialog; the detail page
// ("Konfigurieren") is reduced to the default view + features. Source-level guards
// (project convention — no Vuetify mount) against the overview and the detail SFC.
import { describe, it, expect } from 'vitest'
import tenants from '../AdminTenants.vue?raw'
import detail from '../AdminTenantDetail.vue?raw'

describe('AdminTenants config columns (#210)', () => {
  it('has dedicated Feeds, OpenAIP and Nutzer columns', () => {
    expect(tenants).toContain('<th class="text-center">Feeds</th>')
    expect(tenants).toContain('<th class="text-center">OpenAIP</th>')
    expect(tenants).toContain('<th class="text-center">Nutzer</th>')
    // The empty-state colspan spans every column (incl. the #209 Gastmodus column).
    expect(tenants).toContain('colspan="8"')
  })

  it('opens each config column via a classic config icon', () => {
    // A cog icon-button per function, each wired to its open handler.
    expect(tenants).toContain('icon="mdi-cog-outline"')
    expect(tenants).toContain('@click="openFeeds(t)"')
    expect(tenants).toContain('@click="openOpenAIP(t)"')
    expect(tenants).toContain('@click="openUsers(t)"')
  })

  it('hosts the three functions as focused dialogs', () => {
    expect(tenants).toContain('AdminProvisioning')
    expect(tenants).toContain('AdminTenantOpenAIP')
    expect(tenants).toContain('AdminUsers')
    expect(tenants).toContain('v-model="feedsDialog"')
    expect(tenants).toContain('v-model="openaipDialog"')
    expect(tenants).toContain('v-model="usersDialog"')
  })

  it('refreshes the overview when a hosted dialog changes state', () => {
    // A feed grant/revoke reloads chips + health; closing the users dialog reloads
    // the account count.
    expect(tenants).toContain('async function onFeedsChanged')
    expect(tenants).toContain('async function onUsersDialogToggle')
    expect(tenants).toContain('admin.loadOverview()')
  })
})

describe('AdminTenantDetail is slimmed to view + features (#210)', () => {
  it('no longer embeds the feeds / users components', () => {
    expect(detail).not.toContain('AdminProvisioning')
    expect(detail).not.toContain('AdminUsers')
  })

  it('no longer carries the OpenAIP, Feeds or Zugänge cards', () => {
    expect(detail).not.toContain('OpenAIP-Konfiguration')
    expect(detail).not.toContain('saveOpenAIPKey')
    // The remaining two sections stay.
    expect(detail).toContain('Standard-Ansicht')
    expect(detail).toContain('<v-card-title class="text-subtitle-1">Features</v-card-title>')
  })
})
