// ASD-013: the Alarm-/Ereignis-Panel. There is no component-mount harness for
// the ASD chrome, so — like the other UI tests (see feedStatusChip.test.js) —
// assert the wiring against the raw source. The panel's behaviour proper (event
// derivation, ring buffer) is covered by map/__tests__/events.test.js and
// stores/__tests__/events.test.js.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, it, expect } from 'vitest'

const read = (rel) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
const panel = read('../EventPanel.vue')
const asdView = read('../../views/AsdView.vue')
const engine = read('../../map/engine.js')

describe('EventPanel wiring (ASD-013)', () => {
  it('reads the event log from the events store', () => {
    expect(panel).toContain('useEventsStore')
    expect(panel).toContain('store.events')
  })
  it('maps severity to icon/colour via SEVERITY_META', () => {
    expect(panel).toContain('SEVERITY_META')
  })
  it('offers a clear action and a close emit', () => {
    expect(panel).toContain('store.clear()')
    expect(panel).toContain("emit('close')")
  })
  it('shows an empty-state hint when there are no events', () => {
    expect(panel).toContain('Keine Ereignisse')
  })
})

describe('AsdView event bell wiring (ASD-013)', () => {
  it('mounts the EventPanel and a bell toggle', () => {
    expect(asdView).toContain('EventPanel')
    expect(asdView).toContain('toggleEvents')
  })
  it('marks the log seen on open and badges the unseen count', () => {
    expect(asdView).toContain('events.markSeen()')
    expect(asdView).toContain('events.unseenCount')
    expect(asdView).toContain('badgeContent')
  })
})

describe('engine event derivation wiring (ASD-013)', () => {
  it('feeds derived events into the events store', () => {
    expect(engine).toContain('useEventsStore')
    expect(engine).toContain('feedStatusEvent')
    expect(engine).toContain('connectionEvent')
    expect(engine).toContain('recordTrackEvents')
  })
  it('primes the track baseline so the initial picture does not flood the log', () => {
    expect(engine).toContain('trackEventsPrimed')
  })
})
