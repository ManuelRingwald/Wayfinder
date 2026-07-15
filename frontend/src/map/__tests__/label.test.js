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
})
