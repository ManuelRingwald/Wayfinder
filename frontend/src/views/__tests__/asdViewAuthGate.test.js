// Structure guard for the ASD map's auth gate (ADR 0014: auth is always on). A
// tenant must be able to open '/', log in, and see ONLY their scoped picture; an
// unauthenticated visitor must get a login screen, not a blank map. We assert
// against the raw SFC source (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import sfc from '../AsdView.vue?raw'
import layerFilter from '../../components/LayerFilterContent.vue?raw'
import rail from '../../components/NavigationRail.vue?raw'

describe('AsdView auth gate (ADR 0014)', () => {
  it('probes the session on mount', () => {
    expect(sfc).toContain("import { useSessionStore } from '@/stores/session.js'")
    expect(sfc).toContain('onMounted(')
    expect(sfc).toContain('session.probe()')
  })

  it('shows a spinner while loading, the login card when anonymous', () => {
    expect(sfc).toContain("session.status === 'loading'")
    expect(sfc).toContain("session.status === 'anon'")
    expect(sfc).toContain('<LoginCard')
    // minimal product branding on the landing login (Punkt 1)
    expect(sfc).toContain('title="Wayfinder — Anmelden"')
    expect(sfc).toContain('@submit="onLogin"')
  })

  it('only mounts the map (and thus opens /ws) once authenticated', () => {
    // The map lives in the v-else branch, so MapCanvas never mounts while anon.
    expect(sfc).toContain('<template v-else>')
    expect(sfc).toContain('<MapCanvas')
  })

  // The account chip that used to float top-right on the map was removed — it
  // duplicated the sidebar. Account access now lives ONLY in the navigation
  // sidebar, so the map itself no longer carries subject/logout.
  it('does not duplicate account access on the map (moved to the sidebar)', () => {
    expect(sfc).not.toContain('account-overlay')
    expect(sfc).not.toContain('session.subject')
    expect(sfc).not.toContain('onLogout')
  })

  it('keeps the logged-in subject + logout in the sidebar account section', () => {
    expect(layerFilter).toContain('session.subject')
    expect(layerFilter).toContain('session.logout()')
    expect(layerFilter).toContain('Abmelden')
  })

  it('keeps the admin shortcut in the navigation rail', () => {
    expect(rail).toContain("router.push('/admin')")
  })

  it('runs the sliding-session refresh only while authenticated (WF2-12.5)', () => {
    expect(sfc).toContain('session.startRenew()')
    expect(sfc).toContain('session.stopRenew()')
    // renew on tab focus and on WebSocket (re)connect
    expect(sfc).toContain("document.addEventListener('visibilitychange'")
    expect(sfc).toContain('@connection-change="onConnectionChange"')
    expect(sfc).toContain('session.renewNow()')
  })

  it('makes an expiry visible (login screen, not a frozen map)', () => {
    // on a WS drop, probe the session; a lost session flips to the login screen
    expect(sfc).toContain('session.probe()')
    expect(sfc).toContain('Sitzung abgelaufen')
  })
})
