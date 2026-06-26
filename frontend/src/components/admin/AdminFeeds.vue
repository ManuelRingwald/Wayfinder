<template>
  <!-- Feed lifecycle management (ONB-5, ADR 0011). The platform admin catalogues
       data sources (Firefly feeds) and the server joins/leaves their multicast
       groups live — no restart. The server enforces every boundary
       (requireAdmin → 403), validates the multicast coordinates and rejects a
       duplicate name (409), so the gating here is convenience, not security. -->
  <v-card variant="tonal">
    <v-card-title class="d-flex align-center text-subtitle-1">
      Feeds
      <v-spacer />
      <v-btn size="small" variant="text" prepend-icon="mdi-refresh" :loading="busy" @click="refresh">
        Aktualisieren
      </v-btn>
      <v-btn size="small" color="primary" variant="tonal" prepend-icon="mdi-plus-network" class="ml-2" @click="openCreate">
        Feed anlegen
      </v-btn>
    </v-card-title>
    <v-card-text>
      <p class="text-body-2 text-medium-emphasis mb-3">
        Feeds sind die Datenquellen (Firefly-Sender). Beim Anlegen tritt der Server
        der Multicast-Gruppe sofort bei, beim Löschen verlässt er sie sofort — ohne
        Neustart. Ein gelöschter Feed wird auch aus den Abos der Mandanten entfernt.
      </p>
      <v-table density="comfortable">
        <thead>
          <tr>
            <th>Name</th>
            <th>Multicast</th>
            <th>Sensoren</th>
            <th class="text-right">Gesundheit</th>
            <th class="text-right">Aktionen</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!admin.feeds.length">
            <td colspan="5" class="text-medium-emphasis">Noch keine Feeds im Katalog.</td>
          </tr>
          <tr v-for="f in admin.feeds" :key="f.id">
            <td>
              <div>{{ f.name }}</div>
              <div v-if="f.region" class="text-caption text-medium-emphasis">{{ f.region }}</div>
            </td>
            <td><code>{{ f.multicast_group }}:{{ f.port }}</code></td>
            <td>
              <span v-if="!f.sensor_mix || !f.sensor_mix.length" class="text-medium-emphasis">—</span>
              <v-chip
                v-for="s in f.sensor_mix"
                :key="s"
                size="x-small"
                variant="tonal"
                class="mr-1 mb-1"
              >
                {{ s }}
              </v-chip>
            </td>
            <td class="text-right">
              <v-chip size="x-small" variant="flat" :color="feedColor(f.id)" :title="feedTitle(f.id)">
                {{ feedLabel(f.id) }}
              </v-chip>
            </td>
            <td class="text-right">
              <v-btn size="small" color="error" variant="text" :loading="busy" @click="openDelete(f)">
                Löschen
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>

  <!-- Create feed dialog -->
  <v-dialog v-model="createDialog" max-width="520">
    <v-card>
      <v-card-title class="text-subtitle-1">Feed anlegen</v-card-title>
      <v-card-text>
        <v-text-field v-model="form.name" label="Name" autofocus class="mb-2" />
        <div class="d-flex ga-3">
          <v-text-field
            v-model="form.multicast_group"
            label="Multicast-Gruppe"
            hint="IPv4-Multicast, z. B. 239.255.0.62"
            persistent-hint
            class="mb-2"
          />
          <v-text-field
            v-model.number="form.port"
            type="number"
            label="Port"
            style="max-width: 130px"
            class="mb-2"
          />
        </div>
        <v-text-field
          v-model="form.sensorMix"
          label="Sensor-Mix (optional)"
          hint="Kommagetrennt, z. B. PSR,SSR,ADS-B — gegen das Vokabular geprüft."
          persistent-hint
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="createDialog = false">Abbrechen</v-btn>
        <v-btn color="primary" :loading="busy" :disabled="!canCreate" @click="submitCreate">Anlegen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Delete confirmation -->
  <v-dialog v-model="deleteDialog" max-width="460">
    <v-card>
      <v-card-title class="text-subtitle-1">Feed löschen</v-card-title>
      <v-card-text>
        <p class="mb-2">
          Feed <strong>{{ target?.name }}</strong> (<code>{{ target?.multicast_group }}:{{ target?.port }}</code>)
          endgültig löschen? Der Server verlässt die Multicast-Gruppe sofort.
        </p>
        <v-alert type="info" variant="tonal" density="compact">
          Mandanten, die diesen Feed abonniert haben, verlieren ihn — die Zuweisungen
          werden automatisch entfernt.
        </v-alert>
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
import { ref, computed, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

const admin = useAdminStore()
const busy = ref(false)

const createDialog = ref(false)
const deleteDialog = ref(false)
const target = ref(null)
const form = ref({ name: '', multicast_group: '', port: 8600, sensorMix: '' })

const canCreate = computed(
  () => form.value.name.trim() !== '' && form.value.multicast_group.trim() !== '' && form.value.port > 0,
)

async function refresh() {
  busy.value = true
  await Promise.all([admin.loadFeeds(), admin.loadFeedsHealth()])
  busy.value = false
}

function openCreate() {
  form.value = { name: '', multicast_group: '', port: 8600, sensorMix: '' }
  createDialog.value = true
}

async function submitCreate() {
  busy.value = true
  const payload = {
    name: form.value.name.trim(),
    multicast_group: form.value.multicast_group.trim(),
    port: form.value.port,
  }
  const mix = form.value.sensorMix
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s !== '')
  if (mix.length) payload.sensor_mix = mix
  const r = await admin.createFeed(payload)
  busy.value = false
  if (r.ok) {
    createDialog.value = false
    await refresh()
  }
}

function openDelete(f) {
  target.value = f
  deleteDialog.value = true
}

async function submitDelete() {
  busy.value = true
  const r = await admin.deleteFeed(target.value.id)
  busy.value = false
  if (r.ok) {
    deleteDialog.value = false
    await refresh()
  }
}

// Health chip (AP4): map the per-feed snapshot colour to a Vuetify colour and a
// human title; a feed with no snapshot yet shows a neutral "unbekannt".
const HEALTH_COLORS = { green: 'success', yellow: 'warning', red: 'error' }

function feedColor(feedId) {
  return HEALTH_COLORS[admin.feedsHealth[feedId]?.color] ?? 'default'
}

function feedLabel(feedId) {
  const c = admin.feedsHealth[feedId]?.color
  if (c === 'green') return 'OK'
  if (c === 'yellow') return 'degradiert'
  if (c === 'red') return 'inaktiv'
  return 'unbekannt'
}

function feedTitle(feedId) {
  const h = admin.feedsHealth[feedId]
  if (!h) return 'Gesundheit unbekannt'
  if (h.color === 'green') {
    return h.track_count_recent > 0 ? `OK · ${h.track_count_recent} Tracks` : 'OK · leerer Himmel'
  }
  if (h.color === 'yellow') {
    return h.sensors_total > 0
      ? `Sensor-Teilausfall: ${h.sensors_active} von ${h.sensors_total} Radaren aktiv`
      : 'Sensor-Teilausfall'
  }
  return 'Feed inaktiv (kein Heartbeat)'
}

onMounted(refresh)
</script>
