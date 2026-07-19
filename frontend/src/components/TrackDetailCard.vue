<template>
  <v-card-title class="d-flex align-center justify-space-between pt-3 pb-1">
    <span class="text-h6">{{ track.callsign ?? `#${track.track_num}` }}</span>
    <v-btn icon="mdi-close" size="small" variant="text" @click="emit('close')" />
  </v-card-title>
  <v-card-text class="pb-3">
    <v-list density="compact" class="pa-0">
      <v-list-item v-if="track.flight_level_ft != null" prepend-icon="mdi-airplane-cruise">
        <v-list-item-title class="wf-mono">{{ flLabel }}</v-list-item-title>
        <v-list-item-subtitle>Flugfläche</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="track.flight_level_ft != null || track.rocd_ft_min != null" prepend-icon="mdi-swap-vertical">
        <v-list-item-title>{{ vTrendGlyph }}{{ vTrendLabel }}</v-list-item-title>
        <v-list-item-subtitle>Vertikaltendenz</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="baroAltitudeLabel" prepend-icon="mdi-altimeter">
        <v-list-item-title class="wf-mono">{{ baroAltitudeLabel }}</v-list-item-title>
        <v-list-item-subtitle>Barometrische Höhe (gefiltert)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="rocdLabel" prepend-icon="mdi-trending-up">
        <v-list-item-title class="wf-mono">{{ rocdLabel }}</v-list-item-title>
        <v-list-item-subtitle>Steig-/Sinkrate</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="geometricAltitudeLabel" prepend-icon="mdi-earth">
        <v-list-item-title class="wf-mono">{{ geometricAltitudeLabel }}</v-list-item-title>
        <v-list-item-subtitle>Geometrische Höhe (WGS84)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="groundSpeedKt" prepend-icon="mdi-speedometer">
        <v-list-item-title class="wf-mono">{{ groundSpeedKt }} kt</v-list-item-title>
        <v-list-item-subtitle>Bodengeschwindigkeit</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="headingLabel" prepend-icon="mdi-compass-outline">
        <v-list-item-title class="wf-mono">{{ headingLabel }}</v-list-item-title>
        <v-list-item-subtitle>Kurs über Grund</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="courseTrendLabel" prepend-icon="mdi-arrow-u-right-top">
        <v-list-item-title>{{ courseTrendLabel }}</v-list-item-title>
        <v-list-item-subtitle>Kurventrend</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="speedTrendLabel" prepend-icon="mdi-speedometer-slow">
        <v-list-item-title>{{ speedTrendLabel }}</v-list-item-title>
        <v-list-item-subtitle>Geschwindigkeitstrend</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="accelerationLabel" prepend-icon="mdi-rocket-launch-outline">
        <v-list-item-title class="wf-mono">{{ accelerationLabel }}</v-list-item-title>
        <v-list-item-subtitle>Beschleunigung</v-list-item-subtitle>
      </v-list-item>
      <v-list-item
        v-if="selectedAltitudeLabel"
        :prepend-icon="levelBust ? 'mdi-alert' : 'mdi-target'"
        :base-color="levelBust ? 'warning' : undefined"
      >
        <v-list-item-title class="wf-mono">{{ selectedAltitudeLabel }}</v-list-item-title>
        <v-list-item-subtitle>
          {{ levelBust ? 'Selected Altitude — weicht von Ist-Höhe ab' : 'Selected Altitude (Autopilot)' }}
        </v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="magneticHeadingLabel" prepend-icon="mdi-compass">
        <v-list-item-title class="wf-mono">{{ magneticHeadingLabel }}</v-list-item-title>
        <v-list-item-subtitle>Magnetischer Steuerkurs</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="iasLabel" prepend-icon="mdi-speedometer-medium">
        <v-list-item-title class="wf-mono">{{ iasLabel }}</v-list-item-title>
        <v-list-item-subtitle>Angezeigte Geschwindigkeit (IAS)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="machLabel" prepend-icon="mdi-airplane">
        <v-list-item-title class="wf-mono">{{ machLabel }}</v-list-item-title>
        <v-list-item-subtitle>Mach-Zahl</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="positionLabel" prepend-icon="mdi-crosshairs-gps">
        <v-list-item-title class="wf-mono">{{ positionLabel }}</v-list-item-title>
        <v-list-item-subtitle>Position (WGS84)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="provenanceLabel" prepend-icon="mdi-radar">
        <v-list-item-title>{{ provenanceLabel }}</v-list-item-title>
        <v-list-item-subtitle>Herkunft</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="sensorAges.length" prepend-icon="mdi-access-point">
        <v-list-item-title class="d-flex flex-wrap ga-1">
          <v-chip
            v-for="s in sensorAges"
            :key="s.key"
            size="x-small"
            variant="tonal"
            :color="s.fresh ? 'success' : undefined"
          >
            {{ s.label }} {{ formatAge(s.ageS) }}
          </v-chip>
        </v-list-item-title>
        <v-list-item-subtitle>Sensor-Aktualität</v-list-item-subtitle>
      </v-list-item>
      <v-list-item
        v-if="planCallsign"
        :prepend-icon="planCallsignMismatch ? 'mdi-alert' : 'mdi-clipboard-text-outline'"
        :base-color="planCallsignMismatch ? 'warning' : undefined"
      >
        <v-list-item-title class="wf-mono">{{ planCallsign }}</v-list-item-title>
        <v-list-item-subtitle>
          {{ planCallsignMismatch ? 'Plan-Callsign — weicht von gesendeter Kennung ab' : 'Plan-Callsign (Flugplan)' }}
        </v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="planRouteLabel" prepend-icon="mdi-map-marker-path">
        <v-list-item-title class="wf-mono">{{ planRouteLabel }}</v-list-item-title>
        <v-list-item-subtitle>Route (ADEP → ADES)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="track.mode_3a != null" prepend-icon="mdi-identifier">
        <v-list-item-title class="wf-mono">{{ mode3aStr }}</v-list-item-title>
        <v-list-item-subtitle>Mode 3/A (Squawk)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="icaoLabel" prepend-icon="mdi-airplane">
        <v-list-item-title class="wf-mono">{{ icaoLabel }}</v-list-item-title>
        <v-list-item-subtitle>ICAO-Adresse (Mode S)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item prepend-icon="mdi-numeric">
        <v-list-item-title class="wf-mono">{{ track.track_num }}</v-list-item-title>
        <v-list-item-subtitle>Track-Nummer</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="accuracyLabel" prepend-icon="mdi-target-variant">
        <v-list-item-title class="wf-mono">{{ accuracyLabel }}</v-list-item-title>
        <v-list-item-subtitle>Positionsgenauigkeit</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="systemLabel" prepend-icon="mdi-server-network">
        <v-list-item-title class="wf-mono">{{ systemLabel }}</v-list-item-title>
        <v-list-item-subtitle>System (SAC/SIC)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="track.spi" prepend-icon="mdi-account-voice">
        <v-list-item-title>Ident aktiv</v-list-item-title>
        <v-list-item-subtitle>SPI-Puls — „squawk ident" (I062/080)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item v-if="track.mono" prepend-icon="mdi-alert-circle-outline">
        <v-list-item-title>Monosensor</v-list-item-title>
        <v-list-item-subtitle>nur eine Quelle — keine Kreuzprüfung (I062/080)</v-list-item-subtitle>
      </v-list-item>
      <v-list-item prepend-icon="mdi-information-outline">
        <v-list-item-title>{{ statusLabel }}</v-list-item-title>
        <v-list-item-subtitle>Status</v-list-item-subtitle>
      </v-list-item>
    </v-list>
  </v-card-text>

  <!-- #245 Teil B (ADR 0024): manual flight-plan correlation. Only rendered when
       the feature is enabled server-side (correlation_available) and this track
       rode a real catalogue feed (feed_id present — the ENV fallback feed has no
       Firefly command channel). The controller pins the track to a filed plan,
       marks it explicitly uncorrelated, or clears the override; the server
       authorises the write (must be subscribed to the feed) and relays it. -->
  <template v-if="correlationEnabled">
    <v-divider />
    <v-card-text class="pt-3 pb-3">
      <div class="text-overline mb-1">Korrelation</div>
      <v-text-field
        v-model="correlationCallsign"
        label="Plan-Callsign"
        density="compact"
        variant="outlined"
        hide-details
        autocapitalize="characters"
        class="mb-2"
        :disabled="correlationBusy"
        @keyup.enter="doCorrelate"
      />
      <div class="d-flex flex-wrap ga-2">
        <v-btn
          size="small"
          color="primary"
          variant="flat"
          :loading="correlationBusy"
          :disabled="correlationBusy || !correlationCallsign.trim()"
          @click="doCorrelate"
        >Korrelieren</v-btn>
        <v-btn size="small" variant="tonal" :disabled="correlationBusy" @click="doUncorrelate">
          Unkorreliert
        </v-btn>
        <v-btn size="small" variant="text" :disabled="correlationBusy" @click="doClear">
          Zurücksetzen
        </v-btn>
      </div>
      <v-alert
        v-if="correlationNotice"
        :type="correlationNotice.ok ? 'success' : 'warning'"
        variant="tonal"
        density="compact"
        class="mt-2"
      >{{ correlationNotice.message }}</v-alert>
    </v-card-text>
  </template>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { PROVENANCE_LABELS } from '@/map/provenance.js'
