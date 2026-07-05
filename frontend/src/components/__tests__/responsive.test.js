// #194: responsive wiring for iPhone / iPad / large displays. There is no
// component-mount / WebGL harness, so — like the other UI tests — we assert the
// wiring against the raw source at the three key seams: the safe-area foundation,
// the mobile bottom tab bar + sheets, and the fluid overlays.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, it, expect } from 'vitest'
import bottomNav from '../BottomNav.vue?raw'
import asdView from '../../views/AsdView.vue?raw'
import adminView from '../../views/AdminView.vue?raw'
import mapControls from '../MapControls.vue?raw'
import trackDetail from '../TrackDetailPanel.vue?raw'
import scopeLegend from '../ScopeLegend.vue?raw'

// CSS/HTML `?raw` imports come back empty under Vitest's transform, so read the
// files directly for the foundation assertions.
const read = (rel) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
const base = read('../../design/base.css')
const indexHtml = read('../../../index.html')

describe('safe-area foundation (#194)', () => {
  it('index.html opts into viewport-fit=cover', () => {
    expect(indexHtml).toContain('viewport-fit=cover')
  })
  it('base.css normalises the safe-area insets to --wf-safe-* tokens', () => {
    expect(base).toContain('--wf-safe-top: env(safe-area-inset-top')
    expect(base).toContain('--wf-safe-bottom: env(safe-area-inset-bottom')
    expect(base).toContain('--wf-bottom-nav-h')
    expect(base).toContain('--wf-touch-min')
  })
})

describe('mobile bottom tab bar (#194)', () => {
  it('BottomNav lists Scope/Filter/Konto and pads past the home indicator', () => {
    for (const t of ['Scope', 'Filter', 'Konto']) expect(bottomNav).toContain(t)
    expect(bottomNav).toContain('var(--wf-safe-bottom')
    // 44px minimum touch target.
    expect(bottomNav).toContain('var(--wf-touch-min')
  })
  it('BottomNav gates the Admin tab behind isAdmin', () => {
    expect(bottomNav).toContain('isAdmin')
    expect(bottomNav).toContain("id: 'admin'")
  })
})

describe('AsdView mobile branch (#194)', () => {
  it('renders the rail only on >=md and the bottom nav + sheets below it', () => {
    expect(asdView).toContain('v-if="mdAndUp"') // rail is desktop/tablet-landscape only
    expect(asdView).toContain('<BottomNav')
    expect(asdView).toContain('v-model="filterSheet"')
    expect(asdView).toContain('v-model="kontoSheet"')
    // The old hamburger menu button is gone.
    expect(asdView).not.toContain('mobile-menu-btn')
  })
  it('routes the Admin tab and keeps sheets in sync with the tab bar', () => {
    expect(asdView).toContain("router.push('/admin')")
    expect(asdView).toContain('closeSheets')
  })
})

describe('fluid overlays + mobile controls (#194)', () => {
  it('MapControls sits above the bottom nav on mobile', () => {
    expect(mapControls).toContain('map-controls--mobile')
    expect(mapControls).toContain('var(--wf-bottom-nav-h')
  })
  it('the track-detail card and scope legend use fluid widths', () => {
    expect(trackDetail).toContain('width: min(292px')
    expect(scopeLegend).toContain('width: min(232px')
  })
})

describe('admin panel responsive (#194)', () => {
  it('the section nav collapses to a select and actions go icon-only on phones', () => {
    // Desktop keeps the labelled button toggle; mobile uses a compact select.
    expect(adminView).toContain('v-if="mdAndUp"')
    expect(adminView).toContain('admin-section-select')
    expect(adminView).toContain('sectionItems')
    // Icon-only fallbacks for the actions on small screens.
    expect(adminView).toContain('v-else')
    expect(adminView).toContain('useDisplay')
  })
})

describe('dense tables scroll inside their card on narrow screens (#194)', () => {
  it('base.css makes the Vuetify table wrapper scroll horizontally', () => {
    expect(base).toContain('.v-table__wrapper')
    expect(base).toContain('overflow-x: auto')
  })
})
