<template>
  <!-- #319: account self-service for the ASD operator (a plain tenant user).
       Opened from the sidebar "Konto" section. It talks to the role-agnostic
       /api/account/* endpoints through the session store, so a lotse can set
       their own email + password without an admin — the admin-gated
       /api/admin/me/* was reachable only by admins. Account DELETION stays in
       the admin dashboard on purpose (a heavier, admin-side action). -->
  <v-dialog
    :model-value="modelValue"
    max-width="min(460px, 94vw)"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <v-card>
      <v-card-title class="pa-4 pb-2">Mein Konto</v-card-title>
      <v-card-subtitle class="px-4 pb-0">{{ session.subject }}</v-card-subtitle>

      <v-divider class="mt-3" />

      <!-- Email -->
      <v-card-text class="pb-0">
        <div class="text-subtitle-2 mb-1">E-Mail-Adresse</div>
        <div class="text-caption text-medium-emphasis mb-3">
          Aktuell: {{ session.email || '— keine hinterlegt —' }}
        </div>
        <v-alert
          v-if="emailError"
          type="error"
          variant="tonal"
          density="compact"
          class="mb-3"
          closable
          @click:close="emailError = null"
        >{{ emailError }}</v-alert>
        <v-alert
          v-if="emailSuccess"
          type="success"
          variant="tonal"
          density="compact"
          class="mb-3"
          closable
          @click:close="emailSuccess = null"
        >{{ emailSuccess }}</v-alert>
        <v-form @submit.prevent="submitEmail">
          <v-text-field
            v-model="emailNew"
            label="Neue E-Mail-Adresse"
            type="email"
            autocomplete="email"
            variant="outlined"
            density="compact"
            class="mb-3"
            hide-details="auto"
          />
          <v-btn
            type="submit"
            color="primary"
            variant="tonal"
            :loading="emailLoading"
            :disabled="!emailNew"
          >E-Mail speichern</v-btn>
        </v-form>
      </v-card-text>

      <v-divider class="mt-4 mb-2" />

      <!-- Password -->
      <v-card-text class="pb-4">
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
        <v-form @submit.prevent="submitPassword">
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

      <v-card-actions class="px-4 pb-4 pt-0">
        <v-spacer />
        <v-btn variant="text" @click="$emit('update:modelValue', false)">Schließen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref } from 'vue'
import { useSessionStore } from '@/stores/session.js'

defineProps({ modelValue: Boolean })
defineEmits(['update:modelValue'])

const session = useSessionStore()

// --- email change -------------------------------------------------------------
// Basic client-side shape check; the server validates authoritatively with the
// same conservative pattern (min-length/format live server-side, ADR 0033).
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const emailNew = ref('')
const emailLoading = ref(false)
const emailError = ref(null)
const emailSuccess = ref(null)

async function submitEmail() {
  emailError.value = null
  emailSuccess.value = null
  const e = emailNew.value.trim()
  if (!EMAIL_RE.test(e)) {
    emailError.value = 'Bitte eine gültige E-Mail-Adresse eingeben.'
    return
  }
  emailLoading.value = true
  const r = await session.changeOwnEmail(e)
  emailLoading.value = false
  if (r.ok) {
    emailNew.value = ''
    emailSuccess.value = 'E-Mail-Adresse aktualisiert.'
  } else {
    emailError.value = r.error || 'Änderung fehlgeschlagen.'
  }
}

// --- password change ----------------------------------------------------------
// Min length 8 mirrors the server-side minPasswordLen (the same standard the
// admin applies when creating/resetting a user); the server is authoritative.
const pwCurrent = ref('')
const pwNew = ref('')
const pwConfirm = ref('')
const pwLoading = ref(false)
const pwError = ref(null)
const pwSuccess = ref(null)

async function submitPassword() {
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
  const r = await session.changeOwnPassword(pwCurrent.value, pwNew.value)
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
</script>
