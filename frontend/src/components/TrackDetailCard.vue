<template>
  <v-card-title class="d-flex align-center justify-space-between pt-3 pb-1">
    <span class="text-h6">{{ track.callsign ?? `#${track.track_num}` }}</span>
    <v-btn icon="mdi-close" size="small" variant="text" @click="emit('close')" />
  </v-card-title>
  <v-card-text class="pb-3">
    <v-list density="compact" class="pa-0">
      <v-list-item v-if="track.flight_level_ft != null" prepend-icon="mdi-airplane-cruise">
        <v-list-item-title>{{ flLabel }}</v-list-item-title>
        <v-list-item-subtitle>Flugfläche</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="groundSpeedKt" prepend-icon="mdi-speedometer">
        <v-list-item-title>{{ groundSpeedKt }} kt</v-list-item-title>
        <v-list-item-subtitle>Bodengeschwindigkeit</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="track.mode3a != null" prepend-icon="mdi-identifier">
        <v-list-item-title>{{ mode3aStr }}</v-list-item-title>
        <v-list-item-subtitle>Mode 3/A (Squawk)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item prepend-icon="mdi-numeric">
        <v-list-item-title>{{ track.track_num }}</v-list-item-title>
        <v-list-item-subtitle>Track-Nummer</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="track.status" prepend-icon="mdi-information-outline">
        <v-list-item-title>{{ statusLabel }}</v-list-item-title>
        <v-list-item-subtitle>Status</v-list-item-subtitle>
      </v-list-item>
    </v-list>
  </v-card-text>
</template>

<script setup>
import { computed } from 'vue'
import { useAsdStore } from '@/stores/asd.js'

const emit = defineEmits(['close'])
const store = useAsdStore()
const track = computed(() => store.selectedTrack)

const flLabel = computed(() => {
  if (track.value?.flight_level_ft == null) return ''
  return `FL${Math.round(track.value.flight_level_ft / 100)}`
})

const groundSpeedKt = computed(() => {
  const t = track.value
  if (!t?.vx && !t?.vy) return null
  const kt = Math.round(Math.hypot(t.vx ?? 0, t.vy ?? 0) * 1.9438)
  return kt > 0 ? kt : null
})

const mode3aStr = computed(() => {
  if (track.value?.mode3a == null) return ''
  return track.value.mode3a.toString(8).padStart(4, '0')
})

const statusLabel = computed(() => {
  const s = track.value?.status ?? {}
  if (s.coasting) return 'Coasting'
  if (s.confirmed) return 'Bestätigt'
  return 'Tentativ'
})
</script>
