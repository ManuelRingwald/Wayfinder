# Ist-/Gap-Analyse: Service-Orientierung, Ausfallsicherheit & Redundanz (Firefly ↔ Wayfinder)

> **Stand:** 2026-07-03 · **Status:** Analyse (Momentaufnahme, keine Entscheidung)
> **Anlass:** Frage des Projektverantwortlichen: *„Viele Projekte arbeiten in
> unabhängigen Services (z. B. je ein Service für DWD-Daten, Luftlage-Ebenen,
> Firefly-Empfang), die unabhängig deployt und ausgetauscht werden können.
> Inwieweit trifft das heute auf Wayfinder und Firefly zu? Ist es sinnvoll,
> diesen Gedanken für ein produktions-taugliches, cloud-natives Produkt mit
> Ausfallsicherheit und Redundanz weiter zu verankern — und was würde das
> bedeuten?"*
>
> **Einordnung im Backlog:** Die identifizierten Lücken sind **keine neuen
> Arbeitspakete**, sondern schärfen bestehende: **WF2-52/53** (Stateless-Härtung,
> Ingest-HA), **ORCH-6** (K8s-Backend/HA) und Fireflys **SDPS-002** (HA & State
> Sync, Roadmap §3 #20). Dieses Dokument liefert die Gesamtsicht und eine
> empfohlene Reihenfolge. Umsetzung je Paket nach dem üblichen
> Ankündigen-→-Freigabe-Ablauf (CLAUDE.md §3).

---

## 1. Kurzfazit

- **Auf System-Ebene** (Firefly ↔ Wayfinder) ist der Service-Gedanke bereits
  konsequent umgesetzt: zwei unabhängig baubare, testbare, deploybare und
  **austauschbare** Systeme, verbunden ausschließlich über den versionierten
  **CAT062-Draht-Vertrag** (ICD). Dazu physische Quell-Trennung („1 Feed =
  1 Multicast-Gruppe = 1 Firefly-Instanz") und eine bereits vollzogene,
  sicherheitsmotivierte Service-Abspaltung (Orchestrator-Control-Plane,
  ADR 0012).
- **Innerhalb** der Systeme sind beide **modulare Monolithen**: je ein Prozess,
  der alle Aufgaben als interne Tasks/Goroutinen erledigt. Die Modul-Schnitte
  (Nahtstellen) für eine spätere Aufteilung sind vorbereitet, aber bewusst
  nicht vollzogen.
- **Für Ausfallsicherheit und Redundanz ist nicht „mehr Services" der Hebel**,
  sondern: Mehrfach-Instanzen, wiederherstellbarer Zustand und ein
  Orchestrator, der Ausfälle erkennt und heilt. Ein verfrühter
  Microservice-Schnitt würde Betriebs-Komplexität einführen, ohne die
  Verfügbarkeit zu verbessern.

---

## 2. Ist-Zustand

### 2.1 System-Ebene: Service-Architektur ist da ✅

| Merkmal | Umsetzung |
|---|---|
| Kein gemeinsamer Code, keine Punkt-zu-Punkt-Kopplung | CAT062/065/063 über UDP-Multicast; Sender kennt Empfänger nicht (Firefly ADR 0006/0014) |
| Versionierter, dokumentierter Vertrag | `docs/ICD-CAT062.md` (Firefly, v2.6.0) + Quell-Eingangs-Kontrakt `FIREFLY_SOURCES` (ADR 0023, v1.4.0) |
| Austauschbarkeit | Jeder ICD-konforme Tracker könnte Firefly ersetzen; jeder ICD-konforme Konsument Wayfinder |
| Physische Quell-Trennung | 1 Feed = 1 Multicast-Gruppe = 1 Firefly-Instanz (Orchestrator, ADR 0012) |
| Vollzogene Service-Abspaltung | `cmd/wayfinder-orchestrator` als eigener Least-Privilege-Prozess (einziger mit Docker-Socket); Kopplung an den ASD-Server **nur** über die DB (Soll-Zustand) |

### 2.2 Firefly: modularer Monolith (Rust)

- **Struktur:** 13 Crates, sauberes Ports-&-Adapters-Schichtenmodell. Reiner
  Tracker-Kern (`firefly-track`: keine IO-/Netz-/Uhr-Abhängigkeit,
  deterministisch nach Datenzeit), getrennte Eingangs-Adapter (`firefly-opensky`,
  `firefly-flarm`, `firefly-radar`), getrennte Ausgangs-Adapter
  (`firefly-asterix`, `firefly-multicast`, `firefly-io`).
- **Laufzeit:** **ein** operatives Binary (`firefly-server`); alle Adapter und
  der Tracker laufen als Tokio-Tasks in einem Prozess, verbunden über interne
  Kanäle (mpsc für Plots, watch für Snapshots). Composition Root:
  `crates/firefly-server/src/main.rs`.
- **Zustand:** Track-Zustand nur im Arbeitsspeicher. Die Tracker-Strukturen
  sind zwar `Serialize`/`Deserialize`, aber **kein Codepfad persistiert oder
  restauriert** sie. Wiederherstellungs-Konzept ist deterministisches
  **Replay der Eingangs-Aufzeichnung** (`.ffplots`, ADR 0020) — **Befund:** im
  Live-Pfad ist der `PlotRecorder` derzeit **nicht angeschlossen**
  (`main.rs:329`: `LiveTracker::new(tracker, None)`), d. h. auch dieser
  Wiederherstellungs-Weg ist im Default-Livebetrieb nicht aktiv.
- **Deployment:** ein Container (Dockerfile), `docker-compose.yml` mit einem
  Service; `/health`, `/ready` (503 bis zum ersten erfolgreichen Poll),
  SIGTERM-Handling, `/metrics` — die K8s-*Vorarbeiten* sind komplett, aber
  **keine K8s-Manifeste** im Repo. Skalierung = externes Sharding (eine
  Instanz pro Feed), keine Redundanz je Feed.

### 2.3 Wayfinder: modularer Monolith + Control-Plane (Go)

- **Struktur:** ~28 Pakete, interface-getrieben an den richtigen Stellen
  (`feedmanager.Receiver`/`Factory`, `instance.Backend` mit Memory-/Docker-
  Adapter, `broadcast.Scope`/`ws.ScopeResolver`, Repo-Interfaces in
  `pkg/store`).
- **Der Service-Gedanke aus der Ausgangsfrage existiert modulweise bereits:**
  je ein gekapseltes Modul für **DWD-Regenradar** (`pkg/weathertiles`,
  WMS-Tile-Proxy), **DWD-Wetterwarnungen** (`pkg/weatherwarnings`),
  **QNH/METAR** (`pkg/weather`), **OpenAIP-Luftraum-Ebenen**
  (`pkg/aeronautical`) und **Firefly-Feed-Empfang** (`pkg/receiver` +
  `pkg/cat062/065/063` + `pkg/feedmanager`). Alle laufen **im selben Prozess**
  (`cmd/wayfinder`), sind aber best-effort und außerhalb des Track-Pfads —
  ein DWD-Ausfall stört die Luftlage nicht.
- **Zustand:** Konfiguration/Mandanten/Feeds/Sessions persistent in PostgreSQL;
  Tracks selbst flüchtig (kein Server-Track-Store) — nach Neustart rehydriert
  das Bild aus dem laufenden Multicast. Ohne fixen `WAYFINDER_SESSION_KEY`
  ephemerer Signierschlüssel → **nicht multi-replica-fähig** (Warnung in
  `main.go`).
- **Deployment:** zwei Container (ASD + Orchestrator), drei Compose-Varianten
  (Onboarding/Orchestrated/Bridge — Orchestrated ausdrücklich „single-host
  acceptance harness, nicht Produktions-Topologie"). **Keine K8s-Manifeste.**
  `TECHNICAL.md` (Grenzen): *nicht für horizontale Skalierung ausgelegt* —
  jede Instanz hält ihren eigenen WebSocket-State.

---

## 3. Gap-Analyse

Maßstab: „Produktion, cloud-nativ, ausfallsicher, redundant" (Charter §7/§8;
Firefly ADR 0003).

| # | Dimension | Ist | Lücke | Backlog-Anker |
|---|-----------|-----|-------|---------------|
| 1 | **Redundanz Firefly (pro Feed)** | 1 Instanz je Feed = Single Point of Failure; nach Crash Neustart durch Orchestrator, Lagebild baut sich über mehrere Umläufe neu auf | **groß** — wichtigste betriebliche Lücke | Firefly **SDPS-002** (§3 #20) |
| 2 | **Zustands-Wiederherstellung Firefly** | Konzept vorhanden (ADR 0020: `.ffplots`-Replay), aber Recorder im Live-Pfad nicht verdrahtet; kein Snapshot/Restore | **mittel–groß** | Firefly SDPS-002-Vorstufe (Recorder-Verdrahtung; ggf. Snapshot) |
| 3 | **Redundanz Wayfinder-ASD** | Explizit Single-Node; Session-Key-Vorbehalt; kein LB-Konzept für `/ws` | **mittel** — günstigste Frucht: ASD ist fast zustandslos (Tracks aus Multicast, Config aus DB), N Replikas sind erreichbar | **WF2-52** |
| 4 | **Ingest-HA / kein SPOF am Eingang** | Multicast-Fan-out trägt Multi-Consumer nativ; Ingest-Gateway/Stream-Bus (NATS, ADR 0007) nicht gebaut | **mittel** — erst relevant jenseits Subnetz-Multicast (Public Cloud) | **WF2-53** |
| 5 | **Kubernetes/Orchestrierung** | Keine Manifeste in beiden Repos; Probes/SIGTERM/12-Factor/kleine Images komplett vorbereitet; `instance.Backend`-Naht für K8s-Adapter existiert | **mittel** — Vorarbeiten fertig, Schlussstein fehlt | **ORCH-6** |
| 6 | **Interne Service-Schnitte** (DWD, Aero, Ingest …) | In-Process-Module, sauber gekapselt, Nahtstellen (Interfaces) vorhanden | **klein / bewusst offen** — kein aktueller Treiber; Schnitt jederzeit nachholbar | — (bei Bedarf neues AP) |
| 7 | **Ende-zu-Ende-HA** | beidseitig als offen registriert, nicht begonnen | folgt aus 1–5 | WF2-52/53 ↔ SDPS-002, ORCH-6 |

---

## 4. Bewertung: Weiter verankern — ja, aber über Redundanz, nicht über Zerlegung

1. **„Modularer Monolith mit harten Verträgen" ist hier Stärke, nicht
   Versäumnis.** Verfügbarkeit entsteht durch Mehrfach-Instanzen +
   wiederherstellbaren Zustand + heilende Orchestrierung — nicht durch das
   Zersägen eines Prozesses in fünf (die dann fünf Single Points of Failure
   sind und Netzwerk-Fehlermodi, Versionierung und Betriebs-Aufwand
   mitbringen).
2. **Service-Schnitte nur mit konkretem Treiber** (unabhängige Skalierung,
   Fehler-Isolation, Sicherheit, Team-Grenzen). Genau so ist es bisher
   gehandhabt worden: der einzige vollzogene Schnitt (Orchestrator) hat einen
   Sicherheits-Treiber. Die übrigen Nahtstellen (Feed-Receiver, Wetter-/
   Aero-Module, Quell-Adapter, `instance.Backend`) sind vorbereitet und können
   bei Bedarf ohne Architektur-Bruch herausgelöst werden.
3. **Empfohlene Reihenfolge** (jeweils eigener Ankündigen-→-Go-Zyklus, ggf.
   eigene ADRs):

| Schritt | Inhalt | Repo | Stufe · Modell | Anker |
|---|--------|------|----------------|-------|
| 1 | **ASD multi-replica-fähig:** fixer `WAYFINDER_SESSION_KEY` als Betriebs-Pflicht, Rescope/Live-Apply über Replikas (DB LISTEN/NOTIFY o. ä.), LB-/`/ws`-Konzept, Doku | Wayfinder | S3 · Sonnet 4.6 | WF2-52 (Teil 1) |
| 2 | **Firefly-Zustands-Story schließen:** `PlotRecorder` im Live-Pfad verdrahten (ADR 0020 zu Ende), Rotation/Retention klären; optional periodischer Tracker-Snapshot (Strukturen sind bereits serialisierbar) | Firefly | S3–S4 · Sonnet 4.6 / Opus 4.8 | SDPS-002-Vorstufe |
| 3 | **Firefly-Redundanz pro Feed:** Design-Entscheidung aktiv/aktiv (Empfänger wählt lebenden Strom via CAT065/063) vs. Standby-Failover durch den Orchestrator → **eigener ADR, beidseitig abgestimmt** | beide | S4–S5 · Fable 5 / Opus 4.8 | SDPS-002 ↔ WF2-52/53 |
| 4 | **K8s-Deployment + K8s-`instance.Backend`:** Manifeste/Helm für beide Systeme; dritter Backend-Adapter im Orchestrator; K8s übernimmt Restart/Verteilung/Rolling Update | beide | S3–S4 · Sonnet 4.6 / Opus 4.8 | ORCH-6 |
| 5 | **Interne Service-Schnitte** (Wetter/Aero/Ingest als eigene Deployables) **nur bei konkretem Treiber** | Wayfinder | — | zurückgestellt |

---

## 5. Quellen / Belege (Auswahl)

- **Firefly:** `crates/firefly-server/src/main.rs` (Verdrahtung; Zeile 329
  Recorder=None), `src/live.rs` (Tracker-Task, `PlotRecorder`),
  `crates/firefly-track/src/tracker.rs` (In-Memory-Zustand), `Dockerfile`/
  `docker-compose.yml`, ADRs 0003 (cloud-nativ, Soll), 0006 (Ports & Adapters),
  0013, 0014, 0020 (Live-Modus/Replay), 0023 (Quell-Kontrakt),
  `docs/TECHNICAL.md`.
- **Wayfinder:** `cmd/wayfinder/main.go` (Verdrahtung, Session-Key-Warnung),
  `cmd/wayfinder-orchestrator/main.go`, `pkg/feedmanager`, `pkg/broadcast`,
  `pkg/instance`/`pkg/dockerbackend`, `pkg/weathertiles`/`weatherwarnings`/
  `weather`/`aeronautical`, `docker-compose.*.yml`, ADRs 0001, 0007 (NATS als
  Option), 0012 (Control-Plane-Grenze), 0014 (Multi-Tenant),
  `docs/TECHNICAL.md` (Grenzen: Single-Node).
