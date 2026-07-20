# K0 — Config-Plane für Kartendaten: DB-Override über Env + Hot-Reload

> **Register:** FR-CFG-007 · **Entscheidung:** ADR 0033 · **Epic:** #307 (Admin-
> Bereich „Kartendaten"), Issue #308. **Fundament** — K1–K5 bauen darauf auf.

## Fachlich — warum

Die vier Karten-Datenquellen (Wetter, Basiskarte, Radar-Abdeckung, Aeronautik)
sollen im Admin **live** einstellbar sein, ohne Neustart. Drei davon sind heute
reine **Startup-Env**. K0 baut das **wiederverwendbare Fundament** dafür — noch
**ohne** UI oder Subsystem-Verdrahtung (das sind K1–K5): eine einheitliche Art,
eine Einstellung aus der DB zu überschreiben (mit Env als Rückfall), sie live neu
zu laden, und admin-gesetzte URLs sicher zu prüfen.

## Technisch — `pkg/mapconfig` (rein, unit-getestet)

- **`Setting`** — eine überschreibbare Einstellung: `platform_settings`-Key + Env-
  Default. **Effektiv = DB-Override, sonst Env-Default.** Leerer Wert / `Reset()`
  = zurück auf Default (Zeile gelöscht). Ein Store-Fehler **degradiert auf den
  Env-Default** (kein leerer Wert bei DB-Schluckauf). → 12-Factor bleibt: ein
  frisches Deployment ohne DB-Config läuft wie bisher.
- **`Registry` + `ReloadFunc`** — Hot-Reload-Dispatch je Domain. Ein Dienst
  registriert „Konfig X geändert → neu laden"; die Admin-PUT-Route triggert nach
  dem Speichern. **Defensiv:** bei Reload-Fehler behält der Dienst die letzte gute
  Konfig, der Fehler geht an die Antwort, **nie** ein Crash (CLAUDE §7).
- **`ValidateFetchURL`** — SSRF-Leitplanken für admin-gesetzte, **server-seitig
  gefetchte** URLs: nur `http`/`https`, Host-Pflicht, Ablehnung privater/Loopback-
  /Link-Local-/ULA-IPs + interner Namen (`localhost`/`*.local`/`*.internal`,
  Cloud-Metadaten `169.254.169.254`), optionale Host-Allowlist.
- **`Resource.Handler`** — generischer `GET/PUT`-Admin-Endpunkt: GET liefert
  effektiven Wert + „überschrieben?"-Flag + Env-Default; PUT validiert, speichert
  (leer = Reset), triggert Reload und meldet einen Reload-Fehler **ehrlich** als
  `reload_error` mit 200 (gespeichert, aber nicht angewandt → Dienst hielt
  letzte gute Konfig).

**Secrets** (OpenAIP-Key) laufen **nicht** über diese Plane — sie bleiben
versiegelt (`pkg/secret` + `platform_settings`, ADR 0018).

## Ehrliche Grenzen

- **DNS-Rebinding-SSRF** (öffentlicher Name → private IP) ist **nicht** abgedeckt
  (bräuchte Resolve-Zeit-Prüfung oder strikte Allowlist); beim Trusted-Admin-
  Modell akzeptiert, Verschärfung = Folge-Arbeit (ADR 0033).
- **Kein Subsystem live in K0** — die konkreten Panels + Hot-Reload-Pfade
  (basemap/weather/coverage) kommen in K2–K5; deren Korrektheit wird dort einzeln
  nachgewiesen.

## Tests

`pkg/mapconfig/mapconfig_test.go`: `Setting` (Default/Override/Reset/leer-Reset,
Store-Fehler-Degradation); `Registry` (Trigger ruft Fn, unknown = No-op,
Fehler-Wrap+Return, nil-Fn ignoriert); `ValidateFetchURL` (http/https ok, public
IP ok; Ablehnung file/gopher/kein-Host/localhost/.internal/.local/Loopback/privat/
Link-Local/IPv6-Loopback; Allowlist exakt + Suffix); `Resource.Handler`
(GET-State, PUT-valid speichert+reload, SSRF-URL → 400, leer = Reset, POST → 405,
Reload-Fehler → `reload_error` mit 200).
