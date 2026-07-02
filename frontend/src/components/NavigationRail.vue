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
          <!-- Brand glyph pinned to the top of the rail (design template): a
               30×30 rounded tile with the cyan radar mark on a state-selected
               fill. Static for now; earmarked to become the ASD⇄EFS switch. -->
          <div class="nav-rail__brand" role="img" aria-label="ASD">
            <div class="nav-rail__brand-box">
              <v-icon size="20" color="primary">mdi-radar</v-icon>
            </div>
          </div>
          <div class="nav-rail__divider" role="separator" />

          <!-- Häppchen 3: measurement tools (RBL/DIST/QDM), moved from the
               floating map toolbar into the rail (design mockup). Toggle buttons
               wired to the tools store, which drives the map's measure controller
               (map/measure.js via MapCanvas). PROBE is intentionally omitted
               (undefined content — no fake UI). -->
          <div
            v-for="t in measureTools"
            :key="t.id"
            class="nav-rail__btn"
            :class="{ 'nav-rail__btn--active': tools.activeTool === t.id }"
            role="button"
            :aria-label="t.label"
            :aria-pressed="tools.activeTool === t.id"
            @click="tools.selectTool(t.id)"
          >
            <div class="nav-rail__pill">
              <v-icon size="20">{{ t.icon }}</v-icon>
            </div>
            <span class="nav-rail__label">{{ t.label }}</span>
          </div>

          <div class="nav-rail__divider" role="separator" />

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

          <div class="nav-rail__divider" role="separator" />

          <!-- Häppchen 3: zoom controls in the rail (design mockup). Purely
               presentational — they emit and AsdView delegates to the map engine
               (MapCanvas.zoomIn/zoomOut), keeping the engine framework-agnostic. -->
          <div class="nav-rail__btn" role="button" aria-label="Zoom in" @click="emit('zoom-in')">
            <div class="nav-rail__pill"><v-icon size="20">mdi-plus</v-icon></div>
            <span class="nav-rail__label">Zoom +</span>
          </div>
          <div class="nav-rail__btn" role="button" aria-label="Zoom out" @click="emit('zoom-out')">
            <div class="nav-rail__pill"><v-icon size="20">mdi-minus</v-icon></div>
            <span class="nav-rail__label">Zoom −</span>
          </div>

          <!-- Req 1: Admin entry, visible only to admins; the account section
               (#116) sits below it at the very bottom of the rail. -->
          <div
            v-if="isAdmin"
            class="nav-rail__btn nav-rail__btn--admin"
            role="button"
            aria-label="Admin"
            @click="goAdmin"
          >
            <div class="nav-rail__pill">
              <v-icon size="20">mdi-shield-account</v-icon>
            </div>
            <span class="nav-rail__label">Admin</span>
          </div>

          <!-- #116: Nutzer-Account, pinned to the very bottom -->
          <div
            class="nav-rail__btn nav-rail__btn--account"
            :class="{ 'nav-rail__btn--bottom': !isAdmin, 'nav-rail__btn--active': activePanel === 'account' }"
            role="button"
            aria-label="Konto"
            :aria-pressed="activePanel === 'account'"
            @click="togglePanel('account')"
          >
            <div class="nav-rail__pill">
              <v-icon size="20">mdi-account</v-icon>
            </div>
            <span class="nav-rail__label">Konto</span>
          </div>
        </div>

        <!-- Divider + panel (appear only when a section is active) -->
        <Transition name="nav-panel">
          <div v-if="activePanel" class="nav-panel" key="panel">
            <v-divider vertical />
            <div class="nav-panel__body">
              <LayerFilterContent
                :section="activePanel"
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
      <!-- Häppchen 3: measurement tools + zoom, also reachable from the mobile
           drawer (the desktop rail hosts them as labelled icons). -->
      <div class="nav-mobile-tools">
        <v-btn
          v-for="t in measureTools"
          :key="t.id"
          :icon="t.icon"
          size="small"
          variant="text"
          :color="tools.activeTool === t.id ? 'primary' : undefined"
          :aria-label="t.label"
          @click="tools.selectTool(t.id)"
        />
        <v-divider vertical class="mx-1" />
        <v-btn icon="mdi-plus" size="small" variant="text" aria-label="Zoom in" @click="emit('zoom-in')" />
        <v-btn icon="mdi-minus" size="small" variant="text" aria-label="Zoom out" @click="emit('zoom-out')" />
      </div>
      <v-list-item
        v-if="isAdmin"
        prepend-icon="mdi-shield-account"
        title="Admin"
        @click="goAdmin"
      />
      <LayerFilterContent
        section="all"
        @layer-toggle="onLayerToggle"
        @fl-filter-change="onFlFilterChange"
      />
    </template>
  </v-navigation-drawer>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useDisplay } from 'vuetify'
