<template>
  <!-- Platform-wide OpenAIP configuration (AERO-2, ADR 0018). The global fallback
       key is a secret: the server reports only whether one is stored and whether
       encryption is available, never the key. Setting it seals the value at rest
       and triggers a fetch-all so every tenant picks up the new fallback. -->
  <v-card variant="tonal" class="mb-4">
    <v-card-title class="text-subtitle-1">OpenAIP — globaler Schlüssel</v-card-title>
    <v-card-text>
      <p class="text-body-2 text-medium-emphasis mb-3">
        Der globale OpenAIP-Schlüssel ist der Rückfall für alle Mandanten
        <strong>ohne</strong> eigenen Schlüssel. Er wird verschlüsselt gespeichert
        (nie wieder angezeigt); das Setzen löst einen Abruf für alle Mandanten aus.
      </p>

      <v-alert
        v-if="!encryptionAvailable"
        type="warning"
        variant="tonal"
        density="compact"
        class="mb-3"
      >
        Verschlüsselung nicht verfügbar: Setze <code>WAYFINDER_SECRET_KEY</code>
        (base64-kodierte 32 Bytes), um einen globalen Schlüssel über die UI zu
        hinterlegen. Ohne ihn bleibt nur die Umgebungsvariable
        <code>WAYFINDER_OPENAIP_API_KEY</code>.
      </v-alert>

      <div class="d-flex align-center ga-2 mb-3">
        <span>Status:</span>
        <v-chip :color="configured ? 'success' : 'default'" size="small" variant="tonal">
          {{ configured ? 'gesetzt (verschlüsselt)' : 'nicht gesetzt' }}
        </v-chip>
      </div>

      <div class="d-flex flex-wrap ga-3 align-center">
        <v-text-field
          v-model="apiKey"
          label="Globaler OpenAIP-API-Schlüssel"
          placeholder="Neuen Schlüssel eingeben…"
          variant="outlined"
          density="compact"
          hide-details
          autocomplete="off"
          :disabled="!encryptionAvailable"
          :type="showKey ? 'text' : 'password'"
          :append-inner-icon="showKey ? 'mdi-eye-off' : 'mdi-eye'"
          style="max-width: 420px"
          @click:append-inner="showKey = !showKey"
        />
        <v-btn
          color="primary"
          :loading="busy"
          :disabled="!encryptionAvailable || !apiKey"
          @click="save"
        >
          Schlüssel speichern
        </v-btn>
        <v-btn v-if="configured" color="error" variant="tonal" :loading="busy" @click="clear">
          Schlüssel entfernen
        </v-btn>
      </div>

      <v-divider class="my-4" />

      <div class="d-flex align-center ga-3 flex-wrap">
        <v-btn
          color="primary"
          variant="tonal"
          prepend-icon="mdi-refresh"
          :loading="busy"
          @click="refreshAll"
        >
          Alle Mandanten aktualisieren
        </v-btn>
        <span class="text-caption text-medium-emphasis">
          Zieht die Aeronautik-Daten (Luftraum/Navaids/Wegpunkte) für alle Mandanten
          neu — z. B. zu einem AIRAC-Stichtag.
        </span>
      </div>
    </v-card-text>
  </v-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'

const admin = useAdminStore()

const configured = ref(false)
const encryptionAvailable = ref(false)
const apiKey = ref('')
const showKey = ref(false)
const busy = ref(false)

async function load() {
  const r = await admin.loadGlobalOpenAIP()
  if (r.ok && r.data) {
    configured.value = !!r.data.configured
    encryptionAvailable.value = !!r.data.encryption_available
  }
}

async function save() {
  if (!apiKey.value) return
  busy.value = true
  const r = await admin.setGlobalOpenAIPKey(apiKey.value)
  busy.value = false
  if (r.ok) {
    apiKey.value = ''
    showKey.value = false
    await load()
  }
}

async function clear() {
  busy.value = true
  const r = await admin.setGlobalOpenAIPKey(null)
  busy.value = false
  if (r.ok) await load()
}

async function refreshAll() {
  busy.value = true
  await admin.refreshAllOpenAIP()
  busy.value = false
}

onMounted(load)
</script>
