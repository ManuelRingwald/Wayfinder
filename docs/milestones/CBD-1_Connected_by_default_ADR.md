# CBD-1 — Connected-by-default (ADR 0017 + Rahmen)

> Häppchen 1 der „Connected-by-default"-Umstellung. Reine Entscheidungs-/Doku-Basis;
> die per-Feature-Default-Flips folgen in Häppchen 2 (DWD), 3 (QNH) und AERO
> (OpenAIP).

## Fachlich

Das Zielsegment wird geschärft: Wayfinder ist ein System zur
**Informationsbereitstellung/Lagedarstellung**, **nicht** zur Steuerung von
Flugbewegungen, und wird für den **vernetzten** Betrieb gebaut. Externe
Kontext-Quellen (Karte, Wetter, Aeronautik) sind damit ein selbstverständlicher
Produkt-Bestandteil statt eines risikobehafteten Opt-in. Das behebt die
Betreiber-Reibung („Overlay freigeschaltet, aber Quelle nicht konfiguriert") und
eine bestehende Inkonsistenz (die Basiskarte lud ohnehin schon externe Kacheln).

## Technisch

Reine Entscheidung/Doku — **kein Code-Verhalten geändert**:

- **ADR 0017** (`docs/decisions/0017-connected-by-default.md`): Prämisse; amendet
  ADR 0004 (OpenAIP) + ADR 0016 (Wetter) — Opt-in-„Offline"-Begründung entfällt,
  Quellen werden **default-an mit explizitem `..._ENABLED`-Opt-out**; grenzt
  **NFR-SEC-001** ausdrücklich ab (CAT062-Multicast bleibt netz-isoliert); Fonts/
  Glyphen bleiben self-hosted (Begründung → Robustheit statt Air-Gap).
- **Nachtrag-Verweise** in ADR 0004 und 0016 (Rückverfolgbarkeit).
- **Netzwerk-Anforderungen** als First-Class-Doku: `INSTALLATION.md §8.0`
  (Egress-Ziele + Rollout-Hinweis) und `BETRIEB.md §6.4` (Abgrenzung Feed-Isolation
  vs. ausgehende Kontext-Quellen).
- **Anforderungs-Register:** `NFR-OPS-005` (Connected-by-default-Posture).

## Schnittstellen-Wirkung

**Keine.** Kein CAT062/CAT065/CAT063-Eingriff, keine Firefly-Koordination. Kein
Code-Verhalten geändert; `go build` bleibt grün.

## Gates

- Docs/ADR; `go build`/`go test ./...` unverändert grün (kein Code berührt).
- vitest/`vite build` unberührt (kein Frontend-Code berührt); `dist` unverändert.

## Ehrliche Grenze

Wayfinder bleibt ein System zur Informationsbereitstellung, **kein zertifiziertes
Steuerungssystem**; die externen Kontext-Daten sind Orientierungs-Information, keine
zertifizierten aeronautischen/meteorologischen Datensätze. Die per-Feature-
Default-Umstellung (default-an + `..._ENABLED`) wird erst mit den Folge-Häppchen
umgesetzt; bis dahin gelten die bisherigen Opt-in-Schalter.
