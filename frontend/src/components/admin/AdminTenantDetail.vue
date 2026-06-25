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
  </div>

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

  <!-- Feeds (cross-tenant provisioning, embedded) -->
  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1">Feeds</v-card-title>
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
defineEmits(['back'])

const admin = useAdminStore()
const busy = ref(false)
const entitlements = ref([])

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

async function toggleStatus() {
  if (!tenant.value) return
  busy.value = true
  const next = tenant.value.status === 'paused' ? 'active' : 'paused'
  await admin.setTenantStatus(props.tenantId, next)
  await admin.loadOverview() // refresh the status chip
  busy.value = false
}

function round(n) {
  return Math.round(n * 10) / 10
}

onMounted(async () => {
  await Promise.all([loadView(), loadEntitlements()])
})
</script>
