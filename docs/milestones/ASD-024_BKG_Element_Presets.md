# ASD-024 (E3) — BKG-Element-Presets „Minimal / Standard / Detailliert"

> **Register:** FR-UI-048 · **Bezug:** ADR 0031, Epic #290, Issue #294 (E3).
> Baut auf **E2** (#293, Element-Schalter) + **E4** (#295, Persistenz).

## Fachlich — warum

Mit E2 hat der Lotse acht Element-Schalter. Im Alltag will er selten jeden
einzeln stellen, sondern schnell einen sinnvollen Satz wählen. E3 gibt ihm drei
**1-Klick-Presets** über den Element-Schaltern:

- **Minimal** — nur Orientierung: Gewässer (Küste/Flüsse), Grenzen, Beschriftung
  auf dem blanken Scope (Verkehr/Vegetation/Siedlung/Gebäude/Hintergrund aus).
- **Standard** — eine saubere Betriebs-Karte: zusätzlich Verkehr + Hintergrund.
- **Detailliert** — alle Elemente an.

Passt der Lotse danach einen Einzelschalter an, wechselt die Anzeige auf
**„Benutzerdefiniert"** (kein Preset hervorgehoben).

## Technisch

- **`map/basemapGroups.js`**: `BASEMAP_PRESETS` (jedes Preset weist **jedem**
  Element einen Wert zu — ein Test sichert das, damit Anwenden deterministisch
  ist) + `matchPreset(current)` (liefert die aktive Preset-ID oder `null` =
  benutzerdefiniert; verglichen über `BASEMAP_ELEMENTS`, damit extra/fehlende
  Schlüssel nicht falsch matchen).
- **Store (`asd.js`)**: `applyBasemapPreset(id)` mutiert `basemapElementVisibility`
  direkt — Vue bündelt die synchronen Mutationen, der MapCanvas-Element-Watcher
  (E2) feuert **einmal** → ein `applyBasemap`. Unbekannte ID = No-op.
- **Sidebar (`LayerFilterContent.vue`)**: kompakte Segment-Button-Reihe
  (`BASEMAP_PRESETS`) über den Element-Schaltern, **nur sichtbar wenn die Karte
  an ist**; der aktive Preset (`activeBasemapPreset` = `matchPreset(...)`) ist
  hervorgehoben.

**Persistenz kostenlos:** Ein Preset setzt nur die Element-Zustände, und die
werden bereits über E4 (#295) im View-Profil gespeichert — es muss also **kein**
Preset-Name persistiert werden; er wird beim Laden aus den Element-Zuständen
re-abgeleitet.

## Ehrliche Grenzen

- Die konkrete Element-Zusammenstellung je Preset ist eine **Design-Setzung**
  (in `BASEMAP_PRESETS`), leicht anpassbar; die Feinabstimmung am echten
  Kartenbild macht der Betreiber.
- Kein WebGL-/Mount-Harness → Wiring per Source-Guards; optische Abnahme durch
  den Betreiber.

## Tests

- `basemapGroups.test.js`: jedes Preset deckt jedes Element ab; „Detailliert" =
  alles an; Presets sind abgestuft (Minimal < Standard < Detailliert an-Anzahl);
  `matchPreset` (Treffer, `null` bei No-Match, kein Falsch-Treffer bei
  abweichendem Schlüssel, null-sicher).
- `basemapLayer.test.js` (E3-Block): `applyBasemapPreset` setzt Elemente,
  unbekannte ID = No-op; Sidebar rendert die Preset-Buttons (nur bei Karte-an),
  aktiver Preset hervorgehoben, `matchPreset`-Verdrahtung.
