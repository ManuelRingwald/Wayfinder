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
// REST API; the role gating it exposes (isSuperAdmin) is cosmetic — the server
// independently enforces every boundary (requireSuper → 403).
export const useAdminStore = defineStore('admin', () => {
  const identity = ref(null)     // whoami: { subject, tenant_id, user_id, role }
  const accessError = ref(null)  // set when whoami is refused (401/403)
  const view = ref(null)         // the tenant's effective view config (or null)
  const feeds = ref([])          // full feed catalogue
  const subscriptions = ref([])  // feeds the current tenant is subscribed to
  const tenants = ref([])        // super_admin: all tenants
  const error = ref(null)        // last action error (banner)
  const notice = ref(null)       // last success message (banner)

  const role = computed(() => identity.value?.role ?? null)
  const isSuperAdmin = computed(() => role.value === 'super_admin')
  const isAuthorized = computed(() => identity.value !== null)
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
    } else {
      identity.value = null
      accessError.value = r.error
    }
    return r.ok
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

  // --- super_admin cross-tenant provisioning -------------------------------

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

  function clearBanners() {
    error.value = null
    notice.value = null
  }

  return {
    identity, accessError, view, feeds, subscriptions, tenants, error, notice,
    role, isSuperAdmin, isAuthorized, features, hasFeature,
    loadIdentity, loadView, saveView, loadFeeds, loadSubscriptions,
    loadTenants, loadTenantSubscriptions, grant, revoke, clearBanners,
  }
})
