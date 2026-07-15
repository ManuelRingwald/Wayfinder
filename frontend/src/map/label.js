// buildLabel produces the track's ASD data-block label (ASD-001).
//   Line 1: callsign (I062/245) or track number as fallback.
//   Line 2: altitude block + vertical-tendency indicator (▲ climbing /
//            ▼ descending / empty for level), when a height is known. The block
//            prefers the filtered barometric altitude (I062/135, ICD 3.5.0):
//            "Annn" when it is corrected to an observed regional QNH (a true
//            altitude), "FLnnn" when it is an uncorrected pressure level. Falls
//            back to the measured flight level (I062/136) as "FLnnn".
//   Line 3: ground speed in knots (from Vx/Vy, I062/185), when non-zero.
// vTrend is "▲", "▼", or "" — computed by updateTracksLayer (ASD-001b).
//
// MON (I062/080, ICD 3.2.0): a single-sensor track carries no cross-check, so
// the data block flags it discreetly with a trailing "*" on the identity line
// (spelled out in the detail panel). An ordinary multi-sensor track is unmarked.
//
// Turn indicator (I062/200 TRANS, ICD 3.6.0): a right/left turn adds "→"/"←" to
// the identity line so a manoeuvring aircraft stands out; a constant or
// undetermined course adds nothing.
//
// Callsign mismatch (I062/390 CSN vs I062/245, ICD 3.7.0): when the filed plan
// callsign differs from the downlinked identity, a trailing "≠" flags it — a real
// operational signal (wrong squawk/plan); the detail panel spells it out.
export function buildLabel(track, vTrend) {
  const monoMark = track.mono === true ? '*' : ''
  const ident =
    typeof track.callsign === 'string' && track.callsign !== ''
      ? track.callsign
      : String(track.track_num)
  const turn =
    track.course_trend === 'right' ? ' →' : track.course_trend === 'left' ? ' ←' : ''
  const mismatch =
    typeof track.callsign === 'string' && track.callsign !== '' &&
    typeof track.plan_callsign === 'string' && track.plan_callsign !== '' &&
    track.callsign !== track.plan_callsign
      ? '≠'
      : ''
  const line1 = `${ident}${monoMark}${mismatch}${turn}`

  // Ground speed: sqrt(Vx²+Vy²) m/s → kt (1 m/s ≈ 1.9438 kt).
  const gs = Math.round(Math.hypot(track.vx, track.vy) * 1.9438)
  const gsLine = gs > 0 ? `\n${gs}` : ''

  // I062/380 selected altitude (#238): shown next to the FL as "S<selFL>" so the
  // controller reads the autopilot's target against the actual level at a glance
  // (a mismatch is the level-bust signal). Only when a measured FL is also known.
  const sel = typeof track.selected_altitude_ft === 'number'
    ? ` S${String(Math.round(track.selected_altitude_ft / 100)).padStart(3, '0')}`
    : ''

  // Preferred display height: the filtered barometric altitude (I062/135) is
  // smoother than the jumpier measured flight level, and its QNH-correction flag
  // tells the controller the reference — "A" for a QNH altitude, "FL" for a
  // pressure level. The digits are hundreds of feet (padded to 3), the ASD
  // convention; the exact signed value lives in the detail panel.
  let altBlock = null
  if (typeof track.barometric_altitude_ft === 'number') {
    const digits = String(Math.abs(Math.round(track.barometric_altitude_ft / 100))).padStart(3, '0')
    altBlock = track.qnh_corrected === true ? `A${digits}` : `FL${digits}`
  } else if (typeof track.flight_level_ft === 'number') {
    const fl = Math.round(track.flight_level_ft / 100)
    altBlock = `FL${String(Math.abs(fl)).padStart(3, '0')}`
  }

  if (altBlock !== null) {
    const trend = vTrend ? ` ${vTrend}` : ''
    return `${line1}\n${altBlock}${trend}${sel}${gsLine}`
  }
  return `${line1}${gsLine}`
}
