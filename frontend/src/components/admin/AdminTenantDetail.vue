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
    <!-- WF2-34 (ADR 0008): read-only "View as Tenant" straight from the tenant's
         admin page — mints the grant and jumps to the map, where the
         ImpersonationBar shows the yellow read-only banner with the exit. -->
    <v-btn
      size="small"
      color="primary"
      variant="tonal"
      prepend-icon="mdi-account-eye-outline"
      :loading="impBusy"
      @click="viewAsTenant"
    >
      Als Mandant ansehen
    </v-btn>
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

  <!-- WF2-34: surfaced only when minting the read-only grant failed. -->
  <v-alert
    v-if="impError"
    type="error"
    density="compact"
    class="mb-4"
    closable
    @click:close="impError = null"
  >
    {{ impError }}
  </v-alert>

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
          <span class="text-success">Übernommen: {{ airportApplied }}</span> — bei Bedarf unten anpassen, dann „Ansicht speichern“.
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
          <!-- Show the catalogue's Fachbegriff (e.label); fall back to the raw
               key only if an older server omits it. Reserved keys (#175) carry a
               "noch nicht aktiv" chip so the disabled toggle reads as intentional. -->
          <div>
            {{ e.label || e.key }}
            <v-chip v-if="e.reserved" size="x-small" variant="tonal" class="ml-2">noch nicht aktiv</v-chip>
          </div>
          <div class="text-caption text-medium-emphasis">{{ e.description }}</div>
        </div>
        <v-switch
          :model-value="e.enabled"
          color="primary"
          density="compact"
          hide-details
          inset
          :loading="busy"
          :disabled="e.reserved"
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
      <div class="d-flex align-center ga-2 mb-3 flex-wrap">
        <span>Eigener Schlüssel:</span>
        <v-chip
          :color="openaipConfigured ? 'success' : 'default'"
          size="small"
          variant="tonal"
        >
          {{ openaipConfigured ? 'gesetzt' : 'nicht gesetzt (globaler Schlüssel)' }}
        </v-chip>
        <!-- AERO-1/2: cache freshness for this tenant + a "refresh now" button. -->
        <v-chip v-if="openaipFetchedAt" size="small" variant="text" prepend-icon="mdi-clock-outline">
          zuletzt geholt: {{ formatFetchedAt(openaipFetchedAt) }} · {{ openaipFeatureCount }} Objekte
        </v-chip>
        <v-chip v-else size="small" variant="text" class="text-medium-emphasis">
          noch nichts gecacht
        </v-chip>
        <v-btn
          size="small"
          variant="tonal"
          prepend-icon="mdi-refresh"
          :loading="busy"
          @click="refreshOpenAIP"
        >
          Jetzt aktualisieren
        </v-btn>
      </div>
      <!-- AERO-3: change-impact of the last refresh, per layer. Robuster
           Count-Delta; +hinzu/−entfernt ist Churn (In-Place-Edit zählt als −1/+1). -->
      <div v-if="openaipChanges.length" class="mb-3">
        <div class="text-caption text-medium-emphasis mb-1">Letzte Änderung je Ebene:</div>
        <div class="d-flex flex-wrap ga-2">
          <v-chip v-for="c in openaipChanges" :key="c.kind" size="small" variant="tonal">
            {{ layerLabel(c.kind) }}:
            <template v-if="c.prev_feature_count != null">
              {{ c.prev_feature_count }} → {{ c.feature_count }}
              <span :class="churnClass(c)" class="ml-1">(+{{ c.added ?? 0 }}/−{{ c.removed ?? 0 }})</span>
            </template>
            <template v-else>{{ c.feature_count }} (Erstbefüllung)</template>
          </v-chip>
        </div>
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
      <AdminProvisioning :tenant-id="tenantId" @changed="onFeedsChanged" />
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
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useAdminStore } from '@/stores/admin.js'
import { useImpersonationStore } from '@/stores/impersonation.js'
import { radiusNmToBbox, bboxToRadius } from '@/admin/geo.js'
import { describeFeedHealth } from '@/admin/feedHealth.js'
import AdminProvisioning from '@/components/admin/AdminProvisioning.vue'
import AdminUsers from '@/components/admin/AdminUsers.vue'

const props = defineProps({
  tenantId: { type: Number, required: true },
})
const emit = defineEmits(['back'])

const admin = useAdminStore()
const busy = ref(false)

