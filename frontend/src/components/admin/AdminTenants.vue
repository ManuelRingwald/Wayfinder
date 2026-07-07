<template>
  <!-- AP3 (ADR 0009): tenant-centric admin overview. One row per tenant with its
       status and enabled features. Since #210 the operational configuration that
       used to crowd the detail page — Feeds, OpenAIP and access accounts — lives
       here as its own column, each opened as a focused dialog via a config icon;
       the detail page ("Konfigurieren") is reduced to the default view + features.
       The server enforces every boundary (requireAdmin → 403); this view is
       convenience, not a security control. -->
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
            <th class="text-center">Feeds</th>
            <th class="text-center">OpenAIP</th>
            <th class="text-center">Nutzer</th>
            <th class="text-right">Aktion</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!admin.overview.length">
            <td colspan="7" class="text-medium-emphasis">Keine Mandanten.</td>
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
            <!-- #210: Feeds — a glanceable chip set plus a config icon opening the
                 per-tenant feed-assignment dialog. -->
            <td class="text-center">
              <div class="d-flex align-center justify-center flex-wrap ga-1">
                <span v-if="!t.feeds.length" class="text-medium-emphasis">—</span>
                <v-chip
                  v-for="f in t.feeds"
                  :key="f.id"
                  size="x-small"
                  variant="tonal"
                  :color="feedColor(f.id)"
                  :title="feedTitle(f.id)"
                >
                  {{ f.name }}
                </v-chip>
                <v-btn
                  icon="mdi-cog-outline"
                  size="x-small"
                  variant="text"
                  color="primary"
                  :title="`Feeds konfigurieren — ${t.name}`"
                  :aria-label="`Feeds konfigurieren — ${t.name}`"
                  @click="openFeeds(t)"
                />
              </div>
            </td>
            <!-- #210: OpenAIP — a config icon opening the per-tenant OpenAIP dialog
                 (the dialog loads the key status / cache freshness on open). -->
            <td class="text-center">
              <v-btn
                icon="mdi-cog-outline"
                size="x-small"
                variant="text"
                color="primary"
                :title="`OpenAIP konfigurieren — ${t.name}`"
                :aria-label="`OpenAIP konfigurieren — ${t.name}`"
                @click="openOpenAIP(t)"
              />
            </td>
            <!-- #210: Nutzer — the account count plus a config icon opening the
                 per-tenant access-accounts dialog. -->
            <td class="text-center">
              <div class="d-flex align-center justify-center ga-1">
                <span>{{ t.user_count }}</span>
                <v-btn
                  icon="mdi-cog-outline"
                  size="x-small"
                  variant="text"
                  color="primary"
                  :title="`Zugänge konfigurieren — ${t.name}`"
                  :aria-label="`Zugänge konfigurieren — ${t.name}`"
                  @click="openUsers(t)"
                />
              </div>
            </td>
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

  <!-- #210: Feeds dialog — hosts the cross-tenant provisioning table for the
       selected tenant. A grant/revoke refreshes the overview chips + feed health. -->
  <v-dialog v-model="feedsDialog" max-width="min(720px, 94vw)">
    <v-card>
      <v-card-title class="text-subtitle-1">Feeds — {{ dialogTenantName }}</v-card-title>
      <v-card-text>
        <AdminProvisioning v-if="dialogTenant !== null" :tenant-id="dialogTenant" @changed="onFeedsChanged" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="feedsDialog = false">Schließen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- #210: OpenAIP dialog — per-tenant OpenAIP key + cache controls. -->
  <v-dialog v-model="openaipDialog" max-width="min(640px, 94vw)">
    <v-card>
      <v-card-title class="text-subtitle-1">OpenAIP — {{ dialogTenantName }}</v-card-title>
      <v-card-text>
        <AdminTenantOpenAIP v-if="dialogTenant !== null" :tenant-id="dialogTenant" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="openaipDialog = false">Schließen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- #210: Nutzer dialog — per-tenant access accounts. Creating/suspending a user
       changes the account count, so refresh the overview on close. -->
  <v-dialog v-model="usersDialog" max-width="min(880px, 94vw)" @update:model-value="onUsersDialogToggle">
    <v-card>
      <v-card-title class="text-subtitle-1">Zugänge — {{ dialogTenantName }}</v-card-title>
      <v-card-text>
        <AdminUsers v-if="dialogTenant !== null" :tenant-id="dialogTenant" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="usersDialog = false">Schließen</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

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
import AdminProvisioning from '@/components/admin/AdminProvisioning.vue'
import AdminTenantOpenAIP from '@/components/admin/AdminTenantOpenAIP.vue'
import AdminUsers from '@/components/admin/AdminUsers.vue'

defineEmits(['select'])

const admin = useAdminStore()
const loading = ref(false)

// ONB-4: create-tenant dialog state. Issue #105: the admin only enters a name; the
// slug is derived from it (see slugify) rather than typed by hand.
const createDialog = ref(false)
const form = ref({ name: '' })

// #210: per-column config dialogs. One shared target tenant drives all three; the
// hosted components self-reload on the tenant-id change (Vuetify dialogs render
// lazily, so the component mounts on first open and reacts to the prop thereafter).
const dialogTenant = ref(null)
const feedsDialog = ref(false)
const openaipDialog = ref(false)
const usersDialog = ref(false)
const dialogTenantName = computed(() => {
  const t = admin.overview.find((x) => x.id === dialogTenant.value)
  return t ? t.name : ''
})

function openFeeds(t) {
  dialogTenant.value = t.id
  feedsDialog.value = true
}
function openOpenAIP(t) {
  dialogTenant.value = t.id
  openaipDialog.value = true
}
function openUsers(t) {
  dialogTenant.value = t.id
  usersDialog.value = true
}

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

// onFeedsChanged reacts to a grant/revoke in the feeds dialog: the overview's feed
// chips derive from admin.overview and their colour/title from feed health, so both
// are reloaded to keep the row in sync with the new assignment.
async function onFeedsChanged() {
  await Promise.all([admin.loadOverview(), admin.loadFeedsHealth()])
}

// onUsersDialogToggle refreshes the overview when the users dialog closes, so the
// account count reflects any created/removed access accounts.
async function onUsersDialogToggle(open) {
  if (!open) await admin.loadOverview()
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
