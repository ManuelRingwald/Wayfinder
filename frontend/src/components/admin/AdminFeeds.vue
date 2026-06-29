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
              <v-btn size="small" variant="text" prepend-icon="mdi-tune-variant" @click="openSources(f)">
                Quellen
              </v-btn>
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

  <!-- Source configuration dialog (ORCH-1c, ADR 0012). The platform admin
       configures the generic live sources the orchestrator will turn into a
       dedicated Firefly instance for this feed, plus the coarse coverage bbox.
       The server validates every entry (closed vocabulary, per-kind rules) and
       derives coverage when omitted, so this builder is convenience, not the
       boundary. cred_ref is only a reference to a per-feed secret, never the
       secret value (the secret store follows ORCH-2). -->
  <v-dialog v-model="sourcesDialog" max-width="720">
    <v-card>
      <v-card-title class="text-subtitle-1">
        Quellen — {{ sourcesTarget?.name }}
      </v-card-title>
      <v-card-text>
        <p class="text-body-2 text-medium-emphasis mb-3">
          Die Quellen bestimmen, woraus die diesem Feed gewidmete Firefly-Instanz
          ihre Tracks rechnet. Flächenquellen (ADS-B/FLARM) brauchen eine
          BBox; ein echter Radar braucht SAC/SIC. Die grobe Coverage-BBox wird
          beim Speichern aus den Quell-BBoxen abgeleitet.
        </p>

        <div v-if="!sources.length" class="text-medium-emphasis mb-3">
          Noch keine Quelle konfiguriert (Platzhalter-/Szenen-Tracker).
        </div>

        <v-card
          v-for="(s, i) in sources"
          :key="i"
          variant="outlined"
          class="mb-3 pa-3"
        >
          <div class="d-flex align-center ga-3 mb-2">
            <v-select
              v-model="s.type"
              :items="SOURCE_TYPES"
              item-title="label"
              item-value="value"
              label="Quell-Typ"
              density="compact"
              hide-details
              style="max-width: 260px"
            />
            <v-spacer />
            <v-btn size="small" color="error" variant="text" icon="mdi-delete" @click="removeSource(i)" />
          </div>

          <!-- Area-bounded sources: query bbox -->
          <div v-if="isAreaType(s.type)" class="d-flex flex-wrap ga-2">
            <v-text-field v-model.number="s.bbox.min_lat" type="number" label="min lat" density="compact" hide-details style="max-width: 120px" />
            <v-text-field v-model.number="s.bbox.min_lon" type="number" label="min lon" density="compact" hide-details style="max-width: 120px" />
            <v-text-field v-model.number="s.bbox.max_lat" type="number" label="max lat" density="compact" hide-details style="max-width: 120px" />
            <v-text-field v-model.number="s.bbox.max_lon" type="number" label="max lon" density="compact" hide-details style="max-width: 120px" />
          </div>

          <!-- Real radar: SAC/SIC sensor identity -->
          <div v-else class="d-flex ga-2">
            <v-text-field v-model.number="s.sac" type="number" label="SAC (0–255)" density="compact" hide-details style="max-width: 150px" />
            <v-text-field v-model.number="s.sic" type="number" label="SIC (0–255)" density="compact" hide-details style="max-width: 150px" />
          </div>

          <v-text-field
            v-model="s.cred_ref"
            label="Credential-Referenz (optional)"
            hint="Verweis auf ein Pro-Feed-Secret, z. B. secret/speyer-opensky — nie der Schlüssel selbst."
            persistent-hint
            density="compact"
            class="mt-2"
          />

          <!-- Secret value (ORCH-2c 3a): set/clear the credential the cred_ref
               points at. Write-only — the server reports only whether a value is
               configured, never the value. Hidden when the secret store is off. -->
          <div v-if="secretStoreEnabled && (s.cred_ref || '').trim()" class="mt-2">
            <div class="d-flex align-center ga-2 mb-1">
              <v-chip
                size="x-small"
                :color="isSecretConfigured(s.cred_ref) ? 'success' : 'warning'"
                variant="tonal"
              >
                {{ isSecretConfigured(s.cred_ref) ? 'Secret hinterlegt' : 'Kein Secret' }}
              </v-chip>
            </div>
            <div class="d-flex align-center ga-2">
              <v-text-field
                v-model="secretInput[i]"
                type="password"
                autocomplete="new-password"
                :label="isSecretConfigured(s.cred_ref) ? 'Neuen Wert setzen (ersetzt)' : 'Wert setzen'"
                density="compact"
                hide-details
              />
              <v-btn
                size="small"
                color="primary"
                variant="tonal"
                :loading="secretBusy === i"
                :disabled="!(secretInput[i] || '').length"
                @click="saveSecret(i)"
              >
                Speichern
              </v-btn>
              <v-btn
                v-if="isSecretConfigured(s.cred_ref)"
                size="small"
                color="error"
                variant="text"
                :loading="secretBusy === i"
                @click="clearSecret(i)"
              >
                Entfernen
              </v-btn>
            </div>
          </div>
        </v-card>

        <v-btn size="small" variant="tonal" prepend-icon="mdi-plus" @click="addSource">
          Quelle hinzufügen
        </v-btn>

        <v-alert v-if="sourcesError" type="error" variant="tonal" density="compact" class="mt-3">
          {{ sourcesError }}
        </v-alert>
        <v-alert v-else-if="coveragePreview" type="info" variant="tonal" density="compact" class="mt-3">
          Gespeicherte Coverage-BBox: <code>{{ coveragePreview }}</code>
        </v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="sourcesDialog = false">Schließen</v-btn>
        <v-btn color="primary" :loading="busy" @click="submitSources">Speichern</v-btn>
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

