<template>
  <!-- Access management (AP6, ADR 0009). Pick a tenant, then provision and
       suspend its access accounts (role user). This tab is admin-only; the
       server enforces every boundary (requireAdmin → 403), so the gating here
       is convenience, not security. Immediate session termination is AP7 — a
       paused account is blocked at the next login, not mid-session. -->
  <!-- Standalone tenant picker: shown only when not embedded (the AP3 detail page
       passes a tenantId prop and owns the tenant context + status toggle). -->
  <v-card v-if="!tenantId" variant="tonal" class="mb-4">
    <v-card-text class="d-flex align-center ga-4">
      <v-select
        v-model="selectedTenant"
        :items="admin.tenants"
        item-title="name"
        item-value="id"
        label="Mandant auswählen"
        prepend-inner-icon="mdi-domain"
        hide-details
        @update:model-value="refresh"
      />
      <template v-if="tenant">
        <v-chip
          :color="tenant.status === 'paused' ? 'warning' : 'success'"
          variant="tonal"
          size="small"
        >
          Mandant {{ tenant.status === 'paused' ? 'pausiert' : 'aktiv' }}
        </v-chip>
        <v-btn
          size="small"
          :color="tenant.status === 'paused' ? 'success' : 'warning'"
          variant="text"
          :loading="busy"
          @click="toggleTenant"
        >
          {{ tenant.status === 'paused' ? 'Mandant reaktivieren' : 'Mandant pausieren' }}
        </v-btn>
      </template>
    </v-card-text>
  </v-card>

  <v-card v-if="effectiveTenant" variant="tonal">
    <v-card-title class="d-flex align-center text-subtitle-1">
      Zugänge
      <v-spacer />
      <v-btn size="small" color="primary" variant="tonal" prepend-icon="mdi-account-plus" @click="openCreate">
        Zugang anlegen
      </v-btn>
    </v-card-title>
    <v-card-text>
      <p v-if="tenant && tenant.status === 'paused'" class="text-warning text-caption mb-2">
        Der Mandant ist pausiert — alle Zugänge sind unabhängig von ihrem eigenen Status für die Anmeldung gesperrt.
      </p>
      <v-table density="comfortable">
        <thead>
          <tr>
            <th>Benutzername</th>
            <th>E-Mail</th>
            <th class="text-right">Status</th>
            <th class="text-right">Sitzungslimit</th>
            <th class="text-right">Aktionen</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!users.length">
            <td colspan="5" class="text-medium-emphasis">Noch keine Zugänge.</td>
          </tr>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.subject }}</td>
            <td>{{ u.email || '—' }}</td>
            <td class="text-right">
              <v-chip
                :color="u.status === 'paused' ? 'warning' : 'success'"
                size="small"
                variant="tonal"
              >
                {{ u.status === 'paused' ? 'pausiert' : 'aktiv' }}
              </v-chip>
            </td>
            <td class="text-right">
              <span class="text-caption text-medium-emphasis">{{ sessionLimitLabel(u.session_limit) }}</span>
            </td>
            <td class="text-right">
              <v-btn
                size="small"
                :color="u.status === 'paused' ? 'success' : 'warning'"
                variant="text"
                :loading="busy"
                @click="toggleUser(u)"
              >
                {{ u.status === 'paused' ? 'Reaktivieren' : 'Pausieren' }}
              </v-btn>
              <v-btn size="small" variant="text" :loading="busy" @click="openPassword(u)">
                Passwort
              </v-btn>
              <v-btn size="small" variant="text" :loading="busy" @click="openLimit(u)">
                Limit
              </v-btn>
              <v-btn size="small" color="error" variant="text" :loading="busy" @click="openDelete(u)">
                Löschen
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>

  <!-- Create access dialog -->
  <v-dialog v-model="createDialog" max-width="460">
    <v-card>
      <v-card-title class="text-subtitle-1">Zugang anlegen</v-card-title>
      <v-card-text>
        <!-- Surface why a save was refused (most often a globally-unique subject
             clash) instead of failing silently. -->
        <v-alert v-if="createError" type="error" variant="tonal" density="compact" class="mb-3">
          {{ createError }}
        </v-alert>
        <v-text-field v-model="form.subject" label="Benutzername" autofocus class="mb-2" />
        <v-text-field v-model="form.email" label="E-Mail (optional)" class="mb-2" />
        <v-text-field
          v-model="form.password"
          label="Passwort (optional, min. 8 Zeichen)"
          :type="showPassword ? 'text' : 'password'"
          :append-inner-icon="showPassword ? 'mdi-eye-off' : 'mdi-eye'"
          hint="Leer lassen für Proxy-/OIDC-Zugänge ohne lokales Passwort."
          persistent-hint
          @click:append-inner="showPassword = !showPassword"
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="createDialog = false">Abbrechen</v-btn>
        <v-btn color="primary" :loading="busy" :disabled="!form.subject" @click="submitCreate">Anlegen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Password reset dialog -->
  <v-dialog v-model="passwordDialog" max-width="460">
    <v-card>
      <v-card-title class="text-subtitle-1">Passwort setzen</v-card-title>
      <v-card-text>
        <p class="text-body-2 mb-3">Neues Passwort für <strong>{{ target?.subject }}</strong>.</p>
        <v-text-field
          v-model="newPassword"
          label="Neues Passwort (min. 8 Zeichen)"
          :type="showPassword ? 'text' : 'password'"
          :append-inner-icon="showPassword ? 'mdi-eye-off' : 'mdi-eye'"
          autofocus
          @click:append-inner="showPassword = !showPassword"
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="passwordDialog = false">Abbrechen</v-btn>
        <v-btn color="primary" :loading="busy" :disabled="newPassword.length < 8" @click="submitPassword">Setzen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Session-limit dialog (AP7) -->
  <v-dialog v-model="limitDialog" max-width="460">
    <v-card>
      <v-card-title class="text-subtitle-1">Sitzungslimit</v-card-title>
      <v-card-text>
        <p class="text-body-2 mb-3">
          Max. gleichzeitige Sitzungen für <strong>{{ target?.subject }}</strong>.
        </p>
        <v-checkbox
          v-model="limitUseDefault"
          label="Standard verwenden (Deployment-Default)"
          density="compact"
          hide-details
          class="mb-2"
        />
        <v-text-field
          v-model.number="limitValue"
          type="number"
          min="0"
          label="Limit (0 = unbegrenzt)"
          :disabled="limitUseDefault"
          :error-messages="limitError"
          hint="Gilt ab dem nächsten Login; laufende Sitzungen bleiben."
          persistent-hint
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="limitDialog = false">Abbrechen</v-btn>
        <v-btn color="primary" :loading="busy" @click="submitLimit">Speichern</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Delete confirmation -->
  <v-dialog v-model="deleteDialog" max-width="420">
    <v-card>
      <v-card-title class="text-subtitle-1">Zugang löschen</v-card-title>
      <v-card-text>
        Zugang <strong>{{ target?.subject }}</strong> endgültig löschen? Diese Aktion kann nicht rückgängig
        gemacht werden. Zum Sperren ohne Datenverlust stattdessen „Pausieren“ verwenden.
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="deleteDialog = false">Abbrechen</v-btn>
        <v-btn color="error" :loading="busy" @click="submitDelete">Löschen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

