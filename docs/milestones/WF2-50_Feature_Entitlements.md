# WF2-50 — Feature-Entitlement-Service (`tenant.HasFeature`, Flags als Daten)

> **Stufe:** 5 (Monetarisierung & HA) — vorgezogen · **Paket:** WF2-50 ·
> **Einstufung:** S3 · Sonnet 4.6 (überschaubare Backend-Logik auf vorhandenem
> Repo; Subtilität = Fail-Closed-Semantik + Cross-Tenant-Gating) ·
> **Grundlage:** ADR 0005 §4 (Entitlements als Daten, entkoppelt von Billing).

## Warum (fachlich)

Wayfinder 2.0 ist eine Multi-Mandanten-Plattform. Features müssen **pro Mandant
als Daten** schaltbar sein — ohne Code-Deploy, ohne Hartverdrahtung und
**entkoppelt von Billing** (WF2-51 ruht; `HasFeature` weiß nichts von Stripe).
Das ist der **Enabler**, an dem kommende Funktionen hängen: STCA-Anzeige
(ASD-006), Feed-Sensorklassen (WF2-41), Premium-ASD-Layer — pro Mandant
freischaltbar, **ohne den ASD-Kern anzufassen**.

**Sicherheit:** Ein Feature-Tor ist **fail-closed** — im Zweifel/Fehler **nicht**
freischalten. Ein Tor, das bei einem DB-Fehler „aufgeht", würde ein Premium-
oder sicherheitsrelevantes Feature leaken.

## Ausgangslage (Schlüssel-Befund)

Tabelle `entitlements` (`tenant_id, feature_key, enabled`, PK + `ON DELETE
CASCADE`) **und** `store.EntitlementRepo` (`Set`/`IsEnabled`/`ListByTenant`,
default-deny) existierten bereits aus WF2-10 — waren aber **nirgends
verdrahtet**. WF2-50 baute Service, Katalog, Admin-Verwaltung und Wiring darauf.

## Was (technisch)

### `pkg/feature` — der fail-closed Service
- **Klassifizierung/Lesen:** `HasFeature(ctx, tenantID, key) bool` und
  `Effective(ctx, tenantID) (map[Key]bool, error)` sind **Default-Deny**. Ein
  **unbekannter Katalog-Key** oder **jeder Store-Fehler** → `false` **plus**
  Warn-Log **plus** Zähler (`DBErrorCount`/`UnknownKeyCount`). `Effective`
  liefert selbst im Fehlerfall eine voll bestückte All-Deny-Map, damit ein
  Aufrufer, der den Fehler ignoriert, dennoch fail-closed ist.
- **Schreiben:** `Set(ctx, tenantID, key, enabled)` validiert gegen den Katalog
  (`ErrUnknownFeature` bei unbekanntem Key) — die DB akkumuliert nie Flags, die
  kein Code liest.
- **Katalog** (`catalog.go`): getypte `Key`-Konstanten `stca`, `multi_feed`,
  `premium_layers` + `IsKnown`/`All`/`Describe`. Der Katalog ist die **einzige
  Wahrheitsquelle** dafür, „welche Features existieren".
- **Kein Cache (v1):** Feature-Checks liegen **nicht** am heißen Track-Pfad
  (sie gaten Verfügbarkeit, nicht Per-Track-Rendering). Konsistent mit der
  WF2-30-Zurückstellung; ein TTL-/Redis-Cache kommt erst bei gemessenem Bedarf
  (und brächte sonst das Invalidierungs-Problem über mehrere Instanzen).

### Admin-Verwaltung (`pkg/adminapi`) — admin = Billing-Grenze
- `GET /api/admin/tenants/{tenantID}/entitlements` — die **volle** Katalog-Liste
  mit dem Zustand des Ziel-Mandanten (Default-Deny für nie gesetzte Keys), damit
  die UI jeden Toggle rendern kann.
- `PUT /api/admin/tenants/{tenantID}/entitlements/{key}` — `{"enabled": bool}`;
  unbekannter Key → **400**, unbekannter Mandant → **404**. **Kein** Live-Apply-
  Rescope (anders als View/Subscription): Entitlements gaten Verfügbarkeit, nicht
  den laufenden Track-Scope.
- Beide hinter `requireAdmin` — **cross-tenant nur admin** (NFR-SEC-003,
  Billing-Grenze, gespiegelt von WF2-31b).

### SPA-Schnittstelle
- `whoami` trägt jetzt die **effektiven Feature-Flags** (`features`,
  fail-closed), damit die SPA Rollen **und** Entitlements in einer Probe erhält.
- Frontend-Admin-Store: `features` + `hasFeature(key)`. **UI-Gating ist
  kosmetisch** — der Server erzwingt jedes Feature serverseitig.

### Observability & Wiring
- `cmd/wayfinder/main.go`: der Service wird **einmal** konstruiert (nur Multi-
  Mandant) und von Admin-API **und** `startProbeServer` geteilt.
- `/metrics`: `wayfinder_feature_check_failclosed_total{reason="db_error"|"unknown_key"}`.

## Sicherheit / Korrektheit
- **Fail-closed** auf allen Lesepfaden (unknown key, DB-Fehler → `false` +
  Warn-Log + Metrik), getestet.
- **Cross-Tenant-Isolation:** `user` erhält **403** auf beiden neuen
  Routen und erreicht den `Set`-Pfad nie (Negativtest erweitert).
- **Katalog-Guard** beim Schreiben verhindert Garbage-Keys in der DB.

## Tests
- `pkg/feature/*_test.go` (DB-frei): Default-Deny, beide Fail-Closed-Pfade
  (Zähler **und** Warn-Log asserted), Katalog-Overlay/Unknown-Key-Filter,
  `Set`-Validierung, Nil-Logger-Robustheit.
- `pkg/adminapi/adminapi_test.go`: admin GET/PUT, unknown-key 400,
  unknown-tenant 404, **Cross-Tenant 403** auf beiden Routen (+ `Set` nie
  erreicht), `whoami.features`.
- `pkg/adminapi/adminapi_integration_test.go` (**real-PG**, `scripts/pg-test.sh`):
  set→list-Round-Trip durch den echten `EntitlementRepo`, unbekannter Key → 400,
  `whoami.features` spiegelt den gesetzten Flag.
- `frontend/src/stores/__tests__/admin.test.js`: `hasFeature` true/false +
  Fail-Safe-Default, wenn `whoami` keine `features` liefert.
- Gates grün: `go test ./...` (+ real-PG via `pg-test.sh`), `go vet`, `gofmt`,
  `vitest` (80), `npm run build`.

## Abgrenzung / Nächstes
- **Kein** Frontend-Pflicht-Visual: das Gating-UI eines konkreten Features kommt
  erst mit dem Feature (z. B. ASD-006/STCA, WF2-41).
- **WF2-41** (Feed-Sensorklassen) kann nun auf `HasFeature("multi_feed")` bauen.
- **WF2-51** (Billing) bleibt ruhend; es würde nur Entitlements **setzen** (über
  dieselbe Datenbasis), den ASD-Kern weiterhin nicht berühren.
