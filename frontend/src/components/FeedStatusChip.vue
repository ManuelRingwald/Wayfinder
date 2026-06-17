<template>
  <v-chip
    v-if="store.feedStatus !== 'unknown'"
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
