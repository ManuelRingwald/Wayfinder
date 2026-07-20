# ASD-021 (E0) — BKG-Basiskarte: Element-Bucketing der Style-Ebenen

> **Register:** FR-UI-045 · **Bezug:** ADR 0031 (Sidebar-IA) · Epic #290,
> Issue #291 (E0) · **Auslöser:** Grundlage, damit die Basiskarte später in
> einzelne Elemente (nur Flüsse, nur Straßen …) schaltbar wird (E2/#293).

## Fachlich — warum

Der Lotse soll künftig **einzelne Karten-Elemente** ein-/ausblenden können —
„nur Flüsse", „nur Straßen". Bevor die dafür nötigen Schalter gebaut werden
(E2/#293), braucht es die **Datenbasis**: Jede Ebene der BKG-Vektorkarte muss
ihrer **Element-Gruppe** zugeordnet sein (Gewässer, Verkehr, Vegetation,
Siedlung, Gebäude, Grenzen, Beschriftung, Hintergrund). Diese Stufe liefert
genau das — **noch ohne Bedienoberfläche**.

## Technisch — schema-agnostisch und driftfest

Die BKG-Karte ist ein MapLibre-**Vektor**-Style: jeder Objekttyp ist eine eigene
`source-layer`, und der Style listet die Ebenen in `layers[]`. Die **exakten
Namen driften** mit BKG-Updates, und der basemap.world-Stil nutzt andere
(OSM-abgeleitete) Namen. Deshalb wird **nicht** gegen eine feste Namensliste
gemappt, sondern per **Muster** über `source-layer` + `id` + `type`.

- **`frontend/src/map/basemapGroups.js`** (neu, rein, unit-getestet):
  - `BASEMAP_GROUPS` — die acht Element-Gruppen + `other` (Catch-all).
  - `classifyBasemapLayer(layer)` — priorisierte Regelliste, **erste** passende
    Regel gewinnt (spezifisch vor generisch: Symbol→Beschriftung zuerst, dann
    Hintergrund/Grenzen/Gebäude vor den generischen Flächen). Nicht erkannte
    Ebenen → `other`, damit **nie** eine Ebene verloren geht. Der Haystack
    normalisiert Trennzeichen (`_`, `-`, `.`) zu Leerzeichen, damit Wortgrenzen
    (`\bwood`) auch bei `landcover_wood` greifen.
  - `bucketBasemapLayers(styleLayers, excludeId)` → `{ gruppe: id[] }` mit
    stabiler Form (leere Gruppen bleiben); der synthetische Scope-Grund ist
    ausgeschlossen. Die Vereinigung aller Buckets = `basemapLayerIds` (die
    Menge, die der #274-Master weiter als Ganzes schaltet).
- **`frontend/src/map/engine.js`**: beim Style-`load` wird derselbe Ebenensatz
  zusätzlich gebucketet (`state.basemapGroups`), und die Engine exponiert
  **`setBasemapGroupVisibility(group, visible)`** — den Schalthebel, den E2 an
  die künftigen Element-Switches hängt. **Noch ruft ihn keine UI auf.**

### Die acht Element-Gruppen (Muster-Beispiele)

| Gruppe | basemap.de-Stamm | basemap.world-Stamm |
|--------|------------------|---------------------|
| `water` | Gewaesserflaeche/-linie | water, waterway, river |
| `traffic` | Verkehrsflaeche/-linie, Bahn | transportation, road, rail |
| `vegetation` | Vegetationsflaeche, Wald | park, forest, landcover(green) |
| `settlement` | Siedlungsflaeche, Nutzung | landuse, residential |
| `building` | Gebaeude | building |
| `boundary` | Verwaltungseinheit/-grenze | boundary, admin |
| `label` | Beschriftung (Symbol) | place, water_name (Symbol) |
| `background` | Hintergrund | background, relief |

Ein **Symbol-Layer ist immer Beschriftung** (`label`), egal über welchem Thema —
diese Regel steht bewusst an erster Stelle, sonst würde z. B. `water_name` als
Gewässer-Geometrie fehlgezählt.

## Ehrliche Grenzen

- **Kein Live-Style von hier prüfbar:** Der Agent-Proxy sperrt den BKG-Host
  (403); die Zuordnung ist gegen einen **Fixture** aus realistischen
  basemap.de- **und** basemap.world-Namen getestet. Die endgültige Feinjustierung
  der Muster erfolgt am echten Style (`/basemap/style.json`) — die `other`-Gruppe
  fängt bis dahin alles Unerkannte auf, ohne dass etwas verschwindet.
- **Keine UI:** `setBasemapGroupVisibility` ist die Fähigkeit; die Element-
  Schalter im „Karte"-Panel-Abschnitt kommen mit E2/#293.

## Tests

- `frontend/src/map/__tests__/basemapGroups.test.js` (neu): Klassifikation
  beider Namenswelten, Kanten-Fälle (chaussee ≠ Gewässer, Gebäude vor Siedlung,
  Wald-Landcover = Vegetation, Unbekanntes → `other`, null-sicher),
  `bucketBasemapLayers` (stabile Form, Partition ohne Verlust, Scope-Grund
  ausgeschlossen, Flüsse gruppiert, leere Eingabe).
- `frontend/src/components/__tests__/basemapLayer.test.js` (E0-Block ergänzt):
  Engine bucketet beim Load + exponiert `setBasemapGroupVisibility`.
