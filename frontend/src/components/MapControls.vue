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

      <!-- North-up — reset map bearing to 0° -->
      <v-btn
        icon
        size="small"
        :ripple="false"
        @click="$emit('reset-north')"
      >
        <v-icon>mdi-compass-outline</v-icon>
        <v-tooltip activator="parent" location="left" text="Nord oben" />
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

defineEmits(['zoom-in', 'zoom-out', 'recenter', 'reset-north'])

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
  top: 12px;
  right: 12px;
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  pointer-events: none; /* pass clicks through to map except on buttons */
}

.map-controls__group {
  pointer-events: all;
  background: rgb(var(--v-theme-surface)) !important;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.1);
}
</style>
