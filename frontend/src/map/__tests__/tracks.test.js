import { describe, it, expect } from 'vitest'
import { isFlFiltered, flOpacity, updateTracksLayer } from '../tracks.js'

const noFilter = { minFL: null, maxFL: null, hide: false }

describe('isFlFiltered', () => {
  it('returns false when no filter is set (null/null)', () => {
    expect(isFlFiltered(10000, noFilter)).toBe(false)
  })

  it('returns false for unknown FL (undefined)', () => {
    expect(isFlFiltered(undefined, { minFL: 100, maxFL: 200, hide: false })).toBe(false)
  })

  it('returns false for unknown FL (null)', () => {
    expect(isFlFiltered(null, { minFL: 100, maxFL: 200, hide: false })).toBe(false)
  })

  it('returns false when FL is within [minFL, maxFL]', () => {
    const filter = { minFL: 100, maxFL: 200, hide: false }
    expect(isFlFiltered(15000, filter)).toBe(false) // FL150
  })

  it('returns true when FL is below minFL', () => {
    const filter = { minFL: 100, maxFL: null, hide: false }
    expect(isFlFiltered(5000, filter)).toBe(true) // FL50 < FL100
  })

  it('returns true when FL is above maxFL', () => {
    const filter = { minFL: null, maxFL: 200, hide: false }
    expect(isFlFiltered(25000, filter)).toBe(true) // FL250 > FL200
  })

  it('returns false when FL exactly equals minFL', () => {
    const filter = { minFL: 100, maxFL: null, hide: false }
    expect(isFlFiltered(10000, filter)).toBe(false) // FL100 == minFL 100
  })

  it('returns false when FL exactly equals maxFL', () => {
    const filter = { minFL: null, maxFL: 200, hide: false }
    expect(isFlFiltered(20000, filter)).toBe(false) // FL200 == maxFL 200
  })

  it('handles only minFL set', () => {
    const filter = { minFL: 50, maxFL: null, hide: false }
    expect(isFlFiltered(4000, filter)).toBe(true)  // FL40 < FL50
    expect(isFlFiltered(6000, filter)).toBe(false) // FL60 > FL50
  })

  it('handles only maxFL set', () => {
    const filter = { minFL: null, maxFL: 300, hide: false }
    expect(isFlFiltered(30000, filter)).toBe(false) // FL300 <= 300
    expect(isFlFiltered(31000, filter)).toBe(true)  // FL310 > 300
  })
})

describe('flOpacity', () => {
  it('returns undefined when track passes the filter', () => {
    expect(flOpacity(15000, noFilter)).toBeUndefined()
  })

  it('returns 0 in hide mode for a filtered track', () => {
    const filter = { minFL: 200, maxFL: null, hide: true }
    expect(flOpacity(5000, filter)).toBe(0.0) // FL50 < 200, hide=true
  })

  it('returns 0.15 in dim mode for a filtered track', () => {
    const filter = { minFL: 200, maxFL: null, hide: false }
    expect(flOpacity(5000, filter)).toBeCloseTo(0.15) // FL50 < 200, hide=false
  })

  it('returns undefined for unknown FL regardless of filter', () => {
    const filter = { minFL: 100, maxFL: 300, hide: true }
    expect(flOpacity(undefined, filter)).toBeUndefined()
    expect(flOpacity(null, filter)).toBeUndefined()
  })
})

// WF2-40: the track symbol shape is driven by a `provenance` property baked
// onto each feature. This locks in that updateTracksLayer attaches it (the
// classification logic itself is covered by provenance.test.js).
function makeState() {
  return {
    trackHistory: new Map(),
    trackFlHistory: new Map(),
    trackCoasting: new Map(),
    fadingTracks: new Map(),
    liveTrackFeatures: [],
    liveVectorFeatures: [],
  }
}

// Bug #55: mode_3a and callsign must be baked into feature properties so the
// TrackDetailCard can display them when the user selects a track.
describe('updateTracksLayer identity fields (bug #55)', () => {
  it('bakes mode_3a and callsign onto features when present', () => {
    const state = makeState()
    const msg = {
      tracks: [
        {
          latitude: 50, longitude: 8, vx: 0, vy: 0, confirmed: true, coasting: false,
          track_num: 1, mode_3a: 0o7700, callsign: 'DLH123',
        },
        {
          latitude: 50, longitude: 8, vx: 0, vy: 0, confirmed: true, coasting: false,
          track_num: 2, // no mode_3a, no callsign
        },
      ],
    }
    updateTracksLayer(msg, state, () => {}, () => {})
    const [f1, f2] = state.liveTrackFeatures
    expect(f1.properties.mode_3a).toBe(0o7700)
    expect(f1.properties.callsign).toBe('DLH123')
    expect(f2.properties.mode_3a).toBeNull()
    expect(f2.properties.callsign).toBeNull()
  })
})

