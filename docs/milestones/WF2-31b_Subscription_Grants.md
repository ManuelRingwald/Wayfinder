# WF2-31b — Subscription-Grants (super_admin, cross-tenant)

> **Stufe:** 3 · **Paket:** WF2-31, zweiter Teil — **vervollständigt das
> Admin-Backend** · **Einstufung:** 🔒 S3 (+S4-Touch) · Sonnet 4.6 / Opus-Review
> (Cross-Tenant-Schreibgrenze) · **Grundlage:** ADR 0005, NFR-SEC-003; baut auf
> WF2-31 (Admin-API) + WF2-10 (Repos) + WF2-13 (`RequireRole`).

## Warum (fachlich)

Mandanten-Provisioning ohne DB-Poking ist für das SaaS-Modell geschäftskritisch.
**Welche** Feeds ein Mandant sehen darf, ist eine **Plattform-/Billing-
Entscheidung** — nicht Selbstbedienung des Mandanten. WF2-31b gibt dem
Plattform-Betreiber (super_admin) eine API, um Feed-Zugänge **mandantenübergreifend**
zu granten/entziehen. Damit ist die API-Landschaft des Admin-Backends abgeschlossen,
bevor die UI (WF2-32) als Consumer obendrauf gesetzt wird (Reihenfolge-Entscheid des
Projektverantwortlichen).

## Das Rollen-Modell (das Fundament)

| Rolle | Darf | Skopierung |
|---|---|---|
| **tenant_admin** | eigene View-Config (WF2-31), eigene Abos/Feeds **lesen** | **nur eigener** Mandant — `tenant_id` **aus der Identity** |
| **super_admin** | **alle** Mandanten listen, Abos **granten/entziehen** | **cross-tenant** — Ziel-`tenant_id` **aus dem Pfad** |

Bewusste Asymmetrie: tenant_admin-Routen nehmen den Mandanten *implizit* aus der
Identity (Isolation per Konstruktion, WF2-31); die super_admin-Routen nehmen ihn
*explizit* aus dem Pfad — und super_admin ist die **einzige** Rolle, die das darf.

## Was (technisch)

Neue, **super_admin-only** Endpunkte in `pkg/adminapi`:

| Methode + Pfad | Wirkung |
|---|---|
| `GET /api/admin/tenants` | Alle Mandanten (Provisioning-Übersicht). |
| `GET /api/admin/tenants/{tenantID}/subscriptions` | Abos eines Mandanten. |
| `POST /api/admin/tenants/{tenantID}/subscriptions` | Feed granten (Body `{"feed_id":…}`); Tenant + Feed müssen existieren; idempotent → `204`. |
| `DELETE /api/admin/tenants/{tenantID}/subscriptions/{feedID}` | Feed entziehen; idempotent → `204`. |

- **Doppel-Gate:** der äußere `RequireRole(tenant_admin, super_admin)` (main.go)
  lässt Admins rein; **`requireSuper`** (in-handler) prüft zusätzlich
  `Identity.Role == super_admin` → sonst **`403`**. Saubere Trennung: admin+ kommt
  rein, nur super_admin schreibt cross-tenant.
- **Validierung:** Pfad-`{tenantID}`/`{feedID}` via `r.PathValue` (Go-1.22-Mux,
  ungültig → `400`); Ziel-Tenant existiert (`TenantRepo.GetByID`, sonst `404`);
  Feed existiert (`FeedRepo.GetByID`, sonst `404`); Body wohlgeformt (`400`).
- Neue Store-Interfaces: `TenantStore` (List/GetByID), `SubscriptionStore` um
  `Subscribe`/`Unsubscribe` erweitert, `FeedStore` um `GetByID`. Die echten Repos
  erfüllen sie; `adminapi.New(...)` bekommt zusätzlich den `TenantRepo`.
- **Kein Schema-Change, keine neue Abhängigkeit.**

## Tests

- **DB-frei** (`adminapi_test.go`): `TestGrantSubscription`/`TestRevokeSubscription`
  (Ziel aus dem Pfad → Store mit `(5,3)` aufgerufen); `TestGrantValidation`-Tabelle
  (unbekannter Tenant/Feed → `404`, fehlende `feed_id`/kaputtes JSON/kaputte ID →
  `400`, erreicht den Store nicht); `TestListTenantsSuperAdmin`; **Cross-Tenant-
  Negativtest** `TestCrossTenantRoutesForbidTenantAdmin` — ein **tenant_admin** auf
  **jeder** Provisioning-Route → `403`, **kein** Grant/Revoke erreicht den Store.
- **Real gegen PostgreSQL 16** (`adminapi_integration_test.go`): super_admin
  **grant → `GET subscriptions` (tenant_admin) zeigt den Feed → revoke → wieder
  leer**; tenant_admin-Grant → `403`; `GET tenants` zeigt den Mandanten.

Gates grün (`go build/vet/test`, `gofmt`, `scripts/pg-test.sh`); `go 1.25`
unverändert. Doku: INSTALLATION §7 + TECHNICAL §6 (super_admin-Routen) + Register
FR-ADMIN-001.

## Abgrenzung / Nächstes

- **Damit ist WF2-31 (Admin-API) komplett:** tenant_admin-Selbstbedienung
  (View + Reads) + super_admin-Provisioning (Tenants/Grants).
- **Nicht enthalten:** Anlegen/Löschen von Tenants/Feeds/Usern über die API (heute
  via `bootstrap`/`feed`-CLI); Feature-Flags/Entitlements-Verwaltung. Folgeschritte
  nach Bedarf.
- **Nächster Schritt: WF2-32 — Admin-UI** (Vue 3 + Vuetify) als sauberer Consumer
  dieses API; danach ggf. WF2-30 (Config-Cache) bei gemessenem Bedarf und WF2-33
  (Live-Apply).
