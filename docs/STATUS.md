# Arbeitsstand (Handover-Notiz)

> **Zweck:** Diese Datei ist der schnelle Wiedereinstieg — egal ob am PC oder
> Handy. Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

> 🗺️ **Roadmap:** Arbeitspakete, Findings und empfohlene Reihenfolge stehen in
> `docs/ROADMAP.md` (Stichwort „Roadmap" im Chat zeigt diese Liste).

- **Zuletzt aktualisiert:** 2026-06-16 — **Paket #14 / ASD-004 „Track-Lebenszyklus
  & History-Darstellung" abgeschlossen.** Rein Frontend (`app.js`), kein Backend-Change.
  **ASD-004a History-Dots:** Neuer Source `track-history-dots` + `circle`-Layer
  `track-history-dots-circles` (Radius 2 px, Trail-Farbe); rendert jeden
  `state.trackHistory`-Eintrag als einzelnen Punkt zwischen Trail-Linie und
  Speed-Vector — klassisches Radar-Instrument (Punktabstand = Geschwindigkeit,
  Krümmung = Drehrate). **ASD-004b Coasting-Abdunkeln:** Alle fünf Track-Layer
  erhalten datengesteuerte Opacity-`case`-Expressions: coasting → circle-opacity
  0.5, text/vector-opacity 0.35, trail/dot-opacity 0.2; `state.trackCoasting:
  Map<track_num, boolean>` führt den Zustand für Trail/Dot-Features mit.
  **ASD-004c Graceful Fade-Out bei TSE:** TSE-Tracks landen in
  `state.fadingTracks: Map<track_num, {deadline, track}>` statt sofort weggefiltert
  zu werden; `renderSources()` mischt sie mit `fade_opacity`-Property (0–1) in alle
  vier GeoJSON-Sources; `tickFade()` läuft per `setInterval` (~50 ms) und räumt
  abgelaufene Tracks + ihre History auf. Paint-Expressions: `["has", "fade_opacity"]`
  hat Vorrang vor Coasting-Dimming. `updateTrackHistory` lässt Fading-Track-
  History stehen. Anforderung FR-ASD-004 im Register. Milestone
  `docs/milestones/ASD-004_Track_Lifecycle_History.md`. Gates grün
  (`go test ./...`, `go vet ./...`). S3 · Sonnet 4.6.
  Nächster Schritt: nächstes Roadmap-Paket nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-16 — **Paket #12 / ASD-001 „Erweiterter
  Data Block" abgeschlossen.** Rein Frontend (`app.js`), kein Backend-Change.
  **ASD-001a Ground Speed:** `buildLabel(track, vTrend)` erhält neue dritte
  Zeile mit Bodengeschwindigkeit in Knoten (`Math.hypot(vx, vy) × 1.9438`,
  gerundet, nur wenn > 0). **ASD-001b Steig-/Sinkflug-Indikator:**
  `state.trackFlHistory: Map` speichert letzte bekannte FL pro Track;
  `updateTracksLayer` berechnet FL-Delta, zeigt `▲` bei > +50 ft oder `▼`
  bei < −50 ft gegenüber dem Vorgänger-Scan (Schwellwert 50 ft = 2 LSB,
  filtert Mode-C-Quantisierungsrauschen). History wird parallel zu
  `trackHistory` bereinigt (Einträge verschwundener Tracks gelöscht).
  Alle vier Data-Block-Elemente gebündelt in `buildLabel`:
  `DLH123 / FL350 ▲ / 247`. Anforderung FR-ASD-001 im Register.
  Meilenstein `docs/milestones/ASD-001_Extended_Data_Block.md`. Gates grün
  (`go test ./...`, `go vet ./...`, `node --check app.js`). S2 · Sonnet 4.6.
  Nächster Schritt: nächstes Roadmap-Paket nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-15 — **Paket #13 / ASD-003 „Aeronautical
  Map Layer" abgeschlossen.** Vier Häppchen: **3a Radar Dark Mode** —
  `WAYFINDER_MAP_THEME` (`dark`|`osm`, Default dark), `darkMapStyle` (CARTO
  `dark_nolabels`, key-frei), `mapConfigHandler` liefert Style + `theme`;
  Frontend `PALETTES` wählt helle Labels/Vektoren/Trails auf dunklem Grund
  (FR-MAP-001). **3b OpenAIP-Backend** (ADR 0004) — neues Paket
  `pkg/aeronautical`: defensiver OpenAIP-Client (Timeout, 32-MiB-Limit,
  `validGeometry`), best-effort `Service` mit Last-Good-Cache + nicht-
  blockierendem Refresh (`WAYFINDER_OPENAIP_*`), Endpoints
  `/api/airspace|navaids|waypoints`, `/metrics`-Kennzahlen
  `wayfinder_openaip_*` (FR-MAP-002, NFR-OPS-004/SEC-002/OBS-004). Track-Pfad
  und `/ready` bleiben **vollständig entkoppelt**. **3c/3d Overlays** —
  Luftraum (fill/line/label, schaltbar), VOR/NDB + Waypoints als Symbol-Layer
  mit laufzeit-gezeichneten Icons (kein Sprite-Asset), Zoom-Böden gegen
  Clutter, Layer-Steuerungs-Panel (`#layer-control`); `loadAeronautical`
  zieht alle 5 min nach, Fehler nicht-fatal (FR-MAP-003/004). Gates grün
  (`go build/vet/test`, `gofmt`, `node --check app.js`); Rauchtest des Binaries
  bestätigt Dark-Theme, leere Collections ohne Key (graceful) und die Metriken.
  Modell: Opus 4.8 (S4 wegen 3b). Datenquellen-Entscheidung „Live-OpenAIP" vom
  Projektverantwortlichen getroffen. Nächster Schritt: nächstes Roadmap-Paket
  nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-15 — **Paket #3 „CAT065 Heartbeat"
  abgeschlossen (beide Seiten).** Wayfinder-Teil: neues Paket `pkg/cat065`
  (Decoder für CAT065 SDPS-Status, byte-genau gegen Fireflys Referenz-Dump,
  robust gegen Truncation/falsche Kategorie). Receiver dispatcht den
  gemeinsamen Multicast-Strom am führenden **CAT-Oktett** (`0x3E` → Track,
  `0x41` → Status, sonst Decode-Fehler) — neuer `dispatch`/`handleStatus`,
  `StatusHandler` in der Config, Test `TestDispatchRoutesByCategory`. Neues
  Paket `pkg/health` (`FeedHealth`): verfolgt Heartbeat-Ankunft, erkennt
  Staleness (kein Heartbeat seit > `WAYFINDER_FEED_STALE_TIMEOUT`, Default 3 s),
  `Observe` liefert nur Zustandswechsel. `main.go`: StatusHandler füttert
  Health + Heartbeat-Zähler, Monitor-Goroutine erkennt Staleness ohne Verkehr,
  `broadcastFeedStatus` pusht `feed_status`-WS-Nachricht (separater Pfad, leert
  **nicht** das Lagebild). Frontend: Feed-Banner (grün/rot/grau,
  `updateFeedBanner` in `app.js`, `#feed-status` in `index.html`). `/ready`
  wird bei stale Feed **nicht ready** (nur wenn je Heartbeat gesehen); `/metrics`
  um `wayfinder_cat065_heartbeats_received_total` + `wayfinder_feed_stale`
  ergänzt. `Message.FeedStatus`/`FeedStatusMessage` im Broadcaster. Doku:
  CLAUDE.md §2 (CAT065-Kurzfassung), Register FR-DATA-004/FR-OPS-004/NFR-OBS-003,
  ROADMAP/STATUS. Architektur-Entscheidung (gleiche Multicast-Gruppe, Dispatch
  am CAT-Oktett) vom Projektverantwortlichen bestätigt. **Firefly-Teil** (Sender:
  `firefly-asterix::cat065`, `run_heartbeat`, ADR 0018, ICD 2.3.0) ebenfalls
  fertig. Alle Gates grün (`go build/vet/test`, `gofmt`). Cross-Project-Issue
  (`from-firefly`) zum CAT065-Vertrag wird erstellt + nach beidseitiger
  Umsetzung geschlossen. Nächster Schritt: nächstes Roadmap-Paket nach
  Abstimmung (z. B. #4 Konfigurierbarer System-Referenzpunkt).
- **Vorherige Aktualisierung:** 2026-06-15 — Paket #2 „Observability-Grundgerüst"
  **abgeschlossen** mit Häppchen 2.3: gemeinsamer `/metrics`-Endpoint
  (Prometheus-Textformat). Wayfinder-Teil (NFR-OBS-002): neues Paket
  `pkg/metrics` (`Handler`/`Counter`/`Gauge`, hand-gerollte Prometheus-
  Exposition ohne externe Abhängigkeit). `Broadcaster` bekommt
  `EvictedCount()` (Eviction-Zähler, `pkg/broadcast/broadcast.go`),
  `Receiver` bekommt `DecodeErrorCount()` (`pkg/receiver/receiver.go`).
  `startProbeServer` (Port `:8080`) bekommt eine neue `/metrics`-Route neben
  `/health`/`/ready`: `wayfinder_cat062_blocks_received_total`/
  `wayfinder_cat062_tracks_received_total` (Counter),
  `wayfinder_cat062_decode_errors_total` (Counter),
  `wayfinder_tracks_current` (Gauge), `wayfinder_ws_clients_connected`
  (Gauge), `wayfinder_ws_clients_evicted_total` (Counter). Neue Tests:
  `pkg/metrics/metrics_test.go::TestHandlerRendersPrometheusExpositionFormat`,
  `pkg/broadcast/broadcast_test.go::TestBroadcastEvictsClientWithFullSendChannel`
  (jetzt zusätzlich `EvictedCount()`-Assertion),
  `pkg/receiver/receiver_test.go::TestReceiverDecodeErrorCountStartsAtZero`.
  Neue Anforderung NFR-OBS-002 im Register. Alle Gates grün
  (`go build`/`go vet`/`go test ./...`; `gofmt` clean außer dem
  vorbestehenden, unveränderten Befund in `pkg/receiver/receiver_test.go`).
  Firefly-Teil (Häppchen 2.2, `tracing` in `firefly-multicast`, und 2.3,
  `firefly-server::metrics`) ist ebenfalls erledigt — **Paket #2 vollständig
  abgeschlossen.** Nächster Schritt: nächstes Roadmap-Paket nach Abstimmung
  mit dem Projektverantwortlichen.
- **Vorherige Aktualisierung:** 2026-06-15 — Paket #1 „Multicast-Feed-Sicherheit",
  Häppchen 1.3: **Browser-Rand-Implementierung gemäß ADR 0003.**
  `pkg/ws/handler.go`: globales `CheckOrigin: func(r) bool { return true }`
  entfernt; `Handler` bekommt ein `allowedOrigins []string`-Feld und eine neue
  `checkOrigin`-Methode — Requests ohne `Origin`-Header (Nicht-Browser-Clients)
  und Same-Origin-Requests sind weiterhin erlaubt, Cross-Origin-Requests nur
  noch, wenn der `Origin`-Header in `WAYFINDER_ALLOWED_ORIGINS` steht (sonst
  fail-closed mit Warn-Log). `cmd/wayfinder/main.go`: neue `Config`-Felder
  `AllowedOrigins`, `AuthToken`, `TLSCertFile`, `TLSKeyFile`, alle per
  `loadConfig()` aus `WAYFINDER_ALLOWED_ORIGINS` (kommasepariert),
  `WAYFINDER_AUTH_TOKEN`, `WAYFINDER_TLS_CERT`/`_KEY` gelesen (Default: leer).
  Neue `authMiddleware`: greift nur, wenn `WAYFINDER_AUTH_TOKEN` gesetzt ist
  (sonst Pass-through + Warn-Log "relies on network isolation / reverse
  proxy"); prüft Bearer-Header oder `?token=`-Query-Param (Browser-WS kann
  keine Custom-Header beim Handshake setzen) via
  `crypto/subtle.ConstantTimeCompare`, sonst `401` + `WWW-Authenticate:
  Bearer`. Server-Setup von globalem `http.Handle`/`DefaultServeMux` auf
  lokalen `http.NewServeMux()` umgestellt, durch `authMiddleware` gewrappt;
  optionales TLS (`http.ListenAndServeTLS`, wenn `WAYFINDER_TLS_CERT`/`_KEY`
  beide gesetzt sind, sonst Klartext-HTTP wie bisher). Health-/Readiness-Probes
  (`:8080`) bleiben bewusst unauthentifiziert (separater Mux). Neue Tests:
  `pkg/ws/handler_test.go` (`TestCheckOrigin*`, 6 Fälle: ohne Origin,
  Same-Origin, Cross-Origin ohne/mit Allowlist, ungültiger Origin-Header) und
  `cmd/wayfinder/main_test.go` (`TestAuthMiddleware*` — deaktiviert/fehlender
  Token/falscher Token/Query-Param/Bearer-Header; `TestLoadConfig*SecurityEnvVars*`
  — Parsing und Default-Leerwerte). `docs/requirements/README.md` (NFR-SEC-001):
  Implementierung/Tests für den Browser-Rand jetzt eingetragen. Alle Gates
  grün (`go build`/`go vet`/`go test ./...`; `gofmt` clean außer dem
  vorbestehenden, unveränderten Befund in `pkg/receiver/receiver_test.go`).
  Damit ist **Paket #1 inhaltlich abgeschlossen** (1.4 — optionale
  Sender-Härtung in Firefly — bleibt als unabhängiges Nice-to-have offen).
  Nächster Schritt: mit dem Projektverantwortlichen das nächste Paket
  abstimmen (Vorschlag: Paket #2 „Observability-Grundgerüst", **S3 · Sonnet
  4.6**) oder optional 1.4 angehen.
- **Vorherige Aktualisierung:** 2026-06-15 — Paket #1 „Multicast-Feed-Sicherheit",
  Häppchen 1.2: **ADR 0003 „Sicherheit: Vertrauensgrenze des Empfangspfads und
  Browser-Rand"** erstellt (`docs/decisions/0003-sicherheit-empfangspfad-und-browser-rand.md`).
  Zwei Entscheidungen: (1) **Empfangspfad** spiegelt Fireflys ADR 0017 — Netz-
  Isolation auf der Netzwerk-Schicht, kein App-Krypto auf CAT062, robuster
  Decoder bleibt App-Schutzschicht (keine Code-Änderung). (2) **Browser-Rand**
  (`/`, `/ws`, `/api/map-config` auf `:8081`, heute ohne TLS/Auth, `CheckOrigin
  → true`): TLS+Auth primär am Reverse-Proxy/Ingress (OIDC/mTLS, cloud-native,
  kein Krypto-Eigenbau im ASD); ergänzend fail-closed in Wayfinder — strikter
  Origin-Check (`WAYFINDER_ALLOWED_ORIGINS`), optionale Token-Middleware
  (`WAYFINDER_AUTH_TOKEN`, Default aus + Warn-Log), optionales TLS
  (`WAYFINDER_TLS_CERT`/`_KEY`); Health-/Readiness-Probes (`:8080`) bleiben
  unauthentifiziert. Schließt das transformierte ehem. Issue #7. Neue
  Anforderung **NFR-SEC-001** im Register (Empfangspfad: dokumentiert;
  Browser-Rand: Implementierung folgt Häppchen 1.3). Reine Doku, kein
  Code-Diff. Nächster Schritt: Häppchen 1.3 — Implementierung Browser-Rand
  (Origin-Check, Token-Middleware, optionales TLS) in
  `pkg/ws/handler.go`/`cmd/wayfinder/main.go`, **S4 · Opus 4.8**.
- **Vorherige Aktualisierung:** 2026-06-15 (Branch `claude/tse-i062-080`, nach
  `main` gemergt — PR #8 (Wayfinder) / PR #16 (Firefly):
  **T5 — CAT062 Track-Ende (TSE, I062/080) dekodiert + Track-Entfernung, ICD
  2.2.0.** AP8 (Callsign) war bereits zuvor nach `main` gemergt — PR #7.) `decodeTrackStatus`
  liest die FX-Kette jetzt oktett-genau (CNF Oktett 1, **TSE Oktett 2 Bit 7
  `0x40`**, CST Oktett 4) und füllt `TrackStatus.Ended`; robust gegen früher
  endende Records. Durchgereicht via `broadcast.TrackMessage.Ended`
  (`json:"ended,omitempty"`); das Frontend (`updateTracksLayer`) **filtert**
  Ende-Records heraus → Symbol/Label/Vektor/Trail verschwinden sofort (statt
  Timeout). Test: `pkg/cat062/decoder_test.go::TestDecodeTrackEnd` (Referenz
  aus Fireflys `track_status_carries_tse_when_ended`). `CLAUDE.md` §2 und
  `docs/requirements/README.md` (FR-DATA-003) aktualisiert. Gates grün
  (`go build`/`go vet`/`go test ./...`; `gofmt` für geänderte Dateien). **TSE
  (Firefly T1–T4 + Wayfinder T5) damit beidseitig abgeschlossen.**
- **Vorherige Aktualisierung:** 2026-06-15 (Branch `claude/callsign-i062-245`:
  **AP8 — CAT062 Target Identification I062/245 (Callsign) dekodiert, ICD
  2.1.0.**) `pkg/cat062/decoder.go` zieht FRN 10 nach: 7-Byte-Item
  (STI/spare-Oktett + 8 × 6-Bit-IA-5), `decodeTargetIdentification`/
  `ia5Decode` (fremde Codes defensiv → Leerzeichen, robust gegen
  Fehl-Datagramme). `DecodedTrack.Callsign *string` (trailing spaces
  getrimmt), durchgereicht über `broadcast.TrackMessage.Callsign`
  (`json:"callsign,omitempty"`) bis ins Frontend. `app.js::buildLabel` zeigt
  das Callsign jetzt als primäre Label-Zeile (Track-Nummer als Fallback), FL
  weiterhin als zweite Zeile. FRN 10 liegt im bereits vorhandenen 2.
  FSPEC-Oktett → additiv, kein Wire-Format-Bruch. Test:
  `pkg/cat062/decoder_test.go::TestDecodeCallsign` (Referenzwerte aus Fireflys
  `target_identification_packs_eight_six_bit_ia5_codes`). `CLAUDE.md`
  Abschnitt 2 (FRN-Tabelle) und `docs/requirements/README.md` (FR-DATA-002)
  aktualisiert. Alle Gates grün (`go build`/`go vet`/`go test ./...`/`gofmt`
  für die geänderten Dateien; ein vorbestehender `gofmt`-Befund in
  `pkg/receiver/receiver_test.go` ist unverändert und nicht Teil dieser
  Änderung). **AP7 (Firefly-Encoder) und AP8 (dieser Schritt) sind damit
  beide abgeschlossen.**
- **Frühere Aktualisierung:** 2026-06-15 (Branch `claude/callsign-i062-245`:
  Doku-/Docker-Vorbereitung fürs Testen. `README.md` komplett neu (Quickstart
  Docker/lokal, Architektur-Übersicht, Konfig-Tabelle, Build & Test, Links).
  Neu: `Dockerfile` (Multi-Stage `golang:1.23-bookworm` → `debian:bookworm-
  slim`, Healthcheck `/health`), `docker-compose.yml` (`network_mode: host` —
  notwendig für CAT062-Multicast-Empfang), `.dockerignore`, `DOCKER.md`
  (Standalone + End-to-End mit Firefly, inkl. `FIREFLY_CAT062_ENABLED=true` und
  macOS/Windows-Docker-Desktop-Hinweis). Firefly-seitig analoger Abschnitt in
  README/DOCKER.md ergänzt. Docker-Build konnte in dieser Sitzung nicht
  getestet werden (kein Docker-Daemon verfügbar) — `go build`/`go vet`/
  `go test ./...` sind grün.)
- **Frühere Aktualisierung:** 2026-06-14 (Branch `claude/serene-heisenberg-xq4rla`:
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
