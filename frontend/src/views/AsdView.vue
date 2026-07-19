<template>
  <!-- ASD-013: the ASD is a full-screen scope, no title bar. This is the
       operational view at route '/'. It gates on the session (ADR 0014: auth is
       always on): until the identity probe resolves we show a spinner, then either
       the login screen (anon) or the live picture (authed). The map + WebSocket
       only mount once authenticated, so /ws never opens unauthenticated. While the
       picture is up, the session is slid forward (WF2-12.5) so an active console
       is never logged out; a real expiry surfaces as the login screen, not a
       silent frozen map. -->
  <!-- #208 (ADR 0022): the spinner also covers the post-auth admin gate — an
       authenticated principal stays here until adminGate resolves, so the map
       (and /ws) never mounts for an admin who is about to be redirected. -->
  <v-main
    v-if="session.status === 'loading' || (session.status === 'authed' && adminGate !== 'ok')"
    class="d-flex justify-center align-center"
    style="min-height: 100vh"
  >
    <v-progress-circular indeterminate color="primary" size="48" />
  </v-main>

  <LoginCard
    v-else-if="session.status === 'anon'"
    title="Wayfinder — Anmelden"
    :error="loginNotice"
    :loading="loginLoading"
    @submit="onLogin"
  />

  <template v-else>
    <!-- Desktop + tablet-landscape (>=960px): the MD3 navigation rail + panel.
         Phones and tablet-portrait use the bottom tab bar + sheets below (#194). -->
    <NavigationRail
      v-if="mdAndUp"
      v-model="drawerOpen"
      @layer-toggle="onLayerToggle"
      @fl-filter-change="onFlFilterChange"
      @panel-resize="onPanelResize"
    />

    <v-main style="padding: 0; position: relative">
      <MapCanvas
        ref="mapCanvas"
        @track-click="onTrackClick"
        @connection-change="onConnectionChange"
        @empty-click="onMapEmptyClick"
      />
      <!-- ASD-007 (design template 'nacht' scheme): a faint cyan radar-scope
           bloom at the display centre. Sits above the map tiles but below the
           chrome and the interactive map controls; 5% alpha, so it tints the
           picture imperceptibly. -->
      <div class="scope-glow-overlay" aria-hidden="true" />
      <!-- Top-right cluster: the ICAO/sector + live UTC header sits next to the
           feed-health badge (CAT065 heartbeat, bug #54). The account chip that
           used to be here was removed — account access is the sidebar's "Konto"
           only, to avoid duplication. -->
      <div class="top-right-cluster">
        <AsdHeader />
        <FeedStatusChip />
        <!-- The compact action chrome (view-profile switcher + event bell) shares
             ONE row so the cluster stays short and never runs into the map
             controls on its right edge (icons-overlap fix, operator 2026-07-08). -->
        <div class="cluster-actions">
          <!-- VP-4 (ADR 0023): per-user view-profile switcher (icon-only + tooltip). -->
          <ViewProfileMenu />
          <!-- ASD-013: event-log bell with an unseen badge; toggles the floating
               Alarm-/Ereignis-Panel and marks the log seen on open. -->
          <div class="events-control">
            <v-badge
              :model-value="events.unseenCount > 0"
              :content="badgeContent"
              color="error"
              offset-x="6"
              offset-y="6"
            >
              <v-btn
                :icon="eventsOpen ? 'mdi-bell' : 'mdi-bell-outline'"
                size="small"
                variant="tonal"
                :color="eventsOpen ? 'primary' : undefined"
                aria-label="Ereignisse"
                @click="toggleEvents"
              />
            </v-badge>
          </div>
        </div>
        <EventPanel
          v-if="eventsOpen"
          class="events-panel"
          @close="eventsOpen = false"
          @select-track="onEventSelectTrack"
        />
        <!-- #277 (ADR 0028): sector search over the tenant AOI's base-map data.
             Cosmetic gate mirroring the sidebar's showLayer pattern (fail-open
             for admins in guest mode); the server enforces the basemap
             entitlement on /api/basemap/search independently, fail-closed. -->
        <MapSearch
          v-if="showSearch"
          class="map-search-control"
          @select="onSearchSelect"
          @clear="mapCanvas?.clearSearchMarker()"
        />
        <!-- ASD-018 (overlay-zone layout, ADR 0029): the viewport controls
             (recenter/fullscreen) are the LAST flex child of this right-edge
             rail, so they flow BELOW everything above them — no matter how many
             rows the cluster grows to. This replaced MapControls' hard-coded
             `top:140px`, which guessed the cluster height and overlapped every
             time a new element (profile switch, event bell, search) was added.
             Mobile keeps its own bottom-right stack in MapControls. -->
        <ViewportControls
          v-if="mdAndUp"
          class="viewport-controls-slot"
          @recenter="mapCanvas?.recenter()"
        />
      </div>
      <!-- Reskin 3b: floating scope legend (bottom-left). The bottom-right
           width/leader readout was removed (E2E): it read like a scale bar but
           was only the scope width, and sat confusingly next to the range rings.
           Distance is read from the range rings; the speed leaders themselves
           carry the look-ahead. -->
      <div class="scope-legend-overlay">
        <ScopeLegend />
      </div>
    </v-main>

    <!-- ── Mobile (phone / tablet-portrait): bottom tab bar + sheets (#194) ── -->
    <template v-if="!mdAndUp">
      <BottomNav v-model="mobileTab" :is-admin="isAdmin" @select="onMobileTab" />

      <!-- Filter/Layer as a bottom sheet. Measure tools (RBL/DIST/QDM) live in
           the sheet header on mobile — the rail that normally hosts them is not
           rendered on phones. -->
      <v-bottom-sheet v-model="filterSheet" :scrim="true" @update:model-value="onSheetToggle">
        <v-card class="mobile-sheet" rounded="t-xl">
          <div class="mobile-sheet__grab" />
          <div class="mobile-sheet__hd">
            <span class="mobile-sheet__ttl">Layer &amp; Filter</span>
            <div class="mobile-sheet__tools">
              <v-btn
                v-for="t in measureTools"
                :key="t.id"
                :icon="t.icon"
                size="small"
                variant="text"
                :color="tools.activeTool === t.id ? 'primary' : undefined"
                :aria-label="t.label"
                @click="tools.selectTool(t.id)"
              />
              <v-btn icon="mdi-close" size="small" variant="text" aria-label="Schließen" @click="closeSheets" />
            </div>
          </div>
          <div class="mobile-sheet__body">
            <LayerFilterContent
              section="all"
              @layer-toggle="onLayerToggle"
              @fl-filter-change="onFlFilterChange"
            />
          </div>
        </v-card>
      </v-bottom-sheet>

      <!-- Konto as a bottom sheet (account section: subject + logout). -->
      <v-bottom-sheet v-model="kontoSheet" :scrim="true" @update:model-value="onSheetToggle">
        <v-card class="mobile-sheet" rounded="t-xl">
          <div class="mobile-sheet__grab" />
          <div class="mobile-sheet__hd">
            <span class="mobile-sheet__ttl">Konto</span>
            <v-btn icon="mdi-close" size="small" variant="text" aria-label="Schließen" @click="closeSheets" />
          </div>
          <div class="mobile-sheet__body">
            <LayerFilterContent section="account" />
          </div>
        </v-card>
      </v-bottom-sheet>
    </template>

    <TrackDetailPanel
      v-if="store.selectedTrack"
      @close="store.clearTrackSelection()"
    />
  </template>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useRouter } from 'vue-router'
import { useAsdStore } from '@/stores/asd.js'
import { useSessionStore } from '@/stores/session.js'
import { useToolsStore } from '@/stores/tools.js'
import { useAdminStore } from '@/stores/admin.js'
import { useImpersonationStore } from '@/stores/impersonation.js'
import NavigationRail from '@/components/NavigationRail.vue'
import BottomNav from '@/components/BottomNav.vue'
import MapCanvas from '@/components/MapCanvas.vue'
import AsdHeader from '@/components/AsdHeader.vue'
import ScopeLegend from '@/components/ScopeLegend.vue'
import TrackDetailPanel from '@/components/TrackDetailPanel.vue'
import FeedStatusChip from '@/components/FeedStatusChip.vue'
import EventPanel from '@/components/EventPanel.vue'
import ViewProfileMenu from '@/components/ViewProfileMenu.vue'
import MapSearch from '@/components/MapSearch.vue'
import ViewportControls from '@/components/ViewportControls.vue'
import LoginCard from '@/components/LoginCard.vue'
import { useEventsStore } from '@/stores/events.js'

const { mdAndUp } = useDisplay()
const router = useRouter()
const store = useAsdStore()
const session = useSessionStore()
const tools = useToolsStore()
const adminStore = useAdminStore()
const imp = useImpersonationStore()
const events = useEventsStore()
const drawerOpen = ref(true)
const mapCanvas = ref(null)
const loginLoading = ref(false)

// ASD-013: event-log panel visibility. Opening it marks the log seen (clears the
// unseen badge) without dropping history.
const eventsOpen = ref(false)
const badgeContent = computed(() => (events.unseenCount > 99 ? '99+' : String(events.unseenCount)))
function toggleEvents() {
  eventsOpen.value = !eventsOpen.value
  if (eventsOpen.value) events.markSeen()
}

// #277 Nachtrag (operator 2026-07-19): the search is a companion to the VISIBLE
// base map — it appears only once the Lotse has actually switched the BKG layer
// ON (store.layerVisibility.basemap), not merely on entitlement. No map on
// screen ⇒ nothing to locate against ⇒ no search icon, keeping the scope clear
// over the tracks. The layer toggle itself is entitlement-gated (WF2-50) and the
// server enforces /api/basemap/search fail-closed, so this stays a display gate.
const showSearch = computed(() => store.layerVisibility.basemap === true)

// Turning the base map back off removes the search — clear any leftover result
// marker so a found place does not linger on the synthetic scope.
watch(showSearch, (on) => {
  if (!on) mapCanvas.value?.clearSearchMarker()
})

// #277: a picked search hit → magenta marker + camera onto the place.
function onSearchSelect(hit) {
  mapCanvas.value?.showSearchMarker(hit.lon, hit.lat, hit.name)
}

// ASD-013: clicking a still-live "Track N erschienen" row selects that track
// (opens the detail panel + halo, eases the camera onto it). The engine returns
// false if the track has since gone; only on a real selection do we close the
// panel, keeping the scope tidy (the detail card takes over).
function onEventSelectTrack(trackNum) {
  if (mapCanvas.value?.selectTrackByNum(trackNum)) eventsOpen.value = false
}

// #208 (ADR 0022): admins have no own air picture; the server rejects their /ws
// without an active guest-mode grant. This gate decides AFTER authentication
// whether the map may mount: a must-change principal goes to /admin (the forced
// password mask lives there, and the server refuses every data path anyway); an
// admin goes to /admin unless read-only impersonation is active. 'pending' keeps
// the spinner up so the map never opens a doomed /ws.
const adminGate = ref('pending') // 'pending' | 'ok'

async function applyAdminGate() {
  if (session.mustChangePassword) {
    router.replace('/admin')
    return
  }
  if (session.isAdmin) {
    await imp.loadStatus()
    if (!imp.active) {
      router.replace('/admin')
      return
    }
  }
  adminGate.value = 'ok'
}

watch(() => session.status, (s) => {
  if (s === 'authed') applyAdminGate()
  else adminGate.value = 'pending'
}, { immediate: true })

// #194: mobile navigation state (phone / tablet-portrait). The bottom tab bar
// selects between the scope and the Filter/Konto sheets; Admin routes away.
const mobileTab = ref('scope')
const filterSheet = ref(false)
const kontoSheet = ref(false)
const isAdmin = computed(() => adminStore.isAdmin)

// Measure tools relocated into the mobile Filter sheet (the rail that hosts them
// on desktop is not rendered on phones). Mirrors NavigationRail's list.
const measureTools = [
  { id: 'rbl', icon: 'mdi-vector-line', label: 'RBL' },
  { id: 'dist', icon: 'mdi-ruler', label: 'DIST' },
  { id: 'qdm', icon: 'mdi-compass-outline', label: 'QDM' },
]

function closeSheets() {
  filterSheet.value = false
  kontoSheet.value = false
  mobileTab.value = 'scope'
}

// Bottom-nav tab handler: open the matching sheet, route to Admin, or clear.
function onMobileTab(id) {
  if (id === 'filter') { kontoSheet.value = false; filterSheet.value = true }
  else if (id === 'konto') { filterSheet.value = false; kontoSheet.value = true }
  else if (id === 'admin') { router.push('/admin') }
  else { closeSheets() } // scope
}

// When a sheet is dismissed by swipe/scrim, snap the tab back to Scope so the
// bar's active state matches what's on screen.
function onSheetToggle(open) {
  if (!open && !filterSheet.value && !kontoSheet.value) mobileTab.value = 'scope'
}

// Make an expiry visible: a dropped session shows "session expired" on the login
// screen instead of a bare prompt (WF2-12.5).
const loginNotice = computed(() =>
  session.expired ? 'Sitzung abgelaufen — bitte erneut anmelden.' : session.error,
)

// Resolve the session on entry, and slide it forward when the tab regains focus.
onMounted(() => {
  session.probe()
  // #194: the bottom tab bar's Admin entry needs the admin probe even on phones,
  // where NavigationRail (which normally triggers it) is not rendered. Fail-closed.
  if (!adminStore.isAuthorized) adminStore.loadIdentity()
  document.addEventListener('visibilitychange', onVisible)
})
onUnmounted(() => {
  document.removeEventListener('visibilitychange', onVisible)
  session.stopRenew()
})

// Run the sliding-refresh loop only while authenticated.
watch(() => session.status, (s) => {
  if (s === 'authed') session.startRenew()
  else session.stopRenew()
})

function onVisible() {
  if (document.visibilityState === 'visible' && session.status === 'authed') {
    session.renewNow()
  }
}

// The map's WebSocket lifecycle drives session freshness (WF2-12.5): on connect,
// slide the session forward; on a drop, probe — if auth was lost the probe flips
// to 'anon' and the login overlay appears; a transient drop stays authenticated
// and the engine reconnects on its own.
async function onConnectionChange(state) {
  if (state === 'open') {
    session.renewNow()
  } else if (state === 'closed' && session.status === 'authed') {
    session.probe()
    // #208 (ADR 0022): for an admin the /ws drop may mean the guest-mode grant
    // expired (TTL) — the server then rejects every reconnect. Re-check the
    // grant and return to /admin instead of letting the map spin on a dead
    // stream.
    if (session.isAdmin) {
      await imp.loadStatus()
      if (!imp.active) router.replace('/admin')
    }
  }
}

async function onLogin({ subject, password }) {
  loginLoading.value = true
  try {
    await session.login(subject, password)
  } finally {
    loginLoading.value = false
  }
}

function onLayerToggle({ layer, val }) {
  mapCanvas.value?.setLayerVisibility(layer, val)
}

function onFlFilterChange() {
  mapCanvas.value?.updateFlFilter()
}

// #121: after the sidebar panel opens/closes the drawer width animates
// (56 ↔ 300 px); MapLibre must re-measure its container once the transition
// settles or a dead, unpainted strip remains where the panel was.
function onPanelResize() {
  setTimeout(() => mapCanvas.value?.resize(), 250)
}

function onTrackClick(track) {
  // Häppchen 4: while a measurement tool is active, a track click feeds the tool
  // (DIST/QDM pick it via the map controller) — don't also open the detail panel.
  if (tools.activeTool) return
  store.selectTrack(track)
}

// #273: a click on free map area deselects (panel closes, halo clears) — the
// standard map-UI convention. Same measure-tool guard as onTrackClick: RBL/
// DIST/QDM consume map clicks, so a tool click must never drop the selection.
function onMapEmptyClick() {
  if (tools.activeTool) return
  store.clearTrackSelection()
}
</script>

<style scoped>
/* ASD-018 (overlay-zone layout, ADR 0029): the RIGHT-EDGE overlay zone. This is
   ONE positioned flex column that owns everything on the top-right — ICAO/UTC
   header, feed badge, the action row (profile/bell/search) and, as its last
   child, the viewport controls (recenter/fullscreen). Because every element is a
   flex CHILD of this one container, a new element pushes the ones below it in
   flow — nothing overlaps. The rule (ADR 0029): new chrome goes INTO a zone as a
   flex child, never as a free-floating `position:absolute` element with a
   guessed `top`/`right` (that guessing is what caused the recurring
   controls-overlap bug at #194 and again with the search icon). */
.top-right-cluster {
  position: absolute;
  /* Edge inset from the overlay-gap token: 12px normally, one step wider on a
     24″ display so the cluster breathes rather than clinging to the corner
     (#194 Häppchen 3). */
  top: calc(var(--wf-overlay-gap, 12px) + var(--wf-safe-top, 0px));
  right: calc(var(--wf-overlay-gap, 12px) + var(--wf-safe-right, 0px));
  z-index: 600;
  display: flex;
  /* Design template: header (ICAO/UTC) and the feed badge stack vertically,
     right-aligned, so the feed badge sits on its own line under the clock. */
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  pointer-events: none;
  /* Never let the header run under the opposite chrome on a narrow phone. */
  max-width: calc(100vw - 24px - var(--wf-safe-right, 0px) - var(--wf-safe-left, 0px));
}

/* The profile switcher + event bell share one right-aligned row so the cluster
   stays compact (icons-overlap fix). Its interactive children re-enable pointer
   events individually; the row itself stays click-through over the map. */
.cluster-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* ASD-013: the event bell + floating panel live inside the (pointer-events:none)
   top-right cluster, so they must re-enable pointer events to stay clickable. */
.events-control,
.events-panel {
  pointer-events: auto;
}

/* ASD-018 (ADR 0029): the viewport controls sit at the foot of the right rail.
   A touch more top spacing groups them visually apart from the status/action
   chrome above (they used to be ~140px away as a separate stack). */
.viewport-controls-slot {
  margin-top: 4px;
}

/* #277: the search field lives inside the (pointer-events:none) cluster and
   must stay typeable/clickable. */
.map-search-control {
  pointer-events: auto;
}

/* ASD-007: cyan radar-scope centre bloom (design template 'nacht' glow). Above
   the map tiles, below the chrome (z 600) and the map controls (z 10). */
.scope-glow-overlay {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  background: radial-gradient(circle at 50% 46%, rgba(35, 211, 230, 0.05), transparent 55%);
}

/* Reskin 3b: floating scope legend (bottom-left). pointer-events on the wrapper
   are off; the legend itself re-enables them so its toggle stays clickable. */
.scope-legend-overlay {
  position: absolute;
  bottom: var(--wf-overlay-gap, 12px);
  /* Clear the navigation rail (left edge) + a gap, so the legend is not painted
     over by the opaque fixed drawer. Derived from --wf-nav-rail-width so it
     tracks the rail: 68px on desktop (56+12), 88px on the iPad band (76+12,
     #194 Häppchen 2). v-main spans full width (padding:0), so this offset is
     measured from the viewport left. */
  left: calc(var(--wf-nav-rail-width, 56px) + var(--wf-overlay-gap, 12px));
  z-index: 600;
  pointer-events: none;
}

/* #194 — Mobile (< md): no rail, so the legend hugs the left safe-area edge and
   sits above the bottom tab bar instead of the (absent) rail. */
@media (max-width: 959.98px) {
  .scope-legend-overlay {
    left: calc(12px + var(--wf-safe-left, 0px));
    bottom: calc(12px + var(--wf-bottom-nav-h, 64px) + var(--wf-safe-bottom, 0px));
  }
}

/* #194 — Mobile Filter/Konto bottom sheet (design mockup): grab handle, header
   with title + tools, scrollable body. Rounded top; padded past the home
   indicator. Height is capped so the scope stays partly visible behind it. */
.mobile-sheet {
  display: flex;
  flex-direction: column;
  max-height: 82vh;
  background: rgb(var(--v-theme-surface));
  padding-bottom: var(--wf-safe-bottom, 0px);
}
.mobile-sheet__grab {
  width: 38px;
  height: 4px;
  border-radius: 2px;
  background: rgba(var(--v-border-color), 0.32);
  margin: 10px auto 2px;
  flex-shrink: 0;
}
.mobile-sheet__hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 10px 10px 18px;
  border-bottom: var(--wf-chrome-border);
  flex-shrink: 0;
}
.mobile-sheet__ttl {
  font-size: 16px;
  font-weight: 500;
  color: rgb(var(--v-theme-on-surface));
}
.mobile-sheet__tools {
  display: flex;
  align-items: center;
  gap: 2px;
}
.mobile-sheet__body {
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}
</style>