// WF2-34 (ADR 0008): "Als Mandant ansehen" from the admin page. The server
// mints the HttpOnly grant cookie; navigating to the map hands over to the
// ImpersonationBar (banner, switcher, exit) and the /ws connect picks up the
// target scope. Read-only by construction — no admin action here writes any
// tenant user's view.
const imp = useImpersonationStore()
const router = useRouter()
const impBusy = ref(false)
const impError = ref(null)

async function viewAsTenant() {
  impBusy.value = true
  impError.value = null
  const ok = await imp.start(props.tenantId)
  impBusy.value = false
  if (ok) {
    router.push('/')
    return
  }
  impError.value = imp.error || 'Ansehen als Mandant fehlgeschlagen.'
}
const entitlements = ref([])
const deleteDialog = ref(false) // ONB-4: delete-tenant confirmation

// ONB-6: per-tenant OpenAIP key. The server never returns the key, only whether
// one is configured; the input is for entering a *new* key (or clearing it).
const openaipConfigured = ref(false)
const openaipKey = ref('')
const showKey = ref(false)
// AERO-1/2: persistent-cache freshness for this tenant + refresh button.
const openaipFetchedAt = ref(null)
const openaipFeatureCount = ref(0)
// AERO-3: per-layer change-impact of the last refresh.
const openaipChanges = ref([])

const LAYER_LABELS = { airspace: 'Luftraum', navaid: 'Navaids', waypoint: 'Wegpunkte' }
function layerLabel(kind) {
  return LAYER_LABELS[kind] || kind
}
function churnClass(c) {
  return (c.added ?? 0) + (c.removed ?? 0) > 0 ? 'text-warning' : 'text-medium-emphasis'
}

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
})

// ICAO airport search (offline directory, /api/admin/airports). Selecting a hit
// is the confirmation: the map centre plus the ICAO fields (header + QNH) fill
// in, all still editable and nothing persisted until "Ansicht speichern". The
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
    dto.aoi = aoi // already the backend wire shape (min_lat/min_lon/max_lat/max_lon)
  }
  if (form.flMin !== null && form.flMin !== '') dto.fl_min = form.flMin
  if (form.flMax !== null && form.flMax !== '') dto.fl_max = form.flMax
  if (form.icao && form.icao.trim()) dto.icao = form.icao.trim()
  if (form.qnhIcao && form.qnhIcao.trim()) dto.qnh_icao = form.qnhIcao.trim().toUpperCase()
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

// ONB-6/AERO-1: load whether this tenant has its own OpenAIP key (status only) plus
// the persistent-cache freshness (last fetch time + cached feature count).
async function loadOpenAIP() {
  const r = await admin.loadTenantOpenAIP(props.tenantId)
  if (r.ok && r.data) {
    openaipConfigured.value = !!r.data.configured
    openaipFetchedAt.value = r.data.fetched_at ?? null
    openaipFeatureCount.value = r.data.feature_count ?? 0
  } else {
    openaipConfigured.value = false
    openaipFetchedAt.value = null
    openaipFeatureCount.value = 0
  }
  const c = await admin.loadTenantOpenAIPChanges(props.tenantId)
  openaipChanges.value = c.ok && Array.isArray(c.data) ? c.data : []
}

// AERO-2: force a fresh OpenAIP fetch for this tenant, then reload the status so the
// timestamp updates once the (async) fetch has had a moment to land.
async function refreshOpenAIP() {
  busy.value = true
  const r = await admin.refreshTenantOpenAIP(props.tenantId)
  busy.value = false
  if (r.ok) await loadOpenAIP()
}

// formatFetchedAt renders an ISO/RFC3339 timestamp in the operator's locale.
function formatFetchedAt(ts) {
  const d = new Date(ts)
  return Number.isNaN(d.getTime()) ? String(ts) : d.toLocaleString()
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

// onFeedsChanged reacts to a grant/revoke in the embedded provisioning table.
// The header feed chips derive from admin.overview (loaded once by the parent),
// so without this refresh they drift out of sync with the assignment table below
// (chips still show the old feed set). Reload the overview (chips) and feed health
// (chip colour/title) so the whole Feeds card reflects the new assignment at once.
async function onFeedsChanged() {
  await Promise.all([admin.loadOverview(), admin.loadFeedsHealth()])
}

// Feed-health chip colour/title from the shared helper (AP4 + status
// granularity): red splits into "nie gestartet" vs "abgerissen".
function feedColor(feedId) {
  return describeFeedHealth(admin.feedsHealth[feedId]).color
}

function feedTitle(feedId) {
  return describeFeedHealth(admin.feedsHealth[feedId]).title
}

onMounted(async () => {
  await Promise.all([loadView(), loadEntitlements(), loadOpenAIP(), admin.loadFeedsHealth()])
})
</script>
