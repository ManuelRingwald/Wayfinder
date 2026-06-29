# ORCH-2c (2/3) — Getrennter Control-Plane-Prozess (`wayfinder-orchestrator`)

> Zweites Stück von **ORCH-2c** (ADR 0012 §6). Verpackt die Orchestrierungs-Logik
> (Store-`DesiredState` + Reconciler + Backend) in ein **eigenes Binary**, das
> vom browser-zugewandten Server **prozess-hart getrennt** ist.
>
> **Lieferumfang:** `cmd/wayfinder-orchestrator` (Config, Wiring, `--once`/Loop)
> + Config-Tests. **Noch nicht** Teil: echter Docker-Adapter (ORCH-2b),
> Secret-Auflösung + Änderungs-Trigger (ORCH-2c, 3/3).

## Fachlicher Hintergrund — warum ein eigenes Binary

Container zu starten erfordert Zugriff auf den **Docker-Socket** (später die
K8s-API). Das ist das mächtigste Privileg im System — „Docker-Socket-Zugriff =
Root auf dem Host". Der heutige `wayfinder`-Prozess ist dem **Browser/Internet**
ausgesetzt (`/ws`, `/api`, SPA) und damit die Angriffsfläche.

ADR 0012 §6 verlangt deshalb: die Control-Plane läuft **getrennt** vom
Browser-Rand. Würde der Orchestrator als Goroutine im Hauptprozess laufen, hielte
**dieser eine, exponierte Prozess** den Docker-Socket — ein Browser-Rand-Exploit
(RCE, Auth-Umgehung) erbte sofort die Container-Start-Macht → Host-Übernahme. Als
**eigenes Binary** hat der Browser-Rand den Socket **nicht**; das Schlimmste, was
ein Exploit dort erreicht, ist ein DB-Schreibzugriff (Soll-Zustand), aber kein
Container-Start. Defense-in-Depth, Least-Privilege.

Zusätzlich passt die Trennung zur Skalierung (WF2-52): der Browser-Rand skaliert
horizontal (N zustandslose Repliken), während genau **ein** Orchestrator
reconciled — kein Leader-Election-Bedarf, keine konkurrierenden Reconciler.

## Was umgesetzt wurde (`cmd/wayfinder-orchestrator`)

### Verdrahtung (`run`)
Öffnet den DB-Pool, baut `orchestrator.StoreDesiredState` (über `SubscriptionRepo`
+ `FeedRepo`), das Backend (vorerst `instance.MemoryBackend`) und den
`reconciler.Reconciler`. **Migriert bewusst nicht** — der Hauptserver besitzt das
Schema; ein einziger Migrator vermeidet Races und hält die DB-Rolle dieses
Prozesses lese-förmig.

### Modi
- **`--once`**: ein einzelner Reconcile-Lauf, dann Exit — für CI, Dev-Smoke und
  K8s-`Job`/`CronJob`. Exit 0 = ok, 1 = Laufzeitfehler, 2 = Config-/Flag-Fehler.
- **Default (Loop)**: `reconciler.Run(ctx, interval)` mit Graceful-Shutdown über
  `signal.NotifyContext` (SIGINT/SIGTERM) → sauberes `context.Canceled`.

### Konfiguration (12-Factor, `loadConfig`)
`getenv`/`args` injiziert → unit-testbar ohne DB. `WAYFINDER_DB_URL` ist Pflicht
(ohne Katalog nichts zu tun → harter Fehler). `WAYFINDER_ORCHESTRATOR_INTERVAL`
(Default `15s`; ungültig/≤0 → Default, FR-CFG-002-Leniency). `WAYFINDER_LOG_LEVEL`
(ungültig → `info`). JSON-`slog` wie der Hauptserver.

## Sicherheits-/Betriebs-Betrachtung

- **🔒 Prozess-harte Trennung:** Der Container-Start-Pfad ist hier isoliert; der
  Browser-Rand erhält das Privileg nie (ADR 0012 §6).
- **Ein Migrator:** Der Orchestrator liest nur, migriert nicht.
- **Crash-fest & idempotent:** vom Reconciler geerbt (ORCH-3).
- **Keine Schnittstellen-Wirkung** auf CAT062.
- **Noch kein Standard-Deployment:** Da das Backend bis ORCH-2b ein In-Memory-
  Platzhalter ist (startet real nichts), ist der Orchestrator bewusst **noch
  nicht** in `INSTALLATION.md` verdrahtet — das folgt mit dem Docker-Adapter.

## Tests

`cmd/wayfinder-orchestrator/main_test.go` (DB-frei): `loadConfig`-Defaults,
DSN-Pflicht (Fehler), `--once`-Flag, Custom-Interval + Fallback bei
ungültig/≤0, Log-Level inkl. Fallback, unbekanntes Flag → Fehler. Zusätzlich
Smoke-verifiziert: Exit 2 ohne DSN / bei unbekanntem Flag, Exit 1 bei
DB-Verbindungsfehler (`--once`).

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-003** (Reconciler — um das getrennte Binary
ergänzt).

## Nächstes Stück

- **ORCH-2c (3/3)** — Secret-Auflösung (`cred_ref` → Wert beim Launch,
  Secret-Speicher je Feed) + Änderungs-getriebener Reconcile-Trigger (statt nur
  periodisch).
- **ORCH-2b** — echter Docker-`Backend`-Adapter (`Spec` → Container), der den
  `MemoryBackend`-Platzhalter ablöst und das Binary operativ macht.