// tenantId: when set (AP3 detail page), the component drops its own tenant
// picker and operates on that tenant. When null (standalone tab), the user
// picks a tenant from the dropdown.
const props = defineProps({
  tenantId: { type: Number, default: null },
})

const admin = useAdminStore()
const selectedTenant = ref(null)
const users = ref([])
const busy = ref(false)

// effectiveTenant is the tenant the actions target: the prop if embedded, else
// the picker selection.
const effectiveTenant = computed(() => props.tenantId ?? selectedTenant.value)

const createDialog = ref(false)
const passwordDialog = ref(false)
const deleteDialog = ref(false)
const limitDialog = ref(false)
const showPassword = ref(false)
const target = ref(null)
const newPassword = ref('')
const form = ref({ subject: '', email: '', password: '' })
const createError = ref('')
const limitValue = ref(0)
const limitUseDefault = ref(true)
const limitError = ref('')

// sessionLimitLabel renders a user's per-access session limit (AP7): null =
// deployment default, 0 = unlimited, otherwise the explicit cap.
function sessionLimitLabel(limit) {
  if (limit === null || limit === undefined) return 'Standard'
  if (limit === 0) return 'unbegrenzt'
  return String(limit)
}

const tenant = computed(() => admin.tenants.find((t) => t.id === selectedTenant.value) || null)

