// #208 (ADR 0022): an admin has no own air picture. The ASD view gates the map
// AFTER authentication: a must-change principal is sent to /admin (the forced
// password mask lives there; the server refuses every data path anyway), and an
// admin is sent to /admin unless read-only guest mode (impersonation) is active.
// Ending guest mode returns to /admin. Source-level guards (project convention —
// no Vuetify mount).
import { describe, it, expect } from 'vitest'
import asd from '../AsdView.vue?raw'
import adminView from '../AdminView.vue?raw'
import bar from '../../components/ImpersonationBar.vue?raw'
import session from '../../stores/session.js?raw'

describe('ASD admin gate (#208, ADR 0022)', () => {
  it('session store exposes the must-change flag from whoami', () => {
    expect(session).toContain('must_change_password')
    expect(session).toContain('mustChangePassword')
  })

  it('redirects a must-change principal to /admin before the map mounts', () => {
    expect(asd).toContain('session.mustChangePassword')
    expect(asd).toContain("router.replace('/admin')")
  })

  it('admits an admin only with active guest mode, else redirects to /admin', () => {
    expect(asd).toContain('useImpersonationStore')
    expect(asd).toContain('imp.loadStatus()')
    expect(asd).toContain('if (!imp.active)')
    // The gate holds the spinner until it resolves, so the map (and /ws) never
    // mounts for a principal about to be redirected.
    expect(asd).toContain("adminGate !== 'ok'")
  })

  it('re-checks the grant when the stream drops (TTL expiry) for admins', () => {
    const handler = asd.slice(asd.indexOf('async function onConnectionChange'))
    expect(handler).toContain('imp.loadStatus()')
    expect(handler).toContain("router.replace('/admin')")
  })

  it('ending guest mode returns to /admin instead of a dead own picture', () => {
    expect(bar).toContain('async function exitGuestMode')
    expect(bar).toContain('await imp.stop()')
    expect(bar).toContain("router.push('/admin')")
  })

  it('the admin app bar no longer offers a "Zur Lage" shortcut', () => {
    // The radar shortcut button is gone; the 403 notice's "zur Lage zurück"
    // link for NON-admin visitors deliberately stays (a tenant user does have
    // an own picture to return to).
    expect(adminView).not.toContain('>Zur Lage</v-btn>')
    expect(adminView).not.toContain('mdi-radar')
  })
})
