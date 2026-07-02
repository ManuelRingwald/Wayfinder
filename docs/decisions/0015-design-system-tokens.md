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

## Nachtrag (2026-07-02): Surface-Hierarchie auf tiefes Navy

**Kontext:** Beim E2E-Abgleich gegen den Design-Mockup fiel auf, dass die
umgesetzte Surface-Hierarchie **Near-Black** (`background #070b12`) ist, während
der Mockup ein **tiefes Navy** (Blau statt Schwarz) mit erkennbar navyfarbenen
Panels zeigt. Die ursprünglichen Token-Werte waren rückwärts aus dem *alten* Code
abgeleitet und lagen damit unter der Mockup-Vorlage. Auf Freigabe des
Projektverantwortlichen wird die Surface-Hierarchie auf Navy umgestellt.

**Entscheidung:** Nur die **Surface-Hierarchie** wechselt auf Navy; Cyan-Primary,
Text- und Domänen-/Track-Farben (`constants.js`) bleiben unverändert.

| Token | Alt (Near-Black) | Neu (Navy) |
|-------|------------------|------------|
| `background` | `#070b12` | `#0a1626` |
| `surface` | `#0e1622` | `#12233b` |
| `surface-variant` | `#16202e` | `#1a2f4a` |
| `surface-bright` | `#1c2c3e` | `#223a5a` |

Zusätzlich Map-Style (`cmd/wayfinder/main.go`, `darkMapStyle`):
`background-color` `#0b0f14` → `#0b1a2e`, und die CARTO-`dark_nolabels`-Rasterschicht
auf `raster-opacity: 0.4` gedimmt, damit das Navy durchscheint und Küsten/Grenzen
als feiner Kontext erhalten bleiben.

**Umfang der Änderung (Lockstep):** `frontend/src/design/tokens/colors.css`,
`frontend/src/plugins/vuetify.js` (`asdDarkTheme`), `cmd/wayfinder/main.go`,
`docs/design/color-tokens.md` (v1.1.0). Weiterhin **nicht schnittstellen-relevant**
(reine Präsentation). Die volle Mockup-Karte (echtes Vektor-Grid, Sektorgrenzen,
Airspace/Navaids) bleibt ein separates, teils datenabhängiges Thema.

## Nachtrag-2 (2026-07-02): Zurück auf Near-Black — Design-Export ist maßgeblich

**Kontext:** Der Nachtrag-1 stellte die Surface-Hierarchie auf Navy um, weil ein
Blick auf den Mockup-Screenshot navyfarben *wirkte*. Der Projektverantwortliche
hat den **Claude-Design-Export** (`ASD.zip`: Design-System mit Tokens + ASD-Ziel-
Screens als JSX + Screenshots) inzwischen ausdrücklich zum **verbindlichen
Template** von Wayfinder erklärt — „genau so, wie wir Material Design für die
Komponenten verwenden". Die **maßgeblichen Token-Werte** dieses Exports
(`_ds/.../tokens/colors.css`) sind **Near-Black**, nicht Navy. Damit war die
Navy-Annahme aus Nachtrag-1 eine Fehl-Lesung des Screenshots; der Token-Satz des
Exports ist die Grundwahrheit.

**Entscheidung:** Die Surface-Hierarchie geht **zurück auf Near-Black**; sie
**hebt Nachtrag-1 auf**. Cyan-Primary, Text-, Semantik- und Domänen-/Track-Farben
bleiben unverändert (sie stimmten hex-genau mit dem Export überein).

| Token | Nachtrag-1 (Navy) | Nachtrag-2 (Near-Black, = Export) |
|-------|-------------------|-----------------------------------|
| `background` | `#0a1626` | `#070b12` |
| `surface` | `#12233b` | `#0e1622` |
| `surface-variant` | `#1a2f4a` | `#16202e` |
| `surface-bright` | `#223a5a` | `#1c2c3e` |

Map-Style (`cmd/wayfinder/main.go`, `darkMapStyle`): `background-color`
`#0b1a2e` → `#070b12`. **Die CARTO-`dark_nolabels`-Rasterschicht bleibt** bei
`raster-opacity: 0.4` — echte Küstenlinien/Geografie unter den Tracks ist eine
bewusste Produkt-Entscheidung; der reine synthetische Scope des Design-Exports
ist ein Standalone-Demo-Artefakt (der Export rendert gar keine echte Karte).
Zusätzlich wird ein zarter **Cyan-Mittglow** (`radial-gradient`,
`rgba(35,211,230,0.05)`) über dem Scope ergänzt (Export-`nacht`-Schema).

**Nebeneffekt (Konsistenz):** Die schwebenden Overlay-Panels (Header, Legende,
Mess-Readout) nutzten bereits hartkodiert `rgba(14,22,34,0.85)` = Export-Surface
`#0e1622` @ 85%. Mit Nachtrag-2 sind Vuetify-Surface und Overlay-Chrome wieder
**dieselbe** Farbe (unter Navy waren sie auseinandergelaufen).

**Umfang (Lockstep, unverändert):** `colors.css`, `vuetify.js`,
`cmd/wayfinder/main.go`, `docs/design/color-tokens.md`. Weiterhin **nicht
schnittstellen-relevant**.
