<template>
  <v-card class="event-panel" elevation="8" rounded="lg">
    <v-card-title class="d-flex align-center justify-space-between py-2">
      <span class="text-subtitle-1">Ereignisse</span>
      <div class="d-flex align-center ga-1">
        <v-btn
          size="small"
          variant="text"
          prepend-icon="mdi-notification-clear-all"
          :disabled="events.length === 0"
          @click="store.clear()"
        >
          Leeren
        </v-btn>
        <v-btn icon="mdi-close" size="small" variant="text" aria-label="Schließen" @click="emit('close')" />
      </div>
    </v-card-title>
    <v-divider />
    <v-card-text class="pa-0 event-panel__body">
      <div v-if="events.length === 0" class="pa-4 text-medium-emphasis text-body-2">
        Keine Ereignisse. Feed-Ausfälle, Verbindungsabbrüche und neue/beendete Tracks
        erscheinen hier.
      </div>
      <v-list v-else density="compact" class="pa-0">
        <v-list-item
          v-for="e in events"
          :key="e.id"
          class="event-panel__row"
          :class="{ 'event-panel__row--selectable': isSelectable(e) }"
          :link="isSelectable(e)"
          @click="onRowClick(e)"
        >
          <template #prepend>
            <v-icon :icon="meta(e.severity).icon" :color="meta(e.severity).color" size="small" />
          </template>
          <v-list-item-title class="text-body-2">{{ e.message }}</v-list-item-title>
          <template #append>
            <!-- A crosshair affordance on rows whose track is still on the scope,
                 so the operator sees the row jumps to and selects that track. -->
            <v-icon
              v-if="isSelectable(e)"
              icon="mdi-crosshairs-gps"
              size="x-small"
              class="event-panel__jump me-1"
              aria-hidden="true"
            />
            <span class="wf-mono text-caption text-medium-emphasis">{{ formatTime(e.ts) }}</span>
          </template>
        </v-list-item>
      </v-list>
    </v-card-text>
  </v-card>
</template>

<script setup>
import { computed } from 'vue'
import { useEventsStore } from '@/stores/events.js'
import { useAsdStore } from '@/stores/asd.js'
import { SEVERITY_META, SEV_INFO } from '@/map/events.js'

const emit = defineEmits(['close', 'select-track'])
const store = useEventsStore()
const asd = useAsdStore()
const events = computed(() => store.events)

// meta resolves a severity to its icon/colour, falling back to info for an
// unknown value (fail-safe: never render a broken row).
function meta(severity) {
  return SEVERITY_META[severity] ?? SEVERITY_META[SEV_INFO]
}

// isSelectable is true for a track-appeared event whose track is STILL on the
// scope (operator request 2026-07-08): only then does clicking the row jump to a
// real target. A "Track N beendet" row stays inert (the track ended — and even
// if the number is later reused, an end event should not jump), as does an
// "erschienen" whose track has since gone (no longer in the live set).
function isSelectable(e) {
  return e.type === 'track-appeared' && asd.liveTrackNums.has(e.trackNum)
}

// onRowClick jumps to + selects the track a still-live event refers to. Inert
// rows (no live track) do nothing.
function onRowClick(e) {
  if (isSelectable(e)) emit('select-track', e.trackNum)
}

// formatTime renders the event's wall-clock timestamp as local HH:MM:SS.
function formatTime(ts) {
  return new Date(ts).toLocaleTimeString('de-DE', { hour12: false })
}
</script>

<style scoped>
.event-panel {
  width: 320px;
  max-width: calc(100vw - 24px);
  background: rgba(14, 22, 34, 0.96);
}
.event-panel__body {
  /* Scrollable so the full ring buffer (up to MAX_EVENTS) is reachable — the
     operator asked to keep more events and scroll through them. */
  max-height: min(60vh, 420px);
  overflow-y: auto;
}
.event-panel__row {
  min-height: 40px;
}
/* A live-track row is actionable (click = jump to + select the track); mark it
   with a pointer + a faint hover tint and reveal the crosshair on hover. */
.event-panel__row--selectable {
  cursor: pointer;
}
.event-panel__jump {
  color: var(--wf-primary);
  opacity: 0.5;
  transition: opacity 0.12s;
}
.event-panel__row--selectable:hover .event-panel__jump {
  opacity: 1;
}
</style>
