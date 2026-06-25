<template>
  <!-- AP3 (ADR 0009): tenant-centric admin overview. One row per tenant with its
       status, enabled features, subscribed feeds and account count — the landing
       page of the redesigned admin area. A row's "Konfigurieren" opens the detail
       page (emitted to the parent). The server enforces every boundary
       (requireAdmin → 403); this view is convenience, not a security control. -->
  <v-card variant="tonal">
    <v-card-title class="d-flex align-center text-subtitle-1">
      Mandanten
      <v-spacer />
      <v-btn size="small" variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="refresh">
        Aktualisieren
      </v-btn>
    </v-card-title>
    <v-card-text>
      <v-table density="comfortable">
        <thead>
          <tr>
            <th>Mandant</th>
            <th class="text-right">Status</th>
            <th>Features</th>
            <th>Feeds</th>
            <th class="text-right">Zugänge</th>
            <th class="text-right">Aktion</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!admin.overview.length">
            <td colspan="6" class="text-medium-emphasis">Keine Mandanten.</td>
          </tr>
          <tr v-for="t in admin.overview" :key="t.id">
            <td>
              <div>{{ t.name }}</div>
              <div class="text-caption text-medium-emphasis">{{ t.slug }}</div>
            </td>
            <td class="text-right">
              <v-chip :color="t.status === 'paused' ? 'warning' : 'success'" size="small" variant="tonal">
                {{ t.status === 'paused' ? 'pausiert' : 'aktiv' }}
              </v-chip>
            </td>
            <td>
              <span v-if="!t.features.length" class="text-medium-emphasis">—</span>
              <v-chip
                v-for="key in t.features"
                :key="key"
                size="x-small"
                variant="tonal"
                color="primary"
                class="mr-1 mb-1"
              >
                {{ key }}
              </v-chip>
            </td>
            <td>
              <span v-if="!t.feeds.length" class="text-medium-emphasis">—</span>
              <v-chip
                v-for="f in t.feeds"
                :key="f.id"
                size="x-small"
                variant="tonal"
                class="mr-1 mb-1"
              >
                {{ f.name }}
              </v-chip>
            </td>
            <td class="text-right">{{ t.user_count }}</td>
            <td class="text-right">
              <v-btn size="small" color="primary" variant="text" @click="$emit('select', t.id)">
                Konfigurieren
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

defineEmits(['select'])

const admin = useAdminStore()
const loading = ref(false)

async function refresh() {
  loading.value = true
  await admin.loadOverview()
  loading.value = false
}

onMounted(refresh)
</script>
