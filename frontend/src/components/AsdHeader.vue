<template>
  <!-- Reskin Häppchen 3a: top-centre header over the scope. Shows the tenant's
       optional ICAO location indicator (from whoami; omitted when unset — display
       config, not track data) and a live UTC wall-clock, the controller's time
       reference. Mono + tonal floating chrome so it reads over the moving map. -->
  <div class="asd-header wf-mono">
    <span v-if="icao" class="asd-header__icao">{{ icao }}</span>
    <span class="asd-header__time">{{ utc }}</span>
    <span class="asd-header__zone">UTC</span>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useSessionStore } from '@/stores/session.js'

const session = useSessionStore()
const icao = computed(() => session.icao)

// Live UTC clock. This is the operator's wall-clock reference (distinct from the
// CAT062 data-time that drives track processing) — the one place a wall-clock is
// correct in the ASD.
const utc = ref(formatUtc())
let timer = null

function formatUtc() {
  const d = new Date()
  const p = (n) => String(n).padStart(2, '0')
  return `${p(d.getUTCHours())}:${p(d.getUTCMinutes())}:${p(d.getUTCSeconds())}`
}

onMounted(() => {
  timer = setInterval(() => { utc.value = formatUtc() }, 1000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.asd-header {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  padding: 5px 10px;
  font-size: 13px;
  letter-spacing: 0.05em;
  color: var(--wf-on-surface);
  background: rgba(14, 22, 34, 0.85); /* surface @ 85% */
  backdrop-filter: blur(4px);
  border: var(--wf-chrome-border);
  border-radius: var(--wf-radius-sm);
  pointer-events: none;
  user-select: none;
}
.asd-header__icao {
  color: var(--wf-on-surface);
  font-weight: 500;
}
.asd-header__time {
  color: var(--wf-primary); /* cyan — the live element */
}
.asd-header__zone {
  color: var(--wf-on-surface-variant);
  font-size: 11px;
}
</style>
