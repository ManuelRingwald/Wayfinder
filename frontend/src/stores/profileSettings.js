// View-profile settings (VP-3, ADR 0023): pure functions that serialise the ASD
// store's DISPLAY preferences into a plain object (a profile's `settings`) and
// apply such an object back. Kept pure (operate on the passed store, no Pinia
// import, no side effects beyond the store's own setters) so they are unit-testable
// with a fake store. Only display toggles are captured — NOT the map centre/zoom,
// which stay the tenant/user view framing (Option A, ADR 0023).

// SETTINGS_VERSION tags the captured shape so a future change can migrate or
// ignore an older blob. applySettings is tolerant of any version (best-effort).
export const SETTINGS_VERSION = 1

// finiteOrNull returns x when it is a finite number, else null — so a stale or
// malformed value never corrupts the FL filter / ring config.
function finiteOrNull(x) {
  return Number.isFinite(x) ? x : null
}

// captureSettings snapshots the ASD store's display preferences into a plain,
// JSON-serialisable object. Whatever layer keys the store carries are captured
// verbatim (forward-compatible: a new toggle is included automatically).
export function captureSettings(asd) {
  return {
    v: SETTINGS_VERSION,
    layers: { ...asd.layerVisibility },
    airspaceGroups: { ...asd.airspaceGroupVisibility },
    basemapElements: { ...asd.basemapElementVisibility }, // E4 (#293/#295): per-element base-map switches

    rangeRings: { spacingNM: asd.rangeRingConfig.spacingNM, count: asd.rangeRingConfig.count },
    history: { durationS: asd.historyConfig.durationS },
    flFilter: { minFL: asd.flFilter.minFL, maxFL: asd.flFilter.maxFL, hide: asd.flFilter.hide },
  }
}

// applySettings writes a captured settings object back onto the ASD store via its
// setters (the map follows through the existing MapCanvas watchers). It is
// deliberately TOLERANT: unknown/absent sections and keys are skipped and numbers
// are validated, so a partial or stale profile never throws or corrupts state.
export function applySettings(asd, settings) {
  if (!settings || typeof settings !== 'object') return

  // Layer toggles — skip the derived `airspace` key (it follows the airspace
  // groups applied below) and any key the current store does not know.
  if (settings.layers && typeof settings.layers === 'object') {
    for (const [k, v] of Object.entries(settings.layers)) {
      if (k === 'airspace') continue
      if (k in asd.layerVisibility) asd.setLayerVisibility(k, !!v)
    }
  }
  // Airspace category groups (this also re-derives layerVisibility.airspace).
  if (settings.airspaceGroups && typeof settings.airspaceGroups === 'object') {
    for (const [k, v] of Object.entries(settings.airspaceGroups)) {
      if (k in asd.airspaceGroupVisibility) asd.setAirspaceGroup(k, !!v)
    }
  }
  // E4 (#295): per-element base-map switches (Gewässer/Verkehr/…). Same tolerant
  // pattern — unknown keys skipped, so an older profile (or a future element set)
  // loads without error. Absent section → elements keep their all-on defaults.
  if (settings.basemapElements && typeof settings.basemapElements === 'object') {
    for (const [k, v] of Object.entries(settings.basemapElements)) {
      if (k in asd.basemapElementVisibility) asd.setBasemapElement(k, !!v)
    }
  }
  // Range-ring configuration (spacing / count) — only finite values.
  if (settings.rangeRings && typeof settings.rangeRings === 'object') {
    const u = {}
    if (Number.isFinite(settings.rangeRings.spacingNM)) u.spacingNM = settings.rangeRings.spacingNM
    if (Number.isFinite(settings.rangeRings.count)) u.count = settings.rangeRings.count
    if (Object.keys(u).length) asd.setRangeRingConfig(u)
  }
  // History-dot retention.
  if (settings.history && Number.isFinite(settings.history.durationS)) {
    asd.setHistoryConfig({ durationS: settings.history.durationS })
  }
  // FL filter (min/max may be null = no bound; hide is a boolean).
  if (settings.flFilter && typeof settings.flFilter === 'object') {
    asd.setFlFilter({
      minFL: finiteOrNull(settings.flFilter.minFL),
      maxFL: finiteOrNull(settings.flFilter.maxFL),
      hide: !!settings.flFilter.hide,
    })
  }
}
