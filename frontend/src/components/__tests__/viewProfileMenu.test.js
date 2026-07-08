// VP-4 (ADR 0023): the view-profile switcher + save dialog. Like the other ASD
// chrome (see eventPanel.test.js), there is no component-mount harness, so assert
// the wiring against the raw source. The store logic itself is covered by
// stores/__tests__/profiles.test.js and profileSettings.test.js.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, it, expect } from 'vitest'

const read = (rel) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
const menu = read('../ViewProfileMenu.vue')
const asdView = read('../../views/AsdView.vue')

describe('ViewProfileMenu wiring (VP-4)', () => {
  it('drives the profiles store', () => {
    expect(menu).toContain('useProfilesStore')
    expect(menu).toContain('store.load()') // loads on mount
    expect(menu).toContain('onMounted')
  })
  it('wires select / set-default / delete on each profile', () => {
    expect(menu).toContain('store.apply(')
    expect(menu).toContain('store.setDefault(')
    expect(menu).toContain('store.remove(')
  })
  it('saves the current view (with make-default) and supports rename', () => {
    expect(menu).toContain('store.saveCurrent(n, makeDefault.value)')
    expect(menu).toContain('store.rename(')
  })
  it('offers "make default on login" and reflects the default with a star', () => {
    expect(menu).toContain('Als Standard beim Login')
    expect(menu).toContain('mdi-star')
  })
  it('gates saving at the three-profile cap', () => {
    expect(menu).toContain('store.canCreate')
    expect(menu).toContain('Maximal 3')
  })
})

describe('AsdView mounts the switcher (VP-4)', () => {
  it('imports and renders ViewProfileMenu in the header cluster', () => {
    expect(asdView).toContain("import ViewProfileMenu from '@/components/ViewProfileMenu.vue'")
    expect(asdView).toContain('<ViewProfileMenu')
  })
})
