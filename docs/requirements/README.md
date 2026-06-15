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
| FR-DATA-003 | CAT062-Decoder dekodiert das TSE-Bit (Track Service End, I062/080 Oktett 2 Bit 7, ICD 2.2.0): `decodeTrackStatus` liest die FX-Kette oktett-genau (CNF Oktett 1, TSE Oktett 2, CST Oktett 4) und füllt `TrackStatus.Ended`. Der `Broadcaster` reicht es als `ended` (omitempty) durch; das Frontend (`updateTracksLayer`) **filtert** Tracks mit `ended` heraus → Symbol, Label, Vektor und Trail verschwinden sofort, statt auf einen Timeout zu warten. Robuster Decoder: kürzer endende Records werden längen-geschützt gelesen (CLAUDE.md Abschnitt 7). | Fireflys ICD-CAT062.md v2.2.0 / ADR 0016, CLAUDE.md Abschnitt 2 | — | `pkg/cat062/decoder.go` (`decodeTrackStatus`), `pkg/cat062/types.go` (`TrackStatus.Ended`), `pkg/broadcast/broadcast.go`, `internal/webui/static/app.js` (`updateTracksLayer`) | `pkg/cat062/decoder_test.go::TestDecodeTrackEnd` |

Das Register wird bei jedem Meilenstein ergänzt.
