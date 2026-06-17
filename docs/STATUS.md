# Arbeitsstand (Handover-Notiz)

> **Zweck:** Diese Datei ist der schnelle Wiedereinstieg — egal ob am PC oder
> Handy. Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

> 🗺️ **Roadmap:** Arbeitspakete, Findings und empfohlene Reihenfolge stehen in
> `docs/ROADMAP.md` (Stichwort „Roadmap" im Chat zeigt diese Liste).

- **Zuletzt aktualisiert:** 2026-06-17 — **Phase 1 der ASD-Optik-Verbesserung
  (ASD-007–010) abgeschlossen.** Branch `claude/vue-md3-asd-006`.

  **ASD-007 Farbschema:** Cyan-Primary-Theme aus ASD-Mockup (Command-Center-
  Ästhetik). `vuetify.js`: background `#070b12`, surface `#0e1622`, primary
  `#23d3e6`. `constants.js`: neues `TRACK_COLORS`-Objekt (friendlyCivil
  `#41c4e8`, hostile `#ff4338`, unknown `#ffd23e`, neutral `#43c66b`, friendlyMilitary
  `#ffa726`); PALETTES.dark aktualisiert (label, vector, trail, airspaceFillColor,
  airways). Design-Spec in `docs/design/color-tokens.md`.

  **ASD-008 Navigation Rail:** `NavigationRail.vue` ersetzt die monolithische
  `LayerSidebar.vue`. Permanent-schmale Schiene (56 px Icons + Tooltips) auf
  Desktop; Klick → 240-px-Panel für Layer-/FL-Filter-Controls; Collapse-Button;
  Mobile bleibt Hamburger-Temporary-Drawer. sections-Array vorbereitet für
  ASD-013 Alarm-Panel.

  **ASD-009 Karten-Controls:** `MapControls.vue` — zwei schwebende Button-
  Gruppen rechts (Zoom +/−; Recenter, Nord-up, Fullscreen). `engine.js` um
  `zoomIn/zoomOut/recenter/resetNorth` erweitert.

  **ASD-010 Kategorie-Filter-Chips:** `TrackFilterChips.vue` top-center über
  dem Canvas. Live-Zähler (Confirmed/Coasting/Tentative) aus Pinia
  `trackCounts`. Klick togglet `hiddenCategories`; `render.js` filtert alle
  Feature-Typen (Symbole, Vektoren, Dots, Trails) für ausgeblendete Kategorien.

  Gates: `npm run build` ✅ · `vitest 39/39` ✅ · `go test ./...` ✅.
  S2–S3 · Sonnet 4.6.

  **Nächster Schritt:** Phase 2 beginnen — Reihenfolge ASD-011 → ASD-012 →
  ASD-013. ASD-011 (Erweitertes Track-Detail-Panel) ist S2, gut umsetzbar mit
  Sonnet 4.6. Oder: PR #16 erst mergen lassen und dann auf neuem Branch weiter.

- **Vorherige Aktualisierung:** 2026-06-17 — **ASD-006 „Vue 3 + Vuetify 3
  (Material Design 3)" abgeschlossen.** Branch `claude/vue-md3-asd-006`.
  ADR 0002 ratifiziert. AP0–AP6 vollständig umgesetzt (ADR-Doku, Scaffold,
  Karten-Engine als ES-Module, 39 Vitest-Tests, Pinia-Store, App-Shell,
  Track-Detail-Panel). wayfinder.yaml.example + FR-CFG-003 (YAML-Config).
  Gates: npm run build ✅ · vitest 39/39 ✅ · go test ./... ✅.

- **Vorherige Aktualisierungen:** (vor ASD-006) Pakete 1–3, 10–16, ASD-001–005.

---

## 1. Wo wir gerade stehen

**ASD-006 (Vue 3 + Vuetify 3 MD3): ✅ Abgeschlossen** (PR #16, offen)
**ASD-007 Farbschema: ✅ Abgeschlossen**
**ASD-008 Navigation Rail: ✅ Abgeschlossen**
**ASD-009 Karten-Controls: ✅ Abgeschlossen**
**ASD-010 Kategorie-Filter-Chips: ✅ Abgeschlossen**

Ausstehend (Phase 2):

| AP | Inhalt | Stufe |
|----|--------|-------|
| **ASD-011** | Erweitertes Track-Detail-Panel (Ausbau TrackDetailCard.vue) | S2 · Sonnet 4.6 |
| **ASD-012** | Range-Rings + Scale-Bar + Nord-/Track-up | S3 · Opus 4.8 |
| **ASD-013** | Alarm-/Ereignis-Panel | S3 · Sonnet 4.6 |

## 2. Gesetzte Entscheidungen (Fundament)

| Thema | Entscheidung | Quelle | Status |
|-------|--------------|--------|--------|
| Betriebsmodus | Produktionsbetrieb (nicht Lernprojekt) | Fireflys ADR 0014 | ✅ |
| Schnittstelle | **CAT062 over UDP-Multicast** | Fireflys ADR 0006 + 0014, `CLAUDE.md` §2 | ✅ |
| Stack | Go + MapLibre GL JS + WebSocket-Server-Push | ADR 0001 | ✅ |
| Frontend-Framework | Vue 3 + Vuetify 3 (MD3), Vite, Vitest, Pinia | ADR 0002 | ✅ |
| Farbschema | Cyan-Primary aus ASD-Mockup | `docs/design/color-tokens.md` | ✅ |

## 3. Nächster Schritt

➡️ **ASD-011: Erweitertes Track-Detail-Panel** (S2 · Sonnet 4.6)

Ausbau `TrackDetailCard.vue`: vollständiger ASD-Data-Block (Callsign, ICAO-
Adresse, Squawk, FL mit Tendenz-Pfeil, Ground Speed, Heading, Track-Nummer,
Zeitstempel letztes Update, Status). Rein Frontend, kein Backend-Change nötig.

Dann ASD-012 (Range-Rings, S3 · Opus 4.8) und ASD-013 (Alarm-Panel, S3 · Sonnet 4.6).

## 4. Schnell-Einstieg

```bash
cd /home/user/Wayfinder
git log --oneline | head -10
npm run build          # in frontend/
npm run test -- --run  # in frontend/
go test ./...
```
