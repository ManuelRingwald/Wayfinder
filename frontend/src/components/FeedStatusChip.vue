<template>
  <!-- Feed-health indicator (CAT065 heartbeat, Firefly ADR 0018).
       Always visible so the operator can confirm the feed monitor is active.
       States: ok (green) = heartbeat fresh; stale (red) = heartbeat lost;
       unknown (grey) = no heartbeat received yet (e.g. Firefly not yet running). -->
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
  stale: 'error',
  unknown: 'secondary',
}[store.feedStatus] ?? 'secondary'))

const chipLabel = computed(() => ({
  ok: 'FEED OK',
  stale: 'FEED STALE',
  unknown: 'FEED ?',
}[store.feedStatus] ?? 'FEED ?'))
</script>
