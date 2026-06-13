# Wayfinder – Aktueller Stand

**Sitzung vom:** 2026-06-13  
**Branch:** `claude/firefly-asd-implementation-8qbcsq`  
**Meilenstein:** M1 – WebSocket-Client + Datenmodell

---

## Status

- ✅ Projekt-Charter (`CLAUDE.md`) angelegt
- ✅ Doku-Grundstruktur (`docs/`, Glossar, Decisions, Requirements, Milestones)
- ⏳ **M1 – WebSocket-Client + Datenmodell** – nächster konkreter Baustein

---

## Nächste Schritte

1. **ADR 0001 – Tech-Stack & Firefly-Integration:** Entscheidung dokumentieren (Go + WebSocket-Client, Datenmodell für Firefly Tracks).
2. **Glossar füllen:** Kern-Begriffe erklären (ASD, Track, WebSocket-Feed, etc.).
3. **Go-Projekt initialisieren:** `go mod init`, Ordnerstruktur für M1.
4. **WebSocket-Client bauen:** Firefly empfangen und parsen.
5. **Datenmodell:** Go-Structs für Tracks definieren.

---

## Offene Fragen

- Firefly-WebSocket-Schnittstelle: Welches Format (JSON? ASTERIX über WebSocket)? Von wo die API-Spezifikation?
- Kartendarstellung: Canvas/WebGL oder Web-Framework (z.B. leaflet.js für Basismap)?
