# ASD-020 — Layer-Sidebar: aufklappbare Gruppen statt flacher Liste

> **Register:** FR-UI-044 · **Entscheidung:** ADR 0031 · **Auslöser:**
> Betreiber-Wunsch 2026-07-20 („Ich brauche ein Konzept, wie ich die Fülle an
> Layer-Optionen sauber gliedere im Sidepanel") — E1 aus dem BKG-Element-Epic
> #290 (Issue #292), Vorbereitung für die BKG-Element-Schalter (#293).

## Fachlich — warum

Der „Layer"-Bereich der Seitenleiste war eine **lange, flache Reihe** von
Schaltern (rund ein Dutzend). Für den Lotsen ist das mühsam zu überblicken — und
es wird schlimmer, sobald die Basiskarte in einzelne Elemente (nur Flüsse, nur
Straßen …) zerlegbar wird. Damit die **Fülle** bedienbar bleibt, werden die
Schalter jetzt in **vier benannte, aufklappbare Gruppen** einsortiert, jede mit
einem **Sammel-Schalter** oben, der die ganze Gruppe auf einmal ein- oder
ausblendet.

Was der Lotse sieht: statt einer endlosen Liste vier klar benannte Blöcke
(**Aeronautik**, **Karte**, **Radar & Reichweite**, **Wetter**), die man
zuklappen kann, wenn man sie gerade nicht braucht. Der Sammel-Schalter zeigt drei
Zustände — alles an, alles aus, oder „teilweise" (einige an) — und schaltet auf
Klick die ganze Gruppe.

## Technisch — die Gliederung

Spiegel zu ADR 0029 (Overlay-Zonen am Scope-Rand), jetzt fürs Panel:

- **`LayerGroup.vue`** (neu) — der verbindliche **Rahmen**: ein aufklappbarer
  Block (Chevron + Titel + tri-state `v-checkbox-btn`-Master), Slot für die
  Zeilen. Positions-neutral, `defaultExpanded` (E1 startet alle offen — nichts
  wird versteckt, was vorher sichtbar war).
- **`map/layerGroups.js`** (neu) — die schema-agnostische Tri-State-Logik
  (`masterState`, `nextMaster`), ohne Vuetify-Mount unit-testbar.
- **`LayerFilterContent.vue`** — die vier Gruppen; die Gruppen-**Mitgliedschaft**
  steht hier neben den Zeilen (`{on, set, enabled}`-Member).

### Die vier Gruppen

| Gruppe | Layer |
|--------|-------|
| **Aeronautik** | Lufträume (CTR/TMA/Restricted/Info), AoR, VOR/NDB, Waypoints, Flughäfen, Runways |
| **Karte** | Basiskarte (BKG) — künftig + BKG-Element-Ebenen (#293) |
| **Radar & Reichweite** | Radarabdeckung, History-Dots (+Dauer), Range-Rings (+Konfig) |
| **Wetter** | DWD-Regenradar, DWD-Wetterwarnungen (je +Legende) |

Die **Spurherkunft-Legende** (Symbol = Herkunft) ist ein Referenz-Block, kein
Toggle — sie bleibt außerhalb der Gruppen am Fuß. **FL-Filter** und
**Nutzer-Account** sind eigene Sektionen (unverändert).

### Der Master (Sammel-Schalter)

- Zustand aus den **bedienbaren** Mitgliedern: alle an → an, alle aus → aus,
  sonst „teilweise" (indeterminate).
- Klick = **Select-all/none**: aus alles-aus wird alles-an, sonst alles-aus.
- **Deaktivierte** Toggles (Quelle nicht verfügbar — Radarabdeckung ohne
  Radar-Sensor, DWD ohne Quelle) sind aus Zustand **und** Bulk-Aktion
  ausgeschlossen: der Master hängt nie ewig auf „teilweise" und schaltet keinen
  datenlosen Layer ein.
- Die Bulk-Aktion nutzt **denselben** Store-Pfad wie der Zeilen-Switch
  (`onLayerToggle` / `setAirspaceGroup`) — kein toter Toggle; die Karte reagiert
  identisch (MapCanvas-Watcher auf `layerVisibility` / `airspaceGroupVisibility`).
- Eine Gruppe ohne sichtbares Mitglied (alles ausgegated) verschwindet (`v-if`).

## Ehrliche Grenzen

- **Keine visuelle CI-Zusicherung:** kein WebGL-/Mount-Harness; die Struktur ist
  per Source-Guards gezurrt (`layerGroups.test.js`, `layerGrouping.test.js`), die
  optische Abnahme macht der Betreiber (nach `git pull` + Frontend-Rebuild).
- **Bewusst noch offen (spätere E-Stufen):** Accordion (nur eine Gruppe offen)
  für den schmalen Drawer, **Presets** „Minimal/Standard/Detailliert" (#294) und
  **Persistenz im View-Profil** (#295). Dieser Rahmen trägt sie.

## Tests

- `frontend/src/map/__tests__/layerGroups.test.js` (neu): `masterState`
  (empty/on/off/mixed) und `nextMaster` (Select-all/none).
- `frontend/src/components/__tests__/layerGrouping.test.js` (neu): vier Gruppen
  je mit Master, `LayerGroup`-Rahmen (Chevron/indeterminate/toggle-master),
  Basiskarte in der Karte-Gruppe (Fundament für #293), Master-Bulk über den
  Store-Pfad, Ausschluss deaktivierter Toggles.
- Bestand grün gehalten: `layerSidebarCleanup.test.js` (Airspace-Gruppen weiter
  an `setAirspaceGroup` verdrahtet, Sektions-Header unverändert).
