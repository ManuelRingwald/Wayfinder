<template>
  <!-- Per-tenant OpenAIP configuration (ONB-6 / AERO-1..3, ADR 0011). Extracted
       from the tenant detail page into its own component so the admin overview can
       open it as a focused dialog (#210). The key is a secret: the server reports
       only whether one is set and never returns it, so the field starts empty and
       shows the configured status separately. Saving an empty field clears the key
       (falls back to the global key). -->
  <div>
    <div class="d-flex align-center ga-2 mb-3 flex-wrap">
      <span>Eigener Schlüssel:</span>
      <v-chip :color="openaipConfigured ? 'success' : 'default'" size="small" variant="tonal">
        {{ openaipConfigured ? 'gesetzt' : 'nicht gesetzt (globaler Schlüssel)' }}
      </v-chip>
      <!-- AERO-1/2: cache freshness for this tenant + a "refresh now" button. -->
      <v-chip v-if="openaipFetchedAt" size="small" variant="text" prepend-icon="mdi-clock-outline">
        zuletzt geholt: {{ formatFetchedAt(openaipFetchedAt) }} · {{ openaipFeatureCount }} Objekte
      </v-chip>
      <v-chip v-else size="small" variant="text" class="text-medium-emphasis">
        noch nichts gecacht
      </v-chip>
      <v-btn size="small" variant="tonal" prepend-icon="mdi-refresh" :loading="busy" @click="refreshOpenAIP">
        Jetzt aktualisieren
      </v-btn>
    </div>
    <!-- AERO-3: change-impact of the last refresh, per layer. Robuster Count-Delta;
         +hinzu/−entfernt ist Churn (In-Place-Edit zählt als −1/+1). -->
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
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

// tenantId identifies whose OpenAIP config is edited. The component self-loads on
// mount and whenever the tenant changes, so it works both standalone and when the
// admin overview reuses a single dialog instance across rows (#210).
const props = defineProps({
  tenantId: { type: Number, required: true },
})

const admin = useAdminStore()
const busy = ref(false)

// ONB-6: per-tenant OpenAIP key. The server never returns the key, only whether one
// is configured; the input is for entering a *new* key (or clearing it).
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

watch(() => props.tenantId, loadOpenAIP)
onMounted(loadOpenAIP)
</script>
