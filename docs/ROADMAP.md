# Roadmap — Arbeitspakete & offene Punkte (Wayfinder, ausgerichtet auf **Wayfinder 2.0**)

> **Zweck:** Zentrale, lebende Übersicht über das Wayfinder-Backlog mit
> Aufwandseinschätzung (Komplexitätsstufe S1–S5, siehe `CLAUDE.md` Abschnitt 3)
> und empfohlener Modell-Zuordnung. **Stichwort „Roadmap" im Chat zeigt diese
> Liste.** Diese Datei ist die **maßgebliche Quelle für „was als Nächstes" und
> den Status**; die fachlich-technische Begründung der Wayfinder-2.0-Pakete steht
> im Konzept `docs/design/wayfinder-2.0-konzept.md` (§7 Ausbaustufen, §8
> Modell-Tabelle). Bei Status-Abweichungen gilt **diese** Datei.
>
> **Stand: 2026-06-19** — Roadmap zentral auf **Wayfinder 2.0** ausgerichtet
> (Multi-Mandanten-Plattform). Die bisherigen ASD-Kern-Pakete sind eingeordnet,
> der geteilte Firefly-Backlog mit 2.0-Bezug annotiert. Widersprüche zu
> `docs/STATUS.md` aufgelöst (nächster Schritt = WF2-00 / ADR 0005).
>
> **Geltungsbereich (wichtig):** Die Abschnitte **§0–§2** sind
> **Wayfinder-spezifisch**. Abschnitt **§3** (Firefly-/Cross-Project-Backlog)
> wird mit Fireflys Roadmap synchron gehalten. Die frühere Zusage „diese Datei
> existiert identisch in beiden Repos" gilt damit nur noch für §3 — siehe
> Pflege-Hinweis am Ende.

---

## 0. Strategische Ausrichtung: Wayfinder 2.0

