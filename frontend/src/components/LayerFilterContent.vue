<template>
  <!-- ASD-013 / #116: sidebar panel content, split into three sections the
       NavigationRail opens individually (Layer / Filter / Nutzer-Account).
       section='all' (mobile drawer) renders everything in order, account last.
       Visual hierarchy: subtle uppercase section headers, generous spacing
       between logic blocks, MD3-styled switches with per-group accent colours,
       outlined text fields for the FL range inputs. -->
  <div class="filter-panel">

    <!-- ── Layer (#116) ── -->
    <template v-if="showSection('layers')">
      <div class="filter-section-header">Layer</div>

      <!-- Issue #106 (cosmetic feature gate): showLayer(key) hides a layer the
           tenant is not entitled to. Fail-open while the identity is still loading or
           for an admin viewer (gateReady false → show all). The server enforces
           access independently; this is a pure UX gate. -->
      <div v-if="showLayer('airspaces')" class="filter-row">
        <v-switch
          v-model="store.layerVisibility.airspace"
          label="Lufträume"
          color="primary"
          density="compact"
          hide-details
          inset
          @update:model-value="onLayerToggle('airspace', $event)"
        />
      </div>

      <!-- ASD-011: airspace sub-group toggles, indented, coloured per group -->
      <template v-if="(showLayer('airspaces')) && store.layerVisibility.airspace">
        <div
          v-for="group in AIRSPACE_GROUPS"
          :key="group.id"
          class="filter-row filter-row--sub"
        >
          <span class="airspace-dot" :style="{ background: group.color }" />
          <v-switch
            v-model="store.airspaceGroupVisibility[group.id]"
            :label="group.label"
            :color="group.color"
            density="compact"
            hide-details
            inset
            class="sub-switch"
          />
        </div>
      </template>

      <div v-if="showLayer('vor_ndb')" class="filter-row">
        <v-switch
          v-model="store.layerVisibility.navaids"
          label="VOR / NDB"
          color="primary"
          density="compact"
          hide-details
          inset
          @update:model-value="onLayerToggle('navaids', $event)"
        />
      </div>

      <div v-if="showLayer('waypoints')" class="filter-row">
        <v-switch
          v-model="store.layerVisibility.waypoints"
          label="Waypoints"
          color="primary"
          density="compact"
          hide-details
          inset
          @update:model-value="onLayerToggle('waypoints', $event)"
        />
      </div>

      <!-- #114: the coverage overlay only has data when coverage sensors are
           configured server-side. Without data the toggle is disabled with an
           explanatory hint instead of silently doing nothing. -->
      <div class="filter-row">
        <v-switch
          v-model="store.layerVisibility.coverageRings"
          label="Radarabdeckung"
          color="primary"
          density="compact"
          hide-details
          inset
          :disabled="!store.coverageAvailable"
          @update:model-value="onLayerToggle('coverageRings', $event)"
        />
      </div>
      <div v-if="!store.coverageAvailable" class="filter-hint">
        Keine Radarabdeckung konfiguriert — nur bei Radar-Sensoren verfügbar.
      </div>

      <!-- WX-A (ADR 0016): DWD weather-radar overlay. Feature-gated per tenant
           (weather_radar) and disabled when no DWD source is configured
           server-side, with a hint instead of a dead switch. -->
      <div v-if="showLayer('weather_radar')" class="filter-row">
        <v-switch
          v-model="store.layerVisibility.weatherRadar"
          label="DWD-Regenradar"
          color="primary"
          density="compact"
          hide-details
          inset
          :disabled="!store.weatherRadarAvailable"
          @update:model-value="onLayerToggle('weatherRadar', $event)"
        />
      </div>
      <div v-if="showLayer('weather_radar') && !store.weatherRadarAvailable" class="filter-hint">
        Keine DWD-Radarquelle konfiguriert (WAYFINDER_DWD_WMS_URL).
      </div>

      <!-- WX-C (ADR 0016): DWD weather-warnings overlay. Feature-gated per tenant
           (weather_warnings) and disabled when no DWD WFS is configured. -->
      <div v-if="showLayer('weather_warnings')" class="filter-row">
        <v-switch
          v-model="store.layerVisibility.weatherWarnings"
          label="DWD-Wetterwarnungen"
          color="primary"
          density="compact"
          hide-details
          inset
          :disabled="!store.weatherWarningsAvailable"
          @update:model-value="onLayerToggle('weatherWarnings', $event)"
        />
      </div>
      <div v-if="showLayer('weather_warnings') && !store.weatherWarningsAvailable" class="filter-hint">
        Keine DWD-Warnquelle konfiguriert (WAYFINDER_DWD_WARN_URL).
      </div>

      <div v-if="showLayer('history_dots')" class="filter-row">
        <v-switch
          v-model="store.layerVisibility.historyDots"
          label="History Dots"
          color="primary"
          density="compact"
          hide-details
          inset
          @update:model-value="onLayerToggle('historyDots', $event)"
        />
      </div>

      <div v-if="showLayer('range_rings')" class="filter-row">
        <v-switch
          v-model="store.layerVisibility.rangeRings"
          label="Range-Rings"
          color="primary"
          density="compact"
          hide-details
          inset
          @update:model-value="onLayerToggle('rangeRings', $event)"
        />
      </div>

      <!-- ASD-012: range-ring spacing + count, shown only while the layer is active -->
      <template v-if="(showLayer('range_rings')) && store.layerVisibility.rangeRings">
        <div class="filter-row filter-row--sub">
          <v-select
            v-model.number="ringSpacing"
            :items="RANGE_RING_SPACING_OPTIONS_NM"
            label="Abstand (NM)"
            variant="outlined"
            density="compact"
            hide-details
            class="ring-input"
            @update:model-value="onRangeRingChange"
          />
        </div>
        <div class="filter-row filter-row--sub ring-count-row">
          <span class="ring-count-label">Ringe: {{ ringCount }}</span>
          <v-slider
            v-model="ringCount"
            :min="1"
            :max="MAX_RANGE_RING_COUNT"
            :step="1"
            color="primary"
            density="compact"
            hide-details
            class="ring-slider"
            @update:model-value="onRangeRingChange"
          />
        </div>
      </template>

      <!-- ── Spurherkunft (WF2-40/#119): symbol-glyph legend ── -->
      <div class="filter-section-header filter-section-header--spaced">Spurherkunft</div>
      <div class="legend-caption">Symbol = Herkunft · Farbe = Status</div>
      <div
        v-for="item in provenanceLegend"
        :key="item.label"
        class="filter-row filter-row--sub"
      >
        <span class="prov-glyph">{{ item.glyph }}</span>
        <span class="prov-label">{{ item.label }}</span>
      </div>
    </template>

    <!-- ── Filter (#116): the FL filter, hinting the admin-configured band ── -->
    <template v-if="showSection('filters')">
      <div
        class="filter-section-header"
        :class="{ 'filter-section-header--spaced': props.section === 'all' }"
      >Filter</div>

      <div class="filter-row filter-row--inputs">
        <v-text-field
          v-model.number="minFL"
          type="number"
          label="Min FL"
          :placeholder="session.flMin != null ? String(session.flMin) : undefined"
          persistent-placeholder
          min="0"
          max="999"
          step="10"
          variant="outlined"
          density="compact"
          hide-details
          class="fl-input"
          @update:model-value="onFlFilterChange"
        />
        <span class="fl-dash">–</span>
        <v-text-field
          v-model.number="maxFL"
          type="number"
          label="Max FL"
          :placeholder="session.flMax != null ? String(session.flMax) : undefined"
          persistent-placeholder
          min="0"
          max="999"
          step="10"
          variant="outlined"
          density="compact"
          hide-details
          class="fl-input"
          @update:model-value="onFlFilterChange"
        />
      </div>
      <!-- #116: the admissible band from the tenant's Standard-Ansicht (or the
           user's override), greyed as orientation — filtering happens within it. -->
      <div v-if="flRangeHint" class="filter-hint">{{ flRangeHint }}</div>

      <div class="filter-row">
        <v-switch
          v-model="hideFiltered"
          label="Ausblenden"
          color="primary"
          density="compact"
          hide-details
          inset
          @update:model-value="onFlFilterChange"
        />
      </div>
    </template>

    <!-- ── Nutzer-Account (#116): last section, currently logout only ── -->
    <template v-if="showSection('account')">
      <div
        class="filter-section-header"
        :class="{ 'filter-section-header--spaced': props.section === 'all' }"
      >Nutzer-Account</div>
      <div class="filter-row account-row">
        <v-icon size="18" class="account-icon">mdi-account</v-icon>
        <span class="account-subject">{{ session.subject }}</span>
      </div>
      <div class="filter-row">
        <v-btn
          size="small"
          variant="tonal"
          color="primary"
          prepend-icon="mdi-logout"
          block
          @click="onLogout"
        >
          Abmelden
        </v-btn>
      </div>
    </template>

  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { useSessionStore } from '@/stores/session.js'
import { AIRSPACE_GROUPS, RANGE_RING_SPACING_OPTIONS_NM, MAX_RANGE_RING_COUNT } from '@/map/constants.js'
import { filterProvenanceLegend } from '@/map/provenance.js'

// #116: the NavigationRail opens one section at a time on desktop; the mobile
// drawer renders all of them ('all'), with the account block last.
const props = defineProps({
  section: { type: String, default: 'all' },
})
function showSection(id) {
  return props.section === 'all' || props.section === id
}

const emit = defineEmits(['layer-toggle', 'fl-filter-change'])
const store = useAsdStore()
const session = useSessionStore()

// Issue #106: show a lotse only the layers/filters their tenant is entitled to.
// The gate is driven by the role-agnostic session identity (whoami → features),
// which is populated for a plain tenant user (the admin store's role probe is not).
// gateReady is true only for an authenticated tenant user; for the loading/anon
// state or an admin/platform viewer with no tenant scope we show everything
// (fail-open cosmetic gate — the server enforces access independently).
const gateReady = computed(() => session.status === 'authed' && !session.isAdmin)
function showLayer(featureKey) {
  return !gateReady.value || session.hasFeature(featureKey)
}

const minFL = ref(store.flFilter.minFL)
const maxFL = ref(store.flFilter.maxFL)
const hideFiltered = ref(store.flFilter.hide)

// #116: admissible FL band from the effective view config (whoami), greyed as
// a hint under the filter inputs. Empty string when nothing is configured.
const flRangeHint = computed(() => {
  if (session.flMin == null && session.flMax == null) return ''
  const lo = session.flMin ?? 0
  const hi = session.flMax != null ? `FL ${session.flMax}` : 'unbegrenzt'
  return `Zulässiger Bereich: FL ${lo} – ${hi}`
})

// ASD-012: local range-ring controls, mirrored into the reactive store on change
// (the engine regenerates the overlay; MapCanvas watches store.rangeRingConfig).
const ringSpacing = ref(store.rangeRingConfig.spacingNM)
const ringCount = ref(store.rangeRingConfig.count)
function onRangeRingChange() {
  store.setRangeRingConfig({ spacingNM: ringSpacing.value, count: ringCount.value })
}

// WF2-40 + Issues #107/#119: track-symbol provenance legend, filtered to the
// sensor classes the tenant's feeds can actually produce (session.sensorClasses,
// the union across subscribed feeds). Glyphs mirror the map icons drawn in
// layers.js — ◆ ADS-B, ■ SSR/Mode S, ● primary/PSR (geometric marks per the
// design legend), F for FLARM (#119); colour is omitted on purpose — it encodes
// track state, not provenance (see caption). Fallback: when no sensor classes are known yet
// (still loading / admin viewer / no subscribed feed) the full legend is shown
// rather than an empty box.
// #125: the "Kombiniert" (K) entry is appended by filterProvenanceLegend when ≥2
// sources can contribute (single source of truth in map/provenance.js).
const provenanceLegend = computed(() => filterProvenanceLegend(session.sensorClasses))

function onLayerToggle(layer, val) {
  store.setLayerVisibility(layer, val)
  emit('layer-toggle', { layer, val })
}

function onFlFilterChange() {
  const update = {
    minFL: minFL.value || null,
    maxFL: maxFL.value || null,
    hide: hideFiltered.value,
  }
  store.setFlFilter(update)
  emit('fl-filter-change', update)
}

// #116: logout from the account section. The session store flips to 'anon' and
// AsdView swaps the scope for the login screen.
async function onLogout() {
  await session.logout()
}
</script>

<style scoped>
.filter-panel {
  padding: 8px 0 16px;
}

/* Section header: small, uppercase, subdued — visual separator, not interactive */
.filter-section-header {
  /* Design System v1: the signature "overline" section header, token-driven. */
  padding: 10px 14px 4px;
  font-size: var(--wf-overline-size);
  font-weight: var(--wf-overline-weight);
  letter-spacing: var(--wf-overline-tracking);
  text-transform: uppercase;
  color: var(--wf-overline-color);
  line-height: 1.4;
}

/* Extra top margin before a new logic block */
.filter-section-header--spaced {
  margin-top: 12px;
  border-top: 1px solid rgba(var(--v-border-color), 0.12);
  padding-top: 14px;
}

/* Standard filter row */
.filter-row {
  display: flex;
  align-items: center;
  padding: 2px 10px 2px 12px;
  min-height: 36px;
}

/* Sub-group row: indented with coloured dot */
.filter-row--sub {
  padding-left: 20px;
  min-height: 32px;
}

/* Greyed hint line under a control (admissible FL band, disabled coverage) */
.filter-hint {
  padding: 0 14px 6px;
  font-size: 10.5px;
  line-height: 1.35;
  color: rgba(var(--v-theme-on-surface), 0.45);
}

/* Coloured dot that mirrors the airspace colour on the map */
.airspace-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-right: 6px;
}

/* Sub-switch: slightly smaller label */
.sub-switch :deep(.v-label) {
  font-size: 0.8rem;
  opacity: 0.85;
}

/* FL input pair */
.filter-row--inputs {
  gap: 6px;
  padding: 6px 12px 4px;
  align-items: center;
}
.fl-input {
  flex: 1;
  min-width: 0;
}
.fl-dash {
  font-size: 0.9rem;
  color: rgba(var(--v-theme-on-surface), 0.4);
  flex-shrink: 0;
}

/* Tighten the switch track to be proportional and not oversized */
:deep(.v-switch .v-selection-control) {
  min-height: unset;
}
:deep(.v-switch .v-switch__track) {
  height: 14px;
  width: 28px;
  border-radius: 7px;
}
:deep(.v-switch .v-switch__thumb) {
  width: 10px;
  height: 10px;
}

/* ASD-012 range-ring controls */
.ring-input {
  flex: 1;
  min-width: 0;
}
.ring-count-row {
  flex-direction: column;
  align-items: stretch;
  gap: 2px;
  padding-top: 4px;
}
.ring-count-label {
  font-size: 0.78rem;
  opacity: 0.8;
}
.ring-slider {
  margin: 0 4px;
}

/* WF2-40 provenance legend */
.legend-caption {
  padding: 0 14px 6px;
  font-size: 10px;
  color: rgba(var(--v-theme-on-surface), 0.45);
}
.prov-glyph {
  width: 14px;
  margin-right: 8px;
  text-align: center;
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
  color: rgba(var(--v-theme-on-surface), 0.85);
  flex-shrink: 0;
}
.prov-label {
  font-size: 0.8rem;
  opacity: 0.85;
}

/* #116 account section */
.account-row {
  gap: 8px;
}
.account-icon {
  opacity: 0.7;
}
.account-subject {
  font-size: 0.85rem;
  opacity: 0.9;
}
</style>
