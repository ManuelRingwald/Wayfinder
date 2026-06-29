# ORCH-2c (3b) — Änderungs-getriebener Reconcile (Postgres `LISTEN/NOTIFY`)

> Macht die Auto-Orchestrierung **reaktiv**: eine relevante Katalog-Änderung löst
> **sofort** einen Reconcile aus, statt bis zu einem Intervall zu warten. Das
> Intervall bleibt als Sicherheitsnetz (ADR 0012 §5: „periodisch **und auf
> Anstoß**").

## Fachlicher Hintergrund

Bisher konvergierte der Orchestrator nur im Intervall-Takt
(`WAYFINDER_ORCHESTRATOR_INTERVAL`, Default 15 s). Legt ein Admin einen Feed an,
abonniert ein Mandant oder ändert sich eine Quell-Konfig, passierte **bis zu ein
Intervall lang nichts** — ein neu provisionierter Mandant wartete spürbar, bis
„seine" Firefly-Instanz hochfuhr. 3b schließt diese Latenz: die Zuweisung wirkt
praktisch sofort.

## Was umgesetzt wurde

### DB-Seite — Trigger (`pkg/store/migrations/00012_reconcile_notify.sql`)
Eine `plpgsql`-Trigger-Funktion `wayfinder_notify_reconcile()` ruft
`pg_notify('wayfinder_reconcile', '')`. Zwei **Statement-Level**-Trigger hängen an
den Tabellen, die das Soll definieren:

- **`feeds`** — der Feed (inkl. `source_config`/`coverage_bbox`) ist die Spec;
- **`subscriptions`** — ein Feed ist Soll, sobald er ≥ 1 Abo hat.

Entscheidungen:
- **DB-seitig statt App-seitig:** fängt **jeden** Schreiber (adminapi *und*
  manuelles SQL), nicht nur die eigene App.
- **Statement-Level (nicht Row-Level):** eine Massen-Änderung erzeugt **eine**
  Notification statt einer pro Zeile.
- **Leerer Payload:** der Reconciler berechnet ohnehin das **volle** Soll neu —
  das Signal genügt, der Inhalt ist irrelevant.
- **`feed_secrets` bewusst nicht abgedeckt:** ein Secret-Wert wird erst
  spec-relevant, wenn die Container-Injection landet (ORCH-5); ein Trigger jetzt
  erzeugte nur folgenlose Reconciles.

### Orchestrator-Seite — Listener (`pkg/orchestrator/listen.go`)
`Listener` hält eine **dedizierte** `pgx`-Verbindung (`LISTEN` bindet an genau
eine Session — eine Pool-Verbindung wäre ungeeignet), `LISTEN wayfinder_reconcile`,
und wandelt jede Notification in ein Reconcile-Signal.

- **Resync-on-(Re-)Connect:** nach jedem Verbindungsaufbau wird **ein** Signal
  gefeuert — eine Änderung vor dem ersten `LISTEN` (Start) oder während einer
  Reconnect-Lücke wird so eingeholt (der Reconciler liest das volle Soll, ein
  Extra-Signal ist immer sicher).
- **Reconnect mit fixem Backoff** (`listenBackoff`, 2 s) gegen eine flappende DB.
- **Sauberes Herunterfahren** über `ctx` (`WaitForNotification` respektiert die
  Cancellation).

### Coalescing — Reconciler (`pkg/reconciler/reconciler.go`)
`Run` bekommt einen `trigger <-chan struct{}`; der `select` erhält einen dritten
Fall `case <-trigger:`. Das Signal-Senden im Listener ist **nicht-blockierend** auf
einen gepufferten **Size-1**-Channel (`signalReconcile`): liegt schon ein Signal
an, wird das neue verworfen — ein Burst kollabiert zu **einem** anstehenden
Reconcile statt N. `trigger` ist **nil-bar** (Memory-Backend/Tests → nur das
Intervall treibt). Das Intervall bleibt als Sicherheitsnetz erhalten.

### Verdrahtung (`cmd/wayfinder-orchestrator/main.go`)
Im Loop-Modus: ein Size-1-Trigger-Channel, eine Listener-Goroutine
(`listener.Listen(ctx, trigger)`), und `rec.Run(ctx, interval, trigger)`. Der
`--once`-Modus bleibt unberührt (ein Lauf, kein Listener).

## Sicherheits-/Robustheits-Betrachtung

- **Kein verlorener Zustand:** Da der Reconciler das volle Soll neu berechnet und
  das Ist beobachtet, ist ein verpasstes/zusätzliches Signal nie gefährlich —
  schlimmstenfalls ein No-op-Reconcile. Das Intervall heilt jede Lücke.
- **Reconnect-Korrektheit:** Resync-on-Connect schließt das „Änderung während der
  Verbindungslücke verpasst"-Fenster.
- **Least-Privilege unverändert:** der Listener liest nur Notifications; keine
  neuen Schreibrechte, keine Browser-Rand-Berührung.
- **Keine CAT062-Schnittstellen-Wirkung;** Migration additiv.

## Tests

- `pkg/reconciler/reconciler_test.go::TestRunReconcilesOnTrigger` — ein Trigger
  reconciled sofort (riesiges Intervall, sodass nur der Trigger wirken kann);
  bestehender `TestRunReconcilesUntilCancelled` auf die neue Signatur (nil-Trigger)
  gezogen.
- `pkg/orchestrator/listen_integration_test.go::TestIntegrationListenerSignalsOnChange`
  (**real-PG**, skippt ohne `WAYFINDER_TEST_DB_URL`): Resync-on-Connect,
  `feeds`-Insert/Delete und `subscriptions`-Insert erzeugen je ein Signal — testet
  Migration-Trigger **und** Listener end-to-end.
- Verifiziert gegen ein echtes PostgreSQL (Migration 00012 inkl. dollar-quoted
  Funktion appliziert sauber; bestehende Store-Integrations-Suite unverändert grün).
- Gates: `go test ./...`, `go vet`, `gofmt` grün.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-004** (Änderungs-getriebener Reconcile).

## Nächste Stücke

- **Container-Injection (ORCH-5, cross-project):** `SecretResolver` →
  Firefly-Container-Env; braucht den env-getriebenen Quell-Eingangs-Kontrakt von
  Firefly ([Firefly #35](https://github.com/manuelringwald/firefly/issues/35)).
- **ORCH-4:** automatische, kollisionsfreie Multicast-Gruppe/Port-Vergabe je
  orchestrierter Instanz.
