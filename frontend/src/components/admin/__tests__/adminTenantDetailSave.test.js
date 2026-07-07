// #211: the slimmed tenant config saves globally. Feature toggles are buffered
// locally (featureEdits) and take effect only on the single "Speichern"; both
// Speichern and Abbrechen return to the overview (emit 'back'). Source-level guards
// (project convention — no Vuetify mount) against the detail SFC.
import { describe, it, expect } from 'vitest'
import detail from '../AdminTenantDetail.vue?raw'

describe('AdminTenantDetail global save/cancel (#211)', () => {
  it('buffers feature toggles locally instead of persisting on flip', () => {
    expect(detail).toContain('const featureEdits = reactive({})')
    // The switch reads/writes the buffer, not the server directly.
    expect(detail).toContain(':model-value="featureEdits[e.key]"')
    expect(detail).toContain('@update:model-value="featureEdits[e.key] = $event"')
    // No immediate per-toggle persist handler survives.
    expect(detail).not.toContain('toggleFeature')
  })

  it('offers one global Speichern plus an Abbrechen', () => {
    expect(detail).toContain('@click="saveAll"')
    expect(detail).toContain('@click="cancel"')
    expect(detail).toContain('>Speichern</v-btn>')
    // The old standalone "Ansicht speichern" button is gone.
    expect(detail).not.toContain('>Ansicht speichern</v-btn>')
  })

  it('persists the view and only the changed entitlements, then returns', () => {
    expect(detail).toContain('await admin.saveTenantView(props.tenantId, buildViewDto())')
    expect(detail).toContain('await admin.setTenantEntitlement(props.tenantId, e.key, desired)')
    // saveAll returns to the overview; cancel returns without persisting.
    expect(detail).toMatch(/async function saveAll\(\)[\s\S]*emit\('back'\)/)
    expect(detail).toMatch(/function cancel\(\)\s*\{\s*emit\('back'\)/)
  })
})