// --- source configuration (ORCH-1c) ----------------------------------------
// The closed source vocabulary mirrors the server (store.SourceType). Area types
// carry a query bbox; radar carries a SAC/SIC identity.
const SOURCE_TYPES = [
  { value: 'adsb_opensky', label: 'ADS-B (OpenSky)' },
  { value: 'flarm_aprs', label: 'FLARM (OGN/APRS)' },
  { value: 'radar_asterix', label: 'Radar (ASTERIX CAT048/001)' },
]
const AREA_TYPES = new Set(['adsb_opensky', 'flarm_aprs'])
function isAreaType(t) {
  return AREA_TYPES.has(t)
}

const sourcesDialog = ref(false)
const sourcesTarget = ref(null)
const sources = ref([])
const sourcesError = ref('')
const coveragePreview = ref('')

// Per-feed source credentials (ORCH-2c 3a). secretStoreEnabled mirrors the
// server: false when no WAYFINDER_SECRET_KEY is configured (the secret routes
// return 503), so the controls stay hidden rather than pretending to work.
// secretRefs holds the cred_refs that already have a stored value (the server
// never returns the value itself); secretInput holds the transient new value the
// admin types, keyed by source row index; secretBusy is the row currently saving.
const secretStoreEnabled = ref(false)
const secretRefs = ref(new Set())
const secretInput = ref({})
const secretBusy = ref(-1)

function isSecretConfigured(ref) {
  return secretRefs.value.has((ref || '').trim())
}

// loadFeedSecretsState reads which cred_refs are configured for the feed. A 503
// means the secret store is disabled server-side; anything else leaves it on.
async function loadFeedSecretsState(feedId) {
  secretStoreEnabled.value = false
  secretRefs.value = new Set()
  const r = await admin.loadFeedSecrets(feedId)
  if (r.ok) {
    secretStoreEnabled.value = true
    secretRefs.value = new Set((r.data.secrets || []).map((s) => s.ref))
  } else if (r.status === 503) {
    secretStoreEnabled.value = false
  } else {
    // Unknown error: keep the controls hidden but surface the reason.
    sourcesError.value = r.error || 'Secret-Status konnte nicht geladen werden.'
  }
}

