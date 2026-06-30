# ADR 0014 — Multi-Tenant als einziger Betriebsmodus (Single-Tenant entfernt)

- **Status:** akzeptiert
- **Datum:** 2026-06-30
- **Schnittstellen-relevant:** nein (kein Eingriff in den CAT062-Draht-Vertrag).
  Betrifft den **Betriebsmodus**, den Browser-Rand/Identitäts-Pfad und das
  Deployment.
- **Bezug:** schärft **ADR 0005** (Multi-Mandanten-Pivot) und **ADR 0006**
  (Identitäts-Persistenz) — der dort als degenerierter Rückfall geführte
  **Single-Tenant-Modus** wird **entfernt**, nicht nur abgeraten. Setzt das
  Browser-Rand-Sicherheitsmuster aus **ADR 0003** konsequent fort (Auth ist
  *immer* an). Der Orchestrator (**ADR 0012**) ist davon nicht betroffen
  (mandanten-agnostisch).

## Kontext

Wayfinder kennt heute zwei Betriebsformen:

1. **Multi-Tenant** — `WAYFINDER_DB_URL` gesetzt, Auth-Modus `builtin` oder
   `proxy`; Mandanten/Nutzer/Feeds/Abos/Sichten in der DB, pro-Mandant gescopter
   WebSocket-Strom, Admin-API. Das ist der produktive Pfad (ADR 0005/0006/0011/0012).
2. **Single-Tenant** — ein **degenerierter Rückfall** aus drei gekoppelten Stücken:
   - **`WAYFINDER_AUTH_MODE=none`** (`NoneAuthenticator`: fester Pseudo-Nutzer, kein Login) — und es ist heute der **Default**, wenn die Variable nicht gesetzt ist.
   - **No-DB-Fallback** (`WAYFINDER_DB_URL` leer ⇒ `setupTenancy` liefert keine
     Middleware/Pool; keine Mandanten, kein Login, keine Admin-API).
   - **nil-Scope im Broadcaster** (`scope == nil` ⇒ ein Client sieht **alle**
     Feeds, keine Mandanten-Filterung).

Der Single-Tenant-Modus war als „schneller ASD ohne Datenbank" gedacht. In einem
**produktiv-mandantenfähigen** Produkt ist er aber:

- **ein Sicherheitsrisiko:** unset `AUTH_MODE` ⇒ `none` ⇒ ein **unauthentifiziertes,
  ungescoptes ASD** entsteht *versehentlich* (Fail-open). Genau das soll der
  Browser-Rand (ADR 0003) nie zulassen.
- **eine dauerhafte Code-Last:** ~15 `if dbPool != nil` / `if tenantMW != nil`
  -Verzweigungen in `cmd/wayfinder/main.go` und mehrere `scope == nil`-Sonderfälle
  in `pkg/broadcast` — zwei Codepfade, die jede Mandanten-Änderung doppelt bedenken muss.
- **inkonsistent zur Produktrichtung:** alles, was wir bauen (Mandanten-Isolation,
  Sichten/AOI, Feature-Entitlements, OpenAIP-pro-Mandant, Orchestrierung), gilt
  multi-tenant. Der Single-Tenant-Pfad testet das **nicht** mit.

**Schlüssel-Erkenntnis:** Der **Orchestrator ist bereits mandanten-agnostisch** —
er spawnt Firefly **pro Feed** (getrieben von Abos), nicht pro Mandant. Die
`docker-compose.orchestrated.yml` nutzt `none` **nur zur Harness-Vereinfachung**,
nicht aus Notwendigkeit. Multi-Tenant + Orchestrator + Live-Tracks lassen sich
also in **einem** Stack vereinen.

## Entscheidung

**Multi-Tenant ist der einzige unterstützte Betriebsmodus.** Single-Tenant wird
vollständig entfernt:

1. **Auth-Modi:** `WAYFINDER_AUTH_MODE ∈ {builtin, proxy}`. Der Modus **`none`**
   (samt `NoneAuthenticator`, `WAYFINDER_NONE_SUBJECT`) **entfällt**.
2. **Default-Modus:** ist `WAYFINDER_AUTH_MODE` **nicht** gesetzt, gilt **`builtin`**
   (zero-touch, Auto-Seed `admin`/`admin` mit erzwungenem Passwortwechsel, ADR 0011).
   Kein impliziter „none"-Default mehr.
3. **Datenbank ist Pflicht:** `WAYFINDER_DB_URL` **muss** gesetzt und erreichbar
   sein; fehlt sie, **bricht der Start mit klarer Meldung ab** (kein No-DB-Fallback).
