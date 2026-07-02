<template>
  <!-- ASD-009: Floating map control buttons positioned on the right edge of the
       map canvas. All controls are purely presentational — they emit named events
       and let MapCanvas delegate to the map engine, keeping the engine
       framework-agnostic. -->
  <div class="map-controls">
    <v-btn-group
      direction="vertical"
      density="compact"
      color="surface"
      variant="flat"
      class="map-controls__group elevation-4 rounded-lg"
    >
      <!-- Zoom in -->
      <v-btn
        icon="mdi-plus"
        size="small"
        :ripple="false"
        @click="$emit('zoom-in')"
      >
        <v-icon>mdi-plus</v-icon>
        <v-tooltip activator="parent" location="left" text="Zoom in" />
      </v-btn>

      <!-- Zoom out -->
      <v-btn
        icon="mdi-minus"
        size="small"
        :ripple="false"
        @click="$emit('zoom-out')"
      >
        <v-icon>mdi-minus</v-icon>
        <v-tooltip activator="parent" location="left" text="Zoom out" />
      </v-btn>
    </v-btn-group>

    <!-- Spacer -->
    <div class="my-2" />

    <v-btn-group
      direction="vertical"
      density="compact"
      color="surface"
      variant="flat"
      class="map-controls__group elevation-4 rounded-lg"
    >
      <!-- Recenter — fly back to the configured map center -->
      <v-btn
        icon
        size="small"
        :ripple="false"
        @click="$emit('recenter')"
      >
        <v-icon>mdi-crosshairs-gps</v-icon>
        <v-tooltip activator="parent" location="left" text="Zentrum" />
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

defineEmits(['zoom-in', 'zoom-out', 'recenter'])

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
  /* Issue #101: the account chip (top 12px) and the feed-status chip (top 50px)
     float at the same right edge with a higher z-index, so anchoring the controls
     at the top overlapped — and covered — the zoom and recenter buttons. Start the
     control stack below both chips so nothing overlaps. */
  top: calc(var(--v-layout-top, 0px) + 88px);
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
