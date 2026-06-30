import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiFetch } from '@/api.js'

// useSessionStore backs the ASD map's own auth gate (the operational view at '/').
// Unlike the admin store's role probe (which hits the admin-gated whoami), this
// uses the role-agnostic GET /api/whoami so a plain tenant user can resolve its
// session: status 'authed' renders the live picture, 'anon' renders the login
// screen. Auth is enforced server-side on /ws regardless — this gate only decides
// what the browser shows (fail-closed: no identity ⇒ login, never a blank map).
export const useSessionStore = defineStore('session', () => {
  const identity = ref(null) // { subject, tenant_id, role, ... } or null
  const status = ref('loading') // 'loading' | 'authed' | 'anon'
  const error = ref(null) // last login error (shown on the login card)

  const subject = computed(() => identity.value?.subject ?? null)
  const role = computed(() => identity.value?.role ?? null)
  const isAdmin = computed(() => role.value === 'admin')

  // probe resolves the current session via the role-agnostic identity endpoint.
  // 200 → authed; anything else (401 etc.) → anon.
  async function probe() {
    const r = await apiFetch('/api/whoami')
    if (r.ok) {
      identity.value = r.data
      status.value = 'authed'
    } else {
      identity.value = null
      status.value = 'anon'
    }
    return status.value
  }

  // login posts credentials, then re-probes so the gate flips to 'authed' only
  // once the server has actually accepted the session cookie.
  async function login(subjectName, password) {
    error.value = null
    const r = await apiFetch('/api/login', {
      method: 'POST',
      body: JSON.stringify({ subject: subjectName, password }),
    })
    if (!r.ok) {
      error.value = r.error
      return false
    }
    await probe()
    return status.value === 'authed'
  }

  // logout clears the server session (wf_session cookie) and returns to the login
  // screen.
  async function logout() {
    await apiFetch('/api/logout', { method: 'POST' })
    identity.value = null
    status.value = 'anon'
  }

  return { identity, status, error, subject, role, isAdmin, probe, login, logout }
})
