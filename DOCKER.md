# Docker-Setup für Wayfinder

Wayfinder läuft in Containern — ideal für reproduzierbare Umgebungen,
Cloud-Deployment und das Zusammenspiel mit Firefly ohne lokale
Go-Installation.

## Schnellstart (Standalone)

```bash
docker-compose up
```

Dann im Browser: **http://localhost:8081**

Standalone empfängt Wayfinder nichts (keine Tracks), bis ein CAT062/UDP-
Multicast-Sender (z. B. Firefly) läuft — die Karte bleibt leer, aber leer und
betriebsbereit. `/health` und `/ready` (Port 8080) zeigen den Status.

## Zusammen mit Firefly testen (End-to-End)

Das ist der eigentliche Anwendungsfall: Firefly rechnet Tracks und sendet sie
als CAT062 (+ CAT065-Heartbeat) über UDP-Multicast, Wayfinder empfängt,
dekodiert und zeigt sie live auf der Karte (CAT062-Draht-Vertrag, `CLAUDE.md`
Abschnitt 2). Empfohlen wird das **Frankfurt-Szenario** (drei Radare, acht
Flugzeuge, JPDA/IMM-Manöver) — die kleine Demo-Szene ist für den E2E-Test zu
unauffällig, um wirklich etwas zu sehen.

### Linux — zwei Container, Host-Netzwerk

**Terminal 1 — Firefly** (eigenes Repo; CAT062-Multicast ist standardmäßig
**aus**, damit ein einfacher `cargo run`/`docker-compose up` keinen
Netzwerkverkehr erzeugt):

```bash
FIREFLY_SCENE=frankfurt FIREFLY_CAT062_ENABLED=true docker-compose up
```

(oder lokal: `FIREFLY_SCENE=frankfurt FIREFLY_CAT062_ENABLED=true cargo run -p firefly-server`)

**Terminal 2 — Wayfinder:**

```bash
docker-compose up
```

Dann im Browser: **http://localhost:8081** — nach wenigen Sekunden erscheinen
die ersten Tracks über dem Rhein-Main-Gebiet.

> ⚠️ **Multicast & Docker:** UDP-Multicast (`239.255.0.62:8600`) traversiert
> Docker's Standard-Bridge-Netz nicht. Beide `docker-compose.yml`-Dateien
> nutzen daher `network_mode: host` — funktioniert direkt unter **Linux**.

### macOS/Windows (Docker Desktop) — gemeinsames Bridge-Netzwerk

Unter Docker Desktop bindet `network_mode: host` nur an die interne
Linux-VM, nicht an den eigentlichen Rechner — zwei separat gestartete
`docker-compose up`-Stacks sehen sich dann nicht, die Karte bleibt leer.

Lösung: beide Repos als Geschwister-Ordner anlegen und über ein
**gemeinsames, übergeordnetes `docker-compose.yml`** mit eigenem
Bridge-Netzwerk starten:

```
radar-workspace/
├── firefly/              # Firefly-Repo (geklont)
├── wayfinder/             # Wayfinder-Repo (geklont)
└── docker-compose.yml     # Master-Compose, siehe unten
```

`radar-workspace/docker-compose.yml`:

```yaml
version: '3.8'

networks:
  radar-net:
    driver: bridge

services:
  firefly-server:
    build:
      context: ./firefly
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      FIREFLY_SCENE: frankfurt
      FIREFLY_CAT062_ENABLED: "true"
      RUST_LOG: info
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 3s
      retries: 3
      start_period: 5s
    networks:
      - radar-net
    restart: unless-stopped

  wayfinder:
    build:
      context: ./wayfinder
      dockerfile: Dockerfile
    ports:
      - "8081:8081"
    environment:
      FIREFLY_CAT062_GROUP: 239.255.0.62
      FIREFLY_CAT062_PORT: 8600
      WAYFINDER_MAP_CENTER_LAT: 50.0379
      WAYFINDER_MAP_CENTER_LON: 8.5622
      WAYFINDER_MAP_ZOOM: 8
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 3s
      retries: 3
      start_period: 5s
    networks:
      - radar-net
    depends_on:
      - firefly-server
    restart: unless-stopped
```

