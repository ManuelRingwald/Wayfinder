<template>
  <div class="vp-control">
    <v-btn size="small" variant="tonal" prepend-icon="mdi-bookmark-multiple-outline" class="vp-btn">
      <span class="vp-btn__label">{{ activeName }}</span>
      <v-menu activator="parent" :close-on-content-click="false" location="bottom end">
        <v-card min-width="264" class="vp-menu" elevation="8">
          <v-list density="compact" class="pa-0">
            <v-list-subheader>Ansichts-Profile</v-list-subheader>
            <template v-if="store.list.length">
              <v-list-item
                v-for="p in store.list"
                :key="p.id"
                :active="p.id === store.activeId"
                @click="store.apply(p.id)"
              >
                <template #prepend>
                  <v-icon
                    :icon="p.is_default ? 'mdi-star' : 'mdi-star-outline'"
                    :color="p.is_default ? 'warning' : undefined"
                    size="small"
                    :aria-label="p.is_default ? 'Standard-Profil' : 'Als Standard setzen'"
                    @click.stop="store.setDefault(p.id)"
                  />
                </template>
                <v-list-item-title>{{ p.name }}</v-list-item-title>
                <template #append>
                  <v-btn icon="mdi-pencil" size="x-small" variant="text" aria-label="Umbenennen" @click.stop="startRename(p)" />
                  <v-btn icon="mdi-delete-outline" size="x-small" variant="text" aria-label="Löschen" @click.stop="store.remove(p.id)" />
                </template>
              </v-list-item>
            </template>
            <v-list-item v-else>
              <v-list-item-subtitle>Noch keine Profile gespeichert.</v-list-item-subtitle>
            </v-list-item>
            <v-divider />
            <v-list-item
              :disabled="!store.canCreate"
              prepend-icon="mdi-content-save-outline"
              @click="openSave"
            >
              <v-list-item-title>Aktuelle Ansicht speichern…</v-list-item-title>
              <v-list-item-subtitle v-if="!store.canCreate">Maximal 3 Profile</v-list-item-subtitle>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-btn>

    <!-- Save-new / rename dialog -->
    <v-dialog v-model="dialog" max-width="420">
      <v-card>
        <v-card-title class="text-subtitle-1">{{ renameId ? 'Profil umbenennen' : 'Aktuelle Ansicht speichern' }}</v-card-title>
        <v-card-text>
          <v-text-field
            v-model="name"
            label="Name"
            autofocus
            :counter="60"
            :error-messages="nameError"
            @keyup.enter="submit"
          />
          <v-checkbox
            v-if="!renameId"
            v-model="makeDefault"
            label="Als Standard beim Login aktivieren"
            density="compact"
            hide-details
          />
          <v-alert v-if="store.error" type="error" density="compact" variant="tonal" class="mt-2">
            {{ store.error }}
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">Abbrechen</v-btn>
          <v-btn color="primary" :loading="busy" :disabled="!canSubmit" @click="submit">Speichern</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useProfilesStore } from '@/stores/profiles.js'

const store = useProfilesStore()
const dialog = ref(false)
const name = ref('')
const makeDefault = ref(false)
const renameId = ref(null) // null = save-new, else the id being renamed
const busy = ref(false)

// The button label shows the currently applied profile, or a neutral fallback.
const activeName = computed(() => store.list.find((p) => p.id === store.activeId)?.name ?? 'Ansicht')
const nameError = computed(() => (name.value.length > 60 ? 'Maximal 60 Zeichen' : ''))
const canSubmit = computed(() => name.value.trim().length > 0 && name.value.length <= 60)

onMounted(() => {
  store.load()
})

function openSave() {
  renameId.value = null
  name.value = ''
  makeDefault.value = false
  store.error = ''
  dialog.value = true
}

function startRename(p) {
  renameId.value = p.id
  name.value = p.name
  store.error = ''
  dialog.value = true
}

async function submit() {
  const n = name.value.trim()
  if (!canSubmit.value) return
  busy.value = true
  const ok = renameId.value
    ? await store.rename(renameId.value, n)
    : await store.saveCurrent(n, makeDefault.value)
  busy.value = false
  if (ok) dialog.value = false
}
</script>

<style scoped>
.vp-control {
  pointer-events: auto;
}
.vp-btn__label {
  max-width: 12ch;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.vp-menu {
  background: rgba(14, 22, 34, 0.98);
}
</style>
