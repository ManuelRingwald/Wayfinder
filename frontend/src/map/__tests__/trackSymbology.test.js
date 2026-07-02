// Regression guard for Häppchen 2 (track symbology, aligning the ASD to the
// design mockup + legend). The map icons are canvas-drawn (no DOM to mount, and
// jsdom has no 2D canvas), so — like the repo's other SFC checks — we assert the
// wiring against the raw source rather than rendering pixels.
import { describe, it, expect } from 'vitest'
import layers from '../layers.js?raw'
import legend from '../../components/ScopeLegend.vue?raw'
import asdView from '../../views/AsdView.vue?raw'

describe('track symbology — shapes & hollow coasting (layers.js)', () => {
  it('makeTrackIcon is state-aware (takes a hollow flag)', () => {
    expect(layers).toContain('function makeTrackIcon(shape, color, hollow)')
  })

  it('draws ADS-B as a diamond and PSR as a (fillable) circle', () => {
    // ADS-B: a rotated-square diamond path; PSR: an arc that strokeOrFill fills.
    expect(layers).toContain("shape === 'adsb'")
    expect(layers).toContain('ctx.closePath()') // diamond path is closed
    expect(layers).toContain('ctx.arc(c, c, 6, 0, 2 * Math.PI)')
  })

  it('renders the coasting state hollow (outline) via the icon matrix', () => {
    expect(layers).toContain("makeTrackIcon(shape, color, stateKey === 'coasting')")
  })

  it('no longer dims the coasting symbol by opacity (hollow carries the state)', () => {
    expect(layers).not.toContain("['get', 'coasting'], 0.5")
  })
})

describe('scope legend mirrors the map (ScopeLegend.vue)', () => {
  it('shows the coasting state as a hollow ring', () => {
    expect(legend).toContain('hollow: true')
    expect(legend).toContain("s.hollow ?")
  })
})

describe('legend is not hidden behind the navigation rail (AsdView.vue)', () => {
  it('offsets the bottom-left legend past the rail', () => {
    // left:12px sat under the 56px rail; the fix clears it.
    expect(asdView).toContain('left: 68px')
    expect(asdView).not.toMatch(/\.scope-legend-overlay\s*\{[^}]*left:\s*12px/)
  })
})
