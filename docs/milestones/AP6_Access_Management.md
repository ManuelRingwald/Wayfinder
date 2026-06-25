# AP6 — Zugangs-Verwaltung (Access Management)

> **Programm:** ADR 0009 (Admin-Bereich-Neuschnitt) · **Paket:** AP6 ·
> **Einstufung:** 🔒 S3 · Opus 4.8 (Login-Pfad sicherheitsrelevant) ·
> **Grundlage:** AP1 (Rollen `admin`/`user`), WF2-10 (Repos), WF2-12 (Login),
> WF2-31 (Admin-API) · **Issue:** #63.

## Warum (fachlich)

Bisher entstanden Login-Konten nur per `wayfinder bootstrap` oder direkt per SQL.
Für den realen Betrieb braucht der Plattform-Betreiber (`admin`) eine bedienbare
**Zugangs-Verwaltung pro Mandant**: Konten anlegen, Passwörter setzen, Konten
**pausieren** (z. B. bei Off-Boarding oder Zahlungsstopp) und reaktivieren,
löschen — und einen **ganzen Mandanten** auf einmal sperren. Ein pausierter
Zugang muss sofort von der Anmeldung ausgesperrt sein (fail-closed), seine
Konfiguration aber behalten, damit Reaktivieren verlustfrei ist.

## Was (technisch)

### Schema — `00005_user_status.sql`
`users.status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused'))`.
Nicht-breaking (Default greift für Bestandszeilen). Mandanten-Pause nutzt das in
`00001_init` bereits vorhandene `tenants.status`.

### Store
Ein getypter, geschlossener `Status` (`StatusActive`/`StatusPaused`) mit
`Valid()`-Guard, **geteilt** von `User` und `Tenant` (spiegelt das `Role`-Idiom).
Neu: `UserRepo.SetStatus`/`Delete` (Credentials und per-User-View-Overrides fallen
über `ON DELETE CASCADE`), `TenantRepo.SetStatus`. `SetStatus`/`Delete` liefern
`ErrNotFound` bei fehlender Zeile (→ 404).

### Login-Enforcement (`pkg/tenant/login.go`, fail-closed)
Der Login-Handler bekommt einen `TenantLookup`. Nach dem Auflösen des Nutzers
gilt: ein **pausierter Zugang** — **oder** ein Zugang unter einem **pausierten
Mandanten** — wird abgewiesen, auch bei korrektem Passwort. Es kommt **dasselbe
generische 401** wie bei falschem Passwort zurück (keine paused/active-
Enumeration), und `auth.VerifyPassword` läuft weiterhin immer (Timing-uniform).
Ein Tenant-Lookup-Fehler gilt **fail-closed** als suspendiert. Tenant-Pause ist
damit **verlustfrei** (Reaktivieren des Mandanten stellt alle nicht-einzeln
pausierten Zugänge wieder her — es werden keine User-Zeilen umgeschrieben).

### Admin-API (`pkg/adminapi/adminapi_users.go`, cross-tenant, `requireAdmin`)

| Methode | Pfad | Aktion |
|---|---|---|
| `GET`  | `/api/admin/tenants/{id}/users` | Zugänge auflisten |
| `POST` | `/api/admin/tenants/{id}/users` | Anlegen (`{subject, email?, password?}`) |
| `PATCH`| `/api/admin/tenants/{id}/users/{uid}` | Status `active`/`paused` |
| `DELETE`| `/api/admin/tenants/{id}/users/{uid}` | Löschen |
| `PUT`  | `/api/admin/tenants/{id}/users/{uid}/password` | Passwort setzen/zurücksetzen |
| `PATCH`| `/api/admin/tenants/{id}` | Mandant pausieren/reaktivieren |

Invarianten: neue Konten sind **immer** Rolle `user` (Plattform-Admins entstehen
nur über `bootstrap`); Passwort **min. 8 Zeichen**; **doppelter Subject → 409**
(Pre-Check via `GetBySubject`, die DB-`UNIQUE` bleibt Backstop gegen Races); eine
**User-ID aus einem fremden Mandanten → 404** (`userInTenant`-Guard hält die
Ressourcen-Hierarchie ehrlich — kein Mutieren über die falsche Mandanten-URL).

### Frontend
`AdminUsers.vue` (admin-only Tab „Zugänge"): Mandanten-Wähler, Zugangs-Tabelle mit
Status-Chip, Dialoge für Anlegen/Passwort/Löschen und Inline-Pausieren sowie
Mandant-Pause. Store-Actions in `stores/admin.js`
(`loadTenantUsers`/`createUser`/`setUserStatus`/`deleteUser`/`setUserPassword`/
`setTenantStatus`). Das UI-Gating ist kosmetisch — der Server erzwingt jede Grenze.

## Abgrenzung
Die **Sofort-Wirkung auf bereits laufende Sessions** ist **AP7** (Session-
Registry, DB-gestützt). AP6 sperrt nur **neue** Anmeldungen; eine laufende
Session läuft bis zum Cookie-Ablauf weiter. Die Mandanten-Detailansicht, in die
die Zugangs-Liste später mandantenzentriert eingebettet wird, ist **AP3**
(Dashboard); bis dahin lebt sie als eigener Tab mit Mandanten-Wähler.

## Tests / Gates
- **Store (real-PG):** `TestIntegrationUserStatusLifecycle` (Default-active,
  Pause/Reaktivieren, Tenant-Pause, Delete kaskadiert Credential, `ErrNotFound`),
  `TestStatusValid`.
- **Login:** `TestLoginEnforcesStatus` (paused-Account/paused-Tenant/Lookup-
  Fehler → 401; nil-Tenants überspringt die Kaskade).
- **API:** `adminapi_users_test.go` — CRUD, Validierung (400), 404/409,
  `TestUserCrossTenantMismatch`, **`TestAccessRoutesForbidNonAdmin`** (user → 403
  auf **jeder** Route, keine Mutation).
- **Frontend:** `admin.test.js` AP6-Actions (Pfade/Bodies/Notices, 409-Pfad).
- `go test ./...` + `scripts/pg-test.sh` (real-PG) + `vitest` (108) + `npm build`
  grün; `go vet`/`gofmt` ohne Befund.
