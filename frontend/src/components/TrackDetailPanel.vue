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
  /* Design template: the track detail anchors top-right below the header cluster
     (top 64, right 12, width 292), not bottom-right — the old bottom-right spot
     collided with the "<width> NM Breite" readout. */
  top: 64px;
  right: 12px;
  width: 292px;
  max-height: calc(100vh - 76px);
  overflow-y: auto;
  z-index: 10;
  /* Design System v1: floating chrome over the WebGL canvas pairs elevation
     with a faint hairline so it separates cleanly from the map. */
  border: var(--wf-chrome-border);
}
</style>