import { useRouter } from 'vue-router'
import { useAdminStore } from '@/stores/admin.js'
import { useToolsStore } from '@/stores/tools.js'
import LayerFilterContent from './LayerFilterContent.vue'

const props = defineProps({
  modelValue: { type: Boolean, default: true },
})
const emit = defineEmits([
  'update:modelValue', 'layer-toggle', 'fl-filter-change', 'panel-resize', 'zoom-in', 'zoom-out',
])

const { mdAndUp } = useDisplay()

// Häppchen 3: the measurement tools live in the rail now. activeTool is global
// (tools store) and drives the map's measure controller via MapCanvas, so the
// rail only has to toggle it — no map reference needed here.
const tools = useToolsStore()
const measureTools = [
  { id: 'rbl', icon: 'mdi-vector-line', label: 'RBL' },
  { id: 'dist', icon: 'mdi-ruler', label: 'DIST' },
  { id: 'qdm', icon: 'mdi-compass-outline', label: 'QDM' },
]

// Req 1: an Admin entry appears in the rail only for the admin role (ADR 0009).
// We probe the identity once on mount; fail-closed — a user (or a single-tenant
// deployment) gets 401/403/404 and isAdmin stays false. The real guard is
// server-side (RequireRole(admin) on /api/admin/*); this is convenience only.
const router = useRouter()
const adminStore = useAdminStore()
const isAdmin = computed(() => adminStore.isAdmin)

onMounted(() => {
  if (!adminStore.isAuthorized) adminStore.loadIdentity()
})

function goAdmin() { router.push('/admin') }

// #116: three sections — Layer (toggles + legend), Filter (FL band) and, at the
// bottom of the rail, the Nutzer-Account (logout). Each opens its own panel.
const sections = [
  { id: 'layers', icon: 'mdi-layers-outline', label: 'Layer' },
  { id: 'filters', icon: 'mdi-filter-outline', label: 'Filter' },
]

// #115: the panel starts COLLAPSED — only the rail (sidecar) is visible, so the
// map gets the full width until the operator opens a section.
const activePanel = ref(null)

const drawerOpen = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const drawerWidth = computed(() => {
  if (!mdAndUp.value) return 280
  // Rail 56px collapsed; open panel 248px (design template AsdFilterPanel width).
  return activePanel.value ? 248 : 56
})

function togglePanel(id) {
  activePanel.value = activePanel.value === id ? null : id
  // #121: the drawer width changes (56 ↔ 300 px) — tell the map to resize once
  // the CSS transition settles, or it leaves a dead strip where the panel was.
  emit('panel-resize')
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

/* Häppchen 3: compact tool + zoom row at the top of the mobile drawer */
.nav-mobile-tools {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 8px 12px;
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

/* Brand glyph at the top of the rail (future ASD⇄EFS switch) */
.nav-rail__brand {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  padding: 2px 0 4px;
}
/* 30×30 rounded tile with the cyan mark on a state-selected fill (template) */
.nav-rail__brand-box {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--wf-state-selected);
  flex-shrink: 0;
}

/* Thin horizontal separator between rail groups (design template RailSep:
   stretched hairline, 7px vertical / 10px horizontal margin) */
.nav-rail__divider {
  align-self: stretch;
  height: 1px;
  background: var(--wf-border);
  margin: 7px 10px;
  flex-shrink: 0;
}

/* Req 1 + #116: Admin (when present) and the account entry sit at the bottom of
   the rail; the auto top margin on the FIRST bottom item pushes the group down
   past the section items. The account entry is always last. */
.nav-rail__btn--admin {
  margin-top: auto;
}
.nav-rail__btn--account {
  margin-bottom: 12px;
}
.nav-rail__btn--bottom {
  margin-top: auto;
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
  border-radius: var(--wf-radius-nav-pill); /* 14px MD3 indicator pill */
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s;
}
.nav-rail__btn:hover .nav-rail__pill {
  background: var(--wf-state-hover);
}
.nav-rail__btn--active .nav-rail__pill {
  background: var(--wf-state-selected); /* primary @ 16% — MD3 indicator */
}

/* Label below icon (Design System v1 token: nav-rail item label) */
.nav-rail__label {
  font-size: var(--wf-nav-label-size);
  font-weight: var(--wf-nav-label-weight);
  line-height: 1;
  letter-spacing: var(--wf-nav-label-tracking);
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
