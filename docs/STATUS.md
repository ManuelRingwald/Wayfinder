# Wayfinder – Aktueller Stand

**Letzte Sitzung:** 2026-06-13 (Sonnet 4.6)  
**Branch:** `claude/firefly-asd-implementation-8qbcsq`  
**Meilenstein:** M1 – WebSocket-Client + Datenmodell (Schritt 3 von 4)

---

## ✅ Fertig (bisherige Sitzung)

### Charter & Doku
- `CLAUDE.md` – Projekt-Charter mit goldener Regel, Komplexitäts-Skala S1–S5, Cloud-native & Zertifizierungs-Anforderungen
- `docs/glossary.md` – Luft­fahrt-/ASD-Fachbegriffe (Track, PSR/SSR, Cloud-native, etc.)
- `docs/decisions/ADR0001.md` – Tech-Stack-Entscheidung: Go + WebSocket (`coder/websocket`) + Datenmodell
- `docs/cross-project/` – Austausch-Struktur für Wayfinder ↔ Firefly Findings
- `docs/cross-project/todo-for-firefly.md` – 5 Produktionslücken in Firefly's `/ws`-Schnittstelle

### Code (M1, Schritte 1–2)

**Schritt 1 – Go-Projekt-Grundgerüst:**
- `go.mod` (`github.com/ManuelRingwald/Wayfinder`)
- `internal/server/` – `/healthz` (Liveness) und `/readyz` (Readiness), mit Tests
- `internal/config/` – Env-Var-basierte Konfiguration (`WAYFINDER_PORT`, `WAYFINDER_LOG_FORMAT`)
- `cmd/wayfinder/main.go` – HTTP-Server mit `slog` (Text/JSON) und Graceful Shutdown
- Qualitäts-Gates: ✅ `go vet`, `go test`, `golangci-lint`, `gofmt`; Funktionstest ✅

**Schritt 2 – Track-Datenmodell:**
- `internal/firefly/frame.go` – `Frame`, `FrameTrack`, `FramePlot` (Firefly's Wire-Format)
- Tests: Decodieren echter Firefly-JSON, Round-Trip, leere Arrays, Feldnamen-Konsistenz
- Anforderung FR-DATA-001 ins Register eingetragen
- Qualitäts-Gates: ✅ alle grün

---

## ⏳ Nächste Schritte (M1, Schritt 3 & 4)

### Schritt 3 – WebSocket-Client (S3, Sonnet)

**Fachlich:** Wayfinder empfängt Live-Frame-Updates von Firefly über `/ws`, mit automatischem Reconnect bei Netzwerk-Ausfällen.

**Technisch:**
- `internal/firefly/client.go`: `Client`-Typ mit `Connect(ctx, url)`, Reader-Loop über `coder/websocket`
- Unterscheidung `Frame` vs. `delay_triggered`-Event (generischer JSON-Vor-Parse)
- Reconnect-Backoff (z.B. exponentiell: 100ms → 1s → 5s)
- `/readyz` an Verbindungsstatus koppeln via `AtomicReadiness`
- Tests mit Mock-WS-Server (`httptest.Server`)
- Config um `FIREFLY_ADDR` (Host:Port) erweitern

### Schritt 4 – Integration ins main (S1, Haiku)
- `cmd/wayfinder/main.go` um Firefly-Client starten/stoppen erweitern
- Graceful Shutdown koordiniert WebSocket + HTTP
- Anforderungs-Register aktualisieren

---

## ⚠️ Achtung für nächste Sitzung

- **Cross-Repo-Zugriff:** Funktioniert noch nicht (nur `manuelringwald/wayfinder` in dieser Session)
  - Wenn du `docs/cross-project/todo-for-firefly.md` ins Firefly-Projekt übertragen möchtest: manuell oder neue Session mit beiden Repos
- **Modell:** Schritt 3 braucht S3 (Sonnet); ggf. hochfahren
- **Firefly-Schnittstelle:** Alle 5 Findings aus `todo-for-firefly.md` sind produktionsrelevant — ins Firefly-Projekt übertragen, wenn möglich

---

## Offene Fragen

- **M2 (Kartendarstellung):** Canvas/WebGL vs. Web-Framework (z.B. leaflet.js/MapLibre)?
