# CBD-2 — DWD Radar + Warnungen „default-an"

> Häppchen 2 der „Connected-by-default"-Umstellung (ADR 0017). Erstes Feature, das
> vom Opt-in auf default-an kippt.

## Fachlich

Kein Env-Gefummel mehr für die DWD-Overlays: gibt ein Admin einem Mandanten das
Entitlement `weather_radar` bzw. `weather_warnings`, **funktioniert der Schalter
sofort** (die Quelle ist bereits an). Das behebt die Betreiber-Reibung
(„Overlay freigeschaltet, aber Quelle nicht konfiguriert").

## Technisch

- **`cmd/wayfinder/main.go`:** DWD-URLs bekommen einen **Default** (öffentlicher
  DWD-GeoServer: `…/dwd/wms` bzw. `…/dwd/ows`). Neue Schalter
  `WAYFINDER_DWD_RADAR_ENABLED` / `WAYFINDER_DWD_WARN_ENABLED` (Default **true**).
  `Enabled = <flag> && URL != ""`; `map-config.weather_radar_available` /
  `…warnings_available` werden dadurch **default true** (Sidebar-Schalter nutzbar,
  sobald das Entitlement gesetzt ist — Ebene 2 bleibt default-deny, Entscheidung B).
  URL-Override greift nur bei **gesetztem, nicht-leerem** Wert (Default bleibt sonst).
  Neuer Helper `envBool` (default-tolerant, `strconv.ParseBool`).
- **Compose:** Kommentar-Hinweis auf die `..._ENABLED`-Opt-outs (kein Pflicht-Eintrag).
- **Doku:** INSTALLATION §7.4 (default-an), §8.0-Rollout-Stand, TECHNICAL §6.4
  (Env-Tabelle + `..._ENABLED`), Register (FR-WX-001/003-Gating, NFR-OPS-005-Stand).

## Schnittstellen-Wirkung

**Keine.** Kein CAT062/065/063-Eingriff, keine Firefly-Koordination. Kein
Frontend-Code geändert (das `available`-Verhalten kommt zur Laufzeit aus
`map-config`); `dist` unverändert.

## Gates

- `go build/vet/gofmt` grün; `go test ./...` grün inkl. neue Config-Tests
  (`TestLoadConfigDWDConnectedByDefault`/`…EnabledOptOut`/`…URLOverride`,
  `TestEnvBool`). golangci-lint 0 issues.
- vitest unverändert grün; `dist` **nicht** neu gebaut (kein Frontend-Change).

## Ehrliche Grenze

QNH (NOAA) und OpenAIP folgen in den nächsten Häppchen und bleiben bis dahin
opt-in (QNH braucht die Flugplatz-Liste, OpenAIP einen Schlüssel). Die DWD-Quelle
ist in der Entwurfs-Umgebung weiterhin nicht live erreichbar (Egress-Policy) —
best-effort deckt das ab (transparente/leere Anzeige, kein Absturz).

> **Nachtrag:** QNH ist mit **CBD-3** (`CBD-3_QNH_per_tenant.md`) nachgezogen —
> NOAA-Quelle default-an (`WAYFINDER_QNH_ENABLED`), Flugplatz **pro Mandant**
> (`qnh_icao`). Es bleibt nur noch OpenAIP (AERO) übrig.
