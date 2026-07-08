import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useEventsStore, MAX_EVENTS } from '@/stores/events.js'
import { SEV_INFO } from '@/map/events.js'

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('events store', () => {
  it('stamps id + timestamp, prepends newest-first, and counts unseen', () => {
    const s = useEventsStore()
    s.add({ type: 'a', severity: SEV_INFO, message: 'first' })
    s.add({ type: 'b', severity: SEV_INFO, message: 'second' })
    expect(s.events.map((e) => e.message)).toEqual(['second', 'first'])
    expect(s.events[0].id).not.toBe(s.events[1].id)
    expect(typeof s.events[0].ts).toBe('number')
    expect(s.unseenCount).toBe(2)
  })

  it('caps the buffer at MAX_EVENTS, dropping the oldest', () => {
    const s = useEventsStore()
    for (let i = 0; i < MAX_EVENTS + 25; i++) {
      s.add({ type: 't', severity: SEV_INFO, message: `e${i}` })
    }
    expect(s.events.length).toBe(MAX_EVENTS)
    // Newest first: the most recent event is at the head, the oldest survivor at the tail.
    expect(s.events[0].message).toBe(`e${MAX_EVENTS + 24}`)
    expect(s.events[MAX_EVENTS - 1].message).toBe('e25')
  })

  it('addMany records a batch in order', () => {
    const s = useEventsStore()
    s.addMany([
      { type: 'x', severity: SEV_INFO, message: 'one' },
      { type: 'x', severity: SEV_INFO, message: 'two' },
    ])
    // Each add prepends, so the last of the batch ends up at the head.
    expect(s.events.map((e) => e.message)).toEqual(['two', 'one'])
    expect(s.unseenCount).toBe(2)
  })

  it('clear empties the log and resets the unseen counter', () => {
    const s = useEventsStore()
    s.add({ type: 'a', severity: SEV_INFO, message: 'x' })
    s.clear()
    expect(s.events).toEqual([])
    expect(s.unseenCount).toBe(0)
  })

  it('markSeen resets the unseen counter without dropping history', () => {
    const s = useEventsStore()
    s.add({ type: 'a', severity: SEV_INFO, message: 'x' })
    s.markSeen()
    expect(s.unseenCount).toBe(0)
    expect(s.events.length).toBe(1)
  })
})
