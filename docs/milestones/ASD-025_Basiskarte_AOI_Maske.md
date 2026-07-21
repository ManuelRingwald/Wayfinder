# ASD-025 — BKG-Basiskarte auf die Mandanten-AOI begrenzen (#289)

> **Register:** FR-UI-049 · **Entscheidung:** ADR 0032 · **Auslöser:** #289
> (Betreiber-Wunsch: Basiskarte auf das Kundengebiet begrenzen).

## Fachlich — warum

Tracks und die Wetter-Overlays sind bereits auf das **Zuständigkeitsgebiet (AOI)**
des Kunden zugeschnitten — die Basiskarte war es nicht und wurde flächig
(Deutschland/Welt) gezeichnet. Jetzt endet auch die **Karte am Sektorrand**: außer­
halb der AOI wird sie mit der Scope-Hintergrundfarbe verdeckt. Der Blick des
Lotsen bleibt auf sein Gebiet konzentriert, außerhalb liegende Geografie lenkt
nicht mehr ab, und das Bild ist konsistent (Karte, Tracks, Wetter enden an
derselben Kante).

## Technisch — Masken-Ebene (Rechteck aus der AOI-BBox)

- **Form:** Rechteck aus der vorhandenen AOI-BBox (`{minLat,minLon,maxLat,maxLon}`
  aus `whoami`) — deckt sich exakt mit dem server-seitigen Track-/Wetter-Zuschnitt,
  kein neues Konfig-Feld. Kreis/Radius ist eine dokumentierte Folge-Option
  (ADR 0032 / #289).
- **Mittel:** eine **Masken-Fill-Ebene** — Welt-Polygon mit einem **Loch** in
  AOI-Form (`aoiMaskFeature`, `map/clip.js`, rein + getestet). Außerhalb deckt die
  Scope-Hintergrundfarbe die Karte; innen scheint sie durch. Schema-agnostisch
  (unabhängig von den BKG-Ebenennamen, funktioniert mit bkg/bkg-dark + den
  Element-Gruppen aus #290).
- **Z-Ordnung:** über der Basiskarte, **unter** allen Overlays — die Maske
  begrenzt nur die Karte, nie Tracks/Wetter/Aeronautik (die clippen selbst).
- **Bausteine:** `addBasemapMaskLayer`/`setBasemapMaskAOI` (`map/layers.js`,
  Spiegel zu `setWeatherRadarAOI`); Engine legt die Maske beim Style-`load` an und
  zieht sie über den bestehenden AOI-Hook (`applyWeatherAOI`) bei AOI-Änderungen
  nach.
- **Ohne AOI:** leere Maske → volle Karte (kein Zuschnitt), wie bei den Wetter-
  Overlays.

## Ehrliche Grenzen

- **Harte, rechteckige Kante.** Kreis/Radius **und** ein weicher (ausblendender)
  Rand sind dokumentierte Folge-Optionen (#289) — der Code ist vorbereitet
  (Kreis = nur den Loch-Ring in `aoiMaskFeature` tauschen).
- **Kein WebGL-/Mount-Harness** → Geometrie unit-getestet, Wiring per
  Source-Guards; die optische Abnahme (Karte endet am Sektorrand, Tracks/Wetter
  außerhalb weiter sichtbar) macht der Betreiber.

## Tests

- `map/__tests__/clip.test.js`: `aoiMaskFeature` — `null` ohne AOI / bei
  nicht-endlichen Bounds; Welt-Außenring + geschlossenes AOI-Loch.
- `components/__tests__/basemapLayer.test.js` (#289-Block): Maske über der
  Basiskarte vor den Overlays; `setBasemapMaskAOI` im AOI-Hook; Fill in der
  Scope-Farbe, leer bei null-AOI.

## Nachtrag #324 — Wetter mit beschneiden

Die Maske lag anfangs **unter** den Wetter-Overlays und begrenzte nur die Karte;
das Regenradar (Raster, nur kachel-granulare `bounds`) ragte über die scharfe
AOI-Kante hinaus. Seit #324 wird die Maske **über** Radar + Warnungen eingehängt
(Reihenfolge Basiskarte → Radar → Warnungen → Maske → Aeronautik/Coverage/Tracks),
sodass alle georeferenzierten Kartendaten dieselbe scharfe Kante teilen. Der
Radar-Re-Add (`setWeatherRadarAOI`) fügt den Layer stabil unter den Warnungen ein
(`beforeId`). Details: ADR 0032 (Nachtrag), FR-UI-051.
