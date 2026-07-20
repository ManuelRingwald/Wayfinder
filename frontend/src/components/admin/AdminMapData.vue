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
      <!-- ── Basiskarte ── -->
      <v-window-item value="basemap">
        <v-card variant="tonal">
          <v-card-title class="text-subtitle-1">Basiskarte (BKG)</v-card-title>
          <v-card-text>
            <div class="mapdata-row">
              <span class="mapdata-label">Theme</span>
              <v-chip size="small" variant="tonal" color="primary">{{ cfg.theme || '—' }}</v-chip>
            </div>
            <div class="mapdata-row">
              <span class="mapdata-label">Style-URL</span>
              <span class="mapdata-value">{{ cfg.style || '—' }}</span>
            </div>
            <div class="mapdata-hint">
              Pro Mandant zusätzlich: Freigabe <code>basemap</code> + AOI-Zuschnitt
              (ADR 0027/0032). Editieren folgt in einem späteren Schritt.
            </div>
          </v-card-text>
        </v-card>
      </v-window-item>

      <!-- ── Wetter ── -->
      <v-window-item value="weather">
        <v-card variant="tonal">
          <v-card-title class="text-subtitle-1">Wetter (DWD / QNH)</v-card-title>
          <v-card-text>
            <div class="mapdata-row">
              <span class="mapdata-label">DWD-Regenradar</span>
              <v-chip size="small" variant="tonal" :color="statusColor(cfg.weather_radar_available)">
                {{ statusText(cfg.weather_radar_available) }}
              </v-chip>
            </div>
            <div class="mapdata-row">
              <span class="mapdata-label">DWD-Wetterwarnungen</span>
              <v-chip size="small" variant="tonal" :color="statusColor(cfg.weather_warnings_available)">
                {{ statusText(cfg.weather_warnings_available) }}
              </v-chip>
            </div>
            <div class="mapdata-row">
              <span class="mapdata-label">QNH (METAR)</span>
              <v-chip size="small" variant="tonal" :color="statusColor(cfg.qnh_available)">
                {{ statusText(cfg.qnh_available) }}
              </v-chip>
            </div>
            <div class="mapdata-hint">
              „Verfügbar" = Quelle konfiguriert und aktiv. Pro Mandant zusätzlich:
              Freigaben + AOI-Zuschnitt.
            </div>
          </v-card-text>
        </v-card>
      </v-window-item>

      <!-- ── Radar-/Luftlageabdeckung ── -->
      <v-window-item value="coverage">
        <v-card variant="tonal">
          <v-card-title class="text-subtitle-1">Radar-/Luftlageabdeckung</v-card-title>
          <v-card-text>
            <div class="mapdata-row">
              <span class="mapdata-label">Konfigurierte Sensoren</span>
              <v-chip size="small" variant="tonal" :color="statusColor(sensorCount > 0)">
                {{ sensorCount }}
              </v-chip>
            </div>
            <div class="mapdata-row">
              <span class="mapdata-label">Ringfarbe</span>
              <span class="mapdata-value d-flex align-center ga-2">
                <span class="mapdata-swatch" :style="{ background: cfg.coverage_ring_color || '#5B8DEF' }" />
                {{ cfg.coverage_ring_color || '—' }}
              </span>
            </div>
            <div class="mapdata-hint">
              Ohne konfigurierte Sensoren ist der Abdeckungs-Layer im ASD nicht
              verfügbar. Sensor-Pflege in der UI folgt in einem späteren Schritt.
            </div>
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

// The status view reads the SAME /api/map-config the ASD reads at start-up, so
// the admin sees exactly what the scope sees (single source of truth). K1 is
// read-only; K2–K5 add the editing endpoints (mapconfig plane, K0).
onMounted(async () => {
  try {
    const r = await apiFetch('/api/map-config')
    if (r?.ok && r.data) cfg.value = r.data
    else loadError.value = 'unerwartete Antwort'
  } catch (e) {
    loadError.value = e?.message || 'Netzwerkfehler'
  }
})

const sensorCount = computed(() => Number(cfg.value.coverage_sensor_count ?? 0))

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
.mapdata-hint {
  margin-top: 10px;
  font-size: 0.8rem;
  color: rgba(var(--v-theme-on-surface), 0.55);
}
</style>
