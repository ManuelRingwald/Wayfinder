<template>
  <!-- ASD-013: Layer & filter panel content.
       Visual hierarchy: subtle uppercase section headers, generous spacing
       between logic blocks, MD3-styled switches with per-group accent colours,
       outlined text fields for the FL range inputs. -->
  <div class="filter-panel">

    <!-- ── Kartenlayer ── -->
    <div class="filter-section-header">Kartenlayer</div>

    <div class="filter-row">
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
    <template v-if="store.layerVisibility.airspace">
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

    <div class="filter-row">
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

    <div class="filter-row">
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

  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { AIRSPACE_GROUPS } from '@/map/constants.js'

const emit = defineEmits(['layer-toggle', 'fl-filter-change'])
const store = useAsdStore()

const minFL = ref(store.flFilter.minFL)
const maxFL = ref(store.flFilter.maxFL)
const hideFiltered = ref(store.flFilter.hide)

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
</style>
