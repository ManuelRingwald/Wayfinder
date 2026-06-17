// buildLabel produces the track's ASD data-block label (ASD-001).
//   Line 1: callsign (I062/245) or track number as fallback.
//   Line 2: "FLnnn" (flight level, I062/136) + vertical-tendency indicator
//            (▲ climbing / ▼ descending / empty for level), when FL is known.
//   Line 3: ground speed in knots (from Vx/Vy, I062/185), when non-zero.
// vTrend is "▲", "▼", or "" — computed by updateTracksLayer (ASD-001b).
export function buildLabel(track, vTrend) {
  const line1 =
    typeof track.callsign === 'string' && track.callsign !== ''
      ? track.callsign
      : String(track.track_num)

  // Ground speed: sqrt(Vx²+Vy²) m/s → kt (1 m/s ≈ 1.9438 kt).
  const gs = Math.round(Math.hypot(track.vx, track.vy) * 1.9438)
  const gsLine = gs > 0 ? `\n${gs}` : ''

  if (typeof track.flight_level_ft === 'number') {
    const fl = Math.round(track.flight_level_ft / 100)
    const trend = vTrend ? ` ${vTrend}` : ''
    return `${line1}\nFL${String(fl).padStart(3, '0')}${trend}${gsLine}`
  }
  return `${line1}${gsLine}`
}
