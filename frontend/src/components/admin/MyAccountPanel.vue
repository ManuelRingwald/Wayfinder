<template>
  <!-- ONB-2 (ADR 0011): self-management panel. Password change and account
       deletion are available at any time from the dashboard header — not only
       during the forced-change flow. Backend endpoints are from ONB-1. -->
  <v-dialog :model-value="modelValue" max-width="min(480px, 94vw)" @update:model-value="$emit('update:modelValue', $event)">
    <v-card>
      <v-card-title class="pa-4 pb-2">Mein Konto</v-card-title>
      <v-card-subtitle class="px-4 pb-0">{{ identity?.subject }}</v-card-subtitle>

      <v-divider class="mt-3" />

      <!-- Password change section -->
      <v-card-text class="pb-0">
        <div class="text-subtitle-2 mb-3">Passwort ändern</div>
        <v-alert
          v-if="pwError"
          type="error"
          variant="tonal"
          density="compact"
          class="mb-3"
          closable
          @click:close="pwError = null"
        >{{ pwError }}</v-alert>
        <v-alert
          v-if="pwSuccess"
          type="success"
          variant="tonal"
          density="compact"
          class="mb-3"
          closable
          @click:close="pwSuccess = null"
        >{{ pwSuccess }}</v-alert>
        <v-form @submit.prevent="submitPasswordChange">
          <v-text-field
            v-model="pwCurrent"
            label="Aktuelles Passwort"
            type="password"
            autocomplete="current-password"
            variant="outlined"
            density="compact"
            class="mb-2"
            hide-details="auto"
          />
          <v-text-field
            v-model="pwNew"
            label="Neues Passwort (min. 8 Zeichen)"
            type="password"
            autocomplete="new-password"
            variant="outlined"
            density="compact"
            class="mb-2"
            hide-details="auto"
          />
          <v-text-field
            v-model="pwConfirm"
            label="Neues Passwort wiederholen"
            type="password"
            autocomplete="new-password"
            variant="outlined"
            density="compact"
            class="mb-3"
            hide-details="auto"
          />
          <v-btn
            type="submit"
            color="primary"
            variant="tonal"
            :loading="pwLoading"
            :disabled="!pwCurrent || !pwNew || !pwConfirm"
          >Passwort ändern</v-btn>
        </v-form>
      </v-card-text>

      <v-divider class="mt-4 mb-2" />

      <!-- Account deletion section -->
      <v-card-text class="pb-4">
        <div class="text-subtitle-2 mb-2">Konto löschen</div>
        <p class="text-body-2 text-medium-emphasis mb-3">
          Das Konto wird dauerhaft gelöscht. Als letzter aktiver Administrator
          ist das nicht möglich.
        </p>
        <template v-if="!confirmDelete">
          <v-btn
            color="error"
            variant="tonal"
            @click="confirmDelete = true"
          >Konto löschen …</v-btn>
        </template>
        <template v-else>
          <v-alert type="warning" variant="tonal" density="compact" class="mb-3">
            Wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </v-alert>
          <div class="d-flex ga-2">
            <v-btn
              color="error"
              :loading="deleteLoading"
              @click="submitDeleteAccount"
            >Ja, löschen</v-btn>
            <v-btn variant="text" @click="confirmDelete = false">Abbrechen</v-btn>
          </div>
        </template>
        <v-alert
          v-if="deleteError"
          type="error"
          variant="tonal"
          density="compact"
          class="mt-3"
          closable
          @click:close="deleteError = null"
        >{{ deleteError }}</v-alert>
      </v-card-text>

      <v-card-actions class="px-4 pb-4 pt-0">
        <v-spacer />
        <v-btn variant="text" @click="$emit('update:modelValue', false)">Schließen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

defineProps({ modelValue: Boolean })
defineEmits(['update:modelValue'])

const admin = useAdminStore()
const identity = computed(() => admin.identity)

// --- password change ----------------------------------------------------------
const pwCurrent = ref('')
const pwNew = ref('')
const pwConfirm = ref('')
const pwLoading = ref(false)
const pwError = ref(null)
const pwSuccess = ref(null)

async function submitPasswordChange() {
  pwError.value = null
  pwSuccess.value = null
  if (pwNew.value.length < 8) {
    pwError.value = 'Das neue Passwort muss mindestens 8 Zeichen lang sein.'
    return
  }
  if (pwNew.value !== pwConfirm.value) {
    pwError.value = 'Die Passwörter stimmen nicht überein.'
    return
  }
  pwLoading.value = true
  const r = await admin.changeOwnPassword(pwCurrent.value, pwNew.value)
  pwLoading.value = false
  if (r.ok) {
    pwCurrent.value = pwNew.value = pwConfirm.value = ''
    pwSuccess.value = 'Passwort erfolgreich geändert.'
  } else {
    pwError.value = r.status === 401
      ? 'Das aktuelle Passwort ist falsch.'
      : (r.error || 'Passwortänderung fehlgeschlagen.')
  }
}

// --- account deletion ---------------------------------------------------------
const confirmDelete = ref(false)
const deleteLoading = ref(false)
const deleteError = ref(null)

async function submitDeleteAccount() {
  deleteError.value = null
  deleteLoading.value = true
  const r = await admin.deleteOwnAccount()
  deleteLoading.value = false
  if (!r.ok) {
    confirmDelete.value = false
    deleteError.value = r.status === 409
      ? 'Löschen nicht möglich: Sie sind der letzte aktive Administrator.'
      : (r.error || 'Konto konnte nicht gelöscht werden.')
  }
  // on success: store cleared identity → AdminView re-renders to login form automatically
}
</script>
