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

  it('draws ADS-B as a diamond and PSR as an always-hollow ring', () => {
    // ADS-B: a rotated-square diamond path; PSR: an open ring in every state
    // (design template — the PSR branch ignores the fill channel).
    expect(layers).toContain("shape === 'adsb'")
    expect(layers).toContain('ctx.closePath()') // diamond path is closed
    expect(layers).toContain('PSR is always an open ring')
    expect(layers).toContain('ctx.arc(c, c, 9, 0, 2 * Math.PI)')
  })

  it('renders the coasting state hollow (outline) via the icon matrix', () => {
    expect(layers).toContain("makeTrackIcon(shape, color, stateKey === 'coasting')")
  })

  it('no longer dims the coasting symbol by opacity (hollow carries the state)', () => {
    expect(layers).not.toContain("['get', 'coasting'], 0.5")
  })
})

// ASD-007: symbol geometry aligned to the design template (scope-tracks.jsx
// symbolNode). Symbols are canvas-drawn at pixelRatio 2 on a 32-px canvas, so
// every coordinate here is the template CSS pixel × 2. The earlier 24-px canvas
// capped the footprint at 12 CSS px and rendered symbols ~40% too small.
describe('track symbol geometry matches the design template (layers.js)', () => {
  it('renders track icons on a 32-px canvas (stroke headroom for the 12px diamond)', () => {
    expect(layers).toContain('}, 32)')
  })

  it('sizes the diamond 12 CSS px point-to-point (vertices at c ± 12)', () => {
    expect(layers).toContain('ctx.moveTo(c, c - 12)')
    expect(layers).toContain('ctx.lineTo(c + 12, c)')
  })

  it('sizes the SSR square 8 CSS px (rect 16 canvas px)', () => {
    expect(layers).toContain('ctx.rect(c - 8, c - 8, 16, 16)')
  })

  it('shrinks the history dots to the template r=1.6', () => {
    expect(layers).toContain("'circle-radius': 1.6")
  })
})

// ASD-007: cyan selection halo (design template symbolNode: r=11, stroke primary)
describe('selection halo layer (layers.js)', () => {
  it('registers a hollow selection ring at r=11 under the symbols', () => {
    expect(layers).toContain('addSelectionLayer')
    expect(layers).toContain("'circle-radius': 11")
    expect(layers).toContain("'circle-stroke-color': palette.selection")
    expect(layers).toContain("'circle-opacity': 0") // hollow: no fill
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
