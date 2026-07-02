// Regression guard: the RBL/DIST/QDM distance/bearing readout floats as a label
// AT the measure line (anchored to the A–B midpoint), not only in the bottom hint.
// Source-level assertions (project convention — no Vuetify/map mount).
import { describe, it, expect } from 'vitest'
import status from '../MeasureStatus.vue?raw'
import measure from '../../map/measure.js?raw'

describe('measure readout floats at the line', () => {
  it('measure.js projects the midpoint and reports it as the label anchor', () => {
    expect(measure).toContain('function labelAnchor')
    expect(measure).toContain('map.project(')
    // reported alongside the text, and kept glued on map move
    expect(measure).toContain('labelAnchor()')
    expect(measure).toContain("map.on('move', reproject)")
  })

  it('MeasureStatus positions the label at the reported anchor', () => {
    expect(status).toContain('measure-label')
    expect(status).toContain('readoutAt.x')
    expect(status).toContain('readoutAt.y')
  })

  it('the bottom hint no longer carries the readout (only the instruction)', () => {
    expect(status).not.toContain('measure-hint__readout')
  })
})
