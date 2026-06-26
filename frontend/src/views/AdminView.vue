<template>
  <!-- WF2-32 admin dashboard. This is a standalone view at '/admin' — the ASD map
       is unmounted while it is shown. The role probe (whoami) runs on mount; until
       it resolves we show a spinner, then either the access notice or the panels.
       All tabs are available to the single admin role (ADR 0009). -->
  <v-app-bar density="comfortable" flat color="surface">
    <v-app-bar-title>Wayfinder — Administration</v-app-bar-title>
    <v-spacer />
    <v-chip
      v-if="admin.isAuthorized"
      size="small"
      color="primary"
      variant="tonal"
      class="mr-3"
    >
      {{ admin.identity.subject || 'admin' }} · {{ admin.role }}
    </v-chip>
    <v-btn prepend-icon="mdi-radar" :to="{ name: 'asd' }">Zur Lage</v-btn>
  </v-app-bar>

  <v-main>
    <v-container class="py-6" style="max-width: 1100px">
      <div v-if="!loaded" class="d-flex justify-center pa-12">
        <v-progress-circular indeterminate color="primary" />
      </div>

      <!-- 401: not logged in → show login form -->
      <v-card
        v-else-if="admin.accessStatus === 401"
        class="mx-auto mt-8"
        max-width="420"
        variant="outlined"
      >
        <v-card-title class="pa-4 pb-0">Anmelden</v-card-title>
        <v-card-text>
          <v-alert
            v-if="loginError"
            type="error"
            variant="tonal"
            density="compact"
            class="mb-4"
          >{{ loginError }}</v-alert>
          <v-form @submit.prevent="submitLogin">
            <v-text-field
              v-model="loginSubject"
              label="Benutzername"
              autocomplete="username"
              autofocus
              class="mb-2"
            />
            <v-text-field
              v-model="loginPassword"
              label="Passwort"
              :type="showPassword ? 'text' : 'password'"
              autocomplete="current-password"
              :append-inner-icon="showPassword ? 'mdi-eye-off' : 'mdi-eye'"
              @click:append-inner="showPassword = !showPassword"
              class="mb-4"
            />
            <v-btn
              type="submit"
              color="primary"
              :loading="loginLoading"
              block
            >Anmelden</v-btn>
          </v-form>
        </v-card-text>
      </v-card>

      <!-- 403 or other error: no login form, just an access notice -->
      <v-alert
        v-else-if="admin.accessError"
        type="error"
        variant="tonal"
        title="Kein Zugriff auf die Administration"
      >
        {{ admin.accessError }} —
        <router-link :to="{ name: 'asd' }">zur Lage zurück</router-link>.
      </v-alert>

      <!-- ONB-1 (ADR 0011): a freshly seeded admin (default credential) must
           replace its password before reaching the dashboard. The server enforces
           this independently (403 password_change_required on every other route);
           this mask is the only path forward until the change succeeds. -->
      <v-card
        v-else-if="admin.isAuthorized && admin.mustChangePassword"
        class="mx-auto mt-8"
        max-width="460"
        variant="outlined"
      >
        <v-card-title class="pa-4 pb-0">Passwort ändern erforderlich</v-card-title>
        <v-card-subtitle class="pa-4 pt-1">
          Dieses Konto verwendet noch das Standard-Passwort. Bitte vergeben Sie
          ein eigenes Passwort, um fortzufahren.
        </v-card-subtitle>
        <v-card-text>
          <v-alert
            v-if="pwError"
            type="error"
            variant="tonal"
            density="compact"
            class="mb-4"
          >{{ pwError }}</v-alert>
          <v-form @submit.prevent="submitPasswordChange">
            <v-text-field
              v-model="pwCurrent"
              label="Aktuelles Passwort"
              type="password"
              autocomplete="current-password"
              autofocus
              class="mb-2"
            />
            <v-text-field
              v-model="pwNew"
              label="Neues Passwort (min. 8 Zeichen)"
              type="password"
              autocomplete="new-password"
              class="mb-2"
            />
            <v-text-field
              v-model="pwConfirm"
              label="Neues Passwort wiederholen"
              type="password"
              autocomplete="new-password"
              class="mb-4"
            />
            <v-btn
              type="submit"
              color="primary"
              :loading="pwLoading"
              block
            >Passwort ändern</v-btn>
          </v-form>
        </v-card-text>
      </v-card>

      <template v-else-if="admin.isAuthorized">
        <v-alert
          v-if="admin.error"
          type="error"
          variant="tonal"
          closable
          class="mb-3"
          @click:close="admin.clearBanners()"
        >
          {{ admin.error }}
        </v-alert>
        <v-alert
          v-if="admin.notice"
          type="success"
          variant="tonal"
          closable
          class="mb-3"
          @click:close="admin.clearBanners()"
        >
          {{ admin.notice }}
        </v-alert>

        <!-- AP3 (ADR 0009): tenant-centric admin. The overview lists all tenants;
             selecting one opens its central configuration page. The old per-tab
             layout (own view / subscriptions / provisioning / users) is replaced
             by this master/detail flow. -->
        <AdminTenantDetail
          v-if="selectedTenant !== null"
          :tenant-id="selectedTenant"
          @back="selectedTenant = null"
        />
        <AdminTenants v-else @select="selectedTenant = $event" />
      </template>
    </v-container>
  </v-main>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'
import AdminTenants from '@/components/admin/AdminTenants.vue'
import AdminTenantDetail from '@/components/admin/AdminTenantDetail.vue'

const admin = useAdminStore()
const selectedTenant = ref(null) // null = overview; a tenant id = detail page
const loaded = ref(false)

const loginSubject = ref('')
const loginPassword = ref('')
const loginLoading = ref(false)
const loginError = ref(null)
const showPassword = ref(false)

// ONB-1 (ADR 0011): forced password-change mask state.
const pwCurrent = ref('')
const pwNew = ref('')
const pwConfirm = ref('')
const pwLoading = ref(false)
const pwError = ref(null)

onMounted(async () => {
  await admin.loadIdentity()
  loaded.value = true
})

async function submitLogin() {
  loginError.value = null
  loginLoading.value = true
  const r = await admin.login(loginSubject.value, loginPassword.value)
  loginLoading.value = false
  if (r.ok) {
    await admin.loadIdentity()
  } else {
    loginError.value = 'Benutzername oder Passwort ungültig.'
  }
}

async function submitPasswordChange() {
  pwError.value = null
  if (pwNew.value.length < 8) {
    pwError.value = 'Das neue Passwort muss mindestens 8 Zeichen lang sein.'
    return
  }
  if (pwNew.value !== pwConfirm.value) {
    pwError.value = 'Die Passwörter stimmen nicht überein.'
    return
  }
  pwLoading.value = true
  const r = await admin.changeOwnPassword(pwCurrent.value, pwNew.value)
  pwLoading.value = false
  if (!r.ok) {
    pwError.value = r.status === 401
      ? 'Das aktuelle Passwort ist falsch.'
      : (r.error || 'Passwortänderung fehlgeschlagen.')
    return
  }
  // Success: identity reloaded (flag now false); clear the form fields.
  pwCurrent.value = pwNew.value = pwConfirm.value = ''
}
</script>
