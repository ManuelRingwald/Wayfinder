# Docker-Setup für Wayfinder

Wayfinder läuft in Containern — ideal für reproduzierbare Umgebungen,
Cloud-Deployment und das Zusammenspiel mit Firefly ohne lokale
Go-Installation.

> **Multi-Tenant ist der einzige Betriebsmodus (ADR 0014).** Jeder Start
> braucht eine PostgreSQL-Datenbank (`WAYFINDER_DB_URL`) und hat die Anmeldung
> aktiv (`builtin` oder `proxy`). Einen DB-losen „Standalone ohne Login" gibt
> es nicht mehr — die mitgelieferten Compose-Stacks bringen Postgres direkt mit.

## Schnellstart — Multi-Tenant-Plattform

Der Standard-Stack (`docker-compose.onboarding.yml`) fährt Postgres **und** den
Wayfinder-Server im `builtin`-Login-Modus mit einem Befehl hoch:

```bash
docker compose -f docker-compose.onboarding.yml up --build
```

Dann im Browser: **http://localhost:8081/admin**. Beim ersten Start werden ein
Default-Mandant und ein Default-Admin (`admin` / `admin`) automatisch angelegt
(Zero-Touch, ADR 0011); der erzwungene Passwortwechsel kommt sofort. Kein
`bootstrap`- oder `feed add`-Schritt im Terminal nötig.

Der Stack nutzt **Bridge-Networking** und funktioniert daher auf **Linux, macOS
und Windows** gleichermaßen. `/health` und `/ready` (Port 8080) zeigen den
Status.

> **Karte bleibt zunächst leer.** Bridge-Netze transportieren kein UDP-Multicast,
> der Onboarding-Stack empfängt also ohne weitere Konfiguration noch keine
> Tracks. Für Live-Tracks siehe den nächsten Abschnitt.

## Live-Tracks End-to-End (mit Firefly)

Firefly rechnet Tracks und sendet sie als CAT062 (+ CAT065-Heartbeat) über
UDP-Multicast; Wayfinder empfängt, dekodiert und zeigt sie live auf der Karte
(CAT062-Draht-Vertrag, `CLAUDE.md` Abschnitt 2). Empfohlen wird das
**Frankfurt-Szenario** (drei Radare, acht Flugzeuge, JPDA/IMM-Manöver) — die
kleine Demo-Szene ist für den E2E-Test zu unauffällig, um wirklich etwas zu
sehen.

### Linux — orchestrierter Stack (empfohlen)

`docker-compose.orchestrated.yml` vereint Postgres, den Wayfinder-Server
(`builtin`-Auth) und die **Orchestrator-Steuerebene** in einem Stack. Der
Orchestrator startet je abonniertem Feed automatisch einen Firefly-Tracker;
über `network_mode: host` trifft der Multicast direkt beim Server ein:

```bash
docker compose -f docker-compose.orchestrated.yml up --build
```

Die vollständige Abnahme (Feed zuweisen → Spawn → Tracks → Aufräumen) ist in
`docs/E2E-ABNAHME.md` beschrieben; `scripts/e2e-orchestrated.sh` automatisiert
den Großteil.

> ⚠️ **Multicast & Docker:** UDP-Multicast (`239.255.0.62:8600`) traversiert
> Docker's Standard-Bridge-Netz nicht. Der orchestrierte Stack nutzt daher
> `network_mode: host` und braucht damit **Linux**.
>
> ⚠️ **Docker-Socket:** Nur der Orchestrator-Container bindet
> `/var/run/docker.sock` (root-äquivalent, ADR 0012 §6). Der browser-zugewandte
> Server bekommt diese Privilegierung nie. Den Orchestrator-Host als
> hochwertige Vertrauensgrenze behandeln (Netz-Isolation, restriktiver Zugang).

### macOS/Windows (Docker Desktop) — gemeinsames Bridge-Netzwerk

Unter Docker Desktop bindet `network_mode: host` nur an die interne Linux-VM,
nicht an den eigentlichen Rechner — der host-net-Orchestrator-Stack ist dort
also nicht das richtige Werkzeug. Stattdessen Firefly und Wayfinder (samt
Postgres) in **einem** Bridge-Compose zusammenführen; im selben Netz
funktioniert das Multicast-Routing **zwischen den Containern** problemlos (nur
Host↔Container ist unter Docker Desktop kaputt). Nicht der gemeinsame Ordner an
sich löst das Problem, sondern das **gemeinsame Bridge-Netz**.

Dafür liegt eine fertige, eingecheckte Compose-Datei bereit:
**`docker-compose.bridge.yml`**. Beide Repos als Geschwister-Ordner ablegen
(z. B. unter `~/asd/`) und aus dem Wayfinder-Repo starten:

```
~/asd/
├── firefly/     ← Firefly-Repo (der ../firefly-Build-Kontext)
└── wayfinder/   ← dieses Repo (von hier starten)
```

```bash
cd ~/asd/wayfinder
docker compose -f docker-compose.bridge.yml up --build
```

Dann im Browser: **http://localhost:8081/admin** (Login `admin`/`admin`,
Passwortwechsel).

> **Tracks sichtbar machen (Multi-Tenant).** Da es hier **keinen Orchestrator**
> gibt, ist Firefly ein fester externer Sender auf `239.255.0.62:8600`. Damit ein
> angemeldeter Mandant Tracks sieht, einen **Feed mit genau diesem Endpoint**
> anlegen (`multicast_group: 239.255.0.62`, `port: 8600` — **nicht**
> auto-allokieren) und einen Mandanten darauf abonnieren. Der vollständige
> Ablauf (inkl. der festen-Endpoint-Feinheit und der Frankfurt-Szene) steht im
> Runbook `docs/E2E-ABNAHME.md`, **Teil E-2**.
>
> Alternativ ist derselbe Bridge-Aufbau als Schritt-für-Schritt-Anleitung mit
> einem Master-Compose im **Überordner** in `docs/INSTALLATION.md`, Schritt 4.A
> beschrieben (für Einsteiger ohne Docker-Vorwissen).

