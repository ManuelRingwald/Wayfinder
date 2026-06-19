# ADR 0006 — Wayfinder 2.0: Konfigurations- und Identitäts-Persistenz

- **Status:** akzeptiert (Datastore, Zugriffsschicht, Migrationen, Identitäts-
  Modell und Stateless-Split entschieden; konkrete Umsetzung folgt WF2-10..13)
- **Datum:** 2026-06-19
- **Schnittstellen-relevant:** nein (keine Änderung am CAT062-Draht-Vertrag;
  betrifft Wayfinder-interne Persistenz und den Browser-Rand/Identitäts-Pfad)
- **Bezug:** baut auf **ADR 0005** (Multi-Mandanten-Pivot) auf; entscheidet die
  dort als „offen" markierten Punkte Datastore/Cache (Konzept §11.3) und
  Identitäts-Anbindung (§11.4). Setzt das Identitäts-Muster aus **ADR 0003**
  (TLS/Auth primär am Proxy, fail-closed-Fallback in Wayfinder) konsequent fort.
  Cloud-Ingest/Feed-Transport bleibt **ADR 0007** vorbehalten.

## Kontext

ADR 0005 hat den Pivot zur mandantenfähigen Plattform ratifiziert und das
konzeptuelle Datenmodell (Tenant, User, Feed, Subscription, ViewConfig,
Entitlement) sowie die Isolationsgrenze gesetzt. Wayfinder 1.x hat dafür **keine
Grundlage**: Konfiguration kommt einmalig beim Start aus ENV/`wayfinder.yaml`
(`loadConfig()`), es gibt **keine Persistenz**, **keine Nutzer/Mandanten** und nur
ein optionales geteiltes Token (ADR 0003).

Vor der Umsetzung von Stufe 1 (WF2-10..13) müssen vier Weichen bewusst gestellt
werden:

1. **Welcher Datastore** und **welche Zugriffsschicht** (typsicher, auditierbar,
   zertifizierungs-freundlich)?
2. **Wie werden Schema-Änderungen** versioniert (Konfigurationsmanagement,
   CLAUDE.md §7)?
3. **Wie wird Identität angebunden** — eigener Nutzer-/Passwort-Speicher oder
   Auslagerung an einen IdP — und wie entsteht daraus der **Tenant-Kontext**?
4. **Was bleibt zustandslos** (12-Factor, horizontale Skalierung) und was wandert
   in die DB?

## Entscheidung

### 1. Datastore = PostgreSQL

Relationaler Speicher mit starker Konsistenz für Mandanten-/Identitäts-Daten,
`JSONB` für flexible Felder (ViewConfig, Sensor-Mix), breite Managed-Verfügbarkeit
in jeder Cloud und On-Prem. Bestätigt aus Entwurf und Konzept.

### 2. Zugriffsschicht = `pgx` + `sqlc` (kein schweres ORM)

- **`pgx`** als nativer PostgreSQL-Treiber.
- **`sqlc`** generiert **typsicheres Go aus handgeschriebenem SQL**. Das hält das
  SQL **explizit und auditierbar** (Analysierbarkeit/Zertifizierung, CLAUDE.md
  §7) und vermeidet Reflection-Magie. Verworfen: GORM (reflection-lastig,
  schlechter analysierbar), reines `database/sql` (viel Hand-Scannen, fehleranfällig).

### 3. Migrationen = `goose` (versioniert, eingebettet)

Versionierte SQL-Migrationen, via `//go:embed` ins Binary eingebettet, ausführbar
beim Start oder per CLI. Liefert **getaggte Schema-Baselines** (Konfigurations-
management, CLAUDE.md §7). `golang-migrate` ist gleichwertig und zulässig, falls
sich Betriebsgründe ergeben.

### 4. Schema-Skizze (Detail folgt in den Migrationen, WF2-10)

```
tenants(id PK, slug UNIQUE, name, status, created_at)
users(id PK, tenant_id FK→tenants, subject UNIQUE, email, role, created_at)
        role ∈ {operator, tenant_admin, super_admin}
feeds(id PK, name, multicast_group, port, region, sensor_mix JSONB, created_at)
        -- globaler Katalog; Mandanten besitzen Feeds nicht, sie abonnieren sie
subscriptions(tenant_id FK, feed_id FK, PRIMARY KEY(tenant_id, feed_id))
view_configs(id PK, tenant_id FK, user_id FK NULL, center_lat, center_lon, zoom,
             aoi JSONB, fl_min, fl_max, layers JSONB, updated_at)
        -- tenant-Default (user_id NULL) + optionale Nutzer-Übersteuerung
entitlements(tenant_id FK, feature_key, enabled, PRIMARY KEY(tenant_id, feature_key))
        -- Feature-Flags als Daten (ADR 0005 §4)
```

