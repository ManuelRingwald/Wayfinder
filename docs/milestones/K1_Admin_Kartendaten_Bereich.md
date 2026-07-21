# K1 — Admin-Bereich „Kartendaten": Rahmen + Status-Anzeige

> **Register:** FR-CFG-008 · **Epic:** #307 (Issue #309). Baut auf **K0** (#308,
> `pkg/mapconfig`) auf; die Editier-Panels folgen in **K2–K5**.

## Fachlich — warum

Ein neuer Admin-Bereich, der die vier Karten-Datenquellen (**Basiskarte, Wetter,
Radar-Abdeckung, Aeronautik**) an **einem Ort** bündelt und auf einen Blick zeigt,
**was konfiguriert und verfügbar** ist. K1 liefert **Rahmen + Status/Diagnose**
(read-only) — sofortiger Betriebsnutzen (der Admin sieht Fehlkonfigurationen
sofort), und der Rahmen, in den K2–K5 die Editier-Panels hängen. Das **Editieren
in der UI** folgt je Subsystem.

## Technisch

- **`AdminView.vue`**: der bisherige Top-Level-Abschnitt „OpenAIP" wird zum neuen
  Abschnitt **„Kartendaten"** (`section === 'mapdata'`, Icon `mdi-map-outline`,
  auch im Mobil-Select). Er rendert die neue Komponente `AdminMapData.vue`.
- **`AdminMapData.vue`** (neu): vier Reiter (`v-tabs`/`v-window`):
  - **Basiskarte** — Theme + Style-URL (aus `/api/map-config`).
  - **Wetter** — Verfügbarkeits-Chips für DWD-Regenradar, DWD-Warnungen, QNH
    (`weather_radar_available`/`weather_warnings_available`/`qnh_available`).
  - **Radar-Abdeckung** — Anzahl konfigurierter Sensoren (`coverage_sensor_count`)
    + Ringfarbe (`coverage_ring_color`).
  - **Aeronautik** — bettet das **bestehende** `AdminGlobalOpenAIP.vue` ein
    (globaler Schlüssel + AIRAC + Refresh, AERO-2/ADR 0018) — **keine Doppelung**,
    reine Wiederverwendung.
- **Single Source of Truth:** der Status liest **dasselbe** `/api/map-config`, das
  das ASD beim Start liest — der Admin sieht exakt, was der Scope sieht.

## Ehrliche Grenzen

- **Read-only in K1:** kein PUT/Speichern. Die Editier-Endpunkte (K0-`mapconfig`-
  Plane) kommen je Subsystem in K2 (Basiskarte), K3 (Wetter), K4 (Abdeckung);
  Aeronautik ist über das eingebettete Panel schon editierbar.
- **Kein WebGL-/Mount-Harness** für Admin-Panels → Struktur per Source-Guards;
  optische Abnahme durch den Betreiber.

## Tests

`components/__tests__/adminMapData.test.js` (neu): AdminView verdrahtet den
`mapdata`-Abschnitt (+ Mobil-Select), alter `openaip`-Abschnitt entfernt;
AdminMapData hat die vier Reiter, liest `/api/map-config` (Theme/Style +
Verfügbarkeits-Flags + Sensor-Anzahl), bettet `AdminGlobalOpenAIP` ein, ist in K1
read-only (kein PUT).
