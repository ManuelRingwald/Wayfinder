<template>
  <!-- AP3 (ADR 0009): per-tenant configuration, slimmed since #210 to the default
       view (entered as centre + radius in NM, stored as an AOI bbox) and the
       feature entitlements; Feeds, OpenAIP and access accounts moved to their own
       overview dialogs. #211: a single global save persists both, then returns to
       the overview. The server enforces every boundary (requireAdmin → 403). -->
  <div class="d-flex align-center mb-4 ga-3">
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
  <v-dialog v-model="deleteDialog" max-width="min(480px, 94vw)">
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
      <!-- ICAO airport search: type a code or name, pick a hit → the map centre
           (and the ICAO fields) fill in. Offline directory, no external call. -->
      <v-autocomplete
        v-model="airportPick"
        :items="airportHits"
        :loading="airportSearching"
        item-title="label"
        item-value="icao"
        return-object
        no-filter
        clearable
        auto-select-first
        label="Flughafen suchen (ICAO oder Name)"
        placeholder="z. B. EDDH oder Hamburg"
        prepend-inner-icon="mdi-airport"
        variant="outlined"
        density="compact"
        hide-details
        class="mb-1"
        style="max-width: 380px"
        @update:search="onAirportSearch"
        @update:model-value="onAirportPick"
      >
        <template #no-data>
          <div class="px-3 py-2 text-caption text-medium-emphasis">
            {{ airportQuery.trim().length < 2 ? 'Mindestens 2 Zeichen eingeben…' : 'Keine Treffer' }}
          </div>
        </template>
      </v-autocomplete>
      <p class="text-caption text-medium-emphasis mb-3">
        <template v-if="airportApplied">
          <span class="text-success">Übernommen: {{ airportApplied }}</span> — bei Bedarf unten anpassen, dann „Speichern“.
        </template>
        <template v-else>
          ICAO-Code (z. B. <code>EDDH</code>) oder Name eingeben und einen Treffer wählen — Zentrum-Koordinaten, ICAO-Kürzel und QNH-Flugplatz werden dann automatisch gefüllt.
        </template>
      </p>
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
      <p class="text-caption text-medium-emphasis mt-2">
        <strong>Zentrum &amp; Zoom</strong> legen den Start-Kartenausschnitt der
        ASD-Karte (Mittelpunkt und Zoomstufe) für alle Clients dieses Mandanten fest.
        Zentrum + <strong>Radius</strong> werden clientseitig in eine rechteckige
        AOI-Bounding-Box umgerechnet (das Backend filtert AOI-basiert). Die AOI ist
        eine <strong>harte serverseitige Daten-Minimierungsgrenze</strong>: Tracks
        außerhalb werden verworfen und erreichen den Client nie — keine reine
        Anzeigepräferenz. Ein Radius von 0 oder leer speichert keine AOI (kein
        geografischer Filter).
      </p>
      <div class="d-flex flex-wrap ga-3 mt-3">
        <v-text-field
          v-model.number="form.flMin"
          type="number"
          label="FL min (× 100 ft)"
          variant="outlined"
          density="compact"
          hide-details
          clearable
          style="max-width: 180px"
        />
        <v-text-field
          v-model.number="form.flMax"
          type="number"
          label="FL max (× 100 ft)"
          variant="outlined"
          density="compact"
          hide-details
          clearable
          style="max-width: 180px"
        />
      </div>
      <p class="text-caption text-medium-emphasis mt-2">
        <strong>FL-Band</strong> in Flight-Level-Einheiten (× 100 ft): FL100 =
        10 000 ft. Tracks außerhalb des Bands werden serverseitig verworfen.
        <strong>Fail-open:</strong> Tracks ohne gemeldete Flugfläche (keine
        Mode-C-Höhe) werden immer zugestellt.
      </p>
      <div class="d-flex flex-wrap ga-3 mt-3">
        <v-text-field
          v-model="form.icao"
          label="ICAO-Kürzel (Kopfzeile)"
          placeholder="z. B. EDGG·KTG"
          variant="outlined"
          density="compact"
          hide-details
          clearable
          maxlength="12"
          style="max-width: 220px"
        />
        <v-text-field
          v-model="form.qnhIcao"
          label="QNH-Flugplatz (ICAO)"
          placeholder="z. B. EDDH"
          variant="outlined"
          density="compact"
          hide-details
          clearable
          maxlength="4"
          style="max-width: 220px"
        />
      </div>
      <p class="text-caption text-medium-emphasis mt-2">
        <strong>ICAO-Kürzel</strong> erscheint in der ASD-Kopfzeile (Sektor/FIR,
        z. B. <code>EDGG·KTG</code>). Reine Anzeige — nicht im CAT062-Strom
        enthalten. Leer = keine Anzeige.
        <br />
        <strong>QNH-Flugplatz</strong> ist der echte 4-stellige ICAO-Code (z. B.
        <code>EDDH</code>), dessen aktuelles QNH die Kopfzeile zeigt (NOAA-METAR).
        Braucht zusätzlich das Feature <code>qnh</code>. Leer = keine QNH-Anzeige.
      </p>

      <!-- ASD-014 (ADR 0021): Area of Responsibility — the airspaces (CTR/TMA)
           this tenant controls, highlighted on the map. Held as stable OpenAIP
           airspace ids (robust against AIRAC name drift). -->
      <v-combobox
        v-model="form.aorAirspaceIds"
        label="Verantwortungsbereich — Luftraum-IDs (AoR)"
        placeholder="OpenAIP-Luftraum-ID eintippen und Enter"
        variant="outlined"
        density="compact"
        multiple
        chips
        closable-chips
        clearable
        hide-details
        class="mt-3"
      />
      <p class="text-caption text-medium-emphasis mt-2">
        <strong>Verantwortungsbereich (AoR)</strong> hebt die zugehörigen Lufträume
        (CTR/TMA) auf der Karte hervor (ADR 0021). Eingetragen werden die
        <strong>stabilen OpenAIP-Luftraum-IDs</strong> — nicht der Name, denn Namen
        ändern sich pro AIRAC-Zyklus. Leer = kein hervorgehobener Bereich, max. 500
        IDs. Reine Anzeige, nicht im CAT062-Strom.
      </p>
    </v-card-text>
  </v-card>

  <!-- Features (entitlements) -->
  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1">Features</v-card-title>
    <v-card-text>
      <p v-if="!entitlements.length" class="text-medium-emphasis">Lade Features…</p>
      <div v-for="e in entitlements" :key="e.key" class="d-flex align-center justify-space-between">
        <div>
          <!-- Show the catalogue's Fachbegriff (e.label); fall back to the raw
               key only if an older server omits it. Reserved keys (#175) carry a
               "noch nicht aktiv" chip so the disabled toggle reads as intentional. -->
          <div>
            {{ e.label || e.key }}
            <v-chip v-if="e.reserved" size="x-small" variant="tonal" class="ml-2">noch nicht aktiv</v-chip>
          </div>
          <div class="text-caption text-medium-emphasis">{{ e.description }}</div>
        </div>
        <!-- #211: buffered locally — the switch updates featureEdits only; nothing
             is persisted until the global save below. -->
        <v-switch
          :model-value="featureEdits[e.key]"
          color="primary"
          density="compact"
          hide-details
          inset
          :disabled="e.reserved"
          @update:model-value="featureEdits[e.key] = $event"
        />
      </div>
    </v-card-text>
  </v-card>

  <!-- #211: one global save persists the default view AND the feature toggles at
       once, then returns to the overview; cancel returns without persisting. -->
  <div class="d-flex justify-end ga-3 mb-2">
    <v-btn variant="text" :disabled="busy" @click="cancel">Abbrechen</v-btn>
    <v-btn color="primary" :loading="busy" @click="saveAll">Speichern</v-btn>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useAdminStore } from '@/stores/admin.js'
