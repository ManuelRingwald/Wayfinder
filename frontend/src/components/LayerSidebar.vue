<template>
  <v-navigation-drawer
    v-model="drawer"
    :permanent="mdAndUp"
    :temporary="!mdAndUp"
    color="surface"
    width="220"
  >
    <v-list-item
      title="Wayfinder"
      subtitle="ASD"
      prepend-icon="mdi-radar"
      class="py-4"
    />
    <v-divider />

    <!-- Layer toggles -->
    <v-list density="compact" nav>
      <v-list-subheader>Layer</v-list-subheader>
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

    <v-divider />

    <!-- FL-Filter -->
    <v-list density="compact" nav>
      <v-list-subheader>FL-Filter</v-list-subheader>
      <v-list-item>
        <div class="d-flex align-center gap-2 px-1">
          <v-text-field
            v-model.number="minFL"
            type="number"
            label="Min"
            min="0"
            max="999"
            step="10"
            style="width: 72px"
            @update:model-value="onFlFilterChange"
          />
          <span class="text-medium-emphasis">–</span>
          <v-text-field
            v-model.number="maxFL"
            type="number"
            label="Max"
            min="0"
            max="999"
            step="10"
            style="width: 72px"
            @update:model-value="onFlFilterChange"
          />
          <span class="text-caption text-medium-emphasis">FL</span>
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

const drawer = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

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
