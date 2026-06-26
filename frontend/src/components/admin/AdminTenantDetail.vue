<template>
  <!-- AP3 (ADR 0009): per-tenant central configuration. One page bundling a
       tenant's status, default view (entered as centre + radius in NM, stored as
       an AOI bbox), feature entitlements, feed grants and access accounts. The
       server enforces every boundary (requireAdmin → 403); this is convenience. -->
  <div class="d-flex align-center mb-4 ga-3">
    <v-btn variant="text" prepend-icon="mdi-arrow-left" @click="$emit('back')">Übersicht</v-btn>
    <div class="text-h6">{{ tenant?.name || ('Mandant #' + tenantId) }}</div>
    <v-chip v-if="tenant" :color="tenant.status === 'paused' ? 'warning' : 'success'" size="small" variant="tonal">
      {{ tenant.status === 'paused' ? 'pausiert' : 'aktiv' }}
    </v-chip>
    <v-spacer />
    <v-btn
      v-if="tenant"
      size="small"
      :color="tenant.status === 'paused' ? 'success' : 'warning'"
      variant="tonal"
      :loading="busy"
      @click="toggleStatus"
    >
      {{ tenant.status === 'paused' ? 'Mandant reaktivieren' : 'Mandant pausieren' }}
    </v-btn>
    <!-- ONB-4 (ADR 0011): delete the tenant. The server refuses (409) while it
         still has accounts; the dialog explains that up front. -->
    <v-btn
      size="small"
      color="error"
      variant="tonal"
      prepend-icon="mdi-delete"
      :loading="busy"
      @click="deleteDialog = true"
    >
      Mandant löschen
    </v-btn>
  </div>

  <!-- Delete tenant confirmation (ONB-4) -->
  <v-dialog v-model="deleteDialog" max-width="480">
    <v-card>
      <v-card-title class="text-subtitle-1">Mandant löschen</v-card-title>
      <v-card-text>
        <p class="mb-2">
          Mandant <strong>{{ tenant?.name || ('#' + tenantId) }}</strong> endgültig löschen?
          Mit dem Mandanten werden auch seine Abos, Features und die Standard-Ansicht entfernt.
          Diese Aktion kann nicht rückgängig gemacht werden.
        </p>
        <v-alert
          v-if="tenant && tenant.user_count > 0"
          type="warning"
          variant="tonal"
          density="compact"
        >
          Dieser Mandant hat noch {{ tenant.user_count }} Zugang/Zugänge. Aus
          Sicherheitsgründen muss er leer sein — entfernen Sie zuerst alle Zugänge
          im Abschnitt „Zugänge“.
        </v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="deleteDialog = false">Abbrechen</v-btn>
        <v-btn
          color="error"
          :loading="busy"
          :disabled="tenant && tenant.user_count > 0"
          @click="submitDelete"
        >Löschen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Sicht (Center + Radius + FL-Band) -->
  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1">Standard-Ansicht</v-card-title>
    <v-card-text>
      <div class="d-flex flex-wrap ga-3">
        <v-text-field
          v-model.number="form.centerLat"
          type="number"
          label="Zentrum Breite (°)"
          variant="outlined"
          density="compact"
          hide-details
          style="max-width: 180px"
        />
        <v-text-field
          v-model.number="form.centerLon"
          type="number"
          label="Zentrum Länge (°)"
          variant="outlined"
          density="compact"
          hide-details
          style="max-width: 180px"
        />
        <v-text-field
          v-model.number="form.radiusNm"
          type="number"
          label="Radius (NM)"
          variant="outlined"
          density="compact"
          hide-details
          style="max-width: 160px"
        />
        <v-text-field
          v-model.number="form.zoom"
          type="number"
          label="Zoom"
          variant="outlined"
          density="compact"
          hide-details
          style="max-width: 120px"
        />
      </div>
      <div class="d-flex flex-wrap ga-3 mt-3">
        <v-text-field
          v-model.number="form.flMin"
          type="number"
          label="FL min"
          variant="outlined"
          density="compact"
          hide-details
          clearable
          style="max-width: 140px"
        />
        <v-text-field
          v-model.number="form.flMax"
          type="number"
          label="FL max"
          variant="outlined"
          density="compact"
          hide-details
          clearable
          style="max-width: 140px"
        />
      </div>
      <p class="text-caption text-medium-emphasis mt-2">
        Radius und Zentrum werden clientseitig in eine AOI-Bounding-Box umgerechnet
        (das Backend bleibt AOI-basiert). Ein Radius von 0 oder leer speichert keine AOI.
      </p>
      <div class="mt-3">
        <v-btn color="primary" :loading="busy" @click="save">Ansicht speichern</v-btn>
      </div>
    </v-card-text>
  </v-card>

  <!-- Features (entitlements) -->
  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1">Features</v-card-title>
    <v-card-text>
      <p v-if="!entitlements.length" class="text-medium-emphasis">Lade Features…</p>
      <div v-for="e in entitlements" :key="e.key" class="d-flex align-center justify-space-between">
        <div>
          <div>{{ e.key }}</div>
          <div class="text-caption text-medium-emphasis">{{ e.description }}</div>
        </div>
        <v-switch
          :model-value="e.enabled"
          color="primary"
          density="compact"
          hide-details
          inset
          :loading="busy"
          @update:model-value="toggleFeature(e, $event)"
        />
      </div>
    </v-card-text>
  </v-card>

  <!-- OpenAIP per tenant (ONB-6, ADR 0011). The key is a secret: the server
       reports only whether one is set and never returns it, so the field starts
       empty and shows the configured status separately. Saving an empty field
       clears the key (falls back to the global key). -->
  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1">OpenAIP-Konfiguration</v-card-title>
    <v-card-text>
      <div class="d-flex align-center ga-2 mb-3">
        <span>Eigener Schlüssel:</span>
        <v-chip
          :color="openaipConfigured ? 'success' : 'default'"
          size="small"
          variant="tonal"
        >
          {{ openaipConfigured ? 'gesetzt' : 'nicht gesetzt (globaler Schlüssel)' }}
        </v-chip>
      </div>
      <div class="d-flex flex-wrap ga-3 align-center">
        <v-text-field
          v-model="openaipKey"
          label="OpenAIP-API-Schlüssel"
          placeholder="Neuen Schlüssel eingeben…"
          variant="outlined"
          density="compact"
          hide-details
          autocomplete="off"
          :type="showKey ? 'text' : 'password'"
          :append-inner-icon="showKey ? 'mdi-eye-off' : 'mdi-eye'"
          style="max-width: 420px"
          @click:append-inner="showKey = !showKey"
        />
        <v-btn color="primary" :loading="busy" :disabled="!openaipKey" @click="saveOpenAIPKey">
          Schlüssel speichern
        </v-btn>
        <v-btn
          v-if="openaipConfigured"
          color="error"
          variant="tonal"
          :loading="busy"
          @click="clearOpenAIPKey"
        >
          Schlüssel entfernen
        </v-btn>
      </div>
      <p class="text-caption text-medium-emphasis mt-2">
        Der gesetzte Schlüssel wird aus Sicherheitsgründen nie wieder angezeigt.
        Mandanten ohne eigenen Schlüssel nutzen den globalen Schlüssel. Eine
        Änderung greift sofort (kein Neustart); die Luftraumdaten werden gegen die
        Standard-Ansicht (Zentrum/Radius) dieses Mandanten abgerufen.
      </p>
    </v-card-text>
  </v-card>

  <!-- Feeds (cross-tenant provisioning, embedded) -->
  <v-card variant="tonal" class="mb-4">
    <v-card-title class="d-flex align-center text-subtitle-1">
      Feeds
      <!-- AP4: health chips for feeds the tenant currently subscribes to -->
      <span v-if="tenant?.feeds?.length" class="ml-2 d-flex ga-1 flex-wrap align-center">
        <v-chip
          v-for="f in tenant.feeds"
          :key="f.id"
          size="x-small"
          variant="flat"
          :color="feedColor(f.id)"
          :title="feedTitle(f.id)"
        >
          {{ f.name }}
        </v-chip>
      </span>
    </v-card-title>
    <v-card-text>
      <AdminProvisioning :tenant-id="tenantId" />
    </v-card-text>
  </v-card>

  <!-- Zugänge (access accounts, embedded) -->
  <v-card variant="tonal">
    <v-card-title class="text-subtitle-1">Zugänge</v-card-title>
    <v-card-text>
      <AdminUsers :tenant-id="tenantId" />
    </v-card-text>
  </v-card>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'
