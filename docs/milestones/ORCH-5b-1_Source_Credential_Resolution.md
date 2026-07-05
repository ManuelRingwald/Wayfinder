# ORCH-5b-1 — Quell-Credential-Auflösung in der Control-Plane

> Zweiter Schritt von ORCH-5: Der Orchestrator löst die per-Feed-`cred_ref`s zu
> Klartext-Credentials auf und injiziert sie in den gespawnten Firefly-Container —
> **nur** in der getrennten Least-Privilege-Control-Plane, nie am Browser-Rand.
> Aufbauend auf ORCH-5a (Rendering ohne Werte). UI-Zwei-Felder (UX-2) folgt als
> ORCH-5b-2.

## Fachlicher Hintergrund

ORCH-5a rendert `FIREFLY_SOURCES` mit `cred_env`-**Namen**, aber ohne Werte — eine
credentialled Quelle (z. B. authentifiziertes OpenSky) verweist auf eine noch
leere Cred-Env. ORCH-5b-1 schließt die Lücke: Wo ein Deployment-Key vorhanden ist,
entschlüsselt die Control-Plane die hinterlegten Secrets und übergibt die Werte
dem Tracker, sodass der Feed seine Live-Quelle authentifiziert nutzen kann.

## Architektur-Entscheidung — Variante A (Auflösung in der Control-Plane)

Der Klartext fließt in die Container-Env, **damit** eine Secret-Rotation den
Spec-Hash ändert und der Reconciler die Instanz mit dem neuen Wert neu startet
(sonst bliebe eine rotierte Credential ohne Restart wirkungslos). Bewusst **nicht**
Variante B (Spec trägt nur Referenzen, Backend löst auf): die Auflösung gehört in
den einen Prozess, der den Schlüssel ohnehin hält, und der Spec-Hash soll die
*effektive* Konfiguration abbilden. Der Klartext lebt nur in-memory im
Orchestrator, nie persistiert, nie geloggt, nie am Browser-Rand.

## Was umgesetzt wurde

- **`pkg/instance/instance.go`:** `Spec.ResolvedSecrets map[string]string` (cred_ref
  → Klartext). Gefüllt von der Control-Plane, **nie** von `SpecFromFeed` (bleibt
  rein); nil, wenn kein Key/keine credentialled Quelle.
- **`pkg/orchestrator/desired.go`:** `FeedSecretResolver`-Interface, `secrets`-Feld +
  `WithSecretResolver(r, logger)`-Builder. `resolveSecrets` füllt je Spec
  `ResolvedSecrets` **best-effort**: ein nicht auflösbarer Ref (kein Eintrag/falscher
  Key/manipuliert) wird auf WARN geloggt und ausgelassen → die Quelle läuft anonym,
  **kein** Reconcile-Abbruch. Der Klartext wird **nie** geloggt (nur Ref + Fehler).
- **`pkg/dockerbackend/sources.go`:** `fireflySourcesEnv(sources, resolved)` ersetzt
  `fireflySourcesJSON`. `cred_env` wird **nur** gesetzt — und ein
  `FIREFLY_SOURCE_<i>_SECRET=<wert>`-Env emittiert — wenn der Ref zu einem
  nicht-leeren Wert auflöste; sonst anonym (kein `cred_env`). Der Wert steht **nie**
  im JSON-Blob.
- **`pkg/dockerbackend/backend.go`:** `fireflyEnv` hängt die Cred-Envs an.
- **`cmd/wayfinder-orchestrator/main.go`:** `WAYFINDER_SECRET_KEY` (base64-32-Byte)
  → `secret.Cipher` → `orchestrator.NewSecretResolver(store.NewSecretRepo(pool),
  cipher)` → `desired.WithSecretResolver`. Fehlt/ungültig der Key, ist die Auflösung
  aus (Quellen anonym); gesetzt-aber-ungültig wird laut geloggt.

## Sicherheits-/Schnittstellen-Betrachtung

- **Klartext nur im Least-Privilege-Prozess:** `WAYFINDER_SECRET_KEY` und die
  aufgelösten Werte leben ausschließlich im `wayfinder-orchestrator`, nie im
  Browser-Server. Werte nie im Log, nie im `FIREFLY_SOURCES`-JSON.
- **Best-effort statt fail-closed beim Feed-Verlust:** ein fehlendes Einzel-Secret
  degradiert nur die betroffene Quelle zu anonym — der Reconcile-Lauf bricht nicht
  ab (sonst würden alle Instanzen wegen eines Tippfehlers verwaisen).
- **Rotation wirkt:** Wert in der Env → Spec-Hash ändert sich → Restart mit neuem
  Wert.
- **Eingangs-Kontrakt** (Firefly v1.0.0); **CAT062-Ausgabe unberührt**.

## Tests

- `pkg/orchestrator/desired_test.go`: Resolver verdrahtet → `ResolvedSecrets`
  gefüllt; nicht auflösbares Secret → anonym (kein Abbruch); ohne Resolver →
  `ResolvedSecrets` nil, Referenzen bleiben erhalten.
- `pkg/dockerbackend/sources_test.go`: aufgelöste Cred → Wert nur in der Env, nie
  im JSON; unaufgelöste Cred → kein `cred_env`; Cred per Index; Determinismus.
- `cmd/wayfinder-orchestrator/main_test.go::TestLoadConfigSecretKey`: gültiger Key
  dekodiert, leer/ungültig → nil ohne Fehler.
- `go test ./...`, `go vet`, `gofmt`, `golangci-lint` grün.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-006** (ORCH-5b-1 ✅; 5b-2 UI folgt) und
**NFR-SEC-004** (Container-Injection ✅) nachgezogen.

## Nächster Schritt (ORCH-5b-2)

UI-Zwei-Felder im Admin (Feed → Quellen → Secret): Benutzername + Passwort werden
zu **einem** verschlüsselten `user:pass`-Secret kombiniert (UX-2). Danach
End-to-End-Abnahme.

## Nachtrag (#177) — Secret-Änderung löst prompten Reconcile aus

Ein hinterlegter/rotierter Credential ist seit ORCH-5b-1 **spec-relevant** (er
fließt über `Spec.ResolvedSecrets` in den Spec-Hash), wurde aber vom
**änderungs-getriebenen** Reconcile nicht erfasst: der NOTIFY-Trigger deckte nur
`feeds`/`subscriptions` ab (`00012`), `feed_secrets` war bewusst ausgenommen — mit
einer inzwischen überholten Begründung. Folge: ein neu hinterlegter OpenSky-Key
wirkte erst beim nächsten Intervall-Lauf, für den Betreiber „ohne Wirkung". Fix:
Migration **`00020`** hängt `feed_secrets` (INSERT/UPDATE/DELETE) an
`wayfinder_notify_reconcile()` (gleiche Statement-Level-Form wie feeds/subs); der
überholte Kommentar in `00012` verweist jetzt darauf. Guard-Test
`pkg/store/migrate_test.go::TestFeedSecretsReconcileTriggerMigration`.
**Vorbedingung** bleibt ein gültiger `WAYFINDER_SECRET_KEY` (#171) — ohne ihn wird
gar kein Secret gespeichert/aufgelöst.
