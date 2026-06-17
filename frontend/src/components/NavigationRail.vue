<template>
  <!-- ASD-012: Two-level navigation.
       Desktop: a permanent 56 px icon rail (always visible) plus an optional
       244 px panel that slides in beside it — the drawer width grows from 56
       to 300 px. The map canvas adjusts automatically because Vuetify's layout
       tracks the drawer width.
       Mobile: a standard temporary overlay drawer (hamburger-triggered) that
       shows the panel content directly, no rail column needed. -->
  <v-navigation-drawer
    v-model="drawerOpen"
    :permanent="mdAndUp"
    :temporary="!mdAndUp"
    :width="drawerWidth"
    color="surface"
    class="nav-drawer"
  >
    <!-- ── Desktop layout ── -->
    <template v-if="mdAndUp">
      <div class="nav-two-col">

        <!-- Rail: 56 px icon strip, always visible -->
        <div class="nav-rail">
          <v-list density="compact" class="pa-1 pt-2" nav>
            <v-list-item
              v-for="s in sections"
              :key="s.id"
              :active="activePanel === s.id"
              active-color="primary"
              rounded="lg"
              class="nav-rail__item mb-1"
              @click="togglePanel(s.id)"
            >
              <template #prepend>
                <v-icon size="20">{{ s.icon }}</v-icon>
              </template>
              <v-tooltip activator="parent" location="right" :text="s.label" />
            </v-list-item>
          </v-list>
        </div>

        <!-- Panel: slides in to the right of the rail -->
        <Transition name="nav-panel">
          <div v-if="activePanel" class="nav-panel">
            <v-divider vertical />
            <div class="nav-panel__body">
              <LayerFilterContent
                @layer-toggle="onLayerToggle"
                @fl-filter-change="onFlFilterChange"
              />
            </div>
          </div>
        </Transition>

      </div>
    </template>

    <!-- ── Mobile layout ── -->
    <template v-else>
      <LayerFilterContent
        @layer-toggle="onLayerToggle"
        @fl-filter-change="onFlFilterChange"
      />
    </template>
  </v-navigation-drawer>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useDisplay } from 'vuetify'
import LayerFilterContent from './LayerFilterContent.vue'

const props = defineProps({
  modelValue: { type: Boolean, default: true },
})
const emit = defineEmits(['update:modelValue', 'layer-toggle', 'fl-filter-change'])

const { mdAndUp } = useDisplay()

// ASD-012 nav sections — extend here for future panels (alarms, scenarios …)
const sections = [
  { id: 'layers', icon: 'mdi-layers-outline', label: 'Layer & Filter' },
]

const activePanel = ref('layers')  // open by default

const drawerOpen = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

// Width: 56 px (rail only) when no panel active, 300 px (rail + panel) when open.
// On mobile the drawer uses a fixed 280 px (handled by Vuetify temporary mode).
const drawerWidth = computed(() => {
  if (!mdAndUp.value) return 280
  return activePanel.value ? 300 : 56
})

function togglePanel(id) {
  activePanel.value = activePanel.value === id ? null : id
}

function onLayerToggle(payload) { emit('layer-toggle', payload) }
function onFlFilterChange(payload) { emit('fl-filter-change', payload) }
</script>

<style scoped>
/* Two-column desktop layout */
.nav-two-col {
  display: flex;
  flex-direction: row;
  height: 100%;
  overflow: hidden;
}

/* Rail: icon strip, fixed 56 px */
.nav-rail {
  width: 56px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

/* Rail items: centre icon, hide any default title padding */
.nav-rail__item {
  min-width: 0;
  padding-inline: 8px !important;
}
.nav-rail__item :deep(.v-list-item__spacer) { display: none; }

/* Panel: fills the remaining width beside the rail */
.nav-panel {
  display: flex;
  flex-direction: row;
  flex: 1;
  overflow: hidden;
}
.nav-panel__body {
  flex: 1;
  overflow-y: auto;
}

/* Slide-in transition for the panel */
.nav-panel-enter-active,
.nav-panel-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.nav-panel-enter-from,
.nav-panel-leave-to {
  opacity: 0;
  transform: translateX(-8px);
}

/* Drawer width transition — animates the 56↔300 px change */
.nav-drawer :deep(.v-navigation-drawer__content) {
  overflow: hidden;
}
</style>
