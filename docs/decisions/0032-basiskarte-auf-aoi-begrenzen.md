# ADR 0032 — BKG-Basiskarte auf die Mandanten-AOI begrenzen (Masken-Ebene, Rechteck)

- **Status:** **AKZEPTIERT** ✅ (2026-07-20). Betreiber-Wunsch (#289): die
  amtliche Basiskarte soll — wie die operativen Overlays — auf das
  **Zuständigkeitsgebiet (AOI)** des Kunden begrenzt sein, damit der Blick des
  Lotsen auf seinen Sektor konzentriert bleibt.
- **Datum:** 2026-07-20
- **Schnittstellen-relevant:** nein (reines Browser-Chrome; kein CAT062-/
  Firefly-Bezug, keine Backend-Wirkung). Register: **FR-UI-049**.
- **Bezug:** #289, ADR 0026 (BKG-Basiskarte), #274 (Basiskarte als
  Entitlement-Layer), WF2-21.2 (server-seitiger AOI-Zuschnitt), #189/#190
  (AOI-Zuschnitt der Wetter-Overlays — die Blaupause).

## Kontext

Tracks und die Wetter-Overlays (DWD-Radar/-Warnungen) werden **bereits** auf die
Mandanten-AOI zugeschnitten (server-seitig fail-closed + client-seitig geclippt);
die AOI ist intern eine **WGS84-BBox** `{minLat,minLon,maxLat,maxLon}` aus
`whoami`. Die **Basiskarte** war davon ausgenommen und wurde flächig (Deutschland/
Welt) gerendert, auch weit außerhalb des Kundengebiets.

## Entscheidungen

### 1. Zuschnitt-Form: Rechteck aus der vorhandenen AOI-BBox (nicht Kreis/Radius)

Der Betreiber denkt die Kunden-AOI als **Radius (Kreis)**; der bestehende,
erzwungene Zuschnitt (Tracks, Wetter) ist aber ein **Rechteck (BBox)**. Gewählt
ist das **Rechteck aus der vorhandenen BBox**, weil:

- es sich **exakt mit dem server-seitigen Track-/Wetter-Zuschnitt deckt** →
  konsistentes Bild (Karte, Tracks und Wetter enden an derselben Kante);
- es **kein neues Konfig-Feld** braucht (die BBox ist schon da) und rein
  Frontend ist;
- eine **Kreis-/Radius-Variante** ein neues Radius-Feld beim Mandanten (Admin-API
  + View-Profil-Schema + UI) verlangte und die Karte kreisförmig, Tracks/Wetter
  aber rechteckig ließe (Inkonsistenz) — es sei denn, die AOI würde **insgesamt**
  auf Radius umgestellt. Das ist ein größeres, eigenständiges Vorhaben.

**Offen gehalten (Folge-Option, #289):** eine echte Kreis-/Radius-AOI. Der Code
ist dafür vorbereitet — die Maskengeometrie steckt in **einer** reinen Funktion
(`aoiMaskFeature`, `map/clip.js`); ein Kreis-Zuschnitt tauscht nur deren Loch-Ring.

### 2. Technik: Masken-Ebene (nicht Style-Filter je Ebene)

Über die Basiskarte wird eine **Fill-Ebene** in der Scope-Hintergrundfarbe gelegt,
die die **ganze Welt** deckt und ein **Loch** in AOI-Form hat (GeoJSON-Polygon,
Außenring = Welt, Innenring = AOI-Rechteck). Außerhalb der AOI verdeckt sie die
Karte; innen scheint die Karte durch. Gewählt statt `layers[].filter` je
Basiskarten-Ebene, weil die Maske **schema-agnostisch** ist (unabhängig von den
driftenden BKG-`source-layer`-Namen, funktioniert mit bkg/bkg-dark und den
Element-Gruppen aus #290) und robust bleibt.

- **Z-Ordnung:** oberhalb der Basiskarte, **unterhalb** aller operativen Overlays
  (Wetter, Aeronautik, Tracks). Die Maske begrenzt **nur die Karte** — Tracks,
  Wetter und aeronautische Layer bleiben außerhalb der AOI sichtbar (sie clippen
  selbst nach eigener Logik).
- **Farbe/Kante:** volle Deckung in der Scope-Hintergrundfarbe (`#070b12`) →
  außerhalb sieht es aus wie der blanke Scope (harte Kante). Ein weicher/
  ausblendender Rand ist optionaler Feinschliff (#289).
- **Ohne AOI:** kein Loch-Polygon → leere Maske → volle Karte (kein Zuschnitt).

## Umsetzung

- `map/clip.js`: `aoiMaskFeature(bbox)` (rein, getestet) — Welt-Polygon mit
  AOI-Loch; `null` ohne AOI / bei nicht-endlichen Bounds.
- `map/layers.js`: `addBasemapMaskLayer(map, aoi)` + `setBasemapMaskAOI(map, aoi)`
  (Spiegel zu `setWeatherRadarAOI`).
- `map/engine.js`: Maske beim Style-`load` direkt über der Basiskarte (vor den
  Overlays); AOI-Änderungen laufen über den bestehenden AOI-Hook
  (`applyWeatherAOI` → auch `setBasemapMaskAOI`).

## Konsequenzen

- Die Basiskarte endet jetzt am Sektorrand wie Tracks + Wetter — ein
  konsistentes, fokussiertes Lagebild.
- **Ehrliche Grenzen:** harte rechteckige Kante (Kreis/Radius + weicher Rand sind
  dokumentierte Folge-Optionen, #289); kein WebGL-/Mount-Harness → Wiring per
  Source-Guards, optische Abnahme durch den Betreiber.
