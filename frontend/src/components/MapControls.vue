<template>
  <!-- ASD-009 / ASD-018 / ASD-019 (ADR 0029/0030): the bottom-right map-control
       zone. Zoom lives here on BOTH desktop and mobile now (mockup "Vorschlag A":
       zoom belongs on the scope, not in the tool rail). The recenter/fullscreen
       viewport actions are added here only on MOBILE — on desktop they are the
       last child of AsdView's top-right cluster (ADR 0029), so we don't duplicate
       them. The stack is a flow zone (flex column), so a future control drops in
       as a flex child rather than a free-floating, guessed offset. -->
  <div class="map-controls">
    <ZoomControls @zoom-in="$emit('zoom-in')" @zoom-out="$emit('zoom-out')" />
    <!-- Recenter + fullscreen — mobile only here; desktop hosts them in the
         top-right cluster. Shared component (no duplicated logic). -->
    <ViewportControls v-if="!mdAndUp" @recenter="$emit('recenter')" />
  </div>
</template>

<script setup>
import { useDisplay } from 'vuetify'
import ZoomControls from './ZoomControls.vue'
import ViewportControls from './ViewportControls.vue'

const { mdAndUp } = useDisplay()
defineEmits(['recenter', 'zoom-in', 'zoom-out'])
</script>

<style scoped>
.map-controls {
  position: absolute;
  /* Bottom-right overlay zone. Desktop default: the overlay gap, lifted clear of
     the MapLibre compact attribution ⓘ that sits in the very corner (same idea as
     the track-detail card's attribution clearance). The mobile override below
     lifts the whole stack above the bottom tab bar instead. */
  bottom: calc(var(--wf-overlay-gap, 12px) + 22px + var(--wf-safe-bottom, 0px));
  right: calc(var(--wf-overlay-gap, 12px) + var(--wf-safe-right, 0px));
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
  pointer-events: none; /* pass clicks through to map except on the buttons */
}

/* #194 — Mobile (< md): the navigation rail is not rendered on phones/tablet-
   portrait, so this stack carries zoom AND the viewport actions, lifted above the
   bottom tab bar and clear of the home indicator. */
@media (max-width: 959.98px) {
  .map-controls {
    bottom: calc(12px + var(--wf-bottom-nav-h, 64px) + var(--wf-safe-bottom, 0px));
  }
}
</style>
