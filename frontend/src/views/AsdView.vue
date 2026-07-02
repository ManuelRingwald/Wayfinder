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
    />

    <v-main style="padding: 0; position: relative">
      <MapCanvas
        ref="mapCanvas"
        @track-click="onTrackClick"
        @connection-change="onConnectionChange"
      />
      <!-- Account / logout: shows the logged-in principal and a logout action;
           admins also get a shortcut to the administration. -->
      <div class="account-overlay">
        <v-menu location="bottom end">
          <template #activator="{ props }">
            <v-chip
              v-bind="props"
              size="small"
              color="primary"
              variant="tonal"
              append-icon="mdi-account"
              style="cursor: pointer"
            >{{ session.subject }}</v-chip>
          </template>
          <v-list density="compact">
            <v-list-item
              v-if="session.isAdmin"
              :to="{ name: 'admin' }"
              prepend-icon="mdi-cog"
              title="Administration"
            />
            <v-list-item
              prepend-icon="mdi-logout"
              title="Abmelden"
              @click="onLogout"
            />
          </v-list>
        </v-menu>
      </div>
      <!-- Feed health banner (CAT065 heartbeat, bug #54). -->
      <div class="feed-status-overlay">
        <FeedStatusChip />
      </div>
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
import NavigationRail from '@/components/NavigationRail.vue'
import MapCanvas from '@/components/MapCanvas.vue'
import TrackDetailPanel from '@/components/TrackDetailPanel.vue'
import FeedStatusChip from '@/components/FeedStatusChip.vue'
import LoginCard from '@/components/LoginCard.vue'

const { mdAndUp } = useDisplay()
const store = useAsdStore()
const session = useSessionStore()
const drawerOpen = ref(true)
const mapCanvas = ref(null)
const loginLoading = ref(false)

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

async function onLogout() {
  await session.logout()
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

.account-overlay {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 600;
}

.feed-status-overlay {
  position: absolute;
  top: 50px;
  right: 12px;
  z-index: 500;
  pointer-events: none;
}
</style>
