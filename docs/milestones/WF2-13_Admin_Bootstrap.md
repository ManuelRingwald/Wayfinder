# WF2-13 — Admin-Bootstrap + `/admin`-Rollen-Gate

> **Stufe:** 1 (Identität & Mandanten-Grundgerüst) — **schließt Stufe 1 ab** ·
> **Paket:** WF2-13 · **Einstufung:** S2–S3 · Sonnet 4.6 (Sicherheits-Touch durch
> Credential-Anlage → Opus-Review-Blick) · **Grundlage:** ADR 0005/0006, baut auf
> WF2-10/11/12.

## Warum (fachlich)

Nach WF2-12 ist die Multi-Tenancy verdrahtet — aber ein **frisch aufgesetztes
Deployment hat keinen einzigen Nutzer**, also auch keinen Weg zum ersten Login:
- **proxy-Modus** braucht eine `users`-Zeile, damit das OIDC-`subject` auf einen
  Mandanten gemappt werden kann (sonst `401`);
- **builtin-Modus** braucht zusätzlich ein Passwort (das WF2-12.3 prüft, aber
  niemand setzte).

WF2-13 schließt diese Lücke: ein **Bootstrap-Subcommand** legt den ersten
Mandanten + Admin-Nutzer (+ optional dessen builtin-Passwort) an. Damit ist der in
WF2-12.3 verdrahtete builtin-Login auch *bedienbar* und **Stufe 1 abgeschlossen**.
Dazu kommt das **`/admin`-Rollen-Gate** als wiederverwendbare Autorisierungs-
Schranke, hinter der die Admin-API/-UI (WF2-31/32) baut.

## Was (technisch)

**Bootstrap-Subcommand** (`cmd/wayfinder/bootstrap.go`):
- `wayfinder bootstrap -tenant … -subject … [-tenant-name …] [-email …]
  [-role operator|tenant_admin|super_admin] [-password …]`. Mit **keinem**
  Subcommand startet wie bisher der Server (Dispatch in `main()` über
  `os.Args[1] == "bootstrap"`).
- `runBootstrap(ctx, pool, params, out)` — der testbare Kern: **idempotentes**
  Get-or-Create für Tenant (per Slug) und User (per Subject) + optionales
  `CredentialRepo.Set` (Upsert, (re)setzt das Passwort). Mehrfach ausführbar ohne
  Duplikate. Ein Subject, das bereits in einem **anderen** Mandanten existiert,
  ist ein **Konflikt** (kein stilles Re-Homing → Fehler).
- `bootstrapCommand(args, out)` — dünne Hülle: Flags parsen, `WAYFINDER_DB_URL`
  lesen, `store.Open` + `store.Migrate` (Bootstrap kann das Allererste gegen eine
  neue DB sein), dann `runBootstrap`.
- **Passwort-Hygiene:** bevorzugt `WAYFINDER_BOOTSTRAP_PASSWORD` (ein `-password`-
  Flag ist in der Prozessliste sichtbar); im Code dokumentiert. Passwort wird als
  argon2id-Hash (`auth.HashPassword`) gespeichert, nie im Klartext.

**`/admin`-Rollen-Gate** (`pkg/tenant/authz.go`):
- `RequireRole(allowed …store.Role)` — Middleware, die nur durchlässt, wenn die
  **von `Middleware` gesetzte Identity** eine der erlaubten Rollen trägt; sonst
  `403`, `next` wird nie erreicht. **fail-closed:** ohne Identity im Context
  (Gate ohne vorgelagerte `Middleware`) → ebenfalls `403`.
- Verdrahtung in `main.go`: bei aktiver Multi-Tenancy wird `/admin` als
  `tenantMW(RequireRole(tenant_admin, super_admin)(whoami))` gemountet. Der
  `adminWhoamiHandler` liefert die eigene Identity als JSON — eine minimale,
  ehrliche Zugriffsprüfung; die echte Admin-Oberfläche folgt WF2-31/32.

**Kein Schema-Change** — WF2-13 nutzt ausschließlich die in WF2-10/12.3
vorhandenen Tabellen/Repos.

## Tests

- **DB-frei:**
  - `cmd/wayfinder/bootstrap_test.go` — `bootstrapParams.validate` (Pflichtfelder,
    Rollen-Gültigkeit).
  - `pkg/tenant/authz_test.go` — `RequireRole`: `tenant_admin`/`super_admin`
    erlaubt (200, `next` erreicht), `operator`/leere Rolle/**keine Identität** →
    `403` (`next` nie erreicht).
- **Real gegen PostgreSQL 16** (`scripts/pg-test.sh`):
  - `cmd/wayfinder/bootstrap_integration_test.go::TestIntegrationBootstrap` —
    Erstlauf legt Tenant+User+Credential an (Passwort verifiziert via
    `auth.VerifyPassword`); Zweitlauf ist **idempotent** (genau 1 Tenant/1 User)
    und **aktualisiert** das Passwort; ein Subject in einem anderen Mandanten →
    **Konflikt-Fehler**.
- **End-to-End-Rauchtest** des gebauten Binaries gegen eine Wegwerf-DB: `bootstrap`
  legt an, ein Zweitlauf ist idempotent, ein fehlendes Pflicht-Flag bricht mit
  Exit-Code ≠ 0 ab; `psql` bestätigt genau eine Zeile.

Standard-`go test ./...` bleibt grün ohne DB (Integration skippt). Gates grün
(`go build/vet/test`, `gofmt`, `scripts/pg-test.sh`); `go 1.25` unverändert.

Doku: INSTALLATION §7 (Bootstrap-Aufruf, Flag-Tabelle, `WAYFINDER_BOOTSTRAP_
PASSWORD`, `/admin`-Hinweis) + TECHNICAL §6 (Subcommand + Gate) + Register
FR-TEN-001 (Bootstrap-Impl/Tests). Milestone (diese Datei).

## Abgrenzung / Nächstes

- **Bewusst nicht enthalten:** Self-Service-Registrierung, Passwort-Rotation/-
  Policy, Mehrfach-Admin-Verwaltung, eine echte Admin-API/-UI — das ist
  **WF2-31/32** (hinter dem hier gebauten Gate). `/admin` liefert vorerst nur
  „whoami".
- **Damit ist Stufe 1 (Identität & Mandanten-Grundgerüst) komplett:** Persistenz
  (WF2-10), AuthN in 3 Modi (WF2-11), Tenant-Context + builtin-Login (WF2-12),
  Bootstrap + Admin-Gate (WF2-13).
- **Nächster Schritt: Stufe 2 — der sicherheitskritische Kern.** **WF2-20**
  (Feed-Registry & Multi-Feed-Receiver: 1→N Feeds, `feed_id` pro Track), dann
  **WF2-21** (scoped Fan-out: `broadcast()` → Prädikat feed∩AOI∩FL∩Kategorie) und
  **WF2-22** (Isolations-Testsuite mit Pflicht-Negativtests „A sieht nie B").
