<template>
  <!-- AP3 (ADR 0009): tenant-centric admin overview. One row per tenant with its
       status, enabled features, subscribed feeds and account count — the landing
       page of the redesigned admin area. A row's "Konfigurieren" opens the detail
       page (emitted to the parent). The server enforces every boundary
       (requireAdmin → 403); this view is convenience, not a security control. -->
  <v-card variant="tonal">
    <v-card-title class="d-flex align-center text-subtitle-1">
      Mandanten
      <v-spacer />
      <v-btn size="small" variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="refresh">
        Aktualisieren
      </v-btn>
      <!-- ONB-4 (ADR 0011): create a tenant from the UI. -->
      <v-btn size="small" color="primary" variant="tonal" prepend-icon="mdi-domain-plus" class="ml-2" @click="openCreate">
        Mandant anlegen
      </v-btn>
    </v-card-title>
    <v-card-text>
      <v-table density="comfortable">
        <thead>
          <tr>
            <th>Mandant</th>
            <th class="text-right">Status</th>
            <th>Features</th>
            <th>Feeds</th>
            <th class="text-right">Zugänge</th>
            <th class="text-right">Aktion</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!admin.overview.length">
            <td colspan="6" class="text-medium-emphasis">Keine Mandanten.</td>
          </tr>
          <tr v-for="t in admin.overview" :key="t.id">
            <td>
              <div>{{ t.name }}</div>
              <div class="text-caption text-medium-emphasis">{{ t.slug }}</div>
            </td>
            <td class="text-right">
              <v-chip :color="t.status === 'paused' ? 'warning' : 'success'" size="small" variant="tonal">
                {{ t.status === 'paused' ? 'pausiert' : 'aktiv' }}
              </v-chip>
            </td>
            <td>
              <span v-if="!t.features.length" class="text-medium-emphasis">—</span>
              <v-chip
                v-for="key in t.features"
                :key="key"
                size="x-small"
                variant="tonal"
                color="primary"
                class="mr-1 mb-1"
              >
                {{ key }}
              </v-chip>
            </td>
            <td>
              <span v-if="!t.feeds.length" class="text-medium-emphasis">—</span>
              <span v-for="f in t.feeds" :key="f.id" class="d-inline-flex align-center mr-1 mb-1">
                <v-chip
                  size="x-small"
                  variant="tonal"
                  :color="feedColor(f.id)"
                  :title="feedTitle(f.id)"
                >
                  {{ f.name }}
                </v-chip>
              </span>
            </td>
            <td class="text-right">{{ t.user_count }}</td>
            <td class="text-right">
              <v-btn size="small" color="primary" variant="text" @click="$emit('select', t.id)">
                Konfigurieren
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>

  <!-- Create tenant dialog (ONB-4) -->
  <v-dialog v-model="createDialog" max-width="min(460px, 94vw)">
    <v-card>
      <v-card-title class="text-subtitle-1">Mandant anlegen</v-card-title>
      <v-card-text>
        <v-text-field
          v-model="form.name"
          label="Name"
          hint="Anzeigename des Kunden, z. B. „EDLV Weeze“."
          persistent-hint
          autofocus
          class="mb-2"
        />
        <!-- Issue #105: the slug is a technical identifier (URLs/API references),
             derived automatically from the name and shown read-only — never a
             manual field the admin has to fill in. -->
        <div class="text-caption text-medium-emphasis">
          Kennung (automatisch):
          <code v-if="derivedSlug">{{ derivedSlug }}</code>
          <span v-else class="text-warning">Bitte einen Namen mit Buchstaben oder Ziffern wählen.</span>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="createDialog = false">Abbrechen</v-btn>
        <v-btn color="primary" :loading="loading" :disabled="!derivedSlug" @click="submitCreate">Anlegen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin.js'
import { describeFeedHealth } from '@/admin/feedHealth.js'

defineEmits(['select'])

const admin = useAdminStore()
const loading = ref(false)

// ONB-4: create-tenant dialog state. Issue #105: the admin only enters a name; the
// slug is derived from it (see slugify) rather than typed by hand.
const createDialog = ref(false)
const form = ref({ name: '' })

// slugify turns a display name into a DNS-label-like slug matching the server's
// slugPattern (lowercase a–z0–9 and inner hyphens): transliterate the common
// German umlauts, lowercase, replace every other run of non-alphanumerics with a
// single hyphen, trim leading/trailing hyphens, and cap at the 63-char limit.
function slugify(name) {
  const map = { ä: 'ae', ö: 'oe', ü: 'ue', ß: 'ss' }
  return (name || '')
    .toLowerCase()
    .replace(/[äöüß]/g, (c) => map[c])
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 63)
    .replace(/-+$/g, '')
}

const derivedSlug = computed(() => slugify(form.value.name))

function openCreate() {
  form.value = { name: '' }
  createDialog.value = true
}

async function submitCreate() {
  const slug = derivedSlug.value
  if (!slug) return
  loading.value = true
  const r = await admin.createTenant({ slug, name: form.value.name.trim() })
  loading.value = false
  if (r.ok) {
    createDialog.value = false
    await refresh()
  }
}

// Feed-health chip colour/title from the shared helper (AP4 + status
// granularity): red splits into "nie gestartet" vs "abgerissen".
function feedColor(feedId) {
  return describeFeedHealth(admin.feedsHealth[feedId]).color
}

function feedTitle(feedId) {
  return describeFeedHealth(admin.feedsHealth[feedId]).title
}

async function refresh() {
  loading.value = true
  await Promise.all([admin.loadOverview(), admin.loadFeedsHealth()])
  loading.value = false
}

onMounted(refresh)
</script>
