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
  /* RIGHT edge (operator request 2026-07-08): the flight info must NEVER sit on
     the left, where the expanding navigation-rail panel (LAYER / FILTER) covers
     it — that was the failure of #184's top-left placement. It hugs the right
     edge instead, and starts BELOW the top-right cluster (AsdView
     .top-right-cluster: header / feed badge / profile+bell action row, top 12px)
     AND the map-controls rail (MapControls, top ~140px), so it clears that
     chrome rather than stacking on it. ~220px keeps the same controls→detail
     clearance as before (map-controls moved down 40px when the profile+bell
     action row joined the cluster, icons-overlap fix); the bottom inset keeps
     the scroll area above the OSM attribution chip (bottom-right). */
  top: calc(var(--v-layout-top, 0px) + 220px + var(--wf-safe-top, 0px));
  right: calc(var(--wf-overlay-gap, 12px) + var(--wf-safe-right, 0px));
  /* #194: fluid width (token default 292px, one step wider on a 24″ display),
     capped to the viewport so it never overflows a narrow tablet-landscape. */
  width: min(var(--wf-overlay-detail-width, 292px), calc(100vw - 24px));
  max-height: calc(100vh - 220px - var(--wf-overlay-gap, 12px) - 28px);
  overflow-y: auto;
  z-index: 10;
  /* Design System v1: floating chrome over the WebGL canvas pairs elevation
     with a faint hairline so it separates cleanly from the map. */
  border: var(--wf-chrome-border);
}
</style>
