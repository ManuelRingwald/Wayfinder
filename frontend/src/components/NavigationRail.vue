<template>
  <!-- ASD-012 / ASD-013: Navigation Rail (permanent 56px) + secondary panel.
       Desktop: rail shows icon + label, clicking opens a 244px panel beside it.
       No drawer border; a thin divider appears only between rail and open panel.
       Mobile: temporary overlay drawer with direct panel content. -->
  <v-navigation-drawer
    v-model="drawerOpen"
    :permanent="mdAndUp"
    :temporary="!mdAndUp"
    :width="drawerWidth"
    color="surface"
    :border="0"
    class="nav-drawer"
  >
    <!-- ── Desktop layout ── -->
    <template v-if="mdAndUp">
      <div class="nav-two-col">

        <!-- Rail: 56px icon strip, always visible -->
        <div class="nav-rail">
          <div
            v-for="s in sections"
            :key="s.id"
            class="nav-rail__btn"
            :class="{ 'nav-rail__btn--active': activePanel === s.id }"
            role="button"
            :aria-label="s.label"
            :aria-pressed="activePanel === s.id"
            @click="togglePanel(s.id)"
          >
            <div class="nav-rail__pill">
              <v-icon size="20">{{ s.icon }}</v-icon>
            </div>
            <span class="nav-rail__label">{{ s.label }}</span>
          </div>
        </div>

        <!-- Divider + panel (appear only when a section is active) -->
        <Transition name="nav-panel">
          <div v-if="activePanel" class="nav-panel" key="panel">
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

const sections = [
  { id: 'layers', icon: 'mdi-filter-outline', label: 'Filter' },
]

const activePanel = ref('layers')

const drawerOpen = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

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

/* Rail: fixed 56px strip */
.nav-rail {
  width: 56px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 12px;
  gap: 4px;
}

/* MD3 Navigation Rail item: icon + label, centred in the 56px column */
.nav-rail__btn {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 6px 0;
  cursor: pointer;
  user-select: none;
  color: rgba(var(--v-theme-on-surface), 0.6);
  transition: color 0.15s;
}
.nav-rail__btn:hover { color: rgba(var(--v-theme-on-surface), 0.9); }
.nav-rail__btn--active { color: rgb(var(--v-theme-primary)); }

/* Pill highlight behind the icon (MD3 indicator) */
.nav-rail__pill {
  width: 36px;
  height: 28px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s;
}
.nav-rail__btn:hover .nav-rail__pill {
  background: rgba(var(--v-theme-on-surface), 0.08);
}
.nav-rail__btn--active .nav-rail__pill {
  background: rgba(var(--v-theme-primary), 0.16);
}

/* Label below icon */
.nav-rail__label {
  font-size: 11px;
  font-weight: 500;
  line-height: 1;
  letter-spacing: 0.02em;
}

/* Panel: fills remaining width beside rail */
.nav-panel {
  display: flex;
  flex-direction: row;
  flex: 1;
  overflow: hidden;
  min-width: 0;
}
.nav-panel__body {
  flex: 1;
  overflow-y: auto;
}

/* Slide-in transition */
.nav-panel-enter-active,
.nav-panel-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.nav-panel-enter-from,
.nav-panel-leave-to {
  opacity: 0;
  transform: translateX(-6px);
}
</style>
