// FR-UI-013: the ASD map must open on the tenant's own sector (the effective
// view centre from whoami), not the global WAYFINDER_MAP_CENTER_* default. The
// map engine is a MapLibre wrapper without a unit harness, so — consistent with
// the house style for component wiring — we pin the wiring at the source level:
// MapCanvas must hand the session's viewCenter to initMap at mount AND re-aim on
// later changes (e.g. an admin switching the impersonation target).
import { describe, it, expect } from 'vitest'
import sfc from '../MapCanvas.vue?raw'
import engine from '../../map/engine.js?raw'
import viewportControls from '../ViewportControls.vue?raw'

describe('recenter restores the full start view (#169)', () => {
  it('recenter resets bearing and pitch, not just centre + zoom', () => {
    expect(engine).toMatch(/function recenter\(\)\s*\{[\s\S]*bearing:\s*0[\s\S]*pitch:\s*0[\s\S]*\}/)
  })

  it('the control is relabelled "Ansicht zurücksetzen" (no longer "Zentrum")', () => {
    // ASD-018: the recenter button (with its label) moved into ViewportControls.
    expect(viewportControls).toContain('Ansicht zurücksetzen')
    expect(viewportControls).not.toContain('text="Zentrum"')
  })
})

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

  // #219: initMap is async; when an admin enters read-only guest mode the
  // impersonation-aware whoami can land WHILE initMap awaits, so the viewCenter/aoi
  // watchers fire against a still-null mapEngine and the re-aim is lost — the map
  // (and "Ansicht zurücksetzen") stays pinned to the global Frankfurt default. After
  // initMap resolves, MapCanvas must reconcile the map to the CURRENT effective view.
  it('reconciles centre + AOI after initMap resolves so a late whoami is not lost (#219)', () => {
    expect(sfc).toMatch(/mapEngine\s*=\s*await initMap\([\s\S]*?\)\s*[\s\S]*?mapEngine\.applyViewCenter\(session\.viewCenter\)/)
    expect(sfc).toContain('mapEngine.applyWeatherAOI(session.aoi)')
  })
})

// #179: the airspace type filter must be applied on EVERY map mount, not only on
// the first one. store.mapLoaded is a write-once-true latch on the singleton
// Pinia store, so the false→true edge that the MapCanvas watcher keys on never
// fires on a second mount (logout→login / tenant switch / re-login without a
// full reload). Without the initial filter the airspace layers keep their wide
// defaults and render non-mapped, country-wide types (UIR/FIR/ADIZ/TRA …). The
// fix makes the engine apply the filter itself in its load handler and reset the
// latch on teardown. Pinned at the source level (no MapLibre unit harness).
describe('airspace filter is applied on every mount (#179)', () => {
  it('the load handler applies the airspace filter right after setMapLoaded(true)', () => {
    expect(engine).toMatch(/store\.setMapLoaded\(true\)[\s\S]*?updateAirspaceFilter\(\)/)
  })

  it('destroy() resets the mapLoaded latch so the next mount re-fires the edge', () => {
    expect(engine).toMatch(/function destroy\(\)\s*\{[\s\S]*store\.setMapLoaded\(false\)[\s\S]*\}/)
  })
})

// ASD-014: the map highlights the tenant's Area of Responsibility (CTR/TMA) from
// whoami.aor_airspace_ids. Same source-level wiring guard as the view centre —
// no MapLibre unit harness exists for the paint/filter plumbing.
describe('AoR highlight wiring (ASD-014)', () => {
  it('the engine adds the AoR layer above the airspace layers and exposes updateAoR', () => {
    expect(engine).toMatch(/addAirspaceLayers\(map, palette\)[\s\S]*?addAirspaceAoRLayer\(map\)/)
    expect(engine).toMatch(/function updateAoR\(ids\)/)
    expect(engine).toMatch(/return \{[\s\S]*updateAoR[\s\S]*\}/)
  })

  it('MapCanvas applies the AoR after initMap resolves and re-applies on change', () => {
    expect(sfc).toContain('mapEngine.updateAoR(session.aorAirspaceIds)')
    expect(sfc).toMatch(/watch\(\(\)\s*=>\s*session\.aorAirspaceIds/)
  })
})
