# WX-0 — Wetter-Fundament (ADR + Vertrauensgrenze)

> Häppchen 0 der Wetter-Erweiterung (DWD-Radar-Overlay, QNH-Infobox,
> DWD-Warnungen). Reine Entscheidungs-/Doku-Basis; der Feature-Code folgt in
> WX-A / WX-B / WX-C.

## Fachlich

Der Betreiber möchte Wetter-Kontext im ASD:

- **Wetter-Overlay** (DWD-Radar/Niederschlag) unter der Luftlage,
- **QNH-Infobox** (Höhenmesser-Einstellung, hPa) im Kopfbereich,
- **Wetterwarnungen-Overlay** (amtliche DWD-Warnpolygone).

Diese Daten kommen aus **externen, öffentlichen HTTP-Quellen** (DWD, NOAA) und
sind Wayfinders erste allgemeine ausgehende Internet-Abhängigkeit außerhalb von
OpenAIP. Eine solche neue **Vertrauensgrenze** wird bewusst als
Architektur-Entscheidung festgehalten.

## Technisch

- **ADR 0016** (`docs/decisions/0016-ausgehende-wetter-feeds-dwd-noaa.md`):
  Backend-Proxy + Cache (ein Egress-Punkt), best-effort/graceful degradation
  (nie `/ready` blockieren), robuster misstrauischer Decoder
  (Timeout/`io.LimitReader`/tolerant/kein Panic/Fuzzing), Feature still aus ohne
  URL + per-Tenant-Entitlement, QNH-Datenwahrheit (nur METAR, nie PMSL),
  Lizenz/Attribution (DWD GeoNutzV/CC BY 4.0, NOAA Public Domain).
- **Anforderungs-Register:** `NFR-SEC-005` (Wetter-Vertrauensgrenze) neu; die
  feature-spezifischen `FR-WX-001/002/003` folgen in WX-A/B/C.

## Schnittstellen-Wirkung

**Keine.** Kein CAT062/CAT065/CAT063-Eingriff, keine Firefly-Koordination — rein
Wayfinder-intern. Der Multicast-Sicherheitspfad bleibt unberührt.

## Gates

- Docs/ADR; `go build ./...` unverändert grün (kein Code-Verhalten geändert).
- Die Wetter-Decoder-Referenz-Vektor- und Fuzz-Tests entstehen mit dem jeweiligen
  Feature-Code (WX-A/B/C, Charter §6).

## Ehrliche Grenze

Die konkreten DWD-/NOAA-Endpunkte konnten in der Entwurfs-Umgebung nicht live
verifiziert werden (Egress-Policy). Der Feature-Code wird defensiv gebaut
(falscher Layer/Endpoint ⇒ leeres Overlay, kein Absturz); ein Live-Smoke-Test aus
offenem Netz bleibt ein Deploy-Schritt.
