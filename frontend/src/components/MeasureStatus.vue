<template>
  <!-- Häppchen 3: the measurement tools (RBL/DIST/QDM) moved into the navigation
       rail (design mockup), replacing the old floating toolbar. What stays over
       the map is the live measuring status: a bottom-centre instruction + readout
       shown while a tool is active. The keyboard shortcuts (R/D/Q select, Esc
       ends) live here because this component is always mounted with the map. -->
  <div v-if="activeTool" class="measure-hint wf-mono">
    <span class="measure-hint__text">{{ hint }}</span>
    <span v-if="readout" class="measure-hint__readout">{{ readout }}</span>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useToolsStore } from '@/stores/tools.js'

const store = useToolsStore()
const { activeTool, hint, readout } = storeToRefs(store)

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
.measure-hint {
  position: absolute;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 11px;
  color: var(--wf-primary);
  background: rgba(14, 22, 34, 0.85);
  backdrop-filter: blur(4px);
  border: var(--wf-chrome-border);
  border-radius: var(--wf-radius-sm);
  padding: 4px 10px;
  pointer-events: none;
}
.measure-hint__readout {
  color: var(--wf-on-surface);
  font-weight: 700;
}
</style>
