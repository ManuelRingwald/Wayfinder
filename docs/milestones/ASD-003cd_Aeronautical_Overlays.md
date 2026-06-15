# ASD-003c/d — Aeronautik-Overlays im Frontend

Teil von **ASD-003 (Aeronautical Map Layer)**, Häppchen 3c (Lufträume +
Layer-Steuerung) und 3d (Waypoints + VOR/NDB). Datenquelle: das OpenAIP-Backend
aus 3b (ADR 0004). Siehe `docs/ROADMAP.md` Paket #13.

## Fachlich

Der Lotse braucht zur räumlichen Orientierung die Luftraumstruktur
(Sektor-/FIR-Grenzen) und die Navigationspunkte (VOR/NDB-Beacons, Waypoints)
auf dem Scope — und die Möglichkeit, einzelne Ebenen **auszublenden**, wenn das
Bild zu voll wird (Decluttering).

## Technisch (`internal/webui/static/`)

### 3c — Lufträume + Layer-Steuerung
- **`addAirspaceLayers`**: GeoJSON-Source `airspace` + drei Layer — `fill`
  (Polygone, sehr schwache Füllung 0.06, damit überlappende Sektoren lesbar
  bleiben), `line` (Sektor-/FIR-Grenzen) und `symbol`/`line`-platziertes
  Label (`name`, ab Zoom 6). Farben theme-abhängig (`palette.airspaceLine/
  airspaceText`).
- **Layer-Steuerung**: Panel `#layer-control` (`index.html`) mit drei
  Checkboxen (Lufträume / VOR/NDB / Waypoints). `setupLayerControl` schaltet die
  zugehörigen Layer über `setLayoutProperty(..., "visibility", …)`.

### 3d — Waypoints + VOR/NDB
- **Icons ohne externe Sprites**: `makeIconImage`/`addAeronauticalIcons` malen
  die Marker auf ein Offscreen-Canvas und registrieren sie via `map.addImage`
  (Wayfinder bleibt ein selbst-enthaltenes Binary):
  - `wf-waypoint` — Dreieck (cyan),
  - `wf-vor` — Kompassrose (Ring + 8 Ticks, grün) für die VOR-Familie,
  - `wf-ndb` — gestrichelter Ring mit Punkt (amber),
  - `wf-navaid` — generischer Ring (grau) als Fallback.
- **`addNavaidLayers`**: Symbol-Layer wählt das Icon per `match` über
  `navaid_kind` (VOR-Familie → `wf-vor`, `NDB` → `wf-ndb`, sonst `wf-navaid`),
  Label aus `ident`/`name`, ab Zoom 6.
- **`addWaypointLayers`**: Symbol-Layer mit `wf-waypoint`, Label `name`, ab
  Zoom 7 (Waypoints sind dichter → höherer Zoom-Boden gegen Clutter,
  `icon-allow-overlap: false`).

### Daten-Laden
- **`loadAeronautical`** holt `/api/airspace`, `/api/navaids`, `/api/waypoints`
  und schiebt sie in die Sources. Beim Laden + alle `AERO_REFRESH_MS` (5 min).
  Fehler sind **nicht-fatal** (Overlay bleibt unverändert) — graceful
  degradation, ADR 0004.
- **Layer-Reihenfolge**: Aeronautik-Layer werden **vor** Trails/Vektoren/Tracks
  registriert → liegen darunter, die Tracks dominieren das Bild.

## Architektur-Hinweis

Bleibt bei Vanilla-JS (Frontend-Framework per ADR 0002 nicht ratifiziert). Die
Icons werden zur Laufzeit gezeichnet, es kommen keine Binär-Assets ins Repo.

## Tests / Verifikation

- `node --check internal/webui/static/app.js` (Syntax).
- Manueller Rauchtest des laufenden Servers: `/api/map-config` liefert das
  Dark-Theme; `/api/airspace|navaids|waypoints` liefern ohne API-Key leere
  FeatureCollections (graceful), das ASD startet sauber; `/metrics` zeigt die
  `wayfinder_openaip_*`-Kennzahlen.
- Visuelle Verifikation (Karte/Overlays/Toggle) — wie bei M1.4 — manuell, da
  kein JS-Test-Harness existiert.

## Damit ist ASD-003 abgeschlossen

3a (Dark Mode) + 3b (OpenAIP-Backend) + 3c/3d (Overlays) erfüllen die
Akzeptanzkriterien: dunkles Basis-Theme, Luftraumstrukturen, Waypoints und
VOR/NDB als dedizierte, schaltbare Layer.
