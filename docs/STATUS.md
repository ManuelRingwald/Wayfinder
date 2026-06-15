# Arbeitsstand (Handover-Notiz)

> **Zweck:** Diese Datei ist der schnelle Wiedereinstieg — egal ob am PC oder
> Handy. Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

- **Zuletzt aktualisiert:** 2026-06-15 (Branch `claude/callsign-i062-245`:
  Doku-/Docker-Vorbereitung fürs Testen. `README.md` komplett neu (Quickstart
  Docker/lokal, Architektur-Übersicht, Konfig-Tabelle, Build & Test, Links).
  Neu: `Dockerfile` (Multi-Stage `golang:1.23-bookworm` → `debian:bookworm-
  slim`, Healthcheck `/health`), `docker-compose.yml` (`network_mode: host` —
  notwendig für CAT062-Multicast-Empfang), `.dockerignore`, `DOCKER.md`
  (Standalone + End-to-End mit Firefly, inkl. `FIREFLY_CAT062_ENABLED=true` und
  macOS/Windows-Docker-Desktop-Hinweis). Firefly-seitig analoger Abschnitt in
  README/DOCKER.md ergänzt. Docker-Build konnte in dieser Sitzung nicht
  getestet werden (kein Docker-Daemon verfügbar) — `go build`/`go vet`/
  `go test ./...` sind grün. Nächster Schritt: AP7/AP8 (Callsign I062/245).)
- **Vorherige Aktualisierung:** 2026-06-14 (Branch `claude/serene-heisenberg-xq4rla`:
  AP2 — Vertikallage I062/136 + UAP-Standardtreue; davor Kurs-Pfeile + Trails)
- **Branch:** `claude/serene-heisenberg-xq4rla` — **M1.1–M1.3 abgeschlossen**
  (CAT062 Multicast → Decoder → Broadcaster → WebSocket-Clients, in `main`).
  **M1.4.a/b/c abgeschlossen**: `internal/webui` (eingebettetes Frontend),
  MapLibre GL JS Karte, WebSocket-Client mit Reconnect, Live-Tracks als
  farbige Kartensymbole (grün=confirmed, grau=tentativ, orange=coasting) mit
  Track-Nummern-Labels. Siehe `docs/milestones/M1.4.c_Track_Rendering.md`.
  **M1 ist funktional abgeschlossen** (Backend-Pipeline + Live-Kartendarstellung).
  **Neu (post-M1, UI-Häppchen A.1)**: Kurs-Pfeile (ASD-Speed-Vector-Line,
  60s-Vorausschau) je Track in `internal/webui/static/app.js` — eigene
  GeoJSON-Quelle `track-vectors`/Layer `track-vectors-lines`, berechnet aus
  `vx`/`vy` (m/s, Ost/Nord) per flacher Erdnäherung. Live gegen Firefly
  (CAT062-Multicast) verifiziert.
  **Neu (post-M1, UI-Häppchen A.2)**: Track-Trails — die letzten 20 Positionen
  je Track werden im Frontend-State (`state.trackHistory`) gehalten und als
  blassgraue Spur (`track-trails`/`track-trails-lines`) gerendert; History wird
  bereinigt, sobald ein Track aus dem Update verschwindet. Live gegen Firefly
  verifiziert.
  **Neu (AP2, ICD-Thema): Vertikallage I062/136 + UAP-Standardtreue** (lockstep
  zu Fireflys ADR 0015 / ICD 2.0.0, Issue #5 `from-firefly`). Decoder
  (`pkg/cat062`) zieht nach: **I062/500 von FRN 16 → FRN 27** und neues
  optionales **I062/136** (FRN 17, signed i16, LSB 1/4 FL = 25 ft).
  `DecodedTrack.FlightLevelFt` + `broadcast.TrackMessage.flight_level_ft`
  durchgereicht; `app.js` zeigt die Flugfläche als zweite Label-Zeile „FLnnn"
  (ASD-Datablock-Stil). Referenz-Vektor-Test aktualisiert (FSPEC
  `[0x9F,0x0F,0x01,0x04]`, LEN 40) + neuer `TestDecodeFlightLevel`. Live gegen
  Firefly verifiziert (FL372/FL340 im WS-Strom). → Issue #5 kann nach Merge
  geschlossen werden.
  Nächster Schritt: AP7/AP8 (Callsign I062/245), AP5/AP6 (CAT065 Heartbeat),
  weitere UI-Häppchen.

> 🔁 **Pivot vollzogen: Wayfinder konsumiert CAT062/UDP-Multicast statt
> JSON/WebSocket.** `CLAUDE.md` wurde komplett neu gefasst (Produktionsbetrieb,
> Modell-Angabe pro Schritt jetzt Pflicht, Abschnitt 2 = vollständiger
> CAT062-Draht-Vertrag mit FRN/Item-Tabelle). Begründung und Konsequenzen stehen
> in Fireflys `docs/decisions/0014-produktionsbetrieb-statt-lernprojekt-wayfinder-cat062.md`.
>
> Cross-Project-Status (`docs/cross-project/todo-for-firefly.md`): Issues
> **#6, #8, #10** geschlossen (durch CAT062-Architektur gegenstandslos), **#7**
> transformiert (Netz-Isolation Multicast + Wayfinder-Browser-Rand), **#9** (UTC
> Time-of-Day) bleibt offen und wird zentraler.