async function saveSecret(i) {
  const ref = (sources.value[i]?.cred_ref || '').trim()
  const value = secretInput.value[i] || ''
  if (!ref || !value) return
  secretBusy.value = i
  const r = await admin.setFeedSecret(sourcesTarget.value.id, ref, value)
  secretBusy.value = -1
  if (r.ok) {
    secretRefs.value = new Set(secretRefs.value).add(ref)
    secretInput.value = { ...secretInput.value, [i]: '' }
  } else {
    sourcesError.value = r.error || 'Secret konnte nicht gespeichert werden.'
  }
}

async function clearSecret(i) {
  const ref = (sources.value[i]?.cred_ref || '').trim()
  if (!ref) return
  secretBusy.value = i
  const r = await admin.deleteFeedSecret(sourcesTarget.value.id, ref)
  secretBusy.value = -1
  if (r.ok) {
    const next = new Set(secretRefs.value)
    next.delete(ref)
    secretRefs.value = next
  } else {
    sourcesError.value = r.error || 'Secret konnte nicht entfernt werden.'
  }
}

// blankSource returns a fresh form entry of the given type with the shape the
// per-type inputs bind to (bbox always present so v-model has a target; unused
// fields are stripped on submit).
function blankSource(type = 'adsb_opensky') {
  return { type, bbox: { min_lat: null, min_lon: null, max_lat: null, max_lon: null }, sac: null, sic: null, cred_ref: '' }
}

// toFormSource maps a stored source (wire shape) into the form model, ensuring a
// bbox object exists for the inputs even when the stored source had none.
function toFormSource(s) {
  return {
    type: s.type,
    bbox: s.bbox ? { ...s.bbox } : { min_lat: null, min_lon: null, max_lat: null, max_lon: null },
    sac: s.sac ?? null,
    sic: s.sic ?? null,
    cred_ref: s.cred_ref ?? '',
  }
}

async function openSources(f) {
  sourcesTarget.value = f
  sources.value = []
  sourcesError.value = ''
  coveragePreview.value = ''
  sourcesDialog.value = true
  busy.value = true
  secretStoreEnabled.value = false
  secretRefs.value = new Set()
  secretInput.value = {}
  const r = await admin.loadFeedSources(f.id)
  if (r.ok) {
    sources.value = (r.data.sources || []).map(toFormSource)
    coveragePreview.value = formatBBox(r.data.coverage_bbox)
    await loadFeedSecretsState(f.id)
  } else {
    sourcesError.value = r.error || 'Quellen konnten nicht geladen werden.'
  }
  busy.value = false
}

function addSource() {
  sources.value.push(blankSource())
}

function removeSource(i) {
  sources.value.splice(i, 1)
}

function formatBBox(b) {
  if (!b) return ''
  const r = (n) => Number(n).toFixed(2)
  return `${r(b.min_lat)},${r(b.min_lon)} → ${r(b.max_lat)},${r(b.max_lon)}`
}

// buildSourcesPayload strips each form entry down to the fields its type uses, so
// the server never sees an area source carrying sac/sic (which it would reject).
function buildSourcesPayload() {
  return {
    sources: sources.value.map((s) => {
      const out = { type: s.type }
      if (isAreaType(s.type)) {
        out.bbox = {
          min_lat: s.bbox.min_lat, min_lon: s.bbox.min_lon,
          max_lat: s.bbox.max_lat, max_lon: s.bbox.max_lon,
        }
      } else {
        out.sac = s.sac
        out.sic = s.sic
      }
      const ref = (s.cred_ref || '').trim()
      if (ref) out.cred_ref = ref
      return out
    }),
  }
}

async function submitSources() {
  sourcesError.value = ''
  busy.value = true
  const r = await admin.saveFeedSources(sourcesTarget.value.id, buildSourcesPayload())
  busy.value = false
  if (r.ok) {
    sources.value = (r.data.sources || []).map(toFormSource)
    coveragePreview.value = formatBBox(r.data.coverage_bbox)
  } else {
    sourcesError.value = r.error || 'Speichern fehlgeschlagen.'
  }
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
