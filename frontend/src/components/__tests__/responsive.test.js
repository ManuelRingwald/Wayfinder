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
import navigationRail from '../NavigationRail.vue?raw'
import adminFeeds from '../admin/AdminFeeds.vue?raw'

// CSS/HTML `?raw` imports come back empty under Vitest's transform, so read the
// files directly for the foundation assertions.
const read = (rel) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
const base = read('../../design/base.css')
const indexHtml = read('../../../index.html')
const spacingTokens = read('../../design/tokens/spacing.css')

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
  it('the track-detail card and scope legend use fluid, token-driven widths', () => {
    // #194 Häppchen 3: the base width is a token (so it grows a step on 24″),
    // still capped by the viewport via min().
    expect(trackDetail).toContain('width: min(var(--wf-overlay-detail-width, 292px)')
    expect(scopeLegend).toContain('width: min(var(--wf-overlay-legend-width, 232px)')
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

describe('iPad / tablet-landscape rail (#194 Häppchen 2)', () => {
  it('base.css widens the rail + panel tokens on the md band (960–1279px)', () => {
    expect(base).toContain('@media (min-width: 960px) and (max-width: 1279.98px)')
    expect(base).toContain('--wf-nav-rail-width: 76px')
    expect(base).toContain('--wf-nav-panel-width: 304px')
  })
  it('NavigationRail is token-driven and touch-sizes on the md band', () => {
    // Width from the token, not a hardcoded 56px, so the CSS band drives it.
    expect(navigationRail).toContain('width: var(--wf-nav-rail-width')
    // `md` (exactly the tablet-landscape band) toggles the touch treatment.
    expect(navigationRail).toContain('const { mdAndUp, md } = useDisplay()')
    expect(navigationRail).toContain("'nav-rail--touch': tabletLandscape")
    expect(navigationRail).toContain('.nav-rail--touch .nav-rail__pill')
    // 44px finger target + 24px icons on the tablet band.
    expect(navigationRail).toContain('width: 44px')
    expect(navigationRail).toContain('tabletLandscape.value ? 24 : 20')
  })
  it('NavigationRail widths: 76px rail / 304px panel on tablet, 56/248 desktop', () => {
    expect(navigationRail).toContain('const rail = tabletLandscape.value ? 76 : 56')
    expect(navigationRail).toContain('const panel = tabletLandscape.value ? 304 : 248')
  })
  it('the floating overlays derive their left offset from the rail-width token', () => {
    // So a wider iPad rail shifts the legend + detail card in lockstep (no
    // hardcoded 68px that would overlap the 76px rail).
    const offset = 'calc(var(--wf-nav-rail-width, 56px) + var(--wf-overlay-gap, 12px))'
    expect(asdView).toContain(offset)
    expect(trackDetail).toContain(offset)
    // The actual CSS declaration is the derived calc, not a hardcoded 68px
    // (explanatory comments may still mention 68px as the desktop value).
    expect(asdView).not.toMatch(/left:\s*68px;/)
    expect(trackDetail).not.toMatch(/left:\s*68px;/)
  })
  it('MapControls buttons reach a 44px target on the md band', () => {
    expect(mapControls).toContain("'map-controls--touch': tabletLandscape")
    expect(mapControls).toContain('.map-controls--touch .map-controls__group :deep(.v-btn)')
    expect(mapControls).toContain('var(--wf-touch-min, 44px)')
  })
})

describe('large display / 24" scaling (#194 Häppchen 3)', () => {
  it('spacing tokens declare overlay width defaults; base.css steps them up on xl', () => {
    expect(spacingTokens).toContain('--wf-overlay-legend-width: 232px')
    expect(spacingTokens).toContain('--wf-overlay-detail-width: 292px')
    // xl band (≥1920px) widens the gap + overlay widths one step so 24" breathes.
    expect(base).toContain('@media (min-width: 1920px)')
    expect(base).toContain('--wf-overlay-gap: 20px')
    expect(base).toContain('--wf-overlay-legend-width: 268px')
    expect(base).toContain('--wf-overlay-detail-width: 336px')
  })
  it('the ASD edge insets derive from the overlay-gap token so they breathe on xl', () => {
    // Top-right cluster, scope legend and map controls all read the gap token
    // rather than a hardcoded 12px, so the xl step reaches every corner.
    expect(asdView).toContain('top: calc(var(--wf-overlay-gap, 12px) + var(--wf-safe-top, 0px))')
    expect(asdView).toContain('bottom: var(--wf-overlay-gap, 12px)')
    expect(mapControls).toContain('right: calc(var(--wf-overlay-gap, 12px) + var(--wf-safe-right, 0px))')
  })
})

describe('admin panel large + narrow (#194 Häppchen 4)', () => {
  it('the admin content column widens a step on a 24" display', () => {
    expect(adminView).toContain('.admin-container {\n  max-width: 1180px;\n}')
    expect(adminView).toContain('@media (min-width: 1920px)')
    expect(adminView).toContain('max-width: 1440px')
  })
  it('admin dialogs cap to the viewport on a narrow phone', () => {
    // A 460/520/720px dialog would overflow a 360px phone; min(px, 94vw) caps it.
    expect(adminFeeds).toContain('max-width="min(720px, 94vw)"')
    expect(adminFeeds).toContain('max-width="min(520px, 94vw)"')
    // No bare numeric max-width left on an admin dialog.
    expect(adminFeeds).not.toMatch(/<v-dialog[^>]*max-width="\d+"/)
  })
})
