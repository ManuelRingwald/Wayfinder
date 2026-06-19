# ADR 0007 — Wayfinder 2.0: Cloud-Ingest & Feed-Fan-out

- **Status:** akzeptiert (Architektur + Bus-Wahl entschieden; Umsetzung/Härtung
  folgt WF2-20 und WF2-52/53)
- **Datum:** 2026-06-19
- **Schnittstellen-relevant:** nein (der CAT062/CAT065-Draht-Vertrag bleibt
  unverändert — das Gateway transportiert die **Roh-Datagramme** unverändert).
  Hinweis: Das Hybrid-Mehr-Feed-Modell (ADR 0005) kann bedeuten, dass das Gateway
  **mehrere** Multicast-Gruppen beitritt; das ist Gateway-Konfiguration und
  **keine** Wire-Format-Änderung (Cross-Project-Vorwarnung in
  `docs/cross-project/todo-for-firefly.md`).
- **Bezug:** schließt **Stufe 0** ab; baut auf ADR 0005 (Hybrid-Feeds) und ADR
  0006 (Stateless-Split) auf; entscheidet den in beiden vertagten Ingest-/
  Transport-Punkt (Konzept §5.3/§6.3/§11.6). **Zielumgebung: Public Cloud +
  Kubernetes** (vom Projektverantwortlichen gesetzt).

## Kontext

Wayfinder 2.0 läuft als **zustandslose Instanzen in Public Cloud + Kubernetes**
(ADR 0006). Daraus folgt ein hartes Problem (Konzept §5.3):

- **UDP-Multicast überquert keine Cloud-Subnetze.** VPCs der großen Anbieter
  routen Multicast nicht, und die meisten Kubernetes-CNIs unterstützen es nicht
  zuverlässig. Cloud-Pods können der CAT062/065-Multicast-Gruppe also **nicht
  direkt** beitreten, wie es `pkg/receiver` heute on-prem tut.
- Gleichzeitig soll der **On-Prem-/Dev-Betrieb** (direkter Multicast-Empfang,
  degenerierter Single-Tenant-Fall, ADR 0005 §7) erhalten bleiben.
- Das Live-Lagebild muss an **N zustandslose Instanzen** verteilt werden (jede
  filtert dann pro Mandant, WF2-21) — ein **Fan-out an alle**, keine Lastverteilung.

Es braucht eine Brücke von „dort, wo der Multicast lebt" (on-prem nahe Firefly/
Sensoren bzw. ein Bridging-Segment) in die Cloud, plus die Wahl des Transports.

## Entscheidung

### 1. `FeedSource`-Abstraktion

Ein Go-Interface, das dekodierbare CAT062/065-Datagramme liefert — **unabhängig
von der Herkunft**. Zwei Implementierungen:

- **`MulticastFeedSource`** — tritt der Multicast-Gruppe direkt bei (heutiges
  `pkg/receiver`-Verhalten). Für **On-Prem/Subnetz und Dev** (degenerierter
  Single-Tenant-Fall).
- **`StreamFeedSource`** — konsumiert vom **Stream-Bus** (Cloud).

Der Rest der Pipeline (Decoder → scoped Broadcaster) bleibt von der Quelle
**unberührt**. Auswahl via `WAYFINDER_FEED_SOURCE=multicast|nats`.

### 2. Ingest-Gateway (eigener, minimaler Dienst)

Ein **separater Deployable** (`cmd/wayfinder-ingest`), der dort läuft, wo der
Multicast erreichbar ist (on-prem/Bridge), der **CAT062/065-Gruppe(n) beitritt**
und **jedes Datagramm unverändert auf den Bus republisht**:

- **Pass-through der Roh-Bytes** — das Gateway **dekodiert nicht** (bleibt simpel/
  robust); ein Datagramm = eine Bus-Nachricht.
- **Subject pro Feed** (siehe 3); das Gateway ist je Feed konfiguriert, *welche
  Multicast-Gruppe → welches Subject* (realisiert das Hybrid-Mehr-Feed-Modell).
- Das Gateway ist **nicht** Teil der servierenden Wayfinder-Instanz — es ist die
  einzige Komponente, die Multicast berühren muss.

### 3. Stream-Bus = **NATS JetStream**

