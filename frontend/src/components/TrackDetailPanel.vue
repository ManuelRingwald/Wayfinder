<template>
  <v-bottom-sheet v-if="!mdAndUp" v-model="open" :scrim="false">
    <TrackDetailCard @close="emit('close')" />
  </v-bottom-sheet>

  <v-card
    v-else
    class="track-detail-card"
    elevation="4"
    rounded="lg"
  >
    <TrackDetailCard @close="emit('close')" />
  </v-card>
</template>

<script setup>
import { computed } from 'vue'
import { useDisplay } from 'vuetify'
import { useAsdStore } from '@/stores/asd.js'
import TrackDetailCard from './TrackDetailCard.vue'

const emit = defineEmits(['close'])
const { mdAndUp } = useDisplay()
const store = useAsdStore()
const open = computed(() => store.selectedTrack !== null)
</script>

<style scoped>
.track-detail-card {
  position: fixed;
  /* #184: top-right was a dead end — it stacked on top of the feed-status
     badge/header cluster (AsdView .top-right-cluster) AND the map-controls
     rail (#169, top ~100px), and its max-height could run under the OSM
     attribution chip (bottom-right). Moved to top-left instead of fighting
     for space in that column.
     left: 68px clears the 56px collapsed nav rail + 12px gap — the same
     offset AsdView's .scope-legend-overlay already uses for the identical
     reason. That horizontal offset alone also clears the MapLibre compass
     control (engine.js NavigationControl 'top-left'; MapLibre renders it at
     left 10px, 29px button — right edge at 39px, well inside our 68px start),
     so no extra top margin is needed to dodge it: top: 12px matches the
     corner-hugging convention used elsewhere (scope-legend-overlay,
     mobile-menu-btn). */
  top: 12px;
  left: 68px;
  width: 292px;
  /* Same 76px total inset as before, redistributed: only 12px is spent at the
     top (nothing to clear there once left is out from under the compass), so
     the remaining 64px goes to the bottom — enough to clear the collapsed
     scope-legend-overlay toggle (~30px tall, anchored at the same left: 68px,
     bottom: 12px), which would otherwise sit directly under a tall panel. */
  max-height: calc(100vh - 76px);
  overflow-y: auto;
  z-index: 10;
  /* Design System v1: floating chrome over the WebGL canvas pairs elevation
     with a faint hairline so it separates cleanly from the map. */
  border: var(--wf-chrome-border);
}
</style>
