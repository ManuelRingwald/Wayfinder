# ASD-006 — Vue 3 + Vuetify 3 (Material Design 3)

Paket **ASD-006** aus `docs/ROADMAP.md`. Frontend-Modernisierung: Migration von Vanilla-JS
auf **Vue 3 + Vuetify 3 (Material Design 3)**. Kein Backend- oder ICD-Change.

Grundlage: **ADR 0002** (`docs/decisions/0002-frontend-framework-vue3-vuetify3.md`).

---

## Fachlich

Das bisherige Wayfinder-Frontend war eine Vanilla-JS-Einzeldatei ohne Design-System. Für
einen produktiven ASD-Einsatz sind drei Defizite zu schließen:

1. **Testbarkeit:** ASD-Fachlogik (Deconfliction, FL-Filter, buildLabel) war nicht
   automatisch testbar — nur manuell verifiziert.
2. **Design:** Kein konsistentes Design-System, keine Typographie, keine Barrierefreiheit.
3. **Bedienbarkeit:** Layer-Steuerung und FL-Filter als freischwebende Panels verringerten
   die nutzbare Kartenfläche des Lotsen.

Das neue Design räumt den Scope durch eine **linke Sidebar** (Navigation Drawer) frei.
Der Radar-Scope selbst (dunkle Karte, Tracks, Data Blocks, Leader Lines) bleibt
technisch und optisch **unverändert** — MD3 gilt nur für den Chrome (Bedienoberfläche).

---

## Technisch — Überblick

### Schichtung

| Schicht | Technologie | Verantwortung |
|---------|-------------|---------------|
| **Karten-Engine** | Vanilla-JS-ES-Module (`src/map/*.js`) | MapLibre, Tracks, Deconfliction, Fade-Out, Drag&Drop — unveränderte ASD-Logik |
| **Reaktiver State** | Pinia-Store (`src/stores/asd.js`) | Layer-Visibility, FL-Filter, Feed-Status, Track-Selektion |
| **Chrome** | Vue 3 + Vuetify 3 | App-Bar, Navigation Drawer, Feed-Chip, Track-Detail-Panel |

### Verzeichnisstruktur

```
frontend/
├── index.html                    # Vite-Einstieg (MapLibre CSS, #app mount)
├── vite.config.js                # Vite + Vuetify-Plugin, outDir → ../internal/webui/dist
├── package.json                  # Vue 3, Vuetify 3, Pinia, Vitest, maplibre-gl
├── src/
│   ├── main.js                   # createApp + Pinia + Vuetify
│   ├── App.vue                   # Root: v-app, v-app-bar, LayerSidebar, MapCanvas
│   ├── style.css                 # Reset + Vuetify-Overrides
│   ├── test-setup.js             # Vitest-Globals
│   ├── plugins/
│   │   └── vuetify.js            # MD3 asdDarkTheme-Definition
│   ├── stores/
│   │   └── asd.js                # Pinia-Store: feedStatus, layerVisibility, flFilter, selectedTrack
│   ├── components/
│   │   ├── MapCanvas.vue         # MapLibre-Mount + Engine-Lifecycle
│   │   ├── LayerSidebar.vue      # Navigation Drawer: Layer-Switches + FL-Filter
│   │   ├── FeedStatusChip.vue    # MD3-Chip: FEED OK / STALE / ?
│   │   ├── TrackDetailPanel.vue  # Container: Bottom-Sheet (Mobile) / Card (Desktop)
│   │   └── TrackDetailCard.vue   # Inhalt: Callsign, FL, Speed, Mode3A, Status
│   └── map/                      # Framework-agnostische ASD-Engine (keine Vue-Abhängigkeit)
│       ├── constants.js           # PALETTES, LABEL_SLOTS, Layer/Source-IDs etc.
│       ├── label.js               # buildLabel(track, vTrend)
│       ├── deconflict.js          # bboxCollides + deconflictLabels
│       ├── layers.js              # addXxxLayer/Source-Funktionen
│       ├── render.js              # renderSources + tickFade
│       ├── tracks.js              # updateTracksLayer, isFlFiltered, flOpacity
│       ├── drag.js                # setupLabelDrag
│       ├── aeronautical.js        # loadAeronautical + 5-min-Refresh
│       ├── engine.js              # initMap — Orchestrierung, WS-Verbindung
│       └── __tests__/
│           ├── label.test.js      # buildLabel-Vitest-Tests
│           ├── deconflict.test.js # bboxCollides + deconflictLabels-Vitest-Tests
│           └── tracks.test.js     # isFlFiltered + flOpacity-Vitest-Tests
```

