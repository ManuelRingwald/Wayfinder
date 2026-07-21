// H4 (#197): the feed-status chip shows WHY a source is degraded, decoded from
// the CAT063 I063/RE SRC-REASON (Firefly ADR 0033). There is no component-mount
// harness for the chip, so — like the other UI tests — assert the wiring against
// the raw source: it must read the store's degraded reason and label the three
// reason codes.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, it, expect } from 'vitest'

const read = (rel) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
const chip = read('../FeedStatusChip.vue')

describe('FeedStatusChip degraded reason (#197)', () => {
  it('reads the degraded reason from the store', () => {
    expect(chip).toContain('store.feedDegradedReason')
  })
  it('labels every SRC-REASON code (unreachable / auth / rate_limited)', () => {
    for (const code of ['unreachable', 'auth', 'rate_limited']) {
      expect(chip).toContain(`${code}:`)
    }
  })
  it('appends the reason label to the degraded chip and carries a tooltip', () => {
    expect(chip).toContain("store.feedStatus === 'degraded'")
    expect(chip).toContain(':title="chipTitle"')
  })
})

describe('FeedStatusChip SDPS (service-level) degradation (#261)', () => {
  it('reads the SDPS-degraded flag from the store', () => {
    expect(chip).toContain('store.feedSdpsDegraded')
  })
  it('shows a distinct service-level label, not the sensor-fusion one', () => {
    expect(chip).toContain('DIENST DEGRADIERT')
  })
  it('checks the SDPS branch before composing the sensor label (precedence)', () => {
    const sdpsIdx = chip.indexOf('feedSdpsDegraded')
    const sensorLabelIdx = chip.indexOf("'SENSOR AUSFALL'")
    expect(sdpsIdx).toBeGreaterThan(-1)
    expect(sensorLabelIdx).toBeGreaterThan(sdpsIdx)
  })
})