import { radiusNmToBbox, bboxToRadius } from '@/admin/geo.js'
import AdminProvisioning from '@/components/admin/AdminProvisioning.vue'
import AdminUsers from '@/components/admin/AdminUsers.vue'

const props = defineProps({
  tenantId: { type: Number, required: true },
})
const emit = defineEmits(['back'])

const admin = useAdminStore()
const busy = ref(false)
const entitlements = ref([])
const deleteDialog = ref(false) // ONB-4: delete-tenant confirmation

// ONB-6: per-tenant OpenAIP key. The server never returns the key, only whether
// one is configured; the input is for entering a *new* key (or clearing it).
const openaipConfigured = ref(false)
const openaipKey = ref('')
const showKey = ref(false)

// The tenant header (name/status) comes from the overview the parent loaded.
const tenant = computed(() => admin.overview.find((t) => t.id === props.tenantId) || null)

const form = reactive({
  centerLat: 0,
  centerLon: 0,
  radiusNm: 0,
  zoom: 8,
  flMin: null,
  flMax: null,
})

async function loadView() {
  const r = await admin.loadTenantView(props.tenantId)
  if (r.ok && r.data) {
    form.centerLat = r.data.center_lat
    form.centerLon = r.data.center_lon
    form.zoom = r.data.zoom
    form.flMin = r.data.fl_min ?? null
    form.flMax = r.data.fl_max ?? null
    if (r.data.aoi) {
      const derived = bboxToRadius(r.data.aoi)
      form.radiusNm = derived ? round(derived.radiusNm) : 0
    } else {
      form.radiusNm = 0
    }
  }
  // A 404 (no view yet) simply leaves the defaults in place.
}