---

## 1. Wo wir gerade stehen

**M1.3 (WebSocket-Server — Browser-Push): ✅ Abgeschlossen**

Implementiert:
- ✅ `pkg/broadcast/broadcast.go` — Broadcaster mit Channel-basierter Architektur
  - Track-Channel-Input: `broadcaster.TracksChan() <- tracks`
  - Client-Registry (sync.Map) für non-blocking Broadcast
  - Automatische Eviction bei vollem Client-Channel
  - Message-Format: JSON mit Track-Array
- ✅ `pkg/ws/handler.go` — HTTP-Handler für WebSocket-Upgrade
  - Client-Lifecycle: register → readLoop + writeLoop → unregister
  - Ping/Pong für Keepalive
  - WriteJSON für Message-Serialisierung
- ✅ Integration in `main.go`:
  - Receiver → Broadcaster → WebSocket-Clients (volle Pipeline)
  - Graceful shutdown mit Goroutine-Sync (sync.WaitGroup)
  - Readiness probe: ready wenn Clients verbunden ODER Blocks empfangen
  - WebSocket auf `:8081` (/ws endpoint)

**Qualitäts-Gates:** `go build ./cmd/wayfinder` ✅, `go test ./...` ✅

---

**M1.2 (UDP-Multicast-Receiver): ✅ Abgeschlossen**

Implementiert:
- ✅ `pkg/receiver/receiver.go` — UDP-Multicast-Listener mit CAT062-Decoder-Integration
  - Multicast-Bindung auf `239.255.0.62:8600` (oder env. konfigurierbar via `FIREFLY_CAT062_GROUP`/`FIREFLY_CAT062_PORT`)
  - Handler-Pattern für Track-Verarbeitung (jedes Datagramm = ein Block mit 0+ Tracks)
  - Fehlerbehandlung: truncated/malformed Blöcke werden geloggt und ignoriert (kein Panic)
- ✅ `cmd/wayfinder/main.go` — Server-Einstieg mit
  - Umgebungs-Konfiguration (12-Factor)
  - `/health` (Liveness) und `/ready` (Readiness) probes für Container/Kubernetes
  - Graceful shutdown auf SIGINT/SIGTERM
  - Strukturiertes JSON-Logging (stderr)
- ✅ `pkg/receiver/receiver_test.go` — 5 Tests (Config, Invalid Group, Run/Listen, Context Cancellation)

**Integrationen:** CAT062-Decoder ist direkt in den Receiver-Handler verdrahtet — die ersten Datenpakete von Firefly können jetzt empfangen und dekodiert werden.

---

**M1.1 (CAT062-Decoder-Grundgerüst): ✅ Abgeschlossen**

