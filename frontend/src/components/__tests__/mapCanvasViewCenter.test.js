// FR-UI-013: the ASD map must open on the tenant's own sector (the effective
// view centre from whoami), not the global WAYFINDER_MAP_CENTER_* default. The
// map engine is a MapLibre wrapper without a unit harness, so — consistent with
// the house style for component wiring — we pin the wiring at the source level:
// MapCanvas must hand the session's viewCenter to initMap at mount AND re-aim on
// later changes (e.g. an admin switching the impersonation target).
import { describe, it, expect } from 'vitest'
import sfc from '../MapCanvas.vue?raw'

describe('MapCanvas view-centre wiring (FR-UI-013)', () => {
  it('reads the session store', () => {
    expect(sfc).toContain("import { useSessionStore } from '@/stores/session.js'")
    expect(sfc).toContain('useSessionStore()')
  })

  it('passes the tenant view centre into initMap at mount', () => {
    expect(sfc).toContain('session.viewCenter')
    // The initMap call carries it as the initial-centre argument.
    expect(sfc).toMatch(/initMap\([\s\S]*session\.viewCenter[\s\S]*\)/)
  })

  it('re-aims the camera when the effective view centre changes', () => {
    expect(sfc).toMatch(/watch\(\(\)\s*=>\s*session\.viewCenter/)
    expect(sfc).toContain('applyViewCenter')
  })
})