import {
  formatLatLon,
  formatHeading,
  formatIcao,
  formatAccuracy,
  formatAge,
  verticalTrendLabel,
  sensorAgeList,
  formatSelectedAltitude,
  formatMagneticHeading,
  formatIas,
  formatMach,
  isLevelBust,
  formatGeometricAltitude,
  formatBarometricAltitude,
  formatRateOfClimb,
  formatCourseTrend,
  formatSpeedTrend,
  formatAcceleration,
  formatPlanRoute,
  isPlanCallsignMismatch,
} from '@/map/trackDetail.js'

const emit = defineEmits(['close'])
const store = useAsdStore()
const track = computed(() => store.selectedTrack)

// WF2-40: surveillance source label, from the provenance baked onto the track
// feature when it was selected (see provenance.js / tracks.js).
const provenanceLabel = computed(() => PROVENANCE_LABELS[track.value?.provenance] ?? '')

const flLabel = computed(() => {
  if (track.value?.flight_level_ft == null) return ''
  return `FL${Math.round(track.value.flight_level_ft / 100)}`
})

// ASD-011: vertical tendency worded (Steigend/Sinkend/Gleichbleibend), with the
// glyph as a leading cue when it is climbing/descending.
const vTrendLabel = computed(() => verticalTrendLabel(track.value?.vertical_trend))
const vTrendGlyph = computed(() => {
  const t = track.value?.vertical_trend
  return t === '▲' || t === '▼' ? `${t} ` : ''
})

