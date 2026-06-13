# Anforderungs-Register

Hier werden alle Anforderungen (funktional und nicht-funktional) dokumentiert und rückverfolgt zu:
- Design-Entscheidungen (ADRs)
- Code-Implementierung
- Test-Verifikation

**Struktur:**

- Jede Anforderung bekommt eine eindeutige ID (REQ-NNNN).
- Format: `REQ_NNNN.md` oder `requirements.md` mit Tabelle.

**Beispiel (kommend):**

| ID | Anforderung | Quelle | Design | Implementierung | Tests |
|----|-------------|--------|--------|-----------------|-------|
| REQ-0001 | WebSocket-Client empfängt Firefly-Tracks | M1 Spezifikation | ADR0001 | `pkg/firefly/client.go` | `client_test.go` |
| REQ-0002 | Track-Datenmodell (Position, Velocity, ID) | M1 Spezifikation | ADR0001 | `pkg/track/model.go` | `model_test.go` |

Das Register wird bei jedem Meilenstein ergänzt.
