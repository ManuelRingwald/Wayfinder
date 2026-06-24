<template>
  <!-- Cross-tenant read-only impersonation control (ADR 0008, WF2-34).
       super_admin only. When inactive it is an unobtrusive entry ("Als Mandant
       ansehen"); when active it becomes a prominent read-only banner with a
       tenant switcher and an exit. The server enforces the actual scope. -->
  <div v-if="admin.isSuperAdmin" class="imp-bar" :class="{ 'imp-bar--active': imp.active }">
    <template v-if="imp.active">
      <v-icon size="18">mdi-eye-outline</v-icon>
      <span class="imp-bar__text">
        Sie betrachten <strong>{{ activeName }}</strong> — nur Lesen
      </span>

      <v-menu v-if="otherTenants.length">
        <template #activator="{ props }">
          <v-btn v-bind="props" size="small" variant="text" append-icon="mdi-menu-down">
            Mandant wechseln
          </v-btn>
        </template>
        <v-list density="compact">
          <v-list-item
            v-for="t in otherTenants"
            :key="t.id"
            :title="t.name"
            @click="imp.start(t.id)"
          />
        </v-list>
      </v-menu>

      <v-btn size="small" variant="flat" color="surface" @click="imp.stop()">
        Beenden
      </v-btn>
    </template>

    <template v-else>
      <v-menu>
        <template #activator="{ props }">
          <v-btn
            v-bind="props"
            size="small"
            variant="tonal"
            prepend-icon="mdi-account-eye-outline"
            append-icon="mdi-menu-down"
          >
            Als Mandant ansehen
          </v-btn>
        </template>
        <v-list density="compact">
          <v-list-subheader v-if="!admin.tenants.length">Keine Mandanten</v-list-subheader>
          <v-list-item
            v-for="t in admin.tenants"
            :key="t.id"
            :title="t.name"
            @click="imp.start(t.id)"
          />
        </v-list>
      </v-menu>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'
import { useImpersonationStore } from '@/stores/impersonation.js'

const admin = useAdminStore()
const imp = useImpersonationStore()

onMounted(async () => {
  // Probe identity once (fail-closed: a non-super_admin renders nothing). Only a
  // super_admin loads the tenant list and the current impersonation status.
  if (!admin.isAuthorized) await admin.loadIdentity()
  if (admin.isSuperAdmin) {
    await admin.loadTenants()
    await imp.loadStatus()
  }
})

function tenantName(id) {
  const t = admin.tenants.find((x) => x.id === id)
  return t ? t.name : `Mandant ${id}`
}
const activeName = computed(() => tenantName(imp.tenantId))
const otherTenants = computed(() => admin.tenants.filter((t) => t.id !== imp.tenantId))
</script>

<style scoped>
.imp-bar {
  position: absolute;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 20;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 6px 4px 12px;
  border-radius: 8px;
  background: rgba(var(--v-theme-surface), 0.92);
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.35);
}

/* Active read-only state: a prominent warning banner (no coloured viewport
   frame — that was deliberately declined; the banner alone carries the mode). */
.imp-bar--active {
  background: rgb(var(--v-theme-warning));
  color: #1a1200;
}
.imp-bar--active :deep(.v-btn) {
  color: #1a1200;
}

.imp-bar__text {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}
</style>
