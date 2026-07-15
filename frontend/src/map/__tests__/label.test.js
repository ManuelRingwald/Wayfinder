import { describe, it, expect } from 'vitest'
import { buildLabel } from '../label.js'

describe('buildLabel', () => {
  it('uses callsign as line 1 when present', () => {
    const track = { callsign: 'DLH123', track_num: 42, vx: 0, vy: 0 }
    const label = buildLabel(track, '')
    expect(label.split('\n')[0]).toBe('DLH123')
  })

  it('falls back to track_num when callsign is absent', () => {
    const track = { track_num: 42, vx: 0, vy: 0 }
    const label = buildLabel(track, '')
    expect(label.split('\n')[0]).toBe('42')
  })

  it('falls back to track_num when callsign is empty string', () => {
    const track = { callsign: '', track_num: 99, vx: 0, vy: 0 }
    const label = buildLabel(track, '')
    expect(label.split('\n')[0]).toBe('99')
  })

  it('shows FL line when flight_level_ft is present', () => {
    const track = { callsign: 'BAW1', track_num: 1, vx: 0, vy: 0, flight_level_ft: 35000 }
    const label = buildLabel(track, '')
    const lines = label.split('\n')
    expect(lines[1]).toMatch(/^FL\d+/)
    expect(lines[1]).toBe('FL350')
  })

  it('pads FL to 3 digits', () => {
    const track = { callsign: 'TST', track_num: 1, vx: 0, vy: 0, flight_level_ft: 500 }
    const label = buildLabel(track, '')
    expect(label.split('\n')[1]).toBe('FL005')
  })

  it('omits FL line when flight_level_ft is absent', () => {
    const track = { callsign: 'TST', track_num: 1, vx: 0, vy: 0 }
    const label = buildLabel(track, '')
    const lines = label.split('\n').filter(Boolean)
    expect(lines.every(l => !l.startsWith('FL'))).toBe(true)
  })

  it('appends climb indicator ▲ when vTrend is ▲', () => {
    const track = { callsign: 'TST', track_num: 1, vx: 0, vy: 0, flight_level_ft: 10000 }
    const label = buildLabel(track, '▲')
    expect(label).toContain('▲')
  })

  it('appends descent indicator ▼ when vTrend is ▼', () => {
    const track = { callsign: 'TST', track_num: 1, vx: 0, vy: 0, flight_level_ft: 10000 }
    const label = buildLabel(track, '▼')
    expect(label).toContain('▼')
  })

  it('shows no trend indicator when vTrend is empty', () => {
    const track = { callsign: 'TST', track_num: 1, vx: 0, vy: 0, flight_level_ft: 10000 }
    const label = buildLabel(track, '')
    expect(label).not.toContain('▲')
    expect(label).not.toContain('▼')
  })

  it('shows ground speed line when speed > 0', () => {
    // vx=100 m/s, vy=0 → gs = 100 * 1.9438 ≈ 194 kt
    const track = { callsign: 'TST', track_num: 1, vx: 100, vy: 0 }
    const label = buildLabel(track, '')
    const lines = label.split('\n')
    expect(lines.length).toBeGreaterThanOrEqual(2)
    const gsLine = lines[lines.length - 1]
    expect(Number(gsLine)).toBeGreaterThan(0)
  })

  it('omits ground speed line when speed is 0', () => {
    const track = { callsign: 'TST', track_num: 1, vx: 0, vy: 0 }
    const label = buildLabel(track, '')
    // Only line 1 (callsign), no GS line
    expect(label).toBe('TST')
  })

  it('shows all three lines when callsign, FL and speed are all present', () => {
    const track = { callsign: 'DLH99', track_num: 5, vx: 150, vy: 0, flight_level_ft: 25000 }
    const label = buildLabel(track, '▲')
    const lines = label.split('\n')
    expect(lines[0]).toBe('DLH99')
    expect(lines[1]).toMatch(/FL250 ▲/)
    expect(Number(lines[2])).toBeGreaterThan(0)
  })

  it('appends the MON marker "*" to the identity line for a mono-sensor track (#236)', () => {
    const track = { callsign: 'DLH123', track_num: 42, vx: 0, vy: 0, mono: true }
    expect(buildLabel(track, '').split('\n')[0]).toBe('DLH123*')
  })

  it('marks a mono track that falls back to its track number', () => {
    const track = { track_num: 42, vx: 0, vy: 0, mono: true }
    expect(buildLabel(track, '').split('\n')[0]).toBe('42*')
  })

  it('does not mark an ordinary (multi-sensor) track', () => {
    const track = { callsign: 'DLH123', track_num: 42, vx: 0, vy: 0 }
    expect(buildLabel(track, '').split('\n')[0]).toBe('DLH123')
    // mono: false must behave exactly like an absent flag.
    expect(buildLabel({ ...track, mono: false }, '').split('\n')[0]).toBe('DLH123')
  })

  it('appends the selected altitude "S<FL>" to the FL line when present (#238)', () => {
    const track = { callsign: 'DLH123', track_num: 1, vx: 0, vy: 0, flight_level_ft: 20000, selected_altitude_ft: 35000 }
    expect(buildLabel(track, '').split('\n')[1]).toBe('FL200 S350')
  })

  it('keeps the trend arrow before the selected altitude', () => {
    const track = { callsign: 'DLH123', track_num: 1, vx: 0, vy: 0, flight_level_ft: 20000, selected_altitude_ft: 35000 }
    expect(buildLabel(track, '▲').split('\n')[1]).toBe('FL200 ▲ S350')
  })

  it('omits the selected altitude when the aircraft does not report it', () => {
    const track = { callsign: 'DLH123', track_num: 1, vx: 0, vy: 0, flight_level_ft: 20000 }
    expect(buildLabel(track, '').split('\n')[1]).toBe('FL200')
  })

  // Vertical chain (I062/135, ICD 3.5.0, #241): the label prefers the filtered
  // barometric altitude over the jumpier measured flight level, and marks its
  // reference — "A" for a QNH altitude, "FL" for a pressure level.
  it('prefers the filtered barometric altitude and marks a QNH altitude with "A"', () => {
    const track = { callsign: 'DLH1', track_num: 1, vx: 0, vy: 0, flight_level_ft: 34000, barometric_altitude_ft: 3000, qnh_corrected: true }
    expect(buildLabel(track, '').split('\n')[1]).toBe('A030')
  })

  it('marks an uncorrected barometric altitude as a flight level "FL"', () => {
    const track = { callsign: 'DLH2', track_num: 1, vx: 0, vy: 0, barometric_altitude_ft: 35000, qnh_corrected: false }
    expect(buildLabel(track, '').split('\n')[1]).toBe('FL350')
  })

  it('falls back to the measured flight level when no barometric altitude is present', () => {
    const track = { callsign: 'DLH3', track_num: 1, vx: 0, vy: 0, flight_level_ft: 28000 }
    expect(buildLabel(track, '').split('\n')[1]).toBe('FL280')
  })

  it('keeps the trend arrow and selected altitude on a QNH-altitude line', () => {
    const track = { callsign: 'DLH4', track_num: 1, vx: 0, vy: 0, barometric_altitude_ft: 5000, qnh_corrected: true, selected_altitude_ft: 6000 }
    expect(buildLabel(track, '▲').split('\n')[1]).toBe('A050 ▲ S060')
  })

  // Turn indicator (I062/200 TRANS, ICD 3.6.0, #242): → right turn, ← left turn,
  // nothing for a constant or undetermined course.
  it('appends → to the identity line for a right turn', () => {
    const track = { callsign: 'DLH5', track_num: 1, vx: 0, vy: 0, course_trend: 'right' }
    expect(buildLabel(track, '').split('\n')[0]).toBe('DLH5 →')
  })

  it('appends ← to the identity line for a left turn', () => {
    const track = { callsign: 'DLH6', track_num: 1, vx: 0, vy: 0, course_trend: 'left' }
    expect(buildLabel(track, '').split('\n')[0]).toBe('DLH6 ←')
  })

  it('adds no turn indicator for a constant or undetermined course', () => {
    expect(buildLabel({ callsign: 'A', track_num: 1, vx: 0, vy: 0, course_trend: 'constant' }, '').split('\n')[0]).toBe('A')
    expect(buildLabel({ callsign: 'B', track_num: 1, vx: 0, vy: 0 }, '').split('\n')[0]).toBe('B')
  })

  it('places the turn indicator after the MON marker', () => {
    const track = { callsign: 'DLH7', track_num: 1, vx: 0, vy: 0, mono: true, course_trend: 'right' }
    expect(buildLabel(track, '').split('\n')[0]).toBe('DLH7* →')
  })
})
