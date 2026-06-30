<template>
  <!-- ASD-013: the ASD is a full-screen scope, no title bar. This is the
       operational view at route '/'. It gates on the session (ADR 0014: auth is
       always on): until the identity probe resolves we show a spinner, then either
       the login screen (anon) or the live picture (authed). The map + WebSocket
       only mount once authenticated, so /ws never opens unauthenticated. -->
  <v-main
    v-if="session.status === 'loading'"
    class="d-flex justify-center align-center"
    style="min-height: 100vh"
  >
    <v-progress-circular indeterminate color="primary" size="48" />
  </v-main>

  <LoginCard
    v-else-if="session.status === 'anon'"
    :error="session.error"
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
    />

    <v-main style="padding: 0; position: relative">
      <MapCanvas
        ref="mapCanvas"
        @track-click="onTrackClick"
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
import { ref, onMounted } from 'vue'
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

// Resolve the session on entry — decides login screen vs. live picture.
onMounted(() => { session.probe() })

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
