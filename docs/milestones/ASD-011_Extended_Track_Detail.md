# ASD-011 — Erweitertes Track-Detail-Panel

> **Kontext:** Ausbau des mit **FR-UI-005** eingeführten Track-Detail-Panels.
> Rein Frontend, **kein CAT062-Bezug** — alle gezeigten Felder liegen bereits im
> WS-JSON (`pkg/broadcast.TrackMessage`), das Firefly aus dem CAT062-Strom
> ableitet. Keine neuen Env-Variablen, keine Wire-Änderung.

## Fachlich — warum

Beim Anklicken eines Tracks sah der Lotse bisher nur Callsign, Flugfläche,
Bodengeschwindigkeit, Mode 3/A, Track-Nummer und Status. Für die operative
Beurteilung eines Ziels fehlten damit mehrere Angaben, die im Datenstrom bereits
vorhanden sind:

- **Vertikaltendenz** — steigt/sinkt das Luftfahrzeug? (Konfliktbeurteilung)
- **Kurs über Grund** — wohin fliegt es? (das Label zeigt nur einen Vektorstrich)
- **Position (WGS84)** — exakte Koordinaten zum Absetzen/Weitergeben.
- **Sensor-Aktualität** — welche Überwachungstechnologien tragen den Track
  gerade, und wie frisch ist ihr Beitrag? (Vertrauens-/Qualitätsbild)
- **ICAO-Adresse** — die 24-Bit-Identität für eindeutige Zuordnung.
- **Positionsgenauigkeit** — die geschätzte Unsicherheit (Qualitätsindikator).
- **System (SAC/SIC)** — welches SDPS den Track erzeugt hat.

Das Panel wird damit zum vollständigen „Steckbrief" des Tracks, ohne dass der
Lotse Rohdaten anderswo nachsehen muss.

## Technisch — wie

### Reine Formatierer (`frontend/src/map/trackDetail.js`)
Neu, und bewusst als **reine Funktionen** ausgelagert (isoliert unit-testbar,
kein Vue-Mount nötig):

| Funktion | Feld | Ausgabe |
|----------|------|---------|
| `formatLatLon(lat, lon)` | I062/105 | `53.6304° N, 9.9882° E` (Dezimalgrad + Hemisphäre) |
| `formatHeading(vx, vy)` | I062/185 | `042°` (Kurs im Uhrzeigersinn ab Nord, `atan2(vx, vy)`) |
| `formatIcao(icaoAddr)` | I062/380 | `3C6DD2` (6-stellig Hex) |
| `formatAccuracy(accuracy)` | I062/500 | `±43 m` (gerundet) |
| `formatAge(ageS)` | I062/290 | `2.3 s` / `13 s` (< 10 s eine Nachkommastelle) |
| `verticalTrendLabel(trend)` | ASD-001b | `Steigend`/`Sinkend`/`Gleichbleibend` |
| `sensorAgeList(track)` | I062/290 | Liste `{label, ageS, fresh}` je Technologie |

`sensorAgeList` nutzt `isAdsbFresh` (30-s-Fenster, Wiederverwendung aus
`provenance.js`), listet nur Technologien mit vorhandenem Update-Alter und in
fester Anzeige-Reihenfolge (ADS-B, FLARM, SSR Mode A/C, Mode S). Fehlende Werte
und ein `null`-Track ergeben eine leere Liste, sodass das Panel die Sektion
sauber ausblendet. `formatHeading` normalisiert ein auf 360 gerundetes Ergebnis
zurück auf `000°`.

### Gebackene Feature-Properties (`frontend/src/map/tracks.js`)
`updateTracksLayer` legt die zusätzlichen Felder auf jedes Live-Track-Feature:
`latitude`, `longitude`, `icao_addr`, `accuracy`, `sac`, `sic`, die
per-Technologie-Alter (`adsb_age_s`/`flarm_age_s`/`ssr_age_s`/`mds_age_s`) und
`vertical_trend` (die ohnehin für das Label berechnete Tendenz `▲`/`▼`/`''`).
Das Panel liest die Auswahl direkt aus `store.selectedTrack` (= die Properties
des angeklickten Features), ohne die rohe WS-Nachricht festzuhalten — konsistent
mit dem bestehenden Muster (Bug #55: `mode_3a`/`callsign`).

### Panel (`frontend/src/components/TrackDetailCard.vue`)
Neue `v-list-item`-Zeilen mit den Formatierern als `computed`; jede Zeile ist
über `v-if` an das Vorhandensein ihres Feldes gebunden (nichts leeres wird
gezeigt). „Sensor-Aktualität" rendert je Technologie einen `v-chip` mit
Update-Alter; frische Beiträge (≤ 30 s) sind grün eingefärbt.

## Ehrliche Grenze

**PSR** erscheint **nicht** in „Sensor-Aktualität": `psr_age` liegt zwar immer
auf der Leitung, trägt aber kein sauberes Per-Track-Frische-Signal. Der
Primär-nur-Fall wird stattdessen über die bestehende „Herkunft"-Zeile
(FR-ASD-007, „Primär (PSR)") getragen. Die Vertikaltendenz stammt aus dem
FL-Vergleich zwischen zwei Updates (ASD-001b); vor dem zweiten Update eines
Tracks ist sie „Gleichbleibend".

## Schnittstellen-Wirkung

**Keine** am CAT062. Reines Frontend, alle Felder bereits im WS-JSON; `dist` neu
gebaut und eingebettet.

## Tests

- **`frontend/src/map/__tests__/trackDetail.test.js`** — alle Formatierer
  (Hemisphären, Kardinal-/Diagonal-Kurse, 360→000-Wrap, Hex-Padding,
  ±m-Rundung, Alters-Formatierung, Tendenz-Wörter, Sensor-Liste inkl.
  Primär-nur → leer).
- **`frontend/src/map/__tests__/tracks.test.js`** — `extended detail fields
  (ASD-011)`: Properties gebacken (Position/Identität/Genauigkeit/Alter) und
  Vertikaltendenz-Glyph über zwei Updates.

Gates grün: **vitest 456**, `vite build` + eingebettetes `dist` neu; Go
unberührt (`go build ./...` grün).
