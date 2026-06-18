# Wayfinder

Das **ASD** (*Air Situation Display*) — die Lagedarstellung für den Lotsen.
Wayfinder empfängt den Live-Track-Strom des Radar-Trackers
[Firefly](https://github.com/manuelringwald/firefly), dekodiert ihn und zeigt
ihn als bedienbares Luftlagebild auf einer 2D-Karte im Browser.

> **Status:** Wird für den realen Betrieb gebaut (siehe `CLAUDE.md`), aber
> noch in früher Entwicklung — aktuell M1 (Pipeline + Live-Karte) abgeschlossen.

---

## Loslegen — zur Live-Karte

Wayfinder zeigt nur etwas an, wenn ein **CAT062/UDP-Multicast-Sender** läuft —
in der Praxis ist das **Firefly**. Beide Projekte sind unabhängig, kommunizieren
aber über den CAT062-Draht-Vertrag (`CLAUDE.md` Abschnitt 2).

### Weg A — mit Docker (empfohlen unter Linux)

Voraussetzung: [Docker](https://www.docker.com/) ist installiert.

```bash
docker-compose up
```

Dann im Browser öffnen: **http://localhost:8081**

Für das volle Bild mit Live-Tracks läuft parallel Firefly mit dem
Frankfurt-Szenario und aktiviertem CAT062-Feed:

```bash
# im Firefly-Repo
FIREFLY_SCENE=frankfurt FIREFLY_CAT062_ENABLED=true docker-compose up
```

> Auf **macOS/Windows (Docker Desktop)** sehen sich zwei separat gestartete
> `docker-compose up`-Stacks wegen `network_mode: host` nicht — dafür gibt es
> in [DOCKER.md](DOCKER.md) eine Bridge-Netzwerk-Variante mit gemeinsamem
> Master-Compose.

Details (insbesondere zur Multicast-/Docker-Netzwerk-Besonderheit) siehe
[DOCKER.md](DOCKER.md).

### Weg B — lokal mit Go

Voraussetzung: ein aktueller [Go-Toolchain](https://go.dev/) (>= 1.23) ist
installiert.

```bash
go run ./cmd/wayfinder
```

Dann im Browser öffnen: **http://localhost:8081**

Für Live-Tracks parallel Firefly mit dem Frankfurt-Szenario starten:

```bash
# im Firefly-Repo
FIREFLY_SCENE=frankfurt FIREFLY_CAT062_ENABLED=true cargo run -p firefly-server
```

Wayfinder lauscht standardmäßig auf der CAT062-Multicast-Gruppe
`239.255.0.62:8600` — Fireflys Default, kein zusätzliches Konfigurieren nötig.

---

## Was du im Browser siehst

Eine MapLibre-GL-Karte, zentriert auf Frankfurt (Fireflys Demo-Szene). Jeder
empfangene Track erscheint als:

- ein **farbiger Punkt** (grün = bestätigt, grau = tentativ, orange =
  „coasting" — keine frische Messung, siehe
  [Glossar](docs/glossary.md)),
- ein **Datenblock-Label**: Track-Nummer und — wenn vorhanden — die
  Flugfläche (`FLnnn`, aus I062/136),
- ein **Geschwindigkeitsvektor** (Kurs-Pfeil, 60s-Vorausschau, ASD-Stil),
- eine **Spur** der letzten Positionen.

Ohne laufendes Firefly bleibt die Karte leer (kein Fehler — Wayfinder wartet
einfach auf den ersten CAT062-Block).

---

## Architektur

Ein Go-Binary (`cmd/wayfinder`) plus ein JavaScript/MapLibre-Frontend.

```
pkg/cat062      CAT062-Decoder (FSPEC-Parser, Track-Felder, Referenz-Vektor-Tests)
pkg/receiver    UDP-Multicast-Empfang + Decoder-Integration
pkg/broadcast   Broadcaster: verteilt dekodierte Tracks an WebSocket-Clients
pkg/ws          WebSocket-Handler (Upgrade, Client-Lifecycle)
internal/webui  Eingebettetes Frontend (HTML/JS, MapLibre GL JS)
internal/config Konfiguration aus Umgebungsvariablen (12-Factor)
internal/server Health-/Readiness-Probes
cmd/wayfinder   main: verdrahtet Receiver → Broadcaster → WebSocket + Frontend
```

**Datenfluss:** CAT062/UDP-Multicast → `pkg/receiver` (Decode) →
`pkg/broadcast` (Fan-out) → `pkg/ws` → Browser (MapLibre).

## Konfiguration (Umgebungsvariablen)

| Variable | Default | Bedeutung |
|----------|---------|-----------|
| `FIREFLY_CAT062_GROUP` | `239.255.0.62` | Multicast-Gruppe des CAT062-Feeds |
| `FIREFLY_CAT062_PORT` | `8600` | Multicast-Port des CAT062-Feeds |
| `WAYFINDER_MAP_CENTER_LAT` / `_LON` | Frankfurt (50.0379 / 8.5622) | Karten-Mittelpunkt |
| `WAYFINDER_MAP_ZOOM` | `8` | Karten-Zoomstufe |
| `WAYFINDER_MAP_STYLE_URL` | (eingebauter OSM-Style) | eigener MapLibre-Style |

`/health` und `/ready` laufen fest auf Port `8080`, der WebSocket-/Frontend-Server
fest auf Port `8081`.

## Bauen & Testen

```bash
go build ./...    # alles bauen
go test ./...     # alle Tests
go vet ./...      # statische Prüfung
gofmt -l .        # Formatierung prüfen
```

## Container & Deployment

Details zu Docker, docker-compose und dem End-to-End-Zusammenspiel mit Firefly
stehen in [DOCKER.md](DOCKER.md).

## Mehr erfahren

- [docs/glossary.md](docs/glossary.md) — Domänen-Referenz (ASTERIX, CAT062, ASD …).
- [docs/STATUS.md](docs/STATUS.md) — aktueller Arbeitsstand & nächste Schritte.
- [docs/milestones/](docs/milestones/) — Feature-Dokumentation pro Baustein.
- [CLAUDE.md](CLAUDE.md) — Arbeitsregeln dieses Projekts, inkl. des
  CAT062-Draht-Vertrags (Abschnitt 2).

## Lizenz

Apache-2.0