async function refresh() {
  if (!effectiveTenant.value) {
    users.value = []
    return
  }
  const r = await admin.loadTenantUsers(effectiveTenant.value)
  users.value = r.ok ? r.data : []
}

// reloadTenants refreshes the tenant list so the tenant status chip reflects a
// just-applied pause/reactivate (the list carries the status field).
async function reloadTenants() {
  await admin.loadTenants()
}

async function toggleTenant() {
  if (!tenant.value) return
  busy.value = true
  const next = tenant.value.status === 'paused' ? 'active' : 'paused'
  await admin.setTenantStatus(selectedTenant.value, next)
  await reloadTenants()
  busy.value = false
}

async function toggleUser(u) {
  busy.value = true
  const next = u.status === 'paused' ? 'active' : 'paused'
  const r = await admin.setUserStatus(effectiveTenant.value, u.id, next)
  if (r.ok) await refresh()
  busy.value = false
}

function openCreate() {
  form.value = { subject: '', email: '', password: '' }
  showPassword.value = false
  createError.value = ''
  createDialog.value = true
}

async function submitCreate() {
  createError.value = ''
  busy.value = true
  const subject = form.value.subject.trim()
  const payload = { subject }
  if (form.value.email.trim()) payload.email = form.value.email.trim()
  if (form.value.password) payload.password = form.value.password
  const r = await admin.createUser(effectiveTenant.value, payload)
  busy.value = false
  if (r.ok) {
    createDialog.value = false
    await refresh()
    return
  }
  // The dialog previously closed on success but did nothing on failure, so a
  // refused save (e.g. a subject already used in ANOTHER tenant — subjects are
  // globally unique) looked like a silent no-op. Explain what went wrong.
  createError.value = createErrorMessage(r, subject)
}

// createErrorMessage turns the API failure into a clear German reason. A 409 is
// the common case — the subject is taken; because subjects are unique across all
// tenants, the clash may live in a different tenant than the one being edited.
function createErrorMessage(r, subject) {
  if (r.status === 409) {
    return `Der Benutzername „${subject}" ist bereits vergeben. Benutzernamen sind `
      + 'mandantenübergreifend eindeutig — er wird evtl. in einem anderen Mandanten verwendet.'
  }
  if (r.status === 400 && /password too short/i.test(r.error || '')) {
    return 'Das Passwort ist zu kurz (mindestens 8 Zeichen).'
  }
  return r.error || 'Der Zugang konnte nicht angelegt werden.'
}

function openPassword(u) {
  target.value = u
  newPassword.value = ''
  showPassword.value = false
  passwordDialog.value = true
}

async function submitPassword() {
  busy.value = true
  const r = await admin.setUserPassword(effectiveTenant.value, target.value.id, newPassword.value)
  busy.value = false
  if (r.ok) passwordDialog.value = false
}

function openLimit(u) {
  target.value = u
  limitError.value = ''
  // null → "use default"; a concrete value (incl. 0) → explicit.
  limitUseDefault.value = u.session_limit === null || u.session_limit === undefined
  limitValue.value = limitUseDefault.value ? 0 : u.session_limit
  limitDialog.value = true
}

async function submitLimit() {
  let limit = null
  if (!limitUseDefault.value) {
    const n = Number(limitValue.value)
    if (!Number.isInteger(n) || n < 0) {
      limitError.value = 'Bitte eine ganze Zahl ≥ 0 angeben (0 = unbegrenzt).'
      return
    }
    limit = n
  }
  limitError.value = ''
  busy.value = true
  const r = await admin.setUserSessionLimit(effectiveTenant.value, target.value.id, limit)
  busy.value = false
  if (r.ok) {
    limitDialog.value = false
    await refresh()
  }
}

function openDelete(u) {
  target.value = u
  deleteDialog.value = true
}

async function submitDelete() {
  busy.value = true
  const r = await admin.deleteUser(effectiveTenant.value, target.value.id)
  busy.value = false
  if (r.ok) {
    deleteDialog.value = false
    await refresh()
  }
}

// When embedded, reload the account list whenever the target tenant changes.
watch(() => props.tenantId, refresh)

onMounted(async () => {
  if (props.tenantId) {
    await refresh() // embedded: tenant context comes from the detail page
  } else {
    await admin.loadTenants() // standalone: populate the picker
  }
})
</script>
