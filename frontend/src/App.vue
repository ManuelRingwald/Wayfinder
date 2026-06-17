<template>
  <v-app>
    <v-app-bar elevation="0" color="surface" density="compact">
      <template #prepend>
        <v-app-bar-nav-icon
          v-if="!mdAndUp"
          @click="drawerOpen = !drawerOpen"
        />
      </template>
      <v-app-bar-title>
        <span class="text-primary font-weight-bold">Wayfinder</span>
        <span class="text-medium-emphasis text-body-2 ml-2">ASD</span>
      </v-app-bar-title>
      <template #append>
        <FeedStatusChip class="mr-3" />
      </template>
    </v-app-bar>

    <LayerSidebar
      v-model="drawerOpen"
      @layer-toggle="onLayerToggle"
      @fl-filter-change="onFlFilterChange"
    />

    <v-main style="padding: 0">
      <MapCanvas
        ref="mapCanvas"
        @track-click="onTrackClick"
      />
    </v-main>

    <TrackDetailPanel
      v-if="store.selectedTrack"
      @close="store.clearTrackSelection()"
    />
  </v-app>
</template>

<script setup>
import { ref } from 'vue'
import { useDisplay } from 'vuetify'
import { useAsdStore } from '@/stores/asd.js'
import LayerSidebar from './components/LayerSidebar.vue'
import FeedStatusChip from './components/FeedStatusChip.vue'
import MapCanvas from './components/MapCanvas.vue'
import TrackDetailPanel from './components/TrackDetailPanel.vue'

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
