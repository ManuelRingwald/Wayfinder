# Wayfinder — Farbschema & Design-Tokens

> **Abgeleitet aus:** Militärisches ASD-Mockup (Command-Center-Ästhetik, 2026-06-17).  
> **Zweck:** Verbindliche, einfach pflegbare Farbdefinition für alle UI-Schichten.  
> **Aufbau:** Zwei Schichten — Chrome-Tokens (Vue/Vuetify MD3) und ASD-Domain-Farben (MapLibre-Layer).
>
> 🎨 **Realisiert als Design-Tokens (Design System v1, ADR 0015):** Die hier
> beschriebenen Farben liegen jetzt als CSS-Custom-Properties (`--wf-*`) in
> `frontend/src/design/tokens/colors.css` (zusammen mit Typo/Spacing/Radius/
> Elevation). Maßgeblich für das Vuetify-Theme bleibt `frontend/src/plugins/vuetify.js`,
> für die MapLibre-Paint-Ausdrücke `frontend/src/map/constants.js`.
> Übersicht: `docs/design/README.md`.

---

## 1. Grundprinzip

Das ASD zeigt eine Karte mit Live-Tracks im Vordergrund. Das Farbschema ist deshalb
konsequent auf **maximalen Kontrast zwischen Track-Symbolen und Kartenhintergrund**
ausgerichtet:

- **Hintergrund Near-Black** (`#070b12`) — maßgeblich ist der Claude-Design-Export
  (ADR 0015 Nachtrag-2, 2026-07-02); die leuchtenden Track-Farben (Cyan, Amber)
  springen maximal ins Auge. (Der frühere Navy-Versuch aus Nachtrag-1 beruhte auf
  einer Fehl-Lesung des Screenshots und ist aufgehoben.)
- **Primärfarbe Cyan** (`#23d3e6`) — aerospace/command-center-Standard, lässt sich
  auf dark surfaces gut ablesen und korrespondiert mit dem Friendly-Civil-Track-Cyan.
- **Semantische Farben klar unterscheidbar**: Error-Rot (`#ff4338`) ≠ Warning-Amber
  (`#ffb02e`) ≠ Success-Grün (`#3ecf6b`) — ATC-konventionell.

---

## 2. MD3-Chrome-Tokens (Vue/Vuetify-Ebene)

Diese Farben steuern die gesamte UI-Shell: App Bar, Navigation Rail, Side Panels,
Karten-Controls, Chips, Dialoge. Sie sind in `frontend/src/plugins/vuetify.js`
als `asdDarkTheme.colors` hinterlegt.

| Token | Hex | Verwendung |
|-------|-----|-----------|
| `background` | `#070b12` | App-Root-Hintergrund (hinter der Karte) |
| `surface` | `#0e1622` | Panels, Karten, Navigation Rail, Bottom Sheets |
| `surface-variant` | `#16202e` | Leicht gehobene Flächen (z. B. ausgewählte Nav-Items) |
| `surface-bright` | `#1c2c3e` | Hover-States, Tooltips, aktive Chips |
| `primary` | `#23d3e6` | Haupt-Akzent (aktive Icons, Schalter, Outline-Inputs) |
| `primary-darken-1` | `#0e8a9c` | Pressed-State, Focused-Outline |
| `on-primary` | `#04141a` | Text/Icons auf primärem Hintergrund |
| `secondary` | `#5b7a9d` | Sekundäre Schaltflächen, inaktive Nav-Icons |
| `on-surface` | `#dce6f0` | Primärtext auf dunklen Flächen |
| `on-surface-variant` | `#8a9bb0` | Sekundärtext, Beschriftungen, Hints |
| `error` | `#ff4338` | Fehler, Alarm, Hostile-Track-Statusanzeige |
| `warning` | `#ffb02e` | Warnungen, degradierter Feed |
| `success` | `#3ecf6b` | Bestätigungen, Feed OK |
| `info` | `#3d9be0` | Informationen, neutrale Hinweise |

### Noch nicht aktiv verwendete Token (reserviert für spätere APs)

| Token | Hex | Vorgesehen für |
|-------|-----|---------------|
| `surface-bright` | `#1c2c3e` | ASD-010 Filter-Chips aktiv |
| `primary-darken-1` | `#0e8a9c` | ASD-009 Karten-Controls Pressed |

---

## 3. ASD-Domain-Farben (MapLibre-Ebene)

Diese Farben steuern die Darstellung von Tracks und aeronautischen Overlays auf
der Karte. Sie sind in `frontend/src/map/constants.js` als `PALETTES` und
`TRACK_COLORS` hinterlegt.

