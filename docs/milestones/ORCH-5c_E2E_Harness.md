# ORCH-5c — End-to-End-Abnahme-Harness

> Die Auto-Orchestrierung (ORCH-1…5b) war gebaut und unit-getestet, aber in
> **keiner** lauffähigen Deployment-Konfiguration zusammengesteckt. ORCH-5c
> liefert das Harness, das die ganze Kette auf einem echten Docker-Host
> nachweisbar macht — plus ein Skript, das den Großteil automatisiert.

## Fachlicher Hintergrund

Das damalige Repo-`docker-compose.yml` (Single-Tenant, inzwischen entfernt —
ADR 0014) fuhr nur den ASD-Server gegen einen **externen** Feed. Der Beweis „Feed zuweisen ⇒ Tracker startet ⇒ Tracks im ASD" fehlte als
zusammengesteckter, abnehmbarer Lauf. Das Harness schließt diese Lücke und ist das
Abnahme-Tor für ORCH.

## Was umgesetzt wurde (5c-1: Harness)

- **`Dockerfile.orchestrator`** — baut das `cmd/wayfinder-orchestrator`-Binary als
  eigenes Image (getrennt vom Server, ADR 0012 §6: nur dieser Prozess bekommt das
  Container-Runtime-Privileg).
- **`docker-compose.orchestrated.yml`** — Single-Host-Abnahme-Stack: `db`
  (Postgres) + `wayfinder` (Server, migriert das Schema, empfängt den Feed) +
  `orchestrator` (`BACKEND=docker`, mountet **nur hier** `/var/run/docker.sock`,
  `WAYFINDER_FIREFLY_IMAGE`, `WAYFINDER_FIREFLY_SCENE`, gemeinsamer
  `WAYFINDER_SECRET_KEY`). Alle host-vernetzt, damit CAT062-Multicast den Empfänger
  erreicht; Postgres veröffentlicht 5432 an den Host.
- **`docs/E2E-ABNAHME.md`** — Runbook mit den 8 Prüfpunkten und drei Läufen
  (offline Demo-Scene / anonymes OpenSky / authentifiziert manuell).

## Was umgesetzt wurde (5c-2: Skript)

- **`scripts/e2e-orchestrated.sh`** — bringt den Stack hoch, **seedet den Katalog
  direkt in Postgres** (Tenant + Feed + Subscription — zielt auf den
  Orchestrator-Pfad ohne Admin-Auth-Choreografie) und assertet:
  - **Prüfpunkt 1:** ein Container mit Label `wayfinder.feed_id=<id>` läuft.
  - **Prüfpunkt 2:** dessen Env trägt Endpoint + (Scene-Modus) `FIREFLY_SCENE`
    bzw. (OpenSky-Modus) `FIREFLY_MODE=live` + `FIREFLY_SOURCES`, **ohne**
    `FIREFLY_SOURCE_0_SECRET` (anonym → kein Leak, Teil von Prüfpunkt 4).
  - **Prüfpunkt 5:** `wayfinder_cat062_tracks_received_total` > 0 (best-effort,
    WARN statt Abbruch — Demo-Traffic kann spärlich sein).
  - **Prüfpunkt 8:** nach `DELETE` der Subscription verschwindet der Container
    (Orphan-Cleanup).
  - Aufräum-Trap (`down -v`, außer `--keep`).
  - Zwei Modi: `--mode scene` (offline, Default) und `--mode opensky-anon`.

Die credential-bezogenen Prüfpunkte **3/6/7** brauchen echte OAuth2-Credentials und
sind im Runbook als manueller authentifizierter Lauf beschrieben.

## Sicherheits-/Schnittstellen-Betrachtung

- **Least-Privilege bleibt sichtbar:** nur der `orchestrator`-Service mountet den
  Docker-Socket; der Server nie.
- **Auth-Modus:** Das Harness lief ursprünglich mit `AUTH_MODE=none` (Fokus auf
  die Orchestrierungs-Kette). Seit ADR 0014 (Multi-Tenant-only) fährt der
  orchestrierte Stack `builtin` (Auto-Seed `admin`/`admin`); ein `none`-Modus
  existiert nicht mehr.
- **Kein-Leak** ist im `opensky-anon`-Lauf maschinell geprüft (kein Secret-Env);
  der vollständige Klartext-Nachweis (Prüfpunkt 4) gehört in den authentifizierten
  Lauf.

## Verifikation (Stand dieser Sitzung)

Ohne laufenden Docker-Daemon in der Sandbox sind verifiziert: `docker compose
config` (Syntax gültig), `go build ./...` (beide Binaries), `bash -n` (Skript-
Syntax). Der **echte** DinD-Lauf gehört auf einen Linux-Docker-Host (Runbook
Abschnitt A/B/C).

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-007** (E2E-Abnahme-Harness).

## Stand von ORCH

ORCH-1…5 sind damit gebaut **und** abnehmbar. Offen: der reale Abnahme-Lauf auf
deiner Zielumgebung (inkl. authentifiziertes OpenSky) sowie — separate ADRs —
Fireflys FLARM/APRS- und Radar-ASTERIX-Live-Adapter.
