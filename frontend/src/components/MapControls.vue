<template>
  <!-- ASD-009 / ASD-018 (ADR 0029): the MOBILE map-control stack. On phones and
       tablet-portrait the navigation rail (which hosts zoom on desktop) is not
       rendered, so zoom lives here alongside the viewport actions, anchored
       bottom-right above the tab bar. On desktop/tablet-landscape this component
       is NOT rendered — there the viewport controls live in AsdView's right-edge
       overlay rail (ViewportControls), which is what ended the recurring
       "controls overlap the top-right cluster" bug (no more guessed `top`). -->
  <div class="map-controls">
    <v-btn-group
      direction="vertical"
      density="compact"
      color="surface"
      variant="flat"
      class="map-controls__group elevation-4 rounded-lg"
    >
      <v-btn icon size="small" :ripple="false" aria-label="Zoom in" @click="$emit('zoom-in')">
        <v-icon>mdi-plus</v-icon>
      </v-btn>
      <v-btn icon size="small" :ripple="false" aria-label="Zoom out" @click="$emit('zoom-out')">
        <v-icon>mdi-minus</v-icon>
      </v-btn>
    </v-btn-group>

    <!-- Recenter + fullscreen — shared with the desktop rail (ViewportControls). -->
    <ViewportControls @recenter="$emit('recenter')" />
  </div>
</template>

<script setup>
import ViewportControls from './ViewportControls.vue'

defineEmits(['recenter', 'zoom-in', 'zoom-out'])
</script>

<style scoped>
.map-controls {
  /* Mobile only (MapCanvas renders this component just for !mdAndUp): bottom-right,
     above the bottom tab bar and clear of the home indicator (#194). */
  position: absolute;
  bottom: calc(12px + var(--wf-bottom-nav-h, 64px) + var(--wf-safe-bottom, 0px));
  right: calc(var(--wf-overlay-gap, 12px) + var(--wf-safe-right, 0px));
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
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
