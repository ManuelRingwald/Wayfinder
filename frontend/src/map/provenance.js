// Track provenance (WF2-40): infer the surveillance source of a system track
// from the CAT062 fields Wayfinder already decodes, so the scope can show the
// data origin at a glance (symbol SHAPE; the colour keeps encoding track state).
//
// HONEST LIMIT: this is a *track-derived* classification, not a certified
// per-plot provenance — CAT062 does not carry an explicit per-sensor source on
// the wire (that would need a Firefly ICD change, tracked as WF2-42). We only
// reason from the items the contract already gives us.
//
// Precedence (most authoritative *current* cooperative source first):
//   adsb — an ADS-B (Extended Squitter) component is currently feeding the
//          track: I062/290 ES-age subfield present (adsb_age_s, ICD 2.4.0) AND
//          fresh (≤ ADSB_FRESH_THRESHOLD_S). A stale ADS-B age means the
//          self-report link went quiet, so the track falls back to its
//          remaining cooperative/primary source — this mirrors the freshness
//          rule of the original ADS-B data-block badge (former FR-ASD-006,
//          which WF2-40 reinstates as a symbol shape).
//   ssr  — a cooperative secondary reply identifies the track: Mode S address
//          (I062/380 → icao_addr), Mode 3/A code (I062/060 → mode_3a) or a
//          Mode S identification / callsign (I062/245).
//   psr  — none of the above: primary-only skin paint (position without ID).

export const PROVENANCE_ADSB = 'adsb'
export const PROVENANCE_SSR = 'ssr'
export const PROVENANCE_PSR = 'psr'

// ADS-B freshness window in seconds. Beyond this, an ADS-B contribution is
// considered no longer current (kept identical to the original FR-ASD-006
// data-block badge threshold so behaviour stays consistent across the port).
export const ADSB_FRESH_THRESHOLD_S = 30

// isAdsbFresh reports whether an adsb_age_s value (seconds since the last ADS-B
// hit) is present and still within the freshness window. Presence is tested
// with != null so that a zero age (a brand-new ADS-B update) counts as fresh.
export function isAdsbFresh(adsbAgeS) {
  return adsbAgeS != null && adsbAgeS <= ADSB_FRESH_THRESHOLD_S
}

// trackProvenance returns 'adsb' | 'ssr' | 'psr' for a WS track message
// (see pkg/broadcast.TrackMessage). Optional contract fields are absent (not
// null-valued) when not sent, so presence is tested with != null.
export function trackProvenance(track) {
  if (track == null) return PROVENANCE_PSR
  if (isAdsbFresh(track.adsb_age_s)) return PROVENANCE_ADSB
  if (
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
  [PROVENANCE_ADSB]: 'ADS-B (kooperativ)',
  [PROVENANCE_SSR]: 'SSR / Mode S',
  [PROVENANCE_PSR]: 'Primär (PSR)',
}
