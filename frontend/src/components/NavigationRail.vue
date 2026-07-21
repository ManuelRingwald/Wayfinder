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

        <!-- Rail: icon strip, always visible. Width is token-driven
             (--wf-nav-rail-width): 56px on desktop, 76px on the iPad/
             tablet-landscape band (#194 Häppchen 2), where the `--touch` class
             also enlarges the pill + icon to comfortable finger targets. -->
        <div class="nav-rail" :class="{ 'nav-rail--touch': tabletLandscape }">
          <!-- Brand glyph pinned to the top of the rail (design template): a
               30×30 rounded tile with the cyan radar mark on a state-selected
               fill. Static for now; earmarked to become the ASD⇄EFS switch. -->
          <div class="nav-rail__brand" role="img" aria-label="ASD">
            <div class="nav-rail__brand-box">
              <v-icon size="20" color="primary">mdi-radar</v-icon>
            </div>
          </div>
          <div class="nav-rail__divider" role="separator" />

          <!-- ASD-019 (ADR 0030): the rail is GROUPED into a MEASURE and a MAP
               section, each under a subdued micro-label + divider so the two
               function families read at a glance (design mockup "Vorschlag A").
               Tools (RBL/DIST/QDM) drive the map's measure controller via the
               tools store; they carry the `--tool` group class → they light AMBER
               when armed (a measuring mode grabs map clicks — a modal state).
               PROBE is intentionally omitted (undefined content — no fake UI). -->
          <div class="nav-rail__section">MEASURE</div>
          <div
            v-for="t in measureTools"
            :key="t.id"
            class="nav-rail__btn nav-rail__btn--tool"
            :class="{ 'nav-rail__btn--active': tools.activeTool === t.id }"
            role="button"
            :aria-label="t.label"
            :aria-pressed="tools.activeTool === t.id"
            @click="tools.selectTool(t.id)"
          >
            <div class="nav-rail__pill">
              <v-icon :size="railIconSize">{{ t.icon }}</v-icon>
            </div>
            <span class="nav-rail__label">{{ t.label }}</span>
            <!-- #296: hover/focus tooltip explaining the tool before selection. -->
            <v-tooltip activator="parent" location="right" open-delay="300" :text="t.description" />
          </div>

          <div class="nav-rail__divider" role="separator" />

          <!-- MAP section: Layer/Filter open a side panel (non-modal). They carry
               the `--panel` group class → they keep the CYAN indicator (the MD3
               primary pill), distinct from the amber armed-tool state above. -->
          <div class="nav-rail__section">MAP</div>
          <div
            v-for="s in sections"
            :key="s.id"
            class="nav-rail__btn nav-rail__btn--panel"
            :class="{ 'nav-rail__btn--active': activePanel === s.id, 'nav-rail__btn--engaged': sectionEngaged(s.id) }"
            role="button"
            :aria-label="s.label"
            :aria-pressed="activePanel === s.id"
            @click="togglePanel(s.id)"
          >
            <div class="nav-rail__pill">
              <v-icon :size="railIconSize">{{ s.icon }}</v-icon>
            </div>
            <span class="nav-rail__label">{{ s.label }}</span>
          </div>

          <!-- ASD-019: zoom left the rail for the bottom-right of the scope
               (MapControls, a new overlay zone per ADR 0029/0030). The push
               divider's auto top-margin drops the account cluster to the foot of
               the rail, cleanly separated from the two function groups above. -->
          <div class="nav-rail__divider nav-rail__divider--push" role="separator" />

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
              <v-icon :size="railIconSize">mdi-shield-account</v-icon>
            </div>
            <span class="nav-rail__label">Admin</span>
          </div>

          <!-- #116: Nutzer-Account, pinned to the very bottom. The push divider
               above (nav-rail__divider--push, margin-top:auto) drops this cluster
               to the foot of the rail — no per-item auto margin needed. -->
          <div
            class="nav-rail__btn nav-rail__btn--account"
            :class="{ 'nav-rail__btn--active': activePanel === 'account' }"
            role="button"
            aria-label="Konto"
            :aria-pressed="activePanel === 'account'"
            @click="togglePanel('account')"
          >
            <div class="nav-rail__pill">
              <v-icon :size="railIconSize">mdi-account</v-icon>
            </div>
            <span class="nav-rail__label">Konto</span>
          </div>
        </div>

        <!-- Divider + panel (appear only when a section is active) -->
        <Transition name="nav-panel">
          <div v-if="activePanel" class="nav-panel" key="panel">
            <div class="nav-panel__divider" role="separator" />
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
      <!-- ASD-019: measurement tools in the mobile drawer. Zoom left the rail for
           the bottom-right of the scope (MapControls). Armed tool → amber
           (warning), matching the desktop MEASURE group's colour code. -->
      <div class="nav-mobile-tools">
        <v-btn
          v-for="t in measureTools"
          :key="t.id"
          icon
          size="small"
          variant="text"
          :color="tools.activeTool === t.id ? 'warning' : undefined"
          :aria-label="t.label"
          @click="tools.selectTool(t.id)"
        >
          <v-icon>{{ t.icon }}</v-icon>
          <!-- #296: same tool description as the desktop rail, reachable on touch. -->
          <v-tooltip activator="parent" location="bottom" open-delay="300" :text="t.description" />
        </v-btn>
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
import { useAsdStore } from '@/stores/asd.js'
import LayerFilterContent from './LayerFilterContent.vue'

const props = defineProps({
  modelValue: { type: Boolean, default: true },
})
const emit = defineEmits([
  'update:modelValue', 'layer-toggle', 'fl-filter-change', 'panel-resize',
])

// #194 Häppchen 2: `md` is the iPad / tablet-landscape band (960–1279px) — the
// rail is still shown (mdAndUp) but the display is a touch tablet, so it gets a
// wider, touch-sized rail (76px, 44px targets, 24px icons) and a wider 304px
// panel; `lg`+ keeps the compact desktop rail (56px/248px). The CSS side widens
// the rail purely via the --wf-nav-rail-width token (base.css); this JS side
// only has to feed the matching drawer width + icon size to Vuetify.
const { mdAndUp, md } = useDisplay()
const tabletLandscape = md
const railIconSize = computed(() => (tabletLandscape.value ? 24 : 20))

// Häppchen 3: the measurement tools live in the rail now. activeTool is global
// (tools store) and drives the map's measure controller via MapCanvas, so the
// rail only has to toggle it — no map reference needed here.
const tools = useToolsStore()
// #296: each tool carries a one-line `description` shown as a hover/focus tooltip
// on its rail button, so the ATC abbreviation (RBL/DIST/QDM) is understandable
// BEFORE it is selected. This complements the active-state `hint` (tools store),
// which is the operating instruction shown only WHILE a tool is engaged.
const measureTools = [
  { id: 'rbl', icon: 'mdi-vector-line', label: 'RBL', description: 'Range/Bearing Line: Entfernung + Peilung frei auf der Karte ziehen.' },
  { id: 'dist', icon: 'mdi-ruler', label: 'DIST', description: 'Abstand + Peilung zwischen zwei Tracks.' },
  { id: 'qdm', icon: 'mdi-compass-outline', label: 'QDM', description: 'Peilung von einem Track zu einem beliebigen Punkt (rechtweisend).' },
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
  { id: 'filters', icon: 'mdi-tune-variant', label: 'Filter' },
]

// #318: light a MAP-section rail icon (cyan, mirroring the amber armed-tool glow
// of the MEASURE family) when its section has at least one ACTIVE element — so
// the operator sees that layers/filters are engaged without opening the panel.
// Layer = any layer visible or any airspace category on; Filter = an FL bound
// set or the out-of-band "Ausblenden" switch on. Derived live from the ASD store.
const asd = useAsdStore()
const hasActiveLayers = computed(() =>
  Object.values(asd.layerVisibility).some(Boolean)
  || Object.values(asd.airspaceGroupVisibility).some(Boolean),
)
const hasActiveFilter = computed(() =>
  asd.flFilter.minFL != null || asd.flFilter.maxFL != null || asd.flFilter.hide === true,
)
function sectionEngaged(id) {
  if (id === 'layers') return hasActiveLayers.value
  if (id === 'filters') return hasActiveFilter.value
  return false
}

// #115: the panel starts COLLAPSED — only the rail (sidecar) is visible, so the
// map gets the full width until the operator opens a section.
const activePanel = ref(null)

const drawerOpen = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const drawerWidth = computed(() => {
  if (!mdAndUp.value) return 280
  // iPad/tablet-landscape band: touch-sized 76px rail, 304px open panel (design
  // mockup). Desktop keeps the compact 56px rail; the open panel is 288px (#316:
  // widened from 248 so the layer labels — e.g. the Minimal/Standard/Detailliert
  // map presets — are no longer clipped). These match the .nav-panel CSS widths.
  const rail = tabletLandscape.value ? 76 : 56
  const panel = tabletLandscape.value ? 304 : 288
  return activePanel.value ? panel : rail
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

/* Rail: token-driven width — 56px desktop, 76px on the iPad/tablet-landscape
   band (base.css widens --wf-nav-rail-width there). The width comes from CSS so
   the strip is correct even before Vuetify hydrates the matching drawer width. */
.nav-rail {
  width: var(--wf-nav-rail-width, 56px);
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 12px;
  gap: 4px;
}

/* #194 Häppchen 2 — touch-optimised rail (iPad/tablet-landscape): the indicator
   pill and its tap area grow to a comfortable finger target (~44px), matching
   the 24px icons bound in the template. The whole .nav-rail__btn is clickable,
   so with the taller padding each item clears 44×44 easily. */
.nav-rail--touch .nav-rail__btn {
  padding: 10px 0;
}
.nav-rail--touch .nav-rail__pill {
  width: 44px;
  height: 36px;
  border-radius: 18px;
}
.nav-rail--touch .nav-rail__brand-box {
  width: 36px;
  height: 36px;
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

/* ASD-019: the push divider drops the account cluster to the rail's foot. Its
   auto top-margin absorbs the free space in the flex column, so everything below
   it (Admin/Konto) sinks to the bottom while the divider itself marks the split. */
.nav-rail__divider--push {
  margin-top: auto;
}

/* ASD-019: MEASURE / MAP micro-label above each function group (design mockup
   "Vorschlag A"). Tiny, uppercase, wide-tracked and subdued (the overline
   treatment), sized to fit the 56px rail so the two families read at a glance. */
.nav-rail__section {
  align-self: stretch;
  text-align: center;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--wf-overline-color);
  margin: 2px 0 1px;
  user-select: none;
}

/* Req 1 + #116 + ASD-019: Admin (when present) and the account entry sit at the
   bottom of the rail. The drop to the foot is done ONCE by the push divider's
   auto top-margin (.nav-rail__divider--push) above this cluster — so Admin/Konto
   stay a tight group, cleanly split from the MEASURE/MAP sections (mockup:
   "Konto ... klar vom Rest getrennt"), without a per-item auto margin. */
.nav-rail__btn--account {
  margin-bottom: 12px;
}

/* MD3 Navigation Rail item: icon + label, centred in the 56px column.
   position:relative anchors the left group-accent bar (::before). */
.nav-rail__btn {
  position: relative;
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
/* ASD-019 (ADR 0030): TWO active-state colours. MAP panels keep the cyan primary
   (the default active colour); MEASURE tools override to warning-amber, so an
   armed measuring mode (which grabs map clicks — modal) is unmistakable next to a
   merely-open panel. */
.nav-rail__btn--active { color: rgb(var(--v-theme-primary)); }
.nav-rail__btn--tool.nav-rail__btn--active { color: var(--wf-warning); }

/* ASD-019: persistent group colour-code — a subtle left accent bar tints each
   family even at rest (amber = MEASURE, cyan = MAP), so the orange/blue coding
   reads at a glance. It brightens and glows when the item is active. */
.nav-rail__btn--tool::before,
.nav-rail__btn--panel::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 16px;
  border-radius: 0 3px 3px 0;
  opacity: 0.35;
  transition: opacity 0.15s, box-shadow 0.15s;
}
.nav-rail__btn--tool::before { background: var(--wf-warning); }
.nav-rail__btn--panel::before { background: rgb(var(--v-theme-primary)); }
.nav-rail__btn--tool.nav-rail__btn--active::before {
  opacity: 1;
  box-shadow: var(--wf-glow-armed);
}
.nav-rail__btn--panel.nav-rail__btn--active::before {
  opacity: 1;
  box-shadow: var(--wf-glow-selected);
}

/* Pill highlight behind the icon (MD3 indicator) */
.nav-rail__pill {
  width: 36px;
  height: 28px;
  border-radius: var(--wf-radius-nav-pill); /* 14px MD3 indicator pill */
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s, box-shadow 0.15s;
}
.nav-rail__btn:hover .nav-rail__pill {
  background: var(--wf-state-hover);
}
/* Active pill: tinted fill + a soft halo so the symbol "leuchtet". Cyan is the
   default (MAP panels + account); MEASURE tools override to amber. */
.nav-rail__btn--active .nav-rail__pill {
  background: var(--wf-state-selected); /* primary @ 16% — MD3 indicator */
  box-shadow: var(--wf-glow-selected);
}
.nav-rail__btn--tool.nav-rail__btn--active .nav-rail__pill {
  background: var(--wf-state-armed); /* warning @ 16% — armed-tool indicator */
  box-shadow: var(--wf-glow-armed);
}
/* The icon itself glows in the accent colour when active (drop-shadow works for
   both the font and SVG mdi builds). */
.nav-rail__btn--active .nav-rail__pill :deep(.v-icon) {
  filter: drop-shadow(0 0 5px rgba(35, 211, 230, 0.6));
}
.nav-rail__btn--tool.nav-rail__btn--active .nav-rail__pill :deep(.v-icon) {
  filter: drop-shadow(0 0 5px rgba(255, 176, 46, 0.65));
}

/* #318: an ENGAGED MAP button — its section has active layers/filters — lights
   the same cyan halo as an OPEN panel (pill fill + soft glow + icon drop-shadow
   + lit accent bar), independent of whether the panel is open. Mirrors the amber
   armed-tool glow of the MEASURE family, in the MAP family's cyan, so "layers/
   filters are on" reads at a glance. The label stays un-tinted, so an OPEN panel
   (full primary colour) still reads distinctly from a merely-engaged one. */
.nav-rail__btn--engaged .nav-rail__pill {
  background: var(--wf-state-selected);
  box-shadow: var(--wf-glow-selected);
}
.nav-rail__btn--engaged .nav-rail__pill :deep(.v-icon) {
  filter: drop-shadow(0 0 5px rgba(35, 211, 230, 0.6));
}
.nav-rail__btn--engaged::before {
  opacity: 1;
  box-shadow: var(--wf-glow-selected);
}

/* Label below icon (Design System v1 token: nav-rail item label) */
.nav-rail__label {
  font-size: var(--wf-nav-label-size);
  font-weight: var(--wf-nav-label-weight);
  line-height: 1;
  letter-spacing: var(--wf-nav-label-tracking);
}

/* Panel beside the rail. FIXED width (not flex:1) so its content is laid out at
   its final width the instant it mounts and does NOT reflow while the drawer
   animates its width open/closed (bug: "Schrift baut sich auf / wird beim
   Einklappen zusammengedrückt", operator 2026-07-08). The drawer is narrower
   than rail+panel during the slide, but .nav-two-col (overflow:hidden) clips the
   overhang, so the panel is revealed as a clean left-to-right wipe instead of a
   re-layout. A stable width also removes the transient vertical scrollbar that
   flashed as the text briefly wrapped taller in the momentarily-narrow panel.
   Width = open drawer width (drawerWidth in JS: 288 desktop, #316) minus the rail. */
.nav-panel {
  width: calc(288px - var(--wf-nav-rail-width, 56px));
  flex-shrink: 0;
  display: flex;
  flex-direction: row;
  overflow: hidden;
  min-width: 0;
}
/* iPad/tablet-landscape band: the open drawer is 304px (JS) and the rail 76px. */
@media (min-width: 960px) and (max-width: 1279.98px) {
  .nav-panel {
    width: calc(304px - var(--wf-nav-rail-width, 76px));
  }
}
/* #176 + operator request 2026-07-08: a reliably-rendered, subtle hairline
   between the icon rail and the open content panel. The previous vertical
   v-divider read as almost no separation (it did not stretch full-height in this
   flex row); a plain full-height 1px strip using the slightly-stronger border
   token keeps the line dezent but clearly visible. */
.nav-panel__divider {
  width: 1px;
  align-self: stretch;
  flex-shrink: 0;
  background: var(--wf-border-strong);
}
.nav-panel__body {
  flex: 1;
  overflow-y: auto;
  /* Never a horizontal scrollbar — the content width is fixed to the panel, so
     any transient overhang during the open/close slide is simply clipped. */
  overflow-x: hidden;
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
