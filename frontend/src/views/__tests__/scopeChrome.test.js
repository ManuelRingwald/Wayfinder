// Regression guard for the scope-chrome cleanup (E2E design pass):
//  1. the top-centre status/filter chips are gone (status now reads from the
//     track symbology), and the category-filter machinery is fully removed;
//  2. account access is not duplicated on the map — the feed badge takes the
//     top-right spot (handled in asdViewAuthGate.test.js for the account side);
//  3. the bottom-right readout is a single "<width> NM Breite · Vektor N min"
//     pill, replacing the native scale bar.
// Source-level assertions (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import asdView from '../AsdView.vue?raw'
import mapCanvas from '../../components/MapCanvas.vue?raw'
import engine from '../../map/engine.js?raw'
import render from '../../map/render.js?raw'
import asdStore from '../../stores/asd.js?raw'

describe('top-centre status/filter chips removed', () => {
  it('MapCanvas no longer mounts TrackFilterChips', () => {
    expect(mapCanvas).not.toContain('TrackFilterChips')
  })

  it('the category-filter machinery is gone from store and render', () => {
    expect(asdStore).not.toContain('hiddenCategories')
    expect(asdStore).not.toContain('trackCounts')
    expect(asdStore).not.toContain('toggleCategoryFilter')
    expect(render).not.toContain('hiddenNums')
    expect(render).not.toContain('hiddenCategories')
  })
})

describe('bottom-right readout: "<width> NM Breite · Vektor N min"', () => {
  it('AsdView shows the combined readout and moves it to the corner', () => {
    expect(asdView).toContain('NM Breite · Vektor')
    expect(asdView).toContain('store.viewportWidthNM')
  })

  it('the engine reports viewport width and the native scale bar is removed', () => {
    expect(engine).toContain('reportViewportWidth')
    expect(engine).toContain('store.setViewportWidth')
    expect(engine).not.toContain('ScaleControl')
  })

  it('the store exposes viewportWidthNM + its setter', () => {
    expect(asdStore).toContain('viewportWidthNM')
    expect(asdStore).toContain('setViewportWidth')
  })
})

describe('feed badge takes the top-right (account chip removed)', () => {
  it('AsdView drops the account overlay and keeps the feed-status overlay', () => {
    expect(asdView).not.toContain('account-overlay')
    expect(asdView).toContain('feed-status-overlay')
  })
})
