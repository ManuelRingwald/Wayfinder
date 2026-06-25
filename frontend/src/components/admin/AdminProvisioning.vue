<template>
  <!-- super_admin cross-tenant provisioning (WF2-31b/32). Pick a tenant, then
       grant/revoke catalogue feeds for it. This tab is only rendered for
       super_admin; the server independently enforces the boundary (requireSuper →
       403), so the gating here is convenience, not security. -->
  <!-- Standalone tenant picker: hidden when embedded (AP3 detail page passes a
       tenantId prop and owns the tenant context). -->
  <v-card v-if="!tenantId" variant="tonal" class="mb-4">
    <v-card-text>
      <v-select
        v-model="selectedTenant"
        :items="admin.tenants"
        item-title="name"
        item-value="id"
        label="Tenant auswählen"
        prepend-inner-icon="mdi-domain"
        hide-details
        @update:model-value="refreshTenantSubs"
      />
    </v-card-text>
  </v-card>

  <v-card v-if="effectiveTenant" variant="tonal">
    <v-card-title class="text-subtitle-1">Feed-Zuweisungen</v-card-title>
    <v-card-text>
      <v-table density="comfortable">
        <thead>
          <tr><th>Feed</th><th>Region</th><th class="text-right">Status</th><th class="text-right">Aktion</th></tr>
        </thead>
        <tbody>
          <tr v-for="f in admin.feeds" :key="f.id">
            <td>{{ f.name }}</td>
            <td>{{ f.region || '—' }}</td>
            <td class="text-right">
              <v-chip v-if="subscribedIds.has(f.id)" size="small" color="success" variant="tonal">zugewiesen</v-chip>
              <v-chip v-else size="small" variant="tonal">—</v-chip>
            </td>
            <td class="text-right">
              <v-btn
                v-if="subscribedIds.has(f.id)"
                size="small"
                color="error"
                variant="text"
                :loading="busy"
                @click="revoke(f)"
              >
                Entziehen
              </v-btn>
              <v-btn
                v-else
                size="small"
                color="primary"
                variant="text"
                :loading="busy"
                @click="grant(f)"
              >
                Zuweisen
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

// tenantId: when set (AP3 detail page), the component drops its own tenant picker
// and provisions feeds for that tenant. When null (standalone tab), the user
// picks a tenant from the dropdown.
const props = defineProps({
  tenantId: { type: Number, default: null },
})

const admin = useAdminStore()
const selectedTenant = ref(null)
const tenantSubs = ref([])
const busy = ref(false)

const effectiveTenant = computed(() => props.tenantId ?? selectedTenant.value)
const subscribedIds = computed(() => new Set(tenantSubs.value.map((f) => f.id)))

async function refreshTenantSubs() {
  if (!effectiveTenant.value) {
    tenantSubs.value = []
    return
  }
  const r = await admin.loadTenantSubscriptions(effectiveTenant.value)
  tenantSubs.value = r.ok ? r.data : []
}

async function grant(feed) {
  busy.value = true
  const r = await admin.grant(effectiveTenant.value, feed.id)
  if (r.ok) await refreshTenantSubs()
  busy.value = false
}

async function revoke(feed) {
  busy.value = true
  const r = await admin.revoke(effectiveTenant.value, feed.id)
  if (r.ok) await refreshTenantSubs()
  busy.value = false
}

// When embedded, re-fetch the tenant's grants whenever the target changes.
watch(() => props.tenantId, refreshTenantSubs)

onMounted(async () => {
  if (props.tenantId) {
    await Promise.all([admin.loadFeeds(), refreshTenantSubs()])
  } else {
    await Promise.all([admin.loadTenants(), admin.loadFeeds()])
  }
})
</script>
