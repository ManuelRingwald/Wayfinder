<template>
  <!-- ASD-013: the ASD is a full-screen scope, no title bar. This is the
       operational view at route '/'. It gates on the session (ADR 0014: auth is
       always on): until the identity probe resolves we show a spinner, then either
       the login screen (anon) or the live picture (authed). The map + WebSocket
       only mount once authenticated, so /ws never opens unauthenticated. While the
       picture is up, the session is slid forward (WF2-12.5) so an active console
       is never logged out; a real expiry surfaces as the login screen, not a
       silent frozen map. -->
  <v-main
    v-if="session.status === 'loading'"
    class="d-flex justify-center align-center"
    style="min-height: 100vh"
  >
    <v-progress-circular indeterminate color="primary" size="48" />
  </v-main>

  <LoginCard
    v-else-if="session.status === 'anon'"
    title="Wayfinder — Anmelden"
    :error="loginNotice"
    :loading="loginLoading"
    @submit="onLogin"
  />

  <template v-else>
    <!-- Desktop + tablet-landscape (>=960px): the MD3 navigation rail + panel.
         Phones and tablet-portrait use the bottom tab bar + sheets below (#194). -->
    <NavigationRail
      v-if="mdAndUp"
      v-model="drawerOpen"
      @layer-toggle="onLayerToggle"
      @fl-filter-change="onFlFilterChange"
      @panel-resize="onPanelResize"
      @zoom-in="mapCanvas?.zoomIn()"
      @zoom-out="mapCanvas?.zoomOut()"
    />

    <v-main style="padding: 0; position: relative">
      <MapCanvas
        ref="mapCanvas"
        @track-click="onTrackClick"
        @connection-change="onConnectionChange"
      />
      <!-- ASD-007 (design template 'nacht' scheme): a faint cyan radar-scope
           bloom at the display centre. Sits above the map tiles but below the
           chrome and the interactive map controls; 5% alpha, so it tints the
           picture imperceptibly. -->
      <div class="scope-glow-overlay" aria-hidden="true" />
      <!-- Top-right cluster: the ICAO/sector + live UTC header sits next to the
           feed-health badge (CAT065 heartbeat, bug #54). The account chip that
           used to be here was removed — account access is the sidebar's "Konto"
           only, to avoid duplication. -->
      <div class="top-right-cluster">
        <AsdHeader />
        <FeedStatusChip />
      </div>
      <!-- Reskin 3b: floating scope legend (bottom-left). The bottom-right
           width/leader readout was removed (E2E): it read like a scale bar but
           was only the scope width, and sat confusingly next to the range rings.
           Distance is read from the range rings; the speed leaders themselves
           carry the look-ahead. -->
      <div class="scope-legend-overlay">
        <ScopeLegend />
      </div>
    </v-main>

    <!-- ── Mobile (phone / tablet-portrait): bottom tab bar + sheets (#194) ── -->
    <template v-if="!mdAndUp">
      <BottomNav v-model="mobileTab" :is-admin="isAdmin" @select="onMobileTab" />

      <!-- Filter/Layer as a bottom sheet. Measure tools (RBL/DIST/QDM) live in
           the sheet header on mobile — the rail that normally hosts them is not
           rendered on phones. -->
      <v-bottom-sheet v-model="filterSheet" :scrim="true" @update:model-value="onSheetToggle">
        <v-card class="mobile-sheet" rounded="t-xl">
          <div class="mobile-sheet__grab" />
          <div class="mobile-sheet__hd">
            <span class="mobile-sheet__ttl">Layer &amp; Filter</span>
            <div class="mobile-sheet__tools">
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
              <v-btn icon="mdi-close" size="small" variant="text" aria-label="Schließen" @click="closeSheets" />
            </div>
          </div>
          <div class="mobile-sheet__body">
            <LayerFilterContent
              section="all"
              @layer-toggle="onLayerToggle"
              @fl-filter-change="onFlFilterChange"
            />
          </div>
        </v-card>
      </v-bottom-sheet>

      <!-- Konto as a bottom sheet (account section: subject + logout). -->
      <v-bottom-sheet v-model="kontoSheet" :scrim="true" @update:model-value="onSheetToggle">
        <v-card class="mobile-sheet" rounded="t-xl">
          <div class="mobile-sheet__grab" />
          <div class="mobile-sheet__hd">
            <span class="mobile-sheet__ttl">Konto</span>
            <v-btn icon="mdi-close" size="small" variant="text" aria-label="Schließen" @click="closeSheets" />
          </div>
          <div class="mobile-sheet__body">
            <LayerFilterContent section="account" />
          </div>
        </v-card>
      </v-bottom-sheet>
    </template>

    <TrackDetailPanel
      v-if="store.selectedTrack"
      @close="store.clearTrackSelection()"
    />
  </template>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useRouter } from 'vue-router'
import { useAsdStore } from '@/stores/asd.js'
import { useSessionStore } from '@/stores/session.js'
import { useToolsStore } from '@/stores/tools.js'
import { useAdminStore } from '@/stores/admin.js'
import NavigationRail from '@/components/NavigationRail.vue'
import BottomNav from '@/components/BottomNav.vue'
import MapCanvas from '@/components/MapCanvas.vue'
import AsdHeader from '@/components/AsdHeader.vue'
import ScopeLegend from '@/components/ScopeLegend.vue'
import TrackDetailPanel from '@/components/TrackDetailPanel.vue'
import FeedStatusChip from '@/components/FeedStatusChip.vue'
import LoginCard from '@/components/LoginCard.vue'

const { mdAndUp } = useDisplay()
const router = useRouter()
const store = useAsdStore()
const session = useSessionStore()
const tools = useToolsStore()
const adminStore = useAdminStore()
const drawerOpen = ref(true)
const mapCanvas = ref(null)
const loginLoading = ref(false)

// #194: mobile navigation state (phone / tablet-portrait). The bottom tab bar
// selects between the scope and the Filter/Konto sheets; Admin routes away.
const mobileTab = ref('scope')
const filterSheet = ref(false)
const kontoSheet = ref(false)
const isAdmin = computed(() => adminStore.isAdmin)

// Measure tools relocated into the mobile Filter sheet (the rail that hosts them
// on desktop is not rendered on phones). Mirrors NavigationRail's list.
const measureTools = [
  { id: 'rbl', icon: 'mdi-vector-line', label: 'RBL' },
  { id: 'dist', icon: 'mdi-ruler', label: 'DIST' },
  { id: 'qdm', icon: 'mdi-compass-outline', label: 'QDM' },
]

function closeSheets() {
  filterSheet.value = false
  kontoSheet.value = false
  mobileTab.value = 'scope'
}

// Bottom-nav tab handler: open the matching sheet, route to Admin, or clear.
function onMobileTab(id) {
  if (id === 'filter') { kontoSheet.value = false; filterSheet.value = true }
  else if (id === 'konto') { filterSheet.value = false; kontoSheet.value = true }
  else if (id === 'admin') { router.push('/admin') }
  else { closeSheets() } // scope
}

// When a sheet is dismissed by swipe/scrim, snap the tab back to Scope so the
// bar's active state matches what's on screen.
function onSheetToggle(open) {
  if (!open && !filterSheet.value && !kontoSheet.value) mobileTab.value = 'scope'
}

// Make an expiry visible: a dropped session shows "session expired" on the login
// screen instead of a bare prompt (WF2-12.5).
const loginNotice = computed(() =>
  session.expired ? 'Sitzung abgelaufen — bitte erneut anmelden.' : session.error,
)

// Resolve the session on entry, and slide it forward when the tab regains focus.
onMounted(() => {
  session.probe()
  // #194: the bottom tab bar's Admin entry needs the admin probe even on phones,
  // where NavigationRail (which normally triggers it) is not rendered. Fail-closed.
  if (!adminStore.isAuthorized) adminStore.loadIdentity()
  document.addEventListener('visibilitychange', onVisible)
})
onUnmounted(() => {
  document.removeEventListener('visibilitychange', onVisible)
  session.stopRenew()
})

// Run the sliding-refresh loop only while authenticated.
watch(() => session.status, (s) => {
  if (s === 'authed') session.startRenew()
  else session.stopRenew()
})

function onVisible() {
  if (document.visibilityState === 'visible' && session.status === 'authed') {
    session.renewNow()
  }
}

// The map's WebSocket lifecycle drives session freshness (WF2-12.5): on connect,
// slide the session forward; on a drop, probe — if auth was lost the probe flips
// to 'anon' and the login overlay appears; a transient drop stays authenticated
// and the engine reconnects on its own.
async function onConnectionChange(state) {
  if (state === 'open') {
    session.renewNow()
  } else if (state === 'closed' && session.status === 'authed') {
    session.probe()
  }
}

async function onLogin({ subject, password }) {
  loginLoading.value = true
  try {
    await session.login(subject, password)
  } finally {
    loginLoading.value = false
  }
}

function onLayerToggle({ layer, val }) {
  mapCanvas.value?.setLayerVisibility(layer, val)
}

function onFlFilterChange() {
  mapCanvas.value?.updateFlFilter()
}

// #121: after the sidebar panel opens/closes the drawer width animates
// (56 ↔ 300 px); MapLibre must re-measure its container once the transition
// settles or a dead, unpainted strip remains where the panel was.
function onPanelResize() {
  setTimeout(() => mapCanvas.value?.resize(), 250)
}

function onTrackClick(track) {
  // Häppchen 4: while a measurement tool is active, a track click feeds the tool
  // (DIST/QDM pick it via the map controller) — don't also open the detail panel.
  if (tools.activeTool) return
  store.selectTrack(track)
}
</script>

<style scoped>
/* Top-right cluster: ICAO/sector + UTC header next to the feed-status badge,
   right-aligned so the badge stays at the corner and the header extends left.
   #194: padded past the notch/Dynamic-Island + right safe-area inset. */
.top-right-cluster {
  position: absolute;
  top: calc(12px + var(--wf-safe-top, 0px));
  right: calc(12px + var(--wf-safe-right, 0px));
  z-index: 600;
  display: flex;
  /* Design template: header (ICAO/UTC) and the feed badge stack vertically,
     right-aligned, so the feed badge sits on its own line under the clock. */
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  pointer-events: none;
  /* Never let the header run under the opposite chrome on a narrow phone. */
  max-width: calc(100vw - 24px - var(--wf-safe-right, 0px) - var(--wf-safe-left, 0px));
}

/* ASD-007: cyan radar-scope centre bloom (design template 'nacht' glow). Above
   the map tiles, below the chrome (z 600) and the map controls (z 10). */
.scope-glow-overlay {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  background: radial-gradient(circle at 50% 46%, rgba(35, 211, 230, 0.05), transparent 55%);
}

/* Reskin 3b: floating scope legend (bottom-left). pointer-events on the wrapper
   are off; the legend itself re-enables them so its toggle stays clickable. */
.scope-legend-overlay {
  position: absolute;
  bottom: 12px;
  /* Clear the navigation rail (left edge) + a gap, so the legend is not painted
     over by the opaque fixed drawer. Derived from --wf-nav-rail-width so it
     tracks the rail: 68px on desktop (56+12), 88px on the iPad band (76+12,
     #194 Häppchen 2). v-main spans full width (padding:0), so this offset is
     measured from the viewport left. */
  left: calc(var(--wf-nav-rail-width, 56px) + var(--wf-overlay-gap, 12px));
  z-index: 600;
  pointer-events: none;
}

/* #194 — Mobile (< md): no rail, so the legend hugs the left safe-area edge and
   sits above the bottom tab bar instead of the (absent) rail. */
@media (max-width: 959.98px) {
  .scope-legend-overlay {
    left: calc(12px + var(--wf-safe-left, 0px));
    bottom: calc(12px + var(--wf-bottom-nav-h, 64px) + var(--wf-safe-bottom, 0px));
  }
}

/* #194 — Mobile Filter/Konto bottom sheet (design mockup): grab handle, header
   with title + tools, scrollable body. Rounded top; padded past the home
   indicator. Height is capped so the scope stays partly visible behind it. */
.mobile-sheet {
  display: flex;
  flex-direction: column;
  max-height: 82vh;
  background: rgb(var(--v-theme-surface));
  padding-bottom: var(--wf-safe-bottom, 0px);
}
.mobile-sheet__grab {
  width: 38px;
  height: 4px;
  border-radius: 2px;
  background: rgba(var(--v-border-color), 0.32);
  margin: 10px auto 2px;
  flex-shrink: 0;
}
.mobile-sheet__hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 10px 10px 18px;
  border-bottom: var(--wf-chrome-border);
  flex-shrink: 0;
}
.mobile-sheet__ttl {
  font-size: 16px;
  font-weight: 500;
  color: rgb(var(--v-theme-on-surface));
}
.mobile-sheet__tools {
  display: flex;
  align-items: center;
  gap: 2px;
}
.mobile-sheet__body {
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}
</style>
