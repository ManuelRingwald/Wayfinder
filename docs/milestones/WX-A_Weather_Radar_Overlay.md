# WX-A — DWD-Wetter-Radar-Overlay

> Häppchen A der Wetter-Erweiterung. Baut auf WX-0 (ADR 0016) auf.

## Fachlich

Der Lotse sieht heute nur die reine Luftlage. Ein DWD-Niederschlags-/Radar-Bild
unter den Tracks macht das aktuelle Wettergeschehen (Gewitterzellen,
Niederschlagsgebiete) sichtbar und damit wetterbedingte Umfliegungen,
Verzögerungen und Konfliktpotenzial. Ausdrücklich **best-effort
Hintergrund-Kontext** — der CAT062-Track-Pfad ist unberührt; fällt der DWD aus,
fehlt nur das Overlay.

## Technisch

**Backend — Tile-Proxy (`pkg/weathertiles`).** MapLibre hat keinen nativen
WMS-Client, also fordert das Frontend Standard-XYZ-Kacheln von Wayfinder
(`/api/weather/radar/{z}/{x}/{y}.png`), und der Server übersetzt jede Kachel in
einen DWD-GeoServer-WMS-`GetMap`:

- `tiles.go` — XYZ→EPSG:3857-BBox-Mathematik (Web-Mercator) + 1×1-transparente
  Fallback-Kachel + Koordinaten-Validierung.
- `client.go` — WMS-1.1.1-`GetMap`-URL-Bau (`srs=EPSG:3857`, unzweideutige
  Achsenreihenfolge), defensiver GET: `io.LimitReader` (4 MiB/Kachel),
  Status-/Content-Type-Prüfung (GeoServer-`ServiceException`-XML wird verworfen).
- `service.go` — TTL-Cache je Kachel (Default 5 min, DWD-Radar-Kadenz),
  `TileHandler` (liest `{z}/{x}/{y}`, trimmt `.png`), Fehlerpfad → Last-Good bzw.
  transparente Kachel (immer HTTP 200), Erfolg/Fehler-Zähler + Cache-Alter.

**Verdrahtung (`cmd/wayfinder/main.go`).** Config `WAYFINDER_DWD_WMS_URL` /
`_RADAR_LAYER` / `_REFRESH`; Service (kein Hintergrund-Loop — Kacheln on-demand);
Route hinter `tenantMW` (nur authentifiziert erreicht den Egress);
`map-config.weather_radar_available`; Metriken `wayfinder_weather_*{source="dwd_radar"}`.

**Feature-Gate.** Feature-Key `weather_radar` (`pkg/feature/catalog.go`); der
Sidebar-Schalter erscheint nur bei Entitlement (`showLayer`) und ist deaktiviert,
wenn keine WMS-URL konfiguriert ist.

**Frontend.** Erste Raster-Overlay-Quelle in `layers.js`
(`addWeatherRadarLayer`); im `engine.js`-Load-Handler **zuerst** aufgerufen
(Z-Order: über Basiskarte, unter Aeronautik/Tracks) + `groups`-Eintrag in
`setLayerVisibility`; Store `weatherRadar` (aus) + `weatherRadarAvailable`;
Toggle „DWD-Regenradar" in `LayerFilterContent.vue`; Attribution „© Deutscher
Wetterdienst".

## Schnittstellen-Wirkung

**Keine.** Kein CAT062/CAT065/CAT063-Eingriff, keine Firefly-Koordination. Neue
ausgehende Abhängigkeit zu `maps.dwd.de` (Vertrauensgrenze, ADR 0016 /
NFR-SEC-005).

## Gates

- `go build/vet ./...`, `gofmt` sauber; `go test ./...` grün (inkl. neue
  `pkg/weathertiles`-Tests: BBox-Referenzwerte, Cache/TTL, Fallback-Kette,
  Nicht-Bild-Ablehnung, ungültige Koordinaten, Handler-Pfad).
- vitest grün (Store-Regression `weatherRadar`); `vite build` → `internal/webui/dist`
  neu eingebettet.

## Ehrliche Grenze

Die exakten DWD-Layer-Namen und die EPSG:3857-Unterstützung konnten in dieser
Umgebung nicht live verifiziert werden (Egress-Policy). Der Proxy ist defensiv:
ein falscher Layer/Endpoint ergibt transparente Kacheln, keinen Absturz. Ein
Live-Smoke-Test aus offenem Netz (GetCapabilities/GetMap) bleibt ein Deploy-Schritt.
