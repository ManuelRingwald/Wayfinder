# WX-C — DWD-Wetterwarnungen-Overlay

> Häppchen C der Wetter-Erweiterung. Baut auf WX-0 (ADR 0016) auf; folgt dem
> OpenAIP-Muster (Fetch → Cache → GeoJSON-Serve).

## Fachlich

Amtliche DWD-Warnungen (Gewitter, Sturm, Schnee/Eis …) als optionaler
Kontext-Layer unter der Luftlage, nach Warnstufe eingefärbt. Best-effort — der
CAT062-Track-Pfad ist unberührt.

## Technisch

**Backend (`pkg/weatherwarnings`).** Reines OpenAIP-Muster:

- `client.go` — DWD-GeoServer-**WFS**-Client: `GetFeature` mit
  `outputFormat=application/json`, `srsName=EPSG:4326` (GeoJSON ist ohnehin
  lon,lat → kein Achsenreihenfolge-Problem). Defensiv: `io.LimitReader` (16 MiB),
  toleranter Decode, ungültige Geometrie verworfen. `normaliseProps` reduziert die
  CAP-Attribute **case-insensitiv** auf eine stabile Form (`wf_level` 1–4 aus
  `SEVERITY`, plus `headline`/`event`/`expires`).
- `geojson.go` — normalisierte `FeatureCollection` + `validGeometry` (wie
  Aeronautik) + `EmptyCollection` (nie nil).
- `service.go` — Refresh-Loop (Default 5 min), `atomic.Pointer`-Cache mit
  Last-Good-Fallback, `Handler` → `GET /api/weather/warnings.geojson`,
  Erfolg/Fehler-Zähler + Cache-Alter.

**Verdrahtung (`cmd/wayfinder/main.go`).** Config `WAYFINDER_DWD_WARN_URL` /
`_LAYER` / `_REFRESH`; Service + `go weatherWarn.Run(ctx)`; Route hinter
`tenantMW`; `map-config.weather_warnings_available`; Metriken
`wayfinder_weather_*{source="dwd_warnings"}`.

**Feature-Gate.** Feature-Key `weather_warnings`; Sidebar-Schalter nur bei
Entitlement (`showLayer`) und deaktiviert ohne konfigurierte WFS-URL.

**Frontend.** `addWeatherWarningsLayer` (GeoJSON-Source + `fill`+`line`,
Severity-Farbrampe via `match` auf `wf_level`), im `engine.js` nach dem
Radar-Raster geladen (Z-Order: über Radar, unter Aeronautik/Tracks) +
`groups`-Eintrag; Fetch + 5-min-Refresh via `setData`; Store `weatherWarnings` +
`weatherWarningsAvailable`; Toggle „DWD-Wetterwarnungen"; Attribution
„© Deutscher Wetterdienst".

## Schnittstellen-Wirkung

**Keine.** Kein CAT062/CAT065/CAT063-Eingriff, keine Firefly-Koordination. Neue
ausgehende Abhängigkeit zu `maps.dwd.de` (Vertrauensgrenze, ADR 0016).

## Gates

- `go build/vet/gofmt` sauber; `go test ./...` grün (neue
  `pkg/weatherwarnings`-Tests: `severityLevel`, `normaliseProps` case-insensitiv,
  WFS-URL, Fetch parst + verwirft ungültige Geometrie + re-serialisiert,
  Non-200-Fehler, leer-vor-Fetch, Refresh-Cache + Last-Good, disabled, CacheAge).
- vitest grün (Store-Regression); `vite build` → `internal/webui/dist` neu
  eingebettet.

## Ehrliche Grenze

Der exakte WFS-Layer-Name und die GeoJSON-Ausgabe konnten in dieser Umgebung nicht
live verifiziert werden (Egress-Policy). Der Client ist defensiv (falscher
Layer/Endpoint ⇒ leeres Overlay, kein Absturz); ein Live-Smoke-Test
(GetCapabilities/GetFeature) aus offenem Netz bleibt ein Deploy-Schritt.
