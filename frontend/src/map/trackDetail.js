// Track-detail formatters (ASD-011): pure functions that turn the raw fields
// baked onto a selected track feature (see tracks.js) into the human-readable
// strings shown in the extended TrackDetailCard. Kept separate from the Vue
// component so the formatting logic is unit-testable in isolation (German UI
// labels per project charter §4; code identifiers stay English).
import { isAdsbFresh } from './provenance.js'

// formatLatLon renders a WGS84 position (I062/105) as decimal degrees with a
// hemisphere suffix, e.g. "53.6304° N, 9.9882° E". Returns '' when either
// component is missing, so the caller can hide the row.
export function formatLatLon(lat, lon) {
  if (typeof lat !== 'number' || typeof lon !== 'number') return ''
  const latH = lat >= 0 ? 'N' : 'S'
  const lonH = lon >= 0 ? 'E' : 'W'
  return `${Math.abs(lat).toFixed(4)}° ${latH}, ${Math.abs(lon).toFixed(4)}° ${lonH}`
}

// formatHeading derives the ground-track heading (degrees clockwise from true
// north) from the velocity components Vx/Vy (I062/185, East/North m/s) and
// renders it zero-padded, e.g. "042°". Returns '' for a stationary track
// (Vx=Vy=0), where a heading is undefined.
export function formatHeading(vx, vy) {
  if (typeof vx !== 'number' || typeof vy !== 'number') return ''
  if (vx === 0 && vy === 0) return ''
  const deg = (Math.atan2(vx, vy) * 180) / Math.PI
  const norm = (Math.round(deg) % 360 + 360) % 360
  return `${String(norm).padStart(3, '0')}°`
}

// formatIcao renders the 24-bit Mode S / ICAO aircraft address (I062/380) as a
// 6-digit uppercase hex string, e.g. "3C6DD2". Returns '' when absent.
export function formatIcao(icaoAddr) {
  if (icaoAddr == null) return ''
  return icaoAddr.toString(16).toUpperCase().padStart(6, '0')
}

// formatAccuracy renders the estimated 1-sigma position uncertainty (I062/500,
// metres) as "±N m". Returns '' for a missing or non-positive value.
export function formatAccuracy(accuracy) {
  if (typeof accuracy !== 'number' || !Number.isFinite(accuracy) || accuracy <= 0) return ''
  return `±${Math.round(accuracy)} m`
}

// formatAge renders a per-technology update age (seconds) compactly: one
// decimal below 10 s, whole seconds above. Returns '' for a missing value.
export function formatAge(ageS) {
  if (typeof ageS !== 'number' || !Number.isFinite(ageS)) return ''
  return `${ageS < 10 ? ageS.toFixed(1) : String(Math.round(ageS))} s`
}

// formatSelectedAltitude renders the Mode-S selected altitude (I062/380 SAL,
// #238) as a flight level "FLnnn", so it reads directly against the measured FL.
// Returns '' when absent.
export function formatSelectedAltitude(ft) {
  if (typeof ft !== 'number' || !Number.isFinite(ft)) return ''
  return `FL${String(Math.round(ft / 100)).padStart(3, '0')}`
}

// LEVEL_BUST_THRESHOLD_FT — how far the selected altitude may differ from the
// measured flight level before the ASD flags a mismatch (the level-bust signal,
// #238). 300 ft ≈ 3 FL: meaningfully different, not filter noise.
export const LEVEL_BUST_THRESHOLD_FT = 300

// isLevelBust reports whether the autopilot's selected altitude (I062/380 SAL)
// differs from the measured flight level (I062/136) by more than the threshold —
// the aircraft is heading to a different level than it is at. Both values must be
// present; a missing one is never flagged (fail-safe: never invent an alarm).
export function isLevelBust(selectedAltitudeFt, flightLevelFt, thresholdFt = LEVEL_BUST_THRESHOLD_FT) {
  if (typeof selectedAltitudeFt !== 'number' || typeof flightLevelFt !== 'number') return false
  return Math.abs(selectedAltitudeFt - flightLevelFt) >= thresholdFt
}

// formatMagneticHeading renders the Mode-S magnetic heading (I062/380 MHG) as a
// zero-padded compass value "270°". Returns '' when absent.
export function formatMagneticHeading(deg) {
  if (typeof deg !== 'number' || !Number.isFinite(deg)) return ''
  const norm = ((Math.round(deg) % 360) + 360) % 360
  return `${String(norm).padStart(3, '0')}°`
}

// formatIas renders the Mode-S indicated airspeed (I062/380 IAR) as "250 kt".
export function formatIas(kt) {
  if (typeof kt !== 'number' || !Number.isFinite(kt)) return ''
  return `${Math.round(kt)} kt`
}

// formatMach renders the Mode-S Mach number (I062/380 MAC) as "M0.784".
export function formatMach(m) {
  if (typeof m !== 'number' || !Number.isFinite(m)) return ''
  return `M${m.toFixed(3)}`
}

// formatGeometricAltitude renders the calculated geometric altitude (I062/130,
// WGS-84, ICD 3.5.0) as whole feet, e.g. "10000 ft". Returns '' when absent.
export function formatGeometricAltitude(ft) {
  if (typeof ft !== 'number' || !Number.isFinite(ft)) return ''
  return `${Math.round(ft)} ft`
}

