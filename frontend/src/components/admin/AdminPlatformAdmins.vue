<template>
  <!-- Platform-admin management (ONB-3, ADR 0011). Platform admins are global —
       they belong to no tenant — and are managed here, strictly separate from a
       tenant's access accounts (AdminUsers.vue). The server enforces every
       boundary (requireAdmin → 403) and the "last active admin" guard (409), so
       the gating here is convenience, not security. -->
  <v-card variant="tonal">
    <v-card-title class="d-flex align-center text-subtitle-1">
      Plattform-Administratoren
      <v-spacer />
      <v-btn size="small" variant="text" prepend-icon="mdi-refresh" :loading="busy" @click="refresh">
        Aktualisieren
      </v-btn>
      <v-btn size="small" color="primary" variant="tonal" prepend-icon="mdi-shield-plus" class="ml-2" @click="openCreate">
        Administrator anlegen
      </v-btn>
    </v-card-title>
    <v-card-text>
      <p class="text-body-2 text-medium-emphasis mb-3">
        Administratoren verwalten die gesamte Plattform und sind keinem Mandanten
        zugeordnet. Das Luftlagebild eines Mandanten sehen sie über „Als Mandant
        ansehen“. Der letzte aktive Administrator kann nicht pausiert oder gelöscht
        werden.
      </p>
      <v-table density="comfortable">
        <thead>
          <tr>
            <th>Benutzername</th>
            <th>E-Mail</th>
            <th class="text-right">Status</th>
            <th class="text-right">Aktionen</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!admins.length">
            <td colspan="4" class="text-medium-emphasis">Noch keine Administratoren.</td>
          </tr>
          <tr v-for="a in admins" :key="a.id">
            <td>
              {{ a.subject }}
              <v-chip
                v-if="a.must_change_password"
                size="x-small"
                color="warning"
                variant="tonal"
                class="ml-2"
                title="Dieses Konto muss beim nächsten Login sein Passwort ändern."
              >
                Passwortwechsel nötig
              </v-chip>
            </td>
            <td>{{ a.email || '—' }}</td>
            <td class="text-right">
              <v-chip :color="a.status === 'paused' ? 'warning' : 'success'" size="small" variant="tonal">
                {{ a.status === 'paused' ? 'pausiert' : 'aktiv' }}
              </v-chip>
            </td>
            <td class="text-right">
              <v-btn
                size="small"
                :color="a.status === 'paused' ? 'success' : 'warning'"
                variant="text"
                :loading="busy"
                @click="toggleAdmin(a)"
              >
                {{ a.status === 'paused' ? 'Reaktivieren' : 'Pausieren' }}
              </v-btn>
              <v-btn size="small" variant="text" :loading="busy" @click="openPassword(a)">
                Passwort
              </v-btn>
              <v-btn size="small" color="error" variant="text" :loading="busy" @click="openDelete(a)">
                Löschen
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>

  <!-- Create admin dialog -->
  <v-dialog v-model="createDialog" max-width="460">
    <v-card>
      <v-card-title class="text-subtitle-1">Administrator anlegen</v-card-title>
      <v-card-text>
        <v-text-field v-model="form.subject" label="Benutzername" autofocus class="mb-2" />
        <v-text-field v-model="form.email" label="E-Mail (optional)" class="mb-2" />
        <v-text-field
          v-model="form.password"
          label="Passwort (optional, min. 8 Zeichen)"
          :type="showPassword ? 'text' : 'password'"
          :append-inner-icon="showPassword ? 'mdi-eye-off' : 'mdi-eye'"
          hint="Leer lassen für Proxy-/OIDC-Administratoren ohne lokales Passwort."
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

  <!-- Delete confirmation -->
  <v-dialog v-model="deleteDialog" max-width="420">
    <v-card>
      <v-card-title class="text-subtitle-1">Administrator löschen</v-card-title>
      <v-card-text>
        Administrator <strong>{{ target?.subject }}</strong> endgültig löschen? Diese Aktion kann nicht
        rückgängig gemacht werden. Zum Sperren ohne Datenverlust stattdessen „Pausieren“ verwenden.
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
import { ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

const admin = useAdminStore()
const admins = ref([])
const busy = ref(false)

const createDialog = ref(false)
const passwordDialog = ref(false)
const deleteDialog = ref(false)
const showPassword = ref(false)
const target = ref(null)
const newPassword = ref('')
const form = ref({ subject: '', email: '', password: '' })

async function refresh() {
  busy.value = true
  const r = await admin.loadAdmins()
  admins.value = r.ok ? r.data : []
  busy.value = false
}

function openCreate() {
  form.value = { subject: '', email: '', password: '' }
  showPassword.value = false
  createDialog.value = true
}

async function submitCreate() {
  busy.value = true
  const payload = { subject: form.value.subject.trim() }
  if (form.value.email.trim()) payload.email = form.value.email.trim()
  if (form.value.password) payload.password = form.value.password
  const r = await admin.createAdmin(payload)
  busy.value = false
  if (r.ok) {
    createDialog.value = false
    await refresh()
  }
}

async function toggleAdmin(a) {
  busy.value = true
  const next = a.status === 'paused' ? 'active' : 'paused'
  const r = await admin.setAdminStatus(a.id, next)
  busy.value = false
  if (r.ok) await refresh()
}

function openPassword(a) {
  target.value = a
  newPassword.value = ''
  showPassword.value = false
  passwordDialog.value = true
}

async function submitPassword() {
  busy.value = true
  const r = await admin.setAdminPassword(target.value.id, newPassword.value)
  busy.value = false
  if (r.ok) passwordDialog.value = false
}

function openDelete(a) {
  target.value = a
  deleteDialog.value = true
}

async function submitDelete() {
  busy.value = true
  const r = await admin.deleteAdmin(target.value.id)
  busy.value = false
  if (r.ok) {
    deleteDialog.value = false
    await refresh()
  }
}

onMounted(refresh)
</script>
