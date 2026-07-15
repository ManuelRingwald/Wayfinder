<template>
  <!-- Feed-health indicator (CAT065 heartbeat + CAT063 sensor status).
       Always visible so the operator can confirm the feed monitor is active.
       States:
         ok       (green)  = heartbeat fresh, all sensors operational
         degraded (yellow) = heartbeat fresh, but sensor fusion degraded (CAT063)
         stale    (red)    = heartbeat lost
         unknown  (grey)   = no heartbeat received yet (e.g. Firefly not running)
       #237: when CAT063 carries per-sensor detail worth showing (a degraded
       sensor or an applied registration bias), the chip becomes a menu that
       lists each sensor with its bias — "SIC 2 · Δr +145 m · Δθ +0,30°" — so the
       operator can spot a miscalibrating radar early. -->
  <v-menu v-if="detailSensors.length" location="bottom start">
    <template #activator="{ props }">
      <v-chip
        v-bind="props"
        :color="chipColor"
        size="small"
        variant="tonal"
        class="font-weight-bold feed-badge"
        prepend-icon="mdi-access-point"
        append-icon="mdi-chevron-down"
        :title="chipTitle"
      >
        {{ chipLabel }}
      </v-chip>
    </template>
    <v-card min-width="248" class="pa-2">
      <div class="text-caption text-medium-emphasis px-2 pb-1">
        Sensor-Registrierung (CAT063)
      </div>
      <v-list density="compact" class="pa-0">
        <v-list-item
          v-for="s in detailSensors"
          :key="`${s.feedId}-${s.sac}-${s.sic}`"
          :prepend-icon="s.operational ? 'mdi-radar' : 'mdi-alert-circle'"
        >
          <v-list-item-title class="text-body-2">{{ describeSensor(s) }}</v-list-item-title>
        </v-list-item>
      </v-list>
    </v-card>
  </v-menu>

  <v-chip
    v-else
    :color="chipColor"
    size="small"
    variant="tonal"
    class="font-weight-bold feed-badge"
    prepend-icon="mdi-access-point"
    :title="chipTitle"
  >
    {{ chipLabel }}
  </v-chip>
</template>

<script setup>
import { computed } from 'vue'
import { useAsdStore } from '@/stores/asd.js'
import { describeSensor, sensorNeedsAttention } from '@/admin/feedHealth.js'

const store = useAsdStore()

const chipColor = computed(() => ({
  ok: 'success',
  degraded: 'warning',
  stale: 'error',
  unknown: 'secondary',
}[store.feedStatus] ?? 'secondary'))

// Short German label for the CAT063 SRC-REASON (Firefly ADR 0033), appended to
// the degraded chip so the operator sees WHY a source is down (#197): a firewall
// (nicht erreichbar) needs no credential re-entry, unlike an auth failure.
const REASON_LABEL = {
  unreachable: 'NICHT ERREICHBAR',
  auth: 'AUTH-FEHLER',
  rate_limited: 'RATENLIMIT',
}
const REASON_TITLE = {
  unreachable: 'Quelle nicht erreichbar (Netz/Firewall) — die Zugangsdaten sind vermutlich in Ordnung.',
  auth: 'Authentifizierung fehlgeschlagen — Zugangsdaten der Quelle prüfen.',
  rate_limited: 'Quelle drosselt die Abfragen (Rate Limit) — Abfrageintervall erhöhen oder warten.',
}

const chipLabel = computed(() => {
  const base = {
    ok: 'FEED OK',
    degraded: 'SENSOR AUSFALL',
    stale: 'FEED STALE',
    unknown: 'FEED ?',
  }[store.feedStatus] ?? 'FEED ?'
  if (store.feedStatus === 'degraded' && REASON_LABEL[store.feedDegradedReason]) {
    return `${base} · ${REASON_LABEL[store.feedDegradedReason]}`
  }
  return base
})

const chipTitle = computed(() => {
  if (store.feedStatus === 'degraded') {
    return REASON_TITLE[store.feedDegradedReason]
      ?? 'Sensor-Teilausfall — mindestens eine Quelle liefert keine Daten.'
  }
  return undefined
})

// #237: the sensors worth listing in the expandable detail — a degraded sensor
// or one carrying an applied registration bias. An operational, unbiased sensor
// is omitted so the list stays signal.
const detailSensors = computed(() => store.sensorDetails.filter(sensorNeedsAttention))
</script>

<style scoped>
/* Design System v1: status badge — wide-tracked uppercase over a tonal fill,
   matching the scope's other status chips. */
.feed-badge {
  letter-spacing: 0.06em;
}
</style>
