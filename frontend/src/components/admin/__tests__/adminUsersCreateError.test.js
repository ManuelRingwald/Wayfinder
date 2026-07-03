// Regression guard: creating a user that already exists must explain why, not
// fail silently. Subjects are globally unique (a "lotse" in one tenant blocks the
// same subject in another), so the backend returns 409; the create dialog now
// surfaces a clear German reason instead of closing/doing nothing.
// Source-level assertions (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import sfc from '../AdminUsers.vue?raw'

describe('user create error feedback', () => {
  it('shows an inline error alert in the create dialog', () => {
    expect(sfc).toContain('v-if="createError"')
    expect(sfc).toContain('{{ createError }}')
  })

  it('maps a 409 to a clear "subject taken / globally unique" message', () => {
    expect(sfc).toContain('r.status === 409')
    expect(sfc).toContain('mandantenübergreifend eindeutig')
  })

  it('sets the error instead of silently returning on failure', () => {
    // submitCreate must assign createError on a failed response.
    expect(sfc).toContain('createError.value = createErrorMessage(r, subject)')
  })
})
