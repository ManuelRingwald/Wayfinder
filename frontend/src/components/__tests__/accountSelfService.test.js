// #319: account self-service — a plain tenant user (and admins) can set their
// OWN e-mail + password under "Konto", via the role-agnostic /api/account/*
// endpoints (the admin-gated /api/admin/me/* was reachable only by admins).
// Source-level wiring guards (the repo has no component-mount infra), matching
// the other UI tests' approach.
import { describe, it, expect } from 'vitest'
import sessionStore from '../../stores/session.js?raw'
import adminStore from '../../stores/admin.js?raw'
import dialog from '../AccountSelfServiceDialog.vue?raw'
import myAccount from '../admin/MyAccountPanel.vue?raw'
import lfc from '../LayerFilterContent.vue?raw'

describe('session store exposes role-agnostic account self-service (#319)', () => {
  it('changeOwnPassword / changeOwnEmail hit /api/account/*', () => {
    expect(sessionStore).toContain("'/api/account/password'")
    expect(sessionStore).toContain("'/api/account/email'")
    expect(sessionStore).toContain('changeOwnPassword')
    expect(sessionStore).toContain('changeOwnEmail')
    // email is surfaced from whoami for display + prefill
    expect(sessionStore).toContain('identity.value?.email')
  })

  it('re-probes after an email change so the displayed email refreshes', () => {
    expect(sessionStore).toMatch(/changeOwnEmail[\s\S]*?await probe\(\)/)
  })
})

describe('admin store gains changeOwnEmail for the dashboard "Mein Konto" (#319)', () => {
  it('changeOwnEmail hits /api/account/email and reloads the identity', () => {
    expect(adminStore).toContain('changeOwnEmail')
    expect(adminStore).toContain("'/api/account/email'")
    expect(adminStore).toMatch(/changeOwnEmail[\s\S]*?loadIdentity\(\)/)
  })
})

describe('AccountSelfServiceDialog (ASD operator self-service)', () => {
  it('uses the session store and offers both e-mail and password forms', () => {
    expect(dialog).toContain('useSessionStore')
    expect(dialog).toContain('submitEmail')
    expect(dialog).toContain('submitPassword')
    expect(dialog).toContain('session.changeOwnEmail')
    expect(dialog).toContain('session.changeOwnPassword')
  })

  it('enforces the min-8 password standard client-side (server is authoritative)', () => {
    expect(dialog).toContain('pwNew.value.length < 8')
  })
})

describe('the sidebar "Konto" section opens the self-service dialog (#319)', () => {
  it('mounts the dialog and a "Konto verwalten" button', () => {
    expect(lfc).toContain('AccountSelfServiceDialog')
    expect(lfc).toContain('Konto verwalten')
    expect(lfc).toContain('accountDialog')
  })
})

describe('the admin dashboard "Mein Konto" gains an e-mail field (#319)', () => {
  it('has an e-mail form wired to changeOwnEmail', () => {
    expect(myAccount).toContain('submitEmailChange')
    expect(myAccount).toContain('admin.changeOwnEmail')
    expect(myAccount).toContain('E-Mail-Adresse')
  })
})
