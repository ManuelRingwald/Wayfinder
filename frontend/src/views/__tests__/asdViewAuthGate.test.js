// Structure guard for the ASD map's auth gate (ADR 0014: auth is always on). A
// tenant must be able to open '/', log in, and see ONLY their scoped picture; an
// unauthenticated visitor must get a login screen, not a blank map. We assert
// against the raw SFC source (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import sfc from '../AsdView.vue?raw'

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
    expect(sfc).toContain('@submit="onLogin"')
  })

  it('only mounts the map (and thus opens /ws) once authenticated', () => {
    // The map lives in the v-else branch, so MapCanvas never mounts while anon.
    expect(sfc).toContain('<template v-else>')
    expect(sfc).toContain('<MapCanvas')
  })

  it('offers a logout action wired to the session store', () => {
    expect(sfc).toContain('async function onLogout()')
    expect(sfc).toContain('session.logout()')
    expect(sfc).toContain('Abmelden')
  })

  it('shows the logged-in subject and an admin shortcut for admins', () => {
    expect(sfc).toContain('session.subject')
    expect(sfc).toContain('session.isAdmin')
    expect(sfc).toContain("{ name: 'admin' }")
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
