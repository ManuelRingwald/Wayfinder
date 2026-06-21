<template>
  <!-- Read-only view of the current tenant's bookings and the full catalogue
       (WF2-32). A tenant admin cannot self-provision — granting/revoking feeds is
       a super_admin action (Provisioning tab / WF2-31b). -->
  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1">Gebuchte Feeds</v-card-title>
    <v-card-text>
      <v-table v-if="admin.subscriptions.length" density="comfortable">
        <thead>
          <tr><th>Name</th><th>Region</th><th>Sensorik</th></tr>
        </thead>
        <tbody>
          <tr v-for="f in admin.subscriptions" :key="f.id">
            <td>{{ f.name }}</td>
            <td>{{ f.region || '—' }}</td>
            <td>{{ (f.sensor_mix || []).join(', ') || '—' }}</td>
          </tr>
        </tbody>
      </v-table>
      <p v-else class="text-medium-emphasis">Keine Feeds gebucht.</p>
    </v-card-text>
  </v-card>

  <v-card variant="tonal">
    <v-card-title class="text-subtitle-1">Katalog (alle Feeds)</v-card-title>
    <v-card-text>
      <v-table density="comfortable">
        <thead>
          <tr><th>Name</th><th>Region</th><th>Sensorik</th><th class="text-right">Gebucht</th></tr>
        </thead>
        <tbody>
          <tr v-for="f in admin.feeds" :key="f.id">
            <td>{{ f.name }}</td>
            <td>{{ f.region || '—' }}</td>
            <td>{{ (f.sensor_mix || []).join(', ') || '—' }}</td>
            <td class="text-right">
              <v-icon v-if="subscribedIds.has(f.id)" color="success" icon="mdi-check-circle" />
              <span v-else class="text-medium-emphasis">—</span>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

const admin = useAdminStore()

const subscribedIds = computed(() => new Set(admin.subscriptions.map((f) => f.id)))

onMounted(async () => {
  await Promise.all([admin.loadSubscriptions(), admin.loadFeeds()])
})
</script>
