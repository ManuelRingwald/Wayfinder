# ASD-016 — Amtliche Basiskarte: BKG basemap.de Web Vektor (H1 `bkg` + H2 `bkg-dark`)

> **Anforderung:** FR-UI-030 (H1), FR-UI-031 (H2) · **Entscheidung:** ADR 0026
> (+ Nachtrag H2) · **Stand:** H1 fertig und am echten Dienst abgenommen
> (2026-07-18), H2 gebaut (2026-07-18). H3 (Selbst-Hosting/Air-Gap) folgt als
> eigenes Häppchen.

## Fachlich — warum

Die ASD-Basiskarte kam von `tile.openstreetmap.org` (hell) bzw.
`basemaps.cartocdn.com` (gedimmt dunkel): Community-/US-Anbieter-**Raster** ohne
QS-Zusage und ohne Betriebs-SLA; die OSM-Tile-Policy schließt Produktionslast
aus. Der Betreiber hat entschieden, auf die **amtlichen, qualitätsgesicherten**
Daten der deutschen Vermessungsverwaltungen umzustellen: **basemap.de Web
Vektor** (BKG/AdV) — monatlich aktualisiert, kostenfrei, ohne API-Key, mit
fertigen MapLibre-Styles. Gewählte Variante: **Farbe** (`bm_web_col`).

Für den Lotsen ändert sich mit `WAYFINDER_MAP_THEME=bkg`: helle amtliche
Vektorkarte (gestochen scharf in jedem Zoom), Track-Beschriftung unverändert in
Roboto Mono, Quellenvermerk © basemap.de / BKG in der Attributions-Ecke.

## Technisch — wie

Kern ist die **server-seitige Style-Pipeline** in `pkg/basemap` (Vorbild:
`pkg/weathertiles`-Idiome). Zwei Eigenschaften des Bestands erzwingen sie:

1. **Ein MapLibre-Style hat genau eine `glyphs`-URL.** Unsere Track-Labels
   brauchen das selbst-gehostete `Roboto Mono Medium` (`/glyphs`, ADR 0015).
   Das BKG-Style unverändert einzubinden würde `glyphs` auf den BKG-Server
   zeigen lassen → Track-Labels blieben stumm.
2. **Relative URLs** im Style (Sprite/Kacheln) lösen sich gegen den
   Auslieferungs-Ort auf und müssen absolutisiert werden, sobald Wayfinder den
   Style von der eigenen Origin serviert.

Bausteine:

- **`Service.StyleHandler` → `/basemap/style.json`:** holt das Upstream-Style
  (`WAYFINDER_BKG_STYLE_URL`), schreibt `glyphs` auf
  `/glyphs/{fontstack}/{range}.pbf` um, merkt sich die Upstream-Glyph-Vorlage
  für den Proxy, absolutisiert `sprite`/`sources[].tiles`/`sources[].url`
  (inkl. Wiederherstellung der `{z}`-Template-Klammern nach `url.String()`),
  injiziert die Attribution falls keine Quelle eine trägt. Cache-TTL 12 h;
  Refresh-Fehler ⇒ **stale** weiter servieren (Verfügbarkeit vor Frische);
  ganz ohne Cache ⇒ 502. `/ready` bleibt unberührt.
- **`Service.GlyphsHandler` (Weiche auf `/glyphs/`):** eingebettete Fontstacks
  (Liste aus `webui.GlyphFontstacks()`) gehen an den bisherigen
  Embedded-Handler; unbekannte Fontstacks werden gegen die Upstream-Vorlage
  proxied — Pfadsegmente validiert (kein `..`, keine Steuerzeichen,
  Range-Regex), `url.PathEscape` vor dem Einsetzen (SSRF/Traversal),
  2-MiB-LimitReader, begrenzter In-Memory-Cache (512 Einträge, Verdrängung).
  Ohne `bkg`-Theme ist die Weiche gar nicht montiert — `/glyphs` verhält sich
  exakt wie zuvor.
- **`cmd/wayfinder`:** Theme-Vokabular `dark|osm|bkg` (FR-CFG-002-Fallback),
  `Config.BKGStyleURL`, map-config liefert für `bkg` die Style-**URL**
  `/basemap/style.json` (String, kein Inline-Style — der Rewrite muss immer im
  Pfad sein), Metrik-Trio `wayfinder_basemap_fetch_success_total` /
  `_failures_total` / `_cache_age_seconds`.