`feeds` ist ein **globaler Katalog**; die Sichtbarkeit steuert ausschließlich
`subscriptions` (Hybrid-Modell, ADR 0005 §2). Der **Sensor-Mix ist Feed-Eigenschaft**
(`feeds.sensor_mix`), nicht Per-Track-Tag (ADR 0005 §8).

### 5. Identität = OIDC am Reverse-Proxy **primär**, eingebauter Fallback **optional**

Konsequent zum Muster aus **ADR 0003** (kein Krypto-/Auth-Eigenbau im ASD):

- **Primär (`WAYFINDER_AUTH_MODE=proxy`): OIDC/oauth2-proxy am vorgelagerten
  Ingress.** Der Proxy authentifiziert und reicht die Identität als **vertrauens-
  würdigen, signierten Header / JWT** weiter (z. B. `Authorization: Bearer` oder
  `X-Forwarded-*`). Wayfinder **validiert** (Issuer/Audience/Signatur) und mappt
  `subject → users.subject → tenant_id`. **Kein Passwort-Speicher** in Wayfinder —
  kleinste Angriffsfläche, IdP-Anbindung gehört der Betriebsumgebung.
- **Optional (`WAYFINDER_AUTH_MODE=builtin`): eingebaute Nutzer** (`users` +
  separater Credential-Speicher, **argon2id**-Hashes) für On-Prem-Einzel-
  installationen ohne IdP. Default **aus**; spiegelt ADR 0003 (optionales Token/TLS).
- **`WAYFINDER_AUTH_MODE=none`:** nur der degenerierte Single-Tenant-Fall
  (ADR 0005 §7), Default-Tenant, **mit deutlicher Warn-Log-Zeile** (wie der
  Token-Default in ADR 0003).
- **Tenant-Kontext-Auflösung** (Umsetzung WF2-12): `subject → user → tenant_id`
  in einen server-seitigen Request-Kontext; **fail-closed** — keine auflösbare,
  gültige Zuordnung ⇒ **keine** Daten (NFR-SEC-003).
- **Session:** im `proxy`-Modus hält der Proxy die Session; Wayfinder bleibt
  möglichst **session-los** (validiert je Request den Token). Im `builtin`-Modus
  ein signiertes, kurzlebiges Session-Cookie (Signing-Key aus ENV); **keine**
  Server-Session-Tabelle als Pflicht (zustandslos bevorzugt).

### 6. Stateless-App / State-Split

- **Alle Mandanten-/Konfig-/Identitäts-Daten** liegen in PostgreSQL; das **Live-
  Lagebild** im (späteren) Feed-/Stream-Layer (ADR 0007). **Kein durabler,
  node-lokaler Zustand** in der Wayfinder-Instanz → jede Instanz bedient nach
  einem DB-Lookup jeden Request (horizontale Skalierung, WF2-52).
- **Secrets/Infra in ENV** (12-Factor, ADR 0005 §6): `WAYFINDER_DB_URL`,
  `WAYFINDER_OIDC_ISSUER`/`_AUDIENCE`, `WAYFINDER_SESSION_KEY`,
  `WAYFINDER_AUTH_MODE` — **nicht** in der Mandanten-DB.

### 7. Cache = vorerst In-Proc-TTL, Redis zurückgestellt

Tenant-/View-Config wird bei Session-Aufbau gelesen und **in-proc mit kurzer TTL**
gecacht. Ein **gemeinsamer Cache (Redis)** wird **erst bei gemessenem DB-Hotspot**
eingeführt (Konzept §6.2) — dann als **ephemerer Cache, nicht** als Quelle der
Wahrheit. Das Laufzeit-Nachladen ohne Neustart ist Sache von **WF2-30**.

### 8. Abgrenzung

- **Cloud-Ingest/Feed-Transport** (direkt-Multicast vs. NATS/Kafka, Gateway) →
  **ADR 0007**.
- **Konkreter scoped-Broadcast-Prädikat-Code** → WF2-21.
- Diese ADR liefert die **Persistenz- und Identitäts-Grundlage**, nicht die
  Daten-Ebenen-Umsetzung.

