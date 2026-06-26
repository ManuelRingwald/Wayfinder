# ONB-4 — Mandanten-Lebenszyklus über die Oberfläche

> Teil des Zero-Touch-Onboarding-Epics (ADR 0011). Dieses Paket erlaubt dem
> Betreiber, Kunden-Mandanten **anzulegen und zu löschen**, ohne Terminal — der
> letzte rein-CLI-Provisionierungsschritt entfällt.
>
> **Lieferung in zwei Commits:** (1) Backend, (2) Frontend (Anlegen-Dialog +
> Lösch-Bestätigung + Store-Actions).

## Fachlicher Hintergrund

Bisher entstanden Mandanten nur über `wayfinder bootstrap`. Für „nur die Container
starten, alles andere aus der Oberfläche" fehlte die UI-gestützte Mandanten-
Verwaltung. ONB-4 schließt das.

Das Löschen eines Mandanten ist **destruktiv** (es kaskadiert auf alle abhängigen
Zeilen). Daher steht die Sicherheits-Abwägung im Zentrum — siehe Guard B.

## Was umgesetzt wurde (Backend)

### 1. Store — `TenantRepo.Delete`

Einzeiliges `DELETE FROM tenants WHERE id=$1`. Alle tenant-referenzierenden
Tabellen tragen bereits `ON DELETE CASCADE` (`users` → `credentials`,
`subscriptions`, `view_configs`, `entitlements`), das Löschen ist also **atomar**
ohne explizite Transaktion. Fehlende Zeile → `ErrNotFound`. Feeds sind ein
**globaler** Katalog und überleben das Löschen eines Mandanten.

### 2. API — `POST`/`DELETE /api/admin/tenants`

Beide hinter `requireAdmin`.

- **`POST /api/admin/tenants`** `{slug, name?}` → 201. Slug DNS-label-artig
  validiert (`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, ≤ 63), `name` Default = `slug`.
  Doppelter Slug → **409** (Pre-Check via `GetBySlug`, wie `createUser`).
- **`DELETE /api/admin/tenants/{id}`** → 204; Guard siehe unten.

### 3. Guard B — „Mandant nicht leer" (Betreiber-Entscheidung)

`DELETE` wird **fail-closed mit 409** abgelehnt, solange der Mandant noch
**Zugänge** hat. Der Betreiber muss die Konten bewusst zuerst entfernen — ein
einzelner Klick kann so nie still einen ganzen Lotsen-Login-Bestand vernichten.
Die destruktive Kaskade ist ein bewusster Zweischritt.

> **Hinweis zur ADR-Prämisse:** Der ursprünglich in ADR 0011 §5 skizzierte Guard
> („Mandant mit aktiven Admins schützen") ist nach **ONB-3** gegenstandslos —
> Admins sind tenantlos, ein Mandant-Löschen kann keinen Admin treffen. Guard B
> übersetzt den **Schutz-Geist** des ADR (kein versehentlicher destruktiver
> Verlust) in die neue Welt. Der Komfort-Mandant `default` wird **nicht**
> gesondert geschützt — sein Schutzgrund war der Admin-Bezug, der entfallen ist.

## Byte-/Verhaltens-Vertrag

- `POST /api/admin/tenants`: 201 + `{id, slug, name, status}`; 400 bei ungültigem
  Slug; 409 bei Duplikat.
- `DELETE /api/admin/tenants/{id}`: 204 bei leerem Mandanten; **409** solange
  Zugänge existieren; 404 bei unbekanntem Mandanten.

## Qualitäts-Gates (Backend-Commit)

- `go test -p 1 ./...` gegen real-PG (`scripts/pg-test.sh`) ✅, `go vet`/`gofmt` ✅.
- Tests:
  - `pkg/store/store_integration_test.go::TestIntegrationTenantDeleteCascades`
    (real-PG: Cascade auf User/Credential/Abo, Feed überlebt, Re-Delete →
    `ErrNotFound`).
  - `pkg/adminapi/adminapi_users_test.go`: `TestCreateTenant`,
    `…DefaultsNameToSlug`, `…Validation`, `…DuplicateSlug`,
    `TestDeleteTenantEmptySucceeds`, `…WithAccountsRefused`, `…UnknownIs404`,
    `TestTenantLifecycleRoutesForbidNonAdmin`.

## Rückverfolgbarkeit

- **Anforderung:** FR-ADMIN-007 (`docs/requirements/README.md`).
- **Design:** ADR 0011 (`docs/decisions/0011-zero-touch-onboarding.md`), Guard B.
- **Vorgänger:** ONB-3 (`ONB-3_Platform_Admins.md`).
- **Folgepakete:** ONB-5 (Feed-CRUD + Live-Join), ONB-6 (OpenAIP/Mandant).
