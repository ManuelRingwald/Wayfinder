<template>
  <!-- Panel content for the "Layer & Filter" section of the Navigation Drawer.
       ASD-011: airspace layer includes per-group sub-toggles (CTR, TMA/CTA,
       ED-R/ED-D, FIS/RMZ/TMZ) that are disabled when the master toggle is off. -->
  <v-list density="compact" nav class="pt-1 pb-2">

    <!-- ── Kartenlayer ── -->
    <v-list-subheader class="text-primary font-weight-medium">
      Kartenlayer
    </v-list-subheader>

    <!-- Airspace master toggle -->
    <v-list-item class="py-0">
      <v-switch
        v-model="store.layerVisibility.airspace"
        label="Lufträume"
        density="compact"
        hide-details
        @update:model-value="onLayerToggle('airspace', $event)"
      />
    </v-list-item>

    <!-- ASD-011: airspace sub-group toggles, indented under the master -->
    <template v-if="store.layerVisibility.airspace">
      <v-list-item
        v-for="group in AIRSPACE_GROUPS"
        :key="group.id"
        class="py-0 pl-4"
      >
        <div class="d-flex align-center gap-2">
          <span
            class="airspace-dot"
            :style="{ background: group.color }"
          />
          <v-switch
            v-model="store.airspaceGroupVisibility[group.id]"
            :label="group.label"
            :color="group.color"
            density="compact"
            hide-details
            class="airspace-sub-switch"
          />
        </div>
      </v-list-item>
    </template>

    <!-- VOR / NDB -->
    <v-list-item class="py-0">
      <v-switch
        v-model="store.layerVisibility.navaids"
        label="VOR / NDB"
        density="compact"
        hide-details
        @update:model-value="onLayerToggle('navaids', $event)"
      />
    </v-list-item>

    <!-- Waypoints -->
    <v-list-item class="py-0">
      <v-switch
        v-model="store.layerVisibility.waypoints"
        label="Waypoints"
        density="compact"
        hide-details
        @update:model-value="onLayerToggle('waypoints', $event)"
      />
    </v-list-item>

    <v-divider class="my-2" />

    <!-- ── FL-Filter ── -->
    <v-list-subheader class="text-primary font-weight-medium">
      FL-Filter
    </v-list-subheader>

    <v-list-item class="py-0">
      <div class="d-flex align-center gap-2 px-1 pt-1">
        <v-text-field
          v-model.number="minFL"
          type="number"
          label="Min"
          min="0"
          max="999"
          step="10"
          style="width: 76px"
          density="compact"
          hide-details
          @update:model-value="onFlFilterChange"
        />
        <span class="text-medium-emphasis text-body-2">–</span>
        <v-text-field
          v-model.number="maxFL"
          type="number"
          label="Max"
          min="0"
          max="999"
          step="10"
          style="width: 76px"
          density="compact"
          hide-details
          @update:model-value="onFlFilterChange"
        />
      </div>
    </v-list-item>

    <v-list-item class="py-0">
      <v-switch
        v-model="hideFiltered"
        label="Ausblenden"
        density="compact"
        hide-details
        @update:model-value="onFlFilterChange"
      />
    </v-list-item>

  </v-list>
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
.airspace-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.airspace-sub-switch {
  font-size: 0.8rem;
}
.airspace-sub-switch :deep(.v-label) {
  font-size: 0.8rem;
  opacity: 0.9;
}
</style>
