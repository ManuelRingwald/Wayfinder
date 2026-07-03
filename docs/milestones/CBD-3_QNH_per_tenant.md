# CBD-3 — QNH „default-an" + Flugplatz pro Mandant

> Häppchen 3 der „Connected-by-default"-Umstellung (ADR 0017). Kippt die
> QNH-Infobox (WX-B) vom globalen Env-Opt-in auf **NOAA-Quelle default-an** und
> **Flugplatz pro Mandant**.

## Fachlich

Bisher hing die QNH-Anzeige an einer **globalen** Env-Liste
(`WAYFINDER_METAR_STATIONS`) — dieselbe Kopfzeilen-Station für alle Mandanten. Das
passt nicht zu einer Multi-Tenant-Plattform: Ein kleiner Flugplatz will das QNH
**seines** Platzes sehen, nicht das eines fremden Flughafens. Jetzt setzt der Admin
je Mandant den **Flugplatz-ICAO** in der Admin-UI; die Kopfzeile zeigt dessen QNH.
Passend zur connected-by-default-Prämisse ist die NOAA-Quelle **default-an** —
kein Env-Gefummel mehr, nur noch Entitlement `qnh` + Flugplatz-Feld.

## Technisch

- **DB (`pkg/store`):** Migration `00016` — Spalte `qnh_icao` auf `view_configs`
  (spiegelt `icao`, aber ein **echter** Aerodrom-ICAO, kein Sektor-Label). Neue
  Methode `DistinctQNHICAOs` liefert die Vereinigung aller gesetzten Flugplätze.
- **`pkg/weather`:** Der Poller fragt nicht mehr eine statische Liste ab, sondern
  die **dynamische Vereinigung** der Mandanten-Flugplätze (`StationsProvider`, je
  Refresh frisch aus der DB). `Enabled` bedeutet jetzt nur noch „NOAA-Quelle an/aus"
  (nicht mehr an Stationen gekoppelt) — eine Quelle, die an ist, aber nichts zu
  pollen hat, läuft weiter und dient leer. Neuer tenant-scoped Serve-Pfad
  `SnapshotFor(icaos)` / `TenantHandler(resolve)` + `Refresh()`-Kick für sofortige
  UX beim Speichern. `WAYFINDER_METAR_STATIONS` bleibt **deprecated** globaler
  Fallback.
- **`cmd/wayfinder/main.go`:** `WAYFINDER_QNH_ENABLED` (Default **true**),
  NOAA-URL/UA default-an (Override nur bei nicht-leerem Wert), DB-`StationsProvider`,
  tenant-aware `/api/weather/qnh`-Handler (löst `qnh_icao` aus dem Tenant-Kontext),
  QNH-Kick im rescope-Pfad. `map-config.qnh_available` spiegelt die Quelle.
- **`pkg/adminapi` + Frontend:** `qnh_icao` in `viewDTO` (Validierung
  `validICAOCode` = 4-stelliger ICAO, `normalizeQNHICAO` = trim+upper), Feld
  „QNH-Flugplatz (ICAO)" in `AdminTenantDetail.vue`, Format-Parität in
  `validateView.js`.

## Schnittstellen-Wirkung

**Keine.** Kein CAT062/065/063-Eingriff, keine Firefly-Koordination.

## Gates

- `go build/vet/gofmt` grün; `go test ./...` grün inkl. neuer Tests
  (`pkg/weather`: Provider-Union + tenant-scoped Snapshot + `TenantHandler` +
  `Refresh`-Kick + Quelle-aus/an; `pkg/adminapi`: `validateViewQNHICAO` +
  `normalizeQNHICAO`; `cmd/wayfinder`: `QNHConnectedByDefault`/`OptOut`/
  `MetarStationsFallback`). golangci-lint 0 issues.
- vitest grün inkl. `validateView`-ICAO-Format-Parität; `dist` neu gebaut
  (Admin-UI-Feld ⇒ Frontend-Change).

## Ehrliche Grenze

Die NOAA-Quelle (`aviationweather.gov`) ist in der Entwurfs-Umgebung nicht live
erreichbar (Egress-Policy) — best-effort deckt das ab (leere Anzeige, kein
Absturz). Der Live-Smoke-Test (echtes QNH sichtbar) ist ein Deploy-Schritt. Ein
Mandant ohne gesetzten `qnh_icao` und ohne globalen Fallback sieht keine
QNH-Anzeige (Vorgabe „keine Fake-UI"). OpenAIP (AERO) folgt als eigenes Häppchen.
