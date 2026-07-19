// ASD-018 (overlay-zone layout, ADR 0029): the durable fix for the recurring
// "new chrome overlaps the map controls" bug. The right-edge chrome is ONE flex
// column (a zone); the viewport controls are its last flex CHILD, so they flow
// below whatever grows above them — no component carries a guessed absolute
// offset. These source-guards pin the structure (there is no WebGL/mount harness
// for a visual assertion — the operator confirms the look).
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, it, expect } from 'vitest'

const raw = (rel) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
const asdView = raw('../AsdView.vue')
const mapCanvas = raw('../../components/MapCanvas.vue')
const mapControls = raw('../../components/MapControls.vue')
const viewportControls = raw('../../components/ViewportControls.vue')

describe('ASD-018 overlay-zone layout (ADR 0029)', () => {
  it('ViewportControls carries NO absolute offset of its own (positioned by its zone)', () => {
    // The whole point: the controls are a flex child, not a free-floating element
    // with a guessed top/right. A reintroduced absolute offset would be the bug.
    expect(viewportControls).not.toMatch(/position:\s*absolute/)
    expect(viewportControls).not.toMatch(/\btop:\s/)
  })

  it('AsdView places the controls as the LAST child of the right-rail zone, desktop only', () => {
    expect(asdView).toMatch(/<ViewportControls\s+v-if="mdAndUp"/)
    expect(asdView).toContain('@recenter="mapCanvas?.recenter()"')
    // It comes AFTER the search in the cluster (i.e. it is the last flex child,
    // so everything above pushes it down instead of overlapping it).
    const search = asdView.indexOf('<MapSearch')
    const controls = asdView.indexOf('<ViewportControls')
    expect(search).toBeGreaterThan(-1)
    expect(controls).toBeGreaterThan(search)
    // Still inside the one positioned flex column that is the zone.
    expect(asdView).toContain('top-right-cluster')
  })

  it('MapControls no longer hard-codes a desktop top offset (the old bug source)', () => {
    // The recurring overlap came from `top: calc(... + 140px)` guessing the
    // cluster height. Mobile-only MapControls anchors to the BOTTOM instead.
    expect(mapControls).not.toMatch(/top:\s*calc\([^)]*\d+px\)/)
    expect(mapControls).toMatch(/bottom:\s*calc\(/)
  })

  it('MapCanvas renders MapControls on both desktop + mobile and exposes recenter', () => {
    // ASD-019: zoom moved onto the scope, so MapControls renders unconditionally
    // now (it gates its viewport actions to mobile internally). The desktop
    // top-right cluster still drives recenter, so MapCanvas keeps exposing it.
    expect(mapCanvas).not.toMatch(/<MapControls\s+v-if="!mdAndUp"/)
    expect(mapCanvas).toContain('recenter: () => mapEngine?.recenter()')
  })

  it('desktop and mobile share ONE viewport-control component (no duplication)', () => {
    // MapControls composes the same ViewportControls the desktop cluster uses, but
    // renders it only on mobile (desktop has it in AsdView's top-right cluster).
    expect(mapControls).toContain("import ViewportControls from './ViewportControls.vue'")
    expect(mapControls).toMatch(/<ViewportControls\s+v-if="!mdAndUp"/)
    expect(mapControls).toMatch(/<ViewportControls[^>]*@recenter="\$emit\('recenter'\)"/)
  })

  it('the fullscreen icon state derives from the fullscreenchange event (ESC-safe)', () => {
    // ASD-018 follow-up: the browser fires fullscreenchange on EVERY exit —
    // including ESC/F11 the button never sees. Deriving isFullscreen from the
    // event (not the click handler's promise) keeps the icon correct. Without
    // the listener an ESC exit left the icon stuck on "exit fullscreen".
    expect(viewportControls).toContain("addEventListener('fullscreenchange'")
    expect(viewportControls).toContain("removeEventListener('fullscreenchange'")
    expect(viewportControls).toMatch(/isFullscreen\.value\s*=\s*!!document\.fullscreenElement/)
  })
})
