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

Das Register wird bei jedem Meilenstein ergänzt.