- **Subject-Schema:** `wayfinder.feed.<feed_id>` (ein Subject je Feed, trägt
  beide Kategorien; die servierende Instanz **dispatcht wie heute am führenden
  CAT-Oktett** `0x3E`/`0x41`). Optionale Trennung `…​.cat062`/`.cat065` möglich,
  aber nicht nötig.
- **Core-Fan-out** für das Live-Bild: jede Wayfinder-Instanz subscribt die
  Subjects ihrer Feeds und bekommt **jedes** Datagramm (Subject-Semantik =
  natives „jede Instanz sieht alles").
- **JetStream** liefert ein **kurzes Retention-Fenster** (Puffer/Late-Join: eine
  neu gestartete Instanz holt die letzten Sekunden nach). Der **revisionssichere
  Replay bleibt bei Firefly** (SDPS-005) — der Bus ist **nicht** das
  System-of-Record-Log.
- **Nachricht trägt das Roh-ASTERIX-Datagramm** als Body; ein **NATS-Header**
  trägt nur Ingest-Zeitstempel (für den `timeNowMs()`-TODO/Audit, WF2-23). Keine
  Re-Kodierung → der CAT-Vertrag bleibt Ende-zu-Ende erhalten, der robuste
  Decoder bleibt der **einzige** Parse-Punkt.

### 4. Deployment (Public Cloud + Kubernetes)

- **NATS** als **Managed-Angebot** wo verfügbar, sonst **NATS-Helm/Operator**
  (3-Knoten-Cluster für HA) im selben Cluster.
- **Gateway** als Deployment nahe der Multicast-Quelle (on-prem/Bridge), publisht
  in den Bus.
- **Wayfinder-Instanzen** als Deployment hinter Service/Ingress, **N Replicas,
  zustandslos** (WF2-52), je `StreamFeedSource`.

### 5. Sicherheit

- Bus-Verkehr **privat/in-Cluster**; **NATS mit TLS + Credentials** (NKey/JWT).
- Die CAT062-Vertrauensgrenze (ADR 0003: Netz-Isolation) gilt jetzt am
  **Multicast-Bein des Gateways**; das Bus-Bein ist durch NATS-Auth/TLS gesichert.
- **Secrets in ENV** (ADR 0006 §6): `WAYFINDER_NATS_URL`, `WAYFINDER_NATS_CREDS`,
  `WAYFINDER_FEED_SOURCE`.

### 6. Abgrenzung

- Konkreter **scoped-Broadcast-Prädikat-Code** → WF2-21.
- **Produktive Härtung/HA** von Gateway und Bus (kein SPOF am Eingang) → WF2-53.
- Erste **Umsetzung** der `FeedSource`/Multi-Feed-Registry → WF2-20.

## Begründung

- **`FeedSource`** hält On-Prem/Dev (direkter Multicast) und Cloud (Stream)
  gleichrangig, ohne den Kern zu gabeln.
- **Gateway als separater Minimal-Dienst** ist die einzige Multicast-berührende
  Komponente; alles andere ist Unicast/Bus → cloud-portabel.
- **NATS JetStream** (Auswahl bestätigt nach Abgleich, siehe Alternativen): K8s-
  nativ, niedrigste Latenz, **native Subject-Fan-out-Semantik** passend zu „jede
  Instanz sieht alles", leichter Betrieb; JetStream gibt optionalen Puffer, **ohne**
  den Bus zum System-of-Record zu machen (Firefly besitzt Replay).
- **Roh-Datagramm über den Bus** bewahrt den Draht-Vertrag und hält **einen**
  Decodier-Punkt.

### Verworfene Alternativen (inkl. der geprüften RabbitMQ-vs-Kafka-Frage)

Lastprofil: kleine Datagramme, moderate Rate (Scans 4–12 s, 1 s Heartbeat),
**Fan-out an alle** zustandslosen Instanzen, niedrige Latenz; **durabler Replay
ist bereits Fireflys Job** (SDPS-005) — der Bus muss kein revisionssicheres Log sein.

- **Direkter Multicast-Join in der Cloud:** unzuverlässig (VPC/CNI unterstützen
  UDP-Multicast i. d. R. nicht). Verworfen für Cloud; bleibt als
  `MulticastFeedSource` für On-Prem/Dev.
- **Kafka:** seine Stärken — durables Log, Massendurchsatz, Replay — sind hier
  **ungenutzt** (Firefly liefert Replay; Rate moderat). Seine Kosten sind real:
  **schwerste K8s-Ops** (Broker + Storage, KRaft/Strimzi), höhere Latenz, und
  **Fan-out-an-alle braucht eine Consumer-Group pro Instanz** (gegen den Strich).
  Überdimensioniert. Verworfen — bliebe nur denkbar, falls der **Bus selbst** zum
  revisionssicheren Log werden müsste.
- **RabbitMQ:** guter Live-Fan-out (fanout/topic-Exchange + exklusive Queue je
  Instanz), mature, ausreichender Durchsatz. **Zwischen RabbitMQ und Kafka klar
  RabbitMQ** für dieses Profil (leichter, latenzärmer, einfacheres Routing). Aber
  gegen **NATS** unterlegen: höhere Ops für HA (Quorum-Queues), kein K8s-nativer
  Footprint-Vorteil, Fan-out weniger elegant (Queue-je-Instanz statt Subject).
  Verworfen zugunsten NATS — **bleibt der dokumentierte AMQP-Fallback**, falls der
  Betrieb bereits auf RabbitMQ standardisiert ist.
- **Gateway dekodiert und sendet JSON statt Roh-ASTERIX:** bricht das „ein
  Decodier-Punkt"-Prinzip, doppelte Kodierung, Vertrags-Drift. Verworfen — der Bus
  trägt Roh-Datagramme.

## Konsequenzen

- **Neue Anforderungen im Register** (`docs/requirements/`):
  - **FR-FEED-001** — `FeedSource`-Abstraktion (`MulticastFeedSource` vs.
    `StreamFeedSource`), **Ingest-Gateway** (Multicast→NATS, Roh-Datagramm,
    Subject pro Feed), Multi-Feed-Ingest. Implementierung folgt WF2-20 (Registry/
    Source) und WF2-53 (Gateway produktiv).
  - **NFR-SCALE-001** — Cloud-Ingest/Fan-out auf Public Cloud + K8s: **NATS-
    JetStream**-Fan-out an N **zustandslose** Instanzen, Gateway als eigener
    Dienst, TLS/Credentials, Secrets in ENV, optionaler JetStream-Puffer (kein
    System-of-Record). Implementierung folgt WF2-52/53.
- **Neue ENV-Variablen** (`WAYFINDER_FEED_SOURCE`, `WAYFINDER_NATS_URL`,
  `WAYFINDER_NATS_CREDS`) kommen in `INSTALLATION.md`/`TECHNICAL.md`, **sobald
  WF2-20/53 sie einlesen** (heute wirkungslos).
- **`timeNowMs()`-TODO** (`broadcast.go`): der Ingest-Zeitstempel-Header gibt eine
  natürliche Quelle für echte Nachrichten-Zeit (WF2-23-Audit).
- **Cross-Project:** Mehr-Feed kann mehrere Multicast-Gruppen bedeuten; das
  Gateway abstrahiert das (es tritt N Gruppen bei) — **Firefly braucht dafür
  voraussichtlich keine Änderung**. Bleibt als Vorwarnung notiert.
- **Stufe 0 ist abgeschlossen** (ADR 0005/0006/0007). **Nächster Schritt = WF2-10**
  (Beginn Stufe 1, **erstes Produktivcode-Paket**: Persistenz-Schicht/Migrationen).

## Ehrliche Grenze

- Die Bus-Wahl ist eine **Architektur-Entscheidung**; die echte Bestätigung
  (Latenz/Last/HA) liefert erst ein **Pilot/Benchmark** bei WF2-53.
- Das **Gateway wird kritischer Pfad** (der einzige Cloud-Eingang des Feeds); sein
  **HA/kein-SPOF** ist WF2-53, nicht durch diese ADR garantiert.
- Auf dem **Multicast-Bein** des Gateways gilt weiterhin die Netz-Isolation aus
  ADR 0003 — der Bus ersetzt sie nicht, er verlängert sie nur unicast-seitig.
- Die formale Zertifizierung bleibt außerhalb des Code-Projekts (CLAUDE.md §7).
