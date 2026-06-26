# ONB-1 — Zero-Touch-Onboarding: Auto-Admin beim Boot + Pflichtwechsel

> Teil des Zero-Touch-Onboarding-Epics (ADR 0011). Dieses Paket macht eine frische
> Wayfinder-Instanz **ohne Terminal-Schritt** benutzbar: Container starten →
> einloggen → Passwort setzen. Folgepakete ONB-2…6 ergänzen Selbstverwaltungs-UI,
> Admin-/Mandanten-/Feed-Verwaltung und OpenAIP pro Mandant.

## Fachlicher Hintergrund

Bisher verlangte eine frische Plattform-Instanz drei manuelle Terminal-Schritte,
bevor sich überhaupt jemand einloggen konnte: `wayfinder bootstrap` (erster
Mandant + Admin), das Setzen eines Session-Schlüssels und das Aufnehmen eines
Feeds. Der Betreiber-Wunsch ist klar: **„nur die Container starten, alles andere
aus der Oberfläche."** ONB-1 räumt den ersten und größten Reibungspunkt — den
Login — weg.

Die Sicherheits-Abwägung steht im Zentrum (das ASD ist sicherheitsrelevant): Ein
Login mit bekanntem Default-Passwort ist nur vertretbar, wenn der **erste und
einzig mögliche** Schritt nach dem Login der **erzwungene Passwortwechsel** ist.
Genau das setzt das Pflichtwechsel-Gate fail-closed durch.

## Was umgesetzt wurde

### 1. Schema — `must_change_password` (Migration 00006)

Additive Spalte `users.must_change_password BOOLEAN NOT NULL DEFAULT false`. Nicht
breaking: bestehende Konten bleiben unberührt (Flag `false`).

### 2. Store

- `store.User` trägt `MustChangePassword`; `userColumns`/`scanUser` lesen die
  Spalte (also auch der Middleware-Lookup `GetBySubject`).
- `UserRepo.SetMustChangePassword(id, bool)` — setzt/löscht das Flag.
- `UserRepo.CountActiveAdmins()` — zählt `role='admin' AND status='active'`;
  Basis für den Seed-Guard **und** den „letzter aktiver Admin"-Guard.

### 3. Identity trägt das Flag

`tenant.Identity.MustChangePassword` wird in `tenant.Middleware` aus dem ohnehin
aufgelösten User gefüllt — das Gate braucht **keinen** zweiten DB-Lookup.

### 4. Boot-Auto-Seed (`cmd/wayfinder/seed.go`)

`autoSeedDefaultAdmin` läuft in `setupTenancy`, **nur** in `builtin`-Modus mit DB.
Wenn `CountActiveAdmins == 0`: Standard-Mandant `default` + Standard-Admin
`admin`/`admin` über `runBootstrap` (dieselbe idempotente Provisionierung wie die
CLI), danach `must_change_password=true`. Idempotent und fail-safe: existiert
bereits ein aktiver Admin oder hat der Betreiber das Passwort rotiert, passiert
nichts.

### 5. Session-Key-Komfort

`builtin` ohne `WAYFINDER_SESSION_KEY` → Wayfinder erzeugt beim Start einen
flüchtigen 32-Byte-Zufalls-Schlüssel und **warnt**. Damit startet `docker compose
up` ohne Pflicht-Secret; ein fester Schlüssel bleibt die Produktions-Empfehlung
(Sessions überstehen sonst keinen Neustart, nicht multi-Replica-fähig).

### 6. Pflichtwechsel-Gate + Selbstverwaltung (`pkg/adminapi`)

- **Gate** in `Handler.ServeHTTP`: Trägt die Identity das Flag, sind nur drei
  Routen erreichbar (`passwordChangeAllowlist`): `GET /api/admin/whoami`,
  `GET /api/admin/me`, `PUT /api/admin/me/password`. Alles andere →
  **403 `password_change_required`** (stabiler Marker fürs SPA).
- **`/api/admin/me`** (rollen-unabhängig, kein `requireAdmin`):
  - `GET` — eigenes Konto inkl. Flag.
  - `PUT …/password` — `{current_password, new_password}`; prüft das aktuelle
    Passwort (falsch → 401), neu min. 8 Zeichen, setzt Flag zurück. Die einzige
    Aktion, die im Pflichtwechsel-Zustand das übrige Surface freischaltet.
  - `DELETE` — eigenes Konto; **„letzter aktiver Admin"-Guard** → 409 (keine
    Selbst-Aussperrung). Nach dem Löschen ist das Cookie stale → nächster Request
    fail-closed 401 (Sofort-Revoke ist AP7).
- `whoami` trägt jetzt zusätzlich `must_change_password`.

### 7. Frontend — Pflichtwechsel-Maske

`AdminView.vue` zeigt bei `admin.mustChangePassword` (aus whoami) eine
Passwort-ändern-Maske **statt** des Dashboards. Der Store-Action
`changeOwnPassword(current, new)` ruft `PUT /api/admin/me/password` und lädt bei
Erfolg die Identity neu (Flag kippt auf `false`, Dashboard erscheint).

### 8. Deployment — `docker-compose.onboarding.yml`

Fertiger Multi-Tenant-Stack (PostgreSQL + Wayfinder builtin, host-networking für
Multicast). Ein Befehl: `docker compose -f docker-compose.onboarding.yml up
--build`.

## Byte-/Verhaltens-Vertrag

- Default-Login: Benutzer `admin`, Passwort `admin`, **nur bis zum ersten
  Wechsel** gültig (jede andere Aktion 403).
- Auto-Seed nur bei **null aktiven Admins** — restart-sicher.
- Gate-Marker: HTTP 403, Body `{"error":"password_change_required"}`.

## Qualitäts-Gates

- `go test ./...` (mit `-p 1` gegen Postgres) grün; `go vet`/`gofmt` ohne Befunde.
- `npm run test` (137 Frontend-Tests) grün; `npm run build` aktualisiert `dist/`.
- Tests:
  - `pkg/store/users_onboarding_integration_test.go` (real-PG: Flag-Default/Toggle,
    `CountActiveAdmins`).
  - `cmd/wayfinder/seed_integration_test.go` (real-PG: Erst-Seed, Re-Seed no-op,
    rotiertes Passwort/Flag nicht überschrieben).
  - `pkg/adminapi/adminapi_me_test.go` (Gate blockt/erlaubt, `/me`-Handler,
    Last-Admin-Guard).
  - `frontend/src/stores/__tests__/admin.test.js` (ONB-1-Block).

## Rückverfolgbarkeit

- **Anforderung:** FR-ADMIN-005 (`docs/requirements/README.md`).
- **Design:** ADR 0011 (`docs/decisions/0011-zero-touch-onboarding.md`).
- **Folgepakete:** ONB-2 (Selbstverwaltungs-UI-Ausbau), ONB-3 (Admins verwalten),
  ONB-4 (Mandanten-CRUD), ONB-5 (Feed-CRUD + Live-Join), ONB-6 (OpenAIP/Mandant).