## Details

### Compose-Stacks

| Datei | Zweck | Netz | Plattform |
|-------|-------|------|-----------|
| `docker-compose.onboarding.yml` | Standard-Plattform (Postgres + Server, `builtin`) | Bridge | alle |
| `docker-compose.bridge.yml` | E2E mit **festem** Firefly-Sender (kein Orchestrator); Live-Tracks auf Docker Desktop | Bridge | alle (v. a. Mac mini / Windows) |
| `docker-compose.orchestrated.yml` | E2E-Harness mit Firefly-Auto-Spawn (+ Orchestrator) | Host | Linux |

Beide setzen `WAYFINDER_DB_URL` und `WAYFINDER_AUTH_MODE: builtin`; der
Default-Admin wird beim ersten Start auto-seeded (ADR 0011). Eine fixe
`WAYFINDER_SESSION_KEY` (z. B. `openssl rand -hex 32`) macht Sessions neustart-
und replica-stabil; unset → ephemerer Schlüssel + Warn-Log.

### Dockerfile

**Multi-stage build:**
1. **Builder-Stage** (`golang:1.25-bookworm`): Lädt Module, kompiliert
   `cmd/wayfinder` statisch (`CGO_ENABLED=0`).
2. **Runtime-Stage** (`debian:bookworm-slim`): Minimal-Image mit nur dem Binary
   (+ `curl` für den Healthcheck).

`Dockerfile.orchestrator` baut analog die least-privilege
Orchestrator-Steuerebene (`cmd/wayfinder-orchestrator`).

**Healthcheck:** Der Container prüft, ob der Server auf `/health` (Port 8080)
antwortet.

## Lokaler Build (ohne Compose)

Auch der manuelle Lauf braucht eine erreichbare PostgreSQL-Datenbank und die
Pflicht-Env:

```bash
docker build -t wayfinder:latest .
docker run --network host \
  -e WAYFINDER_DB_URL="postgres://wayfinder:wayfinder@127.0.0.1:5432/wayfinder?sslmode=disable" \
  -e WAYFINDER_AUTH_MODE=builtin \
  wayfinder:latest
```

Ohne erreichbare `WAYFINDER_DB_URL` bricht der Start mit klarer Meldung ab
(ADR 0014) — es gibt keinen DB-losen Rückfall mehr.

## Cloud-Deployment

**12-Factor Config:**
- Alle Parameter via Env-Vars (`FIREFLY_CAT062_GROUP`, `WAYFINDER_*`)
- Graceful Shutdown via SIGTERM/SIGINT
- Strukturiertes JSON-Logging (stderr)
- `/health` (Liveness) und `/ready` (Readiness) für Kubernetes-Probes

`WAYFINDER_DB_URL` zeigt auf einen verwalteten/geclusterten Postgres;
`WAYFINDER_AUTH_MODE: proxy` (OIDC/oauth2-proxy am Ingress) ist der empfohlene
Produktiv-Pfad, `builtin` der Standalone-/Onboarding-Pfad.

In einer Cloud-Umgebung (Kubernetes etc.) ist `network_mode: host` meist nicht
verfügbar — dort empfängt Wayfinder den CAT062-Strom stattdessen z. B. über
einen Multicast-fähigen CNI/Underlay oder eine Unicast-Relay-Lösung. Die
Feed-Topologie (Host/Bridge/Orchestrierung) ist von der Mandantenfähigkeit
unabhängig (Feed-Authentizität/Netz-Isolation: eigenes Thema, `CLAUDE.md`
Abschnitt 7).

## Troubleshooting

**„WAYFINDER_DB_URL is required" beim Start:**
- Multi-Tenant ist Pflicht (ADR 0014): `WAYFINDER_DB_URL` setzen und auf eine
  erreichbare Postgres-Instanz zeigen lassen. Die Compose-Stacks bringen die DB
  mit; bei manuellem Lauf eine eigene Instanz bereitstellen.

**Login klappt nicht / kein Admin:**
- Beim ersten Start gegen eine frische DB wird `admin`/`admin` (builtin)
  auto-seeded; der erzwungene Passwortwechsel kommt sofort. Läuft der Container
  gegen eine bereits initialisierte DB, gilt das dort gesetzte Passwort.

**Karte bleibt leer:**
- Onboarding-Stack: erwartet — das Bridge-Netz transportiert kein Multicast,
  ohne konfigurierten Feed kommen keine Tracks. Für Live-Tracks den
  orchestrierten Stack (Linux) oder das Bridge-Master-Compose (macOS/Windows)
  nutzen.
- Orchestrierter Stack: Läuft Firefly mit `FIREFLY_CAT062_ENABLED=true`? Ist ein
  Feed abonniert (sonst spawnt der Orchestrator nichts)? Logs prüfen:
  `docker compose -f docker-compose.orchestrated.yml logs wayfinder` — wird der
  Multicast-Socket erfolgreich geöffnet?

**Build schlägt fehl:**
- Docker-Daemon läuft? (`docker ps`)
- Genug Disk-Space für den Build vorhanden?

**Port bereits belegt:**
- `lsof -i :8080` / `lsof -i :8081`
