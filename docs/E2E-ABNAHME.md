# End-to-End-Abnahme der Auto-Orchestrierung (ORCH-5c)

> **Zweck:** Die komplette Auto-Orchestrierungs-Kette auf einem echten Docker-Host
> nachweisen: **Feed zuweisen → Orchestrator spawnt Firefly → CAT062/UDP-Multicast
> → Tracks im ASD → Abbestellen räumt auf.** Diese Datei ist das Abnahme-Runbook
> zum Abhaken; das Skript `scripts/e2e-orchestrated.sh` automatisiert den
> Großteil.

## Was hier nachgewiesen wird (und was nicht)

Bis ORCH-5c war die Auto-Orchestrierung **gebaut und unit-getestet**, aber in
keiner lauffähigen Deployment-Konfiguration zusammengesteckt. ORCH-5c liefert das
**Harness** (`docker-compose.orchestrated.yml` + `Dockerfile.orchestrator`) und
diesen Abnahme-Lauf. Der **authentifizierte** OpenSky-Pfad braucht echte
OAuth2-Credentials und Netz-Egress und wird daher zuletzt manuell abgenommen.

## Voraussetzungen

- Docker-Daemon + `docker compose` v2 auf dem Abnahme-Host.
- **Firefly-Image lokal vorhanden.** Aus dem Firefly-Repo bauen:
  ```bash
  docker build -t firefly:latest .   # im Firefly-Repo
  ```
  oder ein veröffentlichtes Tag setzen: `export WAYFINDER_FIREFLY_IMAGE=…`.
- Linux-Host (Host-Networking + Multicast). Auf Docker Desktop (macOS/Windows)
  funktioniert das Host-Networking-Multicast i. d. R. **nicht** — siehe `DOCKER.md`.

## Die Prüfpunkte

| # | Behauptung | Beobachtung |
|---|------------|-------------|
| 1 | Feed-Zuweisung **spawnt** einen Container | `docker ps` zeigt `wayfinder-firefly-feed-<id>` (Label `wayfinder.feed_id`) |
| 2 | Container bekommt die **richtige** Config | `docker inspect` Env: `FIREFLY_CAT062_GROUP/PORT`; je nach Quelle `FIREFLY_SCENE` **oder** `FIREFLY_MODE=live` + `FIREFLY_SOURCES` |
| 3 | **Secret-Injection** wirkt (nur authentifiziert) | Firefly-Log: OpenSky **authentifiziert** (kein 401, höheres Limit) |
| 4 | **Kein Leak** | Secret-Wert **nicht** im `FIREFLY_SOURCES`-JSON, in **keinem** Orchestrator-Log |
| 5 | Tracks landen im ASD | `wayfinder_cat062_tracks_received_total` > 0; Karte zeigt Tracks |
| 6 | **Anonym-Fallback** | Secret entfernen → Quelle läuft anonym weiter, kein Reconcile-Abbruch (WARN-Log) |
| 7 | **Rotation → Restart** | Secret ändern → Spec-Hash ändert sich → neuer Container (neue ID) |
| 8 | **Orphan-Cleanup** | Letztes Abo entfernen → Container gestoppt/entfernt |

Das Skript deckt **1, 2, 5, 8** automatisiert ab (plus den Kein-Leak-Teil von 4
im `opensky-anon`-Modus). **3, 6, 7** sind die credential-bezogenen Punkte und
werden im authentifizierten Lauf manuell geprüft.

## A) Automatischer Lauf (offline, Demo-Scene)

Beweist Spawn → Multicast → ASD → Cleanup **ohne** externen Netzzugriff: der Feed
hat keine Live-Quellen, der Orchestrator gibt `FIREFLY_SCENE` mit, und Firefly
spielt ein Demo-Szenario ein.

```bash
scripts/e2e-orchestrated.sh            # --mode scene ist der Default
```

Erwartung: jeder Prüfpunkt meldet `✓`, am Ende `✅ E2E acceptance (scene) passed.`
Mit `--keep` bleibt der Stack zum Inspizieren oben.

## B) Automatischer Lauf (anonymes OpenSky)

Exerziert den `FIREFLY_SOURCES`-Live-Pfad (`FIREFLY_MODE=live`) mit einer
anonymen `adsb_opensky`-Quelle. **Braucht Netz-Egress zu OpenSky.**

```bash
scripts/e2e-orchestrated.sh --mode opensky-anon
```

Zusätzlich zu A prüft das Skript, dass `FIREFLY_SOURCES` gesetzt ist und **kein**
`FIREFLY_SOURCE_0_SECRET` existiert (anonym → kein Credential-Env).

## C) Authentifizierter Lauf (manuell, Prüfpunkte 3/4/6/7)

Nur auf deiner Zielumgebung mit echten OpenSky-OAuth2-Credentials.

1. **Schlüssel setzen** (auf **beiden**, Server und Orchestrator):
   ```bash
   export WAYFINDER_SECRET_KEY=$(openssl rand -base64 32)
   docker compose -f docker-compose.orchestrated.yml up -d --build
   ```
2. Auf der OpenSky-Account-Seite einen **API-Client** anlegen → `client_id` +
   `client_secret` (OAuth2, ADR 0024 in Firefly).
3. Im Admin (oder per Admin-API) einen Feed mit einer `adsb_opensky`-Quelle
   anlegen, eine `cred_ref` vergeben und im Secret-Dialog **Client-ID + Client-
   Secret** eintragen; einen Mandanten darauf abonnieren.
4. **Prüfpunkt 3/4:** `docker inspect wayfinder-firefly-feed-<id>` → `FIREFLY_SOURCES`
   enthält **keinen** Secret-Wert; `FIREFLY_SOURCE_0_SECRET` trägt ihn separat.
   `docker logs` des Trackers: kein 401, OpenSky authentifiziert. Orchestrator-Log
   enthält **nie** den Klartext.
5. **Prüfpunkt 6 (Anonym-Fallback):** Secret entfernen → der Tracker startet ohne
   `FIREFLY_SOURCE_0_SECRET` neu (anonym), der Orchestrator loggt eine WARN, der
   Reconcile bricht **nicht** ab.
6. **Prüfpunkt 7 (Rotation):** Secret ändern → der Spec-Hash ändert sich → der
   Reconciler ersetzt den Container (neue Container-ID).

## Aufräumen

```bash
docker compose -f docker-compose.orchestrated.yml down -v --remove-orphans
```

Das Skript räumt am Ende selbst auf (außer mit `--keep`).

## Sicherheits-Hinweis (Docker-Socket)

Der Orchestrator mountet `/var/run/docker.sock` — das gibt ihm **root-äquivalente**
Kontrolle über den Host (er startet/stoppt Container). Genau deshalb ist der
Orchestrator ein **getrennter, Least-Privilege-Prozess** und der browser-zugewandte
Server bekommt den Socket **nie** (ADR 0012 §6). Im Produktivbetrieb ist der
Orchestrator-Host/-Node eine **hochwertige Vertrauensgrenze**: Netz-Isolation,
restriktiver Zugang, keine Co-Location mit dem Browser-Rand.

## Bekannte Grenzen

- **Docker Desktop (macOS/Windows):** Host-Networking-Multicast funktioniert dort
  i. d. R. nicht — die Abnahme braucht einen Linux-Host.
- **Diese Repo-CI/Sandbox:** ohne laufenden Docker-Daemon kann der Lauf nicht
  ausgeführt werden; verifiziert sind dort nur `docker compose config`, die
  Binär-Builds und die Skript-Syntax. Der echte Lauf gehört auf einen Docker-Host.
