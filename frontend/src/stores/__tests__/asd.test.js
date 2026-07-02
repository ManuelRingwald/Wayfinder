import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAsdStore } from '@/stores/asd.js'

beforeEach(() => {
  setActivePinia(createPinia())
})

// #117: the broadcast FeedStatusMessage speaks per-feed colors
// (green/yellow/red); the store maps them to chip states and aggregates
// worst-wins across all subscribed feeds.
describe('asd store — per-feed health aggregation (#117)', () => {
  it('starts unknown until the first feed status arrives', () => {
    const s = useAsdStore()
    expect(s.feedStatus).toBe('unknown')
  })

  it('maps the wire colors to chip states', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'green')
    expect(s.feedStatus).toBe('ok')
    s.setFeedHealth(1, 'yellow')
    expect(s.feedStatus).toBe('degraded')
    s.setFeedHealth(1, 'red')
    expect(s.feedStatus).toBe('stale')
  })

  it('aggregates worst-wins across feeds (a dead feed is never masked)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'green')
    s.setFeedHealth(2, 'red')
    expect(s.feedStatus).toBe('stale')
    s.setFeedHealth(2, 'green')
    expect(s.feedStatus).toBe('ok')
  })

  it('ignores an unknown color instead of corrupting the chip', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'green')
    s.setFeedHealth(1, 'chartreuse')
    expect(s.feedStatus).toBe('ok')
  })

  it('resetFeedHealth returns to unknown (WS reconnect drops stale scope)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'red')
    s.resetFeedHealth()
    expect(s.feedStatus).toBe('unknown')
  })
})
