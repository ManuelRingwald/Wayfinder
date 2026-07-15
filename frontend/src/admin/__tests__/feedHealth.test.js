// describeFeedHealth maps a per-feed health snapshot to the admin chip's
// { color, label, title }. The point of this suite is the status granularity:
// the single flat red "inaktiv" is split into "nie gestartet" (!ever_seen) vs
// "abgerissen" (ever_seen && stale), and traffic / empty-sky / sensor share are
// surfaced consistently.
import { describe, it, expect } from 'vitest'
import { describeFeedHealth, formatSensorBias, describeSensor, sensorNeedsAttention } from '../feedHealth.js'

describe('describeFeedHealth', () => {
  it('missing snapshot is neutral "unbekannt"', () => {
    const d = describeFeedHealth(undefined)
    expect(d).toEqual({ color: 'default', label: 'unbekannt', title: 'Gesundheit unbekannt' })
  })

  it('green with traffic shows the track count', () => {
    const d = describeFeedHealth({ color: 'green', ever_seen: true, track_count_recent: 12 })
    expect(d.color).toBe('success')
    expect(d.label).toBe('OK')
    expect(d.title).toContain('12 Tracks')
  })

  it('green with no tracks reads "leerer Himmel", not an error', () => {
    const d = describeFeedHealth({ color: 'green', ever_seen: true, track_count_recent: 0 })
    expect(d.label).toBe('OK')
    expect(d.title).toContain('leerer Himmel')
  })

  it('green appends the sensor share when CAT063 is present', () => {
    const d = describeFeedHealth({
      color: 'green', ever_seen: true, track_count_recent: 3,
      sensors_active: 2, sensors_total: 2,
    })
    expect(d.title).toContain('2/2 Radare')
  })

  it('yellow reports the degraded sensor fusion', () => {
    const d = describeFeedHealth({
      color: 'yellow', ever_seen: true, sensors_active: 1, sensors_total: 3,
    })
    expect(d.color).toBe('warning')
    expect(d.label).toBe('degradiert')
    expect(d.title).toContain('1 von 3')
  })

  it('red + never seen → "nie gestartet"', () => {
    const d = describeFeedHealth({ color: 'red', ever_seen: false, last_heartbeat_ago_s: -1 })
    expect(d.color).toBe('error')
    expect(d.label).toBe('nie gestartet')
    expect(d.title).toMatch(/nie angelaufen/i)
  })

  it('red + previously seen but stale → "abgerissen" with the age', () => {
    const d = describeFeedHealth({
      color: 'red', ever_seen: true, stale: true, last_heartbeat_ago_s: 42.7,
    })
    expect(d.color).toBe('error')
    expect(d.label).toBe('abgerissen')
    expect(d.title).toContain('43 s') // rounded
    expect(d.title).toContain('CAT065')
  })

  it('the two red sub-states carry distinct labels', () => {
    const never = describeFeedHealth({ color: 'red', ever_seen: false })
    const stale = describeFeedHealth({ color: 'red', ever_seen: true, stale: true })
    expect(never.label).not.toBe(stale.label)
  })
})

// #237: per-sensor registration bias presentation.
describe('formatSensorBias', () => {
  it('renders both components with a leading + for non-negative values', () => {
    expect(formatSensorBias({ range_bias_m: 144.69, azimuth_bias_deg: 0.302 }))
      .toBe('Δr +145 m · Δθ +0.30°')
  })

  it('keeps the minus sign for negative corrections', () => {
    expect(formatSensorBias({ range_bias_m: -144.69, azimuth_bias_deg: -0.302 }))
      .toBe('Δr -145 m · Δθ -0.30°')
  })

  it('omits an absent component and returns "" when no bias is present', () => {
    expect(formatSensorBias({ range_bias_m: 20 })).toBe('Δr +20 m')
    expect(formatSensorBias({ azimuth_bias_deg: 0 })).toBe('Δθ +0.00°')
    expect(formatSensorBias({ sic: 2, operational: true })).toBe('')
    expect(formatSensorBias(null)).toBe('')
  })
})

describe('describeSensor', () => {
  it('leads with the SIC and appends the bias for a corrected sensor', () => {
    expect(describeSensor({ sic: 1, operational: true, range_bias_m: 144.69, azimuth_bias_deg: 0.302 }))
      .toBe('SIC 1 · Δr +145 m · Δθ +0.30°')
  })

  it('words a degraded sensor by its reason', () => {
    expect(describeSensor({ sic: 2, operational: false, degraded_reason: 'unreachable' }))
      .toBe('SIC 2 · nicht erreichbar')
  })

  it('falls back to just the SIC for an operational, unbiased sensor', () => {
    expect(describeSensor({ sic: 3, operational: true })).toBe('SIC 3')
  })
})

describe('sensorNeedsAttention', () => {
  it('is true for a degraded or biased sensor, false for a plain operational one', () => {
    expect(sensorNeedsAttention({ operational: false })).toBe(true)
    expect(sensorNeedsAttention({ operational: true, range_bias_m: 1 })).toBe(true)
    expect(sensorNeedsAttention({ operational: true, azimuth_bias_deg: 0 })).toBe(true)
    expect(sensorNeedsAttention({ operational: true })).toBe(false)
    expect(sensorNeedsAttention(null)).toBe(false)
  })
})
