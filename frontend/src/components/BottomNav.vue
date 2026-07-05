<template>
  <!-- #194: mobile bottom tab bar (design mockup "ASD Mobile"). Replaces the
       hamburger drawer on phones: Scope shows the live picture, Filter/Konto open
       bottom sheets, Admin routes to the admin panel (admins only). Fixed to the
       bottom, blurred over the scope, and padded past the home indicator via the
       safe-area token. Tabs are >= 44px tall for comfortable finger targets. -->
  <nav class="bottom-nav" role="tablist" aria-label="Hauptnavigation">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      class="bn-tab"
      :class="{ 'bn-tab--on': tab.id === modelValue }"
      role="tab"
      :aria-selected="tab.id === modelValue"
      :aria-label="tab.label"
      @click="onSelect(tab.id)"
    >
      <v-icon size="23">{{ tab.icon }}</v-icon>
      <span class="bn-lbl">{{ tab.label }}</span>
    </button>
  </nav>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  // Active tab id ('scope' | 'filter' | 'konto' | 'admin').
  modelValue: { type: String, default: 'scope' },
  // Show the Admin tab only when the caller knows the user is an admin.
  isAdmin: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'select'])

const tabs = computed(() => {
  const base = [
    { id: 'scope', icon: 'mdi-radar', label: 'Scope' },
    { id: 'filter', icon: 'mdi-tune-variant', label: 'Filter' },
    { id: 'konto', icon: 'mdi-account-circle', label: 'Konto' },
  ]
  if (props.isAdmin) base.push({ id: 'admin', icon: 'mdi-shield-account', label: 'Admin' })
  return base
})

function onSelect(id) {
  emit('update:modelValue', id)
  emit('select', id)
}
</script>

<style scoped>
.bottom-nav {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 1200;
  display: flex;
  /* Reserve the home-indicator inset below the tappable row. */
  padding: 6px 6px calc(6px + var(--wf-safe-bottom, 0px));
  background: rgba(var(--v-theme-surface), 0.82);
  backdrop-filter: blur(18px);
  border-top: var(--wf-chrome-border);
}

.bn-tab {
  flex: 1;
  min-height: var(--wf-touch-min, 44px);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 3px;
  padding: 6px 0;
  border: none;
  background: transparent;
  border-radius: 14px;
  cursor: pointer;
  color: rgba(var(--v-theme-on-surface), 0.6);
  transition: color 0.15s, background 0.15s;
}
.bn-tab:active {
  background: var(--wf-state-hover);
}
.bn-tab--on {
  color: rgb(var(--v-theme-primary));
}
.bn-lbl {
  font-size: 10.5px;
  font-weight: 500;
  letter-spacing: 0.01em;
  line-height: 1;
}

@media (prefers-reduced-motion: reduce) {
  .bn-tab {
    transition: none;
  }
}
</style>
