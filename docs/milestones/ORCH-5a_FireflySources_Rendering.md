# ORCH-5a — `FIREFLY_SOURCES`-Rendering im Docker-Adapter

> Erster Schritt von ORCH-5 (Wayfinder-Gegenseite zu Fireflys Quell-Eingangs-
> Kontrakt, ADR 0023): der Docker-Adapter übersetzt die generische Quell-Liste
> eines Feeds in die `FIREFLY_SOURCES`-Env. Reine, testbare Rendering-Logik —
> **ohne** Secret-Werte (die Auflösung folgt ORCH-5b).

## Fachlicher Hintergrund

Die Auto-Orchestrierung startet pro Feed eine Firefly-Instanz; Firefly liest seine
Live-Quellen aus `FIREFLY_SOURCES` (Firefly ADR 0023, `source-input-contract.md`
v1.0.0). ORCH-5a baut die Wayfinder-Seite dieses Drahts: `store.SourceConfig` →
`FIREFLY_SOURCES`-JSON + `FIREFLY_MODE=live`.

## Was umgesetzt wurde (`pkg/dockerbackend`)

- **`sources.go`:** `fireflySource`-DTO (Kontrakt-Form: `type`, `bbox`, `sac`,
  `sic`, `cred_env`) + `fireflySourcesJSON(sources)` → deterministisches JSON-Array
  (`store.BBox` trägt bereits die `min_lat…`-JSON-Tags des Kontrakts). Je Quelle mit
  `cred_ref` wird ein `cred_env`-**Name** (`credEnvName(i)` = `FIREFLY_SOURCE_<i>_SECRET`)
  gesetzt — der Credential-**Wert** kommt **nie** in den Blob.
- **`backend.go`:** `fireflyEnv` setzt bei vorhandenen Quellen `FIREFLY_MODE=live`
  + `FIREFLY_SOURCES`; ohne Quellen bleibt der Scene-Fallback. Reihenfolge
  deterministisch → stabiler Spec-Hash (Drift-Erkennung unverändert korrekt).

## Sicherheits-/Schnittstellen-Betrachtung

- **Kein Secret im Rendering:** nur `cred_env`-Namen; die Werte injiziert die
  Control-Plane separat (ORCH-5b) — Klartext bleibt aus dem Spec/Hash und aus dem
  JSON-Blob.
- **Eingangs-Kontrakt** (Firefly v1.0.0); **CAT062-Ausgabe unberührt**.
- **Ehrliche Grenze:** Bis ORCH-5b laufen nur credential-lose Quellen (anonymes
  OpenSky) vollständig; eine credentialled Quelle referenziert eine noch nicht
  gesetzte Cred-Env (Firefly meldet das sauber).

## Tests

`pkg/dockerbackend/sources_test.go`: leere Liste → kein `FIREFLY_SOURCES`;
adsb+Cred → Kontrakt-Form inkl. `cred_env`, **kein** `cred_ref`/Wert im JSON;
Radar (sac/sic, kein bbox) + anonyme Quelle (kein `cred_env`); `cred_env`-Name per
Index; Determinismus. `TestFireflyEnvMapsSpec` (Scene-Fallback) unverändert grün.
`go test ./...`, `go vet`, `gofmt` grün.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-006** (ORCH-5a ✅; 5b folgt); FR-ORCH-002-„Stand"
nachgezogen.

## Nächster Schritt (ORCH-5b)

`SecretResolver` im `wayfinder-orchestrator` verdrahten (`WAYFINDER_SECRET_KEY` →
Open beim Start), `cred_ref` → `user:pass` auflösen und in die benannten Cred-Envs
injizieren; UI-Zwei-Felder (Benutzername/Passwort, UX-2). **Sicherheits-relevant
(Secret-Fluss in den Container) → eigene Ankündigung & „Go".**
