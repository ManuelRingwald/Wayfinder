# AERO-1 — OpenAIP: persistenter Cache + fetch-once

> Backend-Fundament für die „Connected-by-default"-Umstellung von OpenAIP
> (ADR 0018, in der Reihe von ADR 0017). Kippt das Fetch-Modell von
> periodisch-flüchtig auf **persistent + einmalig/on-demand**.

## Fachlich

OpenAIP-Aeronautik-Daten (Lufträume, Navaids, Wegpunkte) folgen dem **AIRAC-Zyklus**
(28 Tage) — sie sind quasi-statisch. Bisher zog Wayfinder sie über einen
**24-h-Ticker** und cachte sie **nur im RAM**: nach jedem Redeploy war der Cache
leer und wurde neu gezogen (Abruf-Sturm, kurzes leeres Kartenbild), und das
Dauer-Polling erzeugte Last ohne fachlichen Gewinn. AERO-1 dreht das um: **einmal
holen, persistent halten, bewusst aktualisieren** (zum AIRAC-Update) — passend zur
Prämisse aus ADR 0017 (Informations-Plattform, betreiber-getriebene Aktualität).

## Technisch

- **DB (Migration `00017`):** Tabelle `aeronautical_cache` (`tenant_id` NULL =
  globaler Fallback, `kind`, `geojson TEXT`, `feature_count`, `fetched_at`; zwei
  partielle Unique-Indizes wie bei `view_configs`). Store: `AeroCacheRepo`
  (`Load`/`Save`-Upsert/`Status`).
- **`pkg/aeronautical`:** neue paket-lokale `CacheStore`-Schnittstelle
  (DB-entkoppelt). `Service` bekommt `Hydrate` (Load aus DB, **kein Netz**),
  `HasData`, `BootstrapOnce` (hydrieren; nur bei leer + Schlüssel einmal fetchen),
  `RefreshNow` (erzwungen); `refreshAll` **persistiert** jeden Erfolg. **Der Ticker
  in `Run` entfällt** (`Run` entfernt). Die `Registry` fetcht ereignisgesteuert:
  `Start(…, force)` holt nur bei geänderten Inputs oder `force`; ein neuer Mandant
  mit persistierten Daten **hydratisiert nur** — damit ist ein Redeploy ein Hydrate,
  kein Fetch-Sturm.
- **`cmd/wayfinder`:** `aeroCacheStore`-Adapter (marshalt `FeatureCollection` ↔
  TEXT + Feature-Count), globaler Service via `BootstrapOnce` (keine
  Goroutine-Schleife mehr), `tenantAeroLifecycle.Apply` (nicht-erzwungen, für
  Boot/AOI) vs. `Refresh` (erzwungen, vom Schlüssel-Setzen).
  `WAYFINDER_OPENAIP_REFRESH` ist **deprecated/ignoriert** (Warn-Log beim Start).
- **Admin-Status:** `GET /api/admin/tenants/{id}/openaip` liefert zusätzlich
  `fetched_at` + `feature_count` (best-effort, weggelassen wenn nichts gecacht) via
  `AeroCacheStatusReader` (`WithAeroCache`). Das Setzen eines Schlüssels **erzwingt**
  jetzt einen Fetch (statt idempotentem Apply).

## Schnittstellen-Wirkung

**Keine.** Kein CAT062/065/063-Eingriff, keine Firefly-Koordination — reine
Wayfinder-interne Speicher-/Abruf-Architektur.

## Gates

- `go build/vet/gofmt` grün; `go test ./...` grün inkl. neuer Tests
  (`pkg/aeronautical/persist_test.go`: Hydrate-ohne-Netz, BootstrapOnce
  skip/fetch, refreshAll-persistiert, Force-Refetch, Neu-Mandant-hydratisiert;
  `pkg/store` real-PG `TestIntegrationAeroCacheRepo`; `pkg/adminapi` Status-Felder
  + Force-Refresh; `cmd/wayfinder` Deprecation-Config). golangci-lint 0 issues.
- vitest unverändert grün; **kein Frontend-Change** (AERO-1 ist Backend-Fundament),
  `dist` unverändert.

## Ehrliche Grenze

- Ändert sich die Abfrage-Box **außerhalb** eines laufenden Servers (z. B. ein
  geänderter Env-Default), erkennt der Boot das nicht automatisch; ein expliziter
  Refresh (AERO-2) deckt das ab. Für den Regelfall (AOI-Änderung über die laufende
  Admin-UI) ist es abgedeckt.
- Die OpenAIP-API ist in der Entwurfs-Umgebung nicht live erreichbar
  (Egress-Policy) — best-effort deckt das ab (leere/Last-Good-Anzeige, kein
  Absturz); der Live-Smoke-Test ist ein Deploy-Schritt.
- **Folgt (nicht Teil von AERO-1):** Refresh-Buttons (global + pro Mandant),
  globaler Schlüssel via Platform-Admin-UI + Fetch-all, Zeitstempel-Anzeige in der
  UI (**AERO-2**); AIRAC-Kalender + Change-Impact (**AERO-3**).
