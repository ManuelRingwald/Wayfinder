<template>
  <!-- View-config editor (WF2-32). The AOI bounding box and the FL band are hard,
       server-enforced data-minimisation boundaries (WF2-21.2); here the tenant
       admin edits their own tenant default. Validation mirrors the server
       (validateView) so problems surface before the PUT. -->
  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1">Kartenmittelpunkt &amp; Zoom</v-card-title>
    <v-card-text>
      <v-row dense>
        <v-col cols="12" sm="4">
          <v-text-field v-model="form.center_lat" label="Zentrum Breite (lat)" type="number" step="0.01" />
        </v-col>
        <v-col cols="12" sm="4">
          <v-text-field v-model="form.center_lon" label="Zentrum Länge (lon)" type="number" step="0.01" />
        </v-col>
        <v-col cols="12" sm="4">
          <v-text-field v-model="form.zoom" label="Zoom (0–24)" type="number" step="0.5" />
        </v-col>
      </v-row>
    </v-card-text>
  </v-card>

  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1 d-flex align-center">
      Interessensgebiet (AOI)
      <v-spacer />
      <v-switch v-model="form.aoiEnabled" label="aktiv" color="primary" hide-details />
    </v-card-title>
    <v-card-text v-if="form.aoiEnabled">
      <v-row dense>
        <v-col cols="6" sm="3">
          <v-text-field v-model="form.aoi.min_lat" label="min lat" type="number" step="0.01" />
        </v-col>
        <v-col cols="6" sm="3">
          <v-text-field v-model="form.aoi.min_lon" label="min lon" type="number" step="0.01" />
        </v-col>
        <v-col cols="6" sm="3">
          <v-text-field v-model="form.aoi.max_lat" label="max lat" type="number" step="0.01" />
        </v-col>
        <v-col cols="6" sm="3">
          <v-text-field v-model="form.aoi.max_lon" label="max lon" type="number" step="0.01" />
        </v-col>
      </v-row>
      <p class="text-caption text-medium-emphasis mt-1">
        Außerhalb der AOI liegende Tracks werden serverseitig verworfen (Datenminimierung).
      </p>
    </v-card-text>
  </v-card>

  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1 d-flex align-center">
      Flugflächen-Band
      <v-spacer />
      <v-switch v-model="form.flEnabled" label="aktiv" color="primary" hide-details />
    </v-card-title>
    <v-card-text v-if="form.flEnabled">
      <v-row dense>
        <v-col cols="6">
          <v-text-field v-model="form.fl_min" label="FL min" type="number" step="10" />
        </v-col>
        <v-col cols="6">
          <v-text-field v-model="form.fl_max" label="FL max" type="number" step="10" />
        </v-col>
      </v-row>
      <p class="text-caption text-medium-emphasis mt-1">
        Tracks ohne Flugflächen-Angabe werden bewusst durchgelassen (fail-open).
      </p>
    </v-card-text>
  </v-card>

  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1">Standard-Layer</v-card-title>
    <v-card-text>
      <v-switch
        v-for="key in LAYER_KEYS"
        :key="key"
        v-model="form.layers[key]"
        :label="LAYER_LABELS[key]"
        color="primary"
        density="compact"
        hide-details
      />
    </v-card-text>
  </v-card>

  <v-alert v-if="errors.length" type="error" variant="tonal" class="mb-3">
    <ul class="pl-4">
      <li v-for="e in errors" :key="e">{{ e }}</li>
    </ul>
  </v-alert>

  <v-btn color="primary" variant="flat" :loading="saving" prepend-icon="mdi-content-save" @click="save">
    Ansicht speichern
  </v-btn>
</template>

<script setup>
import { reactive, ref, onMounted, watch } from 'vue'
import { useAdminStore } from '@/stores/admin.js'
import { validateView } from '@/admin/validateView.js'

const admin = useAdminStore()

const LAYER_KEYS = ['airspace', 'navaids', 'waypoints', 'coverageRings']
const LAYER_LABELS = {
  airspace: 'Lufträume',
  navaids: 'Navigationsanlagen',
  waypoints: 'Wegpunkte',
  coverageRings: 'Abdeckungsringe',
}

const form = reactive({
  center_lat: 50.03,
  center_lon: 8.57,
  zoom: 7,
  aoiEnabled: false,
  aoi: { min_lat: 49, min_lon: 7, max_lat: 51, max_lon: 10 },
  flEnabled: false,
  fl_min: 0,
  fl_max: 400,
  layers: { airspace: true, navaids: true, waypoints: true, coverageRings: true },
})
const errors = ref([])
const saving = ref(false)

// num coerces a v-text-field value (which may arrive as a string) to a finite
// number, or NaN — which validateView then flags rather than silently sending.
function num(v) {
  const n = typeof v === 'number' ? v : parseFloat(v)
  return Number.isFinite(n) ? n : NaN
}

function populate(vc) {
  if (!vc) return
  form.center_lat = vc.center_lat
  form.center_lon = vc.center_lon
  form.zoom = vc.zoom
  if (vc.aoi) {
    form.aoiEnabled = true
    Object.assign(form.aoi, vc.aoi)
  }
  if (vc.fl_min != null || vc.fl_max != null) {
    form.flEnabled = true
    if (vc.fl_min != null) form.fl_min = vc.fl_min
    if (vc.fl_max != null) form.fl_max = vc.fl_max
  }
  if (vc.layers) {
    for (const k of LAYER_KEYS) {
      if (k in vc.layers) form.layers[k] = vc.layers[k]
    }
  }
}

function buildDTO() {
  const dto = {
    center_lat: num(form.center_lat),
    center_lon: num(form.center_lon),
    zoom: num(form.zoom),
    layers: { ...form.layers },
  }
  if (form.aoiEnabled) {
    dto.aoi = {
      min_lat: num(form.aoi.min_lat),
      min_lon: num(form.aoi.min_lon),
      max_lat: num(form.aoi.max_lat),
      max_lon: num(form.aoi.max_lon),
    }
  }
  if (form.flEnabled) {
    dto.fl_min = num(form.fl_min)
    dto.fl_max = num(form.fl_max)
  }
  return dto
}

async function save() {
  const dto = buildDTO()
  errors.value = validateView(dto)
  if (errors.value.length) return
  saving.value = true
  await admin.saveView(dto)
  saving.value = false
}

onMounted(async () => {
  await admin.loadView()
  populate(admin.view)
})
watch(() => admin.view, populate)
</script>
