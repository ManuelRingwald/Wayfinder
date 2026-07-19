# ADR 0029 — ASD-Chrome-Overlay-Zonen: neues Bedien-Chrome fließt, statt frei zu schweben

- **Status:** **AKZEPTIERT** ✅ (2026-07-19). Betreiber-Wunsch nach einem
  Layout, in dem sich Bedien-Elemente **nicht mehr überlappen**, wenn neue
  Funktionen dazukommen („vielleicht Bereiche festlegen, wo neue Funktionen
  dazukommen können").
- **Datum:** 2026-07-19
- **Schnittstellen-relevant:** nein (reines Browser-Chrome-Layout; kein
  CAT062-/Firefly-Bezug, keine Backend-Wirkung).
- **Bezug:** ASD-009 (Map-Controls), #194/FR-UI-029 (voriger Überlappungs-Flick),
  #277/ADR 0028 (Such-Icon, dessen Hinzukommen die Überlappung zuletzt auslöste).
  Register: **FR-UI-039**.

## Kontext — die wiederkehrende Bug-Klasse

Am rechten Scope-Rand lagen **zwei unabhängig positionierte** Overlay-Stapel:

1. `.top-right-cluster` (in `AsdView`) — ICAO/UTC-Header, Feed-Chip, Aktions-Icons
   (Profil, Ereignis-Glocke), zuletzt die **Such-Lupe**. Wächst nach unten.
2. `.map-controls` (Recenter/Vollbild, in `MapControls`/`MapCanvas`) — hing mit
   einem **fest verdrahteten** `top: calc(… + 140px)` da, das die **Höhe** von
   Stapel 1 nur **riet**.

Jedes neue Element in Stapel 1 verschob dessen Unterkante — der geratene Offset
in Stapel 2 stimmte dann nicht mehr → **Überlappung**. Genau das passierte bei
#194 (Profil + Glocke, geflickt `100px`→`140px`) und erneut mit dem Such-Icon
(4. Zeile, Controls saßen wieder darauf). Ein hart kodierter Offset, der die
Größe eines Nachbarn annimmt, **muss** brechen — es ist eine Bug-**Klasse**, kein
Einzelfall.

## Entscheidung — Overlay-Zonen (eine Flex-Spalte je Rand/Ecke)

Chrome über der Karte wird in **Zonen** organisiert. Eine Zone ist **ein**
positionierter Flex-Container; jedes Bedien-Element ist ein **Flex-Kind** dieser
Zone und liegt im **Fluss**. Kommt etwas Neues dazu, wächst der Container und
**schiebt** die darunter liegenden Geschwister — nichts überlappt, weil es keine
unabhängigen, geratenen Offsets mehr gibt. (Dasselbe Muster nutzt MapLibre für
seine eigenen Controls: `.maplibregl-ctrl-top-right` ist so ein Flex-Stapel.)

**Die verbindliche Regel:**

> **Neues Chrome kommt als Flex-Kind in eine bestehende Zone — nie als neues,
> frei-positioniertes `position:absolute`-Element mit eigenem, geratenem
> `top`/`right`.** Braucht eine Funktion wirklich eine neue Position, wird eine
> **Zone** definiert (ein Container), nicht ein Einzel-Element frei gehängt.

### Umsetzung (ASD-018)

- Die **rechte Kante** ist die erste konsequent umgesetzte Zone: `.top-right-cluster`
  ist die eine Flex-Spalte, und die Viewport-Controls (Recenter/Vollbild) sind ihr
  **letztes Flex-Kind** — sie fließen unter alles darüber, egal wie viele Zeilen
  der Cluster hat. Der geratene `top:140px` ist **weg**.
- Die Controls stecken in einer **positions-neutralen** Komponente
  (`ViewportControls.vue`, kein eigener Offset); die Zone (Desktop: der Rail in
  `AsdView`) legt sie aus. **Mobil** rendert `MapControls` denselben
  `ViewportControls` in seinem eigenen Stapel unten rechts (andere Ecke, andere
  Zone) — kein Doppel-Code.
- `MapCanvas` rendert `MapControls` nur für `!mdAndUp` und exponiert `recenter`
  für den Desktop-Rail.

### Bekannte Zonen (Bestand + Konvention für Neues)

| Zone | Ort | Inhalt |
|------|-----|--------|
| **top-right rail** | `AsdView` `.top-right-cluster` | Header, Feed-Chip, Aktionen (Profil/Glocke/Suche), Viewport-Controls |
| **bottom-left** | `AsdView` `.scope-legend-overlay` | Scope-Legende |
| **bottom-right (mobil)** | `MapControls` `.map-controls` | Zoom + Viewport-Controls (nur `!mdAndUp`) |
| **top-left** | MapLibre `NavigationControl` | Kompass |

Neues Chrome ordnet sich einer dieser Zonen zu (Flex-Kind) oder bekommt eine
**neue Zone** als Container.

## Konsequenzen

- Die Überlappungs-Bug-Klasse ist strukturell beendet: Ein künftiges Icon in der
  rechten Zone schiebt die Controls, statt sie zu überlagern.
- Transiente Panels (Ereignis-Log, aufgeklapptes Suchfeld) liegen weiter im
  Fluss der Zone — beim Öffnen **schieben** sie die Controls nach unten (korrektes
  Verhalten, keine Überlappung). Sollen die Controls beim Öffnen völlig ruhig
  bleiben, werden diese Panels später zu **Overlays** der Zone (absolut, aus dem
  Fluss) — ein optionaler Feinschliff, nicht Teil dieser Entscheidung.
- **Ehrliche Grenze:** Es gibt keinen WebGL-/Mount-Harness für eine *visuelle*
  Zusicherung; die Struktur ist per Source-Guards festgezurrt
  (`scopeChromeLayout.test.js`), die optische Abnahme macht der Betreiber.
