// Shared feed-health presentation (AP4 + status granularity).
//
// The backend computes the traffic-light `color` (green/yellow/red) in
// pkg/health `FeedSnapshot.Color()`; this module maps that colour to a Vuetify
// colour plus a human `label`/`title` for the admin feed chips. It is used by
// AdminFeeds, AdminTenantDetail and AdminTenants so the three chips read
// identically (previously each duplicated the mapping).
//
// The red case is split into two operator-distinct sub-states using raw
// snapshot fields the DTO already carries (no backend/wire change):
//   - `!ever_seen`         → "nie gestartet": no CAT065 heartbeat has *ever*
//     arrived. Points at assignment / orchestrator spawn / source (the feed was
//     never live) — not at a feed that died.
//   - `ever_seen && stale` → "abgerissen": a heartbeat was seen but has gone
//     stale (Firefly stopped / network). `last_heartbeat_ago_s` dates it.
// Presentation only — no colour is recomputed here.

const VUETIFY_COLOR = { green: 'success', yellow: 'warning', red: 'error' }

// describeFeedHealth maps a per-feed health snapshot (the admin `feedsHealth`
// DTO entry) to { color, label, title }. A missing snapshot (feed not yet
// reported) is "unbekannt".
export function describeFeedHealth(h) {
  if (!h) {
    return { color: 'default', label: 'unbekannt', title: 'Gesundheit unbekannt' }
  }
  const color = VUETIFY_COLOR[h.color] ?? 'default'

  if (h.color === 'green') {
    // Healthy heartbeat: distinguish traffic from an (equally healthy) empty
    // sky, and append the sensor share when CAT063 is present.
    const parts = [h.track_count_recent > 0 ? `${h.track_count_recent} Tracks` : 'leerer Himmel']
    if (h.sensors_total > 0) parts.push(`${h.sensors_active}/${h.sensors_total} Radare`)
    return { color, label: 'OK', title: `OK · ${parts.join(' · ')}` }
  }

  if (h.color === 'yellow') {
    return {
      color,
      label: 'degradiert',
      title: h.sensors_total > 0
        ? `Sensor-Teilausfall: ${h.sensors_active} von ${h.sensors_total} Radaren aktiv`
        : 'Sensor-Teilausfall',
    }
  }

  // red — split "never started" from "went stale".
  if (!h.ever_seen) {
    return {
      color,
      label: 'nie gestartet',
      title: 'Kein Heartbeat empfangen — Feed nie angelaufen (Zuweisung/Spawn/Quelle prüfen)',
    }
  }
  const ago = Number.isFinite(h.last_heartbeat_ago_s) && h.last_heartbeat_ago_s >= 0
    ? ` — seit ${Math.round(h.last_heartbeat_ago_s)} s kein CAT065`
    : ''
  return { color, label: 'abgerissen', title: `Heartbeat abgerissen${ago}` }
}

// SENSOR_REASON_LABEL maps the CAT063 SRC-REASON (Firefly ADR 0033) to a short
// German label for a per-sensor row. Kept here (not only in FeedStatusChip) so
// the operational chip and the admin feed views word a degraded sensor identically.
const SENSOR_REASON_LABEL = {
  unreachable: 'nicht erreichbar',
  auth: 'Auth-Fehler',
  rate_limited: 'Ratenlimit',
}

// formatSensorBias renders one sensor's applied registration bias (#237) as a
// compact "Δr +145 m · Δθ +0,30°" string (I063/080 SRB metres, I063/081 SAB
// degrees). Each component is omitted when absent; returns "" when the sensor
// carries no correction at all (nothing in force — never shown as 0). A leading
// "+" marks a non-negative value; the minus sign comes from the number itself.
export function formatSensorBias(s) {
  if (!s) return ''
  const parts = []
  if (typeof s.range_bias_m === 'number') {
    const r = Math.round(s.range_bias_m)
    parts.push(`Δr ${r >= 0 ? '+' : ''}${r} m`)
  }
  if (typeof s.azimuth_bias_deg === 'number') {
    const a = s.azimuth_bias_deg
    parts.push(`Δθ ${a >= 0 ? '+' : ''}${a.toFixed(2)}°`)
  }
  return parts.join(' · ')
}

// describeSensor produces the one-line label for a sensor in a per-sensor detail
// list (#237). It always leads with the SIC (the operator's sensor id), then the
// degraded reason (if any) and the applied bias (if any): e.g.
// "SIC 2 · nicht erreichbar" or "SIC 1 · Δr +145 m · Δθ +0,30°".
export function describeSensor(s) {
  if (!s) return ''
  const state = s.operational ? '' : (SENSOR_REASON_LABEL[s.degraded_reason] ?? 'Ausfall')
  const tail = [state, formatSensorBias(s)].filter(Boolean).join(' · ')
  return tail ? `SIC ${s.sic} · ${tail}` : `SIC ${s.sic}`
}

// sensorNeedsAttention reports whether a sensor is worth listing in the detail
// view: it is degraded, or it carries an applied registration bias. An
// operational sensor with no correction is omitted (keeps the list to signal).
export function sensorNeedsAttention(s) {
  return !!s && (!s.operational
    || typeof s.range_bias_m === 'number'
    || typeof s.azimuth_bias_deg === 'number')
}
