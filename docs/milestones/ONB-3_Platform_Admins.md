# ONB-3 — Strikte Admin/Nutzer-Trennung + Plattform-Admin-Verwaltung

> Teil des Zero-Touch-Onboarding-Epics (ADR 0011). Dieses Paket trennt die Welten
> von **Plattform-Admins** und **Mandanten-Nutzern** sauber und gibt dem Betreiber
> eine dedizierte Oberfläche, um weitere Admins ohne Terminal zu verwalten.
>
> **Lieferung in zwei Commits:** (1) Backend (dieses Dokument beschreibt es),
> (2) Frontend (Vue-Komponente „Plattform-Administratoren" + Store + Navigation).

## Fachlicher Hintergrund

Bisher wurden Plattform-Admins ausschließlich über `wayfinder bootstrap` (CLI)
angelegt, und jeder Admin „wohnte" — als reines Seed-Artefakt — unter einem
Mandanten. Das vermischte zwei grundverschiedene Rollen: den **globalen
Plattform-Betreiber** und den **Lotsen-Nutzer eines Kunden**. Für den
Zero-Touch-Betrieb fehlte zudem die Möglichkeit, **weitere Admins** (Vertretung,
Vier-Augen-Prinzip, Ausscheiden des Erst-Admins) über die Oberfläche anzulegen.

ONB-3 macht die Trennung zu einer **harten Invariante** und zieht die zentrale
Sicherheits-Regel des Epics — den **„letzter aktiver Admin"-Guard** — auch auf
die Fremd-Verwaltung von Admins.

## Was umgesetzt wurde (Backend)

### 1. Schema — `tenant_id` nullable + CHECK (Migration 00007)

`users.tenant_id` wird nullable; bestehende Admins werden von ihrem Mandanten
gelöst (`UPDATE … WHERE role='admin'`); ein **CHECK-Constraint** erzwingt die
Invariante:

```sql
CHECK ((role = 'admin' AND tenant_id IS NULL)
    OR (role = 'user'  AND tenant_id IS NOT NULL))
```

Fail-closed an der Datenbank — weder Anwendung noch manuelle Schreibzugriffe
können einen Halb-Zustand erzeugen (Admin **mit** Mandant, Nutzer **ohne**).

### 2. Store

- `scanUser` liest `tenant_id` NULL-sicher → `TenantID 0` (in-Prozess-Sentinel
  „kein Mandant").
- **Getrennte Konstruktoren:** `Create(tenantID, subject, email)` legt **immer**
  einen Nutzer an (Rolle `user`); `CreateAdmin(subject, email)` legt einen
  tenant-losen Admin an (Rolle `admin`, `tenant_id NULL`). Die Rolle ist nicht
  mehr frei wählbar — die Trennung ist im Konstruktor kodiert.
- `ListAdmins()` liefert alle Plattform-Admins (global, nicht pro Mandant).

### 3. Login — keine Mandanten-Pause-Kaskade für Admins

`pkg/tenant/login.go` überspringt die Tenant-Pause-Prüfung, wenn `TenantID == 0`
(Admin). Andernfalls würde `GetByID(0)` fehlschlagen und — fail-closed — **jeden
Admin aussperren**. Nur der eigene Konto-Status (`active`/`paused`) gated einen
Admin.

### 4. Bootstrap + Auto-Seed

`runBootstrap` verzweigt nach Rolle (`provisionAccount`): ein **Admin** wird
global über `CreateAdmin` angelegt (das `-tenant`-Flag ist irrelevant), ein
**Nutzer** unter einem get-or-create-Mandanten. Das Überqueren der Grenze für ein
bestehendes Subject (Admin ↔ Nutzer) ist ein **Konflikt**, kein stilles Umhängen.

Der Auto-Seed (`autoSeedDefaultAdmin`) legt einen **tenant-losen** Default-Admin
an **und** — als Komfort — den Default-Mandanten `default` (Zuhause für die ersten
Lotsen-Zugänge). Idempotenz unverändert (`CountActiveAdmins == 0`).

### 5. API — dedizierte `/api/admin/admins`-Fläche

| Methode & Pfad | Wirkung |
|---|---|
| `GET /api/admin/admins` | alle Plattform-Admins |
| `POST /api/admin/admins` | Admin anlegen (`{subject, email?, password?}`; Passwort min. 8; doppelt → 409) |
| `PATCH /api/admin/admins/{id}` | pausieren/reaktivieren (`{status}`); Pausieren des letzten aktiven Admins → **409** |
| `DELETE /api/admin/admins/{id}` | löschen; letzter aktiver Admin → **409** |
| `PUT /api/admin/admins/{id}/password` | Passwort setzen/zurücksetzen (min. 8) |

- Alle hinter `requireAdmin`. `adminByID` liefert für die ID eines
  **Mandanten-Nutzers** ein **404** — Nutzer sind auf dieser Fläche nicht
  erreichbar.
- Der **„letzter aktiver Admin"-Guard** (`wouldOrphanAdmins` → `CountActiveAdmins`)
  schützt Pausieren **und** Löschen. Reaktivieren ist nie betroffen; das Löschen
  eines bereits **pausierten** Admins ist erlaubt (er ist nicht aktiv).
- Die per-Mandant-Route `POST /api/admin/tenants/{id}/users` lehnt ein
  mitgeschicktes `role:"admin"` mit **400** ab und verweist auf `/api/admin/admins`.

### Betriebs-Konsequenz

Ein Admin hat `TenantID 0` → auf der ASD-Karte **kein Feed-Scope** (leeres Bild).
Das ist gewollt: Admins betrachten die Lage eines Mandanten über „Als Mandant
ansehen" (WF2-34, read-only Impersonation), nicht über eine eigene Mandanten-Bindung.

## Byte-/Verhaltens-Vertrag

- Admin: `role='admin'`, `tenant_id IS NULL`. Nutzer: `role='user'`,
  `tenant_id` gesetzt. DB-CHECK erzwingt beides.
- Last-Admin-Guard: HTTP 409 bei Pausieren/Löschen des letzten aktiven Admins
  (`/api/admin/admins/{id}` **und** `DELETE /api/admin/me`).
- `bootstrap -role admin` benötigt **kein** `-tenant`.

## Qualitäts-Gates (Backend-Commit)

- `go test -p 1 ./...` gegen real-PG (`scripts/pg-test.sh`) ✅, `go vet`/`gofmt` ✅.
- Tests:
  - `pkg/store/store_integration_test.go::TestIntegrationAdminTenantSeparation`
    (tenant-loser Admin, `ListAdmins`, CHECK lehnt beide Halb-Zustände ab).
  - `cmd/wayfinder/bootstrap_integration_test.go::TestIntegrationBootstrap`
    (Admin-Welt tenant-los, Nutzer-Welt mit Mandant, Welten-Konflikt).
  - `pkg/adminapi/adminapi_admins_test.go` (CRUD, Last-Admin-Guard 409,
    Nicht-Admin-ID → 404, requireAdmin → 403).
  - `pkg/adminapi/adminapi_users_test.go::TestCreateUserRejectsAdminRole`.
  - `pkg/tenant/login_test.go` (tenant-loser Admin nicht ausgesperrt).

## Rückverfolgbarkeit

- **Anforderung:** FR-ADMIN-006 (`docs/requirements/README.md`).
- **Design:** ADR 0011 (`docs/decisions/0011-zero-touch-onboarding.md`).
- **Vorgänger:** ONB-1 (`ONB-1_Zero_Touch_Auto_Admin.md`), ONB-2
  (`ONB-2_Self_Management_UI.md`).
- **Folgepakete:** ONB-4 (Mandanten-CRUD), ONB-5 (Feed-CRUD + Live-Join),
  ONB-6 (OpenAIP/Mandant).