Implementiert:
- ✅ `pkg/cat062/types.go` — DecodedTrack, DataSourceID, TimeOfDay, WGS84Position, CartesianPosition, Velocity, TrackStatus, UpdateAge, PositionAccuracy
- ✅ `pkg/cat062/fspec.go` — FSPEC-Parser mit FX-Chaining
- ✅ `pkg/cat062/decoder.go` — DecodeDataBlock, DecodeRecord mit FRN 1,4,5,6,7,9,11,12,13,14,16
  - FRN1 (I062/010): SAC/SIC ✅
  - FRN4 (I062/070): Time-of-Day ✅
  - FRN5 (I062/105): WGS84-Position ✅
  - FRN6 (I062/100): System-Cartesian (i24 **sign-extension**) ✅
  - FRN7 (I062/185): Velocity ✅
  - FRN9 (I062/060): Mode 3/A ✅
  - FRN11 (I062/380): ICAO-Adresse ✅
  - FRN12 (I062/040): Track-Nummer ✅
  - FRN13 (I062/080): Track-Status (variable FX, vereinfacht) ✅
  - FRN14 (I062/290): PSR-Age ✅
  - FRN16 (I062/500): Position-Genauigkeit (APC) ✅
- ✅ `pkg/cat062/decoder_test.go` — **alle 10 Tests grün** (TestSignExtendI24,
  TestFSPECParser, TestDecodeDataSourceID, TestDecodeTimeOfDay,
  TestDecodeWGS84Position, TestDecodeVelocity, TestDecodeCartesianPosition,
  TestDecodeMultipleTracks, TestReferenceVector, BenchmarkDecodeRecord)

**Validierung gegen Firefly (M1.1.d):**
- `TestReferenceVector` dekodiert den byte-exakten Dump aus Fireflys
  `single_track_matches_reference_dump` (firefly-asterix/src/cat062.rs) und
  prüft alle Felder gegen die dort erzeugten Werte — der Wire-Vertrag zwischen
  Firefly (Encoder) und Wayfinder (Decoder) ist somit Ende-zu-Ende verifiziert.

**Qualitäts-Gates:** `go test ./...` ✅, `go vet ./...` ✅, `gofmt` ✅

## 2. Gesetzte Entscheidungen (Fundament)

| Thema | Entscheidung | Quelle | Status |
|-------|--------------|--------|--------|
| Betriebsmodus | Produktionsbetrieb (nicht Lernprojekt) | Fireflys ADR 0014 | ✅ |
| Schnittstelle | **CAT062 over UDP-Multicast** (nicht JSON/WebSocket) | Fireflys ADR 0006 + 0014, `CLAUDE.md` Abschnitt 2 | ✅ |
| Sprache | Code Englisch, Doku/Chat Deutsch | `CLAUDE.md` Abschnitt 4 | ✅ |
| Stack | Go + MapLibre GL JS + WebSocket-Server-Push | `CLAUDE.md` Abschnitt 5, ADR 0001 | ✅ ratifiziert 2026-06-13 |

## 3. Nächster Schritt (hier geht es weiter!)

➡️ **M1.3 (WebSocket-Server — Browser-Push für Live-Tracks)**

Live-Tracks vom Receiver an Browser-Clients über WebSocket:
- Go-WebSocket-Server (mit gorilla/websocket oder stdlib `net/http/httputil/reverseproxy`)
- Receiver → Channel → WebSocket-Broadcast-Loop
- Message format: JSON mit Track-Array (lat, lon, vx, vy, track_num, status, …)
- Client-Verbindung: `/ws`, optional mit Cookie/Bearer-Auth
- Health: WebSocket-Clients zählen als Readiness-Indikator

Danach M1.4 (Frontend mit MapLibre GL JS).

Erst Erklärung → Rückfragen/Go → dann kleine, testbare Umsetzung
(`CLAUDE.md` Abschnitt 3).

## 4. So steige ich wieder ein (Kurzbefehle)

```bash
# Tests laufen und Dekodierung prüfen
go test ./pkg/cat062 -v

# Oder einzelne Tests:
go test ./pkg/cat062 -run TestDecodeCartesianPosition -v

# Code ansehen:
ls -la pkg/cat062/
# decoder.go        (Haupt-Dekoder + i24-Helfer)
# decoder_test.go   (Tests)
# types.go          (Domain-Typen)
# fspec.go          (FSPEC-Parser)
```

Einstieg:
- `CLAUDE.md` Abschnitt 2 = CAT062-Draht-Vertrag + FRN-Tabelle
- `docs/cross-project/todo-for-firefly.md` = Cross-Project-Status
- Commit-Log: `git log --oneline | head` (letzte Arbeit + Messaging)
