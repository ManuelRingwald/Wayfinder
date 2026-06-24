import { defineStore } from 'pinia'
import { ref } from 'vue'

// useImpersonationStore backs the cross-tenant read-only "View as Tenant X"
// banner (ADR 0008, WF2-34). The grant itself lives in an HttpOnly cookie the
// browser sends on every request — including the /ws handshake — so this store
// holds only the *advisory* UI state (mirrored from the server) plus the actions
// that mint/clear the grant. The server is always the enforcement point.
//
// After any state-changing action it bumps reconnectNonce; the map watches that
// and reconnects the WebSocket so the new scope (target tenant, or back to the
// real one) takes effect immediately. loadStatus deliberately does NOT bump it —
// a freshly loaded page already opens its /ws with the current cookie.
export const useImpersonationStore = defineStore('impersonation', () => {
  const active = ref(false)
  const tenantId = ref(null) // the target tenant being viewed (when active)
  const error = ref(null)
  const reconnectNonce = ref(0)

  async function call(method, body) {
    try {
      return await fetch('/api/admin/impersonation', {
        method,
        headers: body ? { 'Content-Type': 'application/json' } : {},
        body: body ? JSON.stringify(body) : undefined,
      })
    } catch (e) {
      return { ok: false, status: 0, _networkError: e }
    }
  }

  // loadStatus mirrors the server's view of the current grant. It survives reloads
  // (the cookie is not readable by JS) and is the source of truth for the banner.
  async function loadStatus() {
    const res = await call('GET')
    if (!res.ok) {
      active.value = false
      tenantId.value = null
      return
    }
    let data = {}
    try { data = await res.json() } catch { data = {} }
    active.value = data.active === true
    tenantId.value = active.value ? data.tenant_id : null
  }

  // start views a target tenant read-only: the server mints the grant cookie; we
  // reflect it and trigger a reconnect so the stream switches to that tenant.
  async function start(targetTenantId) {
    error.value = null
    const res = await call('POST', { tenant_id: targetTenantId })
    if (!res.ok) {
      error.value = `Mandant ${targetTenantId} konnte nicht angesehen werden (HTTP ${res.status}).`
      return false
    }
    active.value = true
    tenantId.value = targetTenantId
    reconnectNonce.value++
    return true
  }

  // stop exits impersonation (clears the cookie) and reconnects to the real scope.
  // The local state is cleared even on a non-2xx response so the operator is never
  // trapped in the banner; the next loadStatus reconciles any residual cookie.
  async function stop() {
    error.value = null
    const res = await call('DELETE')
    active.value = false
    tenantId.value = null
    reconnectNonce.value++
    return res.ok
  }

  return { active, tenantId, error, reconnectNonce, loadStatus, start, stop }
})