const groundSpeedKt = computed(() => {
  const t = track.value
  if (!t?.vx && !t?.vy) return null
  const kt = Math.round(Math.hypot(t.vx ?? 0, t.vy ?? 0) * 1.9438)
  return kt > 0 ? kt : null
})

// ASD-011: ground-track heading from Vx/Vy (I062/185).
const headingLabel = computed(() => formatHeading(track.value?.vx, track.value?.vy))

// ASD-011: WGS84 position (I062/105).
const positionLabel = computed(() => formatLatLon(track.value?.latitude, track.value?.longitude))

// ASD-011: technologies currently contributing, with freshness.
const sensorAges = computed(() => sensorAgeList(track.value))

const mode3aStr = computed(() => {
  if (track.value?.mode_3a == null) return ''
  return track.value.mode_3a.toString(8).padStart(4, '0')
})

// ASD-011: 24-bit Mode S address (I062/380) as hex.
const icaoLabel = computed(() => formatIcao(track.value?.icao_addr))

// ASD-011: estimated position accuracy (I062/500, metres).
const accuracyLabel = computed(() => formatAccuracy(track.value?.accuracy))

// ASD-011: system source (SAC/SIC, I062/010) that produced the track.
const systemLabel = computed(() => {
  const t = track.value
  if (t?.sac == null || t?.sic == null) return ''
  return `${t.sac}/${t.sic}`
})

// #238: Mode-S DAPs (I062/380). Selected Altitude is compared to the measured
// flight level for the level-bust signal; heading/IAS/Mach are informational.
const selectedAltitudeLabel = computed(() => formatSelectedAltitude(track.value?.selected_altitude_ft))
const levelBust = computed(() => isLevelBust(track.value?.selected_altitude_ft, track.value?.flight_level_ft))
const magneticHeadingLabel = computed(() => formatMagneticHeading(track.value?.magnetic_heading_deg))
const iasLabel = computed(() => formatIas(track.value?.ias_kt))
const machLabel = computed(() => formatMach(track.value?.mach))

