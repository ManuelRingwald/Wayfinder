# ADR 0001 — Technologie-Stack: Go, MapLibre GL, WebSocket-Backend-zu-Browser

- **Status:** accepted
- **Datum:** 2026-06-13

## Kontext

Wayfinder (Air Situation Display) ist Greenfield und wird produktiv gebaut. Vor
dem ersten Code müssen die Werkzeuge feststehen — Programmiersprache, Karten-
Frontend, und die Art, wie das Backend (CAT062-Decoder) die Tracks zum Browser
schickt.

## Entscheidung

Wir bauen Wayfinder mit folgendem Stack:

1. **Backend-Sprache: Go**
   - Netz-nativ: UDP/Multicast, Nebenläufigkeit out-of-the-box (goroutines)
   - Statische Binaries → einfaches Container-Deployment
   - Starke Typsicherheit, moderne Ergonomie
   - Cloud-native Standard (Kubernetes)

2. **Karten-Frontend: MapLibre GL JS**
   - Anbieter-neutral, Open-Source, WebGL-basiert
   - Konsistent mit Fireflys bestehender Frontend-Linie (Firefly ADR 0009)
   - Vektorkacheln-Support (proprietär oder OSM)
   - JavaScript/TypeScript in modernem Ökosystem

3. **Transport Backend↔Browser: WebSocket (Server-Push)**
   - Asynchrone Tracks-Lieferung (decodiert aus CAT062-Multicast)
   - Getrennt vom CAT062-Eingangs-Pfad (saubere Architektur)
   - Später: HTTP/2 Server-Push, gRPC, oder ähnliches → Adapter-Pattern

## Begründung

- **Go:** ADR 0003-Prinzipien (Cloud-nativ) + bewährte Radar/Echtzeit-Systeme
  (Firefly selbst ist Rust, aber für ASD-Konsumenten ist Go etabliert).
- **MapLibre:** Keine vendor lock-in, bereits bekannt (Firefly), universell
  einsetzbar, Web-Standard.
- **WebSocket:** Simpel, bewährt, keine zusätzliche Infrastruktur nötig;
  Trennung von CAT062-Eingang und Browser-Ausgang erlaubt parallele Verarbeitung.

## Konsequenzen

- Go-Projekt-Struktur mit `go mod`, `go test`, `golangci-lint` (per CLAUDE.md)
- Frontend: HTML + TypeScript/Vite; ggf. Vue/React (später entscheiden)
- CAT062-Decoder wird als Go-Library gebaut, **nicht** importiert von Firefly
  (Kopplung läuft über den CAT062-Draht-Vertrag / ICD, siehe Firefly ADR 0014)
- Zertifizierungs-fähiges Logging, Health-/Readiness-Probes, 12-Factor-Config
  (Firefly ADR 0003-Prinzipien adaptiert für Go-Kontext)
- Robuster, fuzzing-getesteter CAT062-Decoder (niemals auf fehlerhafte
  Datagramme panicken, siehe CLAUDE.md §7)

## Ehrliche Grenze

Dies ist die **Stack-Wahl**. Nicht entschieden:
- Genaue Frontend-UI-Bibliotheken (Vue/React/Svelte)
- Details des Browser-Auth (falls Wayfinder-Zugriff geschützt werden soll)
- Deployment-Strategie (Docker, Kubernetes, bare metal)
- Konfiguration von Kartenstil, Zoom, Layers

Diese folgen in eigenen, später abgestimmten Häppchen (ADR 0002–ADR 0005, ggf.
Abschnitte zu Sicherheit, Observability, Frontend-UI).
