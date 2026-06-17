# ADR 0002 — Frontend-Framework: Vue 3 + Vuetify 3 (Material Design 3)

- **Status:** akzeptiert
- **Datum:** 2026-06-17
- **Schnittstellen-relevant:** nein (kein CAT062-/ICD-Change; betrifft ausschließlich die
  Browser-seitige Präsentationsschicht)

## Kontext

Das Wayfinder-Frontend wurde in M1 als **Vanilla-JS-Einzeldatei** (`internal/webui/static/app.js`,
~1300 Zeilen) aufgebaut — pragmatisch für einen schnellen ersten Aufbau. Mit ASD-001 bis ASD-005
hat die Datei signifikante ASD-Fachlogik (Deconfliction, Fade-Out, FL-Filter, Drag&Drop-Pinning,
Feed-Banner) akkumuliert. Die UI besteht aus rohem HTML mit inline CSS — kein Design-System,
keine Komponenten-Kapselung, kein automatischer JS-Test-Harness.

Für einen produktiven ASD-Einsatz sind drei Defizite zu schließen:

1. **Wartbarkeit/Testbarkeit:** Die ASD-Fachlogik ist mit DOM-Manipulation verwoben; automatisierte
   Unit-Tests für JS-Logik fehlen vollständig (Register-Vermerk „manuell verifiziert" an allen
   FR-ASD-*-Einträgen).
2. **Design-Qualität:** Das UI ist funktional, aber ungeformt — kein konsistentes Design-System,
   keine MD3-Typographie, kein Spacing-System, keine Barrierefreiheit.
3. **Bedienbarkeit:** Konfigurierbare Layer-Steuerung und FL-Filter hängen als freischwebende
   Panels über dem Scope und verringern die nutzbare Kartenfläche.

Aus `CLAUDE.md` ADR-0002-Slot: **Frontend-UI-Framework** ist explizit als noch nicht ratifiziert
markiert und wird hier entschieden.

## Entscheidung

### 1. Framework: Vue 3 + Vuetify 3

**Vue 3** (Composition API, TypeScript-optional) als reaktives Frontend-Framework, kombiniert
mit **Vuetify 3** als MD3-Komponentenbibliothek. Vuetify 3 ist die vollständigste, aktiv
gewartete MD3-Implementierung im Vue-Ökosystem — Navigation Drawer, App Bar, Switches,
Text Fields, Cards und Chips sind als fertige MD3-Komponenten verfügbar.

### 2. Build-Toolchain: Vite + Vitest

**Vite** als Build-Tool (schnelle HMR, ES-Module nativ, produktionsreifes Rollup-Bundle).
**Vitest** als Test-Runner (Vite-nativ, ersetzt den fehlenden JS-Test-Harness):
Deconfliction-Algorithmus, FL-Filter-Logik, `buildLabel`, Palette-Auswahl und State-Mutations
werden erstmals automatisiert getestet.

### 3. Klare Schichtung — Chrome vs. Karten-Engine

Dies ist die **kritischste Architekturentscheidung** dieses ADR:

| Schicht | Technologie | Verantwortung |
|---------|-------------|---------------|
| **Karten-Engine** | Vanilla-JS-ES-Module (`src/map/*.js`) | MapLibre-Instanz, Track-Rendering, Deconfliction, Fade-Out, Leader Lines, Trails, Vektoren, Drag&Drop-Pinning, Palette — unveränderte ASD-Fachlogik |
| **Reaktiver State** | Pinia-Store (`src/stores/*.js`) | FL-Filter, Layer-Visibility, Feed-Status, Track-Metadaten, Label-Pins |
| **Chrome** | Vue 3 + Vuetify 3 | App-Bar, Navigation Drawer (Sidebar), Feed-Status-Chip, Track-Detail-Panel, A11y |

**Die ASD-Fachlogik (deconflictLabels, tickFade, renderSources, setupLabelDrag …) wird
aus `app.js` in framework-agnostische ES-Module herausgelöst und dabei inhaltlich nicht
verändert — nur modularisiert.** Das begrenzt das Regressions-Risiko drastisch.

### 4. Go-Embed-Umstellung

`npm run build` erzeugt `frontend/dist/`. Das Go-Backend embedded `frontend/dist/`
via `//go:embed` statt `internal/webui/static/`. Der Multi-Stage-Dockerfile bekommt
eine Node-Build-Stufe. Kein Backend-/API-/CAT062-Change.

