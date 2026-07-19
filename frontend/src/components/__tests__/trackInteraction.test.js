// #271–#273 (operator requests 2026-07-18): track interaction trio.
//  - #271: clicking the data-block LABEL selects the track (not just the symbol)
//  - #272: the open detail panel follows live WS updates instead of freezing
//  - #273: clicking free map area deselects (panel closes, halo clears)
// Store behaviour is tested directly; the MapLibre/component wiring is pinned
// with source-guards (house pattern, cf. viewProfileMenu.test.js) because the
// engine's click handlers need a real map to execute.
import { describe, it, expect, beforeEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { setActivePinia, createPinia } from 'pinia'
import { useAsdStore } from '@/stores/asd.js'

const src = (rel) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

beforeEach(() => {
  setActivePinia(createPinia())
})

const feature = (num, extra = {}) => ({
  properties: { track_num: num, callsign: 'DLH123', flight_level: 310, ...extra },
})

describe('refreshSelectedTrack (#272)', () => {
  it('replaces the selected snapshot with the live feature properties', () => {
    const store = useAsdStore()
    store.selectTrack({ track_num: 7, callsign: 'DLH123', flight_level: 310 })
    store.refreshSelectedTrack([feature(3), feature(7, { flight_level: 320, rocd_ft_min: 800 })])
    expect(store.selectedTrack.flight_level).toBe(320)
    expect(store.selectedTrack.rocd_ft_min).toBe(800)
    expect(store.selectedTrack.track_num).toBe(7)
  })

  it('assigns a fresh object so Vue reactivity fires on every update', () => {
    const store = useAsdStore()
    store.selectTrack({ track_num: 7 })
    const before = store.selectedTrack
    store.refreshSelectedTrack([feature(7)])
    expect(store.selectedTrack).not.toBe(before)
  })

  it('keeps the last snapshot when the track vanished (TSE) — panel stays open', () => {
    const store = useAsdStore()
    store.selectTrack({ track_num: 7, callsign: 'DLH123' })
    store.refreshSelectedTrack([feature(3)])
    expect(store.selectedTrack).not.toBeNull()
    expect(store.selectedTrack.callsign).toBe('DLH123')
  })

  it('is a no-op without a selection and tolerates a missing feature list', () => {
    const store = useAsdStore()
    store.refreshSelectedTrack([feature(1)])
    expect(store.selectedTrack).toBeNull()
    store.selectTrack({ track_num: 1 })
    store.refreshSelectedTrack(undefined)
    expect(store.selectedTrack.track_num).toBe(1)
  })
})

describe('engine wiring (source-guard)', () => {
  const engine = src('../../map/engine.js')

  it('#271: registers the shared track-click handler on the labels layer too', () => {
    expect(engine).toMatch(/map\.on\('click', TRACKS_LAYER_ID, emitTrackClick\(TRACKS_LAYER_ID\)\)/)
    expect(engine).toMatch(/map\.on\('click', LABELS_LAYER_ID, emitTrackClick\(LABELS_LAYER_ID\)\)/)
  })

  it('#273: the general click handler queries BOTH track layers and only then reports empty', () => {
    expect(engine).toMatch(/layers: \[TRACKS_LAYER_ID, LABELS_LAYER_ID\]/)
    expect(engine).toMatch(/if \(!hits \|\| hits\.length === 0\) onEmptyClick\(\)/)
  })

  it('#272: refreshes the selected track in the WS path and the pending flush', () => {
    const calls = engine.match(/store\.refreshSelectedTrack\(state\.liveTrackFeatures\)/g) || []
    expect(calls.length).toBe(2)
  })
})

describe('component wiring (source-guard)', () => {
  it('#272: the correlation pre-fill watch keys on track_num, not the object', () => {
    const card = src('../TrackDetailCard.vue')
    expect(card).toMatch(/watch\(\s*\(\) => track\.value\?\.track_num,/)
    expect(card).not.toMatch(/watch\(\s*track,/)
  })

  it('#273: AsdView guards empty-click with the measure-tool check', () => {
    const view = src('../../views/AsdView.vue')
    expect(view).toMatch(/@empty-click="onMapEmptyClick"/)
    expect(view).toMatch(/function onMapEmptyClick\(\) \{\s*if \(tools\.activeTool\) return\s*store\.clearTrackSelection\(\)/)
  })

  it('#273: MapCanvas declares and forwards the empty-click emit', () => {
    const canvas = src('../MapCanvas.vue')
    expect(canvas).toMatch(/'empty-click'/)
    expect(canvas).toMatch(/\(\) => emit\('empty-click'\)/)
  })
})
