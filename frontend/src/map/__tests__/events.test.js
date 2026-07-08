import { describe, it, expect } from 'vitest'
import {
  feedStatusEvent,
  connectionEvent,
  trackLifecycleEvents,
  SEVERITY_META,
  SEV_INFO,
  SEV_WARN,
  SEV_ERROR,
  SEV_SUCCESS,
} from '../events.js'

describe('feedStatusEvent', () => {
  it('returns null when the aggregate status is unchanged', () => {
    expect(feedStatusEvent('ok', 'ok')).toBeNull()
    expect(feedStatusEvent('stale', 'stale')).toBeNull()
  })
  it('suppresses the benign initial climb to a healthy feed on (re)connect', () => {
    expect(feedStatusEvent('unknown', 'ok')).toBeNull()
    expect(feedStatusEvent(null, 'ok')).toBeNull()
  })
  it('never fires toward unknown', () => {
    expect(feedStatusEvent('ok', 'unknown')).toBeNull()
    expect(feedStatusEvent('stale', null)).toBeNull()
  })
  it('logs an error when the feed goes stale', () => {
    expect(feedStatusEvent('ok', 'stale')).toMatchObject({ type: 'feed-stale', severity: SEV_ERROR })
  })
  it('logs a warning when the feed degrades', () => {
    expect(feedStatusEvent('ok', 'degraded')).toMatchObject({ type: 'feed-degraded', severity: SEV_WARN })
    // Also fired on a bad initial status (not the benign unknown→ok case).
    expect(feedStatusEvent('unknown', 'degraded')).toMatchObject({ severity: SEV_WARN })
  })
  it('logs a success when the feed recovers to ok', () => {
    expect(feedStatusEvent('stale', 'ok')).toMatchObject({ type: 'feed-recovered', severity: SEV_SUCCESS })
    expect(feedStatusEvent('degraded', 'ok')).toMatchObject({ type: 'feed-recovered', severity: SEV_SUCCESS })
  })
})

describe('connectionEvent', () => {
  it('is silent on the very first connect', () => {
    expect(connectionEvent(null, 'open')).toBeNull()
  })
  it('returns null when unchanged', () => {
    expect(connectionEvent('open', 'open')).toBeNull()
    expect(connectionEvent('closed', 'closed')).toBeNull()
  })
  it('logs an error on a drop', () => {
    expect(connectionEvent('open', 'closed')).toMatchObject({ type: 'connection-lost', severity: SEV_ERROR })
  })
  it('logs a success on recovery', () => {
    expect(connectionEvent('closed', 'open')).toMatchObject({ type: 'connection-restored', severity: SEV_SUCCESS })
  })
})

describe('trackLifecycleEvents', () => {
  it('reports appeared for a newly-live track number', () => {
    const evts = trackLifecycleEvents([1, 2], [1, 2, 3], [])
    expect(evts).toEqual([
      { type: 'track-appeared', severity: SEV_INFO, message: 'Track 3 erschienen', trackNum: 3 },
    ])
  })
  it('reports disappeared only for TSE-ended tracks, not mere gaps', () => {
    // Track 2 drops out without a TSE (transient miss) → no event; track 1 is
    // explicitly ended → one disappeared event.
    const evts = trackLifecycleEvents([1, 2], [], [1])
    expect(evts).toEqual([
      { type: 'track-disappeared', severity: SEV_INFO, message: 'Track 1 beendet', trackNum: 1 },
    ])
  })
  it('reports appeared and disappeared together, appeared first', () => {
    const evts = trackLifecycleEvents([1], [1, 5], [9])
    expect(evts.map((e) => e.type)).toEqual(['track-appeared', 'track-disappeared'])
    expect(evts.map((e) => e.trackNum)).toEqual([5, 9])
  })
  it('accepts a Set for the previous numbers', () => {
    expect(trackLifecycleEvents(new Set([1, 2]), [2, 3], [])).toEqual([
      { type: 'track-appeared', severity: SEV_INFO, message: 'Track 3 erschienen', trackNum: 3 },
    ])
  })
  it('returns an empty array when nothing changed', () => {
    expect(trackLifecycleEvents([1, 2], [1, 2], [])).toEqual([])
    expect(trackLifecycleEvents(undefined, undefined, undefined)).toEqual([])
  })
})

describe('SEVERITY_META', () => {
  it('has an icon and colour for every severity', () => {
    for (const sev of [SEV_INFO, SEV_WARN, SEV_ERROR, SEV_SUCCESS]) {
      expect(SEVERITY_META[sev]).toHaveProperty('icon')
      expect(SEVERITY_META[sev]).toHaveProperty('color')
    }
  })
})
