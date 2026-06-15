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
als CAT062 über UDP-Multicast, Wayfinder empfängt, dekodiert und zeigt sie live
auf der Karte (CAT062-Draht-Vertrag, `CLAUDE.md` Abschnitt 2).

**Terminal 1 — Firefly** (eigenes Repo; CAT062-Multicast ist standardmäßig
**aus**, damit ein einfacher `cargo run`/`docker-compose up` keinen
Netzwerkverkehr erzeugt):

```bash
FIREFLY_CAT062_ENABLED=true docker-compose up
```

(oder lokal: `FIREFLY_CAT062_ENABLED=true cargo run -p firefly-server`)

**Terminal 2 — Wayfinder:**

```bash
docker-compose up
```

Dann im Browser: **http://localhost:8081** — die von Firefly berechneten
Tracks erscheinen live auf der Karte.

> ⚠️ **Multicast & Docker:** UDP-Multicast (`239.255.0.62:8600`) traversiert
> Docker's Standard-Bridge-Netz nicht. Beide `docker-compose.yml`-Dateien
> nutzen daher `network_mode: host` — funktioniert direkt unter **Linux**.
> Unter **macOS/Windows (Docker Desktop)** ist Host-Networking eingeschränkt;
> dort empfiehlt sich der lokale Weg (`cargo run` / `go run`) für den
> End-to-End-Test, siehe [README.md](README.md).

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
- Sind beide Container im selben Host-Netzwerk (`network_mode: host`)?
- Logs prüfen: `docker-compose logs wayfinder` — wird der Multicast-Socket
  erfolgreich geöffnet?

**Build schlägt fehl:**
- Docker-Daemon läuft? (`docker ps`)
- Genug Disk-Space für den Build vorhanden?

**Port bereits belegt:**
- `lsof -i :8080` / `lsof -i :8081`
