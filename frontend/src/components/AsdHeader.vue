<template>
  <!-- Reskin Häppchen 3a: top-centre header over the scope. Shows the tenant's
       optional ICAO location indicator (from whoami; omitted when unset — display
       config, not track data) and a live UTC wall-clock, the controller's time
       reference. Mono + tonal floating chrome so it reads over the moving map. -->
  <div class="asd-header wf-mono">
    <span v-if="icao" class="asd-header__icao">{{ icao }}</span>
    <span class="asd-header__time">{{ utc }}</span>
    <span class="asd-header__zone">UTC</span>
    <!-- WX-B (ADR 0016): QNH infobox. Gated per tenant by the qnh feature; shown
         only once the backend has a reading. QNH is whole hPa; a stale reading is
         dimmed and marked with '*' rather than hidden, so an old value is visible
         rather than silently trusted. -->
    <span
      v-if="showQnh"
      class="asd-header__qnh"
      :class="{ 'asd-header__qnh--stale': qnh.stale }"
      :title="qnhTitle"
    >QNH {{ qnh.qnh_hpa }}<span v-if="qnh.stale" class="asd-header__qnh-flag">*</span></span>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useSessionStore } from '@/stores/session.js'
import { useWeatherStore } from '@/stores/weather.js'

const session = useSessionStore()
const icao = computed(() => session.icao)

// QNH infobox (WX-B). Only a qnh-entitled tenant sees it, and only once the
// backend proxy has a reading for the configured aerodrome.
const weather = useWeatherStore()
const qnh = computed(() => weather.primary)
const showQnh = computed(() => session.hasFeature('qnh') && qnh.value != null)
const qnhTitle = computed(() => {
  if (!qnh.value) return ''
  const base = `QNH ${qnh.value.icao} — ${qnh.value.qnh_hpa} hPa`
  return qnh.value.stale ? `${base} (veraltet)` : base
})

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
  // WX-B: poll QNH. The store is a no-op when the tenant lacks the feature or the
  // backend has no station configured (the span stays hidden).
  weather.start()
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
  weather.stop()
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
  /* "UTC" suffix renders inline at the clock's 13px (design template), muted */
  color: var(--wf-on-surface-variant);
}
.asd-header__qnh {
  /* Divider before QNH so it reads as a distinct field from the clock. */
  padding-left: 8px;
  margin-left: 2px;
  border-left: 1px solid var(--wf-outline-variant, rgba(255, 255, 255, 0.15));
  color: var(--wf-on-surface);
  font-weight: 500;
}
.asd-header__qnh--stale {
  /* A stale QNH is dimmed so an old value never reads as current. */
  color: var(--wf-on-surface-variant);
}
.asd-header__qnh-flag {
  color: var(--wf-warning, #e0a030);
}
</style>
