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
