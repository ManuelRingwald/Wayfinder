<template>
  <!-- Reusable login card: a username/password form that emits `submit` with the
       entered credentials. The parent owns the actual auth call (and the loading/
       error state), so this component stays presentational and testable. Used by
       the ASD map's auth gate (AsdView). -->
  <v-container class="d-flex justify-center align-center" style="min-height: 100vh">
    <v-card max-width="420" width="100%" variant="outlined">
      <v-card-title class="pa-4 pb-0">{{ title }}</v-card-title>
      <v-card-text>
        <v-alert
          v-if="error"
          type="error"
          variant="tonal"
          density="compact"
          class="mb-4"
        >{{ error }}</v-alert>
        <v-form @submit.prevent="submit">
          <v-text-field
            v-model="subject"
            label="Benutzername"
            autocomplete="username"
            autofocus
            class="mb-2"
          />
          <v-text-field
            v-model="password"
            label="Passwort"
            :type="showPassword ? 'text' : 'password'"
            autocomplete="current-password"
            :append-inner-icon="showPassword ? 'mdi-eye-off' : 'mdi-eye'"
            class="mb-4"
            @click:append-inner="showPassword = !showPassword"
          />
          <v-btn
            type="submit"
            color="primary"
            :loading="loading"
            :disabled="!subject || !password"
            block
          >Anmelden</v-btn>
        </v-form>
      </v-card-text>
    </v-card>
  </v-container>
</template>

<script setup>
import { ref } from 'vue'

defineProps({
  title: { type: String, default: 'Anmelden' },
  error: { type: String, default: null },
  loading: { type: Boolean, default: false },
})
const emit = defineEmits(['submit'])

const subject = ref('')
const password = ref('')
const showPassword = ref(false)

function submit() {
  if (!subject.value || !password.value) return
  emit('submit', { subject: subject.value, password: password.value })
}
</script>