Starten:

```bash
cd radar-workspace
docker-compose up --build
```

Dann im Browser:
- **Firefly (Tracker-Status):** http://localhost:8080
- **Wayfinder (Live-Karte mit Tracks):** http://localhost:8081

Innerhalb von `radar-net` funktioniert das Multicast-Routing zwischen den
Containern problemlos; `network_mode: host` wird nicht benötigt. Nach wenigen
Sekunden erscheinen die ersten Tracks über dem Rhein-Main-Gebiet.

> Dieses Master-Compose ist eine **Ergänzung** für den E2E-Fall — für
> Einzelbetrieb (z. B. nur Firefly mit der Demo-Szene) weiterhin die
> jeweilige Standalone-`docker-compose.yml` im eigenen Repo verwenden.

## Details

### Dockerfile

**Multi-stage build:**
1. **Builder-Stage** (`golang:1.23-bookworm`): Lädt Module, kompiliert
   `cmd/wayfinder` statisch (`CGO_ENABLED=0`).
2. **Runtime-Stage** (`debian:bookworm-slim`): Minimal-Image mit nur dem
   Binary.

**Healthcheck:** Der Container prüft, ob der Server auf `/health` (Port 8080)
antwortet.

### docker-compose.yml

**Service `wayfinder`:**
- Netzwerk: `network_mode: host` (Multicast-Empfang, siehe oben)
- Ports (im Host-Netzwerk direkt erreichbar): `8080` (Health/Readiness),
  `8081` (WebSocket + ASD-Frontend)
- Umgebungsvariablen:
  - `FIREFLY_CAT062_GROUP` / `FIREFLY_CAT062_PORT`: Multicast-Quelle
    (Default: `239.255.0.62:8600`, Fireflys Default)
  - `WAYFINDER_MAP_CENTER_LAT/LON`, `WAYFINDER_MAP_ZOOM`: Karten-Ausschnitt
    (Default: Frankfurt, passend zu Fireflys Demo-Szene)
- Healthcheck: prüft alle 10 Sekunden
- Restart-Policy: `unless-stopped`

## Lokaler Build (ohne docker-compose)

```bash
docker build -t wayfinder:latest .
docker run --network host wayfinder:latest
```

## Cloud-Deployment

**12-Factor Config:**
- Alle Parameter via Env-Vars (`FIREFLY_CAT062_GROUP`, `WAYFINDER_*`)
- Graceful Shutdown via SIGTERM/SIGINT
- Strukturiertes JSON-Logging (stderr)
- `/health` (Liveness) und `/ready` (Readiness) für Kubernetes-Probes

In einer Cloud-Umgebung (Kubernetes etc.) ist `network_mode: host` meist nicht
verfügbar — dort empfängt Wayfinder den CAT062-Strom stattdessen z. B. über
einen Multicast-fähigen CNI/Underlay oder eine Unicast-Relay-Lösung
(offener Punkt, siehe Abschnitt 7 in `CLAUDE.md`: Feed-Authentizität/
Netz-Isolation ist ohnehin ein eigenes Thema).

## Troubleshooting

**Karte bleibt leer:**
- Läuft Firefly mit `FIREFLY_CAT062_ENABLED=true`?
- Linux: Sind beide Container im selben Host-Netzwerk (`network_mode: host`)?
- macOS/Windows: Läuft das Master-Compose mit `radar-net` (siehe oben) statt
  zwei separater Standalone-Stacks?
- Logs prüfen: `docker-compose logs wayfinder` — wird der Multicast-Socket
  erfolgreich geöffnet?

**Build schlägt fehl:**
- Docker-Daemon läuft? (`docker ps`)
- Genug Disk-Space für den Build vorhanden?

**Port bereits belegt:**
- `lsof -i :8080` / `lsof -i :8081`