- **Frontend:** `PALETTES.bkg` teilt die helle Vordergrund-Palette mit `osm`
  (beides helle Basen); sonst keine Änderung — root-relative Style-URLs sind
  im Bestand erprobt (Wetter-Radar-Kacheln).

## H2 — Radar-Scope-Dunkelvariante `bkg-dark` (FR-UI-031)

**Fachlich:** Der Lotsen-Default `dark` dimmt bisher ein fremdes CARTO-
Rasterbild auf 40 % — ein Trick, kein Design. `WAYFINDER_MAP_THEME=bkg-dark`
liefert erstmals einen **echten dunklen Radar-Scope aus den amtlichen
BKG-Vektordaten**: Near-Black-Grund, zarte Küsten/Grenzen/Straßen als
Struktur, gedämpft-helle Ortsnamen, gedimmte Schilder.

**Technisch:** Kein zweites, hand-gepflegtes Style-JSON (der BKG-Schema-Katalog
driftet mit Updates), sondern eine **regelbasierte HSL-Transformation** in der
bestehenden Pipeline (`pkg/basemap/scope.go`, `Config.Dark`): jede Farbe des
geholten Styles wird je Rolle (Fläche/Linie, Text, Halo) in ein Scope-Band
gemappt — Helligkeit invertiert (Kontrast-Ordnung bleibt), Sättigung kollabiert,
Alpha erhalten; rekursiv auch in Interpolations-Expressions und Legacy-Stops;
nicht parsebare Werte bleiben unverändert. Symbol-Icons werden via
`icon-opacity` gedimmt (numerische Werte skaliert, Expressions unangetastet).
Frontend: `bkg-dark` teilt die dunkle Vordergrund-Palette mit `dark`.

**Warum Default-Wechsel noch aussteht:** Staatsgrenzen-Abdeckung (siehe
Ehrliche Grenzen) — erst mit basemap.world wird `bkg-dark` zum sinnvollen
`dark`-Nachfolger.

## Ehrliche Grenzen

- **Abdeckung endet an der Staatsgrenze.** Auslandskontext (basemap.world:
  innerhalb D amtlich, außerhalb OSM/NaturalEarth) ist ein Folge-Häppchen; bis
  dahin bleibt für grenzüberschreitende Sektoren `dark` der praktikable
  Default. Deshalb wechselt H1 auch den Default **nicht**.
- **Dunkler Radar-Modus** bleibt bis H2 der CARTO-Dimm-Trick.
- **Server braucht Netz zum BKG** (oder Mirror); Kacheln/Sprite lädt der
  Browser weiter direkt vom BKG. Vollständig internetfreier Browser-Rand = H3
  (BKG-Download-Paket, `WAYFINDER_BKG_STYLE_URL` auf den Mirror).
- **Verifikation gegen den echten Dienst steht aus:** Die Entwicklungs-Sandbox
  hatte keinen Netzzugriff auf `sgx.geodatenzentrum.de` (Proxy-Policy); die
  Pipeline ist gegen einen realistisch geformten Test-Upstream (httptest)
  verifiziert. Der Betreiber prüft H0/H1 am echten Netz (Karte lädt, Labels
  intakt, Attribution sichtbar) — Punkt ist im STATUS als offen vermerkt.

## Tests

- `pkg/basemap/basemap_test.go`: Rewrite (Glyphs-Umschreibung, Absolutisierung
  inkl. `{z}`-Templates, Attribution injizieren/erhalten), Cache (TTL,
  stale-on-error, 502 ohne Cache, kaputtes Upstream-JSON), Glyph-Weiche
  (lokal vs. Proxy, Cache-Hit, Upstream-Fehler → 502, Pfad-Validierung erreicht
  nie den Upstream, Cache-Bound).
- `cmd/wayfinder/main_test.go`: Theme-Parsing `bkg`, map-config-Style,
  `WAYFINDER_BKG_STYLE_URL`-Default/Override.
- `internal/webui/webui_test.go`: `GlyphFontstacks` = `["Roboto Mono Medium"]`.
- `frontend/src/map/__tests__/palettes.test.js`: `bkg`-Palette vorhanden, hell,
  `dark` distinkt.
