<template>
  <!-- ASD-008: Navigation Rail + expandable secondary panel.
       On desktop the rail is always visible (56 px, icons + tooltips).
       Clicking a rail icon expands it to a 240 px secondary panel with the
       relevant controls. On mobile it reverts to a temporary overlay drawer
       triggered by the App Bar hamburger. -->
  <v-navigation-drawer
    v-model="drawerOpen"
    :permanent="mdAndUp"
    :temporary="!mdAndUp"
    :rail="railCollapsed"
    rail-width="56"
    width="240"
    color="surface"
  >
    <!-- Rail icon list — always visible regardless of expand state -->
    <v-list nav density="compact" class="pa-1">
      <v-list-item
        v-for="section in sections"
        :key="section.id"
        :value="section.id"
        :prepend-icon="section.icon"
        :active="activeSection === section.id"
        active-color="primary"
        rounded="lg"
        class="mb-1"
        @click="toggleSection(section.id)"
      >
        <template #title>
          <span v-if="!railCollapsed">{{ section.label }}</span>
        </template>
        <!-- Tooltip shown only in collapsed rail mode -->
        <v-tooltip
          v-if="railCollapsed"
          activator="parent"
          location="right"
          :text="section.label"
        />
      </v-list-item>
    </v-list>

    <!-- Panel content — only rendered when a section is open -->
    <template v-if="!railCollapsed">
      <v-divider class="my-1" />

      <!-- Layer controls panel -->
      <template v-if="activeSection === 'layers'">
        <v-list density="compact" nav class="pt-1">
          <v-list-subheader class="text-primary font-weight-medium">
            Kartenlayer
          </v-list-subheader>
          <v-list-item>
            <v-switch
              v-model="store.layerVisibility.airspace"
              label="Lufträume"
              @update:model-value="onLayerToggle('airspace', $event)"
            />
          </v-list-item>
          <v-list-item>
            <v-switch
              v-model="store.layerVisibility.navaids"
              label="VOR / NDB"
              @update:model-value="onLayerToggle('navaids', $event)"
            />
          </v-list-item>
          <v-list-item>
            <v-switch
              v-model="store.layerVisibility.waypoints"
              label="Waypoints"
              @update:model-value="onLayerToggle('waypoints', $event)"
            />
          </v-list-item>
        </v-list>

        <v-divider class="my-1" />

        <v-list density="compact" nav class="pt-1">
          <v-list-subheader class="text-primary font-weight-medium">
            FL-Filter
          </v-list-subheader>
          <v-list-item>
            <div class="d-flex align-center gap-2 px-1">
              <v-text-field
                v-model.number="minFL"
                type="number"
                label="Min"
                min="0"
                max="999"
                step="10"
                style="width: 76px"
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
                @update:model-value="onFlFilterChange"
              />
            </div>
          </v-list-item>
          <v-list-item>
            <v-switch
              v-model="hideFiltered"
              label="Ausblenden"
              @update:model-value="onFlFilterChange"
            />
          </v-list-item>
        </v-list>
      </template>
    </template>

    <!-- Collapse toggle at bottom (desktop only) -->
    <template #append>
      <v-divider v-if="mdAndUp" />
      <v-list nav density="compact" class="pa-1" v-if="mdAndUp">
        <v-list-item
          :prepend-icon="railCollapsed ? 'mdi-chevron-right' : 'mdi-chevron-left'"
          rounded="lg"
          @click="railCollapsed = !railCollapsed"
        >
          <template #title>
            <span v-if="!railCollapsed" class="text-body-2 text-medium-emphasis">Einklappen</span>
          </template>
        </v-list-item>
      </v-list>
    </template>
  </v-navigation-drawer>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useDisplay } from 'vuetify'
import { useAsdStore } from '@/stores/asd.js'

const props = defineProps({
  modelValue: { type: Boolean, default: true },
})
const emit = defineEmits(['update:modelValue', 'layer-toggle', 'fl-filter-change'])

const { mdAndUp } = useDisplay()
const store = useAsdStore()

// ASD-008 nav sections — extend here for ASD-013 (alarms) and future panels
const sections = [
  { id: 'layers', icon: 'mdi-layers-outline', label: 'Layer & Filter' },
]

const activeSection = ref('layers')
const railCollapsed = ref(false)

const drawerOpen = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

function toggleSection(id) {
  if (railCollapsed.value) {
    // First click on a collapsed rail: expand and show that section
    railCollapsed.value = false
    activeSection.value = id
  } else if (activeSection.value === id) {
    // Clicking the already-active section while expanded: collapse rail
    railCollapsed.value = true
  } else {
    // Switch to a different section while already expanded
    activeSection.value = id
  }
}

// FL filter local refs synced to store
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
