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
      <!-- ONB-4 (ADR 0011): create a tenant from the UI. -->
      <v-btn size="small" color="primary" variant="tonal" prepend-icon="mdi-domain-plus" class="ml-2" @click="openCreate">
        Mandant anlegen
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
              <span v-for="f in t.feeds" :key="f.id" class="d-inline-flex align-center mr-1 mb-1">
                <v-chip
                  size="x-small"
                  variant="tonal"
                  :color="feedColor(f.id)"
                  :title="feedTitle(f.id)"
                >
                  {{ f.name }}
                </v-chip>
              </span>
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

  <!-- Create tenant dialog (ONB-4) -->
  <v-dialog v-model="createDialog" max-width="460">
    <v-card>
      <v-card-title class="text-subtitle-1">Mandant anlegen</v-card-title>
      <v-card-text>
        <v-text-field
          v-model="form.slug"
          label="Slug (Kennung)"
          hint="Kleinbuchstaben, Ziffern und Bindestriche; eindeutig und URL-sicher (z. B. kunde-nord)."
          persistent-hint
          autofocus
          class="mb-3"
        />
        <v-text-field
          v-model="form.name"
          label="Anzeigename (optional)"
          hint="Leer lassen, um den Slug als Namen zu verwenden."
          persistent-hint
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="createDialog = false">Abbrechen</v-btn>
        <v-btn color="primary" :loading="loading" :disabled="!form.slug.trim()" @click="submitCreate">Anlegen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

defineEmits(['select'])

const admin = useAdminStore()
const loading = ref(false)

// ONB-4: create-tenant dialog state.
const createDialog = ref(false)
const form = ref({ slug: '', name: '' })

function openCreate() {
  form.value = { slug: '', name: '' }
  createDialog.value = true
}

async function submitCreate() {
  loading.value = true
  const payload = { slug: form.value.slug.trim() }
  if (form.value.name.trim()) payload.name = form.value.name.trim()
  const r = await admin.createTenant(payload)
  loading.value = false
  if (r.ok) {
    createDialog.value = false
    await refresh()
  }
}

const HEALTH_COLORS = { green: 'success', yellow: 'warning', red: 'error' }

function feedColor(feedId) {
  return HEALTH_COLORS[admin.feedsHealth[feedId]?.color] ?? 'default'
}

function feedTitle(feedId) {
  const h = admin.feedsHealth[feedId]
  if (!h) return 'Gesundheit unbekannt'
  if (h.color === 'green') {
    return h.track_count_recent > 0
      ? `OK · ${h.track_count_recent} Tracks`
      : 'OK · leerer Himmel'
  }
  if (h.color === 'yellow') {
    return h.sensors_total > 0
      ? `Sensor-Teilausfall: ${h.sensors_active} von ${h.sensors_total} Radaren aktiv`
      : 'Sensor-Teilausfall'
  }
  return 'Feed inaktiv (kein Heartbeat)'
}

async function refresh() {
  loading.value = true
  await Promise.all([admin.loadOverview(), admin.loadFeedsHealth()])
  loading.value = false
}

onMounted(refresh)
</script>
