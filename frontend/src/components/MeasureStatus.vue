<template>
  <!-- Live measuring status over the map. The distance/bearing readout floats as
       a label AT the measure line (anchored to the A–B midpoint, projected to
       screen pixels in map/measure.js); only the one-line instruction stays at
       the bottom. The keyboard shortcuts (R/D/Q select, Esc ends) live here
       because this component is always mounted with the map. -->
  <div
    v-if="activeTool && readout && readoutAt"
    class="measure-label wf-mono"
    :style="{ left: readoutAt.x + 'px', top: readoutAt.y + 'px' }"
  >{{ readout }}</div>

  <div v-if="activeTool" class="measure-hint wf-mono">{{ hint }}</div>
</template>

<script setup>
import { onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useToolsStore } from '@/stores/tools.js'

const store = useToolsStore()
const { activeTool, hint, readout, readoutAt } = storeToRefs(store)

// Keyboard shortcuts: R/D/Q select, Esc ends. Ignored while typing in a field.
function onKey(e) {
  const tag = e.target?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA') return
  const k = e.key.toLowerCase()
  if (k === 'escape') store.clearTool()
  else if (k === 'r') store.selectTool('rbl')
  else if (k === 'd') store.selectTool('dist')
  else if (k === 'q') store.selectTool('qdm')
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<style scoped>
/* Floating readout pill at the measure line's midpoint (screen-anchored). */
.measure-label {
  position: absolute;
  /* sit just above the midpoint so the pill doesn't cover the line */
  transform: translate(-50%, calc(-100% - 8px));
  z-index: 10;
  font-size: 10.5px; /* design template ToolsOverlay readout: mono 10.5, no bold */
  color: var(--wf-primary);
  background: rgba(14, 22, 34, 0.9);
  backdrop-filter: blur(4px);
  border: 1px solid var(--wf-primary); /* cyan outline, distinct from track chrome */
  border-radius: var(--wf-radius-sm);
  padding: 2px 8px;
  white-space: nowrap;
  pointer-events: none;
}

/* Bottom-centre instruction while a tool is active. */
.measure-hint {
  position: absolute;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
  font-size: 11px;
  color: var(--wf-primary);
  background: rgba(14, 22, 34, 0.85);
  backdrop-filter: blur(4px);
  border: var(--wf-chrome-border);
  border-radius: var(--wf-radius-sm);
  padding: 4px 10px;
  pointer-events: none;
}
</style>
