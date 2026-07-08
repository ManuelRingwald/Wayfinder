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
const mapCanvas = read('../MapCanvas.vue')

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

// Operator request 2026-07-08: clicking an event whose track is still on the
// scope selects that track. Only still-live rows are actionable.
describe('EventPanel track-select wiring (ASD-013)', () => {
  it('gates selectability on the live-track set from the asd store', () => {
    expect(panel).toContain('useAsdStore')
    expect(panel).toContain('asd.liveTrackNums.has(e.trackNum)')
    expect(panel).toContain('function isSelectable')
  })
  it('emits select-track with the track number on a live-row click', () => {
    expect(panel).toContain("defineEmits(['close', 'select-track'])")
    expect(panel).toContain("emit('select-track', e.trackNum)")
    expect(panel).toContain('@click="onRowClick(e)"')
  })
  it('marks only selectable rows as a link/clickable', () => {
    expect(panel).toContain(":link=\"isSelectable(e)\"")
    expect(panel).toContain('event-panel__row--selectable')
  })
})

describe('AsdView + MapCanvas track-select wiring (ASD-013)', () => {
  it('AsdView handles select-track and closes the panel on a real selection', () => {
    expect(asdView).toContain('@select-track="onEventSelectTrack"')
    expect(asdView).toContain('function onEventSelectTrack')
    expect(asdView).toContain('mapCanvas.value?.selectTrackByNum(trackNum)')
  })
  it('MapCanvas exposes selectTrackByNum delegating to the engine', () => {
    expect(mapCanvas).toContain('selectTrackByNum')
    expect(mapCanvas).toContain('mapEngine?.selectTrackByNum(trackNum)')
  })
  it('the engine selects a live track by number and syncs the live set', () => {
    expect(engine).toContain('function selectTrackByNum')
    expect(engine).toContain('store.selectTrack(feature.properties)')
    expect(engine).toContain('const syncLiveTrackNums')
    expect(engine).toContain('store.setLiveTrackNums(')
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