async function save() {
  busy.value = true
  const dto = {
    center_lat: form.centerLat,
    center_lon: form.centerLon,
    zoom: form.zoom,
  }
  const aoi = radiusNmToBbox(form.centerLat, form.centerLon, form.radiusNm)
  if (aoi) {
    dto.aoi = { min_lat: aoi.minLat, min_lon: aoi.minLon, max_lat: aoi.maxLat, max_lon: aoi.maxLon }
  }
  if (form.flMin !== null && form.flMin !== '') dto.fl_min = form.flMin
  if (form.flMax !== null && form.flMax !== '') dto.fl_max = form.flMax
  await admin.saveTenantView(props.tenantId, dto)
  busy.value = false
}

async function loadEntitlements() {
  const r = await admin.loadTenantEntitlements(props.tenantId)
  entitlements.value = r.ok ? r.data : []
}

async function toggleFeature(e, enabled) {
  busy.value = true
  const r = await admin.setTenantEntitlement(props.tenantId, e.key, enabled)
  if (r.ok) await loadEntitlements()
  busy.value = false
}

// ONB-6: load whether this tenant has its own OpenAIP key (status only).
async function loadOpenAIP() {
  const r = await admin.loadTenantOpenAIP(props.tenantId)
  openaipConfigured.value = r.ok ? !!r.data.configured : false
}

async function saveOpenAIPKey() {
  if (!openaipKey.value) return
  busy.value = true
  const r = await admin.setTenantOpenAIPKey(props.tenantId, openaipKey.value)
  busy.value = false
  if (r.ok) {
    openaipKey.value = ''
    showKey.value = false
    await loadOpenAIP()
  }
}

async function clearOpenAIPKey() {
  busy.value = true
  const r = await admin.setTenantOpenAIPKey(props.tenantId, null)
  busy.value = false
  if (r.ok) {
    openaipKey.value = ''
    await loadOpenAIP()
  }
}

async function toggleStatus() {
  if (!tenant.value) return
  busy.value = true
  const next = tenant.value.status === 'paused' ? 'active' : 'paused'
  await admin.setTenantStatus(props.tenantId, next)
  await admin.loadOverview() // refresh the status chip
  busy.value = false
}

// submitDelete removes the tenant (ONB-4). On success the parent returns to the
// overview, which reloads on mount; the server's guard B (409 while accounts
// remain) is surfaced as a banner by the store.
async function submitDelete() {
  busy.value = true
  const r = await admin.deleteTenant(props.tenantId)
  busy.value = false
  if (r.ok) {
    deleteDialog.value = false
    await admin.loadOverview()
    emit('back')
  }
}

function round(n) {
  return Math.round(n * 10) / 10
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

onMounted(async () => {
  await Promise.all([loadView(), loadEntitlements(), loadOpenAIP(), admin.loadFeedsHealth()])
})
</script>
