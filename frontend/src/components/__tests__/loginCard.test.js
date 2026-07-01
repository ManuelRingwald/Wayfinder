// Structure guard for the reusable LoginCard (ADR 0014 tenant-facing login). The
// component is presentational and emits `submit` with the entered credentials;
// the parent owns the auth call. We assert against the raw SFC source (the project
// convention) rather than mounting the full Vuetify stack.
import { describe, it, expect } from 'vitest'
import sfc from '../LoginCard.vue?raw'

describe('LoginCard', () => {
  it('renders username and password fields', () => {
    expect(sfc).toContain('label="Benutzername"')
    expect(sfc).toContain('label="Passwort"')
    expect(sfc).toContain('autocomplete="current-password"')
  })

  it('has a submit button labelled Anmelden', () => {
    expect(sfc).toContain('type="submit"')
    expect(sfc).toContain('Anmelden')
  })

  it('emits a submit event carrying the entered credentials', () => {
    expect(sfc).toContain("defineEmits(['submit'])")
    expect(sfc).toContain("emit('submit', { subject: subject.value, password: password.value })")
  })

  it('disables submit until both fields are filled', () => {
    expect(sfc).toContain(':disabled="!subject || !password"')
  })
})
