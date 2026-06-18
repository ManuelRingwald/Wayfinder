import { describe, it, expect } from 'vitest'
import { isFlFiltered, flOpacity } from '../tracks.js'

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