// #241: vertical chain (I062/130/135/220, ICD 3.5.0). Barometric altitude is the
// filtered height (with QNH reference); geometric altitude is the WGS-84 value;
// rate is signed feet per minute. Absent when Firefly has no fresh estimate.
const baroAltitudeLabel = computed(() =>
  formatBarometricAltitude(track.value?.barometric_altitude_ft, track.value?.qnh_corrected),
)
const geometricAltitudeLabel = computed(() => formatGeometricAltitude(track.value?.geometric_altitude_ft))
const rocdLabel = computed(() => formatRateOfClimb(track.value?.rocd_ft_min))

// #242: Mode of Movement course/speed trend (I062/200), and calculated
// acceleration magnitude (I062/210). Each hidden when its axis/estimate is
// absent. Vertical movement is already conveyed by the Vertikaltendenz row
// (rate-driven, #241), so it is not repeated here.
const courseTrendLabel = computed(() => formatCourseTrend(track.value?.course_trend))
const speedTrendLabel = computed(() => formatSpeedTrend(track.value?.speed_trend))
const accelerationLabel = computed(() =>
  formatAcceleration(track.value?.accel_ax_ms2, track.value?.accel_ay_ms2),
)

// #245: flight-plan correlation (I062/390). The filed plan callsign and route
// (ADEP → ADES); a mismatch between the filed callsign and the downlinked
// identity (I062/245) is highlighted as an operational signal.
const planCallsign = computed(() => track.value?.plan_callsign ?? '')
const planRouteLabel = computed(() => formatPlanRoute(track.value?.plan_departure, track.value?.plan_destination))
const planCallsignMismatch = computed(() =>
  isPlanCallsignMismatch(track.value?.callsign, track.value?.plan_callsign),
)

const statusLabel = computed(() => {
  if (track.value?.coasting) return 'Coasting'
  if (track.value?.confirmed) return 'Bestätigt'
  return 'Tentativ'
})

// #245 Teil B (ADR 0024): manual flight-plan correlation controls. Enabled only
// when the backend has a command token (correlation_available, cosmetic — the
// server enforces it) AND this track carries a real feed_id (the ENV fallback
// feed, feed_id null, has no command channel).
const feedId = computed(() => track.value?.feed_id ?? null)
const correlationEnabled = computed(() => store.correlationAvailable && feedId.value != null)

const correlationCallsign = ref('')
const correlationBusy = ref(false)
const correlationNotice = ref(null)

// Pre-fill the field with the track's best-known identity (filed plan callsign,
// else the downlinked I062/245 callsign) and clear the last result whenever the
// selected track changes — so a notice never bleeds across selections.
// #272: the watch keys on the track NUMBER, not the track object — the live
// panel replaces the snapshot object on every WS update, and resetting the
// operator's typed callsign (or wiping a notice) mid-edit would be wrong.
watch(
  () => track.value?.track_num,
  () => {
    const t = track.value
    correlationCallsign.value = t?.plan_callsign || t?.callsign || ''
    correlationNotice.value = null
  },
  { immediate: true },
)

// runCorrelation drives one command: it disables the controls, awaits the store
// action (which returns { ok, message }), and shows the result. feedId is
// re-checked defensively — the controls are hidden without it, but a command
// must never post a null feed.
async function runCorrelation(fn) {
  if (feedId.value == null) return
  correlationBusy.value = true
  correlationNotice.value = null
  try {
    correlationNotice.value = await fn()
  } finally {
    correlationBusy.value = false
  }
}
function doCorrelate() {
  const cs = correlationCallsign.value.trim()
  if (!cs) return
  runCorrelation(() => store.correlate(feedId.value, track.value.track_num, cs))
}
function doUncorrelate() {
  runCorrelation(() => store.setUncorrelated(feedId.value, track.value.track_num))
}
function doClear() {
  runCorrelation(() => store.clearOverride(feedId.value, track.value.track_num))
}
</script>
