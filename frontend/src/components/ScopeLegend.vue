<template>
  <!-- Reskin Häppchen 3b: floating, collapsible scope legend (bottom-left).
       Two keys the map actually uses: SHAPE = surveillance provenance (glyphs
       from map/layers.js, filtered to the tenant's feeds) and COLOUR = track
       lifecycle state. Deliberately omits target-type colours (Firefly emits
       civil only) and the alarm section (safety nets are a later chunk) —
       no legend entry without data behind it. -->
  <div class="scope-legend">
    <button type="button" class="scope-legend__toggle" :aria-expanded="open" @click="open = !open">
      <v-icon size="16">mdi-map-legend</v-icon>
      <span class="scope-legend__title wf-overline">Legende</span>
      <v-icon size="16" class="scope-legend__chev">{{ open ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
    </button>
    <div v-if="open" class="scope-legend__body">
      <div class="scope-legend__section wf-overline">Form · Herkunft</div>
      <div v-for="p in provenance" :key="p.label" class="scope-legend__row">
        <span class="scope-legend__glyph">{{ p.glyph }}</span>
        <span class="scope-legend__label">{{ p.label }}</span>
      </div>
      <div class="scope-legend__section wf-overline">Farbe · Status</div>
      <div v-for="s in states" :key="s.key" class="scope-legend__row">
        <span class="scope-legend__dot" :style="{ background: s.color }" />
        <span class="scope-legend__label">{{ s.label }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useSessionStore } from '@/stores/session.js'
import { filterProvenanceLegend } from '@/map/provenance.js'
import { TRACK_STATE_COLORS } from '@/map/constants.js'

const session = useSessionStore()
const open = ref(false)

// Shape = provenance, filtered to the tenant's actual feeds (#107).
const provenance = computed(() => filterProvenanceLegend(session.sensorClasses))

// Colour = the track lifecycle state the map paints (precedence:
// filtered > coasting > confirmed > tentative). Only rendered states are
// listed — no target-type colours (no wire data) and no alarm rows yet.
const states = [
  { key: 'confirmed', label: 'Bestätigt', color: TRACK_STATE_COLORS.confirmed },
  { key: 'coasting', label: 'Coasting', color: TRACK_STATE_COLORS.coasting },
  { key: 'tentative', label: 'Tentativ', color: TRACK_STATE_COLORS.tentative },
  { key: 'filtered', label: 'Ausgefiltert (FL)', color: TRACK_STATE_COLORS.filtered },
]
</script>

<style scoped>
.scope-legend {
  background: rgba(14, 22, 34, 0.85); /* surface @ 85% */
  backdrop-filter: blur(4px);
  border: var(--wf-chrome-border);
  border-radius: var(--wf-radius-sm);
  box-shadow: var(--wf-elevation-4);
  min-width: 158px;
  pointer-events: all;
  overflow: hidden;
}
.scope-legend__toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 6px 10px;
  background: transparent;
  border: 0;
  cursor: pointer;
  color: var(--wf-on-surface);
}
.scope-legend__title {
  flex: 1;
  text-align: left;
}
.scope-legend__chev {
  opacity: 0.6;
}
.scope-legend__body {
  padding: 0 10px 8px;
}
.scope-legend__section {
  margin-top: 8px;
  margin-bottom: 2px;
}
.scope-legend__row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 22px;
}
.scope-legend__glyph {
  width: 14px;
  text-align: center;
  font-weight: 700;
  font-size: 13px;
  line-height: 1;
  color: var(--wf-on-surface);
  flex-shrink: 0;
}
.scope-legend__dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.scope-legend__label {
  font-size: 12px;
  color: var(--wf-on-surface);
  opacity: 0.9;
}
</style>
