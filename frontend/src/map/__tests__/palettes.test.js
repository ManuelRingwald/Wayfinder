// ADR 0026: the official basemap.de base map ("bkg" theme) is a bright
// cartographic base, so it must reuse the bright foreground palette — engine.js
// looks the theme up in PALETTES verbatim, and a missing key would silently
// fall back to the dark palette (light labels on a bright map = unreadable).
import { describe, it, expect } from 'vitest'
import { PALETTES } from '../constants.js'

describe('PALETTES per base-map theme', () => {
  it('provides a palette for every built-in theme', () => {
    expect(Object.keys(PALETTES).sort()).toEqual(['bkg', 'dark', 'osm'])
  })

  it('bkg shares the bright palette with osm', () => {
    expect(PALETTES.bkg).toBe(PALETTES.osm)
    expect(PALETTES.bkg.label).toBe('#212121')
    expect(PALETTES.bkg.labelHalo).toBe('#ffffff')
  })

  it('dark stays a distinct light-on-dark palette', () => {
    expect(PALETTES.dark).not.toBe(PALETTES.bkg)
    expect(PALETTES.dark.label).toBe('#dce6f0')
  })
})
