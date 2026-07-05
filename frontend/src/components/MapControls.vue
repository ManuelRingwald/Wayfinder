<template>
  <!-- ASD-009: Floating map control buttons positioned on the right edge of the
       map canvas. All controls are purely presentational — they emit named events
       and let MapCanvas delegate to the map engine, keeping the engine
       framework-agnostic. -->
  <div class="map-controls">
    <!-- Häppchen 3: zoom moved to the navigation rail; these are the viewport
         actions (recenter to configured centre, fullscreen toggle). -->
    <v-btn-group
      direction="vertical"
      density="compact"
      color="surface"
      variant="flat"
      class="map-controls__group elevation-4 rounded-lg"
    >
      <!-- Reset view — centre + zoom + north-up + top-down, the full start view (#169) -->
      <v-btn
        icon
        size="small"
        :ripple="false"
        @click="$emit('recenter')"
      >
        <v-icon>mdi-image-filter-center-focus</v-icon>
        <v-tooltip activator="parent" location="left" text="Ansicht zurücksetzen" />
      </v-btn>

      <!-- Fullscreen toggle -->
      <v-btn
        icon
        size="small"
        :ripple="false"
        @click="toggleFullscreen"
      >
        <v-icon>{{ isFullscreen ? 'mdi-fullscreen-exit' : 'mdi-fullscreen' }}</v-icon>
        <v-tooltip activator="parent" location="left" :text="isFullscreen ? 'Vollbild beenden' : 'Vollbild'" />
      </v-btn>
    </v-btn-group>
  </div>
</template>

<script setup>
import { ref } from 'vue'

defineEmits(['recenter'])

const isFullscreen = ref(false)

function toggleFullscreen() {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen().then(() => {
      isFullscreen.value = true
    })
  } else {
    document.exitFullscreen().then(() => {
      isFullscreen.value = false
    })
  }
}
</script>

<style scoped>
.map-controls {
  position: absolute;
  /* The top-right cluster now stacks TWO rows (ICAO/UTC header + feed badge,
     AsdView .top-right-cluster, top 12px, 8px gap). Start the control stack
     clearly below that cluster so the icons never overlap the feed badge (#169).
     ~100px clears both rows with margin; raised from 50px, which sat on the
     badge row. */
  top: calc(var(--v-layout-top, 0px) + 100px);
  right: 12px;
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  pointer-events: none; /* pass clicks through to map except on buttons */
}

.map-controls__group {
  /* Design System v1: floating chrome over the WebGL canvas — surface fill +
     the faint hairline token so it separates cleanly from the map. */
  pointer-events: all;
  background: rgb(var(--v-theme-surface)) !important;
  border: var(--wf-chrome-border);
}
</style>
