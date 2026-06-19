# Wayfinder 2.0 — Konzept: Von statischem Single-Tenant-ASD zur konfigurierbaren Multi-Mandanten-Plattform

> **Status:** Entwurf / zur Abstimmung (noch **nicht** ratifiziert)
> **Datum:** 2026-06-19
> **Grundlage:** Entwurf „Wayfinder 2.0" (Ziel-Architektur: Von Statisch zu SaaS)
> **Einstufung dieses Dokuments:** Architektur-Konzept, S4–S5 (strategische
> Weichenstellung mit Sicherheits- und Schnittstellen-Wirkung) — Modell Opus 4.8.
> **Verbindlichkeit:** Dieses Konzept ist ein **Vorschlag**. Es ersetzt keine
> ADRs; die hier benannten Leitentscheidungen werden vor der Umsetzung je als
> ADR ratifiziert (CLAUDE.md §3 „Erst abstimmen, dann bauen").
> **Entscheidungs-Stand (2026-06-19):** Zwei Richtungsentscheidungen gefallen
> (§11): **Mandanten-Modell = Hybrid** (§6.1) und **Kommerz-Scope = Feature-Flags
> ja, Stripe-Billing zurückgestellt** (§6.5). Übrige Entscheidungen (§11.3–11.6)
> fallen je im zugehörigen ADR.

---

## 1. Management-Summary (TL;DR)

Der Entwurf beschreibt den Umbau von Wayfinder vom **einprozessigen,
beim-Start-konfigurierten Single-Tenant-ASD** zu einer **mandantenfähigen,
zur-Laufzeit-konfigurierbaren Cloud-Plattform** (SaaS) mit Datenbank-Konfiguration,
Admin-Oberfläche, Sensor-/Stream-Management, optionaler Monetarisierung und
hochverfügbarem Betrieb.

Das ist **technisch tragfähig und sinnvoll gestaffelt umsetzbar**, aber es ist ein
**größerer strategischer Pivot als ADR 0014** und berührt drei harte Realitäten
der bestehenden Architektur, die der Entwurf noch nicht auflöst:

1. **Ein Feed = ein Himmel.** Heute trägt genau **ein** CAT062-Strom **eine**
   Luftlage. „Mehrere Kunden" heißt entweder *mehrere Sichten auf dieselbe Lage*
   (Variante A) oder *ein eigener Feed je Mandant* (Variante B). Das ist die
   zentrale Weichenstellung (§6.1) und muss **zuerst** entschieden werden.
2. **CAT062 liefert fusionierte System-Tracks, keine Roh-Plots.** Der im Entwurf
   skizzierte „Decoder taggt Plots mit `source=ADS-B/FLARM`" passt nicht zum
   Vertrag: Wayfinder sieht bereits getrackte Daten, und Firefly-Code wird
   **nicht** importiert (CLAUDE.md §10). Sensor-Filterung ist nur über
   *Feed-Ebene* + aus dem Vertrag *ableitbare* Provenienz-Signale sauber
   machbar (§5.2, §6.4).
3. **Multicast überquert keine Cloud-Subnetze.** Der Entwurf hat das richtige
   Bauchgefühl („zentraler Ingest → Kafka/RabbitMQ"): in der Cloud ist ein
   **Multicast→Unicast-Ingest-Gateway** keine Kür, sondern Pflicht (§5.3, §6.2).

Vorgeschlagen werden **sechs Ausbaustufen (0–5)** mit **insgesamt ~28
Arbeitspaketen** (`WF2-xx`), bei denen die **sicherheitskritische Daten-Isolation
zwischen Mandanten** der Dreh- und Angelpunkt ist (Variante: „Frankfurt darf
Stuttgart nicht sehen"). Monetarisierung/Billing (Epic 4 des Entwurfs) wird
**bewusst als letzte, optionale Stufe** außerhalb des zertifizierbaren ASD-Kerns
geführt.

---

## 2. Ist-Stand (worauf wir real aufsetzen)

Geerdet am Code, nicht am Wunsch:

| Dimension | Heute (Wayfinder 1.x) | Beleg im Code |
|-----------|------------------------|---------------|
| **Prozessmodell** | Ein Go-Prozess, ein Binary | `cmd/wayfinder/main.go` |
| **Konfiguration** | **Einmalig beim Start**: ENV > `wayfinder.yaml` > Defaults (12-Factor) | `loadConfig()`, `loadYAMLFile()` |
| **Eingang** | **Ein** CAT062/CAT065-Multicast-Strom = **eine** Luftlage | `pkg/receiver`, `FIREFLY_CAT062_GROUP` |
| **Verteilung** | **Ein** `Broadcaster` fächert **jeden** Track an **jeden** Client (all-to-all, kein Filter) | `broadcast.go::broadcast()` |
| **Identität** | Keine Nutzer, keine Rollen, keine Mandanten | — |
| **AuthN/AuthZ** | Optional **ein** geteiltes Bearer-Token (`WAYFINDER_AUTH_TOKEN`), optional TLS, Origin-Allowlist | `main.go::authMiddleware`, `ws/handler.go::checkOrigin` |
| **Persistenz** | **Keine** — keine DB, kein State auf Platte | — |
| **Live-State** | In-Prozess: Client-Registry (`sync.Map`) + Lagebild im Browser-State | `broadcast.go`, `frontend/src/stores/asd.js` |
| **Karten-Kontext** | OpenAIP-Overlays um **ein** konfiguriertes Zentrum/Radius | `pkg/aeronautical`, ADR 0004 |
| **Sensor-Provenienz** | Aus dem Vertrag *ableitbar*: `adsb_age_s` (ADS-B-Anteil), `psr_age` (PSR-Anteil); ADS-B-Badge ◆ existiert bereits | `cat062/decoder.go`, FR-ASD-006 |

**Kernbefund:** Wayfinder ist heute schon weitgehend *zustandslos* (keine
Persistenz), aber **nicht mandantenfähig**: Der Datenpfad kennt nur „alle Tracks
an alle". Multi-Tenancy ist deshalb **kein Anbau am Rand**, sondern ein Umbau im
**Herzen des Broadcasters** — und damit sicherheitskritisch.

---

## 3. Soll-Vision (Entwurf, getreu zusammengefasst)

Der Entwurf definiert fünf Themenblöcke:

- **Fundament:** Tenant-ID je Kunde; Konfiguration aus `docker-compose.yml` →
  relationale DB (PostgreSQL); **Dynamic Lookup** je Session statt Neustart.
- **Epic 1 — Multi-Tenancy & Daten-Isolation:** globale User-/Tenant-DB; jeder
  API-Request **und jeder WebSocket-Strom** wird per Tenant-ID gefiltert.
- **Epic 2 — Dynamic Configuration Engine & Admin-UI:** geschützter `/admin`-
  Bereich (Formulare/Slider); Backend liest Config je Session aus DB/Cache (Redis)
  statt aus ENV.
- **Epic 3 — Sensor-/Stream-Management (SSR, PSR, ADS-B, FLARM):** Feeds pro
  Mandant freischaltbar; Middleware blockt nicht-abonnierte Sensortypen.
- **Epic 4 — Monetarisierung & Feature-Flagging:** Basic/Premium, Stripe-Billing,
  Feature-Flags (`tenant.HasPremiumFeature("psr_radar")`).
- **Cloud-Infrastruktur & HA:** zustandslose Container hinter Load Balancer (3×
  parallel); zentraler Ingest nimmt Multicast/UDP an und verteilt intern (Kafka/
  RabbitMQ).

---

## 4. Ziel-Architektur (Komponentenbild)

```
                         ┌─────────────────────────────────────────────┐
   Radar/Sensoren        │                 WAYFINDER 2.0                │
   (Firefly-Instanzen)   │                                             │
        │ CAT062/065     │   ┌───────────────┐    ┌──────────────────┐ │
        │ UDP-Multicast  │   │ Ingest-Gateway│    │  Config-/Identity│ │
        ▼                │   │ (mcast→unicast│    │   -Store (Pg)    │ │
   ┌─────────┐  Feed A   │   │  Fan-out:     │    │ tenants, users,  │ │
   │ Feed A  ├───────────┼──▶│  Kafka/NATS/  │    │ feeds, subs,     │ │
   ├─────────┤  Feed B   │   │  direkt mcast)│    │ view-config,     │ │
   │ Feed B  ├───────────┼──▶│               │    │ entitlements     │ │
   └─────────┘           │   └───────┬───────┘    └────────┬─────────┘ │
                         │           │ pro-Feed-Tracks      │ lookup    │
                         │           ▼                      ▼           │
                         │   ┌─────────────────────────────────────┐   │
                         │   │   Wayfinder-Instanz (stateless, N×)  │   │
                         │   │  ┌────────────┐   ┌───────────────┐  │   │
                         │   │  │ Scoped     │   │ Tenant-Context│  │   │
                         │   │  │ Broadcaster│◀──│ + AuthZ       │  │   │
                         │   │  │ (Prädikat: │   │ (Session→     │  │   │
                         │   │  │ feed∩AOI∩  │   │  Tenant-ID)   │  │   │
                         │   │  │ FL∩Kat)    │   └───────────────┘  │   │
                         │   │  └─────┬──────┘                      │   │
                         │   └────────┼─────────────────────────────┘   │
                         └────────────┼─────────────────────────────────┘
                                      │ nur abonnierte/erlaubte Tracks
                                      ▼  (WSS, je Mandant isoliert)
                            ┌──────────────────────┐
                            │  Browser je Mandant  │  (Admin: /admin,
                            │  (Lotse / Operator)  │   Operator: ASD-Karte)
                            └──────────────────────┘
```

Neue tragende Bausteine gegenüber heute: **Ingest-Gateway**, **Config-/Identity-
Store**, **Tenant-Context/AuthZ**, **Scoped Broadcaster** (Prädikat statt
all-to-all), **Feed-Registry** (mehrere Feeds), **Admin-API/-UI**,
**Entitlement-Service** (+ optional Billing-Plane).

---

## 5. Architektur-Findings / Spannungsfelder (die ehrliche Prüfung)

### 5.1 Ein Feed = ein Himmel (zentrale Weichenstellung)
Heute ⇒ **eine** Luftlage. „Mehrere Kunden" hat zwei grundverschiedene
Bedeutungen, die die gesamte Architektur prägen (Entscheidung in §6.1).

### 5.2 CAT062 ist fusioniert, nicht plot-roh (Epic 3 muss umformuliert werden)
Der Entwurf modelliert Sensor-Filterung auf **Plot-Ebene** (`source=ADS-B`,
`source=FLARM`). Wayfinder sieht aber **fusionierte System-Tracks**; einen
sauberen „FLARM-vs-ADS-B-vs-PSR"-Diskriminator je Track gibt der CAT062-Vertrag
heute **nicht** her, und Firefly-Code wird **nicht** importiert (CLAUDE.md §10).
Real verfügbar sind nur *abgeleitete* Provenienz-Signale (`adsb_age_s`,
`psr_age`, Teile von I062/080). **Saubere Lösung:** Sensor-*Mix* ist eine
Eigenschaft des **Feeds** (Feed-Metadaten), nicht des einzelnen Tracks; echte
Per-Track-Provenienz (z. B. FLARM) wäre eine **ICD-Änderung in Firefly**
(Cross-Project, §10).

### 5.3 Multicast überquert keine Cloud-Subnetze
AWS/Azure-VPCs routen UDP-Multicast standardmäßig nicht. Der Entwurf hat den
richtigen Reflex („zentraler Ingest → Kafka"). On-Prem/Sub-Netz kann jede
Wayfinder-Instanz der Multicast-Gruppe direkt beitreten; **in der Cloud** ist das
**Ingest-Gateway** (Multicast→Unicast/Stream) **zwingend** und der natürliche Ort
für den Feed-Fan-out an die zustandslosen Instanzen.

### 5.4 Strategischer Reframe braucht ein ADR
Der Charter rahmt Wayfinder als **sicherheitsrelevantes ASD für Lotsen**
(Orientierung ED-153/ED-109A/DO-278A). Der Entwurf rahmt es als **kommerzielles
SaaS** (Stripe, Basic/Premium, „Netflix-Modell"). Das ist legitim, ändert aber
**Produktnatur, Bedrohungsmodell und Zertifizierungs-Story**. ⇒ **ADR 0005
„Multi-Mandanten-Pivot"** ist Pflicht *vor* dem Bauen. Monetarisierung ist
orthogonal zur ASD-Mission und gehört **zuletzt** und **optional** (separate
Plane, §9).

### 5.5 Daten-Isolation ist das dominierende Sicherheitsrisiko
Cross-Tenant-Leckage (Frankfurt sieht Stuttgart) ist der Worst Case eines
sicherheitsrelevanten Lagebilds. Jede AuthZ-Entscheidung muss **server-seitig**
und **pro Nachricht/Subscription** fallen. Das macht aus dem Broadcaster
(„alle an alle") einen **scoped Broadcaster** („nur, was die Subscription
abdeckt") — S4–S5-Arbeit mit **Pflicht-Negativtests**.

---

## 6. Leitentscheidungen (zu ratifizieren — mit Empfehlung)

### 6.1 Mandanten-Modell (ADR 0005)
- **Variante A — Geteilter Himmel, mandanten-spezifische Sichten:** wenige Feeds;
  Mandant = server-seitiger Filter (AOI-Bounding-Box, FL-Band, Kategorie) +
  Branding. *Günstig, nah an heutiger Architektur, passt zu „mehrere Leitstellen
  über derselben Region".*
- **Variante B — Himmel pro Mandant:** je Mandant ein eigener Upstream-Feed
  (eigene Multicast-Gruppe / eigene Firefly-Instanz / eigenes Einzugsgebiet).
  *Schwerer; nötig bei verschiedenen Regionen/Sensoren je Kunde.*
- **✅ Gewählt (2026-06-19) — Hybrid:** **Feed-Katalog** (N Feeds) + Mandant
  **abonniert eine Teilmenge** und legt **Sicht-Filter** darüber. Subsumiert A und
  B, ist die einzige Variante, die mit Wachstum nicht bricht.

### 6.2 Persistenz & Identität (ADR 0006)
- **➡️ Empfehlung:** PostgreSQL als Konfig-/Identitäts-Store (wie im Entwurf);
  Zugriff über `pgx`/`sqlc` (typsicher, kein schweres ORM); Schema-Migrationen
  versioniert (`goose`/`migrate`). Optionaler **Redis-Cache** erst, wenn
  Messdaten einen DB-Hotspot je Session-Aufbau zeigen — nicht prophylaktisch.
  **App bleibt zustandslos**, *State wandert vollständig in DB + Stream.*

### 6.3 Cloud-Ingest & Fan-out (ADR 0007)
- **➡️ Empfehlung:** Abstrakte `FeedSource`-Schnittstelle mit zwei Implementierungen:
  (1) **direktes Multicast-Join** (On-Prem/Dev, = heute) und (2) **Stream-Consumer**
  (Cloud: NATS JetStream **oder** Kafka). Default-Empfehlung **NATS** (leichter,
  cloud-nativ, einfacheres Betriebsmodell) — Kafka nur, wenn Replay/Retention-
  Garantien das verlangen. Das Gateway ist ein **eigener kleiner Dienst**
  (mcast→Stream), kein Teil der Wayfinder-Instanz.

### 6.4 Sensor-/Stream-Modell (Teil von ADR 0005)
- **➡️ Empfehlung:** Sensor-*Mix* als **Feed-Metadatum** (z. B. „Feed A =
  ADS-B-only", „Feed B = PSR+SSR+ADS-B"); Entitlements binden an **Feeds**, nicht
  an Per-Track-Sensortypen. Aus dem Vertrag ableitbare Badges (ADS-B ◆, PSR)
  bleiben *ehrlich* als „track-abgeleitet" beschriftet. Echte Per-Track-Provenienz
  ⇒ Cross-Project-Issue an Firefly (§10).

### 6.5 Monetarisierung (ADR 0008, optional/zuletzt)
- **✅ Gewählt (2026-06-19): Feature-Flags ja, Stripe-Billing zurückgestellt.**
  **Feature-Flags als Daten** im Entitlement-Store (`tenant.HasFeature(...)`),
  **entkoppelt** von Billing — das hält das sicherheitsrelevante ASD frei von
  Billing-Kopplung. Eine etwaige spätere Stripe-Anbindung bleibt eine **separate
  Plane** (Webhook → Entitlement-Update); der ASD-Kern importiert **nie** Stripe.
  ⇒ In Stufe 5 ist **WF2-50 (Entitlements) aktiv**, **WF2-51 (Billing) ruht**.

---

## 7. Ausbaustufen & Arbeitspakete

Jede Stufe ist **für sich auslieferbar** und **de-riskt die nächste**. Reihenfolge
ist bewusst: erst entscheiden, dann Identität/Persistenz *neben* dem laufenden
Pfad, dann den **sicherheitskritischen** Datenstrom umbauen, dann Komfort
(Admin-UI), dann Sensor-Feinheit, zuletzt Kommerz/HA.

Legende je AP: **Fachlich** (warum) · **Technisch** (wie/Dateien) · **Stufe ·
Modell** · **Abhängig von** · 🔒 = sicherheitskritisch.

### Ausbaustufe 0 — Entscheidung & Fundament (reine ADRs, kein Produktivcode)
> Höchste Hebelwirkung, geringstes Risiko. **Kein Feature-Code, bis 0 steht.**

- **WF2-00 · ADR 0005 „Multi-Mandanten-Pivot"** — Produkt-Reframe ratifizieren,
  Mandanten-Variante (A/B/Hybrid) wählen, Vertrauens-/Isolationsgrenze und
  Zertifizierungs-Haltung der Kommerz-Plane festlegen.
  **S4 · Opus 4.8** · Abh.: — · 🔒
- **WF2-01 · ADR 0006 „Konfig-/Identitäts-Persistenz"** — Datastore (Postgres),
  Schema-Skizze (`tenants`, `users`, `sessions`, `feeds`, `subscriptions`,
  `view_configs`, `entitlements`), Migrations-Tooling, Stateless-Split.
  **S4 · Opus 4.8** · Abh.: WF2-00 · 🔒
- **WF2-02 · ADR 0007 „Cloud-Ingest & Feed-Fan-out"** — `FeedSource`-Abstraktion,
  Gateway-Dienst, Stream-Transport (NATS/Kafka vs direkt-Multicast).
  **S4 · Opus 4.8** · Abh.: WF2-00

### Ausbaustufe 1 — Identität & Mandanten-Grundgerüst (ohne Datenfluss-Änderung)
> Persistenz + Identität **neben** der bestehenden Pipeline einziehen; der
> Track-Fan-out bleibt vorerst unverändert. De-riskt DB + Auth, bevor der
> sicherheitskritische Strom angefasst wird.

- **WF2-10 · Persistenz-Schicht & Migrationen** — `pkg/store` (pgx/sqlc),
  Schema + Migrationen aus WF2-01, Repository-Funktionen, Testcontainer-Tests.
  **S3 · Sonnet 4.6** (mechanisch nach ADR), 🔒-Review durch Opus.
  Abh.: WF2-01
- **WF2-11 · AuthN: echtes Nutzer-/Session-Modell** — den einzelnen geteilten
  Token ablösen: OIDC am Reverse-Proxy + Session-Lookup (Empfehlung) **oder**
  eingebaute Nutzer; Tenant-Claim in der Session.
  **S4 · Opus 4.8** · Abh.: WF2-10 · 🔒
- **WF2-12 · Tenant-Context-Middleware** — jeder HTTP/WS-Request löst die
  Tenant-ID auf (fail-closed); `context.Context`-Propagation; Ablehnung ohne
  gültigen Mandanten.
  **S4 · Opus 4.8** · Abh.: WF2-11 · 🔒
- **WF2-13 · Admin-Bootstrap** — `create-tenant`/`create-user` (CLI/Seed) +
  minimaler `/admin`-Auth-Gate (nur Schutz, noch keine Funktionen).
  **S2–S3 · Sonnet 4.6** · Abh.: WF2-12

### Ausbaustufe 2 — Mandanten-isolierter Datenstrom (der sicherheitskritische Kern)
> Aus „alle an alle" wird „nur, was die Subscription abdeckt". **Herzstück.**

- **WF2-20 · Feed-Registry & Multi-Feed-Receiver** — heute 1 Feed → N Feeds;
  jeder dekodierte Track trägt seine `feed_id`; `receiver`/`broadcaster` um die
  Feed-Dimension erweitern.
  **S4 · Opus 4.8** · Abh.: WF2-01, WF2-02 · 🔒
- **WF2-21 · Subscription-Modell & scoped Fan-out** — `broadcast()` wird vom
  Broadcast zur **prädikat-gefilterten Zustellung**: ein Track geht an einen
  Client nur, wenn `feed ∈ subs ∧ AOI-Box ∧ FL-Band ∧ Kategorie`. Pro-Client-
  Prädikat statt globaler Nachricht.
  **S4–S5 · Opus 4.8 / Fable 5** · Abh.: WF2-12, WF2-20 · 🔒
- **WF2-22 · Isolations-Testsuite** — **Negativtests** (A bekommt *nie* B),
  Property-/Fuzz-Tests auf das Prädikat, Golden-Vektoren, Race-Detektor.
  Pflicht-Gate für diese Stufe.
  **S4 · Opus 4.8** · Abh.: WF2-21 · 🔒
- **WF2-23 · Pro-Mandant-Metriken & Audit-Log** — „wer sah welchen Scope"
  (Observability + Zertifizierungs-Rückverfolgbarkeit); `/metrics` um
  `tenant`-Label, strukturiertes Audit-Event.
  **S3 · Sonnet 4.6** · Abh.: WF2-21

### Ausbaustufe 3 — Dynamische Konfiguration & Admin-UI
> Jetzt, wo Config in der DB liegt und Ströme scoped sind: Laufzeit-Konfiguration.

- **WF2-30 · Config-Service (Hot-Reload)** — Tenant-/View-Config bei
  Session-Aufbau aus DB lesen (In-Proc-TTL-Cache, optional Redis); Änderungen
  ohne Neustart wirksam (Versioned-Poll oder Pub/Sub). Löst `loadConfig` für
  *mandanten-skopierte* Parameter ab (Prozess-Parameter bleiben ENV/12-Factor).
  **S3–S4 · Sonnet 4.6 / Opus 4.8** · Abh.: WF2-10
- **WF2-31 · Admin-API (REST)** — tenant-skopierte, validierte Endpunkte für
  Zentrum/Radius/FL-Bänder/Feed-Abos; serverseitige Validierung (kein Vertrauen
  ins Frontend).
  **S3 · Sonnet 4.6** · Abh.: WF2-30, WF2-13 · 🔒 (AuthZ je Endpoint)
- **WF2-32 · Admin-UI (`/admin`, Vue 3 + Vuetify)** — Formulare/Slider (passt zu
  ADR 0002); optimistisch + server-validiert; Live-Apply auf die Karte des
  Nutzers ohne Neustart.
  **S3 · Sonnet 4.6** · Abh.: WF2-31
- **WF2-33 · Live-Apply auf der Daten-Ebene** — Änderung von AOI/FL re-skopiert
  die **laufende** WS-Subscription ohne Reconnect (Prädikat neu auswerten).
  **S4 · Opus 4.8** · Abh.: WF2-21, WF2-31

### Ausbaustufe 4 — Sensor-/Stream-Management (innerhalb der CAT062-Realität)
> Epic 3 des Entwurfs, korrigiert auf das, was der Vertrag hergibt.

- **WF2-40 · Provenienz aus dem Vertrag als Sicht-Layer** — ADS-B (◆ existiert),
  PSR und (mehr von I062/080 dekodieren) als schaltbare, *ehrlich beschriftete*
  („track-abgeleitet") Layer.
  **S3 · Sonnet 4.6** · Abh.: WF2-32
- **WF2-41 · Feed-Sensorklassen-Katalog & Entitlements** — Feed-Metadaten
  („ADS-B-only" / „PSR+SSR+ADS-B"); Abo-Entitlements binden an Feeds.
  **S3 · Sonnet 4.6** · Abh.: WF2-20, WF2-50
- **WF2-42 · Cross-Project-Issue an Firefly** — falls *echte* Per-Track-Provenienz
  (FLARM-Diskriminator) gebraucht wird: ICD-Änderung anstoßen (`from-wayfinder`)
  + beidseitiges ADR.
  **S2 · Sonnet 4.6** (Doku-/Abstimm-Paket) · Abh.: WF2-40

### Ausbaustufe 5 — Monetarisierung & HA-Betrieb (optional / zuletzt)
> Bewusst außerhalb des zertifizierbaren ASD-Kerns.

- **WF2-50 · Feature-Entitlement-Service** — `tenant.HasFeature("psr_layer")`
  aus DB-Entitlements; Feature-Flags als Daten, **nicht** Stripe-gekoppelt.
  **S3 · Sonnet 4.6** · Abh.: WF2-10
- **WF2-51 · Billing-Adapter (Stripe) als separate Plane** *(zurückgestellt,
  §6.5 — derzeit nicht im Ziel)* — Webhook → Entitlement-Update; ASD-Kern bleibt
  Stripe-frei. Sekret-/PCI-Sorgfalt.
  **S3 · Sonnet 4.6** (mit 🔒-Review) · Abh.: WF2-50
- **WF2-52 · Stateless-Härtung & horizontale Skalierung** — bestätigen, dass kein
  node-lokaler State bleibt; Live-Lagebild je Feed extern/teilbar, sodass jede
  Instanz jeden Client bedient; LB ohne Sticky-Sessions; 3× parallel.
  **S4–S5 · Opus 4.8 / Fable 5** · Abh.: WF2-21
- **WF2-53 · Ingest-Gateway produktiv + HA** — mcast→Stream-Dienst gehärtet,
  Gateway selbst hochverfügbar (kein Single Point of Failure am Eingang).
  **S4–S5 · Opus 4.8 / Fable 5** · Abh.: WF2-02

---

## 8. Schwierigkeitsgrad → Modell (konsolidiert — die explizite Bitte)

> Lesart der Stufen für dieses Vorhaben: Die Einstufung schätzt die *Anspruchs­tiefe*
> (Architektur-Abwägung, subtile Logik, Testumfang), nicht die Zeilenzahl
> (CLAUDE.md §3). **Sicherheits- oder Schnittstellen-Wirkung hebt das Modell
> eine Stufe an** — deshalb stehen viele isolations-nahe S3-Pakete „mit
> Opus-Review".

| AP | Titel | Stufe | Modell | 🔒 | Begründung der Einstufung |
|----|-------|-------|--------|----|---------------------------|
| WF2-00 | ADR Multi-Mandanten-Pivot | **S4** | Opus 4.8 | 🔒 | Große, irreversible Strategie-/Threat-Model-Weiche |
| WF2-01 | ADR Persistenz/Identität | **S4** | Opus 4.8 | 🔒 | Datenmodell prägt alle Folgepakete |
| WF2-02 | ADR Cloud-Ingest/Fan-out | **S4** | Opus 4.8 | | Transport-/Verfügbarkeits-Abwägung, mehrere Optionen |
| WF2-10 | Persistenz-Schicht & Migrationen | **S3** | Sonnet 4.6 (+Opus-Review) | 🔒 | Mechanisch nach ADR, aber sicherheitsnah |
| WF2-11 | AuthN Nutzer/Session | **S4** | Opus 4.8 | 🔒 | Auth ist klassisch fehleranfällig |
| WF2-12 | Tenant-Context-Middleware | **S4** | Opus 4.8 | 🔒 | Fail-closed-Grenze, jede Request-Pfad-Ecke |
| WF2-13 | Admin-Bootstrap | **S2–S3** | Sonnet 4.6 | | Klar umrissen, wenig Logik |
| WF2-20 | Feed-Registry/Multi-Feed | **S4** | Opus 4.8 | 🔒 | Umbau am Eingang + Broadcaster-Datenmodell |
| WF2-21 | Scoped Fan-out (Prädikat) | **S4–S5** | Opus 4.8 / Fable 5 | 🔒 | Sicherheitskritisches Herzstück, subtile Logik |
| WF2-22 | Isolations-Testsuite | **S4** | Opus 4.8 | 🔒 | Negativ-/Property-/Fuzz-Tests korrekt entwerfen |
| WF2-23 | Pro-Mandant-Metriken/Audit | **S3** | Sonnet 4.6 | | Überschaubar, additiv |
| WF2-30 | Config-Service Hot-Reload | **S3–S4** | Sonnet 4.6 / Opus 4.8 | | Cache-Kohärenz/Race-Fragen |
| WF2-31 | Admin-API (REST) | **S3** | Sonnet 4.6 | 🔒 | Validierung + AuthZ je Endpoint |
| WF2-32 | Admin-UI (Vue) | **S3** | Sonnet 4.6 | | Bekanntes Vuetify-Muster (ADR 0002) |
| WF2-33 | Live-Apply Datenebene | **S4** | Opus 4.8 | 🔒 | Laufende Subscription sicher re-skopieren |
| WF2-40 | Provenienz-Sicht-Layer | **S3** | Sonnet 4.6 | | Frontend + etwas Decoder-Ausbau |
| WF2-41 | Feed-Sensorklassen/Entitlements | **S3** | Sonnet 4.6 | | Datenmodell + Verdrahtung |
| WF2-42 | Cross-Project-Issue Firefly | **S2** | Sonnet 4.6 | | Doku-/Abstimm-Paket |
| WF2-50 | Entitlement-Service | **S3** | Sonnet 4.6 | | Klar umrissen |
| WF2-51 | Billing-Adapter (Stripe) | **S3** | Sonnet 4.6 (+🔒-Review) | 🔒 | Sekret-/Webhook-Sorgfalt |
| WF2-52 | Stateless-Härtung/Skalierung | **S4–S5** | Opus 4.8 / Fable 5 | | Verteilter State, Konsistenz |
| WF2-53 | Ingest-Gateway HA | **S4–S5** | Opus 4.8 / Fable 5 | | Verfügbarkeit am Single-Eingang |

**Faustregel-Anwendung:** S1–S2 → Haiku; S3 → Sonnet 4.6; S4–S5 → Opus 4.8 /
Fable 5. **Jedes 🔒-Paket** läuft mindestens mit Opus-Review, auch wenn die
Grund-Einstufung S3 ist.

---

## 9. Querschnitts-Prinzipien (gelten über alle Stufen)

- **Sicherheit zuerst (🔒):** Daten-Isolation ist Gate-Zero jedes Pakets, das den
  Strom oder die Identität berührt. Server-seitige AuthZ, fail-closed, Negativtests
  verpflichtend. Geheimnisse (DB-Credentials, Stripe-Keys) nie an den Browser.
- **Rückverfolgbarkeit (Zertifizierungs-Fähigkeit):** neue Anforderungs-IDs im
  Register — vorgeschlagene Familien: `FR-TEN-xxx` (Tenancy), `FR-ADM-xxx`
  (Admin/Config), `FR-FEED-xxx` (Feeds/Subscriptions), `NFR-SEC-xxx`
  (Isolation), `NFR-SCALE-xxx` (HA/Skalierung). Anforderung → ADR → Code → Test.
- **12-Factor bleibt für Prozess-Parameter:** ENV/Secrets für *Infrastruktur*
  (DB-URL, Stream-Endpunkt); DB nur für *mandanten-/fachliche* Konfiguration.
  Der Entwurf-Satz „Config wandert ganz aus ENV in die DB" gilt **nur** für die
  fachliche Mandanten-Config, nicht für Infra-Secrets.
- **Observability/Betreibbarkeit:** jede Stufe erweitert `/metrics`, Logs, ggf.
  Grafana; `/ready` muss Mandanten-Pfad-Gesundheit einschließen.
- **Erst abstimmen, dann bauen:** jedes AP wird vor der Umsetzung einzeln
  angekündigt (Fachlich/Technisch/Stufe·Modell) und freigegeben.

---

## 10. Cross-Project-Wirkung (Firefly)

- **Per-Track-Sensor-Provenienz (FLARM/SSR/PSR/ADS-B-Diskriminator):** nur über
  eine **CAT062-ICD-Änderung** sauber lösbar. ⇒ `from-wayfinder`-Issue + ADR
  beidseitig (WF2-42). Bis dahin: Sensor-Mix auf **Feed-Ebene**.
- **Feed pro Mandant (Variante B/Hybrid):** falls Mandanten eigene Himmel
  brauchen, betrifft das Fireflys Multicast-Gruppen-/Instanz-Modell — Abstimmung
  über `docs/cross-project/`.
- **Zeitstempel:** `broadcast.go::timeNowMs()` liefert heute `0` (TODO). Für
  Audit-Log/Replay je Mandant sollte echte Daten-/Wall-Clock-Zeit gesetzt werden
  — kleiner, aber jetzt relevanter Altlast-Punkt.

---

## 11. Offene Entscheidungen (für die Abstimmung)

1. ✅ **ENTSCHIEDEN (2026-06-19) — Mandanten-Modell: Hybrid** (Feed-Katalog +
   Abos + Sicht-Filter; §6.1).
2. ✅ **ENTSCHIEDEN (2026-06-19) — Kommerz-Scope: Feature-Flags ja, Stripe-
   Billing zurückgestellt** (§6.5; Stufe 5 ohne Billing, WF2-51 ruht).
3. **Datastore/Cache:** PostgreSQL bestätigt? Redis erst bei gemessenem Bedarf
   (Empfehlung) oder von Anfang an?
4. **Identität:** OIDC am Reverse-Proxy (Empfehlung, cloud-nativ) oder
   eingebaute Nutzerverwaltung in Wayfinder?
5. **Sensor-Filterung:** CAT062-abgeleitete Näherung + Feed-Ebene akzeptieren
   (Empfehlung) — oder ICD-Änderung an Firefly für echte Per-Track-Provenienz
   anstoßen?
6. **Zielumgebung:** AWS/Azure/Hetzner/On-Prem-Kubernetes — bestimmt Ingest-
   Transport (NATS vs Kafka vs direkt-Multicast) in ADR 0007.

---

## 12. Empfohlene Reihenfolge & nächster Schritt

```
Stufe 0 (ADRs 0005–0007)   ──►  Stufe 1 (Identität/Persistenz)
   └─ Entscheidungen §11           └─ DB + Auth neben dem Pfad
                                         │
Stufe 2 (Scoped Stream) 🔒  ◄───────────┘   ← sicherheitskritischer Kern
   │
   ├──►  Stufe 3 (Config + Admin-UI)
   │         │
   │         └──►  Stufe 4 (Sensor-/Stream-Mgmt)
   │
   └──►  Stufe 5 (Monetarisierung/HA, optional, zuletzt)
```

**Begründung:** Entscheiden (0) vor Bauen; Identität/Persistenz (1) *neben* dem
laufenden Pfad de-riskt DB+Auth ohne den Strom anzufassen; der sicherheits­kritische
Umbau (2) folgt auf stabilisierter Basis mit Pflicht-Negativtests; Komfort (3),
Sensor-Feinheit (4) und Kommerz/HA (5) zuletzt.

**➡️ Nächster Schritt:** **WF2-00 — ADR 0005 „Multi-Mandanten-Pivot"** entwerfen,
sobald die offenen Entscheidungen (§11, v. a. #1 Mandanten-Modell und #2
Kommerz-Scope) abgestimmt sind. **S4 · Opus 4.8.** Kein Produktivcode vor dieser
Ratifizierung.