### 5. Konfigurationsdatei für Karten-Defaults

Statt hartkodierter Defaults oder reiner Env-Var-Konfiguration wird ein optionales
`wayfinder.yaml` (Go-seitig geladen, 12-Factor-kompatibel) eingeführt. Damit können
Betreiber Karten-Zentrum (Lat/Lon), Zoom und OpenAIP-Radius dauerhaft festlegen ohne
Env-Var-Akrobatik. Env-Vars überschreiben die Datei (12-Factor gewinnt).

## Begründung

- **Vue 3 vs. React:** Vue 3 Composition API ist für diesen Umfang (1 Navigation Drawer,
  1 App Bar, 2–3 Panels) weniger boilerplate-intensiv. Vuetify 3 liefert MD3-Komponenten
  vollständiger als MUI v6 (MUI MD3-Support ist noch experimentell).
- **Vue 3 vs. Svelte:** Vuetify ist für Svelte nicht verfügbar; ein MD3-Komponentensatz
  müsste selbst gebaut werden — erhöhter Aufwand ohne Traceability-Gewinn.
- **Vanilla JS + CSS-Tokens:** Würde MD3-Komponenten (Navigation Drawer mit MD3-Easing,
  MD3-Switches, Elevation-Shadows) manuell implementieren — Aufwand ohne Lieferantenunabhängigkeit.
  Bei diesem Chrome-Umfang kein Vorteil gegenüber Vuetify.
- **Material Web Components (@material/web):** Von Google in den Wartungsmodus versetzt —
  inakzeptabel für ein Produktionssystem.
- **Kein Umschreiben der Karten-Engine:** Die ASD-Logik ist sicherheitsrelevant und
  durch manuelle Tests verifiziert. Eine Neuimplementierung in Vue-Komponenten würde
  Regressionsrisiko ohne funktionalen Mehrwert erzeugen.

### Verworfene Alternativen

- React 19 + MUI v6: mehr Boilerplate, MUI MD3 experimentell, kein klarer Vorteil.
- Vanilla JS + MD3-CSS-Tokens: vollständige MD3-Komponentenimplementierung von Hand —
  überproporzionaler Aufwand.
- Material Web Components: Wartungsmodus (Google, 2025).
- Kein Framework (Status quo): schließt die Testbarkeits- und Design-Lücken nicht.

## Konsequenzen

- **Neues Verzeichnis** `frontend/` mit Vite-Projekt (Vue 3, Vuetify 3, Vitest, Pinia).
- **Neues Verzeichnis** `frontend/src/map/` — herausgelöste ASD-Karten-Engine-Module.
- **Automatisierte JS-Tests** (Vitest) schließen die Test-Lücke in FR-ASD-001/002/004/005.
- **Go-Backend** embedded `frontend/dist/` statt `internal/webui/static/`.
- **Dockerfile** erhält eine Node 22 Build-Stufe.
- **`wayfinder.yaml`** als optionale Konfigurationsdatei (Karten-Defaults, OpenAIP-Radius).
- Neue Anforderungen im Register: FR-UI-001 bis FR-UI-005, NFR-UI-001/002.
- **`internal/webui/static/`** wird nach vollständiger Migration entfernt (nicht vorher —
  der alte Pfad bleibt bis AP2 parallel, um Regressions-Vergleiche zu ermöglichen).

## Qualitäts-Gate (Abschluss der Migration, AP6)

- `npm run build` ✅ (kein Fehler, alle Vitest-Tests grün)
- `go test ./...` ✅ (Backend unverändert grün)
- `go vet ./...` ✅
- Alle heutigen manuell-verifizierten FR-ASD-*-Einträge haben Vitest-Entsprechungen
- ASD-Fachlogik (Deconfliction, Fade-Out, FL-Filter, Feed-Banner, Drag&Drop) funktional
  identisch zur Pre-Migration — kein Verhaltens-Regresssion

## Ehrliche Grenze

Vue 3 und Vuetify 3 sind Community-geführte Open-Source-Projekte; sie sind nicht Teil
eines zertifizierten Software-Stacks. Die Trennlinie zwischen „Chrome" (Vue/Vuetify) und
„sicherheitsrelevanter ASD-Logik" (Karten-Engine, Vanilla-JS-Module) wird bewusst
aufrechterhalten — die Zertifizierungs-Fähigkeit des ASD-Kerns hängt nicht an der
Komponentenbibliothek.