// formatBarometricAltitude renders the filtered barometric altitude (I062/135,
// ICD 3.5.0) with its reference: a QNH-corrected value is a true altitude in
// feet ("3000 ft (QNH)"); an uncorrected value is a pressure flight level
// ("FL350 (Standard)"). The reference travels with the value so the two are
// never confused. Returns '' when absent.
export function formatBarometricAltitude(ft, qnhCorrected) {
  if (typeof ft !== 'number' || !Number.isFinite(ft)) return ''
  if (qnhCorrected === true) return `${Math.round(ft)} ft (QNH)`
  return `FL${String(Math.abs(Math.round(ft / 100))).padStart(3, '0')} (Standard)`
}

// formatRateOfClimb renders the calculated rate of climb/descent (I062/220,
// ICD 3.5.0) as signed feet per minute, e.g. "+3000 ft/min" / "-1200 ft/min".
// Returns '' when absent.
export function formatRateOfClimb(ftMin) {
  if (typeof ftMin !== 'number' || !Number.isFinite(ftMin)) return ''
  const r = Math.round(ftMin)
  return `${r > 0 ? '+' : ''}${r} ft/min`
}

// Mode of Movement (I062/200, ICD 3.6.0, #242): the three qualitative motion
// axes worded for the detail panel. Each returns '' for an absent/undetermined
// axis so the caller hides the row (Firefly omits an axis it cannot determine).
const COURSE_TREND_LABELS = { right: 'Rechtskurve', left: 'Linkskurve', constant: 'Konstanter Kurs' }
const SPEED_TREND_LABELS = { increasing: 'Zunehmend', decreasing: 'Abnehmend', constant: 'Konstant' }
const VERTICAL_MOTION_LABELS = { climb: 'Steigen', descent: 'Sinken', level: 'Level' }

export function formatCourseTrend(trend) {
  return COURSE_TREND_LABELS[trend] ?? ''
}

export function formatSpeedTrend(trend) {
  return SPEED_TREND_LABELS[trend] ?? ''
}

export function formatVerticalMotion(trend) {
  return VERTICAL_MOTION_LABELS[trend] ?? ''
}

// formatAcceleration renders the calculated horizontal acceleration magnitude
// (I062/210, ICD 3.6.0) from its Cartesian components Ax/Ay (m/s²), e.g.
// "1.1 m/s²". Returns '' when either component is missing.
export function formatAcceleration(axMs2, ayMs2) {
  if (typeof axMs2 !== 'number' || typeof ayMs2 !== 'number') return ''
  if (!Number.isFinite(axMs2) || !Number.isFinite(ayMs2)) return ''
  return `${Math.hypot(axMs2, ayMs2).toFixed(1)} m/s²`
}

// formatPlanRoute renders the filed flight-plan route (I062/390 DEP/DST, ICD
// 3.7.0) as "EDDF → EDDM". A missing endpoint is shown as "—" so a one-sided plan
// still reads as a route; returns '' when neither endpoint is present.
export function formatPlanRoute(departure, destination) {
  const dep = typeof departure === 'string' && departure !== '' ? departure : null
  const dst = typeof destination === 'string' && destination !== '' ? destination : null
  if (dep === null && dst === null) return ''
  return `${dep ?? '—'} → ${dst ?? '—'}`
}

// isPlanCallsignMismatch reports whether the filed plan callsign (I062/390 CSN)
// differs from the downlinked identity (I062/245) — both must be present. This is
// an operational signal (the aircraft squawks a callsign other than its filed
// plan), surfaced as a highlight; a missing side is never flagged (fail-safe).
export function isPlanCallsignMismatch(downlinkedCallsign, planCallsign) {
  if (typeof downlinkedCallsign !== 'string' || downlinkedCallsign === '') return false
  if (typeof planCallsign !== 'string' || planCallsign === '') return false
  return downlinkedCallsign !== planCallsign
}

// VERTICAL_TREND_LABELS maps the tendency glyph baked in tracks.js (ASD-001b:
// ▲ climbing, ▼ descending) to a German word. Anything else (including '') is
// treated as level flight.
export const VERTICAL_TREND_LABELS = { '▲': 'Steigend', '▼': 'Sinkend' }

// verticalTrendLabel turns the baked tendency glyph into a word. Only meaningful
// when a flight level is known; the caller gates the row on that.
export function verticalTrendLabel(trend) {
  return VERTICAL_TREND_LABELS[trend] ?? 'Gleichbleibend'
}

// SENSOR_AGES lists the per-technology update ages Firefly emits from I062/290
// (ICD 2.6.0), in display order. PSR is intentionally absent: psr_age is always
// present on the wire and carries no clean per-track freshness semantics, so the
// primary-only case is represented by the "Herkunft" (provenance) row instead.
const SENSOR_AGES = [
  { key: 'adsb_age_s', label: 'ADS-B' },
  { key: 'flarm_age_s', label: 'FLARM' },
  { key: 'ssr_age_s', label: 'SSR (Mode A/C)' },
  { key: 'mds_age_s', label: 'Mode S' },
]

// sensorAgeList returns the technologies that currently contribute to a track,
// each with its update age (seconds) and a freshness flag (same window as the
// provenance classifier). Only technologies whose age is present are included,
// so a primary-only track yields an empty list and the caller hides the section.
export function sensorAgeList(track) {
  if (track == null) return []
  return SENSOR_AGES.filter((s) => typeof track[s.key] === 'number').map((s) => ({
    key: s.key,
    label: s.label,
    ageS: track[s.key],
    fresh: isAdsbFresh(track[s.key]),
  }))
}
