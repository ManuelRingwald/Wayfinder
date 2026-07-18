// ADR 0026 (Nachtrag Ausbau OSM/CARTO): only the official BKG themes remain —
// engine.js looks the theme up in PALETTES verbatim, and a missing key would
// silently fall back to the dark palette (wrong foregrounds on a bright base).
// Legacy env values ("dark"/"osm") are aliased server-side, so these two keys
// are the complete vocabulary.
import { describe, it, expect } from 'vitest'
import { PALETTES } from '../constants.js'

describe('PALETTES per base-map theme', () => {
  it('provides exactly the built-in BKG themes', () => {
    expect(Object.keys(PALETTES).sort()).toEqual(['bkg', 'bkg-dark'])
  })

  it('bkg is the bright palette (dark-on-light)', () => {
    expect(PALETTES.bkg.label).toBe('#212121')
    expect(PALETTES.bkg.labelHalo).toBe('#ffffff')
  })

  it('bkg-dark is the distinct light-on-dark scope palette', () => {
    expect(PALETTES['bkg-dark']).not.toBe(PALETTES.bkg)
    expect(PALETTES['bkg-dark'].label).toBe('#dce6f0')
    expect(PALETTES['bkg-dark'].labelHalo).toBe('#000000')
  })
})
