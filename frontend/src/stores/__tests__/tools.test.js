import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useToolsStore } from '@/stores/tools.js'

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('tools store — controller measurement tools (Häppchen 4)', () => {
  it('starts with no active tool and no readout', () => {
    const s = useToolsStore()
    expect(s.activeTool).toBe(null)
    expect(s.readout).toBe(null)
    expect(s.hint).toBe(null)
  })

  it('selectTool activates a tool and exposes its hint', () => {
    const s = useToolsStore()
    s.selectTool('rbl')
    expect(s.activeTool).toBe('rbl')
    expect(s.hint).toMatch(/RBL/)
  })

  it('selecting the same tool again toggles it off', () => {
    const s = useToolsStore()
    s.selectTool('dist')
    s.selectTool('dist')
    expect(s.activeTool).toBe(null)
  })

  it('switching tools clears the previous readout', () => {
    const s = useToolsStore()
    s.selectTool('rbl')
    s.setReadout('12.3 NM · 087°')
    expect(s.readout).toBe('12.3 NM · 087°')
    s.selectTool('qdm')
    expect(s.activeTool).toBe('qdm')
    expect(s.readout).toBe(null)
  })

  it('clearTool resets tool and readout', () => {
    const s = useToolsStore()
    s.selectTool('qdm')
    s.setReadout('5.0 NM · 010°')
    s.clearTool()
    expect(s.activeTool).toBe(null)
    expect(s.readout).toBe(null)
    expect(s.hint).toBe(null)
  })

  it('setReadout carries the screen anchor for the floating label', () => {
    const s = useToolsStore()
    s.selectTool('rbl')
    s.setReadout('12.3 NM · 087°', { x: 100, y: 60 })
    expect(s.readoutAt).toEqual({ x: 100, y: 60 })
    // switching/clearing drops the anchor with the readout
    s.selectTool('dist')
    expect(s.readoutAt).toBe(null)
  })
})