import { radiusNmToBbox, bboxToRadius } from '@/admin/geo.js'

const props = defineProps({
  tenantId: { type: Number, required: true },
})
const emit = defineEmits(['back'])

const admin = useAdminStore()
const busy = ref(false)

const entitlements = ref([])
// #211: feature toggles are buffered here and only persisted on the global save —
// flipping a switch no longer takes effect immediately. cancel() drops the buffer.
const featureEdits = reactive({})
const deleteDialog = ref(false) // ONB-4: delete-tenant confirmation

// The tenant header (name/status) comes from the overview the parent loaded.
const tenant = computed(() => admin.overview.find((t) => t.id === props.tenantId) || null)

const form = reactive({
  centerLat: 0,
  centerLon: 0,
  radiusNm: 0,
  zoom: 8,
  flMin: null,
  flMax: null,
  icao: '',
  qnhIcao: '',
  // ASD-014 (ADR 0021): the tenant's Area of Responsibility as a list of stable
  // OpenAIP airspace ids (CTR/TMA). Edited as chips; the map highlights them.
  aorAirspaceIds: [],
})

// ICAO airport search (offline directory, /api/admin/airports). Selecting a hit
// is the confirmation: the map centre plus the ICAO fields (header + QNH) fill
// in, all still editable and nothing persisted until the global "Speichern". The
// search is debounced so it doesn't fire on every keystroke.
const airportHits = ref([])
const airportSearching = ref(false)
const airportQuery = ref('')
const airportPick = ref(null)
const airportApplied = ref('')
let airportDebounce = null
let skipNextSearch = false

