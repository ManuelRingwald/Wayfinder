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
        <v-list-item v-for="e in events" :key="e.id" class="event-panel__row">
          <template #prepend>
            <v-icon :icon="meta(e.severity).icon" :color="meta(e.severity).color" size="small" />
          </template>
          <v-list-item-title class="text-body-2">{{ e.message }}</v-list-item-title>
          <template #append>
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
import { SEVERITY_META, SEV_INFO } from '@/map/events.js'

const emit = defineEmits(['close'])
const store = useEventsStore()
const events = computed(() => store.events)

// meta resolves a severity to its icon/colour, falling back to info for an
// unknown value (fail-safe: never render a broken row).
function meta(severity) {
  return SEVERITY_META[severity] ?? SEVERITY_META[SEV_INFO]
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
  max-height: min(60vh, 420px);
  overflow-y: auto;
}
.event-panel__row {
  min-height: 40px;
}
</style>