// ASD-011: the extended TrackDetailCard reads its fields straight off the baked
// feature properties, so updateTracksLayer must expose them.
describe('updateTracksLayer extended detail fields (ASD-011)', () => {
  it('bakes position, identity, accuracy and per-tech ages onto features', () => {
    const state = makeState()
    const msg = {
      tracks: [
        {
          latitude: 53.63, longitude: 9.99, vx: 100, vy: 0, confirmed: true, coasting: false,
          track_num: 7, sac: 25, sic: 10, accuracy: 42, icao_addr: 0x3c6dd2,
          adsb_age_s: 2, ssr_age_s: 40,
        },
      ],
    }
    updateTracksLayer(msg, state, () => {}, () => {})
    const p = state.liveTrackFeatures[0].properties
    expect(p.latitude).toBe(53.63)
    expect(p.longitude).toBe(9.99)
    expect(p.sac).toBe(25)
    expect(p.sic).toBe(10)
    expect(p.accuracy).toBe(42)
    expect(p.icao_addr).toBe(0x3c6dd2)
    expect(p.adsb_age_s).toBe(2)
    expect(p.ssr_age_s).toBe(40)
    expect(p.flarm_age_s).toBeNull()
    expect(p.mds_age_s).toBeNull()
  })

  it('bakes the vertical-tendency glyph as a property once a prior FL is known', () => {
    const state = makeState()
    const base = { latitude: 50, longitude: 8, vx: 0, vy: 0, confirmed: true, coasting: false, track_num: 3 }
    // First update establishes the FL baseline → no trend yet.
    updateTracksLayer({ tracks: [{ ...base, flight_level_ft: 10000 }] }, state, () => {}, () => {})
    expect(state.liveTrackFeatures[0].properties.vertical_trend).toBe('')
    // Second update climbs > 50 ft → ▲.
    updateTracksLayer({ tracks: [{ ...base, flight_level_ft: 12000 }] }, state, () => {}, () => {})
    expect(state.liveTrackFeatures[0].properties.vertical_trend).toBe('▲')
  })
})

describe('updateTracksLayer provenance (WF2-40)', () => {
  it('attaches the derived provenance to every live track feature', () => {
    const state = makeState()
    const base = { latitude: 50, longitude: 8, vx: 0, vy: 0, confirmed: true, coasting: false }
    const msg = {
      tracks: [
        { ...base, track_num: 1, adsb_age_s: 2 },          // fresh ADS-B
        { ...base, track_num: 2, icao_addr: 0x3c6dd2 },    // Mode S, no ADS-B
        { ...base, track_num: 3, adsb_age_s: 120 },        // stale ADS-B, no id
      ],
    }
    updateTracksLayer(msg, state, () => {}, () => {})

    const byNum = Object.fromEntries(
      state.liveTrackFeatures.map((f) => [f.properties.track_num, f.properties.provenance]),
    )
    expect(byNum).toEqual({ 1: 'adsb', 2: 'ssr', 3: 'psr' })
  })
})

// #236: I062/080 MON/SPI trust flags must be baked onto the feature as real
// booleans (the SPI highlight layer filters on spi==true; the label/detail read
// mono). An omitted wire field must coerce to false, never undefined.
describe('updateTracksLayer MON/SPI flags (#236)', () => {
  it('bakes mono and spi as booleans, defaulting an absent flag to false', () => {
    const state = makeState()
    const base = { latitude: 50, longitude: 8, vx: 0, vy: 0, confirmed: true, coasting: false }
    const msg = {
      tracks: [
        { ...base, track_num: 1, mono: true, spi: true },  // both set
        { ...base, track_num: 2 },                          // both absent on the wire
      ],
    }
    updateTracksLayer(msg, state, () => {}, () => {})
    const [f1, f2] = state.liveTrackFeatures
    expect(f1.properties.mono).toBe(true)
    expect(f1.properties.spi).toBe(true)
    expect(f2.properties.mono).toBe(false)
    expect(f2.properties.spi).toBe(false)
  })
})

