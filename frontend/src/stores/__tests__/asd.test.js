import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAsdStore } from '@/stores/asd.js'

beforeEach(() => {
  setActivePinia(createPinia())
})

// #176: the standalone "Lufträume" parent toggle was removed; the airspace layer
// visibility is derived from the four group toggles (visible iff any is on).
describe('asd store — airspace layer visibility derived from groups (#176)', () => {
  it('stays visible while any group is on, hides when all are off', () => {
    const s = useAsdStore()
    expect(s.layerVisibility.airspace).toBe(true) // all groups on by default
    s.setAirspaceGroup('ctr', false)
    s.setAirspaceGroup('tma', false)
    s.setAirspaceGroup('restricted', false)
    expect(s.layerVisibility.airspace).toBe(true) // info still on
    s.setAirspaceGroup('info', false)
    expect(s.layerVisibility.airspace).toBe(false) // all off → layer hidden
    s.setAirspaceGroup('tma', true)
    expect(s.layerVisibility.airspace).toBe(true) // one back on → visible
    expect(s.airspaceGroupVisibility.tma).toBe(true)
  })

  it('toggleAirspaceGroup flips a group and re-derives visibility', () => {
    const s = useAsdStore()
    ;['ctr', 'tma', 'restricted', 'info'].forEach((g) => s.setAirspaceGroup(g, false))
    expect(s.layerVisibility.airspace).toBe(false)
    s.toggleAirspaceGroup('ctr')
    expect(s.airspaceGroupVisibility.ctr).toBe(true)
    expect(s.layerVisibility.airspace).toBe(true)
  })
})

// #117: the broadcast FeedStatusMessage speaks per-feed colors
// (green/yellow/red); the store maps them to chip states and aggregates
// worst-wins across all subscribed feeds.
describe('asd store — per-feed health aggregation (#117)', () => {
  it('starts unknown until the first feed status arrives', () => {
    const s = useAsdStore()
    expect(s.feedStatus).toBe('unknown')
  })

  it('maps the wire colors to chip states', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'green')
    expect(s.feedStatus).toBe('ok')
    s.setFeedHealth(1, 'yellow')
    expect(s.feedStatus).toBe('degraded')
    s.setFeedHealth(1, 'red')
    expect(s.feedStatus).toBe('stale')
  })

  it('aggregates worst-wins across feeds (a dead feed is never masked)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'green')
    s.setFeedHealth(2, 'red')
    expect(s.feedStatus).toBe('stale')
    s.setFeedHealth(2, 'green')
    expect(s.feedStatus).toBe('ok')
  })

  it('ignores an unknown color instead of corrupting the chip', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'green')
    s.setFeedHealth(1, 'chartreuse')
    expect(s.feedStatus).toBe('ok')
  })

  it('resetFeedHealth returns to unknown (WS reconnect drops stale scope)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'red')
    s.resetFeedHealth()
    expect(s.feedStatus).toBe('unknown')
  })

  it('surfaces the degraded reason from the CAT063 RE field (#197)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'yellow', 'auth')
    expect(s.feedStatus).toBe('degraded')
    expect(s.feedDegradedReason).toBe('auth')
  })

  it('shows no reason unless the aggregate state is degraded', () => {
    const s = useAsdStore()
    // A healthy feed with a (stale) reason must not leak a reason onto the chip.
    s.setFeedHealth(1, 'green', '')
    expect(s.feedDegradedReason).toBe('')
    // A dead (stale) feed outranks degraded, so no degraded reason is shown.
    s.setFeedHealth(2, 'red', '')
    s.setFeedHealth(3, 'yellow', 'unreachable')
    expect(s.feedStatus).toBe('stale')
    expect(s.feedDegradedReason).toBe('')
  })

  it('picks the most actionable reason across degraded feeds (auth > rate_limited > unreachable)', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'yellow', 'unreachable')
    s.setFeedHealth(2, 'yellow', 'auth')
    s.setFeedHealth(3, 'yellow', 'rate_limited')
    expect(s.feedDegradedReason).toBe('auth')
  })

  it('resetFeedHealth also clears reasons', () => {
    const s = useAsdStore()
    s.setFeedHealth(1, 'yellow', 'auth')
    s.resetFeedHealth()
    expect(s.feedDegradedReason).toBe('')
  })
})

// WX-A: DWD weather-radar overlay is off by default and gated by an
// availability flag set from /api/map-config (weather_radar_available).
describe('asd store — weather radar overlay (WX-A)', () => {
  it('is off by default and reports unavailable until configured', () => {
    const s = useAsdStore()
    expect(s.layerVisibility.weatherRadar).toBe(false)
    expect(s.weatherRadarAvailable).toBe(false)
  })

  it('setWeatherRadarAvailable coerces to a boolean', () => {
    const s = useAsdStore()
    s.setWeatherRadarAvailable(true)
    expect(s.weatherRadarAvailable).toBe(true)
    s.setWeatherRadarAvailable(0)
    expect(s.weatherRadarAvailable).toBe(false)
  })

  it('setLayerVisibility toggles the weatherRadar layer', () => {
    const s = useAsdStore()
    s.setLayerVisibility('weatherRadar', true)
    expect(s.layerVisibility.weatherRadar).toBe(true)
  })
})

// WX-C: DWD weather-warnings overlay — same off-by-default + availability gate.
describe('asd store — weather warnings overlay (WX-C)', () => {
  it('is off by default and reports unavailable until configured', () => {
    const s = useAsdStore()
    expect(s.layerVisibility.weatherWarnings).toBe(false)
    expect(s.weatherWarningsAvailable).toBe(false)
  })

  it('setWeatherWarningsAvailable coerces to a boolean', () => {
    const s = useAsdStore()
    s.setWeatherWarningsAvailable(1)
    expect(s.weatherWarningsAvailable).toBe(true)
    s.setWeatherWarningsAvailable('')
    expect(s.weatherWarningsAvailable).toBe(false)
  })
})

// ASD-013: the live-track set feeds the Ereignis-Panel's decision whether a
// "Track N erschienen" event still points at a selectable track.
describe('asd store — live-track set for the event panel (ASD-013)', () => {
  it('starts empty and mirrors the numbers pushed from the engine', () => {
    const s = useAsdStore()
    expect(s.liveTrackNums.size).toBe(0)
    s.setLiveTrackNums([12, 36, 7])
    expect(s.liveTrackNums.has(36)).toBe(true)
    expect(s.liveTrackNums.has(99)).toBe(false)
    expect(s.liveTrackNums.size).toBe(3)
  })

  it('accepts a Set directly and replaces the previous set', () => {
    const s = useAsdStore()
    s.setLiveTrackNums([1, 2])
    s.setLiveTrackNums(new Set([2, 3]))
    expect(s.liveTrackNums.has(1)).toBe(false) // replaced, not merged
    expect(s.liveTrackNums.has(3)).toBe(true)
  })
})
