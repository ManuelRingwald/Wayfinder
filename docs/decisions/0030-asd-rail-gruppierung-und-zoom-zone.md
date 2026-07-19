# ADR 0030 — ASD-Rail: gruppierte Werkzeuge, zwei Aktiv-Farben, Zoom auf die Karte

- **Status:** **AKZEPTIERT** ✅ (2026-07-19). Betreiber-Wunsch (Design-Mockup
  „Vorschlag A"): die Rail-Werkzeuge in **MEASURE** und **MAP** gruppieren, mit
  **Orange/Blau-Farbcodierung** und **aktiv leuchtenden Symbolen**; die
  **Zoom-Knöpfe** an die **untere rechte Ecke** des Scopes.
- **Datum:** 2026-07-19
- **Schnittstellen-relevant:** **nein** (reines Browser-Chrome; kein
  CAT062-/Firefly-Bezug, keine Backend-Wirkung, keine ICD-Änderung).
- **Bezug:** ASD-012/013 (Navigation Rail), Häppchen 3 (Werkzeuge + Zoom kamen
  seinerzeit **in** die Rail), **ADR 0029** (Overlay-Zonen — dieses ADR fügt die
  **bottom-right-Zone für Desktop** hinzu). Register: **FR-UI-040**,
  Milestone **ASD-019**.

## Kontext

Die Rail trug bislang **eine** flache Icon-Spalte: erst RBL/DIST/QDM, dann
Layer/Filter, dann Zoom +/−, unten Konto. Zwei Probleme:

1. **Keine sichtbare Gruppierung.** Mess-Werkzeuge und Karten-Panels standen
   ununterscheidbar untereinander — der Lotse musste die Funktion aus dem Icon
   raten. Beides zeigte denselben cyan-farbenen Aktiv-Indikator, obwohl es
   **zwei verschiedene Dinge** sind: ein Mess-Werkzeug ist **modal** (es fängt
   Karten-Klicks ab, solange es „scharf" ist), ein Panel ist **nicht-modal** (es
   klappt nur eine Seitenspalte auf).
2. **Zoom als Fremdkörper in der Werkzeug-Leiste.** Zoom ist eine
   **Karten-Navigation**, kein Lotsen-Werkzeug. In der Rail machte es die Spalte
   länger und lag weit weg von der Karte, auf der es wirkt. (Genau das benannte
   das Mockup: „die Leiste verliert 2 Fremdkörper und wird kürzer".)

## Entscheidung

### 1. Gruppierte Rail mit Mikro-Labels

Die Rail wird in zwei benannte Sektionen geteilt — **MEASURE** (RBL/DIST/QDM)
und **MAP** (Layer/Filter) — jeweils unter einem **subdued Mikro-Label**
(Overline-Stil, in die 56-px-Spalte passend) und durch Trenner abgesetzt. Das
Konto sitzt darunter, per **Push-Divider** (auto-Top-Margin) an den Fuß der Rail
gedrückt und damit klar vom Rest getrennt. Englische Labels (Betreiber-Vorgabe).

### 2. Zwei Aktiv-Farben (Orange/Blau) — eine semantische Konvention

> **Werkzeuge (MEASURE) leuchten BERNSTEIN (`--wf-warning`), wenn scharf;
> Panels (MAP) behalten den CYAN-Indikator (`--v-theme-primary`).**

Die Farbe kodiert **Zustandsart**, nicht bloß Dekoration: Bernstein signalisiert
einen **modalen, scharfen** Modus (Achtung — Karten-Klicks gehen ans Werkzeug),
Cyan einen **offenen, nicht-modalen** Layer/Filter. Jede Familie trägt zusätzlich
eine **dezente, dauerhafte** Akzentleiste am linken Rand (amber/cyan), damit die
Codierung schon im Ruhezustand lesbar ist. Aktiv „leuchtet" das Symbol über einen
weichen Halo (`--wf-glow-armed` / `--wf-glow-selected`) hinter dem Pill.

Neue Tokens (`colors.css`): `--wf-state-armed` (Amber @ 16 %, Spiegel zu
`--wf-state-selected`), `--wf-glow-armed`, `--wf-glow-selected`.

### 3. Zoom in die bottom-right-Zone des Scopes (erweitert ADR 0029)

Zoom verlässt die Rail und wird zur **unteren rechten Ecke** der Karte. Damit
bekommt die in ADR 0029 begonnene Zonen-Ordnung ihre **bottom-right-Zone auch auf
dem Desktop**:

- Zoom +/− steckt in einer **positions-neutralen** `ZoomControls.vue` (kein
  eigener Offset — dieselbe Disziplin wie `ViewportControls`).
- `MapControls` ist die bottom-right-**Zone** und rendert jetzt auf **Desktop
  UND Mobil**. Sie zeigt immer Zoom; die Viewport-Aktionen (Recenter/Vollbild)
  fügt sie **nur mobil** hinzu — auf dem Desktop bleiben die in der top-right-Zone
  (ADR 0029), damit sie nicht doppelt erscheinen.
- `MapCanvas` rendert `MapControls` unbedingt und verdrahtet Zoom direkt an die
  Engine (`mapEngine.zoomIn/zoomOut`). Die native MapLibre-Zoom-Buttons bleiben
  aus (`showZoom:false`), um Doppel-Buttons zu vermeiden.

**Aktualisierte Zonen-Tabelle (Ergänzung zu ADR 0029):**

| Zone | Ort | Inhalt |
|------|-----|--------|
| **top-right** | `AsdView` `.top-right-cluster` | Header, Feed-Chip, Aktionen, **Viewport-Controls (Desktop)** |
| **bottom-left** | `AsdView` `.scope-legend-overlay` | Scope-Legende |
| **bottom-right** | `MapControls` `.map-controls` | **Zoom (Desktop + Mobil)** + Viewport-Controls (nur mobil) |
| **top-left** | MapLibre `NavigationControl` | Kompass |

## Konsequenzen

- Die Rail wird kürzer und selbsterklärend; Werkzeug-Zustand ist auf einen Blick
  von Panel-Zustand unterscheidbar (Sicherheit: ein scharfes Mess-Werkzeug wird
  nicht mit einem offenen Layer verwechselt).
- Zoom sitzt dort, wo es wirkt (an der Karte), konsistent auf allen Geräten.
- **Kopplung track-detail-card ↔ bottom-right-Zone:** Beide sind unabhängige
  `fixed`-Overlays am rechten Rand; die Karte-Detail-Karte reserviert die
  Zoom-Höhe über ein Token (`--wf-map-controls-reserve`), damit ihr Scroll-Bereich
  nie unter die Zoom-Knöpfe läuft. Eine dokumentierte Reservierung, wie schon für
  Top-Cluster (220 px) und Attribution — kein frei geratener Offset.
- **Ehrliche Grenze:** kein WebGL-/Mount-Harness für eine *visuelle* Zusicherung;
  die Struktur ist per Source-Guards gezurrt (`railTools.test.js`,
  `scopeChromeLayout.test.js`, `responsive.test.js`), die **optische Abnahme**
  (Farb-Codierung, Glow, Zoom-Position, keine Überlappung) macht der Betreiber.