// #238: Mode-S DAPs (I062/380) baked onto the feature; an absent parameter
// becomes null (not undefined) so the detail panel v-if reads cleanly.
describe('updateTracksLayer Mode-S DAPs (#238)', () => {
  it('bakes selected altitude, heading, IAS and Mach, defaulting absent ones to null', () => {
    const state = makeState()
    const base = { latitude: 50, longitude: 8, vx: 0, vy: 0, confirmed: true, coasting: false }
    const msg = {
      tracks: [
        { ...base, track_num: 1, selected_altitude_ft: 35000, magnetic_heading_deg: 270, ias_kt: 250, mach: 0.784 },
        { ...base, track_num: 2 }, // no DAPs
      ],
    }
    updateTracksLayer(msg, state, () => {}, () => {})
    const [f1, f2] = state.liveTrackFeatures
    expect(f1.properties.selected_altitude_ft).toBe(35000)
    expect(f1.properties.magnetic_heading_deg).toBe(270)
    expect(f1.properties.ias_kt).toBe(250)
    expect(f1.properties.mach).toBe(0.784)
    expect(f2.properties.selected_altitude_ft).toBeNull()
    expect(f2.properties.magnetic_heading_deg).toBeNull()
    expect(f2.properties.ias_kt).toBeNull()
    expect(f2.properties.mach).toBeNull()
  })
})

// Vertical chain (I062/130/135/220, ICD 3.5.0, #241).
describe('updateTracksLayer vertical chain (#241)', () => {
  it('bakes geometric/barometric altitude, QNH flag and rate, defaulting absent ones', () => {
    const state = makeState()
    const base = { latitude: 50, longitude: 8, vx: 0, vy: 0, confirmed: true, coasting: false }
    const msg = {
      tracks: [
        { ...base, track_num: 1, geometric_altitude_ft: 10000, barometric_altitude_ft: 3000, qnh_corrected: true, rocd_ft_min: 1500 },
        { ...base, track_num: 2 }, // no vertical data
      ],
    }
    updateTracksLayer(msg, state, () => {}, () => {})
    const [f1, f2] = state.liveTrackFeatures
    expect(f1.properties.geometric_altitude_ft).toBe(10000)
    expect(f1.properties.barometric_altitude_ft).toBe(3000)
    expect(f1.properties.qnh_corrected).toBe(true)
    expect(f1.properties.rocd_ft_min).toBe(1500)
    expect(f2.properties.geometric_altitude_ft).toBeNull()
    expect(f2.properties.barometric_altitude_ft).toBeNull()
    expect(f2.properties.qnh_corrected).toBe(false)
    expect(f2.properties.rocd_ft_min).toBeNull()
  })

  it('drives the trend arrow from the rate (I062/220) with a ±300 ft/min dead-band', () => {
    const state = makeState()
    const base = { latitude: 50, longitude: 8, vx: 0, vy: 0, confirmed: true, coasting: false, track_num: 9 }
    // A single update with a strong climb rate yields ▲ immediately — no prior
    // FL needed, unlike the fallback heuristic.
    updateTracksLayer({ tracks: [{ ...base, rocd_ft_min: 1200, flight_level_ft: 20000 }] }, state, () => {}, () => {})
    expect(state.liveTrackFeatures[0].properties.vertical_trend).toBe('▲')
    // A rate inside the dead-band shows no arrow, even against a changed FL.
    updateTracksLayer({ tracks: [{ ...base, rocd_ft_min: 100, flight_level_ft: 22000 }] }, state, () => {}, () => {})
    expect(state.liveTrackFeatures[0].properties.vertical_trend).toBe('')
    // A strong descent rate yields ▼.
    updateTracksLayer({ tracks: [{ ...base, rocd_ft_min: -800, flight_level_ft: 21000 }] }, state, () => {}, () => {})
    expect(state.liveTrackFeatures[0].properties.vertical_trend).toBe('▼')
  })

  it('falls back to the FL-delta heuristic when no rate is present', () => {
    const state = makeState()
    const base = { latitude: 50, longitude: 8, vx: 0, vy: 0, confirmed: true, coasting: false, track_num: 11 }
    // No rocd_ft_min → establish FL baseline, then climb > 50 ft → ▲.
    updateTracksLayer({ tracks: [{ ...base, flight_level_ft: 10000 }] }, state, () => {}, () => {})
    expect(state.liveTrackFeatures[0].properties.vertical_trend).toBe('')
    updateTracksLayer({ tracks: [{ ...base, flight_level_ft: 12000 }] }, state, () => {}, () => {})
    expect(state.liveTrackFeatures[0].properties.vertical_trend).toBe('▲')
  })
})
