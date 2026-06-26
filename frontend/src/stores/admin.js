import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// apiFetch wraps fetch for the admin API: it always sends/expects JSON and
// normalises the result into { ok, status, data, error } so callers never juggle
// response parsing or status branching. A non-2xx with an {"error": "..."} body
// surfaces that message; otherwise a generic "HTTP <status>" is used.
async function apiFetch(path, options = {}) {
  let res
  try {
    res = await fetch(path, {
      headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
      ...options,
    })
  } catch (e) {
    return { ok: false, status: 0, data: null, error: `network error: ${e?.message ?? e}` }
  }
  let data = null
  const text = await res.text()
  if (text) {
    try { data = JSON.parse(text) } catch { data = null }
  }
  if (!res.ok) {
    return { ok: false, status: res.status, data, error: (data && data.error) || `HTTP ${res.status}` }
  }
  return { ok: true, status: res.status, data, error: null }
}

// useAdminStore backs the WF2-32 admin dashboard. It consumes the WF2-31 admin
// REST API; the role gating it exposes (isAdmin) is cosmetic — the server
// independently enforces every boundary (RequireRole(admin) → 403).
export const useAdminStore = defineStore('admin', () => {
  const identity = ref(null)     // whoami: { subject, tenant_id, user_id, role }
  const accessError = ref(null)  // set when whoami is refused (401/403)
  const accessStatus = ref(null) // HTTP status when whoami fails (401 = not logged in, 403 = wrong role)
  const view = ref(null)         // the tenant's effective view config (or null)
  const feeds = ref([])          // full feed catalogue
  const subscriptions = ref([])  // feeds the current tenant is subscribed to
  const tenants = ref([])        // admin: all tenants (cross-tenant provisioning)
  const overview = ref([])       // AP3: aggregated per-tenant dashboard rows
  const error = ref(null)        // last action error (banner)
  const notice = ref(null)       // last success message (banner)

  const role = computed(() => identity.value?.role ?? null)
  // Admin-rail visibility (Req 1, ADR 0009): only admin may reach /admin.
  // Cosmetic gating — the server enforces /api/admin/* via RequireRole(admin).
  const isAdmin = computed(() => role.value === 'admin')
  const isAuthorized = computed(() => identity.value !== null)
  // ONB-1 (ADR 0011): a freshly seeded admin must replace its known default
  // password before anything else. While true, the server refuses every admin
  // route except the password change (403 password_change_required); the UI
  // mirrors that by routing the user to the forced-change mask.
  const mustChangePassword = computed(() => identity.value?.must_change_password === true)
  // WF2-50: per-tenant feature entitlements, delivered by whoami. UI gating off
  // these is cosmetic — the server enforces every feature server-side.
  const features = computed(() => identity.value?.features ?? {})
  function hasFeature(key) {
    return features.value[key] === true
  }

  // loadIdentity is the role probe the dashboard runs on entry. A refusal means
  // the principal is not an admin → the UI shows an access notice instead of the
  // panels. Returns true on success.
  async function loadIdentity() {
    const r = await apiFetch('/api/admin/whoami')
    if (r.ok) {
      identity.value = r.data
      accessError.value = null
      accessStatus.value = null
    } else {
      identity.value = null
      accessError.value = r.error
      accessStatus.value = r.status
    }
    return r.ok
  }

  // login posts credentials to the builtin auth endpoint. On success the server
  // sets a wf_session HttpOnly cookie; the caller should follow up with
  // loadIdentity() to populate the store.
  async function login(subject, password) {
    return apiFetch('/api/login', {
      method: 'POST',
      body: JSON.stringify({ subject, password }),
    })
  }

  async function loadView() {
    const r = await apiFetch('/api/admin/view')
    if (r.ok) view.value = r.data
    else if (r.status === 404) view.value = null // no config yet → defaults
    else error.value = r.error
    return r
  }

  async function saveView(dto) {
    error.value = null
    notice.value = null
    const r = await apiFetch('/api/admin/view', { method: 'PUT', body: JSON.stringify(dto) })
    if (r.ok) {
      view.value = r.data
      notice.value = 'Ansicht gespeichert.'
    } else {
      error.value = r.error
    }
    return r
  }

  async function loadFeeds() {
    const r = await apiFetch('/api/admin/feeds')
    if (r.ok) feeds.value = r.data
    else error.value = r.error
    return r
  }

  async function loadSubscriptions() {
    const r = await apiFetch('/api/admin/subscriptions')
    if (r.ok) subscriptions.value = r.data
    else error.value = r.error
    return r
  }

  // --- cross-tenant provisioning (admin) -----------------------------------

  async function loadTenants() {
    const r = await apiFetch('/api/admin/tenants')
    if (r.ok) tenants.value = r.data
    else error.value = r.error
    return r
  }

  // loadTenantSubscriptions returns the feeds a target tenant is subscribed to
  // without storing it globally (the caller owns that transient list).
  async function loadTenantSubscriptions(tenantId) {
    return apiFetch(`/api/admin/tenants/${tenantId}/subscriptions`)
  }

  // --- AP3: tenant-centric dashboard ----------------------------------------

  const feedsHealth = ref({}) // AP4: feedId → { color, stale, ever_seen, last_heartbeat_ago_s, track_count_recent }

  // loadFeedsHealth fetches per-feed health state and stores it keyed by feed_id.
  async function loadFeedsHealth() {
    const r = await apiFetch('/api/admin/feeds/health')
    if (r.ok && Array.isArray(r.data)) {
      const map = {}
      for (const h of r.data) map[h.feed_id] = h
      feedsHealth.value = map
    }
    return r
  }

  // loadOverview fetches the aggregated per-tenant dashboard (status, features,
  // feeds, account count) in one call and stores it for the overview table.
  async function loadOverview() {
    const r = await apiFetch('/api/admin/overview')
    if (r.ok) overview.value = r.data
    else error.value = r.error
    return r
  }

  // loadTenantView reads a target tenant's default view (cross-tenant editor).
  // Returns the raw result; the caller owns the transient view (it edits a copy).
  // A 404 (no view configured yet) is surfaced to the caller as r.status === 404.
  async function loadTenantView(tenantId) {
    return apiFetch(`/api/admin/tenants/${tenantId}/view`)
  }

  // saveTenantView writes a target tenant's default view (cross-tenant editor).
  async function saveTenantView(tenantId, dto) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/view`, {
      method: 'PUT',
      body: JSON.stringify(dto),
    })
    if (r.ok) notice.value = 'Ansicht gespeichert.'
    else error.value = r.error
    return r
  }

  // loadTenantEntitlements returns the full feature catalogue with the target
  // tenant's state (the caller owns the transient list).
  async function loadTenantEntitlements(tenantId) {
    return apiFetch(`/api/admin/tenants/${tenantId}/entitlements`)
  }

  async function setTenantEntitlement(tenantId, key, enabled) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/entitlements/${key}`, {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    })
    if (r.ok) notice.value = enabled ? 'Feature aktiviert.' : 'Feature deaktiviert.'
    else error.value = r.error
    return r
  }

  async function grant(tenantId, feedId) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/subscriptions`, {
      method: 'POST',
      body: JSON.stringify({ feed_id: feedId }),
    })
    if (r.ok) notice.value = 'Feed zugewiesen.'
    else error.value = r.error
    return r
  }

  async function revoke(tenantId, feedId) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/subscriptions/${feedId}`, {
      method: 'DELETE',
    })
    if (r.ok) notice.value = 'Zuweisung entfernt.'
    else error.value = r.error
    return r
  }

  // --- access management (AP6) ----------------------------------------------
  // Per-tenant access accounts (role user). The server enforces every boundary
  // (requireAdmin → 403); the UI gating is convenience only.

  // loadTenantUsers returns a tenant's access accounts without storing them
  // globally (the caller owns that transient list, like loadTenantSubscriptions).
  async function loadTenantUsers(tenantId) {
    return apiFetch(`/api/admin/tenants/${tenantId}/users`)
  }

  async function createUser(tenantId, payload) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/users`, {
      method: 'POST',
      body: JSON.stringify(payload),
    })
    if (r.ok) notice.value = 'Zugang angelegt.'
    else error.value = r.error
    return r
  }

  async function setUserStatus(tenantId, userId, status) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/users/${userId}`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    })
    if (r.ok) notice.value = status === 'paused' ? 'Zugang pausiert.' : 'Zugang reaktiviert.'
    else error.value = r.error
    return r
  }

  async function deleteUser(tenantId, userId) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/users/${userId}`, { method: 'DELETE' })
    if (r.ok) notice.value = 'Zugang gelöscht.'
    else error.value = r.error
    return r
  }

  async function setUserPassword(tenantId, userId, password) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/users/${userId}/password`, {
      method: 'PUT',
      body: JSON.stringify({ password }),
    })
    if (r.ok) notice.value = 'Passwort gesetzt.'
    else error.value = r.error
    return r
  }

  async function setTenantStatus(tenantId, status) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    })
    if (r.ok) notice.value = status === 'paused' ? 'Mandant pausiert.' : 'Mandant reaktiviert.'
    else error.value = r.error
    return r
  }

  // changeOwnPassword changes the logged-in principal's own password (ONB-1,
  // ADR 0011). On success it reloads the identity so must_change_password flips to
  // false and the dashboard unlocks. The current password is required so a stolen
  // live session cannot silently rotate the credential.
  async function changeOwnPassword(currentPassword, newPassword) {
    error.value = null
    notice.value = null
    const r = await apiFetch('/api/admin/me/password', {
      method: 'PUT',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    })
    if (r.ok) {
      await loadIdentity()
      notice.value = 'Passwort geändert.'
    } else {
      error.value = r.error
    }
    return r
  }

  function clearBanners() {
    error.value = null
    notice.value = null
  }

  return {
    identity, accessError, accessStatus, view, feeds, subscriptions, tenants, overview, feedsHealth, error, notice,
    role, isAdmin, isAuthorized, mustChangePassword, features, hasFeature,
    loadIdentity, login, loadView, saveView, loadFeeds, loadSubscriptions,
    loadTenants, loadTenantSubscriptions, grant, revoke,
    loadOverview, loadFeedsHealth, loadTenantView, saveTenantView, loadTenantEntitlements, setTenantEntitlement,
    loadTenantUsers, createUser, setUserStatus, deleteUser, setUserPassword, setTenantStatus,
    changeOwnPassword, clearBanners,
  }
})
