<template>
  <!-- ASD-013: Layer & filter panel content.
       Visual hierarchy: subtle uppercase section headers, generous spacing
       between logic blocks, MD3-styled switches with per-group accent colours,
       outlined text fields for the FL range inputs. -->
  <div class="filter-panel">

    <!-- ── Kartenlayer ── -->
    <div class="filter-section-header">Kartenlayer</div>

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

    <div class="filter-row">
      <v-switch
        v-model="store.layerVisibility.coverageRings"
        label="Radarabdeckung"
        color="primary"
        density="compact"
        hide-details
        inset
        @update:model-value="onLayerToggle('coverageRings', $event)"
      />
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

    <!-- ── FL-Filter ── -->
    <div class="filter-section-header filter-section-header--spaced">FL-Filter</div>

    <div class="filter-row filter-row--inputs">
      <v-text-field
        v-model.number="minFL"
        type="number"
        label="Min FL"
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

    <!-- ── Spurherkunft (WF2-40): symbol-shape legend ── -->
    <div class="filter-section-header filter-section-header--spaced">Spurherkunft</div>
    <div class="legend-caption">Form = Herkunft · Farbe = Status</div>
    <div
      v-for="item in provenanceLegend"
      :key="item.label"
      class="filter-row filter-row--sub"
    >
      <span class="prov-glyph">{{ item.glyph }}</span>
      <span class="prov-label">{{ item.label }}</span>
    </div>

  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { useSessionStore } from '@/stores/session.js'
import { AIRSPACE_GROUPS, RANGE_RING_SPACING_OPTIONS_NM, MAX_RANGE_RING_COUNT } from '@/map/constants.js'

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

// ASD-012: local range-ring controls, mirrored into the reactive store on change
// (the engine regenerates the overlay; MapCanvas watches store.rangeRingConfig).
const ringSpacing = ref(store.rangeRingConfig.spacingNM)
const ringCount = ref(store.rangeRingConfig.count)
function onRangeRingChange() {
  store.setRangeRingConfig({ spacingNM: ringSpacing.value, count: ringCount.value })
}

// WF2-40 + Issue #107: track-symbol provenance legend, filtered to the sensor
// classes the tenant's feeds can actually produce (session.sensorClasses, the
// union across subscribed feeds). Glyphs mirror the map icons drawn in layers.js
// (◆ cooperative, ■ SSR/Mode S, ○ primary/PSR); colour is omitted on purpose — it
// encodes track state, not provenance (see caption). FLARM shares the cooperative
// ◆ glyph until distinct FLARM provenance lands (Issue #90). Fallback: when no
// sensor classes are known yet (still loading / admin viewer / no subscribed feed)
// the full legend is shown rather than an empty box.
const PROVENANCE_LEGEND = [
  { glyph: '◆', label: 'ADS-B (kooperativ)', classes: ['ADS-B'] },
  { glyph: '◆', label: 'FLARM', classes: ['FLARM'] },
  { glyph: '■', label: 'SSR / Mode S', classes: ['SSR', 'MODE_S', 'MLAT'] },
  { glyph: '○', label: 'Primär (PSR)', classes: ['PSR'] },
]
const provenanceLegend = computed(() => {
  const active = new Set(session.sensorClasses)
  if (active.size === 0) return PROVENANCE_LEGEND
  return PROVENANCE_LEGEND.filter((e) => e.classes.some((c) => active.has(c)))
})

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
</script>

<style scoped>
.filter-panel {
  padding: 8px 0 16px;
}

/* Section header: small, uppercase, subdued — visual separator, not interactive */
.filter-section-header {
  padding: 10px 14px 4px;
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.09em;
  text-transform: uppercase;
  color: rgba(var(--v-theme-on-surface), 0.45);
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
  line-height: 1;
  color: rgba(var(--v-theme-on-surface), 0.85);
  flex-shrink: 0;
}
.prov-label {
  font-size: 0.8rem;
  opacity: 0.85;
}
</style>
