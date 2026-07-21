import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { apiFetch } from '@/api.js'

// #111: transient success notices auto-dismiss after this delay. Errors are
// left sticky on purpose (the operator must acknowledge a failure).
const NOTICE_TIMEOUT_MS = 5000

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

  // #111: auto-dismiss a success notice after NOTICE_TIMEOUT_MS so the "…
  // gespeichert" badges do not linger. Any new notice resets the timer; clearing
  // it (or an error taking over) cancels the pending dismissal.
  let noticeTimer = null
  watch(notice, (val) => {
    if (noticeTimer) { clearTimeout(noticeTimer); noticeTimer = null }
    if (val) {
      noticeTimer = setTimeout(() => { notice.value = null; noticeTimer = null }, NOTICE_TIMEOUT_MS)
    }
  })

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

  // logout clears the server session (wf_session cookie) and resets the store to
  // the unauthenticated state, so the dashboard falls back to its login form.
  async function logout() {
    const r = await apiFetch('/api/logout', { method: 'POST' })
    identity.value = null
    accessError.value = null
    accessStatus.value = 401
    return r
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

  // loadTenantAirspaces lists a target tenant's cached airspaces (id + name +
  // type) for the AoR editor picker (ASD-014). Read-only; the caller reads
  // r.ok / r.data. Empty list when the tenant has no OpenAIP data yet; a 404
  // means the picker route is unavailable (no airspace lister wired).
  async function loadTenantAirspaces(tenantId) {
    return apiFetch(`/api/admin/tenants/${tenantId}/airspaces`)
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

  // searchAirports queries the offline ICAO airport directory for the
  // view-config centre search. Read-only, no notice/error side effects (it runs
  // on every keystroke); the caller reads r.ok / r.data.
  async function searchAirports(q) {
    return apiFetch(`/api/admin/airports?q=${encodeURIComponent(q)}`)
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

  // --- tenant lifecycle (ONB-4, ADR 0011) -----------------------------------
  // Create and delete tenants. The server enforces requireAdmin and guard B
  // (deleting a tenant that still has accounts → 409); the UI gating is
  // convenience only.

  async function createTenant(payload) {
    error.value = null
    notice.value = null
    const r = await apiFetch('/api/admin/tenants', { method: 'POST', body: JSON.stringify(payload) })
    if (r.ok) notice.value = 'Mandant angelegt.'
    else error.value = r.error
    return r
  }

  async function deleteTenant(tenantId) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}`, { method: 'DELETE' })
    if (r.ok) {
      notice.value = 'Mandant gelöscht.'
    } else if (r.status === 409) {
      // Guard B: a tenant with accounts cannot be deleted until they are removed.
      error.value = 'Löschen nicht möglich: Der Mandant hat noch Zugänge. Bitte zuerst alle Zugänge entfernen.'
    } else {
      error.value = r.error
    }
    return r
  }

  // --- feed lifecycle (ONB-5, ADR 0011) -------------------------------------
  // Create and delete catalogue feeds. The server joins/leaves the multicast
  // group live (no restart) and enforces requireAdmin; the UI gating is
  // convenience only. createFeed validates server-side (multicast range, port,
  // duplicate name → 409); deleteFeed cascades the feed's subscriptions away.

  async function createFeed(payload) {
    error.value = null
    notice.value = null
    const r = await apiFetch('/api/admin/feeds', { method: 'POST', body: JSON.stringify(payload) })
    if (r.ok) {
      notice.value = 'Feed angelegt.'
    } else if (r.status === 409) {
      // 409 covers a duplicate name and (manual override) a taken multicast endpoint.
      error.value = /endpoint|multicast/i.test(r.error || '')
        ? 'Anlegen nicht möglich: Dieser Multicast-Endpoint ist bereits belegt.'
        : 'Anlegen nicht möglich: Ein Feed mit diesem Namen existiert bereits.'
    } else if (r.status === 507) {
      // ORCH-4: the auto-allocation pool has no free multicast endpoint left.
      error.value = 'Anlegen nicht möglich: Kein freier Multicast-Endpoint verfügbar (Pool erschöpft).'
    } else {
      error.value = r.error
    }
    return r
  }

  async function deleteFeed(feedId) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/feeds/${feedId}`, { method: 'DELETE' })
    if (r.ok) notice.value = 'Feed gelöscht.'
    else error.value = r.error
    return r
  }

  // --- feed source configuration (ORCH-1b, ADR 0012) ------------------------
  // A feed carries the generic source list (adsb_opensky/flarm_aprs/
  // radar_asterix) the orchestrator will turn into a dedicated Firefly instance,
  // plus the coarse outer coverage bbox. loadFeedSources returns the raw response
  // (the dialog owns the transient form state). saveFeedSources PUTs the config;
  // the server validates (closed vocabulary, per-kind rules → 400 with index) and
  // derives coverage_bbox when omitted — the UI gating is convenience only.

  async function loadFeedSources(feedId) {
    return apiFetch(`/api/admin/feeds/${feedId}/sources`)
  }

  async function saveFeedSources(feedId, payload) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/feeds/${feedId}/sources`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    })
    if (r.ok) notice.value = 'Quellen gespeichert.'
    else error.value = r.error
    return r
  }

  // --- per-feed source credentials (ORCH-2c 3a, ADR 0012 §6) ----------------
  // A source's cred_ref points at a per-feed secret whose value is set here. The
  // route is write-only: loadFeedSecrets reports only which refs are configured
  // (never a value); a 503 means no encryption key is configured server-side
  // (WAYFINDER_SECRET_KEY) and the secret controls stay disabled. setFeedSecret
  // sends the value (sealed at rest by the server); deleteFeedSecret clears it.
  // The cred_ref may contain slashes (e.g. secret/opensky); encodeURI keeps them
  // so the server's {ref...} wildcard captures the full handle.

  async function loadFeedSecrets(feedId) {
    return apiFetch(`/api/admin/feeds/${feedId}/secrets`)
  }

  async function setFeedSecret(feedId, ref, value) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/feeds/${feedId}/secrets/${encodeURI(ref)}`, {
      method: 'PUT',
      body: JSON.stringify({ value }),
    })
    if (r.ok) notice.value = 'Secret gespeichert.'
    else error.value = r.error
    return r
  }

  async function deleteFeedSecret(feedId, ref) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/feeds/${feedId}/secrets/${encodeURI(ref)}`, {
      method: 'DELETE',
    })
    if (r.ok) notice.value = 'Secret entfernt.'
    else error.value = r.error
    return r
  }

  // --- OpenAIP per tenant (ONB-6, ADR 0011) ---------------------------------
  // Each tenant may carry its own OpenAIP key. loadTenantOpenAIP reports only
  // whether a key is set (the server never returns the key itself); the caller
  // owns the transient {configured} state. setTenantOpenAIPKey sets (string) or
  // clears (null) the key — the server (re)applies the per-tenant refresh live.

  async function loadTenantOpenAIP(tenantId) {
    return apiFetch(`/api/admin/tenants/${tenantId}/openaip`)
  }

  async function setTenantOpenAIPKey(tenantId, apiKey) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/openaip`, {
      method: 'PUT',
      body: JSON.stringify({ api_key: apiKey }),
    })
    if (r.ok) notice.value = apiKey ? 'OpenAIP-Schlüssel gespeichert.' : 'OpenAIP-Schlüssel entfernt.'
    else error.value = r.error
    return r
  }

  // --- OpenAIP: refresh + global key (AERO-2, ADR 0018) ---------------------
  // Force a fresh fetch for one tenant (per-tenant "refresh now" button).
  async function refreshTenantOpenAIP(tenantId) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/openaip/refresh`, { method: 'POST' })
    if (r.ok) notice.value = 'OpenAIP-Aktualisierung angestoßen.'
    else error.value = r.error
    return r
  }

  // The platform-wide (global fallback) OpenAIP key. loadGlobalOpenAIP reports only
  // whether a key is stored and whether encryption is available (never the key).
  async function loadGlobalOpenAIP() {
    return apiFetch('/api/admin/openaip')
  }

  // setGlobalOpenAIPKey sets (string) or clears (null) the global key; the server
  // seals it and triggers a fetch-all. Returns the raw result (503 when no cipher).
  async function setGlobalOpenAIPKey(apiKey) {
    error.value = null
    notice.value = null
    const r = await apiFetch('/api/admin/openaip', {
      method: 'PUT',
      body: JSON.stringify({ api_key: apiKey }),
    })
    if (r.ok) notice.value = apiKey ? 'Globaler OpenAIP-Schlüssel gespeichert.' : 'Globaler OpenAIP-Schlüssel entfernt.'
    else error.value = r.error
    return r
  }

  // Force a fresh fetch for every tenant ("refresh all" button).
  async function refreshAllOpenAIP() {
    error.value = null
    notice.value = null
    const r = await apiFetch('/api/admin/openaip/refresh', { method: 'POST' })
    if (r.ok) notice.value = 'OpenAIP-Aktualisierung für alle Mandanten angestoßen.'
    else error.value = r.error
    return r
  }

  // --- AIRAC calendar + change-impact (AERO-3, ADR 0018) --------------------
  // The current AIRAC cycle + next effective date (deterministic, no external data).
  async function loadAirac() {
    return apiFetch('/api/admin/airac')
  }

  // The per-layer change-impact of a tenant's last OpenAIP refresh.
  async function loadTenantOpenAIPChanges(tenantId) {
    return apiFetch(`/api/admin/tenants/${tenantId}/openaip/changes`)
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

  // setUserSessionLimit sets or clears an access's per-access concurrent-session
  // limit (AP7). limit is a non-negative number (0 = unlimited) or null (fall back
  // to the deployment default WAYFINDER_SESSION_LIMIT_DEFAULT). Applies to the next
  // login — it does not evict existing sessions.
  async function setUserSessionLimit(tenantId, userId, limit) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/tenants/${tenantId}/users/${userId}/session-limit`, {
      method: 'PUT',
      body: JSON.stringify({ limit }),
    })
    if (r.ok) notice.value = limit === null ? 'Sitzungslimit auf Standard zurückgesetzt.' : 'Sitzungslimit gesetzt.'
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

  // changeOwnEmail sets the logged-in principal's own contact email (#319) via
  // the role-agnostic self-service endpoint (reachable by admins too — they pass
  // the tenant middleware). On success it reloads the identity so the displayed
  // and prefilled email updates (whoami now carries it); for a tenant user the
  // change also surfaces in the admin access table on its next load.
  async function changeOwnEmail(newEmail) {
    error.value = null
    notice.value = null
    const r = await apiFetch('/api/account/email', {
      method: 'PUT',
      body: JSON.stringify({ email: newEmail }),
    })
    if (r.ok) {
      await loadIdentity()
      notice.value = 'E-Mail-Adresse aktualisiert.'
    } else {
      error.value = r.error
    }
    return r
  }

  // deleteOwnAccount deletes the logged-in principal's own account (ONB-2,
  // ADR 0011). The server refuses with 409 if this is the last active admin.
  // On success the session is effectively terminated — identity is cleared so
  // the next render returns to the login form.
  async function deleteOwnAccount() {
    error.value = null
    notice.value = null
    const r = await apiFetch('/api/admin/me', { method: 'DELETE' })
    if (r.ok) {
      identity.value = null
      accessStatus.value = 401
      accessError.value = null
    } else {
      error.value = r.error
    }
    return r
  }

  // --- platform-admin management (ONB-3, ADR 0011) --------------------------
  // Platform admins are global — they belong to no tenant. The server enforces
  // every boundary (requireAdmin → 403) and the "last active admin" guard (409);
  // the UI gating is convenience only. loadAdmins returns the raw result (the
  // caller owns the transient list, like loadTenantUsers).

  async function loadAdmins() {
    return apiFetch('/api/admin/admins')
  }

  async function createAdmin(payload) {
    error.value = null
    notice.value = null
    const r = await apiFetch('/api/admin/admins', { method: 'POST', body: JSON.stringify(payload) })
    if (r.ok) notice.value = 'Administrator angelegt.'
    else error.value = r.error
    return r
  }

  async function setAdminStatus(adminId, status) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/admins/${adminId}`, { method: 'PATCH', body: JSON.stringify({ status }) })
    if (r.ok) {
      notice.value = status === 'paused' ? 'Administrator pausiert.' : 'Administrator reaktiviert.'
    } else if (r.status === 409) {
      // "Last active admin" guard — pausing the final admin would lock everyone out.
      error.value = 'Pausieren nicht möglich: Das ist der letzte aktive Administrator.'
    } else {
      error.value = r.error
    }
    return r
  }

  async function deleteAdmin(adminId) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/admins/${adminId}`, { method: 'DELETE' })
    if (r.ok) {
      notice.value = 'Administrator gelöscht.'
    } else if (r.status === 409) {
      error.value = 'Löschen nicht möglich: Das ist der letzte aktive Administrator.'
    } else {
      error.value = r.error
    }
    return r
  }

  async function setAdminPassword(adminId, password) {
    error.value = null
    notice.value = null
    const r = await apiFetch(`/api/admin/admins/${adminId}/password`, { method: 'PUT', body: JSON.stringify({ password }) })
    if (r.ok) notice.value = 'Passwort gesetzt.'
    else error.value = r.error
    return r
  }

  function clearBanners() {
    error.value = null
    notice.value = null
  }

  return {
    identity, accessError, accessStatus, view, feeds, subscriptions, tenants, overview, feedsHealth, error, notice,
    role, isAdmin, isAuthorized, mustChangePassword, features, hasFeature,
    loadIdentity, login, logout, loadView, saveView, loadFeeds, loadSubscriptions,
    loadTenants, loadTenantSubscriptions, grant, revoke,
    loadOverview, loadFeedsHealth, loadTenantView, loadTenantAirspaces, saveTenantView, loadTenantEntitlements, setTenantEntitlement,
    searchAirports,
    loadTenantUsers, createUser, setUserStatus, deleteUser, setUserPassword, setUserSessionLimit, setTenantStatus,
    createTenant, deleteTenant,
    createFeed, deleteFeed,
    loadFeedSources, saveFeedSources,
    loadFeedSecrets, setFeedSecret, deleteFeedSecret,
    loadTenantOpenAIP, setTenantOpenAIPKey,
    refreshTenantOpenAIP, loadGlobalOpenAIP, setGlobalOpenAIPKey, refreshAllOpenAIP,
    loadAirac, loadTenantOpenAIPChanges,
    changeOwnPassword, changeOwnEmail, deleteOwnAccount,
    loadAdmins, createAdmin, setAdminStatus, deleteAdmin, setAdminPassword,
    clearBanners,
  }
})
