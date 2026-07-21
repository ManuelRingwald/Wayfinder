<template>
  <!-- ASD-013 / #116: sidebar panel content, split into three sections the
       NavigationRail opens individually (Layer / Filter / Nutzer-Account).
       section='all' (mobile drawer) renders everything in order, account last.
       ASD-020 (ADR 0031): the Layer section is organised into collapsible GROUPS
       (Aeronautik / Karte / Radar & Reichweite / Wetter), each a LayerGroup with
       a tri-state master — the flat switch list is gone. -->
  <div class="filter-panel">

    <!-- ── Layer (#116) ── -->
    <template v-if="showSection('layers')">
      <div class="filter-section-header">Layer</div>

      <!-- Issue #106 (cosmetic feature gate): showLayer(key) hides a layer the
           tenant is not entitled to. Fail-open while the identity is still loading or
           for an admin viewer (gateReady false → show all). The server enforces
           access independently; this is a pure UX gate. A GROUP hides entirely
           (v-if) only when it has no visible member at all. -->

      <!-- ── Aeronautik: airspaces + AoR + navaids + waypoints + airport/runways ── -->
      <LayerGroup
        v-if="showAero"
        title="Aeronautik"
        :master="aeroState"
        :expanded="openGroup === 'aero'"
        @toggle="toggleGroup('aero')"
        @toggle-master="onGroupMaster(aeroMembers, aeroState)"
      >
        <!-- ASD-011 / #176: the four airspace groups are first-class toggles (the
             parent "Lufträume" toggle was removed). The airspace layer is visible
             iff at least one group is on, derived in the store (setAirspaceGroup). -->
        <template v-if="showLayer('airspaces')">
          <div
            v-for="group in AIRSPACE_GROUPS"
            :key="group.id"
            class="filter-row"
          >
            <v-switch
              :model-value="store.airspaceGroupVisibility[group.id]"
              :label="group.label"
              color="primary"
              density="compact"
              hide-details
              inset
              @update:model-value="onAirspaceGroup(group.id, $event)"
            />
          </div>
          <!-- ASD-014: highlight the tenant's Area of Responsibility (CTR/TMA) with a
               bright outline, distinct from context airspace. Only visible when an AoR
               is configured (whoami.aor_airspace_ids); the toggle simply hides it. -->
          <div class="filter-row">
            <v-switch
              :model-value="store.layerVisibility.aor"
              color="primary"
              density="compact"
              hide-details
              inset
              @update:model-value="onLayerToggle('aor', $event)"
            >
              <template #label>
                <span class="aor-legend">
                  <span class="aor-swatch" :style="{ background: AIRSPACE_AOR_COLOR }"></span>
                  Verantwortungsbereich (AoR)
                </span>
              </template>
            </v-switch>
          </div>
        </template>

        <div v-if="showLayer('vor_ndb')" class="filter-row">
          <v-switch
            v-model="store.layerVisibility.navaids"
            label="VOR / NDB"
            color="primary"
            density="compact"
            hide-details
            inset
            @update:model-value="onLayerToggle('navaids', $event)"
          />
        </div>

        <div v-if="showLayer('waypoints')" class="filter-row">
          <v-switch
            v-model="store.layerVisibility.waypoints"
            label="Waypoints"
            color="primary"
            density="compact"
            hide-details
            inset
            @update:model-value="onLayerToggle('waypoints', $event)"
          />
        </div>

        <!-- #192: airport reference-point markers, feature-gated (airport). -->
        <div v-if="showLayer('airport')" class="filter-row">
          <v-switch
            v-model="store.layerVisibility.airport"
            label="Flughäfen"
            color="primary"
            density="compact"
            hide-details
            inset
            @update:model-value="onLayerToggle('airport', $event)"
          />
        </div>

        <!-- #192: runway centrelines, feature-gated (runways). -->
        <div v-if="showLayer('runways')" class="filter-row">
          <v-switch
            v-model="store.layerVisibility.runways"
            label="Runways"
            color="primary"
            density="compact"
            hide-details
            inset
            @update:model-value="onLayerToggle('runways', $event)"
          />
        </div>
      </LayerGroup>

      <!-- ── Karte: the official BKG base map (the visual floor). #274: a
           grantable nice-to-have; the scope runs synthetic by default and an
           entitled user opts in here. Future BKG element sub-layers (#290) join
           this group. ── -->
      <LayerGroup
        v-if="showKarte"
        title="Karte"
        :master="karteState"
        :expanded="openGroup === 'karte'"
        @toggle="toggleGroup('karte')"
        @toggle-master="onGroupMaster(karteMembers, karteState)"
      >
        <div v-if="showLayer('basemap')" class="filter-row">
          <v-switch
            v-model="store.layerVisibility.basemap"
            label="Basiskarte (BKG)"
            color="primary"
            density="compact"
            hide-details
            inset
            @update:model-value="onLayerToggle('basemap', $event)"
          />
        </div>

        <!-- E3 (#294): one-click element presets. Shown only while the map is on
             (they are meaningless otherwise); the active preset is highlighted,
             or none when the element set is "Benutzerdefiniert". -->
        <div
          v-if="showLayer('basemap') && store.layerVisibility.basemap"
          class="filter-row filter-row--sub basemap-presets"
        >
          <v-btn-group density="compact" divided class="basemap-presets__group">
            <v-btn
              v-for="p in BASEMAP_PRESETS"
              :key="p.id"
              size="x-small"
              :variant="activeBasemapPreset === p.id ? 'flat' : 'text'"
              :color="activeBasemapPreset === p.id ? 'primary' : undefined"
              @click="store.applyBasemapPreset(p.id)"
            >{{ p.label }}</v-btn>
          </v-btn-group>
        </div>

        <!-- E2 (#293): per-element switches — "only rivers"/"only roads". They
             REFINE the base map WHEN it is shown, so they are disabled (greyed)
             while the master is off. All on by default; toggling one takes effect
             at once (MapCanvas element watcher → engine applyBasemap). -->
        <template v-if="showLayer('basemap')">
          <div
            v-for="el in BASEMAP_ELEMENTS"
            :key="el.id"
            class="filter-row filter-row--sub"
          >
            <v-switch
              :model-value="store.basemapElementVisibility[el.id]"
              :label="el.label"
              color="primary"
              density="compact"
              hide-details
              inset
              :disabled="!store.layerVisibility.basemap"
              @update:model-value="store.setBasemapElement(el.id, $event)"
            />
          </div>
        </template>
      </LayerGroup>

      <!-- ── Radar & Reichweite: coverage, history dots, range rings ── -->
      <LayerGroup
        v-if="showRadar"
        title="Radar & Reichweite"
        :master="radarState"
        :expanded="openGroup === 'radar'"
        @toggle="toggleGroup('radar')"
        @toggle-master="onGroupMaster(radarMembers, radarState)"
      >
        <!-- #114: the coverage overlay only has data when coverage sensors are
             configured server-side. Without data the toggle is disabled with an
             explanatory hint instead of silently doing nothing. -->
        <div class="filter-row">
          <v-switch
            v-model="store.layerVisibility.coverageRings"
            label="Radarabdeckung"
            color="primary"
            density="compact"
            hide-details
            inset
            :disabled="!store.coverageAvailable"
            @update:model-value="onLayerToggle('coverageRings', $event)"
          />
        </div>
        <div v-if="!store.coverageAvailable" class="filter-hint">
          Keine Radarabdeckung konfiguriert — nur bei Radar-Sensoren verfügbar.
        </div>

        <div v-if="showLayer('history_dots')" class="filter-row">
          <v-switch
            v-model="store.layerVisibility.historyDots"
            label="History Dots"
            color="primary"
            density="compact"
            hide-details
            inset
            @update:model-value="onLayerToggle('historyDots', $event)"
          />
        </div>

        <!-- #191: history retention window, shown only while the layer is active.
             Older dots fade out toward the end of the trail (engine-side). -->
        <template v-if="showLayer('history_dots') && store.layerVisibility.historyDots">
          <div class="filter-row filter-row--sub">
            <v-select
              v-model.number="historyDurationS"
              :items="HISTORY_DURATION_OPTIONS_S"
              label="Dauer (s)"
              variant="outlined"
              density="compact"
              hide-details
              class="ring-input"
              @update:model-value="onHistoryChange"
            />
          </div>
        </template>

        <div v-if="showLayer('range_rings')" class="filter-row">
          <v-switch
            v-model="store.layerVisibility.rangeRings"
            label="Range-Rings"
            color="primary"
            density="compact"
            hide-details
            inset
            @update:model-value="onLayerToggle('rangeRings', $event)"
          />
        </div>

        <!-- ASD-012: range-ring spacing + count, shown only while the layer is active -->
        <template v-if="(showLayer('range_rings')) && store.layerVisibility.rangeRings">
          <div class="filter-row filter-row--sub">
            <v-select
              v-model.number="ringSpacing"
              :items="RANGE_RING_SPACING_OPTIONS_NM"
              label="Abstand (NM)"
              variant="outlined"
              density="compact"
              hide-details
              class="ring-input"
              @update:model-value="onRangeRingChange"
            />
          </div>
          <div class="filter-row filter-row--sub ring-count-row">
            <span class="ring-count-label">Ringe: {{ ringCount }}</span>
            <v-slider
              v-model="ringCount"
              :min="1"
              :max="MAX_RANGE_RING_COUNT"
              :step="1"
              color="primary"
              density="compact"
              hide-details
              class="ring-slider"
              @update:model-value="onRangeRingChange"
            />
          </div>
        </template>
      </LayerGroup>

      <!-- ── Wetter: DWD radar + warnings (each feature-gated + availability-gated) ── -->
      <LayerGroup
        v-if="showWetter"
        title="Wetter"
        :master="wetterState"
        :expanded="openGroup === 'wetter'"
        @toggle="toggleGroup('wetter')"
        @toggle-master="onGroupMaster(wetterMembers, wetterState)"
      >
        <!-- WX-A (ADR 0016): DWD weather-radar overlay. Feature-gated per tenant
             (weather_radar) and disabled when no DWD source is configured
             server-side, with a hint instead of a dead switch. -->
        <template v-if="showLayer('weather_radar')">
          <div class="filter-row">
            <v-switch
              v-model="store.layerVisibility.weatherRadar"
              label="DWD-Regenradar"
              color="primary"
              density="compact"
              hide-details
              inset
              :disabled="!store.weatherRadarAvailable"
              @update:model-value="onLayerToggle('weatherRadar', $event)"
            />
          </div>
          <div v-if="!store.weatherRadarAvailable" class="filter-hint">
            Keine DWD-Radarquelle konfiguriert (WAYFINDER_DWD_WMS_URL).
          </div>
          <!-- #190: radar intensity legend, shown only while the layer is on. -->
          <div
            v-if="store.weatherRadarAvailable && store.layerVisibility.weatherRadar"
            class="wx-legend"
          >
            <span class="wx-legend-caption">Niederschlag</span>
            <div class="wx-legend-items">
              <span v-for="s in WEATHER_RADAR_LEGEND" :key="s.label" class="wx-legend-item">
                <span class="wx-swatch" :style="{ background: s.color }" />{{ s.label }}
              </span>
            </div>
          </div>
        </template>

        <!-- WX-C (ADR 0016): DWD weather-warnings overlay. Feature-gated per tenant
             (weather_warnings) and disabled when no DWD WFS is configured. -->
        <template v-if="showLayer('weather_warnings')">
          <div class="filter-row">
            <v-switch
              v-model="store.layerVisibility.weatherWarnings"
              label="DWD-Wetterwarnungen"
              color="primary"
              density="compact"
              hide-details
              inset
              :disabled="!store.weatherWarningsAvailable"
              @update:model-value="onLayerToggle('weatherWarnings', $event)"
            />
          </div>
          <div v-if="!store.weatherWarningsAvailable" class="filter-hint">
            Keine DWD-Warnquelle konfiguriert (WAYFINDER_DWD_WARN_URL).
          </div>
          <!-- #190: warnings severity legend, shown only while the layer is on. -->
          <div
            v-if="store.weatherWarningsAvailable && store.layerVisibility.weatherWarnings"
            class="wx-legend"
          >
            <span class="wx-legend-caption">Warnstufe</span>
            <div class="wx-legend-items">
              <span v-for="s in WEATHER_WARNINGS_LEGEND" :key="s.label" class="wx-legend-item">
                <span class="wx-swatch" :style="{ background: s.color }" />{{ s.label }}
              </span>
            </div>
          </div>
        </template>
      </LayerGroup>

      <!-- ── Spurherkunft (WF2-40/#119): symbol-glyph legend — a reference block,
           not a layer toggle, so it stays outside the groups at the section foot. ── -->
      <div class="filter-section-header filter-section-header--spaced">Spurherkunft</div>
      <div class="legend-caption">Symbol = Herkunft · Farbe = Status</div>
      <div
        v-for="item in provenanceLegend"
        :key="item.label"
        class="filter-row filter-row--sub"
      >
        <span class="prov-glyph">{{ item.glyph }}</span>
        <span class="prov-label">{{ item.label }}</span>
      </div>
    </template>

    <!-- ── Filter (#116): the FL filter, hinting the admin-configured band ── -->
    <template v-if="showSection('filters')">
      <div
        class="filter-section-header"
        :class="{ 'filter-section-header--spaced': props.section === 'all' }"
      >Filter</div>

      <div class="filter-row filter-row--inputs">
        <v-text-field
          v-model.number="minFL"
          type="number"
          label="Min FL"
          :placeholder="session.flMin != null ? String(session.flMin) : undefined"
          persistent-placeholder
          min="0"
          max="999"
          step="10"
          variant="outlined"
          density="compact"
          hide-details
          class="fl-input"
          @update:model-value="onFlFilterChange"
        />
        <span class="fl-dash">–</span>
        <v-text-field
          v-model.number="maxFL"
          type="number"
          label="Max FL"
          :placeholder="session.flMax != null ? String(session.flMax) : undefined"
          persistent-placeholder
          min="0"
          max="999"
          step="10"
          variant="outlined"
          density="compact"
          hide-details
          class="fl-input"
          @update:model-value="onFlFilterChange"
        />
      </div>
      <!-- #116: the admissible band from the tenant's Standard-Ansicht (or the
           user's override), greyed as orientation — filtering happens within it. -->
      <div v-if="flRangeHint" class="filter-hint">{{ flRangeHint }}</div>

      <div class="filter-row">
        <v-switch
          v-model="hideFiltered"
          label="Ausblenden"
          color="primary"
          density="compact"
          hide-details
          inset
          @update:model-value="onFlFilterChange"
        />
      </div>
    </template>

    <!-- ── Nutzer-Account (#116): last section, currently logout only ── -->
    <template v-if="showSection('account')">
      <div
        class="filter-section-header"
        :class="{ 'filter-section-header--spaced': props.section === 'all' }"
      >Nutzer-Account</div>
      <div class="filter-row account-row">
        <v-icon size="18" class="account-icon">mdi-account</v-icon>
        <span class="account-subject">{{ session.subject }}</span>
      </div>
      <div v-if="session.email" class="filter-row account-email-row">
        <span class="account-email">{{ session.email }}</span>
      </div>
      <!-- #319: self-service — set own e-mail + password (role-agnostic
           /api/account/*). Opens the dialog reused by admins in the dashboard. -->
      <div class="filter-row">
        <v-btn
          size="small"
          variant="tonal"
          color="primary"
          prepend-icon="mdi-account-cog"
          block
          @click="accountDialog = true"
        >
          Konto verwalten
        </v-btn>
      </div>
      <div class="filter-row">
        <v-btn
          size="small"
          variant="text"
          prepend-icon="mdi-logout"
          block
          @click="onLogout"
        >
          Abmelden
        </v-btn>
      </div>
      <AccountSelfServiceDialog v-model="accountDialog" />
    </template>

  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { useSessionStore } from '@/stores/session.js'
import { AIRSPACE_GROUPS, AIRSPACE_AOR_COLOR, RANGE_RING_SPACING_OPTIONS_NM, MAX_RANGE_RING_COUNT, HISTORY_DURATION_OPTIONS_S, WEATHER_RADAR_LEGEND, WEATHER_WARNINGS_LEGEND } from '@/map/constants.js'
import { filterProvenanceLegend } from '@/map/provenance.js'
import { masterState, nextMaster } from '@/map/layerGroups.js'
import { BASEMAP_ELEMENTS, BASEMAP_PRESETS, matchPreset } from '@/map/basemapGroups.js'
import LayerGroup from './LayerGroup.vue'
import AccountSelfServiceDialog from './AccountSelfServiceDialog.vue'

// #116: the NavigationRail opens one section at a time on desktop; the mobile
// drawer renders all of them ('all'), with the account block last.
const props = defineProps({
  section: { type: String, default: 'all' },
})
function showSection(id) {
  return props.section === 'all' || props.section === id
}

const emit = defineEmits(['layer-toggle', 'fl-filter-change'])
const store = useAsdStore()
const session = useSessionStore()

// Issue #106: show a lotse only the layers/filters their tenant is entitled to.
// The gate is driven by the role-agnostic session identity (whoami → features),
// which is populated for a plain tenant user (the admin store's role probe is not).
// gateReady is true only for an authenticated tenant user; for the loading/anon
// state or an admin/platform viewer with no tenant scope we show everything
// (fail-open cosmetic gate — the server enforces access independently).
const gateReady = computed(() => session.status === 'authed' && !session.isAdmin)
function showLayer(featureKey) {
  return !gateReady.value || session.hasFeature(featureKey)
}

// ── ASD-020 (ADR 0031): Layer-group model ──────────────────────────────────
// Each group's MEMBERSHIP lives here, next to the rows it renders. A member is
// { on, set, enabled }: `on` is its current visibility, `set(v)` applies a new
// one through the SAME store path the row's own switch uses (so a master click
// is indistinguishable from clicking each row), and `enabled` (default true)
// marks whether the operator can change it — a disabled toggle (unavailable
// source) is excluded from the master state and from the bulk action.
function layerMember(key) {
  return { on: store.layerVisibility[key], set: (v) => onLayerToggle(key, v) }
}

const aeroMembers = () => {
  const list = []
  if (showLayer('airspaces')) {
    for (const g of AIRSPACE_GROUPS) {
      list.push({ on: store.airspaceGroupVisibility[g.id], set: (v) => onAirspaceGroup(g.id, v) })
    }
    list.push({ on: store.layerVisibility.aor, set: (v) => onLayerToggle('aor', v) })
  }
  if (showLayer('vor_ndb')) list.push(layerMember('navaids'))
  if (showLayer('waypoints')) list.push(layerMember('waypoints'))
  if (showLayer('airport')) list.push(layerMember('airport'))
  if (showLayer('runways')) list.push(layerMember('runways'))
  return list
}

const karteMembers = () => {
  const list = []
  if (showLayer('basemap')) list.push(layerMember('basemap'))
  return list
}

const radarMembers = () => {
  const list = []
  list.push({ on: store.layerVisibility.coverageRings, set: (v) => onLayerToggle('coverageRings', v), enabled: store.coverageAvailable })
  if (showLayer('range_rings')) list.push(layerMember('rangeRings'))
  if (showLayer('history_dots')) list.push(layerMember('historyDots'))
  return list
}

const wetterMembers = () => {
  const list = []
  if (showLayer('weather_radar')) list.push({ on: store.layerVisibility.weatherRadar, set: (v) => onLayerToggle('weatherRadar', v), enabled: store.weatherRadarAvailable })
  if (showLayer('weather_warnings')) list.push({ on: store.layerVisibility.weatherWarnings, set: (v) => onLayerToggle('weatherWarnings', v), enabled: store.weatherWarningsAvailable })
  return list
}

// A group's master reflects only its ENABLED members; it is hidden when a group
// has no visible member at all (v-if on the group).
function groupMaster(members) {
  return masterState(members().filter((m) => m.enabled !== false).map((m) => m.on))
}
const aeroState = computed(() => groupMaster(aeroMembers))
const karteState = computed(() => groupMaster(karteMembers))
const radarState = computed(() => groupMaster(radarMembers))
const wetterState = computed(() => groupMaster(wetterMembers))

// E3 (#294): which preset (if any) the current element set matches — drives the
// highlight; null = "Benutzerdefiniert" (no button highlighted).
const activeBasemapPreset = computed(() => matchPreset(store.basemapElementVisibility))

const showAero = computed(() => aeroMembers().length > 0)
const showKarte = computed(() => karteMembers().length > 0)
const showRadar = computed(() => radarMembers().length > 0)
const showWetter = computed(() => wetterMembers().length > 0)

// #317: accordion — only ONE Layer group is expanded at a time, so the second
// sidebar level never grows tall enough to scroll. openGroup holds the single
// open group's id (null = all collapsed); toggleGroup opens the clicked group
// and, by replacing the id, collapses whichever was open before (e.g. opening
// "Wetter" folds Aeronautik/Karte/Radar shut). Starts on 'aero' (the airspace
// group, present for virtually every tenant); a second click on the open group
// collapses it to null.
const openGroup = ref('aero')
function toggleGroup(id) {
  openGroup.value = openGroup.value === id ? null : id
}

// Master click: select-all/none over the group's ENABLED members (a disabled
// toggle is left as-is — the operator cannot turn on a layer with no data).
function onGroupMaster(members, state) {
  const target = nextMaster(state)
  for (const m of members()) {
    if (m.enabled !== false && m.on !== target) m.set(target)
  }
}

const minFL = ref(store.flFilter.minFL)
const maxFL = ref(store.flFilter.maxFL)
const hideFiltered = ref(store.flFilter.hide)

// #116: admissible FL band from the effective view config (whoami), greyed as
// a hint under the filter inputs. Empty string when nothing is configured.
const flRangeHint = computed(() => {
  if (session.flMin == null && session.flMax == null) return ''
  const lo = session.flMin ?? 0
  const hi = session.flMax != null ? `FL ${session.flMax}` : 'unbegrenzt'
  return `Zulässiger Bereich: FL ${lo} – ${hi}`
})

// ASD-012: local range-ring controls, mirrored into the reactive store on change
// (the engine regenerates the overlay; MapCanvas watches store.rangeRingConfig).
const ringSpacing = ref(store.rangeRingConfig.spacingNM)
const ringCount = ref(store.rangeRingConfig.count)
function onRangeRingChange() {
  store.setRangeRingConfig({ spacingNM: ringSpacing.value, count: ringCount.value })
}

// #191: history-dots retention window (seconds), mirrored into the store on
// change (MapCanvas watches store.historyConfig and re-renders the dots).
const historyDurationS = ref(store.historyConfig.durationS)
function onHistoryChange() {
  store.setHistoryConfig({ durationS: historyDurationS.value })
}

// WF2-40 + Issues #107/#119: track-symbol provenance legend, filtered to the
// sensor classes the tenant's feeds can actually produce (session.sensorClasses,
// the union across subscribed feeds). Glyphs mirror the map icons drawn in
// layers.js — ◆ ADS-B, ■ SSR/Mode S, ● primary/PSR (geometric marks per the
// design legend), F for FLARM (#119); colour is omitted on purpose — it encodes
// track state, not provenance (see caption). Fallback: when no sensor classes are known yet
// (still loading / admin viewer / no subscribed feed) the full legend is shown
// rather than an empty box.
// #125: the "Kombiniert" (K) entry is appended by filterProvenanceLegend when ≥2
// sources can contribute (single source of truth in map/provenance.js).
const provenanceLegend = computed(() => filterProvenanceLegend(session.sensorClasses))

function onLayerToggle(layer, val) {
  store.setLayerVisibility(layer, val)
  emit('layer-toggle', { layer, val })
}

// #176: toggling an airspace group updates the group filter AND derives the
// airspace layer visibility (visible iff any group is on). Both mutations live
// in the store; MapCanvas's layerVisibility + airspaceGroupVisibility watchers
// pick them up, so no explicit layer-toggle emit is needed here.
function onAirspaceGroup(id, val) {
  store.setAirspaceGroup(id, val)
}

function onFlFilterChange() {
  const update = {
    minFL: minFL.value || null,
    maxFL: maxFL.value || null,
    hide: hideFiltered.value,
  }
  store.setFlFilter(update)
  emit('fl-filter-change', update)
}

// #319: the account self-service dialog (own e-mail + password), opened from the
// "Konto" section. Lets a plain tenant user manage its account from the ASD.
const accountDialog = ref(false)

// #116: logout from the account section. The session store flips to 'anon' and
// AsdView swaps the scope for the login screen.
async function onLogout() {
  await session.logout()
}
</script>

<style scoped>
.filter-panel {
  padding: 8px 0 16px;
}

/* Section header: small, uppercase, subdued — visual separator, not interactive.
   #176: every header carries an underline so the logic blocks read as clearly
   separated (previously only the --spaced variant had a rule, so the first
   "Layer" header had none). */
.filter-section-header {
  /* Design System v1: the signature "overline" section header, token-driven.
     #187: calibrated to the ASD-display template — a more prominent section
     heading ("LAYER") than the base overline tokens, so it clearly outranks the
     (now smaller) row labels. Size/weight are set explicitly above the tokens. */
  padding: 10px 14px 6px;
  margin: 0 6px;
  font-size: 0.82rem;
  font-weight: 700;
  letter-spacing: var(--wf-overline-tracking);
  text-transform: uppercase;
  color: var(--wf-overline-color);
  line-height: 1.4;
  border-bottom: 1px solid rgba(var(--v-border-color), 0.16);
}

/* Extra top margin before a new logic block (the underline itself is on every
   header now, so no separate top border here). */
.filter-section-header--spaced {
  margin-top: 14px;
}

/* Standard filter row */
.filter-row {
  display: flex;
  align-items: center;
  padding: 2px 10px 2px 12px;
  min-height: 36px;
}

/* Sub-group row: indented with coloured dot */
.filter-row--sub {
  padding-left: 20px;
  min-height: 32px;
}

/* Greyed hint line under a control (admissible FL band, disabled coverage) */
.filter-hint {
  padding: 0 14px 6px;
  font-size: 10.5px;
  line-height: 1.35;
  color: rgba(var(--v-theme-on-surface), 0.45);
}

/* FL input pair */
.filter-row--inputs {
  gap: 6px;
  padding: 6px 12px 4px;
  align-items: center;
}
.fl-input {
  flex: 1;
  min-width: 0;
}
.fl-dash {
  font-size: 0.9rem;
  color: rgba(var(--v-theme-on-surface), 0.4);
  flex-shrink: 0;
}

/* Tighten the switch track to be proportional and not oversized.
   #187: shortened further to match the ASD-display template (24×12, thumb 9). */
:deep(.v-switch .v-selection-control) {
  min-height: unset;
}
:deep(.v-switch .v-switch__track) {
  height: 12px;
  width: 24px;
  border-radius: 6px;
}
:deep(.v-switch .v-switch__thumb) {
  width: 9px;
  height: 9px;
}

/* #187: smaller row labels than the Vuetify default, so the section header
   ("LAYER") clearly outranks them, per the ASD-display template. */
.filter-row :deep(.v-label) {
  font-size: 0.8rem;
  opacity: 0.9;
}

/* ASD-012 range-ring controls */
.ring-input {
  flex: 1;
  min-width: 0;
}
.ring-count-row {
  flex-direction: column;
  align-items: stretch;
  gap: 2px;
  padding-top: 4px;
}
.ring-count-label {
  font-size: 0.78rem;
  opacity: 0.8;
}
.ring-slider {
  margin: 0 4px;
}

/* WF2-40 provenance legend */
.legend-caption {
  padding: 0 14px 6px;
  font-size: 10px;
  color: rgba(var(--v-theme-on-surface), 0.45);
}
.prov-glyph {
  width: 14px;
  margin-right: 8px;
  text-align: center;
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
  color: rgba(var(--v-theme-on-surface), 0.85);
  flex-shrink: 0;
}
.prov-label {
  font-size: 0.8rem;
  opacity: 0.85;
}

/* #190: DWD weather legends (radar intensity / warning severity). Sits directly
   under the toggle, indented like a sub-row, so it never overlaps the provenance
   legend or the map chips. Shown only while the layer is on. */
.wx-legend {
  padding: 0 14px 8px 20px;
}
.wx-legend-caption {
  display: block;
  font-size: 10px;
  color: rgba(var(--v-theme-on-surface), 0.45);
  margin-bottom: 3px;
}
.wx-legend-items {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 10px;
}
.wx-legend-item {
  display: inline-flex;
  align-items: center;
  font-size: 0.72rem;
  opacity: 0.85;
}
.wx-swatch {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  margin-right: 4px;
  flex-shrink: 0;
}

/* E3 (#294): base-map preset buttons — a compact segmented row above the element
   switches, so a preset is one tap and does not crowd the narrow panel. */
.basemap-presets {
  padding-top: 4px;
  padding-bottom: 2px;
  /* #316: the segmented preset control spans the panel width — it is a control,
     not an indented child row, so it drops the .filter-row--sub 20px indent and
     uses the normal row inset. Combined with the wider panel this gives the
     three labels ("Minimal/Standard/Detailliert") room to render in full. */
  padding-left: 10px;
  padding-right: 10px;
}
.basemap-presets__group {
  width: 100%;
}
.basemap-presets__group :deep(.v-btn) {
  /* Equal thirds with min-width:0 so a long label ("Detailliert") shrinks the
     button to its share of the row instead of overflowing it (#316). */
  flex: 1 1 0;
  min-width: 0;
  padding: 0 6px;
  font-size: 0.68rem;
  letter-spacing: 0;
}

/* ASD-014: AoR toggle swatch — a short line stroke echoing the map highlight. */
.aor-legend {
  display: inline-flex;
  align-items: center;
}
.aor-swatch {
  width: 14px;
  height: 3px;
  border-radius: 2px;
  margin-right: 6px;
  flex-shrink: 0;
}

/* #116 account section */
.account-row {
  gap: 8px;
}
.account-icon {
  opacity: 0.7;
}
.account-subject {
  font-size: 0.85rem;
  opacity: 0.9;
}
/* #319: the caller's own e-mail, shown under the subject in the account section. */
.account-email-row {
  min-height: unset;
  padding-top: 0;
  padding-bottom: 2px;
}
.account-email {
  font-size: 0.78rem;
  opacity: 0.6;
  word-break: break-all;
}
</style>
