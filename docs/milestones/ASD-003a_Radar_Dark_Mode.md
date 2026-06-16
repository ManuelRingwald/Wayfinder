# ASD-003a — Radar Dark Mode (Basis-Theme)

Teil von **ASD-003 (Aeronautical Map Layer)**, Häppchen 3a. Siehe
`docs/ROADMAP.md` Paket #13.

## Fachlich

Der Lotse arbeitet auf einem **Radar-Scope**, nicht auf einer Touristenkarte.
Die bisherige helle OpenStreetMap-Basis blendet, konkurriert farblich mit den
Tracks und zeigt für die Flugsicherung irrelevante Details. Ein **dunkles,
kontrastarmes Basis-Theme** lässt die Track-Symbole und (ab 3c/3d) die
Aeronautik-Overlays dominieren — so, wie es ein echtes Radar-Lagebild tut.

## Technisch

- **Neues Theme-Konzept** in `cmd/wayfinder/main.go`:
  - `Config.MapTheme` (`WAYFINDER_MAP_THEME`, Default `dark`; akzeptiert
    `dark`\|`osm`, case-insensitive; ungültige Werte fallen auf `dark` zurück —
    FR-CFG-002).
  - `darkMapStyle`: minimaler MapLibre-Style mit **CARTO `dark_nolabels`**
    Raster-Kacheln auf dunklem Hintergrund-Layer. Wie OSM **ohne API-Key**,
    bleibt also selbst-enthalten.
  - `mapConfigHandler` wählt den Style: ein explizit gesetztes
    `WAYFINDER_MAP_STYLE_URL` gewinnt immer; sonst entscheidet das Theme
    (`osm` → OSM-Raster, `dark` → Radar Dark Mode). Das Feld `theme` wird
    zusätzlich im JSON ausgeliefert, damit das Frontend die passende
    Vordergrund-Palette wählen kann.
- **Frontend** (`internal/webui/static/app.js`):
  - `PALETTES` (dark/osm) steuert Label-Farbe/-Halo, Geschwindigkeitsvektor-
    und Trail-Farbe sowie den Symbol-Rand. `main()` wählt die Palette anhand
    von `cfg.theme` (Default dark; unbekanntes Theme → dark).
  - Auf dem dunklen Grund sind Labels hell (`#e8eef5`) mit dunklem Halo; auf
    OSM bleibt die ursprüngliche dunkel-auf-weiß-Palette erhalten.

## Architektur-Hinweis

Das Theme ist über eine Env-Variable konfigurierbar (Cloud-native, CLAUDE.md
§7): ein Deployment kann ohne Code-Änderung zwischen Radar-Dark und der hellen
OSM-Basis wechseln, oder via `WAYFINDER_MAP_STYLE_URL` einen ganz eigenen
(z. B. selbst gehosteten Vektor-)Style einspielen.

## Tests

- `cmd/wayfinder/main_test.go`:
  - `TestMapConfigHandlerDarkThemeByDefault` — Dark-Theme liefert den
    `carto-dark`-Source und `theme: "dark"`.
  - `TestMapConfigHandlerOSMTheme` — OSM-Theme liefert den `osm`-Source und
    `theme: "osm"`.
  - `TestMapConfigHandlerCustomStyleURLReportsTheme` — eine Style-URL gewinnt,
    das konfigurierte Theme wird trotzdem gemeldet.
  - `TestLoadConfigMapTheme` — Default, gültige, case-insensitive und
    ungültige (→ Default) Werte.
- Frontend wird — wie M1.4 — manuell verifiziert (kein JS-Test-Harness).

## Nächste Schritte

- **ASD-003 ADR**: Live-OpenAIP als Aeronautik-Datenquelle (Backend-Proxy +
  Cache, graceful degradation, Key server-seitig).
- **3b**: OpenAIP-Backend (Client + Cache + interne GeoJSON-Endpoints).
- **3c/3d**: Luftraum- bzw. Waypoint-/Navaid-Layer im Frontend.
