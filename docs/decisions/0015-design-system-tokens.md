# ADR 0015 — Design System v1: Tokens & Webfonts

- **Status:** akzeptiert
- **Datum:** 2026-07-02
- **Schnittstellen-relevant:** nein (betrifft ausschließlich die Browser-seitige
  Präsentationsschicht; kein CAT062-/ICD-Change, kein Backend-API-Change)

## Kontext

Für Wayfinder wurde ein neues Design mit **Claude Design** erstellt und als
Projekt-Export (Design-System + ASD-Ziel-Screens als React/JSX + Screenshots)
bereitgestellt. Das Design-System wurde dabei **aus dem bestehenden Wayfinder-Code
rückwärts abgeleitet** — die Farbwerte decken sich mit `asdDarkTheme`
(`src/plugins/vuetify.js`) und den Scope-Paletten (`src/map/constants.js`).

Der Reskin auf dieses Zielbild wird in mehreren freigegebenen Häppchen umgesetzt
(siehe Umsetzungsplan in `docs/STATUS.md`). Bevor Komponenten umgestellt werden,
braucht es ein **Fundament**:

1. **Keine formalen Design-Tokens.** Farben lagen nur als Vuetify-Theme (Chrome)
   und als JS-Konstanten (Scope) vor; Typografie/Spacing/Radius/Elevation waren
   nirgends als wiederverwendbares Vokabular ausformuliert.
2. **Schrift nicht paketiert.** Vuetify verweist implizit auf Roboto, aber die
   Schrift wurde nirgends mitgeliefert — die Darstellung hing von einer
   zufällig vorhandenen System-Schrift ab. Für eine **luftgespaltene
   ATC-Konsole** ist ein Laufzeit-CDN (Google Fonts) ausgeschlossen.

## Entscheidung

### 1. Design-Tokens als CSS-Custom-Properties (`--wf-*`)

Die Tokens liegen in `frontend/src/design/tokens/` (`colors`, `typography`,
`spacing`, `radius`, `elevation`) und werden über eine Sammel-Datei
(`tokens/index.css`) einmalig importiert. Alle Werte sind CSS-Custom-Properties
im `:root` — lesbar von jeder Komponente und von rohem DOM (Karten-Overlays).

### 2. Schrift self-hosted via `@fontsource` (offline, kein CDN)

Roboto (UI) und Roboto Mono (tabellarische Zahlen-Readouts: Squawk, Flugfläche,
Koordinaten, Track-Nummern) werden über `@fontsource/roboto` /
`@fontsource/roboto-mono` **ins Bundle eingebettet** und in `src/main.js`
importiert — **kein** Laufzeit-Aufruf nach außen. Es werden nur die Subsets
**latin + latin-ext** geladen (die UI ist deutsch; Umlaute liegen in latin-ext),
damit Kyrillisch/Griechisch/Vietnamesisch/Math nicht mitgeschleppt werden.

### 3. Autoritäts-Nähte (bewusste, dokumentierte Doppelungen)

- **Chrome-Farben:** `src/plugins/vuetify.js` (`asdDarkTheme`) braucht literale
  Hex-Werte zum Theme-Bauzeitpunkt und kann keine CSS-Vars lesen. `colors.css`
  spiegelt dieselben Werte für Nicht-Vuetify-DOM. **Beide werden von Hand im
  Gleichschritt gehalten**; ändert sich eine Chrome-Farbe, ändern sich beide.
- **Scope-/Domänen-Farben:** maßgeblich bleibt `src/map/constants.js` (treibt
  MapLibre-GL-Paint-Ausdrücke, die keine CSS-Vars lesen). Die Domänen-Sektion in
  `colors.css` ist nur ein **DOM-seitiger Spiegel** für Legenden-Swatches.

### 4. Nur Fundament — kein Umstyling in diesem Schritt

`base.css` bindet lediglich die Grund-Schriftfamilie an `body` und stellt zwei
Signatur-Klassen bereit (`.wf-mono`, `.wf-overline`). Das Aussehen der
Komponenten ändert sich **nicht**; sie werden in den Folge-Häppchen inkrementell
auf die Tokens migriert. Die sicherheitsrelevante Karten-Engine (`src/map/*`)
bleibt unberührt (Fortführung von ADR 0002).

### 5. Daten-Ehrlichkeit (bewusste Auslassungen)

Das Zielbild zeigt auch Elemente, die der heutige CAT062-Wire-Vertrag **nicht**
deckt. Diese werden **nicht** angezeigt (Vorgabe des Projektverantwortlichen):
Track-Typ-Farben mil/hostile/neutral (Firefly sendet nur Civil),
Zuständigkeits-Dimming und STCA (kein Sektor-/Konflikt-Signal im Vertrag). Der
Token-Satz führt entsprechend nur die belegbare Civil-Track-Farbe; die übrigen
Typ-Farben bleiben in `constants.js` reserviert, aber ungenutzt.

## Begründung

- **Offline/luftgespalten:** self-hosted Fonts erfüllen die Sicherheits-/
  Betriebsvorgabe; kein externer Request, keine Telemetrie an Dritte.
- **Ein Vokabular:** Tokens machen die späteren Reskin-Häppchen konsistent und
  überprüfbar (ein Ort für Farbe/Typo/Spacing/Radius/Elevation).
- **Risiko-arm:** reine Additive; keine Verhaltensänderung, Karten-Engine
  unangetastet, alle Tests bleiben grün.

## Konsequenzen

- **Neue Abhängigkeiten:** `@fontsource/roboto`, `@fontsource/roboto-mono`.
- **Neues Verzeichnis:** `frontend/src/design/` (Tokens + `base.css`).
- **`src/main.js`** importiert Fonts + Tokens.
- **Neue Anforderung** im Register: **FR-UI-019**.
- **Eingebettetes `internal/webui/dist`** neu gebaut (Fonts als self-hosted
  Assets enthalten).
- **`docs/design/`** um ein README (Provenienz des Design-Exports + maßgebliche
  Token-Quelle) ergänzt; `color-tokens.md` verweist auf die realisierten Tokens.

## Ehrliche Grenze

Die Chrome-Farben leben an zwei Orten (`vuetify.js` **und** `colors.css`) und
werden manuell synchron gehalten — eine bewusste Vereinfachung, weil Vuetify
literale Hex-Werte braucht. Ein späterer Schritt könnte die Vuetify-Theme-Werte
generativ aus den Tokens ableiten; das ist hier nicht Teil des Fundaments.
