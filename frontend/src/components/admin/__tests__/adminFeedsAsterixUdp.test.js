// Regression guard for the local ASTERIX-over-UDP source types adsb_asterix
// (#239, Firefly contract v1.6.0) and mlat_asterix (#240, v1.7.0):
//  - both appear in the closed vocabulary with clean labels;
//  - they form a third form category (neither area nor radar): a listen endpoint
//    + optional SAC/SIC + optional sensor_id, no bbox/location;
//  - the payload sends listen/sac/sic/sensor_id and never a bbox or lat/lon;
//  - they are auth-free: NO CREDENTIAL entry, so no credential block / cred_ref;
//  - the form round-trip helpers carry sensor_id.
// Source-level assertions (project convention — no Vuetify mount).
import { describe, it, expect } from 'vitest'
import sfc from '../AdminFeeds.vue?raw'

describe('ASTERIX-over-UDP source types (#239/#240)', () => {
  it('offers both types in the vocabulary with clean labels', () => {
    expect(sfc).toContain("{ value: 'adsb_asterix', label: 'ADS-B (Bodenstation, CAT021/UDP)' }")
    expect(sfc).toContain("{ value: 'mlat_asterix', label: 'WAM/MLAT (CAT020/019/UDP)' }")
  })

  it('classes them as a third category, not area-bounded', () => {
    expect(sfc).toContain("ASTERIX_UDP_TYPES = new Set(['adsb_asterix', 'mlat_asterix'])")
    // Must not be folded into the area set (no bbox / centre+radius editor).
    expect(sfc).toContain("AREA_TYPES = new Set(['adsb_opensky', 'adsb_aggregator', 'flarm_aprs'])")
  })

  it('renders a dedicated form branch with listen + sac/sic + sensor_id', () => {
    expect(sfc).toContain(`v-else-if="isAsterixUdpType(s.type)"`)
    expect(sfc).toContain('label="Sensor-ID (optional)"')
  })

  it('sends listen/sac/sic/sensor_id and no bbox/location in the payload', () => {
    expect(sfc).toContain('} else if (isAsterixUdpType(s.type)) {')
    expect(sfc).toContain('out.sensor_id = Number(s.sensor_id)')
    // The dedicated payload branch must not emit bbox or lat/lon — those belong to
    // the area and radar branches.
    const start = sfc.indexOf('} else if (isAsterixUdpType(s.type)) {')
    const end = sfc.indexOf('} else {', start)
    const branch = sfc.slice(start, end)
    expect(branch).not.toContain('out.bbox')
    expect(branch).not.toContain('out.lat')
  })

  it('carries sensor_id through the form round-trip helpers', () => {
    expect(sfc).toContain('sensor_id: null')
    expect(sfc).toContain('sensor_id: s.sensor_id ?? null')
  })

  it('are auth-free: no CREDENTIAL entry for either type', () => {
    const credBlock = sfc.slice(sfc.indexOf('const CREDENTIAL = {'), sfc.indexOf('function credInfo'))
    expect(credBlock).not.toContain('adsb_asterix')
    expect(credBlock).not.toContain('mlat_asterix')
  })
})
