import { describe, it, expect, vi } from 'vitest'
import { captureSettings, applySettings, SETTINGS_VERSION } from '@/stores/profileSettings.js'

// makeAsd builds a fake ASD store: plain reactive-like fields plus setter spies
// that mutate them, mirroring the real store's setter contract (asd.js).
function makeAsd() {
  const layerVisibility = { airspace: true, aor: true, rangeRings: false, historyDots: true, navaids: true }
  const airspaceGroupVisibility = { ctr: true, tma: true, restricted: true, info: true }
  const basemapElementVisibility = { water: true, traffic: true, vegetation: true, settlement: true, building: true, boundary: true, label: true, background: true }
  const rangeRingConfig = { spacingNM: 10, count: 5 }
  const historyConfig = { durationS: 60 }
  const flFilter = { minFL: null, maxFL: null, hide: false }
  return {
    layerVisibility,
    airspaceGroupVisibility,
    basemapElementVisibility,
    rangeRingConfig,
    historyConfig,
    flFilter,
    setLayerVisibility: vi.fn((k, v) => { layerVisibility[k] = v }),
    setAirspaceGroup: vi.fn((k, v) => {
      airspaceGroupVisibility[k] = v
      layerVisibility.airspace = Object.values(airspaceGroupVisibility).some(Boolean)
    }),
    setBasemapElement: vi.fn((k, v) => { basemapElementVisibility[k] = v }),
    setRangeRingConfig: vi.fn((u) => Object.assign(rangeRingConfig, u)),
    setHistoryConfig: vi.fn((u) => Object.assign(historyConfig, u)),
    setFlFilter: vi.fn((u) => Object.assign(flFilter, u)),
  }
}

describe('captureSettings', () => {
  it('snapshots the display prefs (versioned, no map framing)', () => {
    const asd = makeAsd()
    asd.layerVisibility.rangeRings = true
    asd.rangeRingConfig.spacingNM = 15
    asd.flFilter.minFL = 100
    const s = captureSettings(asd)
    expect(s.v).toBe(SETTINGS_VERSION)
    expect(s.layers.rangeRings).toBe(true)
    expect(s.airspaceGroups.ctr).toBe(true)
    expect(s.rangeRings).toEqual({ spacingNM: 15, count: 5 })
    expect(s.history).toEqual({ durationS: 60 })
    expect(s.flFilter).toEqual({ minFL: 100, maxFL: null, hide: false })
    // No map centre/zoom is captured (Option A).
    expect(s).not.toHaveProperty('center')
    expect(s).not.toHaveProperty('zoom')
  })

  it('captures whatever layer keys the store carries (forward-compatible)', () => {
    const asd = makeAsd()
    asd.layerVisibility.somethingNew = true
    expect(captureSettings(asd).layers.somethingNew).toBe(true)
  })
})

describe('applySettings', () => {
  it('applies layers/groups/rings/history/flFilter through the setters', () => {
    const asd = makeAsd()
    applySettings(asd, {
      v: 1,
      layers: { rangeRings: true, historyDots: false },
      airspaceGroups: { ctr: false },
      rangeRings: { spacingNM: 20, count: 8 },
      history: { durationS: 120 },
      flFilter: { minFL: 50, maxFL: 300, hide: true },
    })
    expect(asd.layerVisibility.rangeRings).toBe(true)
    expect(asd.layerVisibility.historyDots).toBe(false)
    expect(asd.airspaceGroupVisibility.ctr).toBe(false)
    expect(asd.rangeRingConfig).toEqual({ spacingNM: 20, count: 8 })
    expect(asd.historyConfig.durationS).toBe(120)
    expect(asd.flFilter).toEqual({ minFL: 50, maxFL: 300, hide: true })
  })

  it('never sets the derived airspace layer directly (follows the groups)', () => {
    const asd = makeAsd()
    applySettings(asd, { layers: { airspace: false }, airspaceGroups: { ctr: false, tma: false, restricted: false, info: false } })
    // airspace layer was NOT set directly...
    expect(asd.setLayerVisibility).not.toHaveBeenCalledWith('airspace', expect.anything())
    // ...but derives to false because every group is off.
    expect(asd.layerVisibility.airspace).toBe(false)
  })

  it('is tolerant: unknown keys skipped, partial input, and no-op on junk', () => {
    const asd = makeAsd()
    applySettings(asd, { layers: { doesNotExist: true }, rangeRings: { spacingNM: 'x', count: NaN } })
    expect(asd.setLayerVisibility).not.toHaveBeenCalled() // unknown key ignored
    expect(asd.setRangeRingConfig).not.toHaveBeenCalled() // no finite values → no update
    // Non-object settings are a safe no-op.
    applySettings(asd, null)
    applySettings(asd, 42)
    expect(asd.setFlFilter).not.toHaveBeenCalled()
  })

  it('coerces a malformed FL bound back to null', () => {
    const asd = makeAsd()
    applySettings(asd, { flFilter: { minFL: 'abc', maxFL: 200, hide: false } })
    expect(asd.flFilter).toEqual({ minFL: null, maxFL: 200, hide: false })
  })

  it('round-trips: apply(capture(a)) reproduces a on b', () => {
    const a = makeAsd()
    a.layerVisibility.rangeRings = true
    a.airspaceGroupVisibility.tma = false
    a.rangeRingConfig.count = 7
    a.flFilter = { minFL: 80, maxFL: null, hide: true }
    const snapshot = captureSettings(a)

    const b = makeAsd()
    applySettings(b, snapshot)
    expect(b.layerVisibility.rangeRings).toBe(true)
    expect(b.airspaceGroupVisibility.tma).toBe(false)
    expect(b.rangeRingConfig.count).toBe(7)
    expect(b.flFilter).toEqual({ minFL: 80, maxFL: null, hide: true })
  })

  // E4 (#295): the per-element base-map switches persist in the view profile so a
  // decluttered scope (e.g. buildings + labels off) survives a reload/profile swap.
  it('captures and restores the per-element base-map switches', () => {
    const a = makeAsd()
    a.basemapElementVisibility.building = false
    a.basemapElementVisibility.label = false
    const snapshot = captureSettings(a)
    expect(snapshot.basemapElements.building).toBe(false)
    expect(snapshot.basemapElements.label).toBe(false)

    const b = makeAsd()
    applySettings(b, snapshot)
    expect(b.basemapElementVisibility.building).toBe(false)
    expect(b.basemapElementVisibility.label).toBe(false)
    expect(b.basemapElementVisibility.water).toBe(true) // untouched stays on
  })

  it('tolerates an older profile without a basemapElements section (elements stay default)', () => {
    const asd = makeAsd()
    applySettings(asd, { v: 1, layers: { rangeRings: true } }) // no basemapElements key
    expect(asd.setBasemapElement).not.toHaveBeenCalled()
    // all elements keep their all-on defaults
    for (const v of Object.values(asd.basemapElementVisibility)) expect(v).toBe(true)
  })

  it('skips an unknown element key in a stored profile', () => {
    const asd = makeAsd()
    applySettings(asd, { basemapElements: { water: false, bogusElement: true } })
    expect(asd.basemapElementVisibility.water).toBe(false)
    expect(asd.setBasemapElement).not.toHaveBeenCalledWith('bogusElement', expect.anything())
  })
})
