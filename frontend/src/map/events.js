// Event derivation (ASD-013): pure functions that turn observable transitions in
// the WS stream — feed health, WebSocket connection, track lifecycle — into
// operator-facing event records for the Alarm-/Ereignis-Panel. Kept pure (no
// Date, no store, no side effects) so the derivation is unit-testable in
// isolation; the store stamps id/timestamp and the component renders.
//
// No wire change: every signal here already exists in the stream Wayfinder
// consumes (CAT062 tracks + TSE, CAT065 feed status). The event log is therefore
// automatically tenant-scoped, because the WS stream already is (WF2-21).
//
// German UI text per project charter §4; code identifiers stay English.

// Severity levels — mapped to Vuetify colours / MDI icons via SEVERITY_META.
export const SEV_INFO = 'info'
export const SEV_WARN = 'warning'
export const SEV_ERROR = 'error'
export const SEV_SUCCESS = 'success'

// SEVERITY_META drives the panel's per-row icon and colour. Kept here (single
// source) so the store, panel and badge stay consistent.
export const SEVERITY_META = {
  [SEV_INFO]: { icon: 'mdi-information-outline', color: 'info' },
  [SEV_WARN]: { icon: 'mdi-alert-outline', color: 'warning' },
  [SEV_ERROR]: { icon: 'mdi-alert-circle-outline', color: 'error' },
  [SEV_SUCCESS]: { icon: 'mdi-check-circle-outline', color: 'success' },
}

// feedStatusEvent derives an event from a change in the aggregate feed health
// (asd store `feedStatus`: 'unknown' | 'ok' | 'degraded' | 'stale', worst across
// feeds). Returns null when nothing operationally worth logging changed. The
// benign initial climb to a healthy feed on (re)connect (unknown → ok) is
// suppressed so the log does not open with a spurious "recovered".
export function feedStatusEvent(prev, curr) {
  if (prev === curr) return null
  if (curr == null || curr === 'unknown') return null
  if ((prev == null || prev === 'unknown') && curr === 'ok') return null
  if (curr === 'stale') {
    return { type: 'feed-stale', severity: SEV_ERROR, message: 'Feed ausgefallen (keine Quelle aktuell)' }
  }
  if (curr === 'degraded') {
    return { type: 'feed-degraded', severity: SEV_WARN, message: 'Feed degradiert' }
  }
  return { type: 'feed-recovered', severity: SEV_SUCCESS, message: 'Feed wiederhergestellt' }
}

// connectionEvent derives an event from a WebSocket lifecycle change
// ('open' | 'closed'). The very first connect (prev null → 'open') is silent;
// only a genuine drop and a subsequent recovery are logged.
export function connectionEvent(prev, curr) {
  if (prev === curr) return null
  if (curr === 'closed') {
    return { type: 'connection-lost', severity: SEV_ERROR, message: 'Verbindung zum ASD-Server verloren' }
  }
  if (curr === 'open' && prev === 'closed') {
    return { type: 'connection-restored', severity: SEV_SUCCESS, message: 'Verbindung wiederhergestellt' }
  }
  return null
}

// trackLifecycleEvents derives track appeared/disappeared events from the diff
// between the previous and current live track-number sets plus the TSE-ended set
// of a single WS batch:
//   appeared     — a track number now live that was not live before.
//   disappeared  — a track number explicitly ended (I062/080 TSE); this is the
//                  authoritative "track deleted" signal, so a mere gap in a scan
//                  (a number dropping out without a TSE) is deliberately NOT
//                  reported, to keep the log free of transient-miss noise.
// prevNums/currNums/endedNums may be arrays or Sets. Track events are info-level.
export function trackLifecycleEvents(prevNums, currNums, endedNums = []) {
  const prev = prevNums instanceof Set ? prevNums : new Set(prevNums || [])
  const events = []
  for (const n of currNums || []) {
    if (!prev.has(n)) {
      events.push({ type: 'track-appeared', severity: SEV_INFO, message: `Track ${n} erschienen`, trackNum: n })
    }
  }
  for (const n of endedNums || []) {
    events.push({ type: 'track-disappeared', severity: SEV_INFO, message: `Track ${n} beendet`, trackNum: n })
  }
  return events
}
