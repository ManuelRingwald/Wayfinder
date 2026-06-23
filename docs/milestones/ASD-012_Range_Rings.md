# ASD-012 — Range-Rings + Scale-Bar + Nord-Orientierung

> **Paket:** ASD-012 (ASD-Kern-Feinschliff) · **Einstufung:** S3 · Opus 4.8
> (geodätische Geometrie + Orientierungs-Handhabung) · **Branch:**
> `claude/asd-012-range-rings` (off `main`, unabhängig von WF2-41).

## Warum (fachlich)

Der Lotse schätzt permanent **Distanz und Orientierung** ab. Drei klassische
ASD-Hilfen fehlten:

1. **Range-Rings** — konzentrische **NM-Kreise** um den Referenz-Mittelpunkt
   („dieses Flugzeug ist ~25 NM draußen"), für Staffelung/Sequencing.
   Abgegrenzt von den **Sensor-Coverage-Ringen** (Paket 6 = Radar-Reichweite) —
   Range-Rings sind ein **Distanz-Raster** um einen Bezugspunkt.
2. **Scale-Bar** — verankert bei variablem Zoom die absolute Distanz.
3. **Nord-Orientierung** — bei gedrehter Karte muss „wo ist Norden?" immer
   sichtbar und per Klick rücksetzbar sein.

## Was (technisch)

### A. Range-Rings (geodätisch, operator-konfigurierbar)
- **`src/map/rangerings.js`** (rein, getestet): `destinationPoint(lat, lon, d,
  bearing)` über die sphärische Destination-Point-Formel; `ringPolygon` sampelt
  den Ring (128 Segmente); `rangeRingsGeoJSON` baut LineString-Ringe + NM-Label-
  Punkte (Label nördlich des Zentrums).
- **Anti-Squish (der kritische Punkt):** Jeder Ring-Vertex liegt in *jeder*
  Richtung in **derselben Boden-Distanz**. Der naive „d/111320° auf lat und lon
  gleich" würde den Ring in Längsrichtung stauchen (1° lon < 1° lat in Metern),
  also auf Web-Mercator „gequetscht" zeichnen. Test `does not squash longitude`
  beweist: gleiche Meter (haversine) in Ost und Nord, aber **größeres Längen- als
  Breiten-Delta in Grad**.
- **Live-konfigurierbar:** Abstand (5/10/15 NM) + Anzahl liegen als **reaktiver
  Pinia-State** in `asd.js` (`rangeRingConfig`, Default 10 NM / 5). Die Sidebar
  (`LayerFilterContent.vue`) zeigt — **nur wenn der Layer aktiv ist** — einen
  Abstand-Select + Anzahl-Slider; der Toggle steuert die Sichtbarkeit (default
  **aus**, Declutter). `MapCanvas.vue` watcht `rangeRingConfig` und ruft
  `engine.updateRangeRings(spacing, count)`, das das Overlay aus `cfg.center`
  (aus `/api/map-config`, tenant-skopierbar via WF2-30/31) neu erzeugt.
- **Layer:** `addRangeRingsLayer` (Linien gestrichelt + Symbol-Labels, eine
  GeoJSON-Source, `kind`-Property trennt Ring/Label), unter den Track-Layern.

### B. Scale-Bar
- MapLibre-Built-in `ScaleControl({ unit: 'nautical', maxWidth: 120 })`,
  bottom-left.

### C. Nord-Orientierung
- MapLibre-Built-in `NavigationControl({ showZoom: false, showCompass: true,
  visualizePitch: false })`, top-left: zeigt das Bearing und setzt per Klick auf
  Nord. Freie Kartendrehung (Drag-Rotate) bleibt; Zoom bleibt auf den
  Custom-Controls (`showZoom: false` vermeidet Doppel-Buttons).
- Der **alte hand-gebaute Reset-Nord-Button** (`MapControls.vue` +
  `engine.resetNorth`) wurde **entfernt** — der native Kompass übernimmt
  (Declutter, kein UI-Wildwuchs).
- **Kein wörtliches „Track-up":** für ein Multi-Track-ASD ist eine Drehung auf
  *einen* Kurs undefiniert (Cockpit-Feature). North-up + freie Drehung ist der
  operative Default; ein fester Sektor-/Runway-Kurs wurde bewusst nicht gebaut.

## Tests
- `src/map/__tests__/rangerings.test.js` (9 Vitest): Destination-Distanz für
  jedes Bearing (konstante Boden-Distanz), Nord/Ost-Richtung, **Anti-Squish**,
  Ring-Schließung + Radius-Korrektheit, GeoJSON-Struktur, Count-Clamping,
  Label-Platzierung.
- `vitest run` 89 grün, `npm run build` grün.
- **Optik manuell verifiziert** (Ring-Stil/-Kontrast, Control-Platzierung,
  Kompass-Verhalten) — kein WebGL-Test-Harness im Projekt (vgl. FR-ASD-001/003).

## Abgrenzung / Nächstes
- View-Config-Migration (Abstand/Anzahl je Mandant) ist vorbereitet: liest schon
  `/api/map-config` für das Zentrum; Ring-Parameter könnten später additiv in die
  View-Config wandern (WF2-30/31), ohne Rework.
