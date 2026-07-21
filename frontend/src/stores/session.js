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
  // email mirrors the caller's own contact email (whoami, #319) so the "Konto"
  // self-service panel can show and prefill it. Null when the user has none.
  const email = computed(() => identity.value?.email ?? null)

  // mustChangePassword mirrors the whoami flag (ONB-1, ADR 0011): the principal is
  // still on the well-known seed credential and the server refuses every data path
  // (403 password_change_required, #208/ADR 0022). The ASD gate redirects such a
  // principal to /admin, where the forced-change mask lives.
  const mustChangePassword = computed(() => identity.value?.must_change_password === true)

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

  // flMin/flMax mirror the effective view's flight-level band (Standard-Ansicht
  // or per-user override, from whoami). The sidebar greys them into the FL
  // filter inputs as the admissible-range hint (#116). Null when unset.
  const flMin = computed(() => identity.value?.fl_min ?? null)
  const flMax = computed(() => identity.value?.fl_max ?? null)

  // icao mirrors the effective view's optional location indicator (whoami) so the
  // ASD header can show which sector/FIR this picture belongs to (e.g. "EDGG·KTG").
  // Null when unset — the header then omits it (display config, not track data).
  const icao = computed(() => identity.value?.icao ?? null)

  // viewCenter mirrors the effective view's map viewport (centre + zoom, from
  // whoami) so the ASD opens on the tenant's own sector instead of the global
  // WAYFINDER_MAP_CENTER_* default (FR-UI-013). Null when the tenant has no view
  // config → the map keeps the /api/map-config env centre. center_lat === 0 is a
  // valid latitude (equator), so presence is tested with != null, not truthiness.
  const viewCenter = computed(() => {
    const i = identity.value
    if (!i || i.center_lat == null || i.center_lon == null) return null
    return { lat: i.center_lat, lon: i.center_lon, zoom: i.zoom ?? null }
  })

  // aoi mirrors the effective view's area of interest (WGS84 bbox, from whoami).
  // The ASD clips the DWD weather overlays (radar raster + warnings) to it so a
  // controller only sees weather in their own sector (#189/#190). Null when the
  // tenant has no AOI configured → the overlays are shown unclipped.
  const aoi = computed(() => {
    const a = identity.value?.aoi
    if (!a) return null
    return { minLat: a.min_lat, minLon: a.min_lon, maxLat: a.max_lat, maxLon: a.max_lon }
  })

  // aorAirspaceIds mirrors the effective view's Area of Responsibility (ASD-014,
  // ADR 0021): the OpenAIP airspace ids the map highlights as the tenant's
  // controlled volumes (CTR/TMA). Empty array when no AoR is configured (whoami
  // omits the field) — the map then highlights nothing. Cosmetic display config;
  // the airspace features themselves already arrive via /api/airspace.
  const aorAirspaceIds = computed(() => identity.value?.aor_airspace_ids ?? [])

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

  // changeOwnPassword changes the logged-in principal's OWN password (#319) via
  // the role-agnostic self-service endpoint, so a plain tenant user can do it
  // from the ASD "Konto" panel (the admin-gated /api/admin/me/* was admin-only).
  // The current password is required (a stolen live session cannot silently
  // rotate the credential). Returns the raw apiFetch result.
  async function changeOwnPassword(currentPassword, newPassword) {
    return apiFetch('/api/account/password', {
      method: 'PUT',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    })
  }

  // changeOwnEmail sets the principal's OWN contact email (#319). On success it
  // re-probes so the displayed/prefilled email updates (whoami carries it); the
  // change also surfaces in the admin access table on its next load. Returns the
  // raw apiFetch result.
  async function changeOwnEmail(newEmail) {
    const r = await apiFetch('/api/account/email', {
      method: 'PUT',
      body: JSON.stringify({ email: newEmail }),
    })
    if (r.ok) await probe()
    return r
  }

  return {
    identity, status, error, expired, subject, role, isAdmin, email, mustChangePassword,
    features, hasFeature, sensorClasses, flMin, flMax, icao, viewCenter, aoi, aorAirspaceIds,
    probe, login, renewNow, startRenew, stopRenew, logout,
    changeOwnPassword, changeOwnEmail,
  }
})
