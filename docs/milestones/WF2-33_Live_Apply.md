# WF2-33 — Live-Apply (laufende Subscriptions re-skopieren, ohne Reconnect)

> **Stufe:** 3 · **Paket:** WF2-33 — **schließt Stufe 3 ab** · **Einstufung:**
> 🔒 S4 · Opus 4.8 (subtile Nebenläufigkeit am Isolations-Rand) · **Grundlage:**
> ADR 0005 (Mandanten-Isolation), baut auf WF2-21 (Scope/Fan-out) + WF2-31/31b/32
> (Admin-API/UI).

## Warum (fachlich)

Ein ASD darf für eine Konfigurationsänderung **nicht flackern**. Bisher wirkte
eine View- oder Abo-Änderung erst beim **Reconnect** (der Scope wurde nur beim
`/ws`-Connect aufgelöst). Für nahtlose, unterbrechungsfreie Übergänge zieht
WF2-33 den Scope einer **aktiven** Verbindung **live** nach: Der Lotse behält sein
Lagebild, die Grenze verschiebt sich im Hintergrund.

## Die zwei Leitplanken des Projektverantwortlichen

### 1. Thread-Safety am heißen Pfad
Der Broadcaster ist ein **Single-Goroutine-Actor**: Registrierung, Deregistrierung
und die Track-Auswertung (`broadcastTracks`) laufen **alle** im einen `Run`-Select-
Loop; `Client.scope` wird ausschließlich dort gelesen **und** geschrieben. WF2-33
nutzt genau das: Der Scope-Tausch ist ein **weiteres Kommando durch denselben
Loop** (`rescopeChan`). Damit gilt **per Konstruktion**:

- **Kein Lock** auf dem heißen Pfad — der Tausch und die Track-Evaluation teilen
  sich eine Goroutine, können also nie kollidieren (keine Race-Condition).
- **Der Run-Loop wird nie blockiert** — `ApplyScopes` schiebt nur auf einen
  gepufferten Channel (ctx-begrenzt, damit ein Shutdown nicht deadlockt).

`TestRescopeRaceUnderLoad` fährt das unter `-race` (parallele Track-Batches +
Re-Scopes) gegen die Wand — sauber.

### 2. Shrinking: keep it simple
Verkleinert ein Admin die AOI, sendet der Server für nun außenliegende Tracks
**einfach keine Updates mehr** — **keine** expliziten Lösch-Signale. Die bestehende
Frontend-Coast/Drop-Logik lässt sie über den regulären Client-Timeout **sanft
auslaufen**. Das ist **null Zusatzcode** — die natürliche Folge des getauschten
Scopes (`filterView` lässt sie fallen).

## Was (technisch)

**Zwei-Phasen-Re-Scope, DB-I/O abseits des heißen Pfads:**

1. **Snapshot** (`broadcast.ClientsForTenant`): liefert pro Tenant die betroffenen
   Clients. Liest nur **immutable Identity-Felder** (`tenantID`/`userID`, am Client
   bei Registrierung gepinnt) — **nie** den mutablen `scope` —, daher
   concurrent-safe neben dem Run-Loop.
2. **Resolve** (`cmd/wayfinder.rescopeTenant` → `resolveScope`): die Admin-Handler-
   Goroutine löst die neuen Scopes auf (Feeds sind pro-Tenant → einmal; die
   **effektive View kann pro User abweichen** → pro distinct User). `resolveScope`
   ist exakt die Logik des Connect-Pfads (refaktoriert aus `newScopeResolver`), also
   landet eine live-nachgezogene Verbindung im **identischen** Scope wie eine frisch
   verbundene — inklusive User-Overrides.
3. **Apply** (`broadcast.ApplyScopes` → Run-Loop): tauscht die Scopes in `Run` ein.
   Clients, die zwischen Snapshot und Apply weggingen, werden **übersprungen**
   (`clients.Load`-Guard → kein Senden auf einen geschlossenen Channel).

**Auslöser** (`pkg/adminapi`, entkoppelt über einen injizierten `RescopeFunc`):
`PUT /api/admin/view` (Tenant aus Identity) sowie `POST`/`DELETE …/subscriptions`
(Tenant aus dem Pfad) rufen nach Erfolg `triggerRescope`. Bei Validierungsfehler
(`400`) wird **nicht** re-skopiert.

- Dateien: `pkg/broadcast/broadcast.go` (`Scope.UserID`, immutable Client-Identity,
  `ClientsForTenant`/`ApplyScopes`/`applyScopes`/`rescopeChan` + Run-Case),
  `cmd/wayfinder/main.go` (`resolveScope`-Refactor + `rescopeTenant` + Verdrahtung),
  `pkg/adminapi/adminapi.go` (`RescopeFunc`-Hook).
- **Kein Schema-Change, keine neue Abhängigkeit.**

## Sicherheit / Korrektheit

- **Isolations-Rand bleibt intakt:** Der getauschte Scope ist dieselbe geprüfte
  Struktur wie beim Connect; ein Grant erweitert, ein Revoke verengt — beides sofort
  wirksam. Eine fehlgeschlagene Auflösung lässt **alle** Scopes unverändert (fail-
  safe: lieber alter, gültiger Scope als ein halber).
- **Per-User-Korrektheit:** Da der Connect-Pfad pro User auflöst (`GetEffective`),
  tut es der Re-Scope auch — ein per-User-Override wird nicht vom Tenant-Default
  überschrieben.

## Tests

- **`pkg/broadcast/rescope_test.go`:** `TestApplyScopesShrinkAOILive` (AOI-Shrink →
  außenliegender Track fällt weg, Verbindung bleibt; kein Delete),
  `TestApplyScopesGrantAndRevokeFeedLive`, `TestApplyScopesOnlyTargetClients`,
  `TestApplyScopesSkipsUnknownClient` (Disconnect-Race-Guard), `TestClientsForTenant
  Snapshot`, **`TestRescopeRaceUnderLoad`** (`-race`, 3000 Batches × 800 Re-Scopes).
- **`cmd/wayfinder/rescope_test.go`:** `TestResolveScopeBuildsScope` (geteilte
  Auflösung), `TestRescopeTenantAppliesLive` (Ende-zu-Ende: Feed-2 erst nach
  Live-Grant zugestellt, gegen echten Broadcaster).
- **`pkg/adminapi/adminapi_test.go`:** `TestPutViewTriggersRescope`,
  `TestPutViewInvalidDoesNotRescope` (kein Re-Scope bei `400`),
  `TestGrantTriggersRescope`/`TestRevokeTriggersRescope` (Ziel-Tenant korrekt).

Gates grün: `go build/vet/test`, **`-race`** (cmd/adminapi/broadcast/ws), `gofmt`,
`scripts/pg-test.sh`.

## Abgrenzung / Nächstes

- **Damit ist Stufe 3 komplett** (dynamische Konfiguration: Admin-API + UI + Live-
  Apply). **WF2-30** (Config-Cache) bleibt zurückgestellt (YAGNI, bis Last-Metriken
  ihn fordern).
- **Nicht enthalten:** Push einer „Sicht geändert"-Notiz an den Browser (rein
  serverseitiger Scope-Tausch genügt); Live-Apply bei Tenant-/User-Anlage über die
  API (heute CLI).
- **Nächster Schritt:** Stufe 4 (Sensor-/Stream-Management) oder ASD-Kern nach
  Abstimmung & „Go".
