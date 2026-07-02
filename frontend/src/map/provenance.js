// Track provenance (WF2-40, #118): infer the surveillance source of a system
// track from the CAT062 fields Wayfinder decodes, so the scope can show the
// data origin at a glance (symbol GLYPH; the colour keeps encoding track state).
//
// Since ICD 2.6.0 (Firefly ADR 0027) I062/290 carries authoritative
// per-technology update ages (PSR/SSR/MDS/ES/FLARM), broadcast to the browser
// as psr_age / ssr_age_s / mds_age_s / adsb_age_s / flarm_age_s. FLARM is
// therefore cleanly distinguishable from ADS-B for the first time (#118).
//
// Precedence (most authoritative first):
//   combined — ≥2 distinct surveillance technologies are *currently fresh*
//           (any two of ES/ADS-B, FLARM, SSR Mode A/C, Mode S). A multi-sensor
//           fused track is the highest-quality picture, so it gets its own glyph
//           rather than being reduced to a single source (#125, from #90).
//   adsb  — a fresh ES (Extended Squitter) age: adsb_age_s present AND
//           ≤ ADSB_FRESH_THRESHOLD_S. A stale age means the self-report link
//           went quiet, so the track falls back to its remaining sources.
//           ADS-B outranks FLARM when both are fresh (the ICAO-standardised,
//           richer report wins the single-glyph slot).
//   flarm — a fresh FLARM age (flarm_age_s, Firefly vendor subfield).
//   ssr   — a cooperative secondary reply identifies the track: a fresh SSR /
//           Mode S age (I062/290), or the legacy id fields Mode S address
//           (I062/380 → icao_addr), Mode 3/A code (I062/060 → mode_3a) or a
//           Mode S identification / callsign (I062/245).
//   psr   — none of the above: primary-only skin paint (position without ID).
//
// The classification is re-derived on every WS update in tracks.js (never
// cached on the track), so a source change corrects the glyph immediately.
//
// HONEST LIMIT: "combined" counts the per-technology ages Firefly actually
// emits (ICD 2.6.0). How often it triggers depends on Firefly's emission — the
// ≥2 threshold is the agreed definition (#90) and can be tuned against live data.

export const PROVENANCE_ADSB = 'adsb'
export const PROVENANCE_FLARM = 'flarm'
export const PROVENANCE_SSR = 'ssr'
export const PROVENANCE_PSR = 'psr'
export const PROVENANCE_COMBINED = 'combined'

// ADS-B freshness window in seconds. Beyond this, an ADS-B contribution is
// considered no longer current (kept identical to the original FR-ASD-006
// data-block badge threshold so behaviour stays consistent across the port).
export const ADSB_FRESH_THRESHOLD_S = 30

// isAdsbFresh reports whether an age value (seconds since the last hit of that
// technology) is present and still within the freshness window. Presence is
// tested with != null so that a zero age (a brand-new update) counts as fresh.
// Used for both the ES/ADS-B and the FLARM age (same window).
export function isAdsbFresh(ageS) {
  return ageS != null && ageS <= ADSB_FRESH_THRESHOLD_S
}

// trackProvenance returns 'combined' | 'adsb' | 'flarm' | 'ssr' | 'psr' for a WS
// track message (see pkg/broadcast.TrackMessage). Optional contract fields are
// absent (not null-valued) when not sent, so presence is tested with != null.
export function trackProvenance(track) {
  if (track == null) return PROVENANCE_PSR
  // Count the distinct surveillance technologies currently fresh (ICD 2.6.0
  // per-technology ages). ≥2 → a genuine multi-sensor fusion (#125).
  const freshTechs = [
    track.adsb_age_s,
    track.flarm_age_s,
    track.ssr_age_s,
    track.mds_age_s,
  ].filter((age) => isAdsbFresh(age)).length
  if (freshTechs >= 2) return PROVENANCE_COMBINED
  if (isAdsbFresh(track.adsb_age_s)) return PROVENANCE_ADSB
  if (isAdsbFresh(track.flarm_age_s)) return PROVENANCE_FLARM
  if (
    isAdsbFresh(track.ssr_age_s) ||
    isAdsbFresh(track.mds_age_s) ||
    track.icao_addr != null ||
    track.mode_3a != null ||
    (typeof track.callsign === 'string' && track.callsign !== '')
  ) {
    return PROVENANCE_SSR
  }
  return PROVENANCE_PSR
}

// PROVENANCE_LABELS: human-readable German labels for the detail panel and the
// scope legend (German per project charter §4).
export const PROVENANCE_LABELS = {
  [PROVENANCE_COMBINED]: 'Kombiniert (Mehr-Sensor)',
  [PROVENANCE_ADSB]: 'ADS-B (kooperativ)',
  [PROVENANCE_FLARM]: 'FLARM',
  [PROVENANCE_SSR]: 'SSR / Mode S',
  [PROVENANCE_PSR]: 'Primär (PSR)',
}
