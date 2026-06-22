<template>
  <!-- ASD-013: the ASD is a full-screen scope, no title bar. On mobile a minimal
       floating hamburger opens the navigation drawer. This is the operational view
       at route '/'; navigating away (e.g. to /admin) unmounts it and tears down the
       map + WebSocket via MapCanvas.onUnmounted → mapEngine.destroy() (WF2-32). -->
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
    <!-- Feed health banner (CAT065 heartbeat, bug #54). Floats top-right above
         the map so it is visible regardless of navigation rail state. -->
    <div class="feed-status-overlay">
      <FeedStatusChip />
    </div>
  </v-main>

  <TrackDetailPanel
    v-if="store.selectedTrack"
    @close="store.clearTrackSelection()"
  />
</template>

<script setup>
import { ref } from 'vue'
import { useDisplay } from 'vuetify'
import { useAsdStore } from '@/stores/asd.js'
import NavigationRail from '@/components/NavigationRail.vue'
import MapCanvas from '@/components/MapCanvas.vue'
import TrackDetailPanel from '@/components/TrackDetailPanel.vue'
import FeedStatusChip from '@/components/FeedStatusChip.vue'

const { mdAndUp } = useDisplay()
const store = useAsdStore()
const drawerOpen = ref(true)
const mapCanvas = ref(null)

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

.feed-status-overlay {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 500;
  pointer-events: none;
}
</style>
