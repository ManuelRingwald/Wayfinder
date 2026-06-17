<template>
  <!-- ASD-010: Filter-Chips for track status categories, positioned top-centre
       of the map canvas. Each chip shows the live count for that category;
       clicking toggles the category's visibility on the map. -->
  <div class="track-filter-chips">
    <v-chip
      v-for="cat in categories"
      :key="cat.id"
      :color="store.hiddenCategories.has(cat.id) ? undefined : cat.color"
      :variant="store.hiddenCategories.has(cat.id) ? 'outlined' : 'tonal'"
      size="small"
      class="track-filter-chips__chip"
      @click="store.toggleCategoryFilter(cat.id)"
    >
      <template #prepend>
        <span
          class="track-filter-chips__dot"
          :style="{ background: store.hiddenCategories.has(cat.id) ? 'transparent' : cat.color }"
        />
      </template>
      {{ cat.label }}
      <span class="track-filter-chips__count ml-1">
        {{ store.trackCounts[cat.id] }}
      </span>
    </v-chip>
  </div>
</template>

<script setup>
import { useAsdStore } from '@/stores/asd.js'
import { TRACK_COLORS } from '@/map/constants.js'

const store = useAsdStore()

// Category definitions — order and colour follow ATC convention.
const categories = [
  { id: 'confirmed', label: 'Confirmed', color: TRACK_COLORS.friendlyCivil },
  { id: 'coasting',  label: 'Coasting',  color: '#607d8b' },
  { id: 'tentative', label: 'Tentative', color: TRACK_COLORS.unknown },
]
</script>

<style scoped>
.track-filter-chips {
  position: absolute;
  top: 10px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
  display: flex;
  gap: 6px;
  pointer-events: none;
}

.track-filter-chips__chip {
  pointer-events: all;
  backdrop-filter: blur(4px);
  background: rgba(var(--v-theme-surface), 0.85) !important;
}

.track-filter-chips__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 4px;
  border: 1px solid currentColor;
}

.track-filter-chips__count {
  font-weight: 600;
  font-size: 0.75rem;
  opacity: 0.9;
}
</style>