Wayfinder 2.0 ist der **leitende Programmrahmen** für die nächste Phase: der
Umbau vom einprozessigen, beim-Start-konfigurierten **Single-Tenant-ASD** zur
**mandantenfähigen, zur-Laufzeit-konfigurierbaren Plattform** (Konzept:
`docs/design/wayfinder-2.0-konzept.md`, auf `main` seit PR #25).

**Zwei ratifizierte Leitentscheidungen (2026-06-19):**
1. **Mandanten-Modell = Hybrid** — Feed-Katalog (N Feeds) + Mandant abonniert
   eine Teilmenge + legt Sicht-Filter (AOI/FL/Kategorie) darüber.
2. **Kommerz-Scope = Feature-Flags ja, Stripe-Billing zurückgestellt** —
   Entitlements als Daten; Billing (WF2-51) ruht.

**Verhältnis zum bisherigen Backlog (kein Widerspruch):**
- Der **ASD-Kern** (Track-Darstellung, Karten-Layer, Data-Block, Filter) bleibt
  gültig und wird in 2.0 zur **mandanten-skopierten Sicht**. Die offenen
  ASD-Kern-Pakete (ASD-011/012/013) sind **mandanten-unabhängig** und laufen
  **parallel** weiter (§2) — sie blockieren das 2.0-Fundament nicht und werden
  nicht von ihm blockiert.
- Der **kritische Pfad** ist aber das 2.0-Fundament: **erst entscheiden (ADRs),
  dann Identität/Persistenz, dann der sicherheitskritische mandanten-isolierte
  Datenstrom** — bevor Komfort/Sensorik/Kommerz folgen.

**Stufe 0 — Fortschritt:**
- **WF2-00 — ADR 0005 „Multi-Mandanten-Pivot" ✅ erledigt** (`0005-multi-mandanten-pivot.md`):
  Pivot ratifiziert, Hybrid-Modell + Datenmodell-Skizze, Isolationsgrenze
  (server-seitige AuthZ pro Subscription, fail-closed, Pflicht-Negativtests),
  Kommerz-Scope, 12-Factor-Grenze; Register FR-TEN-001/NFR-SEC-003.
- **WF2-01 — ADR 0006 „Konfig-/Identitäts-Persistenz" ✅ erledigt** (`0006-konfig-identitaets-persistenz.md`):
  PostgreSQL + `pgx`/`sqlc` + `goose`-Migrationen, Schema-Skizze, Stateless-Split;
  Identität OIDC@Proxy primär + eingebauter Fallback (`WAYFINDER_AUTH_MODE`),
  Tenant-Kontext fail-closed; Redis zurückgestellt; Register FR-TEN-002/NFR-SEC-004.
- **WF2-02 — ADR 0007 „Cloud-Ingest & Feed-Fan-out" ✅ erledigt** (`0007-cloud-ingest-und-feed-fan-out.md`):
  Zielumgebung Public Cloud + Kubernetes; `FeedSource`-Abstraktion (direkt-Multicast
  vs. Stream), Ingest-Gateway (Multicast→Bus, Roh-Datagramm, Subject pro Feed),
  **Stream-Bus = NATS JetStream** (Core-Fan-out + optionaler Puffer; Replay bleibt
  Firefly); RabbitMQ/Kafka geprüft & verworfen (RabbitMQ > Kafka, beide < NATS für
  dieses Profil); Register FR-FEED-001/NFR-SCALE-001.

**✅ Stufe 0 (Entscheidung & Fundament) abgeschlossen** — ADR 0005/0006/0007.

**Stufe 1 — in Arbeit:**
- **WF2-10.1 ✅** — `pkg/store` (pgx-Pool, In-House-Migrationsrunner statt goose,
  DB-freie Tests). Dabei (ADR 0006 Nachtrag): goose verworfen (`modernc.org/sqlite`-
  Ballast), Go-Baseline 1.23 → 1.25.
- **WF2-10.2 ✅** — Repository-Zugriffe `tenants`/`users` (handgeschriebene pgx-
  Queries statt sqlc; `Tenant`/`User`/`Role`-Typen, `GetBySubject` = Identität→
  Mandant). **Real gegen PostgreSQL 16 getestet** via `scripts/pg-test.sh`.
- **WF2-10.3a ✅** — Repos `feeds`/`subscriptions`/`entitlements` (JSONB-`sensor_mix`;
  `subscriptions` = isolations-kritisch; Entitlements default-deny). Daten-Isolations-Test.
- **WF2-10.3b ✅** — `view_configs`-Repo (JSONB AOI/Layers, Tenant-Default +
  Nutzer-Override, `GetEffective`); **Migration `00002`** (Partial-Unique-Index,
  zeigt Schema-Evolution des Runners). Real gegen PostgreSQL 16 getestet.

**✅ WF2-10 (Persistenz-Schicht) abgeschlossen** — alle 6 Tabellen-Repos vorhanden
und real gegen Postgres verifiziert.

**✅ WF2-11 (AuthN) abgeschlossen** — `pkg/auth` liefert in allen 3 Modi ein
Subject: **11.1** (Mode, argon2id, HMAC-Session, None-/Builtin) **+ 11.2**
(`ProxyAuthenticator`, go-oidc-Validierung Issuer/Audience/Signatur/Ablauf gegen
JWKS). Neue Deps: `golang.org/x/crypto` (argon2), `github.com/coreos/go-oidc/v3`.

**✅ WF2-12 (Tenant-Context) abgeschlossen:**
- **WF2-12.1 ✅** — `pkg/auth`-Factory + neues `pkg/tenant` (`Middleware`
  subject→user→tenant **fail-closed**). DB-frei getestet.
- **WF2-12.2 ✅** — Verdrahtung in `main.go`: `setupTenancy` (DB-Open+Migrate+
  Authenticator+Middleware **nur bei gesetztem `WAYFINDER_DB_URL`**, sonst
  Single-Tenant), `/ws` mandanten-gegated, Legacy-Token abgelöst. Neue ENV-Vars
  in INSTALLATION/TECHNICAL. **Real gegen PG getestet** (`tenancy_test.go`: 401
  ohne Nutzer, Tenant aufgelöst mit Nutzer).
- **WF2-12.3 ✅** — builtin-Login: Migration `00003_credentials` + `CredentialRepo`
  (Set-Upsert/GetHash), `/api/login` (timing-gehärtet gegen Nutzer-Enumeration →
  `auth.MintSession`-HttpOnly-Cookie) + `/api/logout`; `WAYFINDER_SESSION_TTL`.
  Registriert **nur in `builtin`-Modus**. DB-freie Login-Tests + real-PG
  `CredentialRepo`-Test. *(proxy-Modus war schon voll funktionsfähig.)*

- **WF2-13 ✅** — Admin-Bootstrap: Subcommand `wayfinder bootstrap` (`cmd/wayfinder/
  bootstrap.go`, idempotentes Get-or-Create erster Tenant/Admin/Credential, kein
  Cross-Tenant-Re-Homing) + `/admin`-Rollen-Gate (`pkg/tenant/authz.go`
  `RequireRole`, fail-closed `403`). DB-freie + real-PG-Tests + E2E-Rauchtest.

**✅ Stufe 1 (Identität & Mandanten-Grundgerüst) abgeschlossen** — Persistenz
(WF2-10), AuthN in 3 Modi (WF2-11), Tenant-Context + builtin-Login (WF2-12),
Bootstrap + Admin-Gate (WF2-13).

**Stufe 2 — in Arbeit (der sicherheitskritische Kern):**
- **WF2-20.1 ✅** — `feed_id`-Durchstich: `receiver.Config.FeedID` →
  `broadcast.TrackBatch` → `TrackMessage.feed_id`. Attribution-Naht steht.
- **WF2-20.2 ✅** — Multi-Feed-Receiver: `feeds`-Katalog (DB) → N Receiver je
  `feed_id` (`resolveFeeds`/`buildReceivers`); `main()`-Reorder (DB vor Receiver),
  per-Feed-Listen-Skip, Decode-Fehler-Summe; **Feed-CLI** `wayfinder feed
  add/list`; Fallback auf den ENV-Einzelfeed bei leerem Katalog/kein-DB. Real-PG +
  E2E getestet. **→ WF2-20 komplett.**

- **WF2-21.1 ✅** — scoped Fan-out **Feed-Ebene** (der Isolations-Boundary):
  `broadcast.Scope`/`broadcastTracks` (Track nur an Clients mit
  `AllowsFeed(feed_id)`, leerer Scope = nichts, fail-closed); `ws.ScopeResolver`
  am Handshake (`403` ohne Scope); `newScopeResolver` via
  `subscriptions.ListFeedIDsByTenant`. **Pflicht-Negativtest** „A bekommt nie
  B's Feed" (`TestBroadcastFeedIsolation`). Single-Tenant bleibt all-to-all.

- **WF2-21.2 ✅** — Sicht-Filter **AOI + FL-Band** als **harte server-seitige
  Grenze** (Datensparsamkeit/Billing/kein F12-Leck), **fail-open** bei fehlendem
  Attribut: `broadcast.ViewFilter`/`Scope.filterView` (per-Client-Track-Filter);
  `resolveViewFilter` via `view_configs.GetEffective` (FL→Fuß). Lebenszyklus bleibt
  client-seitig; Klassifizierung später (Premium, WF2-40). **→ WF2-21 komplett.**

- **WF2-22 ✅** — Isolations-Testsuite (`pkg/broadcast/isolation_test.go`):
  Differential-Property `TestFilterViewMatchesOracle` (50k Iter vs. unabhängiges
  Oracle, beide Richtungen), Ende-zu-Ende `TestBroadcasterIsolationProperty` (jeder
  empfangene Track ∈ Client-Scope), `FuzzScopeFilter` (755k execs, 0 Fehler).
  **Test-only, kein Befund.** **→ sicherheitskritischer Kern (WF2-20/21/22)
  testseitig abgesichert.**

- **WF2-23.1 ✅** — Audit-Log: strukturiertes `slog`-Event beim `/ws`-Connect
  (`component=audit`/`ws_connect`, tenant/user/subject + aufgelöster Scope
  feeds+AOI/FL), 12-Factor (keine DB-Tabelle); hochkardinale Identität nur im
  Audit-Log, nie als Metrik-Label. `TestScopeResolverEmitsAudit`.

**➡️ Nächster Schritt:** **WF2-23.2 — Pro-Mandant-Metriken** (`pkg/metrics` um
Label-Unterstützung erweitern; Broadcaster zählt je Mandant verbundene Clients +
zugestellte Tracks → `…{tenant="…"}`) 🔒 **S3 · Sonnet 4.6**, nach Ankündigung &
„Go" — **schließt WF2-23 und damit Stufe 2 ab.**

---

## 1. Wayfinder 2.0 — Ausbaustufen & Arbeitspakete (zentral)

Jede Stufe ist für sich auslieferbar und de-riskt die nächste. Reihenfolge:
erst entscheiden (0), dann Identität/Persistenz neben dem Pfad (1), dann der
sicherheitskritische Stream-Umbau (2), dann Komfort (3), Sensorik (4), zuletzt
Kommerz/HA (5). 🔒 = sicherheitskritisch (mind. Opus-Review, auch bei S3).
Details & Begründung: Konzept §7/§8.

### Stufe 0 — Entscheidung & Fundament (reine ADRs, kein Produktivcode)
| AP | Inhalt | Stufe · Modell | Abh. | Status |
|----|--------|----------------|------|--------|
| **WF2-00** 🔒 | ADR 0005 „Multi-Mandanten-Pivot" (Reframe, Hybrid-Modell, Vertrauensgrenze, Zert-Haltung) | **S4 · Opus 4.8** | — | ✅ **erledigt** (ADR 0005) |
| **WF2-01** 🔒 | ADR 0006 „Konfig-/Identitäts-Persistenz" (Postgres-Schema, Migrationen, Stateless-Split) | **S4 · Opus 4.8** | WF2-00 | ✅ **erledigt** (ADR 0006) |
| **WF2-02** | ADR 0007 „Cloud-Ingest & Feed-Fan-out" (`FeedSource`, Gateway, **NATS JetStream** gewählt) | **S4 · Opus 4.8** | WF2-00 | ✅ **erledigt** (ADR 0007) |

### Stufe 1 — Identität & Mandanten-Grundgerüst (ohne Datenfluss-Änderung)
| AP | Inhalt | Stufe · Modell | Abh. | Status |
|----|--------|----------------|------|--------|
| **WF2-10** 🔒 | Persistenz-Schicht, Migrationen & Repositories (`pkg/store`, pgx) | **S3 · Sonnet 4.6** (+Opus-Review) | WF2-01 | ✅ **erledigt** (10.1–10.3b) |
| **WF2-11** 🔒 | AuthN: echtes Nutzer-/Session-Modell (`pkg/auth`; argon2id, HMAC-Session, OIDC@Proxy) | **S4 · Opus 4.8** | WF2-10 | ✅ **erledigt** (11.1 + 11.2) |
| **WF2-12** 🔒 | Tenant-Context-Middleware (jeder HTTP/WS-Request → Tenant-ID, fail-closed) | **S4 · Opus 4.8** | WF2-11 | ✅ **erledigt** (12.1 Middleware + 12.2 Verdrahtung + 12.3 builtin-Login) |
| **WF2-13** | Admin-Bootstrap (create-tenant/-user, `/admin`-Auth-Gate) | **S2–S3 · Sonnet 4.6** | WF2-12 | ✅ **erledigt** (`bootstrap`-Subcommand + `RequireRole`-Gate) |

### Stufe 2 — Mandanten-isolierter Datenstrom (sicherheitskritischer Kern)
| AP | Inhalt | Stufe · Modell | Abh. | Status |
|----|--------|----------------|------|--------|
| **WF2-20** 🔒 | Feed-Registry & Multi-Feed-Receiver (1→N Feeds; `feed_id` pro Track) | **S4 · Opus 4.8** | WF2-01, WF2-02 | ✅ **erledigt** (20.1 `feed_id`-Naht + 20.2 Multi-Feed + Feed-CLI) |
| **WF2-21** 🔒 | Subscription-Modell & scoped Fan-out (`broadcast()` → Prädikat feed∩AOI∩FL) | **S4–S5 · Opus 4.8 / Fable 5** | WF2-12, WF2-20 | ✅ **erledigt** (21.1 Feed-Isolation + 21.2 AOI/FL-Sicht-Filter) |
| **WF2-22** 🔒 | Isolations-Testsuite (Negativ-/Property-/Fuzz-Tests; A sieht nie B) | **S4 · Opus 4.8** | WF2-21 | ✅ **erledigt** (Property + Fuzz, kein Befund) |
| **WF2-23** | Pro-Mandant-Metriken & Audit-Log (`tenant`-Label, Audit-Event) | **S3 · Sonnet 4.6** | WF2-21 | 🔵 in Arbeit (23.1 Audit-Log ✅; 23.2 Pro-Tenant-Metriken offen — schließt Stufe 2 ab) |

### Stufe 3 — Dynamische Konfiguration & Admin-UI
| AP | Inhalt | Stufe · Modell | Abh. | Status |
|----|--------|----------------|------|--------|
| **WF2-30** | Config-Service (Hot-Reload aus DB, In-Proc-TTL/Redis, ohne Neustart) | **S3–S4 · Sonnet 4.6 / Opus 4.8** | WF2-10 | geplant |
| **WF2-31** 🔒 | Admin-API (REST, tenant-skopiert, server-validiert: Zentrum/Radius/FL/Abos) | **S3 · Sonnet 4.6** | WF2-30, WF2-13 | geplant |
| **WF2-32** | Admin-UI (`/admin`, Vue 3 + Vuetify, Formulare/Slider, Live-Apply) | **S3 · Sonnet 4.6** | WF2-31 | geplant |
| **WF2-33** 🔒 | Live-Apply auf der Datenebene (laufende Subscription re-skopieren, kein Reconnect) | **S4 · Opus 4.8** | WF2-21, WF2-31 | geplant |

### Stufe 4 — Sensor-/Stream-Management (innerhalb der CAT062-Realität)
| AP | Inhalt | Stufe · Modell | Abh. | Status |
|----|--------|----------------|------|--------|
| **WF2-40** | Provenienz aus dem Vertrag als Sicht-Layer (ADS-B ◆, PSR, mehr I062/080; ehrlich „track-abgeleitet") | **S3 · Sonnet 4.6** | WF2-32 | geplant |
| **WF2-41** | Feed-Sensorklassen-Katalog & Entitlements (Feed-Metadaten; Abos binden an Feeds) | **S3 · Sonnet 4.6** | WF2-20, WF2-50 | geplant |
| **WF2-42** | Cross-Project-Issue an Firefly: echte Per-Track-Provenienz (FLARM-Diskriminator) = ICD-Änderung | **S2 · Sonnet 4.6** | WF2-40 | geplant (siehe §3) |

### Stufe 5 — Monetarisierung & HA-Betrieb (optional / zuletzt)
| AP | Inhalt | Stufe · Modell | Abh. | Status |
|----|--------|----------------|------|--------|
| **WF2-50** | Feature-Entitlement-Service (`tenant.HasFeature(...)`, Flags als Daten) | **S3 · Sonnet 4.6** | WF2-10 | geplant |
| **WF2-51** 🔒 | Billing-Adapter (Stripe) als separate Plane (Webhook→Entitlement) | **S3 · Sonnet 4.6** (+🔒-Review) | WF2-50 | **ruht** (Kommerz-Entscheid §0) |
| **WF2-52** | Stateless-Härtung & horizontale Skalierung (kein node-lokaler State; LB ohne Sticky) | **S4–S5 · Opus 4.8 / Fable 5** | WF2-21 | geplant |
| **WF2-53** | Ingest-Gateway produktiv + HA (mcast→Stream, kein SPOF am Eingang) | **S4–S5 · Opus 4.8 / Fable 5** | WF2-02 | geplant |

---

## 2. ASD-Kern-Feinschliff (mandanten-unabhängig, parallel zu 2.0 möglich)

Diese Pakete waren vor der 2.0-Ausrichtung als „Phase 2" geplant. Sie bleiben
gültig, sind **rein Frontend/Sicht** und **nicht im kritischen 2.0-Pfad** — sie
können jederzeit von einem leichteren Modell parallel gezogen werden. Wichtig
für **„keine gegensätzlichen Anforderungen"** sind die 2.0-Bezüge je Paket:

| AP | Inhalt | Stufe · Modell | Status | 2.0-Bezug (Abgleich) |
|----|--------|----------------|--------|----------------------|
| **ASD-011** | Erweitertes Track-Detail-Panel (`TrackDetailCard.vue`) | **S2 · Sonnet 4.6** | offen | Reine Sicht, keine Tenancy-Wirkung — jederzeit ziehbar. |
| **ASD-012** | Range-Rings + Scale-Bar + Nord-/Track-up | **S3 · Opus 4.8** | offen | „Zentrum/Radius" wird in 2.0 **Mandanten-View-Config** (WF2-30/31). Frontend liest schon `/api/map-config` → wird transparent tenant-skopiert; **keine Nacharbeit**, wenn ASD-012 weiter aus `/api/map-config` liest statt aus einer eigenen Konstante. |
| **ASD-013** | Alarm-/Ereignis-Panel (Feed-stale, Track appeared/disappeared, Status) | **S3 · Sonnet 4.6** | offen | Speist sich aus dem WS-Strom → wird nach WF2-21 automatisch **mandanten-skopiert** (nur eigene Events). Heute single-tenant baubar, keine Rearchitektur nötig. |

**Empfehlung:** ASD-011/012/013 als opportunistische Parallel-Spur behandeln
(geringes Risiko, hoher Sicht-Nutzen). Der strategische Fokus bleibt der
2.0-Pfad (§1). Falls gewünscht, können sie auch bewusst **nach** dem
2.0-Fundament eingeplant werden — beides ist widerspruchsfrei.

---

## 3. Cross-Project- / Firefly-Backlog (geteilt, mit 2.0-Bezug)

Diese Pakete liegen **überwiegend bei Firefly** (eigene Sitzung/Repo) und werden
mit Fireflys Roadmap synchron gehalten. Annotiert ist je der Bezug zu Wayfinder
2.0, damit keine gegensätzlichen Anforderungen entstehen.

| # | Paket | Repo(s) | Inhalt | Stufe/Modell | 2.0-Bezug |
|---|-------|---------|--------|--------------|-----------|
| 4 | **Konfigurierbarer System-Referenzpunkt** | Firefly | I062/100-Referenzpunkt jenseits Demo-Ursprung Frankfurt | S3 · Sonnet 4.6 | **Relevant:** je Feed/Mandant ggf. eigener Referenzpunkt (Hybrid-Modell). Mit WF2-20 (Feed-Registry) abstimmen. |
| 5 | **Out-of-Order-Eingang (Robustheit)** | Firefly | Tracker-Härtung gegen verspätete/umsortierte Plots | S3 · Sonnet 4.6 | Neutral (Upstream-Qualität). |
| 6 | **Coverage-Werkzeug** ✅ *abgeschlossen* | Firefly + Wayfinder | Visualisierung Sensor-Abdeckung (Radar-Ringe). `pkg/coverage` + `/api/coverage/rings`; `WAYFINDER_COVERAGE_SENSOR_N_*`; Frontend-Layer-Toggle. Firefly: `SensorModel.min_range_m`/`max_range_m`. (PR #27) | S3 · Sonnet 4.6 | Neutral; ggf. später als mandanten-konfigurierter ASD-Layer (WF2-30/31). |
| 6a | **Firefly-UI-Aufräumen** | Firefly | Fireflys eingebettetes MapLibre-WebUI (`/` + `/ws`) entfernen — war nur relevant, bevor Wayfinder existierte; jetzt toter Code-Pfad. | S2 · Sonnet 4.6 | Neutral; rein Firefly-intern. |
| 7 | **FHA / Hazard-Analyse** | Firefly + Wayfinder | Sicherheits-Analyse-Dokument | S4 · Opus 4.8 | **Erweitern:** muss die **Multi-Tenant-Isolations-Hazards** (Cross-Tenant-Leckage) aufnehmen — koppelt an WF2-21/22. |
| 8 | **Sensor-Registrierung/Bias-Korrektur** | Firefly | M4-Nachtrag, Mess-Fusions-Erweiterung | S5 · Fable 5 / Opus 4.8 | **Enabler** für ehrliche Per-Track-Sensor-Provenienz (vgl. WF2-40/42). |
| 9 | **Live-OpenAIP-Integration** | Firefly | Wayfinder-Seite via ASD-003/ADR 0004 **erledigt**; offen nur etwaiger Firefly-Bedarf | S3 · Sonnet 4.6 | Wayfinder-seitig erledigt. |
| 17 | **SDPS-003 — Environment & Meteo** | Firefly | QNH für baro. Höhenkorrektur, DTM-Basis | S3 · Sonnet 4.6 | Neutral; künftiger ASD-Layer denkbar. |
| 18 | **SDPS-004 + ASD-006 — STCA (gekoppelt)** | Firefly + Wayfinder | Firefly: Konflikterkennung→Flag in CAT062 (I062/340), ICD-Bump. **Wayfinder ASD-006:** reiner Flag-Konsum (Data-Block blinkt), keine eigene Geometrie. | S4 · Opus 4.8 | **Wayfinder-Anteil ASD-006** ist ein **ASD-Kern-Sicht-Feature** in 2.0, ggf. **entitlement-gated** (WF2-50). Abh.: Firefly-ICD-Update zuerst. |
| 19 | **SDPS-001 — FEP Sensor Ingestion** | Firefly | UDP-Receiver CAT048/CAT001, dyn. Sensor-Konfig, Polar→kartesisch | S5 · Fable 5 / Opus 4.8 | **Enabler** für echte Sensor-Vielfalt/-Provenienz (WF2-40/41/42). |
| 20 | **SDPS-002 — High Availability & State Sync** | Firefly | Main/Standby, Leader Election, State-Sync, drop-out-freier Feed | S5 · Fable 5 / Opus 4.8 | **Parallel** zu Wayfinders HA (WF2-52/53): durchgängige Verfügbarkeit Ende-zu-Ende. |

**Erhaltene Architektur-Entscheidung (SDPS-004/ASD-006):** ASD-006 wird **nicht**
als unabhängige, Wayfinder-seitige STCA-Berechnung umgesetzt, sondern als
Konsument des von Firefly im CAT062-Strom gesetzten Alarm-Flags (I062/340) — kein
zweiter, abweichender Determinismus-Pfad. Das CAT062-ICD-Update wird im Rahmen
von #18 angekündigt, abgestimmt und versioniert. Diese Entscheidung bleibt unter
2.0 gültig.

**Cross-Project-Abhängigkeiten aus 2.0 (neu, in `docs/cross-project/todo-for-firefly.md` vermerkt):**
- **Per-Track-Sensor-Provenienz** (WF2-42) — nur via CAT062-ICD-Änderung sauber;
  `from-wayfinder`-Issue **erst bei Erreichen von Stufe 4** erstellen.
- **Feed-pro-Mandant** (Hybrid-Modell) — betrifft Fireflys Multicast-Gruppen-/
  Instanz-Modell; bei Stufe 2 (WF2-20) abstimmen.
- **Ende-zu-Ende-HA** — Wayfinder WF2-52/53 ↔ Firefly #20.

---

## 4. Begründung der Reihenfolge

**Wayfinder 2.0 (§1):** Entscheiden (Stufe 0) vor Bauen; Identität/Persistenz
(Stufe 1) **neben** dem laufenden Pfad de-riskt DB+Auth, ohne den Strom
anzufassen; der **sicherheitskritische** Stream-Umbau (Stufe 2) folgt auf
stabilisierter Basis mit **Pflicht-Negativtests** (Cross-Tenant-Isolation ist der
Worst-Case eines sicherheitsrelevanten Lagebilds); Komfort (3), Sensorik (4) und
Kommerz/HA (5) zuletzt. Sicherheits-/Schnittstellen-Wirkung hebt das Modell je
eine Stufe (deshalb viele 🔒-S3-Pakete „mit Opus-Review").

**ASD-Kern (§2):** parallele, risikoarme Sicht-Verbesserungen ohne
Architektur-Wirkung — nicht auf dem kritischen Pfad, aber jederzeit wertstiftend.

**Firefly-Backlog (§3):** nach Fireflys eigener Priorisierung; hier nur mit
2.0-Bezug annotiert, damit Schnittstellen-Wirkungen früh sichtbar sind.

---

## 5. Findings (Logging/Observability-Audit, 2026-06-15 — weiter gültig)

**Wayfinder:**
- `slog`/JSON durchgängig; Decode-Fehler mit Kontext geloggt; Log-Level
  konfigurierbar (NFR-OBS-001, erledigt).
- `broadcast.go::timeNowMs()` liefert noch `0` (TODO) — wird mit 2.0 relevant
  (Audit-Log/Replay je Mandant, WF2-23). Als Altlast vorgemerkt.
- **Shutdown-Härtung (vorbestehend, bei WF2-20.2 bestätigt):** `receiver.Run`
  prüft `ctx` nur zwischen Datagrammen und blockiert in `ReadFromUDP`; sauberes
  Herunterfahren hängt am Conn-Schließen/Prozess-Ende. Mit N Feeds unverändert
  relevant — als Betriebs-Härtung (sauberes `SIGTERM`) vormerken (Stufe-5-Umfeld).

**Firefly:** siehe Fireflys Roadmap (tracing/Metriken-Lücken historisch, größtenteils geschlossen).

---

## 6. Erledigt (Referenz)

**Wayfinder-2.0-Vorlauf:**
- ✅ Konzept Wayfinder 2.0 erstellt & auf `main` (`docs/design/wayfinder-2.0-konzept.md`, PR #25) — 6 Ausbaustufen, ~28 WF2-Pakete, Modell-Tabelle, zwei Leitentscheidungen.
- ✅ **WF2-00 / ADR 0005 — Multi-Mandanten-Pivot** (`docs/decisions/0005-multi-mandanten-pivot.md`): Pivot ratifiziert, Hybrid-Modell, Isolationsgrenze, Kommerz-Scope, 12-Factor-Grenze; Register FR-TEN-001/NFR-SEC-003.
- ✅ **WF2-01 / ADR 0006 — Konfig-/Identitäts-Persistenz** (`docs/decisions/0006-konfig-identitaets-persistenz.md`): PostgreSQL + pgx/sqlc + goose, Schema-Skizze, Stateless-Split, Identität (OIDC@Proxy primär + Fallback), Redis zurückgestellt; Register FR-TEN-002/NFR-SEC-004.
- ✅ **WF2-02 / ADR 0007 — Cloud-Ingest & Feed-Fan-out** (`docs/decisions/0007-cloud-ingest-und-feed-fan-out.md`): Public Cloud + K8s; FeedSource-Abstraktion, Ingest-Gateway, **NATS JetStream** (RabbitMQ/Kafka geprüft & verworfen); Register FR-FEED-001/NFR-SCALE-001. **→ Stufe 0 abgeschlossen.**
- ✅ **WF2-10.1 — Persistenz-Schicht** (`pkg/store`: pgx-Pool, eingebetteter In-House-Migrationsrunner, Schema `00001_init`, DB-freie Tests; ADR 0006 Nachtrag: goose→Runner, Go-Baseline 1.25). Milestone `docs/milestones/WF2-10.1_Persistence_Layer.md`.
- ✅ **WF2-10.2 — Tenant-/User-Repositories** (`models.go`, `tenants.go`, `users.go`, `repo.go`; handgeschriebene pgx-Queries; `GetBySubject` = Identität→Mandant). Tests DB-frei + Integration; **real gegen PostgreSQL 16** via `scripts/pg-test.sh`. Milestone `docs/milestones/WF2-10.2_Tenant_User_Repos.md`.
- ✅ **WF2-10.3a — Feed-/Subscription-/Entitlement-Repositories** (`feeds.go`, `subscriptions.go`, `entitlements.go`; JSONB-`sensor_mix`, `ListFeedsByTenant` = WF2-21-Query, Entitlements default-deny). Daten-Isolations-Test + real gegen PostgreSQL 16. Milestone `docs/milestones/WF2-10.3a_Feed_Subscription_Entitlement_Repos.md`.
- ✅ **WF2-10.3b — View-Config-Repository** (`view_configs.go`; BBox/ViewConfig, Tenant-Default + Nutzer-Override, `GetEffective`; Migration `00002` Partial-Unique-Index). **→ WF2-10 (Persistenz-Schicht) komplett.** Milestone `docs/milestones/WF2-10.3b_ViewConfig_Repo.md`.
- ✅ **WF2-11.1 — AuthN builtin-Primitive** (`pkg/auth`: Mode/ParseMode, argon2id-Passwort-Hashing, HMAC-Session-Token, None-/Builtin-Authenticator; `golang.org/x/crypto`). 10 DB-freie Tests. Milestone `docs/milestones/WF2-11.1_Auth_Builtin_Primitives.md`.
- ✅ **WF2-11.2 — AuthN proxy-Modus OIDC** (`proxy.go`: `ProxyAuthenticator`, go-oidc-Validierung Issuer/Audience/Signatur/Ablauf; `github.com/coreos/go-oidc/v3`). Tests gegen lokalen Test-Issuer (RSA/JWKS/JWT). **→ WF2-11 (AuthN) komplett.** Milestone `docs/milestones/WF2-11.2_Auth_Proxy_OIDC.md`.
- ✅ **WF2-12.1 — Tenant-Context-Middleware** (`pkg/auth/factory.go` `NewAuthenticator`; neues `pkg/tenant`: `Identity`/Context, `Middleware` subject→user→tenant fail-closed). DB-freie Tests (Erfolg + 3× fail-closed → 401). Milestone `docs/milestones/WF2-12.1_Tenant_Context_Middleware.md`.
- ✅ **WF2-12.2 — Tenancy-Verdrahtung im Server** (`main.go` `setupTenancy`: DB-Open+Migrate+Authenticator+`tenant.Middleware` auf `/ws`, nur bei `WAYFINDER_DB_URL`; sonst Single-Tenant). ENV-Vars in INSTALLATION/TECHNICAL; real-PG-Integrationstest. Milestone `docs/milestones/WF2-12.2_Tenancy_HTTP_Wiring.md`.
- ✅ **WF2-12.3 — Builtin-Login** (Migration `00003_credentials` + `pkg/store/credentials.go` `CredentialRepo` Set-Upsert/GetHash; `pkg/tenant/login.go` `/api/login` timing-gehärtet → `auth.MintSession`-HttpOnly-Cookie + `/api/logout`; `WAYFINDER_SESSION_TTL`; nur in `builtin`-Modus registriert). DB-freie Login-Tests + real-PG `CredentialRepo`-Test. **→ WF2-12 (Tenant-Context) komplett.** Milestone `docs/milestones/WF2-12.3_Builtin_Login.md`.
- ✅ **WF2-13 — Admin-Bootstrap** (`cmd/wayfinder/bootstrap.go`: Subcommand `wayfinder bootstrap`, idempotentes Get-or-Create erster Tenant/Admin + optional builtin-Passwort via `WAYFINDER_BOOTSTRAP_PASSWORD`, kein Cross-Tenant-Re-Homing; `pkg/tenant/authz.go` `RequireRole`-Gate auf `/admin`, fail-closed `403`). DB-freie Tests (`validate`, `RequireRole`) + real-PG `TestIntegrationBootstrap` + E2E-Rauchtest. **→ Stufe 1 komplett.** Milestone `docs/milestones/WF2-13_Admin_Bootstrap.md`.
- ✅ **WF2-20.1 — `feed_id`-Durchstich** (`receiver.Config.FeedID` + Handler-Signatur `(feedID, tracks)`; `broadcast.TrackBatch{FeedID,Tracks}` stempelt `TrackMessage.feed_id`; `WAYFINDER_FEED_ID`). Attribution-Naht Receiver→Broadcaster→Wire; Single-Tenant unverändert. Tests: `TestHandleTracksStampsFeedID`, `TestTracksToMessage` (feed_id). Milestone `docs/milestones/WF2-20.1_FeedID_Plumbing.md`.
- ✅ **WF2-20.2 — Multi-Feed-Receiver** (`cmd/wayfinder/feeds.go` `resolveFeeds`/`buildReceivers`: DB-`feeds`-Katalog → N Receiver je `feed_id`, ENV-Fallback bei leerem Katalog/kein-DB; `main()`-Reorder DB-vor-Receiver, per-Feed-Listen-Skip, Decode-Fehler-Summe; Feed-CLI `cmd/wayfinder/feedcmd.go` `feed add`/`feed list`). DB-freie + real-PG (`TestIntegrationFeedCatalogue`) + E2E-Rauchtest. **→ WF2-20 komplett.** Milestone `docs/milestones/WF2-20.2_Multi_Feed_Receiver.md`.
- ✅ **WF2-21.1 — Scoped Fan-out (Feed-Isolation)** (`pkg/broadcast` `Scope`/`NewScope`/`AllowsFeed` + `broadcastTracks` feed-gefiltert, Feed-Health bleibt global; `pkg/ws` `ScopeResolver` am Handshake fail-closed `403`; `cmd/wayfinder.newScopeResolver` via `subscriptions.ListFeedIDsByTenant`). **Pflicht-Negativtest** `TestBroadcastFeedIsolation` (A bekommt nie B's Feed) + `TestScopeAllowsFeed` + Resolver-Tests (fail-closed). Single-Tenant unverändert. Milestone `docs/milestones/WF2-21.1_Feed_Level_Isolation.md`.
- ✅ **WF2-21.2 — Scoped Fan-out (Sicht-Filter AOI/FL)** (`pkg/broadcast` `BBox`/`ViewFilter`/`Scope.filterView` — harte server-seitige AOI/FL-Grenze, **fail-open** bei fehlendem Attribut; `cmd/wayfinder.resolveViewFilter` via `view_configs.GetEffective`, FL→Fuß). Tests: `TestViewFilterAdmits` (inkl. fail-open) + `TestBroadcastViewScoping` + `TestResolveViewFilter` + real-PG `TestIntegrationResolveViewFilter`. Lebenszyklus client-seitig; Klassifizierung später (WF2-40). **→ WF2-21 komplett.** Milestone `docs/milestones/WF2-21.2_View_Filter.md`.
- ✅ **WF2-22 — Isolations-Testsuite** (`pkg/broadcast/isolation_test.go`: Differential-Property `TestFilterViewMatchesOracle` 50k Iter vs. unabhängiges Oracle, Ende-zu-Ende `TestBroadcasterIsolationProperty`, `FuzzScopeFilter` 755k execs 0 Fehler). Test-only, kein Produktivcode-Befund. **→ sicherheitskritischer Kern testseitig abgesichert.** Milestone `docs/milestones/WF2-22_Isolation_Test_Suite.md`.
- ✅ **WF2-23.1 — Audit-Log** (`cmd/wayfinder.logScopeAudit` + `newScopeResolver`-Logger: strukturiertes `slog`-Event `component=audit`/`ws_connect` mit tenant_id/user_id/subject/role/feeds/aoi/fl beim `/ws`-Connect; 12-Factor, keine DB; hochkardinale Identität nur im Audit-Log). `TestScopeResolverEmitsAudit`. Milestone `docs/milestones/WF2-23.1_Audit_Log.md`. *(Pro-Tenant-Metriken folgen WF2-23.2.)*

**Cross-Project / Firefly:**
- ✅ Paket #6 / Coverage-Werkzeug — Radar-Ringe-Overlay (`pkg/coverage`, `/api/coverage/rings`, Frontend-Layer-Toggle, Firefly `SensorModel`-Erweiterung; PR #27)

**ASD-Kern / Frontend:**
- ✅ ASD-007 Farbschema · ASD-008 Navigation Rail · ASD-009 Karten-Controls · ASD-010 Kategorie-Filter-Chips (Phase 1, Vue 3 + Vuetify 3 MD3)
- ✅ ASD-006 Vue 3 + Vuetify 3 (MD3) Migration (ADR 0002)
- ✅ Paket #16 / ASD-002 — Anti-Garbling (Label-Deconfliction + Drag&Drop)
- ✅ Paket #15 / ASD-005 — Höhen- und Filter-Tools
- ✅ Paket #14 / ASD-004 — Track-Lebenszyklus & History
- ✅ Paket #13 / ASD-003 — Aeronautical Map Layer (Radar Dark Mode, Live-OpenAIP, Overlays; ADR 0004)
- ✅ Paket #12 / ASD-001 — Erweiterter Data Block
- ✅ AP9.9 — ADS-B-Badge im Track-Label (ICD 2.4.0, PR #22)

**Daten-/Betriebs-Pakete:**
- ✅ Paket #3 / AP5/AP6 — CAT065 SDPS-Heartbeat, ICD 2.3.0 (ADR 0018)
- ✅ Paket #2 — Observability-Grundgerüst (Log-Level, `/metrics` beidseitig)
- ✅ Paket #1 — Multicast-Feed-Sicherheit (Wayfinder ADR 0003, Browser-Rand)
- ✅ AP7/AP8 — CAT062 I062/245 Callsign, ICD 2.1.0 (PR #7)
- ✅ ADR 0016/TSE — CAT062 I062/080 Track-Ende, ICD 2.2.0 (PR #8)
- ✅ AP1/AP2 — CAT062 I062/136 Vertikallage + UAP-Standardtreue, ICD 2.0.0 (ADR 0015)
- ✅ (Firefly) Paket #10 SDPS-005 Recording/Replay · #11 SDPS-006 Observability

---

## 7. Pflege-Hinweis

- **Status & Reihenfolge** werden **hier** gepflegt (maßgeblich). Neue 2.0-Pakete
  bekommen eine `WF2-xx`-Nummer; erledigte wandern nach §6.
- **Konsistenz-Regel (keine Widersprüche):** `docs/STATUS.md` „Nächster Schritt"
  **muss** mit §0/§1 dieser Datei übereinstimmen. Die fachliche Begründung der
  2.0-Pakete steht im Konzept (`docs/design/wayfinder-2.0-konzept.md`); ändert
  sich der Plan, werden **beide** nachgezogen.
- **Geteilter Teil:** Nur **§3** wird mit Fireflys Roadmap synchron gehalten;
  §0–§2 sind Wayfinder-spezifisch. Cross-Project-Wirkungen laufen über
  `docs/cross-project/todo-for-firefly.md` + `from-wayfinder`-Issues (erst beim
  Erreichen der jeweiligen Stufe erstellen, nicht prophylaktisch).
