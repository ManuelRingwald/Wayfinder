# ASD-019 — Gruppierte Rail, Orange/Blau-Farbcode, Zoom auf die Karte

> **Register:** FR-UI-040 · **Entscheidung:** ADR 0030 · **Auslöser:**
> Betreiber-Design-Mockup „Vorschlag A" (2026-07-19): Rail-Optionen in MEASURE
> und MAP gruppieren, Orange/Blau-Farbcodierung mit aktiv leuchtenden Symbolen,
> Zoom in/out an die untere rechte Ecke des ASD-Screens.

## In normaler Sprache — was sich sichtbar ändert

Die schmale Werkzeug-Leiste am linken Rand des Lagebilds war eine flache Reihe
gleich aussehender Symbole. Jetzt ist sie **sichtbar in zwei Gruppen geteilt**:

- **MEASURE** — die Mess-Werkzeuge (RBL, DIST, QDM).
- **MAP** — die Karten-Panels (Layer, Filter).

Jede Gruppe steht unter einem kleinen Titel, und die beiden Familien sind
**farblich getrennt**: Aktiviert ein Lotse ein **Mess-Werkzeug**, leuchtet es
**bernstein/orange** — das ist die Warnfarbe und passt, weil ein scharfes
Mess-Werkzeug die Karten-Klicks „an sich zieht" (ein besonderer Modus). Öffnet
er ein **Karten-Panel**, leuchtet es **cyan/blau** — ein normaler, offener
Zustand. Das aktive Symbol bekommt einen weichen Schein („leuchtet"), damit man
den aktiven Zustand sofort sieht. Das **Konto** sitzt klar abgesetzt ganz unten.

Die **Zoom-Knöpfe (+/−)** sind aus der Leiste **auf die Karte gewandert** — an
die **untere rechte Ecke**, dort, wo sie wirken. Die Leiste wird dadurch kürzer
und enthält nur noch echte Lotsen-Werkzeuge.

## Fachlich — welches Problem das löst

Ein Mess-Werkzeug ist **modal**: solange es scharf ist, gehen Karten-Klicks an
das Werkzeug, nicht an die Track-Auswahl. Ein Panel ist **nicht-modal**. Beides
sah vorher gleich aus (derselbe cyan Indikator). Das ist im Betrieb riskant — der
Lotse konnte nicht auf einen Blick sehen, ob gerade ein Mess-Modus „scharf" ist.
Die **zwei Aktiv-Farben** kodieren genau diesen Unterschied. Zoom gehört
fachlich zur **Karten-Navigation**, nicht zum Werkzeug-Satz; auf der Karte, unten
rechts, ist es am erwarteten Ort und auf allen Geräten konsistent.

## Technisch

- **`NavigationRail.vue`** — zwei Sektionen mit `.nav-rail__section`-Mikro-Label
  (MEASURE/MAP) und Trennern. Die Mess-Werkzeuge tragen `--tool`, die Panels
  `--panel`. Aktiv-Zustände: `--tool` überschreibt die Aktiv-Farbe auf
  `--wf-warning` (Bernstein) mit `--wf-state-armed`-Pill + `--wf-glow-armed`;
  `--panel`/Konto behalten Cyan (`--wf-state-selected` + `--wf-glow-selected`).
  Eine dezente, dauerhafte **Akzentleiste** (`::before`, opacity 0.35) tönt jede
  Familie schon im Ruhezustand. Das Konto rutscht per **Push-Divider**
  (`--push`, `margin-top:auto`) an den Fuß. Zoom-Buttons + `zoom-in/out`-Emits
  **entfernt**.
- **`ZoomControls.vue`** (neu) — positions-neutrale +/−-Gruppe (Spiegel zu
  `ViewportControls`), emittiert `zoom-in`/`zoom-out`.
- **`MapControls.vue`** — die **bottom-right-Zone**, jetzt auf Desktop **und**
  Mobil gerendert. Immer Zoom (`ZoomControls`); Recenter/Vollbild
  (`ViewportControls`) nur mobil (Desktop hat sie in der top-right-Zone,
  ADR 0029). Mobiler Lift über die Tab-Bar in einer `<960px`-Media-Query; Desktop
  klärt die Attribution-ⓘ in der Ecke.
- **`MapCanvas.vue`** — rendert `MapControls` unbedingt (kein `!mdAndUp`-Gate mehr)
  und verdrahtet Zoom direkt an die Engine; die nun ungenutzten
  `zoomIn/zoomOut`-Expose-Methoden entfernt (`useDisplay`/`mdAndUp` ebenfalls, da
  nicht mehr gebraucht).
- **`AsdView.vue`** — Zoom-Delegation an die Rail entfernt.
- **`TrackDetailPanel.vue`** — die Desktop-Karte subtrahiert die
  Zoom-Stack-Höhe (`--wf-map-controls-reserve`) von ihrer `max-height`, damit ihr
  Scroll-Bereich nie unter die Zoom-Knöpfe am gemeinsamen rechten Rand läuft.
- **`engine.js`** — Kommentar aktualisiert (Zoom lebt jetzt in den bottom-right
  Map-Controls, nicht in der Rail).
- **Tokens** — `colors.css`: `--wf-state-armed`, `--wf-glow-armed`,
  `--wf-glow-selected`; `spacing.css`: `--wf-map-controls-reserve`.

## Farb-/Token-Referenz

| Zustand | Farbe | Pill-Fill | Glow |
|---------|-------|-----------|------|
| MEASURE-Werkzeug scharf | `--wf-warning` (#ffb02e) | `--wf-state-armed` | `--wf-glow-armed` |
| MAP-Panel / Konto offen | `--v-theme-primary` (#23d3e6) | `--wf-state-selected` | `--wf-glow-selected` |

## Tests

- `railTools.test.js` (neu geschnitten): Rail listet RBL/DIST/QDM + `selectTool`;
  MEASURE/MAP-Mikro-Labels + `--tool`/`--panel`-Gruppen; Amber/Cyan-Aktiv-Farbcode
  + Glow-Tokens; Zoom **nicht** mehr in der Rail; `ZoomControls` positions-neutral
  + Emits; `MapControls` hostet `ZoomControls` und rendert nicht mehr nur mobil;
  `MapCanvas` verdrahtet Zoom an die Engine.
- `scopeChromeLayout.test.js` (angepasst): `MapControls` rendert auf Desktop +
  Mobil, `ViewportControls` darin `!mdAndUp`-gegatet; `MapCanvas` exponiert
  weiter `recenter`.
- `responsive.test.js` (angepasst): mobiler Zoom-Lift jetzt in der
  `<960px`-Media-Query.
- `scopeFixups.test.js` (unverändert grün): Brand-Glyph + Gruppen-Trenner.
- Ergebnis: `vitest run` 648 grün, `vite build` grün, `dist` neu gebaut.

## Ehrliche Grenzen

Keine visuelle CI-Zusicherung (kein WebGL-/Mount-Harness) — Struktur/Verdrahtung
sind per Source-Guards gezurrt, die **optische Abnahme** (Farbcode, Glow,
Zoom-Position, keine Überlappung mit der Track-Detail-Karte, sauberer Abstand zur
Attribution-ⓘ) macht der Betreiber nach `git pull` + Frontend-Rebuild.
