<template>
  <v-card-title class="d-flex align-center justify-space-between pt-3 pb-1">
    <span class="text-h6">{{ track.callsign ?? `#${track.track_num}` }}</span>
    <v-btn icon="mdi-close" size="small" variant="text" @click="emit('close')" />
  </v-card-title>
  <v-card-text class="pb-3">
    <v-list density="compact" class="pa-0">
      <v-list-item v-if="track.flight_level_ft != null" prepend-icon="mdi-airplane-cruise">
        <v-list-item-title class="wf-mono">{{ flLabel }}</v-list-item-title>
        <v-list-item-subtitle>Flugfläche</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="track.flight_level_ft != null" prepend-icon="mdi-swap-vertical">
        <v-list-item-title>{{ vTrendGlyph }}{{ vTrendLabel }}</v-list-item-title>
        <v-list-item-subtitle>Vertikaltendenz</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="groundSpeedKt" prepend-icon="mdi-speedometer">
        <v-list-item-title class="wf-mono">{{ groundSpeedKt }} kt</v-list-item-title>
        <v-list-item-subtitle>Bodengeschwindigkeit</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="headingLabel" prepend-icon="mdi-compass-outline">
        <v-list-item-title class="wf-mono">{{ headingLabel }}</v-list-item-title>
        <v-list-item-subtitle>Kurs über Grund</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="positionLabel" prepend-icon="mdi-crosshairs-gps">
        <v-list-item-title class="wf-mono">{{ positionLabel }}</v-list-item-title>
        <v-list-item-subtitle>Position (WGS84)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="provenanceLabel" prepend-icon="mdi-radar">
        <v-list-item-title>{{ provenanceLabel }}</v-list-item-title>
        <v-list-item-subtitle>Herkunft</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="sensorAges.length" prepend-icon="mdi-access-point">
        <v-list-item-title class="d-flex flex-wrap ga-1">
          <v-chip
            v-for="s in sensorAges"
            :key="s.key"
            size="x-small"
            variant="tonal"
            :color="s.fresh ? 'success' : undefined"
          >
            {{ s.label }} {{ formatAge(s.ageS) }}
          </v-chip>
        </v-list-item-title>
        <v-list-item-subtitle>Sensor-Aktualität</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="track.mode_3a != null" prepend-icon="mdi-identifier">
        <v-list-item-title class="wf-mono">{{ mode3aStr }}</v-list-item-title>
        <v-list-item-subtitle>Mode 3/A (Squawk)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="icaoLabel" prepend-icon="mdi-airplane">
        <v-list-item-title class="wf-mono">{{ icaoLabel }}</v-list-item-title>
        <v-list-item-subtitle>ICAO-Adresse (Mode S)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item prepend-icon="mdi-numeric">
        <v-list-item-title class="wf-mono">{{ track.track_num }}</v-list-item-title>
        <v-list-item-subtitle>Track-Nummer</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="accuracyLabel" prepend-icon="mdi-target-variant">
        <v-list-item-title class="wf-mono">{{ accuracyLabel }}</v-list-item-title>
        <v-list-item-subtitle>Positionsgenauigkeit</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="systemLabel" prepend-icon="mdi-server-network">
        <v-list-item-title class="wf-mono">{{ systemLabel }}</v-list-item-title>
        <v-list-item-subtitle>System (SAC/SIC)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item prepend-icon="mdi-information-outline">
        <v-list-item-title>{{ statusLabel }}</v-list-item-title>
        <v-list-item-subtitle>Status</v-list-item-subtitle>
      </v-list-item>
    </v-list>
  </v-card-text>
</template>

<script setup>
import { computed } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { PROVENANCE_LABELS } from '@/map/provenance.js'
import {
  formatLatLon,
  formatHeading,
  formatIcao,
  formatAccuracy,
  formatAge,
  verticalTrendLabel,
  sensorAgeList,
} from '@/map/trackDetail.js'

const emit = defineEmits(['close'])
const store = useAsdStore()
const track = computed(() => store.selectedTrack)

// WF2-40: surveillance source label, from the provenance baked onto the track
// feature when it was selected (see provenance.js / tracks.js).
const provenanceLabel = computed(() => PROVENANCE_LABELS[track.value?.provenance] ?? '')

const flLabel = computed(() => {
  if (track.value?.flight_level_ft == null) return ''
  return `FL${Math.round(track.value.flight_level_ft / 100)}`
})

// ASD-011: vertical tendency worded (Steigend/Sinkend/Gleichbleibend), with the
// glyph as a leading cue when it is climbing/descending.
const vTrendLabel = computed(() => verticalTrendLabel(track.value?.vertical_trend))
const vTrendGlyph = computed(() => {
  const t = track.value?.vertical_trend
  return t === '▲' || t === '▼' ? `${t} ` : ''
})

const groundSpeedKt = computed(() => {
  const t = track.value
  if (!t?.vx && !t?.vy) return null
  const kt = Math.round(Math.hypot(t.vx ?? 0, t.vy ?? 0) * 1.9438)
  return kt > 0 ? kt : null
})

// ASD-011: ground-track heading from Vx/Vy (I062/185).
const headingLabel = computed(() => formatHeading(track.value?.vx, track.value?.vy))

// ASD-011: WGS84 position (I062/105).
const positionLabel = computed(() => formatLatLon(track.value?.latitude, track.value?.longitude))

// ASD-011: technologies currently contributing, with freshness.
const sensorAges = computed(() => sensorAgeList(track.value))

const mode3aStr = computed(() => {
  if (track.value?.mode_3a == null) return ''
  return track.value.mode_3a.toString(8).padStart(4, '0')
})

// ASD-011: 24-bit Mode S address (I062/380) as hex.
const icaoLabel = computed(() => formatIcao(track.value?.icao_addr))

// ASD-011: estimated position accuracy (I062/500, metres).
const accuracyLabel = computed(() => formatAccuracy(track.value?.accuracy))

// ASD-011: system source (SAC/SIC, I062/010) that produced the track.
const systemLabel = computed(() => {
  const t = track.value
  if (t?.sac == null || t?.sic == null) return ''
  return `${t.sac}/${t.sic}`
})

const statusLabel = computed(() => {
  if (track.value?.coasting) return 'Coasting'
  if (track.value?.confirmed) return 'Bestätigt'
  return 'Tentativ'
})
</script>