4. **Immer gescopt:** der WebSocket-Pfad bekommt **immer** Mandanten-Middleware und
   einen gültigen `*Scope`; der `scope == nil`-Sonderfall im Broadcaster entfällt.
   Tenant-Metriken sind damit immer pro-Mandant.
5. **Deployment:** der Single-Tenant-Quick-Start `docker-compose.yml` (keine DB,
   kein Auth) **entfällt**. Es gibt **einen** Multi-Tenant-Stack (Postgres +
   Wayfinder `builtin`); die Orchestrator-Variante stellt von `none` auf `builtin`
   um und liefert damit *Multitenant + Auto-Spawn + Live-Tracks* aus einem Guss.
6. **`proxy`-Modus bleibt** unverändert der empfohlene **Produktiv**-Pfad
   (OIDC/oauth2-proxy am Ingress; `builtin` ist der Standalone-/Onboarding-Pfad).

## Konsequenzen

- **Positiv:** Auth ist *immer* an (kein versehentlich offenes ASD — Sicherheitsgewinn,
  ADR 0003); ein einziger Codepfad (Fallback-Zweige + nil-Scope weg ⇒ schlanker,
  weniger Fehlerfläche); **alles** läuft und testet multi-tenant; ein kohärenter
  Deployment-Stack. Frontend (auth-agnostisch) und Orchestrator (feed-getrieben)
  brauchen **keine** Änderung.
- **Negativ / Grenzen:** ein „ASD ohne Datenbank in 10 Sekunden" gibt es nicht mehr —
  jeder Start braucht Postgres. Das ist gewollt (Produktreife vor Bequemlichkeit);
  der Onboarding-Stack hält den Aufwand mit einem Compose-Befehl + Auto-Seed minimal.
- **Migration für Betreiber:** wer bisher ohne DB/`none` fuhr, setzt künftig
  `WAYFINDER_DB_URL` (Postgres) und meldet sich an (`builtin` Default oder `proxy`).

## Umsetzungs-Häppchen (je eigener PR, Charta §3)

- **A** *(dieser ADR)* — Entscheidung + Charta-Prinzip. **Kein Code.**
- **B — Code:** `none`-Modus, No-DB-Fallback und nil-Scope entfernen; DB **+** Auth
  Pflicht; unset `AUTH_MODE` → `builtin`-Default; `setupTenancy`/`main.go`-
  Verzweigungen und `broadcast.go`-Sonderfälle vereinfachen; `none`/No-DB-Tests
  löschen/umschreiben. `go test`/`vet` grün.
- **C — Deployment:** **ein** Multi-Tenant-Stack; `orchestrated.yml` → `builtin`
  (Auto-Seed), Single-Tenant-`docker-compose.yml` löschen; `DOCKER.md` aufräumen.
- **D — Doku:** `INSTALLATION.md` (Single-Tenant-„Teil 4" raus), `TECHNICAL.md`
  (Env-Tabelle), Anforderungs-Register (NFR-SEC-004), `docs/E2E-ABNAHME.md` rein
  multi-tenant (ersetzt PR #93).

## Alternativen erwogen

- **Single-Tenant behalten, nur abraten (Status quo):** verworfen — der Fail-open-
  Default (`none`) und der doppelte Codepfad bleiben; das Risiko/„zwei Welten"-Problem
  bleibt bestehen.
- **`none` behalten, aber DB erzwingen:** verworfen — `none` *ist* der unauthentifizierte
  Pfad; ihn zu behalten widerspricht „Auth immer an".
- **Default auf harten Startfehler statt `builtin`:** erwogen, aber verworfen zugunsten
  des **`builtin`-Defaults** (zero-touch-Onboarding, ADR 0011): eine frische Instanz
  startet ohne Extra-Env sofort multi-tenant mit Login statt mit einem Abbruch.

## Querverweise

- ADR 0005 (Multi-Mandanten-Pivot), ADR 0006 (Identitäts-Persistenz, Auth-Modi),
  ADR 0003 (Browser-Rand/Auth), ADR 0011 (Zero-Touch-Onboarding, Auto-Seed),
  ADR 0012 (Orchestrierung, mandanten-agnostisch).
- Anforderungs-Register: **NFR-SEC-004** (Auth-Modi — wird auf `{builtin, proxy}`
  nachgezogen), **FR-CFG-001** (Env-Konfiguration).
- Charta: `CLAUDE.md` §7 (neues Prinzip „Multi-Tenant ist der einzige Modus").
