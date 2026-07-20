<template>
  <!-- K1 (#309, Epic #307): the "Kartendaten" admin area — one place for the four
       map-data sources (Basiskarte / Wetter / Radar-Abdeckung / Aeronautik).
       K1 shows a read-only STATUS/diagnostics view (is a source configured? is it
       available?) sourced from /api/map-config + the OpenAIP status; live EDITING
       arrives per subsystem in K2–K5. The Aeronautik tab embeds the existing
       global-OpenAIP panel (AERO-2, ADR 0018) — no duplication. -->
  <div>
    <v-tabs v-model="tab" color="primary" class="mb-4">
      <v-tab value="basemap" prepend-icon="mdi-map">Basiskarte</v-tab>
      <v-tab value="weather" prepend-icon="mdi-weather-partly-rainy">Wetter</v-tab>
      <v-tab value="coverage" prepend-icon="mdi-radar">Radar-Abdeckung</v-tab>
      <v-tab value="aero" prepend-icon="mdi-airplane-marker">Aeronautik</v-tab>
    </v-tabs>

    <v-alert type="info" variant="tonal" density="compact" class="mb-4">
      Übersicht der Karten-Datenquellen. Die Werte werden heute über
      Umgebungsvariablen gesetzt und hier zur Kontrolle angezeigt; das Bearbeiten
      direkt in der UI folgt je Quelle (Basiskarte, Wetter, Abdeckung).
    </v-alert>

    <v-window v-model="tab">
      <!-- ── Basiskarte (K2 #310: live editierbar) ── -->
      <v-window-item value="basemap">
        <v-card variant="tonal">
          <v-card-title class="text-subtitle-1 d-flex align-center ga-2">
            Basiskarte (BKG)
            <v-chip size="x-small" :color="basemap.overridden ? 'primary' : 'default'" variant="tonal">
              {{ basemap.overridden ? 'überschrieben' : 'Standard (Env)' }}
            </v-chip>
          </v-card-title>
          <v-card-text>
            <p class="text-body-2 text-medium-emphasis mb-3">
              Style-URL und Theme werden live angewandt (der Server holt den Style
              neu; ein leeres Feld setzt auf den Umgebungs-Standard zurück). Die
              Änderung greift, sobald der Lotse die Karte neu lädt.
            </p>

            <div class="d-flex flex-wrap ga-3 align-center mb-3">
              <v-select
                v-model="themeInput"
                :items="['bkg', 'bkg-dark']"
                label="Theme"
                variant="outlined"
                density="compact"
                hide-details
                style="max-width: 200px"
              />
              <v-btn color="primary" :loading="busy" @click="saveTheme">Theme speichern</v-btn>
            </div>

            <div class="d-flex flex-wrap ga-3 align-center">
              <v-text-field
                v-model="styleInput"
                label="Style-URL"
                :placeholder="basemap.style_default"
                persistent-placeholder
                variant="outlined"
                density="compact"
                hide-details
                style="min-width: 340px; flex: 1"
              />
              <v-btn color="primary" :loading="busy" :disabled="!styleInput" @click="saveStyle">URL speichern</v-btn>
              <v-btn v-if="basemap.style_overridden" color="error" variant="tonal" :loading="busy" @click="resetStyle">
                Auf Standard
              </v-btn>
            </div>

            <v-alert v-if="basemap.reloadError" type="warning" variant="tonal" density="compact" class="mt-3">
              Gespeichert, aber der Dienst konnte nicht neu laden (letzte gute
              Konfiguration bleibt aktiv): {{ basemap.reloadError }}
            </v-alert>

            <div class="mapdata-hint">
              Pro Mandant zusätzlich: Freigabe <code>basemap</code> + AOI-Zuschnitt
              (ADR 0027/0032).
            </div>
          </v-card-text>
        </v-card>
      </v-window-item>

      <!-- ── Wetter (K3 #311: live editierbar) ── -->
      <v-window-item value="weather">
        <v-card variant="tonal">
          <v-card-title class="text-subtitle-1">Wetter (DWD / QNH)</v-card-title>
          <v-card-text>
            <v-alert type="info" variant="tonal" density="compact" class="mb-4">
              An/Aus wirkt <strong>sofort</strong> (der Lotse sieht die Quelle nach
              einem Neuladen der Karte). Geänderte URLs/Layer greifen erst beim
              <strong>nächsten Neustart</strong> des Servers — ein laufender
              Abruf-Dienst wird nicht im Betrieb umkonfiguriert.
            </v-alert>

            <!-- DWD-Regenradar -->
            <div class="mapdata-src">
              <div class="d-flex align-center ga-2 mb-2">
                <v-switch v-model="weather.radarEnabled" label="DWD-Regenradar" color="primary" density="compact" hide-details inset />
                <v-chip size="small" variant="tonal" :color="statusColor(cfg.weather_radar_available)">
                  {{ statusText(cfg.weather_radar_available) }}
                </v-chip>
              </div>
              <div class="d-flex flex-wrap ga-3">
                <v-text-field v-model="weather.radarURL" label="WMS-URL" :placeholder="weather.radarDefault" persistent-placeholder variant="outlined" density="compact" hide-details style="min-width: 320px; flex: 2" />
                <v-text-field v-model="weather.radarLayer" label="Layer" variant="outlined" density="compact" hide-details style="min-width: 160px; flex: 1" />
              </div>
            </div>

            <!-- DWD-Wetterwarnungen -->
            <div class="mapdata-src">
              <div class="d-flex align-center ga-2 mb-2">
                <v-switch v-model="weather.warnEnabled" label="DWD-Wetterwarnungen" color="primary" density="compact" hide-details inset />
                <v-chip size="small" variant="tonal" :color="statusColor(cfg.weather_warnings_available)">
                  {{ statusText(cfg.weather_warnings_available) }}
                </v-chip>
              </div>
              <div class="d-flex flex-wrap ga-3">
                <v-text-field v-model="weather.warnURL" label="WFS/WMS-URL" :placeholder="weather.warnDefault" persistent-placeholder variant="outlined" density="compact" hide-details style="min-width: 320px; flex: 2" />
                <v-text-field v-model="weather.warnLayer" label="Layer" variant="outlined" density="compact" hide-details style="min-width: 160px; flex: 1" />
              </div>
            </div>

            <!-- QNH (METAR) -->
            <div class="mapdata-src">
              <div class="d-flex align-center ga-2">
                <v-switch v-model="weather.qnhEnabled" label="QNH (METAR)" color="primary" density="compact" hide-details inset />
                <v-chip size="small" variant="tonal" :color="statusColor(cfg.qnh_available)">
                  {{ statusText(cfg.qnh_available) }}
                </v-chip>
              </div>
            </div>

            <div class="d-flex ga-3 align-center mt-4">
              <v-spacer />
              <v-btn color="primary" :loading="busy" @click="saveWeather">Speichern</v-btn>
            </div>

            <v-alert v-if="weather.error" type="warning" variant="tonal" density="compact" class="mt-3">
              {{ weather.error }}
            </v-alert>
            <div class="mapdata-hint">
              „Verfügbar" = Quelle aktiviert und URL konfiguriert. Pro Mandant
              zusätzlich: Freigaben + AOI-Zuschnitt.
            </div>
          </v-card-text>
        </v-card>
      </v-window-item>

      <!-- ── Radar-/Luftlageabdeckung (K4 #312: Sensor-CRUD) ── -->
      <v-window-item value="coverage">
        <v-card variant="tonal">
          <v-card-title class="text-subtitle-1 d-flex align-center ga-2">
            Radar-/Luftlageabdeckung
            <v-chip size="x-small" :color="coverage.overridden ? 'primary' : 'default'" variant="tonal">
              {{ coverage.overridden ? 'überschrieben' : 'Standard (Env)' }}
            </v-chip>
          </v-card-title>
          <v-card-text>
            <p class="text-body-2 text-medium-emphasis mb-3">
              Reichweiten-Ringe der Radar-Standorte. Änderungen greifen live (die
              Abdeckungs-GeoJSON wird neu berechnet). Ohne Sensoren ist der Layer
              im ASD nicht verfügbar.
            </p>

            <div v-for="(s, i) in sensors" :key="i" class="coverage-sensor">
              <v-text-field v-model.number="s.Lat" label="Lat" type="number" density="compact" variant="outlined" hide-details style="width: 90px" />
              <v-text-field v-model.number="s.Lon" label="Lon" type="number" density="compact" variant="outlined" hide-details style="width: 90px" />
              <v-text-field v-model.number="s.MinRangeM" label="Min (m)" type="number" density="compact" variant="outlined" hide-details style="width: 100px" />
              <v-text-field v-model.number="s.MaxRangeM" label="Max (m)" type="number" density="compact" variant="outlined" hide-details style="width: 100px" />
              <v-text-field v-model="s.Label" label="Label" density="compact" variant="outlined" hide-details style="flex: 1; min-width: 100px" />
              <v-btn icon="mdi-delete" size="small" variant="text" color="error" @click="sensors.splice(i, 1)" />
            </div>

            <div class="d-flex flex-wrap ga-3 align-center mt-3">
              <v-btn size="small" variant="tonal" prepend-icon="mdi-plus" @click="addSensor">Sensor</v-btn>
              <v-text-field v-model="ringColor" label="Ringfarbe" density="compact" variant="outlined" hide-details style="width: 140px" />
              <span class="mapdata-swatch" :style="{ background: ringColor || '#5B8DEF' }" />
              <v-spacer />
              <v-btn color="primary" :loading="busy" @click="saveCoverage">Speichern</v-btn>
              <v-btn v-if="coverage.overridden" color="error" variant="tonal" :loading="busy" @click="resetCoverage">Auf Standard</v-btn>
            </div>

            <v-alert v-if="coverage.error" type="warning" variant="tonal" density="compact" class="mt-3">
              {{ coverage.error }}
            </v-alert>
          </v-card-text>
        </v-card>
      </v-window-item>

      <!-- ── Aeronautik (OpenAIP) — bestehendes Panel eingegliedert ── -->
      <v-window-item value="aero">
        <AdminGlobalOpenAIP />
      </v-window-item>
    </v-window>

    <v-alert v-if="loadError" type="warning" variant="tonal" density="compact" class="mt-4">
      Status konnte nicht geladen werden: {{ loadError }}
    </v-alert>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { apiFetch } from '@/api.js'
import AdminGlobalOpenAIP from '@/components/admin/AdminGlobalOpenAIP.vue'

const tab = ref('basemap')
const cfg = ref({})
const loadError = ref(null)
const busy = ref(false)

// K2 (#310): live base-map editing. themeInput/styleInput are the form fields;
// `basemap` holds the loaded setting state (overridden flags, env default, a
// reload warning). The two settings are the K0 mapconfig endpoints.
const themeInput = ref('bkg-dark')
const styleInput = ref('')
const basemap = ref({ overridden: false, style_overridden: false, style_default: '', reloadError: '' })

// The status view reads the SAME /api/map-config the ASD reads at start-up, so
// the admin sees exactly what the scope sees (single source of truth).
onMounted(async () => {
  try {
    const r = await apiFetch('/api/map-config')
    if (r?.ok && r.data) cfg.value = r.data
    else loadError.value = 'unerwartete Antwort'
  } catch (e) {
    loadError.value = e?.message || 'Netzwerkfehler'
  }
  await Promise.all([loadBasemap(), loadCoverage(), loadWeather()])
})

async function loadBasemap() {
  const [theme, style] = await Promise.all([
    apiFetch('/api/admin/mapdata/basemap/theme'),
    apiFetch('/api/admin/mapdata/basemap/style-url'),
  ])
  if (theme?.ok && theme.data) {
    themeInput.value = theme.data.value || 'bkg-dark'
    basemap.value.overridden = !!theme.data.overridden
  }
  if (style?.ok && style.data) {
    styleInput.value = style.data.overridden ? style.data.value : ''
    basemap.value.style_overridden = !!style.data.overridden
    basemap.value.style_default = style.data.default || ''
  }
}

async function putSetting(path, value) {
  busy.value = true
  basemap.value.reloadError = ''
  try {
    const r = await apiFetch(path, { method: 'PUT', body: JSON.stringify({ value }) })
    if (r?.ok && r.data?.reload_error) basemap.value.reloadError = r.data.reload_error
    await loadBasemap()
  } finally {
    busy.value = false
  }
}

const saveTheme = () => putSetting('/api/admin/mapdata/basemap/theme', themeInput.value)
const saveStyle = () => putSetting('/api/admin/mapdata/basemap/style-url', styleInput.value)
const resetStyle = () => putSetting('/api/admin/mapdata/basemap/style-url', '')

// K4 (#312): coverage sensor CRUD.
const sensors = ref([])
const ringColor = ref('#5B8DEF')
const coverage = ref({ overridden: false, error: '' })

async function loadCoverage() {
  const r = await apiFetch('/api/admin/mapdata/coverage')
  if (r?.ok && r.data) {
    sensors.value = (r.data.sensors || []).map((s) => ({ ...s }))
    ringColor.value = r.data.ring_color || '#5B8DEF'
    coverage.value.overridden = !!r.data.overridden
  }
}
function addSensor() {
  sensors.value.push({ Lat: 50, Lon: 8, MinRangeM: 0, MaxRangeM: 100000, Label: '' })
}
async function saveCoverage() {
  busy.value = true
  coverage.value.error = ''
  try {
    const r = await apiFetch('/api/admin/mapdata/coverage', {
      method: 'PUT',
      body: JSON.stringify({ sensors: sensors.value, ring_color: ringColor.value }),
    })
    if (!r?.ok) coverage.value.error = r?.error || 'Speichern fehlgeschlagen (Werte prüfen).'
    await loadCoverage()
  } finally {
    busy.value = false
  }
}
async function resetCoverage() {
  // DELETE resets to the env default (distinct from a PUT with an empty list,
  // which is an explicit "zero sensors" override).
  busy.value = true
  try {
    await apiFetch('/api/admin/mapdata/coverage', { method: 'DELETE' })
    await loadCoverage()
  } finally {
    busy.value = false
  }
}

const sensorCount = computed(() => Number(cfg.value.coverage_sensor_count ?? 0))

// K3 (#311): weather live editing. Enable/disable + availability are LIVE; the
// URL/layer overrides are stored now and applied at the next server restart
// (honest note in the UI). Each field is its own K0 mapconfig endpoint.
const weather = ref({
  radarEnabled: false, radarURL: '', radarLayer: '', radarDefault: '',
  warnEnabled: false, warnURL: '', warnLayer: '', warnDefault: '',
  qnhEnabled: false,
  error: '',
})

const wBase = '/api/admin/mapdata/weather'
const asBool = (v) => String(v).toLowerCase() === 'true'

async function loadWeather() {
  const paths = ['radar-enabled', 'radar-url', 'radar-layer', 'warn-enabled', 'warn-url', 'warn-layer', 'qnh-enabled']
  const [rEn, rUrl, rLayer, wEn, wUrl, wLayer, qEn] = await Promise.all(
    paths.map((p) => apiFetch(`${wBase}/${p}`)),
  )
  const val = (r) => (r?.ok && r.data ? r.data.value ?? '' : '')
  const def = (r) => (r?.ok && r.data ? r.data.default ?? '' : '')
  weather.value.radarEnabled = asBool(val(rEn))
  weather.value.radarURL = val(rUrl)
  weather.value.radarDefault = def(rUrl)
  weather.value.radarLayer = val(rLayer)
  weather.value.warnEnabled = asBool(val(wEn))
  weather.value.warnURL = val(wUrl)
  weather.value.warnDefault = def(wUrl)
  weather.value.warnLayer = val(wLayer)
  weather.value.qnhEnabled = asBool(val(qEn))
}

async function saveWeather() {
  busy.value = true
  weather.value.error = ''
  const put = (p, value) => apiFetch(`${wBase}/${p}`, { method: 'PUT', body: JSON.stringify({ value }) })
  try {
    const results = await Promise.all([
      put('radar-enabled', boolStr(weather.value.radarEnabled)),
      put('radar-url', weather.value.radarURL),
      put('radar-layer', weather.value.radarLayer),
      put('warn-enabled', boolStr(weather.value.warnEnabled)),
      put('warn-url', weather.value.warnURL),
      put('warn-layer', weather.value.warnLayer),
      put('qnh-enabled', boolStr(weather.value.qnhEnabled)),
    ])
    if (results.some((r) => !r?.ok)) {
      weather.value.error = 'Nicht alle Werte konnten gespeichert werden (URL-Format prüfen).'
    }
    // Refresh availability (LIVE) + the stored values.
    const r = await apiFetch('/api/map-config')
    if (r?.ok && r.data) cfg.value = r.data
    await loadWeather()
  } finally {
    busy.value = false
  }
}

const boolStr = (b) => (b ? 'true' : 'false')

function statusColor(ok) { return ok ? 'success' : 'default' }
function statusText(ok) { return ok ? 'verfügbar' : 'nicht konfiguriert' }
</script>

<style scoped>
.mapdata-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 36px;
}
.mapdata-label {
  min-width: 200px;
  font-size: 0.9rem;
  color: rgba(var(--v-theme-on-surface), 0.8);
}
.mapdata-value {
  font-size: 0.9rem;
  word-break: break-all;
}
.mapdata-swatch {
  width: 14px;
  height: 14px;
  border-radius: 3px;
  display: inline-block;
  flex-shrink: 0;
}
.coverage-sensor {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid rgba(var(--v-border-color), 0.12);
}
.mapdata-src {
  padding: 12px 0;
  border-bottom: 1px solid rgba(var(--v-border-color), 0.12);
}
.mapdata-hint {
  margin-top: 10px;
  font-size: 0.8rem;
  color: rgba(var(--v-theme-on-surface), 0.55);
}
</style>