## Begründung

- **`pgx`/`sqlc` statt ORM:** explizites, auditierbares SQL ist für ein
  sicherheits-/zertifizierungs-orientiertes System wertvoller als ORM-Komfort.
- **OIDC@Proxy primär:** identisch begründet wie ADR 0003 — erprobte Auth am
  Rand, kein Eigenbau im ASD, Wayfinder bleibt schlank/analysierbar; der
  eingebaute Fallback hält Standalone-/On-Prem-Betrieb möglich (fail-closed,
  Default-aus).
- **Stateless + State in DB/Stream:** erfüllt das „zustandslose Container"-Ziel
  des Entwurfs und ermöglicht die spätere horizontale Skalierung ohne Sticky-
  Sessions.
- **Redis zurückgestellt:** keine prophylaktische Betriebs-Komplexität; erst
  Messung, dann Cache.

### Verworfene Alternativen

- **Eingebaute Nutzerverwaltung als Primärweg:** mehr sicherheitsrelevanter
  Eigenbau (Passwort-Reset, MFA, Lockout …), schlechter auditierbar. Verworfen
  als Primär-, erhalten als optionaler Standalone-Pfad.
- **GORM / schweres ORM:** Reflection-lastig, schlechter analysierbar. Verworfen.
- **Konfiguration weiter nur aus ENV/Datei:** widerspricht der
  Laufzeit-Konfigurierbarkeit (ADR 0005); ENV bleibt nur für Infra/Secrets.
- **Redis von Anfang an:** verfrühte Komplexität ohne gemessenen Bedarf. Verworfen
  (zurückgestellt).
- **SQLite/Document-Store:** SQLite skaliert nicht für mehr-Instanz-Schreiblast;
  ein Document-Store verliert die relationale Integrität der Tenant-/Abo-
  Beziehungen. Verworfen.

## Konsequenzen

- **Neue Anforderungen im Register** (`docs/requirements/`):
  - **FR-TEN-002** — Konfig-/Identitäts-Persistenz: PostgreSQL + `pgx`/`sqlc` +
    `goose`-Migrationen; Schema (tenants/users/feeds/subscriptions/view_configs/
    entitlements); Stateless-Split (State in DB, Infra/Secrets in ENV).
    Implementierung folgt WF2-10.
  - **NFR-SEC-004** — Identitäts-/Session-Sicherheit: `WAYFINDER_AUTH_MODE`
    (proxy|builtin|none), OIDC-Validierung (Issuer/Audience/Signatur) im
    proxy-Modus, argon2id im builtin-Modus, Tenant-Kontext fail-closed,
    Secrets in ENV. Implementierung folgt WF2-11/12.
- **Neue ENV-Variablen** (`WAYFINDER_DB_URL`, `WAYFINDER_OIDC_*`,
  `WAYFINDER_SESSION_KEY`, `WAYFINDER_AUTH_MODE`) werden in `docs/INSTALLATION.md`
  und `docs/TECHNICAL.md` eingetragen, **sobald WF2-10/11 sie tatsächlich
  einlesen** — nicht jetzt (sie sind heute noch wirkungslos; die Betriebsdoku
  soll nichts Nicht-Existierendes versprechen).
- **Neue Go-Abhängigkeiten** (`pgx`, `sqlc`-generiert, `goose`) kommen mit WF2-10.
- **ROADMAP/STATUS:** WF2-01 erledigt; nächster Schritt = **WF2-02 / ADR 0007**.

## Ehrliche Grenze

- Diese ADR **garantiert keine Isolation/Sicherheit**, solange WF2-10..12 (Schema,
  Tenant-Context, AuthZ) nicht implementiert und — für die Isolation — WF2-22
  (Negativtests) nicht grün sind.
- Die **Identitäts-Sicherheit im proxy-Modus hängt am korrekt konfigurierten
  Proxy** (gleiche ehrliche Grenze wie ADR 0003): ein fehlkonfigurierter Proxy,
  der ungeprüfte Identitäts-Header durchlässt, untergräbt das Modell. Wayfinder
  validiert den Token defensiv, ersetzt aber kein Proxy-/Netz-Audit.
- **Migrations-/Backup-/DR-Betrieb** der Postgres-Instanz ist Deployment-Sache und
  Teil der späteren „Betriebs-Härtung", nicht dieser Entscheidung.