function onAirportSearch(q) {
  // Selecting an item sets the search text to its label; skip that echo so we
  // don't fire a pointless query for "EDDH — Hamburg …".
  if (skipNextSearch) {
    skipNextSearch = false
    return
  }
  airportQuery.value = q || ''
  if (airportDebounce) clearTimeout(airportDebounce)
  const query = airportQuery.value.trim()
  if (query.length < 2) {
    airportHits.value = []
    return
  }
  airportDebounce = setTimeout(async () => {
    airportSearching.value = true
    const r = await admin.searchAirports(query)
    airportSearching.value = false
    const rows = r.ok && Array.isArray(r.data) ? r.data : []
    airportHits.value = rows.map((a) => ({ ...a, label: `${a.icao} — ${a.name}` }))
  }, 250)
}

function onAirportPick(hit) {
  if (!hit || typeof hit !== 'object') return
  form.centerLat = hit.lat
  form.centerLon = hit.lon
  form.icao = hit.icao
  form.qnhIcao = hit.icao
  airportApplied.value = `${hit.icao} — ${hit.name} (${round(hit.lat)}, ${round(hit.lon)})`
  // Reset the picker so it stays a reusable search tool; the "Übernommen" hint
  // keeps the confirmation visible.
  skipNextSearch = true
  airportHits.value = []
  nextTick(() => {
    airportPick.value = null
  })
}

async function loadView() {
  const r = await admin.loadTenantView(props.tenantId)
  if (r.ok && r.data) {
    form.centerLat = r.data.center_lat
    form.centerLon = r.data.center_lon
    form.zoom = r.data.zoom
    form.flMin = r.data.fl_min ?? null
    form.flMax = r.data.fl_max ?? null
    form.icao = r.data.icao ?? ''
    form.qnhIcao = r.data.qnh_icao ?? ''
    form.aorAirspaceIds = Array.isArray(r.data.aor_airspace_ids) ? r.data.aor_airspace_ids : []
    if (r.data.aoi) {
      const derived = bboxToRadius(r.data.aoi)
      form.radiusNm = derived ? round(derived.radiusNm) : 0
    } else {
      form.radiusNm = 0
    }
  }
  // A 404 (no view yet) simply leaves the defaults in place.
}

// buildViewDto assembles the Standard-Ansicht wire payload from the form. The AOI
// is derived from centre + radius (radiusNmToBbox); optional fields are sent only
// when set, so an empty field clears nothing it shouldn't.
function buildViewDto() {
  const dto = {
    center_lat: form.centerLat,
    center_lon: form.centerLon,
    zoom: form.zoom,
  }
  const aoi = radiusNmToBbox(form.centerLat, form.centerLon, form.radiusNm)
  if (aoi) {
    dto.aoi = aoi // already the backend wire shape (min_lat/min_lon/max_lat/max_lon)
  }
  if (form.flMin !== null && form.flMin !== '') dto.fl_min = form.flMin
  if (form.flMax !== null && form.flMax !== '') dto.fl_max = form.flMax
  if (form.icao && form.icao.trim()) dto.icao = form.icao.trim()
  if (form.qnhIcao && form.qnhIcao.trim()) dto.qnh_icao = form.qnhIcao.trim().toUpperCase()
  // ASD-014: trim + de-duplicate the AoR ids (mirrors server normalizeAoRIDs);
  // sent only when non-empty so an empty list clears the AoR (SQL NULL).
  const aor = normalizeAorIds(form.aorAirspaceIds)
  if (aor.length) dto.aor_airspace_ids = aor
  return dto
}

// normalizeAorIds mirrors the server's normalizeAoRIDs: trim each id, drop empties,
// de-duplicate while preserving order. Keeps the wire payload clean; the server
// re-normalises and validates authoritatively.
function normalizeAorIds(ids) {
  const seen = new Set()
  const out = []
  for (const raw of ids ?? []) {
    const t = String(raw).trim()
    if (!t || seen.has(t)) continue
    seen.add(t)
    out.push(t)
  }
  return out
}

async function loadEntitlements() {
  const r = await admin.loadTenantEntitlements(props.tenantId)
  entitlements.value = r.ok ? r.data : []
  // #211: seed the local edit buffer from the server state. Reserved keys are
  // included so their (disabled) switch still renders, but they are skipped on save.
  for (const e of entitlements.value) featureEdits[e.key] = !!e.enabled
}

// #211: the single global save. Persist the default view AND every feature toggle
// that actually changed against the loaded state, then return to the overview.
// Nothing here takes effect until this runs, so an admin can toggle freely and back
// out via cancel().
async function saveAll() {
  busy.value = true
  await admin.saveTenantView(props.tenantId, buildViewDto())
  for (const e of entitlements.value) {
    if (e.reserved) continue
    const desired = !!featureEdits[e.key]
    if (desired !== !!e.enabled) {
      await admin.setTenantEntitlement(props.tenantId, e.key, desired)
    }
  }
  busy.value = false
  emit('back')
}

// #211: discard the buffered edits and return to the overview without persisting.
function cancel() {
  emit('back')
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

onMounted(async () => {
  await Promise.all([loadView(), loadEntitlements()])
})
</script>
