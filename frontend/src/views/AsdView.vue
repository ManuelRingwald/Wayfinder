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
    <v-btn
      v-if="!mdAndUp"
      icon="mdi-menu"
      size="small"
      elevation="4"
      class="mobile-menu-btn"
      @click="drawerOpen = !drawerOpen"
    />

    <NavigationRail
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
      <!-- Reskin 3b: floating scope legend (bottom-left) + bottom-right readout
           "<width> NM Breite · Vektor N min" (replaces the native scale bar). -->
      <div class="scope-legend-overlay">
        <ScopeLegend />
      </div>
      <div class="vector-readout-overlay wf-mono">{{ store.viewportWidthNM }} NM Breite · Vektor {{ vectorMinutes }} min</div>
    </v-main>

    <TrackDetailPanel
      v-if="store.selectedTrack"
      @close="store.clearTrackSelection()"
    />
  </template>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useAsdStore } from '@/stores/asd.js'
import { useSessionStore } from '@/stores/session.js'
import { useToolsStore } from '@/stores/tools.js'
import NavigationRail from '@/components/NavigationRail.vue'
import MapCanvas from '@/components/MapCanvas.vue'
import AsdHeader from '@/components/AsdHeader.vue'
import ScopeLegend from '@/components/ScopeLegend.vue'
import TrackDetailPanel from '@/components/TrackDetailPanel.vue'
import FeedStatusChip from '@/components/FeedStatusChip.vue'
import LoginCard from '@/components/LoginCard.vue'
import { VECTOR_LOOKAHEAD_S } from '@/map/constants.js'

const { mdAndUp } = useDisplay()
const store = useAsdStore()
const session = useSessionStore()
const tools = useToolsStore()
const drawerOpen = ref(true)
const mapCanvas = ref(null)
const loginLoading = ref(false)

// Reskin 3b: speed-vector look-ahead in minutes, shown in the bottom-right
// readout. Fixed today (VECTOR_LOOKAHEAD_S); becomes operator-tunable in the
// tweaks panel (Häppchen 5), at which point this reads the live setting.
const vectorMinutes = Math.round(VECTOR_LOOKAHEAD_S / 60)

// Make an expiry visible: a dropped session shows "session expired" on the login
// screen instead of a bare prompt (WF2-12.5).
const loginNotice = computed(() =>
  session.expired ? 'Sitzung abgelaufen — bitte erneut anmelden.' : session.error,
)

// Resolve the session on entry, and slide it forward when the tab regains focus.
onMounted(() => {
  session.probe()
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
.mobile-menu-btn {
  position: fixed;
  top: 8px;
  left: 8px;
  z-index: 1100;
  background: rgba(var(--v-theme-surface), 0.9) !important;
}

/* Top-right cluster: ICAO/sector + UTC header next to the feed-status badge,
   right-aligned so the badge stays at the corner and the header extends left. */
.top-right-cluster {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 600;
  display: flex;
  /* Design template: header (ICAO/UTC) and the feed badge stack vertically,
     right-aligned, so the feed badge sits on its own line under the clock. */
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  pointer-events: none;
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
  /* Clear the 56px navigation rail (left edge) + a 12px gap, so the legend is
     not painted over by the opaque fixed drawer. v-main spans full width
     (padding:0), so this offset is measured from the viewport left. */
  left: 68px;
  z-index: 600;
  pointer-events: none;
}

/* Bottom-right distance/vector readout: "<width> NM Breite · Vektor N min"
   (design). Replaces the native scale bar, which was removed in the engine. */
.vector-readout-overlay {
  position: absolute;
  bottom: 12px;
  right: 12px;
  z-index: 600;
  pointer-events: none;
  font-size: 10.5px;
  color: var(--wf-on-surface-variant);
  background: rgba(14, 22, 34, 0.85);
  backdrop-filter: blur(4px);
  border: var(--wf-chrome-border);
  border-radius: var(--wf-radius-sm);
  padding: 3px 8px;
}
</style>
