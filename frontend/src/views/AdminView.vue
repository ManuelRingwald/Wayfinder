<template>
  <!-- WF2-32 admin dashboard. This is a standalone view at '/admin' — the ASD map
       is unmounted while it is shown. The role probe (whoami) runs on mount; until
       it resolves we show a spinner, then either the access notice or the panels.
       The Provisioning tab is gated to super_admin (cosmetic — the server enforces
       it independently with a 403). -->
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

      <v-alert
        v-else-if="admin.accessError"
        type="error"
        variant="tonal"
        title="Kein Zugriff auf die Administration"
      >
        {{ admin.accessError }} —
        <router-link :to="{ name: 'asd' }">zur Lage zurück</router-link>.
      </v-alert>

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

        <v-tabs v-model="tab" color="primary" class="mb-4">
          <v-tab value="view">Ansicht</v-tab>
          <v-tab value="subs">Abos &amp; Feeds</v-tab>
          <v-tab v-if="admin.isSuperAdmin" value="provisioning">Provisioning</v-tab>
        </v-tabs>

        <v-window v-model="tab">
          <v-window-item value="view">
            <AdminViewConfig />
          </v-window-item>
          <v-window-item value="subs">
            <AdminSubscriptions />
          </v-window-item>
          <v-window-item v-if="admin.isSuperAdmin" value="provisioning">
            <AdminProvisioning />
          </v-window-item>
        </v-window>
      </template>
    </v-container>
  </v-main>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'
import AdminViewConfig from '@/components/admin/AdminViewConfig.vue'
import AdminSubscriptions from '@/components/admin/AdminSubscriptions.vue'
import AdminProvisioning from '@/components/admin/AdminProvisioning.vue'

const admin = useAdminStore()
const tab = ref('view')
const loaded = ref(false)

onMounted(async () => {
  await admin.loadIdentity()
  loaded.value = true
})
</script>
