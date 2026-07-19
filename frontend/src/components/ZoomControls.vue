<template>
  <!-- ASD-019 (ADR 0030): zoom +/− as a POSITION-NEUTRAL button group, carrying
       no absolute offset of its own — the parent zone (the bottom-right
       .map-controls stack in MapControls) lays it out in flow. Same overlay-zone
       discipline as ViewportControls (ADR 0029): a control is a flex child of a
       zone, never a free-floating element with a guessed offset. Zoom moved here
       from the navigation rail (mockup "Vorschlag A": zoom belongs on the scope,
       bottom-right, not in the tool rail). -->
  <v-btn-group
    direction="vertical"
    density="compact"
    color="surface"
    variant="flat"
    class="zoom-controls elevation-4 rounded-lg"
    :class="{ 'zoom-controls--touch': tabletLandscape }"
  >
    <v-btn icon size="small" :ripple="false" aria-label="Zoom in" @click="$emit('zoom-in')">
      <v-icon>mdi-plus</v-icon>
      <v-tooltip activator="parent" location="left" text="Vergrößern" />
    </v-btn>
    <v-btn icon size="small" :ripple="false" aria-label="Zoom out" @click="$emit('zoom-out')">
      <v-icon>mdi-minus</v-icon>
      <v-tooltip activator="parent" location="left" text="Verkleinern" />
    </v-btn>
  </v-btn-group>
</template>

<script setup>
import { useDisplay } from 'vuetify'

defineEmits(['zoom-in', 'zoom-out'])

// #194 Häppchen 2 parity: on the iPad/tablet-landscape band (`md`) the buttons
// grow to a 44px finger target, matching ViewportControls. Phones/desktop keep
// the compact size.
const { md } = useDisplay()
const tabletLandscape = md
</script>

<style scoped>
.zoom-controls {
  /* Design System v1: floating chrome over the WebGL canvas — surface fill + the
     faint hairline token so it separates cleanly from the map. Interactive, so it
     re-enables pointer events inside the (click-through) overlay zone. */
  pointer-events: all;
  background: rgb(var(--v-theme-surface)) !important;
  border: var(--wf-chrome-border);
}

/* #194 Häppchen 2 parity — iPad/tablet-landscape: enlarge the compact icon
   buttons to a comfortable 44px finger target. */
.zoom-controls--touch :deep(.v-btn) {
  width: var(--wf-touch-min, 44px);
  height: var(--wf-touch-min, 44px);
}
</style>
