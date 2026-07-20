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

describe('nextMaster (select-all / none on click)', () => {
  it('turns everything ON only from the all-off state', () => {
    expect(nextMaster('off')).toBe(true)
  })

  it('turns everything OFF from on or mixed (any-on collapses to none)', () => {
    expect(nextMaster('on')).toBe(false)
    expect(nextMaster('mixed')).toBe(false)
  })
})
