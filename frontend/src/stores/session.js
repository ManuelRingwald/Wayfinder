import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiFetch } from '@/api.js'

// Sliding-session refresh cadence (WF2-12.5): the ASD re-mints the cookie this
// often while the live picture is open, so an actively-used console is never
// logged out. Kept comfortably below the server session TTL (default 12h).
export const RENEW_INTERVAL_MS = 10 * 60 * 1000 // 10 min

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
  // expired is true when a previously-authenticated session dropped (vs. never
  // logged in) — lets the login screen say "Sitzung abgelaufen" instead of a bare
  // prompt, so an expiry is visible rather than a silent frozen map (WF2-12.5).
  const expired = ref(false)

  let renewTimer = null

  const subject = computed(() => identity.value?.subject ?? null)
  const role = computed(() => identity.value?.role ?? null)
  const isAdmin = computed(() => role.value === 'admin')

  // Feature entitlements of the logged-in principal's tenant, delivered by the
  // role-agnostic whoami (WF2-50). The ASD map uses these to show a lotse only the
  // layers/filters their tenant is entitled to (Issue #106). Cosmetic gating — the
  // server enforces access independently.
  const features = computed(() => identity.value?.features ?? {})
  function hasFeature(key) {
    return features.value[key] === true
  }

  // sensorClasses is the union of sensor classes across the tenant's subscribed
  // feeds (Issue #107), so the map's provenance legend lists only the entries the
  // tenant's feeds can actually produce. Empty when nothing is subscribed / not yet
  // loaded — the legend then falls back to showing all entries.
  const sensorClasses = computed(() => identity.value?.sensor_classes ?? [])

  // probe resolves the current session via the role-agnostic identity endpoint.
  // 200 → authed; anything else (401 etc.) → anon. A transition from authed → anon
  // marks the session as expired (for the visible "session expired" hint).
  async function probe() {
    const wasAuthed = status.value === 'authed'
    const r = await apiFetch('/api/whoami')
    if (r.ok) {
      identity.value = r.data
      status.value = 'authed'
      expired.value = false
    } else {
      identity.value = null
      status.value = 'anon'
      if (wasAuthed) expired.value = true
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
    expired.value = false
    await probe()
    return status.value === 'authed'
  }

  // renewNow slides the session forward (re-mints the cookie) for the current
  // principal. A 401 means the session is already gone → re-probe so the UI flips
  // to the login screen. Safe to call when anonymous (the 401 path just confirms).
  async function renewNow() {
    const r = await apiFetch('/api/session/renew', { method: 'POST' })
    if (r.status === 401) {
      await probe()
      return false
    }
    return r.ok
  }

  // startRenew begins the periodic sliding refresh. Idempotent — a second call
  // replaces the prior timer.
  function startRenew(intervalMs = RENEW_INTERVAL_MS) {
    stopRenew()
    renewTimer = setInterval(renewNow, intervalMs)
  }

  function stopRenew() {
    if (renewTimer) {
      clearInterval(renewTimer)
      renewTimer = null
    }
  }

  // logout clears the server session (wf_session cookie) and returns to the login
  // screen.
  async function logout() {
    stopRenew()
    await apiFetch('/api/logout', { method: 'POST' })
    identity.value = null
    status.value = 'anon'
    expired.value = false
  }

  return {
    identity, status, error, expired, subject, role, isAdmin,
    features, hasFeature, sensorClasses,
    probe, login, renewNow, startRenew, stopRenew, logout,
  }
})
