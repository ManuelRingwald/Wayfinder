# ORCH-2b — Docker-Backend (`pkg/dockerbackend`)

> Löst den `MemoryBackend`-Platzhalter ab: der Orchestrator startet jetzt **echte
> Firefly-Container**, einen pro abonniertem Feed (ADR 0012 §4). Damit wird „Feed
> zuweisen ⇒ passende Firefly-Instanz startet" real.
>
> **Entscheidung (Betreiber):** Anbindung über das **offizielle Docker Go-SDK**
> (nicht CLI-shell-out) — wegen Sicherheit (keine Shell-/Injection-Fläche),
> Robustheit (typisierte Status-Abfragen) und selbst-genügsamem Binary.

## Architektur — schwere Abhängigkeit isoliert

Die gesamte Lebenszyklus-Logik liegt in `backend.go` hinter einem **schmalen
`ContainerClient`-Interface** (`List/Create/Start/Stop/Remove`) und ist mit einem
Fake **voll unit-getestet** — ohne Docker-Daemon. Die **einzige** Datei, die das
schwere Docker-SDK importiert, ist `client.go`: eine dünne Übersetzung des
Interfaces auf die Docker-Engine-API. So bleibt der Adapter testbar, der
SDK-Blast-Radius minimal und der Weg (SDK ↔ CLI ↔ K8s) später austauschbar, ohne
Reconciler/Backend anzufassen.

## Container-Lebenszyklus (`Backend`)

- **Identität:** ein Container je Feed, Label `wayfinder.feed_id=<id>`.
  Container-Name nur aus der **numerischen** Feed-ID (`wayfinder-firefly-feed-<id>`)
  — injection-sicher, kein Operator-String im Namen; das SDK ruft ohnehin keine
  Shell.
- **Drift-Erkennung:** beim Create wird ein `wayfinder.spec_hash` (SHA-256 über
  Image + Netzwerk + Env) als Label gestempelt. `Start` vergleicht den Soll-Hash
  mit dem laufenden Container:
  - **gleicher Hash, läuft** → No-op (idempotent — der Reconciler ruft `Start`
    jeden Zyklus);
  - **gleicher Hash, gestoppt** → Container starten (Recovery);
  - **anderer Hash** → alten Container stop+remove, neuen create+start (Replace).
- **`Stop`:** stop + remove; unbekannter Feed = No-op.
- **`Status`:** running → `Running`, exited/dead → `Failed`, kein Container →
  `Stopped`.
- **`RunningFeeds`:** listet **alle** managed Container (auch gestoppte), damit der
  Reconciler verwaiste (auch tote) Container abräumt.

## Firefly-Env (`fireflyEnv`)

Heute gesetzt: `FIREFLY_CAT062_GROUP`, `FIREFLY_CAT062_PORT`, bei vorhandener
Coverage `FIREFLY_COVERAGE_BBOX`, optional `FIREFLY_SCENE` (Platzhalter-Quelle, bis
echte Live-Ingestion existiert). **Bewusst noch nicht** gesetzt: die echte
Quell-Eingangs-Env (`FIREFLY_SOURCES`) und die aufgelösten Quell-Credentials —
das ist cross-project (ORCH-5, Firefly) bzw. ORCH-2c (3/3). Env-Reihenfolge ist
deterministisch, damit der Spec-Hash stabil ist.

## Binary-Verdrahtung (`cmd/wayfinder-orchestrator`)

`WAYFINDER_ORCHESTRATOR_BACKEND` wählt `memory` (Default, startet nichts — sicher
für Dev/CI, redet nie mit Docker) oder `docker`. `docker` erfordert
`WAYFINDER_FIREFLY_IMAGE`; optional `WAYFINDER_FIREFLY_NETWORK` (Default `host`)
und `WAYFINDER_FIREFLY_SCENE`. `newBackend` baut den `dockerbackend` über
`NewDockerClient()` (Docker-Env/Socket, API-Versions-Aushandlung).

## Sicherheits-Betrachtung

- **🔒 Docker-Socket nur hier:** Nur der getrennte Orchestrator-Prozess (ORCH-2c)
  erhält Socket-Zugriff; der Browser-Rand nie (ADR 0012 §6). „Docker-Socket =
  Root auf dem Host" — deshalb die prozess-harte Trennung.
- **Keine Shell/Injection:** Das SDK baut keine Kommandozeilen; Container-Namen
  sind rein numerisch.
- **Sicherer Default:** `memory` ist Default — ein versehentlicher Lauf redet nie
  mit einem Daemon.
- **Keine CAT062-Schnittstellen-Wirkung.**

## Abhängigkeit

Neu: `github.com/docker/docker v27.5.1+incompatible` (klassisches `+incompatible`-
Layout mit Paket `.../client`) + Transitives; `github.com/docker/go-connections`
auf `v0.5.0` gepinnt (Kompatibilität mit dem SDK-Socket-Dialer). Bewusst
akzeptierte schwere Abhängigkeit (Betreiber-Entscheidung), durch das
`ContainerClient`-Interface auf eine Datei eingegrenzt.

## Tests

`pkg/dockerbackend/backend_test.go` (Fake-`ContainerClient`, daemon-frei): Create+
Run, Idempotenz auf gleichem Spec, Restart eines gestoppten Containers, Replace
bei Drift, Stop+Remove, Stop-unknown-No-op, `Failed`-Status, `RunningFeeds`,
Reject-invalid-Spec, `fireflyEnv`-Mapping — unter `-race`. Plus
`cmd/wayfinder-orchestrator/main_test.go`: Backend-Default `memory`, `docker`
erfordert Image, unbekanntes Backend → Fehler. `client.go` ist bewusst
**nicht** unit-getestet (braucht Daemon) und minimal gehalten.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-002** (Instanz-Abstraktion — um den Docker-
Adapter + Binary-Verdrahtung ergänzt).

## Nächstes Stück

- **ORCH-2c (3/3)** — Secret-Auflösung (`cred_ref` → Wert beim Launch,
  Secret-Speicher je Feed) + Änderungs-getriebener Reconcile-Trigger; danach
  volle `INSTALLATION.md`-Integration (Compose-Service + Socket-Mount).
- **ORCH-5** (Firefly) — generische Live-Quell-Ingestion; erst damit wird
  `FIREFLY_SOURCES` real und der Platzhalter `FIREFLY_SCENE` entfällt.
