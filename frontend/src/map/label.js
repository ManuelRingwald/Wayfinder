// buildLabel produces the track's ASD data-block label (ASD-001).
//   Line 1: callsign (I062/245) or track number as fallback.
//   Line 2: "FLnnn" (flight level, I062/136) + vertical-tendency indicator
//            (▲ climbing / ▼ descending / empty for level), when FL is known.
//   Line 3: ground speed in knots (from Vx/Vy, I062/185), when non-zero.
// vTrend is "▲", "▼", or "" — computed by updateTracksLayer (ASD-001b).
//
// MON (I062/080, ICD 3.2.0): a single-sensor track carries no cross-check, so
// the data block flags it discreetly with a trailing "*" on the identity line
// (spelled out in the detail panel). An ordinary multi-sensor track is unmarked.
export function buildLabel(track, vTrend) {
  const monoMark = track.mono === true ? '*' : ''
  const ident =
    typeof track.callsign === 'string' && track.callsign !== ''
      ? track.callsign
      : String(track.track_num)
  const line1 = `${ident}${monoMark}`

  // Ground speed: sqrt(Vx²+Vy²) m/s → kt (1 m/s ≈ 1.9438 kt).
  const gs = Math.round(Math.hypot(track.vx, track.vy) * 1.9438)
  const gsLine = gs > 0 ? `\n${gs}` : ''

  // I062/380 selected altitude (#238): shown next to the FL as "S<selFL>" so the
  // controller reads the autopilot's target against the actual level at a glance
  // (a mismatch is the level-bust signal). Only when a measured FL is also known.
  const sel = typeof track.selected_altitude_ft === 'number'
    ? ` S${String(Math.round(track.selected_altitude_ft / 100)).padStart(3, '0')}`
    : ''

  if (typeof track.flight_level_ft === 'number') {
    const fl = Math.round(track.flight_level_ft / 100)
    const trend = vTrend ? ` ${vTrend}` : ''
    return `${line1}\nFL${String(fl).padStart(3, '0')}${trend}${sel}${gsLine}`
  }
  return `${line1}${gsLine}`
}
