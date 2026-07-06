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
  <v-dialog v-model="createDialog" max-width="min(520px, 94vw)">
    <v-card>
      <v-card-title class="text-subtitle-1">Feed anlegen</v-card-title>
      <v-card-text>
        <v-text-field v-model="form.name" label="Name" autofocus class="mb-2" />

        <!-- ORCH-4: by default the server auto-allocates a collision-free
             multicast endpoint (one group per feed). Advanced users can override
             with an explicit group/port. -->
        <v-switch
          v-model="form.autoEndpoint"
          color="primary"
          density="compact"
          hide-details
          label="Multicast-Endpoint automatisch zuweisen"
          class="mb-1"
        />
        <p v-if="form.autoEndpoint" class="text-caption text-medium-emphasis mb-2">
          Der Server vergibt die nächste freie Multicast-Gruppe (eigene Gruppe je
          Feed) — sauberste Netz-Isolation, keine Kollisionen.
        </p>
        <div v-else class="d-flex ga-3">
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
        <!-- Issue #102: the sensor mix is derived automatically from the feed's
             sources when they are saved (under „Quellen“). This field is only an
             optional initial hint before sources exist and is overwritten on save. -->
        <v-text-field
          v-model="form.sensorMix"
          label="Sensor-Mix (optional)"
          hint="Wird beim Speichern der Quellen automatisch aus deren Typen gesetzt — hier meist leer lassen."
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
  <v-dialog v-model="sourcesDialog" max-width="min(720px, 94vw)">
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
              @update:model-value="ensureCredRef(s)"
            />
            <v-spacer />
            <v-btn size="small" color="error" variant="text" icon="mdi-delete" @click="removeSource(i)" />
          </div>

          <!-- Area-bounded sources: centre + radius (#109/#113). The tenant
               dropdown fills these from that tenant's Standard-Ansicht; the query
               bbox is derived on save (radiusNmToBbox), so the admin reasons in the
               same centre+radius terms as the tenant view rather than four corners. -->
          <div v-if="isAreaType(s.type)">
            <v-select
              v-model="s.tenant_id"
              :items="tenantOptions"
              item-title="name"
              item-value="id"
              label="Aus Mandant übernehmen (optional)"
              hint="Füllt Zentrum + Radius aus der Standard-Ansicht des gewählten Mandanten."
              persistent-hint
              density="compact"
              clearable
              class="mb-2"
              style="max-width: 340px"
              :loading="tenantAreaBusy === i"
              @update:model-value="applyTenantArea(i, $event)"
            />
            <div class="d-flex flex-wrap ga-2">
              <v-text-field v-model.number="s.center_lat" type="number" label="Zentrum Breite (°)" density="compact" hide-details style="max-width: 160px" />
              <v-text-field v-model.number="s.center_lon" type="number" label="Zentrum Länge (°)" density="compact" hide-details style="max-width: 160px" />
              <v-text-field v-model.number="s.radius_nm" type="number" label="Radius (NM)" density="compact" hide-details style="max-width: 130px" />
            </div>

            <!-- Provider (#201) — nur Community-Aggregator: welcher freie Dienst
                 abgefragt wird. Beide sprechen dasselbe API-Format; die Auswahl
                 ist der Ausweichweg bei Ausfall/Drosselung eines Anbieters. -->
            <div v-if="s.type === 'adsb_aggregator'" class="mt-2">
              <v-select
                v-model="s.provider"
                :items="AGG_PROVIDERS"
                item-title="label"
                item-value="value"
                label="Anbieter"
                hint="Frei und ohne Zugangsdaten. Bei Ausfall einfach auf den anderen Anbieter umstellen."
                persistent-hint
                density="compact"
                style="max-width: 240px"
              />
            </div>

            <!-- Poll-Intervall (ADR 0029/0031) — nur gepollte Quellen (FLARM ist
                 Push). Leer = Firefly-Default (10 s). Die Infobox erklärt die
                 Grenzen des jeweiligen Dienstes, damit der Betreiber das
                 Rate-Limit (HTTP 429) respektiert. -->
            <div v-if="isPolledType(s.type)" class="mt-2">
              <v-text-field
                v-model.number="s.poll_interval_secs"
                type="number"
                label="Poll-Intervall (s, optional)"
                :placeholder="`Standard ${DEFAULT_POLL_SECS}`"
                :hint="`Leer = Firefly-Default (${DEFAULT_POLL_SECS} s). Bereich ${MIN_POLL_SECS}–${MAX_POLL_SECS} s.`"
                persistent-hint
                density="compact"
                style="max-width: 240px"
              />
              <v-alert v-if="s.type === 'adsb_opensky'" type="info" variant="tonal" density="compact" class="mt-2">
                OpenSky ist ratenbegrenzt: <strong>anonym ~1 Abfrage/10 s</strong>,
                <strong>authentifiziert ~1/5 s</strong>. Ein zu kurzes Intervall läuft in
                HTTP&nbsp;429 — lieber am Limit bleiben oben oder Zugangsdaten hinterlegen.
              </v-alert>
              <v-alert v-else type="info" variant="tonal" density="compact" class="mt-2">
                Community-Dienst ohne Zugangsdaten, betrieben von Freiwilligen. Öffentliche
                Grenze ~1 Abfrage/s — der Standard von {{ DEFAULT_POLL_SECS }}&nbsp;s bleibt
                bewusst höflich darunter.
              </v-alert>
            </div>
          </div>

          <!-- Real radar (radar_asterix): SAC/SIC identity + site location. CAT048
               is polar relative to the radar and does not carry the site, so Firefly
               needs lat/lon (Pflicht, #91); Höhe/Listen-Endpoint sind optional. -->
          <div v-else>
            <div class="d-flex ga-2">
              <v-text-field v-model.number="s.sac" type="number" label="SAC (0–255)" density="compact" hide-details style="max-width: 150px" />
              <v-text-field v-model.number="s.sic" type="number" label="SIC (0–255)" density="compact" hide-details style="max-width: 150px" />
            </div>
            <div class="d-flex flex-wrap ga-2 mt-2">
              <v-text-field v-model.number="s.lat" type="number" label="Radar Breite (°)" density="compact" hide-details style="max-width: 150px" />
              <v-text-field v-model.number="s.lon" type="number" label="Radar Länge (°)" density="compact" hide-details style="max-width: 150px" />
              <v-text-field v-model.number="s.height_m" type="number" label="Höhe (m, optional)" density="compact" hide-details style="max-width: 150px" />
            </div>
            <v-text-field
              v-model="s.listen"
              label="Listen-Endpoint (optional)"
              hint="UDP group:port für den ASTERIX-Eingang, z. B. 239.255.0.48:8048"
              persistent-hint
              density="compact"
              class="mt-2"
              style="max-width: 320px"
            />
          </div>

          <!-- Credentials (UX-4) — only for source types that authenticate. Radar
               (CAT048) is a network endpoint with no auth, so it gets NO credential
               UI. The credential reference is auto-managed (ensureCredRef), so the
               operator no longer has to invent a handle; the two fields combine into
               the single "id:secret" value the store keeps (credential.js). -->
          <div v-if="credInfo(s.type)" class="mt-2">
            <div class="d-flex align-center ga-2 mb-1">
              <span class="text-caption text-medium-emphasis">{{ credInfo(s.type).title }}</span>
              <v-chip
                v-if="secretStoreEnabled"
                size="x-small"
                :color="isSecretConfigured(s.cred_ref) ? 'success' : (credInfo(s.type).required ? 'warning' : 'default')"
                variant="tonal"
              >
                {{ isSecretConfigured(s.cred_ref) ? 'hinterlegt' : 'nicht gesetzt' }}
              </v-chip>
            </div>

            <!-- Secret store off: nothing can be stored — explain instead of showing
                 a dead reference field (the recurring stumbling block). -->
            <v-alert v-if="!secretStoreEnabled" type="warning" variant="tonal" density="compact">
              Secret-Store deaktiviert (kein <code>WAYFINDER_SECRET_KEY</code> gesetzt) — hier lässt sich kein Zugang hinterlegen.
              <template v-if="credInfo(s.type).required">
                Ohne Zugang läuft OpenSky <strong>anonym</strong> und wird schnell rate-limitiert (HTTP&nbsp;429).
              </template>
              <template v-else>
                Die Quelle läuft dann anonym (bei FLARM/APRS-IS der Normalfall).
              </template>
            </v-alert>

            <div v-else class="d-flex align-start ga-2">
              <v-text-field
                v-model="secretUser[i]"
                type="text"
                autocomplete="off"
                :label="credInfo(s.type).user"
                density="compact"
                hide-details
              />
              <v-text-field
                v-model="secretPass[i]"
                type="password"
                autocomplete="new-password"
                :label="isSecretConfigured(s.cred_ref) ? credInfo(s.type).pass + ' (ersetzt)' : credInfo(s.type).pass"
                density="compact"
                hide-details
              />
              <v-btn
                size="small"
                color="primary"
                variant="tonal"
                class="mt-1"
                :loading="secretBusy === i"
                :disabled="!(secretUser[i] || '').length || !(secretPass[i] || '').length || secretError(i) !== ''"
                @click="saveSecret(i)"
              >
                Speichern
              </v-btn>
              <v-btn
                v-if="isSecretConfigured(s.cred_ref)"
                size="small"
                color="error"
                variant="text"
                class="mt-1"
                :loading="secretBusy === i"
                @click="clearSecret(i)"
              >
                Entfernen
              </v-btn>
            </div>
            <v-alert
              v-if="secretStoreEnabled && secretError(i)"
              type="warning"
              variant="tonal"
              density="compact"
              class="mt-1"
            >
              {{ secretError(i) }}
            </v-alert>
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
  <v-dialog v-model="deleteDialog" max-width="min(460px, 94vw)">
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
import { validateCredential, combineCredential } from '@/admin/credential.js'
import { radiusNmToBbox, bboxToRadius } from '@/admin/geo.js'
import { describeFeedHealth } from '@/admin/feedHealth.js'

const admin = useAdminStore()
const busy = ref(false)

// #113: tenant options for the source dialog's "adopt from tenant" dropdown.
// Loaded lazily when a source dialog opens; each option carries the tenant id
// and name, and selecting one fills the source's centre+radius from that
// tenant's Standard-Ansicht (applyTenantArea).
const tenantOptions = computed(() =>
  admin.tenants.map((t) => ({ id: t.id, name: t.name })),
)
const tenantAreaBusy = ref(-1)

async function applyTenantArea(i, tenantId) {
  if (tenantId == null) return
  tenantAreaBusy.value = i
  const r = await admin.loadTenantView(tenantId)
  tenantAreaBusy.value = -1
  if (!r.ok || !r.data) {
    sourcesError.value = 'Der Mandant hat noch keine Standard-Ansicht.'
    return
  }
  const s = sources.value[i]
  s.center_lat = r.data.center_lat
  s.center_lon = r.data.center_lon
  // Prefer the stored AOI radius; fall back to a sensible default when the
  // tenant view carries no AOI (centre only), so the fields are never left half-set.
  const derived = r.data.aoi ? bboxToRadius(r.data.aoi) : null
  s.radius_nm = derived ? Math.round(derived.radiusNm) : (s.radius_nm ?? null)
}

const createDialog = ref(false)
const deleteDialog = ref(false)
const target = ref(null)
const form = ref({ name: '', autoEndpoint: true, multicast_group: '', port: 8600, sensorMix: '' })

const canCreate = computed(() => {
  if (form.value.name.trim() === '') return false
  // Auto-allocation needs only a name; a manual override needs group + port.
  if (form.value.autoEndpoint) return true
  return form.value.multicast_group.trim() !== '' && form.value.port > 0
})

async function refresh() {
  busy.value = true
  await Promise.all([admin.loadFeeds(), admin.loadFeedsHealth()])
  busy.value = false
}

function openCreate() {
  form.value = { name: '', autoEndpoint: true, multicast_group: '', port: 8600, sensorMix: '' }
  createDialog.value = true
}

async function submitCreate() {
  busy.value = true
  // ORCH-4: omit group/port to auto-allocate; send them only for a manual override.
  const payload = { name: form.value.name.trim() }
  if (!form.value.autoEndpoint) {
    payload.multicast_group = form.value.multicast_group.trim()
    payload.port = form.value.port
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
  { value: 'adsb_aggregator', label: 'ADS-B (Community-Aggregator)' },
  { value: 'flarm_aprs', label: 'FLARM (OGN/APRS)' },
  { value: 'radar_asterix', label: 'Radar (ASTERIX CAT048/001)' },
]
const AREA_TYPES = new Set(['adsb_opensky', 'adsb_aggregator', 'flarm_aprs'])
function isAreaType(t) {
  return AREA_TYPES.has(t)
}
// Polled sources may carry a poll-interval override (Firefly ADR 0029/0031);
// FLARM is a push stream, radar has its own scan period.
const POLLED_TYPES = new Set(['adsb_opensky', 'adsb_aggregator'])
function isPolledType(t) {
  return POLLED_TYPES.has(t)
}

// Community-aggregator providers (#201; Firefly contract v1.5.0, ADR 0031).
// Human-readable labels only in the UI — the snake_case values stay wire/DB
// internal. airplanes.live is deliberately absent until its radius unit is
// verified (ADR 0031 in Firefly).
const AGG_PROVIDERS = [
  { value: 'adsb_lol', label: 'adsb.lol' },
  { value: 'adsb_fi', label: 'adsb.fi' },
]
const DEFAULT_AGG_PROVIDER = 'adsb_lol'

// OpenSky poll-interval bounds (ADR 0029). Mirrors the server's write-boundary
// range (pkg/store minPollIntervalSecs..maxPollIntervalSecs); the field is
// optional and empty means Firefly's default. Gating is cosmetic — the server
// enforces the range.
const DEFAULT_POLL_SECS = 10
const MIN_POLL_SECS = 5
const MAX_POLL_SECS = 3600

const sourcesDialog = ref(false)
const sourcesTarget = ref(null)
const sources = ref([])
const sourcesError = ref('')
const coveragePreview = ref('')

// Per-feed source credentials (ORCH-2c 3a; ORCH-5b-2). secretStoreEnabled mirrors
// the server: false when no WAYFINDER_SECRET_KEY is configured (the secret routes
// return 503), so the controls stay hidden rather than pretending to work.
// secretRefs holds the cred_refs that already have a stored value (the server
// never returns the value itself). The admin enters a credential as two fields —
// secretUser (client id) and secretPass (client secret), keyed by source row index
// — which are combined into a single "client_id:client_secret" value before storing
// (UX-2; Firefly splits at the first colon and runs the OAuth2 client-credentials
// flow, ADR 0024). secretBusy is the row currently saving.
const secretStoreEnabled = ref(false)
const secretRefs = ref(new Set())
const secretUser = ref({})
const secretPass = ref({})
const secretBusy = ref(-1)

// Credential applicability + operator-facing labels per source type (UX-4). Radar
// (CAT048) is a network endpoint with no auth and the community aggregator
// (#201) is an open service — both get NO credential UI (no map entry). ADS-B
// via OpenSky needs an OAuth2 client (required); FLARM/APRS-IS is optional
// (anonymous read-only works, or a callsign + passcode for an account). Both
// fields still combine into the single "id:secret" value the store keeps
// (credential.js).
const CREDENTIAL = {
  adsb_opensky: { required: true, title: 'OpenSky-Zugang (OAuth2)', user: 'OpenSky Client-ID', pass: 'OpenSky Client-Secret' },
  flarm_aprs: { required: false, title: 'APRS-IS-Zugang (optional)', user: 'APRS-IS Rufzeichen', pass: 'APRS-IS Passcode' },
}
function credInfo(type) { return CREDENTIAL[type] || null }

// ensureCredRef auto-manages a source's credential reference so the operator no
// longer has to invent a handle (UX-4): a credentialled source with no ref gets a
// deterministic one derived from the feed; a non-credentialled source (radar) has
// its ref cleared. An already-persisted ref is kept, so a stored secret stays
// linked to its source.
function ensureCredRef(s) {
  // #198: always (re)derive the ref for a credentialled source so a source-type
  // change (adsb_opensky ↔ flarm_aprs) yields a ref whose suffix matches the NEW
  // type. Previously it was set only when empty, so after switching to FLARM the
  // stale "…-adsb_opensky" ref stuck around — a reference to a secret that never
  // existed, producing endless "secret unresolved" noise in the orchestrator. The
  // ref is deterministic from (feed_id, type), so re-deriving is idempotent for an
  // unchanged type and self-heals an already-broken one on the next load + save.
  if (credInfo(s.type)) {
    s.cred_ref = `secret/feed-${sourcesTarget.value?.id ?? 'new'}-${s.type}`
  } else {
    s.cred_ref = ''
  }
}

function isSecretConfigured(ref) {
  return secretRefs.value.has((ref || '').trim())
}

// secretError surfaces the per-row validation message (empty username/password or
// a colon in the username) so the UI can block an invalid save; '' when valid or
// when nothing has been typed yet.
function secretError(i) {
  const u = secretUser.value[i] || ''
  const p = secretPass.value[i] || ''
  if (!u && !p) return ''
  return validateCredential(u, p)
}

// secretTyped reports whether a complete, VALID credential pair was typed for
// source row i — so submitSources can persist it in the same action (#199) and
// buildSourcesPayload knows an optional source is about to get a secret (#198).
function secretTyped(i) {
  return combineCredential(secretUser.value[i], secretPass.value[i]) !== null
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
  // Combine the two fields into the single "user:pass" value the store keeps;
  // null means the pair is invalid (empty field / colon in username) — never store
  // a malformed value the resolver would later misread. Returns true on success
  // so submitSources can report an honest overall result (#199).
  const value = combineCredential(secretUser.value[i], secretPass.value[i])
  if (!ref || value === null) return false
  secretBusy.value = i
  const r = await admin.setFeedSecret(sourcesTarget.value.id, ref, value)
  secretBusy.value = -1
  if (r.ok) {
    secretRefs.value = new Set(secretRefs.value).add(ref)
    secretUser.value = { ...secretUser.value, [i]: '' }
    secretPass.value = { ...secretPass.value, [i]: '' }
    return true
  }
  sourcesError.value = r.error || 'Secret konnte nicht gespeichert werden.'
  return false
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

// blankSource returns a fresh form entry of the given type. Area sources bind to
// centre+radius (#109/#113; the query bbox is derived on submit); radar binds to
// sac/sic. tenant_id backs the "adopt from tenant" dropdown (never sent).
function blankSource(type = 'adsb_opensky') {
  return {
    type, center_lat: null, center_lon: null, radius_nm: null, tenant_id: null,
    sac: null, sic: null, lat: null, lon: null, height_m: null, listen: '',
    // #201: the provider backs the aggregator's select; harmless on other types
    // (only sent for adsb_aggregator, see buildSourcesPayload).
    provider: DEFAULT_AGG_PROVIDER,
    // #172: prefill the poll interval of a polled source with the visible
    // default so the field shows "10" instead of an empty box — the operator
    // sees which value applies without focusing it. Still editable/clearable
    // (empty ⇒ Firefly default). Push/scan source types have no poll field.
    cred_ref: '', poll_interval_secs: isPolledType(type) ? DEFAULT_POLL_SECS : null,
  }
}

// toFormSource maps a stored source (wire shape, still a bbox) into the form
// model, converting the persisted bbox back into centre+radius so the operator
// edits the same terms as the tenant view (#109). A missing/degenerate bbox
// leaves the centre+radius fields empty.
function toFormSource(s) {
  const cr = s.bbox ? bboxToRadius(s.bbox) : null
  return {
    type: s.type,
    center_lat: cr ? round(cr.centerLat) : null,
    center_lon: cr ? round(cr.centerLon) : null,
    radius_nm: cr ? Math.round(cr.radiusNm) : null,
    tenant_id: null,
    sac: s.sac ?? null,
    sic: s.sic ?? null,
    lat: s.lat ?? null,
    lon: s.lon ?? null,
    height_m: s.height_m ?? null,
    listen: s.listen ?? '',
    provider: s.provider ?? DEFAULT_AGG_PROVIDER,
    cred_ref: s.cred_ref ?? '',
    poll_interval_secs: s.poll_interval_secs ?? null,
  }
}

// round keeps the centre coordinates readable (5 decimals ≈ 1 m) when a stored
// bbox is converted back to centre+radius for display.
function round(n) {
  return Math.round(n * 1e5) / 1e5
}

async function openSources(f) {
  sourcesTarget.value = f
  sources.value = []
  sourcesError.value = ''
  coveragePreview.value = ''
  sourcesDialog.value = true
  busy.value = true
  // #113: populate the "adopt from tenant" dropdown when the dialog opens. Always
  // reload (not just when empty) so a tenant created after the list was first
  // fetched still appears — the earlier lazy guard showed a stale set, missing
  // newly-created tenants. `admin.tenants` (cross-tenant list) is distinct from
  // `admin.overview` (dashboard rows), so it does not refresh on tenant creation.
  admin.loadTenants()
  secretStoreEnabled.value = false
  secretRefs.value = new Set()
  secretUser.value = {}
  secretPass.value = {}
  const r = await admin.loadFeedSources(f.id)
  if (r.ok) {
    sources.value = (r.data.sources || []).map(toFormSource)
    sources.value.forEach(ensureCredRef) // UX-4: auto-manage the credential ref
    coveragePreview.value = formatBBox(r.data.coverage_bbox)
    await loadFeedSecretsState(f.id)
  } else {
    sourcesError.value = r.error || 'Quellen konnten nicht geladen werden.'
  }
  busy.value = false
}

function addSource() {
  const s = blankSource()
  ensureCredRef(s)
  sources.value.push(s)
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
// Area sources are edited as centre+radius (#109) and converted to the query
// bbox the wire contract still expects (radiusNmToBbox); a missing/zero radius
// yields no bbox (the server treats it as unbounded, as before).
function buildSourcesPayload() {
  return {
    sources: sources.value.map((s, i) => {
      const out = { type: s.type }
      if (isAreaType(s.type)) {
        const box = radiusNmToBbox(s.center_lat, s.center_lon, s.radius_nm)
        if (box) {
          out.bbox = box // already the backend wire shape (min_lat/min_lon/max_lat/max_lon)
        }
        // Poll interval applies to the polled sources (ADR 0029/0031); FLARM is
        // a push stream. Send it only when set, so an empty field keeps
        // Firefly's default (10 s).
        if (isPolledType(s.type) && s.poll_interval_secs != null && s.poll_interval_secs !== '') {
          out.poll_interval_secs = Number(s.poll_interval_secs)
        }
        // #201: the aggregator provider — only meaningful (and only accepted by
        // the server) on this type; other types keep the form value to
        // themselves.
        if (s.type === 'adsb_aggregator' && s.provider) {
          out.provider = s.provider
        }
      } else {
        out.sac = s.sac
        out.sic = s.sic
        // #91: radar site — lat/lon required, height_m/listen optional.
        if (s.lat != null && s.lat !== '') out.lat = s.lat
        if (s.lon != null && s.lon !== '') out.lon = s.lon
        if (s.height_m != null && s.height_m !== '') out.height_m = s.height_m
        const listen = (s.listen || '').trim()
        if (listen) out.listen = listen
      }
      // #198: only carry a cred_ref that will actually resolve — a required
      // credential (OpenSky always expects one) or an optional one that has a
      // secret already stored or validly typed (about to be saved, #199). An
      // anonymous FLARM source with no secret sends NO ref, so the orchestrator
      // never logs "secret unresolved" for a reference that was never meant to
      // exist.
      const ref = (s.cred_ref || '').trim()
      const info = credInfo(s.type)
      if (ref && info && (info.required || isSecretConfigured(ref) || secretTyped(i))) {
        out.cred_ref = ref
      }
      return out
    }),
  }
}

async function submitSources() {
  sourcesError.value = ''
  // #199: pre-save validation. A source whose credential is REQUIRED (OpenSky)
  // with neither a stored nor a validly typed secret must NOT be saved — that is
  // the silent failure this fixes (a green toast, but the source runs anonymously
  // and gets rate-limited). Also block an invalid typed secret rather than
  // dropping it. Only meaningful when the secret store is on.
  if (secretStoreEnabled.value) {
    for (let i = 0; i < sources.value.length; i++) {
      const info = credInfo(sources.value[i].type)
      if (!info) continue
      if (secretError(i)) {
        sourcesError.value = `${info.title}: ${secretError(i)}`
        return
      }
      const configured = isSecretConfigured((sources.value[i].cred_ref || '').trim())
      if (info.required && !configured && !secretTyped(i)) {
        sourcesError.value = `${info.title}: Zugang erforderlich, aber weder hinterlegt noch eingegeben.`
        return
      }
    }
  }
  busy.value = true
  const r = await admin.saveFeedSources(sourcesTarget.value.id, buildSourcesPayload())
  if (!r.ok) {
    sourcesError.value = r.error || 'Speichern fehlgeschlagen.'
    busy.value = false
    return
  }
  sources.value = (r.data.sources || []).map(toFormSource)
  sources.value.forEach(ensureCredRef) // UX-4: auto-manage the credential ref
  coveragePreview.value = formatBBox(r.data.coverage_bbox)
  // #199: persist any typed secrets in the SAME action — one "Speichern" click
  // saves the source AND its credential (previously the secret needed a separate,
  // easily-missed button and was silently dropped by the main save).
  let secretsOk = true
  for (let i = 0; i < sources.value.length; i++) {
    if (secretTyped(i) && !(await saveSecret(i))) secretsOk = false
  }
  // #112: the feed row's sensor-mix chips are derived from its sources; reload
  // the catalogue so the change shows immediately, without a manual tab switch.
  await admin.loadFeeds()
  busy.value = false
  // #199: honest result — if a credential failed to persist, surface it in the
  // dialog instead of leaving only the "Quellen gespeichert." toast standing.
  if (!secretsOk && !sourcesError.value) {
    sourcesError.value = 'Quellen gespeichert, aber ein Zugang konnte nicht hinterlegt werden.'
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

// Health chip (AP4): the per-feed colour/label/title come from the shared
// describeFeedHealth helper so all three admin chips read identically; red is
// split into "nie gestartet" (!ever_seen) vs "abgerissen" (stale).
function feedColor(feedId) {
  return describeFeedHealth(admin.feedsHealth[feedId]).color
}

function feedLabel(feedId) {
  return describeFeedHealth(admin.feedsHealth[feedId]).label
}

function feedTitle(feedId) {
  return describeFeedHealth(admin.feedsHealth[feedId]).title
}

onMounted(refresh)
</script>
