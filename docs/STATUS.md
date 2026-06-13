# Wayfinder – Aktueller Stand

**Sitzung vom:** 2026-06-13  
**Branch:** `claude/firefly-asd-implementation-8qbcsq`  
**Meilenstein:** M1 – WebSocket-Client + Datenmodell

---

## Status

- ✅ Projekt-Charter (`CLAUDE.md`) angelegt
- ✅ Doku-Grundstruktur (`docs/`, Glossar, Decisions, Requirements, Milestones)
- ✅ ADR0001 – Tech-Stack (Go) & Firefly-Integration (WebSocket via `coder/websocket`)
- ✅ Firefly-Schnittstelle recherchiert (`Frame`/`FrameTrack`/`FramePlot`-Wireformat),
  Produktionslücken in `docs/cross-project/todo-for-firefly.md` notiert
- ✅ **M1, Schritt 1:** Go-Projekt-Grundgerüst (`go.mod`, `cmd/wayfinder`,
  `/healthz`, `/readyz`, Config über Env-Vars, Graceful Shutdown, slog-Logging)
- ⏳ **M1, Schritt 2:** Track-Datenmodell (Go-Structs passend zu Firefly's `Frame`) – nächster Baustein

---

## Nächste Schritte

1. **M1, Schritt 2 – Datenmodell:** Go-Structs `Frame`, `FrameTrack`, `FramePlot`
   mit JSON-Tags passend zu Firefly's Wire-Format (siehe ADR0001).
2. **M1, Schritt 3 – WebSocket-Client:** Verbindung zu Firefly (`/ws`),
   Reconnect-Handling, Unterscheidung Frame vs. `delay_triggered`-Event,
   `/readyz` an Verbindungsstatus koppeln.
3. **M1, Schritt 4:** Konfiguration um `FIREFLY_ADDR` erweitern.

---

## Offene Fragen

- Kartendarstellung (M2): Canvas/WebGL oder Web-Framework (z.B. leaflet.js/MapLibre für Basismap)?
