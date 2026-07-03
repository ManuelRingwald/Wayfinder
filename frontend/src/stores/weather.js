import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiFetch } from '@/api.js'

// QNH poll cadence (WX-B, ADR 0016). QNH (from METAR) changes on the order of
// ~30 min, so a slow poll is plenty — well under the upstream's fair-use limit,
// which the backend proxy already enforces (the browser only hits Wayfinder).
export const QNH_POLL_INTERVAL_MS = 5 * 60 * 1000 // 5 min

// useWeatherStore backs the QNH header infobox. It polls the backend proxy
// (/api/weather/qnh), which returns the current QNH for the configured
// aerodrome(s). Best-effort: an empty/absent value simply hides the infobox; the
// display is gated per tenant by the `qnh` feature in AsdHeader. QNH comes only
// from a real METAR (never DWD PMSL) and is shown as whole hPa (cockpit/ATC
// convention); a stale reading is flagged rather than hidden so the operator sees
// that the value is old rather than silently trusting it.
export const useWeatherStore = defineStore('weather', () => {
  const stations = ref([]) // [{ icao, qnh_hpa, obs_time, stale }]
  const primary = ref(null) // the header station (first configured with a reading)
  let timer = null

  // available is true once the backend has at least one QNH reading to show.
  const available = computed(() => primary.value != null)

  function applyPayload(data) {
    stations.value = Array.isArray(data?.stations) ? data.stations : []
    primary.value = data?.primary ?? null
  }

  async function poll() {
    const res = await apiFetch('/api/weather/qnh')
    // Best-effort: on any error keep the last-good value rather than blanking it.
    if (res.ok && res.data) {
      applyPayload(res.data)
    }
    return res
  }

  // start polls immediately and then on an interval. Idempotent (a second start
  // clears the previous timer) so remounts never stack pollers.
  function start(intervalMs = QNH_POLL_INTERVAL_MS) {
    stop()
    poll()
    timer = setInterval(poll, intervalMs)
  }

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  return { stations, primary, available, applyPayload, poll, start, stop }
})
