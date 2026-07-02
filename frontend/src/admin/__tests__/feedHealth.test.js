// describeFeedHealth maps a per-feed health snapshot to the admin chip's
// { color, label, title }. The point of this suite is the status granularity:
// the single flat red "inaktiv" is split into "nie gestartet" (!ever_seen) vs
// "abgerissen" (ever_seen && stale), and traffic / empty-sky / sensor share are
// surfaced consistently.
import { describe, it, expect } from 'vitest'
import { describeFeedHealth } from '../feedHealth.js'

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
