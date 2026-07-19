<template>
  <!-- #277 (ADR 0028): sector search. The Lotse types a street/place name and
       gets hits from the server-side index over the tenant's own AOI (use case:
       "eine Drohne startet aus der Friedrichstraße — wo ist die?"). Picking a
       hit drops the magenta marker + eases the camera there (via AsdView →
       MapCanvas → engine). The index is built lazily on the first search; while
       it builds, the server answers 202 and we poll until it is ready. -->
  <div class="map-search" @keydown.esc.stop="onClear">
    <v-text-field
      v-model="q"
      density="compact"
      variant="solo-filled"
      hide-details
      clearable
      placeholder="Ort / Straße suchen…"
      prepend-inner-icon="mdi-magnify"
      aria-label="Sektor-Suche"
      autocomplete="off"
      @click:clear="onClear"
      @keydown.enter="onEnter"
    />
    <div v-if="hint" class="map-search__hint">{{ hint }}</div>
    <v-list v-else-if="results.length" class="map-search__list" density="compact">
      <v-list-item
        v-for="(h, i) in results"
        :key="`${h.name}-${i}`"
        @click="onSelect(h)"
      >
        <v-list-item-title class="map-search__name">{{ h.name }}</v-list-item-title>
        <v-list-item-subtitle class="map-search__cat">{{ hitDetail(h) }}</v-list-item-subtitle>
      </v-list-item>
    </v-list>
  </div>
</template>

<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'

const emit = defineEmits(['select', 'clear'])

// Debounce keeps the request rate humane while typing; the building-state
// retry re-asks the SAME query until the server-side index build finishes.
const DEBOUNCE_MS = 300
const BUILDING_RETRY_MS = 1500
// Server-side build failures retry in the background; poll them more gently
// than the normal build progress.
const UNAVAILABLE_RETRY_MS = 3000
const MIN_QUERY_LEN = 2 // mirrors the server's minimum (pkg/basemapsearch)

const q = ref('')
const results = ref([])
// idle | building | ready | noarea | unavailable | error — drives the hint
// line. 'unavailable' = the SERVER reported a failed index build (it keeps
// retrying; we keep polling); 'error' = the request itself failed (no polling).
const status = ref('idle')

let debounceTimer = null
let retryTimer = null
// Monotonic request id: a stale (slow) response must never overwrite the
// results of a newer query.
let requestSeq = 0

const hint = computed(() => {
  if (status.value === 'building') return 'Suchindex wird aufgebaut …'
  if (status.value === 'noarea') return 'Kein Suchgebiet konfiguriert.'
  if (status.value === 'unavailable') return 'Suche derzeit nicht verfügbar — neuer Versuch läuft …'
  if (status.value === 'error') return 'Suche derzeit nicht verfügbar.'
  if (status.value === 'ready' && results.value.length === 0) return 'Keine Treffer.'
  return ''
})

// Best-effort German labels for the BKG vector-tile layer names the index
// reports as hit category; unknown schema names pass through unchanged.
const CATEGORY_LABELS = {
  verkehrslinie: 'Straße / Weg',
  verkehrspunkt: 'Verkehr',
  siedlung: 'Siedlung',
  siedlungsflaeche: 'Siedlung',
  gewaessslinie: 'Gewässer',
  gewaesserlinie: 'Gewässer',
  gewaesserflaeche: 'Gewässer',
  vegetationsflaeche: 'Vegetation',
  grenze: 'Grenze',
  name: 'Ort',
}
function categoryLabel(cat) {
  return CATEGORY_LABELS[cat] || cat || ''
}

// #277 Nachtrag: the secondary line that tells same-named hits apart —
// category · nearest town · radial (distance + bearing from the sector centre).
// Each piece is optional and dropped when absent, so a hit with no nearby town
// still shows category + radial, and one with neither shows just the category.
function hitDetail(h) {
  const parts = [categoryLabel(h.category)]
  if (h.near) parts.push(`bei ${h.near}`)
  // Radial is always sent for a real hit; suppress it only for the meaningless
  // ~0 NM case (a feature essentially at the sector centre).
  if (typeof h.dist_nm === 'number' && h.dist_nm >= 0.1 && typeof h.bearing_deg === 'number') {
    const dist = h.dist_nm < 10 ? h.dist_nm.toFixed(1) : String(Math.round(h.dist_nm))
    const brg = String(h.bearing_deg).padStart(3, '0')
    parts.push(`${dist} NM · ${brg}°`)
  }
  return parts.filter(Boolean).join(' · ')
}

function scheduleSearch() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(runSearch, DEBOUNCE_MS)
}

async function runSearch() {
  if (retryTimer) { clearTimeout(retryTimer); retryTimer = null }
  const query = (q.value || '').trim()
  if (query.length < MIN_QUERY_LEN) {
    results.value = []
    status.value = 'idle'
    return
  }
  const seq = ++requestSeq
  try {
    const res = await fetch(`/api/basemap/search?q=${encodeURIComponent(query)}`)
    if (seq !== requestSeq) return // superseded by a newer query
    if (res.status === 202) {
      // Index build in progress (single-flight server-side) — poll gently.
      status.value = 'building'
      results.value = []
      retryTimer = setTimeout(runSearch, BUILDING_RETRY_MS)
      return
    }
    if (res.status === 503) {
      status.value = 'noarea'
      results.value = []
      return
    }
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const body = await res.json()
    if (seq !== requestSeq) return
    if (body.status === 'error') {
      // The server's index build failed (e.g. tile upstream down) and it keeps
      // retrying in the background — say so honestly instead of pretending
      // "building" forever, and poll on gently.
      status.value = 'unavailable'
      results.value = []
      retryTimer = setTimeout(runSearch, UNAVAILABLE_RETRY_MS)
      return
    }
    results.value = Array.isArray(body.results) ? body.results : []
    status.value = 'ready'
  } catch (err) {
    if (seq !== requestSeq) return
    console.warn('basemap search failed:', err)
    status.value = 'error'
    results.value = []
  }
}

// Typing re-arms the debounce (v-model drives q).
watch(q, scheduleSearch)

function onSelect(hit) {
  emit('select', hit)
  // The picked hit stays in the field (the marker carries the name); collapse
  // the list so the scope is unobstructed again.
  results.value = []
  status.value = 'idle'
}

function onEnter() {
  if (results.value.length > 0) onSelect(results.value[0])
}

function onClear() {
  q.value = ''
  results.value = []
  status.value = 'idle'
  if (debounceTimer) { clearTimeout(debounceTimer); debounceTimer = null }
  if (retryTimer) { clearTimeout(retryTimer); retryTimer = null }
  requestSeq++
  emit('clear')
}

onUnmounted(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
  if (retryTimer) clearTimeout(retryTimer)
})
</script>

<style scoped>
.map-search {
  width: 260px;
  max-width: calc(100vw - 24px);
}

.map-search__hint {
  margin-top: 4px;
  padding: 6px 10px;
  border-radius: 6px;
  background: rgba(var(--v-theme-surface), 0.92);
  color: rgba(var(--v-theme-on-surface), 0.75);
  font-size: 12px;
}

.map-search__list {
  margin-top: 4px;
  border-radius: 6px;
  max-height: 260px;
  overflow-y: auto;
  background: rgba(var(--v-theme-surface), 0.96);
}

.map-search__name {
  font-size: 13px;
}

.map-search__cat {
  font-size: 11px;
}
</style>
