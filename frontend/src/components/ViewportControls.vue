<template>
  <!-- ASD-018 (overlay-zone layout, ADR 0029): the viewport actions (recenter,
       fullscreen) as a POSITION-NEUTRAL button group. It carries no absolute
       offset of its own — the parent zone (AsdView's right rail on desktop, the
       mobile control stack in MapControls) lays it out in flow. This is what
       stops the recurring "new chrome overlaps the map controls" bug: the
       controls are a flex child of a zone, never a free-floating element with a
       guessed `top`. -->
  <v-btn-group
    direction="vertical"
    density="compact"
    color="surface"
    variant="flat"
    class="viewport-controls elevation-4 rounded-lg"
    :class="{ 'viewport-controls--touch': tabletLandscape }"
  >
    <!-- Reset view — centre + zoom + north-up + top-down, the full start view (#169) -->
    <v-btn icon size="small" :ripple="false" @click="$emit('recenter')">
      <v-icon>mdi-image-filter-center-focus</v-icon>
      <v-tooltip activator="parent" location="left" text="Ansicht zurücksetzen" />
    </v-btn>

    <!-- Fullscreen toggle -->
    <v-btn icon size="small" :ripple="false" @click="toggleFullscreen">
      <v-icon>{{ isFullscreen ? 'mdi-fullscreen-exit' : 'mdi-fullscreen' }}</v-icon>
      <v-tooltip activator="parent" location="left" :text="isFullscreen ? 'Vollbild beenden' : 'Vollbild'" />
    </v-btn>
  </v-btn-group>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useDisplay } from 'vuetify'

defineEmits(['recenter'])

// #194 Häppchen 2: on the iPad/tablet-landscape band (`md`) the buttons grow to a
// 44px finger target (design mockup). Phones keep the compact size; desktop too.
const { md } = useDisplay()
const tabletLandscape = md
const isFullscreen = ref(false)

// ASD-018 follow-up: the `fullscreenchange` event is the single source of truth
// for the icon state — the browser fires it on EVERY entry/exit, including the
// ones the button never sees (ESC key, F11, the browser's own exit chrome).
// Deriving the ref from the event (rather than setting it in the click handler)
// keeps the icon correct in all of those cases; the old handler only flipped it
// on its own promise, so an ESC exit left the icon stuck on "exit fullscreen".
function syncFullscreen() {
  isFullscreen.value = !!document.fullscreenElement
}

function toggleFullscreen() {
  if (!document.fullscreenElement) {
    // requestFullscreen rejects if the browser blocks it (e.g. no user gesture);
    // swallow it so there is no unhandled rejection. syncFullscreen keeps state.
    document.documentElement.requestFullscreen().catch(() => {})
  } else {
    document.exitFullscreen().catch(() => {})
  }
}

onMounted(() => document.addEventListener('fullscreenchange', syncFullscreen))
onBeforeUnmount(() => document.removeEventListener('fullscreenchange', syncFullscreen))
</script>

<style scoped>
.viewport-controls {
  /* Design System v1: floating chrome over the WebGL canvas — surface fill + the
     faint hairline token so it separates cleanly from the map. Interactive, so it
     re-enables pointer events inside the (click-through) overlay zone. */
  pointer-events: all;
  background: rgb(var(--v-theme-surface)) !important;
  border: var(--wf-chrome-border);
}

/* #194 Häppchen 2 — iPad/tablet-landscape: enlarge the compact icon buttons to a
   comfortable 44px finger target (the size="small" default is ~28px). */
.viewport-controls--touch :deep(.v-btn) {
  width: var(--wf-touch-min, 44px);
  height: var(--wf-touch-min, 44px);
}
</style>