### Build-Integration (Go-Embed)

`vite build` schreibt nach `internal/webui/dist/`. Das Go-Backend embedded dieses
Verzeichnis via `//go:embed dist`. Der Dockerfile bekommt eine Node 22-Stufe vor der
Go-Stufe.

### Konfigurationsdatei `wayfinder.yaml`

Optionale Konfigurationsdatei für Karten-Defaults (Zentrum, Zoom, OpenAIP-Radius).
Env-Vars überschreiben die Datei (12-Factor gewinnt). `wayfinder.yaml.example`
im Repo, `wayfinder.yaml` in `.gitignore`.

---

## AP1 — Scaffold + Build-Setup

Vite-Projekt, Vuetify 3-Plugin mit MD3-Dark-Theme (`asdDarkTheme`), Vitest-Konfiguration,
`main.js`, Vuetify-Defaults (Switches/TextFields/Buttons). Build-Output nach
`internal/webui/dist/`.

## AP2 — Karten-Engine-Extraktion + Vitest-Tests

Die Logik aus `internal/webui/static/app.js` wird **inhaltlich unverändert** in
framework-agnostische ES-Module unter `frontend/src/map/` herausgelöst. Dies schließt
die Testbarkeits-Lücke: `buildLabel`, `bboxCollides`, `deconflictLabels`, `isFlFiltered`,
`flOpacity` haben erstmals automatisierte Vitest-Tests.

## AP3 — Pinia-Store + Sidebar

Pinia-Store (`src/stores/asd.js`) als reaktive Wahrheitsquelle für alle UI-Zustände.
`LayerSidebar.vue`: MD3 Navigation Drawer links mit Layer-Switches und FL-Filter —
räumt den Karten-Scope frei.

## AP4 — App-Shell

`App.vue` mit `v-app-bar` (Titel, Hamburger-Menü auf Mobile), `FeedStatusChip` (MD3-Chip
ersetzt altes `#feed-status`-Banner), `MapCanvas.vue` als Karten-Mount.

## AP5 — Track-Detail-Panel

`TrackDetailPanel.vue` + `TrackDetailCard.vue`: MD3 Bottom-Sheet (Mobile) / Fixed Card
(Desktop) bei Track-Klick. Zeigt Callsign, FL, Bodengeschwindigkeit, Mode 3/A,
Track-Nummer, Status. **Neu** — bisher nicht im ASD vorhanden.

## AP6 — A11y & Responsive

Touch-Targets ≥ 44 dp (Vuetify-Standard), WCAG-AA-Kontrast im Dark-Theme, Navigation
Drawer als Overlay auf Mobile (Hamburger-Toggle), keine focus-ring-Unterdrückung.

---

## Schnittstellen-Wirkung

Keine. Kein CAT062-/ICD-/Backend-Change. Der einzige Go-seitige Touch ist der
`//go:embed`-Pfad von `static/` auf `dist/`.

## Qualitäts-Gates

- `npm run build` ✅ (Vite-Build ohne Fehler)
- `npx vitest run` ✅ (alle Vitest-Tests grün: buildLabel, deconflictLabels, isFlFiltered)
- `go test ./...` ✅ (Backend-Tests unverändert)
- `go vet ./...` ✅
- FR-UI-001 bis FR-UI-005, NFR-UI-001/002 im Anforderungs-Register eingetragen ✅
- Manuelle Rauchtest: ASD-Fachlogik (Tracks, Deconfliction, FL-Filter, Drag&Drop,
  Feed-Banner) funktional identisch zur Pre-Migration ✅ (zu verifizieren)
