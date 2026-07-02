<template>
  <!-- Häppchen 4: controller measurement tools. A compact vertical tool group on
       the left edge (RBL/DIST/QDM), plus a bottom-centre hint + live readout
       while a tool is active. Icons carry meaning; the label spells the tool. -->
  <div class="measure-toolbar">
    <button
      v-for="t in TOOLS"
      :key="t.id"
      type="button"
      class="measure-btn"
      :class="{ 'measure-btn--active': activeTool === t.id }"
      :aria-pressed="activeTool === t.id"
      @click="store.selectTool(t.id)"
    >
      <v-icon size="20">{{ t.icon }}</v-icon>
      <span class="measure-btn__label">{{ t.id.toUpperCase() }}</span>
      <v-tooltip activator="parent" location="right" :text="t.label" />
    </button>
  </div>

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

const TOOLS = [
  { id: 'rbl', icon: 'mdi-vector-line', label: 'Range/Bearing-Line (R)' },
  { id: 'dist', icon: 'mdi-ruler', label: 'Distanz Track ↔ Track (D)' },
  { id: 'qdm', icon: 'mdi-compass-outline', label: 'Peilung QDM (Q)' },
]

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
.measure-toolbar {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 10;
  display: flex;
  flex-direction: column;
  background: rgb(var(--v-theme-surface));
  border: var(--wf-chrome-border);
  border-radius: var(--wf-radius-sm);
  box-shadow: var(--wf-elevation-4);
  overflow: hidden;
}
.measure-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  width: 48px;
  padding: 8px 0;
  background: transparent;
  border: 0;
  cursor: pointer;
  color: rgba(var(--v-theme-on-surface), 0.6);
  transition: color 0.15s, background 0.15s;
}
.measure-btn:hover {
  color: rgba(var(--v-theme-on-surface), 0.9);
  background: var(--wf-state-hover);
}
.measure-btn--active {
  color: rgb(var(--v-theme-primary));
  background: var(--wf-state-selected);
}
.measure-btn__label {
  font-size: 9px;
  font-weight: 600;
  letter-spacing: 0.04em;
  line-height: 1;
}
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
