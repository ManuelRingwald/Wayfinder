// ASD-020 (ADR 0031): the sidebar layer-group tri-state logic. Pure functions,
// unit-tested without a Vuetify mount (the membership + wiring is source-guarded
// separately in components/__tests__/layerGrouping.test.js).
import { describe, it, expect } from 'vitest'
import { masterState, nextMaster } from '../layerGroups.js'

describe('masterState (tri-state group master)', () => {
  it('is "empty" when there is nothing to control', () => {
    expect(masterState([])).toBe('empty')
    expect(masterState(undefined)).toBe('empty')
  })

  it('is "on" only when every member is on', () => {
    expect(masterState([true])).toBe('on')
    expect(masterState([true, true, true])).toBe('on')
  })

  it('is "off" only when every member is off', () => {
    expect(masterState([false])).toBe('off')
    expect(masterState([false, false])).toBe('off')
  })

  it('is "mixed" when members disagree', () => {
    expect(masterState([true, false])).toBe('mixed')
    expect(masterState([false, true, true])).toBe('mixed')
  })
})

describe('nextMaster (fill-then-clear on click, #315)', () => {
  it('turns everything ON from the all-off state', () => {
    expect(nextMaster('off')).toBe(true)
  })

  it('turns everything ON from the mixed state — a click selects/completes the group', () => {
    // #315: the previous rule collapsed a partially-active group to all-off,
    // so clicking Aeronautik (which starts partially on) deselected every
    // sub-layer instead of selecting them. Fill-then-clear turns it fully on.
    expect(nextMaster('mixed')).toBe(true)
  })

  it('turns everything OFF only from the already-fully-on state', () => {
    expect(nextMaster('on')).toBe(false)
  })
})
