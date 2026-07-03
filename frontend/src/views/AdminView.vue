<template>
  <!-- WF2-32 admin dashboard. This is a standalone view at '/admin' — the ASD map
       is unmounted while it is shown. The role probe (whoami) runs on mount; until
       it resolves we show a spinner, then either the access notice or the panels.
       All tabs are available to the single admin role (ADR 0009). -->
  <v-app-bar density="comfortable" flat color="surface">
    <v-app-bar-title>Wayfinder — Administration</v-app-bar-title>
    <!-- ONB-3 (ADR 0011): top-level navigation between the tenant world and the
         platform-admin world — the strict separation made visible. Hidden during
         the forced password change (the only reachable action then is the mask). -->
    <v-btn-toggle
      v-if="admin.isAuthorized && !admin.mustChangePassword"
      v-model="section"
      density="comfortable"
      variant="text"
      color="primary"
      mandatory
      class="ml-4"
    >
      <v-btn value="tenants" prepend-icon="mdi-domain">Mandanten</v-btn>
      <v-btn value="feeds" prepend-icon="mdi-access-point-network">Feeds</v-btn>
      <v-btn value="openaip" prepend-icon="mdi-airplane-marker">OpenAIP</v-btn>
      <v-btn value="admins" prepend-icon="mdi-shield-account">Plattform-Administratoren</v-btn>
    </v-btn-toggle>
    <v-spacer />
    <!-- ONB-2 (ADR 0011): chip opens the self-management panel (password change
         + account deletion) for the currently logged-in principal. -->
    <v-chip
      v-if="admin.isAuthorized"
      size="small"
      color="primary"
      variant="tonal"
      class="mr-3"
      style="cursor: pointer"
      append-icon="mdi-account-cog"
      @click="myAccountOpen = true"
    >
      {{ admin.identity.subject || 'admin' }} · {{ admin.role }}
    </v-chip>
    <v-btn prepend-icon="mdi-radar" :to="{ name: 'asd' }">Zur Lage</v-btn>
    <v-btn
      v-if="admin.isAuthorized"
      prepend-icon="mdi-logout"
      variant="text"
      class="ml-1"
      @click="onLogout"
    >Abmelden</v-btn>
  </v-app-bar>

  <MyAccountPanel v-model="myAccountOpen" />

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

        <!-- ONB-3 (ADR 0011): platform-admin management — a separate world from
             the tenant overview, selected via the header navigation. -->
        <AdminPlatformAdmins v-if="section === 'admins'" />

        <!-- ONB-5 (ADR 0011): feed catalogue management — create/delete data
             sources; the server joins/leaves their multicast groups live. -->
        <AdminFeeds v-else-if="section === 'feeds'" />

        <!-- AERO-2 (ADR 0018): platform-wide OpenAIP — the global fallback key
             (sealed) + a fetch-all button for AIRAC updates. -->
        <AdminGlobalOpenAIP v-else-if="section === 'openaip'" />

        <!-- AP3 (ADR 0009): tenant-centric admin. The overview lists all tenants;
             selecting one opens its central configuration page. The old per-tab
             layout (own view / subscriptions / provisioning / users) is replaced
             by this master/detail flow. -->
        <template v-else>
          <AdminTenantDetail
            v-if="selectedTenant !== null"
            :tenant-id="selectedTenant"
            @back="selectedTenant = null"
          />
          <AdminTenants v-else @select="selectedTenant = $event" />
        </template>
      </template>
    </v-container>
  </v-main>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'
import AdminTenants from '@/components/admin/AdminTenants.vue'
import AdminTenantDetail from '@/components/admin/AdminTenantDetail.vue'
import AdminPlatformAdmins from '@/components/admin/AdminPlatformAdmins.vue'
import AdminFeeds from '@/components/admin/AdminFeeds.vue'
import AdminGlobalOpenAIP from '@/components/admin/AdminGlobalOpenAIP.vue'
import MyAccountPanel from '@/components/admin/MyAccountPanel.vue'

const admin = useAdminStore()
const section = ref('tenants') // 'tenants' | 'feeds' | 'openaip' | 'admins' — header navigation (ONB-3/5/AERO-2)
const selectedTenant = ref(null) // null = overview; a tenant id = detail page
const loaded = ref(false)
const myAccountOpen = ref(false) // ONB-2: "Mein Konto" dialog open state

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

// Logout clears the server session; the store drops to accessStatus 401, so the
// view falls back to its login form.
async function onLogout() {
  await admin.logout()
  section.value = 'tenants'
}
</script>