### 3.1 Track-Symbolfarben (nach ICAO-Zieltyp)

| Kategorie | Hex | Bedeutung |
|-----------|-----|-----------|
| `friendlyCivil` | `#41c4e8` | Ziviler bestätigter Track (zivile Luftfahrt) |
| `friendlyMilitary` | `#ffa726` | Militärischer bestätigter Track |
| `hostile` | `#ff4338` | Feindlicher / ordnance Track — sofortige Aufmerksamkeit |
| `unknown` | `#ffd23e` | Unbekannter Track (noch nicht korreliert) |
| `neutral` | `#43c66b` | Neutraler Track |

> **Hinweis:** In der aktuellen Demo-Phase sendet Firefly ausschließlich zivile Tracks
> (`friendlyCivil`). Die übrigen Farben sind für spätere Differenzierung (IFF, Mode 3/A)
> reserviert und bereits in constants.js hinterlegt.

### 3.2 Track-Label & Vektor (Palette Dark)

| Element | Hex | Bedeutung |
|---------|-----|-----------|
| `label` | `#dce6f0` | Datenblockttext — identisch mit `on-surface` |
| `labelHalo` | `#000000` | Schatten/Halo um Labeltext |
| `vector` | `#9ec8de` | Geschwindigkeitsvektor (SVL-Linie) |
| `trail` | `#3a5a72` | Vergangenheitsspur (gedämpft, kein ablenkender Hintergrund) |
| `symbolStroke` | `#000000` | Punkt-Symbol-Kontur |

### 3.3 Luftraum-Overlays

| Element | Hex / Alpha | Bedeutung |
|---------|-------------|-----------|
| `airspaceFill` | `#3a6fb0` @ 12% | Füllfläche (muss Karte durchscheinen lassen) |
| `airspaceLine` | `#5b8fd6` | Luftraumgrenze |
| `airspaceText` | `#9fc0e8` | Luftraum-Label |
| `airways` | `#2a8fa8` | Luftstraße (Airways) |
| `aeroHalo` | `#000000` | Halo um aeronautische Symbole und Labels |

---

## 4. Farbsystem-Übersicht (visuell)

```
Dunkel → Hell (Hintergrund-Hierarchie, Near-Black):
  #070b12  background        ████████████
  #0e1622  surface           ████████████
  #16202e  surface-variant   ████████████
  #1c2c3e  surface-bright    ████████████

Primär / Akzent (Cyan):
  #23d3e6  primary           ████████████
  #0e8a9c  primary-darken-1  ████████████

Semantisch:
  #ff4338  error/hostile     ████████████
  #ffb02e  warning           ████████████
  #3ecf6b  success           ████████████
  #3d9be0  info              ████████████

Track-Domain:
  #41c4e8  friendly civil    ████████████
  #ffa726  friendly mil      ████████████
  #ff4338  hostile           ████████████  (= error)
  #ffd23e  unknown           ████████████
  #43c66b  neutral           ████████████
```

---

## 5. Änderungshistorie

| Version | Datum | Inhalt |
|---------|-------|--------|
| 1.0.0 | 2026-06-17 | Initiale Ableitung aus ASD-Mockup (Command-Center-Ästhetik, Cyan-Primary), implementiert in ASD-007 |
| 1.1.0 | 2026-07-02 | Surface-Hierarchie von Near-Black auf **tiefes Navy** umgestellt (`background #0a1626`, `surface #12233b`, `surface-variant #1a2f4a`, `surface-bright #223a5a`), passend zum Design-Mockup; Map-Hintergrund `#0b1a2e` + CARTO-Raster auf `raster-opacity 0.4` gedimmt (`cmd/wayfinder/main.go`). Cyan-Primary, Text- und Domain-/Track-Farben unverändert. |
| 1.2.0 | 2026-07-02 | **Zurück auf Near-Black** (ADR 0015 Nachtrag-2): Der Claude-Design-Export ist verbindliches Template; seine maßgeblichen Token-Werte sind Near-Black (`background #070b12`, `surface #0e1622`, `surface-variant #16202e`, `surface-bright #1c2c3e`). Nachtrag-1 (Navy) war eine Screenshot-Fehl-Lesung und ist aufgehoben. Map-Hintergrund `#070b12`; CARTO-Raster bleibt bei `raster-opacity 0.4` (echte Geografie ist bewusst gewollt). Cyan-Primary/Text/Domain-Farben unverändert. |
