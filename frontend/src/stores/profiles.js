import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiFetch } from '@/api.js'
import { useAsdStore } from '@/stores/asd.js'
import { captureSettings, applySettings } from '@/stores/profileSettings.js'

// MAX_PROFILES mirrors the server-side cap (store.MaxViewProfilesPerUser) so the
// UI can disable "save" once three profiles exist; the server stays authoritative.
export const MAX_PROFILES = 3

// useProfilesStore holds the user's view profiles (VP-3, ADR 0023) and drives the
// per-user API. Capturing/applying the ASD display prefs goes through the pure
// helpers in profileSettings.js so the map follows via the existing watchers.
export const useProfilesStore = defineStore('profiles', () => {
  const list = ref([]) // [{ id, name, settings, is_default, updated_at }]
  const activeId = ref(null) // last applied profile (for UI highlight); not persisted
  const loading = ref(false)
  const error = ref('')
  // VP-5: whether the login default has already been auto-applied this app load.
  // Guards apply-on-login so it runs at most once and never overrides the operator's
  // later manual choice within the session.
  const defaultApplied = ref(false)

  const canCreate = computed(() => list.value.length < MAX_PROFILES)
  const defaultProfile = computed(() => list.value.find((p) => p.is_default) ?? null)

  // load fetches the user's profiles. Returns true on success.
  async function load() {
    loading.value = true
    error.value = ''
    const r = await apiFetch('/api/view-profiles')
    loading.value = false
    if (!r.ok) {
      error.value = r.error
      return false
    }
    list.value = Array.isArray(r.data) ? r.data : []
    return true
  }

  // saveCurrent captures the current ASD display prefs into a new named profile.
  async function saveCurrent(name, makeDefault = false) {
    const settings = captureSettings(useAsdStore())
    const r = await apiFetch('/api/view-profiles', {
      method: 'POST',
      body: JSON.stringify({ name, settings, make_default: makeDefault }),
    })
    if (!r.ok) {
      error.value = r.error
      return false
    }
    list.value = [...list.value, r.data]
    if (r.data?.is_default) markDefaultLocally(r.data.id)
    return true
  }

  // update replaces a profile's name and settings (PUT replaces both).
  async function update(id, name, settings) {
    const r = await apiFetch(`/api/view-profiles/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ name, settings }),
    })
    if (!r.ok) {
      error.value = r.error
      return false
    }
    list.value = list.value.map((p) => (p.id === id ? r.data : p))
    return true
  }

  // rename changes only the name (re-sends the stored settings, since PUT is a
  // full replace).
  async function rename(id, name) {
    const p = list.value.find((x) => x.id === id)
    if (!p) return false
    return update(id, name, p.settings)
  }

  // overwrite re-captures the current view into an existing profile (keeps its name).
  async function overwrite(id) {
    const p = list.value.find((x) => x.id === id)
    if (!p) return false
    return update(id, p.name, captureSettings(useAsdStore()))
  }

  // remove deletes a profile.
  async function remove(id) {
    const r = await apiFetch(`/api/view-profiles/${id}`, { method: 'DELETE' })
    if (!r.ok) {
      error.value = r.error
      return false
    }
    list.value = list.value.filter((p) => p.id !== id)
    if (activeId.value === id) activeId.value = null
    return true
  }

  // setDefault marks a profile as the login default (server clears the previous one).
  async function setDefault(id) {
    const r = await apiFetch(`/api/view-profiles/${id}/default`, { method: 'POST' })
    if (!r.ok) {
      error.value = r.error
      return false
    }
    markDefaultLocally(id)
    return true
  }

  // apply writes a profile's settings onto the ASD store (map updates via the
  // existing MapCanvas watchers). Returns false for an unknown id.
  function apply(id) {
    const p = list.value.find((x) => x.id === id)
    if (!p) return false
    applySettings(useAsdStore(), p.settings)
    activeId.value = id
    return true
  }

  // applyDefaultOnce applies the login-default profile exactly once per app load
  // (VP-5). Returns true when it applied a default. It does NOT mark itself done
  // when there is no default yet, so it can be retried once the list has loaded;
  // once it applies (or if there is genuinely a default), the guard latches.
  function applyDefaultOnce() {
    if (defaultApplied.value) return false
    const d = list.value.find((p) => p.is_default)
    if (!d) return false
    defaultApplied.value = true
    applySettings(useAsdStore(), d.settings)
    activeId.value = d.id
    return true
  }

  function markDefaultLocally(id) {
    list.value = list.value.map((p) => ({ ...p, is_default: p.id === id }))
  }

  return {
    list,
    activeId,
    loading,
    error,
    defaultApplied,
    canCreate,
    defaultProfile,
    load,
    saveCurrent,
    update,
    rename,
    overwrite,
    remove,
    setDefault,
    apply,
    applyDefaultOnce,
  }
})
