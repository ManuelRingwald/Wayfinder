<template>
  <!-- Feed-health indicator (CAT065 heartbeat + CAT063 sensor status).
       Always visible so the operator can confirm the feed monitor is active.
       States:
         ok       (green)  = heartbeat fresh, all sensors operational
         degraded (yellow) = heartbeat fresh, but sensor fusion degraded (CAT063)
         stale    (red)    = heartbeat lost
         unknown  (grey)   = no heartbeat received yet (e.g. Firefly not running) -->
  <v-chip
    :color="chipColor"
    size="small"
    class="font-weight-bold"
    prepend-icon="mdi-access-point"
  >
    {{ chipLabel }}
  </v-chip>
</template>

<script setup>
import { computed } from 'vue'
import { useAsdStore } from '@/stores/asd.js'

const store = useAsdStore()

const chipColor = computed(() => ({
  ok: 'success',
  degraded: 'warning',
  stale: 'error',
  unknown: 'secondary',
}[store.feedStatus] ?? 'secondary'))

const chipLabel = computed(() => ({
  ok: 'FEED OK',
  degraded: 'SENSOR AUSFALL',
  stale: 'FEED STALE',
  unknown: 'FEED ?',
}[store.feedStatus] ?? 'FEED ?'))
</script>
