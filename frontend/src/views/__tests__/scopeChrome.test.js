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

// The bottom-right "<width> NM Breite · Vektor N min" readout was removed (E2E):
// it looked like a scale bar but was only the scope width, and sat confusingly
// next to the range rings. Distance is read from the range rings; the speed
// vectors carry the look-ahead. These guards keep the whole chain from creeping
// back — the store ref, the engine reporter and the view overlay.
describe('bottom-right readout removed', () => {
  it('AsdView no longer renders the readout', () => {
    expect(asdView).not.toContain('NM Breite')
    expect(asdView).not.toContain('viewportWidthNM')
    expect(asdView).not.toContain('vector-readout-overlay')
    expect(asdView).not.toContain('vectorMinutes')
  })

  it('the engine drops the viewport-width reporter (and still has no scale bar)', () => {
    expect(engine).not.toContain('reportViewportWidth')
    expect(engine).not.toContain('setViewportWidth')
    expect(engine).not.toContain('ScaleControl')
  })

  it('the store no longer exposes viewportWidthNM / its setter', () => {
    expect(asdStore).not.toContain('viewportWidthNM')
    expect(asdStore).not.toContain('setViewportWidth')
  })
})

describe('feed badge takes the top-right (account chip removed)', () => {
  it('AsdView drops the account overlay and groups header + feed top-right', () => {
    expect(asdView).not.toContain('account-overlay')
    // header (ICAO/UTC) now sits next to the feed badge in one top-right cluster
    expect(asdView).toContain('top-right-cluster')
    expect(asdView).toContain('<FeedStatusChip')
    expect(asdView).toContain('<AsdHeader')
  })
})
