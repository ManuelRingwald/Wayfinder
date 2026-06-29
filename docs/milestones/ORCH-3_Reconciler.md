# ORCH-3 — Reconciler (Operator-Muster, gegen `MemoryBackend`)

> Häppchen von **ORCH-2/3** (ADR 0012 §5). Baut den Reconciler-Kern, der die
> laufenden Tracker-Instanzen automatisch an die Katalog-Lage angleicht. Bewusst
> **vor** dem echten Docker-Adapter gebaut (Betreiber-Entscheidung) — voll
> testbar gegen das `MemoryBackend` aus ORCH-2a.
>
> **Lieferumfang ORCH-3:** `pkg/reconciler` (DesiredState-Interface, `Reconcile`,
> `Run`) + kleine `instance.Backend`-Erweiterung (`RunningFeeds`) + Tests.
> **Noch nicht** Teil: echter Store-Adapter für `DesiredState` und der
> Änderungs-Trigger (ORCH-2c), Docker-Backend (ORCH-2b).

## Fachlicher Hintergrund

ADR 0012 will „Feed zuweisen ⇒ Instanz startet, letztes Abo weg ⇒ Instanz
verschwindet". Imperativ (beim Abo starten, beim Unsubscribe stoppen) wäre das
fragil: ein Absturz, ein verpasstes Event oder ein Neustart ließen Soll und Ist
auseinanderlaufen. Das **Operator-Muster** (wie ein Kubernetes-Controller) ist
robust: der Reconciler vergleicht periodisch **Soll** (was laut Katalog laufen
soll) mit **Ist** (was tatsächlich läuft) und korrigiert die Differenz —
idempotent und selbstheilend.

## Was umgesetzt wurde (`pkg/reconciler`)

### `DesiredState`-Interface
`DesiredSpecs(ctx) ([]instance.Spec, error)` — liefert die Soll-Specs: alle Feeds
mit ≥ 1 aktivem Abo, je aus ihrer Quell-Konfig (ORCH-1) abgeleitet. Injiziert;
der Store-Adapter (Feeds ⨝ Subscriptions + `GetSourceConfig`) wird in der
Control-Plane verdrahtet (ORCH-2c). In Tests ein Fake.

### `Reconcile(ctx)` — ein Soll-gleich-Ist-Durchlauf
1. **Konvergenz:** jeder Soll-Spec wird ge-`Start`et. Da das Backend idempotent
   ist, ist das ein No-op bei gleichem Spec, ein **Re-Apply** bei geändertem und
   eine **Recovery** eines abgestürzten/fehlgeschlagenen.
2. **Orphan-Cleanup:** das Ist wird **beobachtet** (`Backend.RunningFeeds`), nicht
   aus dem Gedächtnis genommen — jede laufende Instanz, deren Feed nicht mehr Soll
   ist, wird ge-`Stop`t. Ist-statt-Erinnerung macht den Lauf **crash-fest**
   (nach einem Neustart stimmt der Zustand trotzdem).
3. **Fehler-Isolation:** ein Per-Feed-Fehler bricht den Durchlauf **nicht** ab —
   er wird geloggt und via `errors.Join` gesammelt zurückgegeben; die übrigen
   Feeds konvergieren weiter, der nächste Tick heilt.

### `Run(ctx, interval)` — die Schleife
Reconciled sofort, dann je `interval`-Tick bis `ctx`-Cancel; Reconcile-Fehler
werden geloggt, die Schleife läuft weiter (transiente Fehler heilen selbst).

### `instance.Backend.RunningFeeds(ctx)` (Erweiterung)
Liefert die Feed-IDs mit laufender Instanz — die Ist-Beobachtung für die
Orphan-Erkennung. Kern des Operator-Musters (beobachten statt erinnern). Im
`MemoryBackend` implementiert.

## Sicherheits-/Robustheits-Betrachtung

- **Crash-fest:** Da der Reconciler das tatsächliche Backend-Ist abfragt, statt
  einen erinnerten Zustand zu pflegen, konvergiert er nach jedem Neustart korrekt
  (keine verwaisten oder fehlenden Instanzen durch verpasste Events).
- **Fehler-Eindämmung:** Ein einzelner kaputter Feed blockiert nie die
  Konvergenz der übrigen.
- **Determinismus:** `SpecFromFeed` liefert deterministische Specs (sortierte
  Secret-Refs), sodass „Spec geändert" verlässlich erkannt wird (Re-Apply nur bei
  echter Änderung — relevant, sobald das Docker-Backend Specs vergleicht).
- **Keine Schnittstellen-Wirkung** auf CAT062; rein Wayfinder-intern.

## Tests

`pkg/reconciler/reconciler_test.go`: Soll wird gestartet; Orphans werden gestoppt;
Idempotenz über mehrere Läufe; Re-Apply eines geänderten Specs; Recovery einer
„abgestürzten" Instanz; DesiredState-Fehler bricht ab; Per-Feed-Fehler bricht
**nicht** ab (übrige laufen, der fehlerhafte ist `failed`); `Run` reconciled
wiederholt bis Cancel — unter `-race`.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-003** (Reconciler).

## Nächste Häppchen

- **ORCH-2c** — getrennte Control-Plane: Store-Adapter für `DesiredState`
  (Feeds ⨝ Subscriptions + Quell-Konfig), Secret-Auflösung beim Launch,
  Änderungs-Trigger, Least-Privilege-Prozess-Trennung.
- **ORCH-2b** — echter Docker-`Backend`-Adapter (`Spec` → Container).
