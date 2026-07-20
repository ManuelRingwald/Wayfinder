<template>
  <!-- ASD-020 (sidebar information architecture, ADR 0031): ONE collapsible
       group in the Layer section. It is the binding "Rahmen" for layer chrome —
       a titled, collapsible block with a tri-state master control in its header.
       New layers join a group as a row in its slot; they never float as loose
       switches in a flat list. The master toggles the whole group at once
       (select-all/none); its indeterminate state shows a partial group. -->
  <div class="layer-group">
    <div
      class="layer-group__header"
      role="button"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <v-icon size="18" class="layer-group__chevron">
        {{ expanded ? 'mdi-chevron-down' : 'mdi-chevron-right' }}
      </v-icon>
      <span class="layer-group__title">{{ title }}</span>
      <v-spacer />
      <!-- The master is a CONTROLLED tri-state checkbox: its visual comes purely
           from `master` (on/off/indeterminate); the click is handled by the
           parent (toggle-master), so Vuetify's own toggle never fights the
           derived state. Hidden when the group has nothing controllable. -->
      <v-checkbox-btn
        v-if="master !== 'empty'"
        :model-value="master === 'on'"
        :indeterminate="master === 'mixed'"
        color="primary"
        density="compact"
        hide-details
        :aria-label="`${title} — alle ein/aus`"
        class="layer-group__master"
        @click.stop="$emit('toggle-master')"
      />
    </div>
    <v-expand-transition>
      <div v-show="expanded" class="layer-group__body">
        <slot />
      </div>
    </v-expand-transition>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const props = defineProps({
  title: { type: String, required: true },
  // 'on' | 'off' | 'mixed' | 'empty' — from map/layerGroups.js masterState().
  master: { type: String, default: 'empty' },
  // Groups start expanded so E1 hides nothing that was visible before; the
  // operator can collapse a group they do not need.
  defaultExpanded: { type: Boolean, default: true },
})

defineEmits(['toggle-master'])

const expanded = ref(props.defaultExpanded)
</script>

<style scoped>
.layer-group {
  margin: 2px 0;
}

/* Group header: the collapse affordance + title + master. Indented to sit under
   the section header ("LAYER"), a touch quieter than it but louder than a row
   label, so the three levels (section > group > row) read as a clear hierarchy. */
.layer-group__header {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px 4px 8px;
  min-height: 34px;
  cursor: pointer;
  user-select: none;
}
.layer-group__header:hover {
  background: var(--wf-state-hover);
}
.layer-group__chevron {
  opacity: 0.6;
  flex-shrink: 0;
}
.layer-group__title {
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.01em;
  color: rgba(var(--v-theme-on-surface), 0.92);
}
/* The tri-state master, compact so it does not dominate the header. */
.layer-group__master {
  flex: none;
}

/* Body: the group's rows, indented one step so they read as children of the
   group header. The rows themselves keep their existing .filter-row styling. */
.layer-group__body {
  padding-left: 6px;
}
</style>
