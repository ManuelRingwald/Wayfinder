# Anforderungs-Register

Hier werden alle Anforderungen (funktional und nicht-funktional) dokumentiert und rückverfolgt zu:
- Design-Entscheidungen (ADRs)
- Code-Implementierung
- Test-Verifikation

| ID | Anforderung | Quelle | Design | Implementierung | Tests |
|----|-------------|--------|--------|-----------------|-------|
| FR-OPS-001 | Liveness-Probe (`/healthz`) signalisiert, dass der Prozess läuft | CLAUDE.md Abschnitt 8 (Cloud-nativ) | ADR0001 | `internal/server/server.go` | `internal/server/server_test.go` |
| FR-OPS-002 | Readiness-Probe (`/readyz`) signalisiert Betriebsbereitschaft (später: Firefly-Verbindung) | CLAUDE.md Abschnitt 8 (Cloud-nativ) | ADR0001 | `internal/server/server.go` | `internal/server/server_test.go` |
| FR-CFG-001 | Konfiguration (Port, Log-Format) über Env-Vars mit dokumentierten Defaults | CLAUDE.md Abschnitt 8 (12-Factor) | ADR0001 | `internal/config/config.go` | `internal/config/config_test.go` |
| FR-CFG-002 | Ungültige Konfigurationswerte fallen auf Defaults zurück statt abzustürzen | CLAUDE.md Abschnitt 8 (Cloud-nativ) | ADR0001 | `internal/config/config.go` | `internal/config/config_test.go` |
| FR-OPS-003 | Sauberes Herunterfahren (Graceful Shutdown) auf SIGINT/SIGTERM | CLAUDE.md Abschnitt 8 (Cloud-nativ) | ADR0001 | `cmd/wayfinder/main.go` | manuell verifiziert (M1 Schritt 1) |
| FR-DATA-001 | Track-Datenmodell (`Frame`/`FrameTrack`/`FramePlot`) bildet Firefly's `/ws`-Wireformat verlustfrei ab (Feldnamen, Typen, leere Arrays) | M1 Spezifikation, Firefly `crates/firefly-io/src/frame.rs` | ADR0001 | `internal/firefly/frame.go` | `internal/firefly/frame_test.go` |
| FR-DATA-002 | CAT062-Decoder dekodiert I062/245 (Target Identification / Callsign, FRN 10, ICD 2.1.0) — 8 Zeichen als 6-Bit-IA-5-Codes, MSB-first, fremde/ungültige Codes defensiv auf Leerzeichen abgebildet (robuster Decoder, CLAUDE.md Abschnitt 7). Der `Broadcaster` reicht das Callsign als `callsign` ans Frontend durch; `buildLabel` zeigt es als primäre Label-Zeile (Track-Nummer als Fallback). | Fireflys ICD-CAT062.md v2.1.0 (AP7), CLAUDE.md Abschnitt 2 | — | `pkg/cat062/decoder.go` (`decodeTargetIdentification`, `ia5Decode`), `pkg/broadcast/broadcast.go`, `internal/webui/static/app.js` (`buildLabel`) | `pkg/cat062/decoder_test.go::TestDecodeCallsign` |

Das Register wird bei jedem Meilenstein ergänzt.
