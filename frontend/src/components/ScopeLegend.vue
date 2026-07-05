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
        <!-- Draw the SAME geometric mark the map paints (map/layers.js
             makeTrackIcon): ADS-B diamond, SSR square (filled), PSR a HOLLOW
             ring, FLARM an upward triangle (#185); combined keeps its letter
             glyph (K), which is how the map renders that superset source too. -->
        <svg
          v-if="p.kind === 'adsb' || p.kind === 'ssr' || p.kind === 'psr' || p.kind === 'flarm'"
          class="scope-legend__mark"
          width="16"
          height="16"
          viewBox="0 0 16 16"
        >
          <path v-if="p.kind === 'adsb'" d="M8 2 L14 8 L8 14 L2 8 Z" fill="var(--wf-scope-label)" />
          <rect v-else-if="p.kind === 'ssr'" x="2.6" y="2.6" width="10.8" height="10.8" fill="var(--wf-scope-label)" />
          <path v-else-if="p.kind === 'flarm'" d="M8 2.5 L14 13.5 L2 13.5 Z" fill="var(--wf-scope-label)" />
          <circle v-else cx="8" cy="8" r="5.6" fill="none" stroke="var(--wf-scope-label)" stroke-width="1.8" />
        </svg>
        <span v-else class="scope-legend__glyph">{{ p.glyph }}</span>
        <span class="scope-legend__label">{{ p.label }}</span>
      </div>
      <div class="scope-legend__section wf-overline">Farbe · Status</div>
      <div v-for="s in states" :key="s.key" class="scope-legend__row">
        <span
          class="scope-legend__dot"
          :style="s.hollow ? { border: `2px solid ${s.color}`, background: 'transparent' } : { background: s.color }"
        />
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
// Coasting is shown as a hollow ring to mirror the map, where a coasting track
// is drawn as an outline (hollow) rather than a filled symbol.
const states = [
  { key: 'confirmed', label: 'Bestätigt', color: TRACK_STATE_COLORS.confirmed },
  { key: 'coasting', label: 'Coasting (hohl)', color: TRACK_STATE_COLORS.coasting, hollow: true },
  { key: 'tentative', label: 'Tentativ', color: TRACK_STATE_COLORS.tentative },
  { key: 'filtered', label: 'Ausgefiltert (FL)', color: TRACK_STATE_COLORS.filtered },
]
</script>

<style scoped>
.scope-legend {
  background: rgba(14, 22, 34, 0.96); /* surface @ 96% (design template legend) */
  backdrop-filter: blur(4px);
  border: var(--wf-chrome-border);
  border-radius: var(--wf-radius-md); /* 12px (design template legend) */
  box-shadow: var(--wf-elevation-4);
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
  width: 232px; /* fixed open width (design template) */
  padding: 0 12px 10px;
}
.scope-legend__section {
  margin-top: 8px;
  margin-bottom: 2px;
}
.scope-legend__row {
  display: flex;
  align-items: center;
  gap: 9px;
  min-height: 20px;
}
.scope-legend__glyph {
  width: 16px;
  text-align: center;
  font-weight: 700;
  font-size: 13px;
  line-height: 1;
  color: var(--wf-on-surface);
  flex-shrink: 0;
}
/* Geometric provenance mark (SVG), matching the map symbols (design template) */
.scope-legend__mark {
  flex-shrink: 0;
  display: block;
}
.scope-legend__dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  box-sizing: border-box; /* keep hollow (bordered) and filled dots the same size */
  flex-shrink: 0;
}
.scope-legend__label {
  font-size: 11.5px;
  color: var(--wf-on-surface);
  opacity: 0.9;
}
</style>
