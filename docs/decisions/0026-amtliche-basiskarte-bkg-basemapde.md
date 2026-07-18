# ADR 0026 — Amtliche Basiskarte: BKG basemap.de Web Vektor statt OSM/CARTO-Raster

- **Status:** **AKZEPTIERT** ✅ (2026-07-18). Betreiber-Entscheidung: weg von
  OpenStreetMap als direktem Kachel-Lieferanten, hin zu den **amtlichen,
  qualitätsgesicherten** Daten des BKG (basemap.de). Style-Variante: **Farbe**
  (`bm_web_col`). Umsetzung als **H1** (Analyse-Freigabe 2026-07-18); der
  dunkle Radar-Style auf Vektorbasis (**H2**) und Selbst-Hosting/Air-Gap
  (**H3**) folgen als eigene Häppchen mit eigener Freigabe.
- **Datum:** 2026-07-18
- **Schnittstellen-relevant:** **nein** für den CAT062-Draht-Vertrag (kein
  Firefly-Bezug). **Ja** für den Browser-Rand: neuer Endpoint
  `/basemap/style.json`, veränderte Rolle des `/glyphs`-Endpoints (lokal +
  Proxy), zwei neue Umgebungsvariablen-Werte.
- **Bezug:** ADR 0001 (MapLibre GL JS), ADR 0015 (Glyph-Selbsthosting /
  Air-Gap-Linie), ASD-003a (Radar Dark Mode), FR-CFG-002 (invalid config →
  Fallback). Anforderungs-Register: **FR-UI-030**.

> ℹ️ **Auslöser:** Die Basiskarte des ASD kam bisher von
> `tile.openstreetmap.org` (helles `osm`-Theme) bzw. `basemaps.cartocdn.com`
> (gedimmtes `dark`-Theme) — Community- bzw. US-Anbieter-Raster ohne
> QS-Zusage und ohne Betriebs-SLA; die OSM-Tile-Policy schließt Produktionslast
> ausdrücklich aus. Für ein produktives ASD ist eine **amtliche,
> qualitätsgesicherte** Quelle angemessen.

---

## Kontext

**basemap.de Web Vektor** ist der gemeinsame Vector-Tiles-Dienst von Bund und
Ländern (AdV), betrieben vom BKG: amtliche Geobasisdaten, deutschlandweit
einheitlich, **monatlich aktualisiert**, kostenfrei, ohne API-Key, mit fertigen
MapLibre-Style-JSONs (Farbe `bm_web_col`, Grau `bm_web_gry`, Relief
`bm_web_top`). Für Auslandskontext existiert **basemap.world Web Vektor**
(innerhalb D amtlich, außerhalb OSM/NaturalEarth, halbjährlich). Zum
Selbst-Hosting bietet das BKG monatliche Download-Pakete an (Pfad für H3).

Drei Eigenschaften des Bestands machen die Migration zu mehr als einem
URL-Tausch:

1. **Ein Style hat genau eine `glyphs`-URL.** Wayfinders Track-Labels
   verlangen das selbst-gehostete `Roboto Mono Medium` (`/glyphs`,
   `internal/webui`, ADR 0015). Bindet man das BKG-Style-JSON unverändert ein,
   zeigt dessen `glyphs`-URL auf den BKG-Server, der unsere Fontstacks nicht
   kennt → **Track-Beschriftung bleibt stumm** (Callsign, Squawk, Höhe).
2. **Relative URLs im Style** (Sprite, ggf. Kacheln) lösen sich gegen den
   Auslieferungs-Ort auf. Serviert Wayfinder den Style von der eigenen Origin,
   müssen sie vorher gegen die Upstream-URL absolutisiert werden.
3. **Attribution ist Lizenzpflicht** (Quellenvermerk © basemap.de / BKG).

## Entscheidung

**Neues eingebautes Theme `bkg`** (`WAYFINDER_MAP_THEME=bkg`), umgesetzt als
server-seitige Style-Pipeline in **`pkg/basemap`**:

- **`/basemap/style.json`:** Wayfinder holt das Upstream-Style
  (`WAYFINDER_BKG_STYLE_URL`, Default: öffentliches BKG-„Farbe"-Style),
  **schreibt `glyphs` auf `/glyphs/…` um**, absolutisiert relative
  Sprite-/Kachel-URLs, ergänzt die Attribution falls fehlend, und cached das
  Ergebnis (TTL 12 h; bei Upstream-Ausfall wird der letzte gute Stand
  **stale** weiter serviert — Verfügbarkeit vor Frische; ohne jeden Cache
  ehrliches 502).
- **`/glyphs/` wird zur Weiche:** eingebettete Fontstacks (Roboto Mono) werden
  wie bisher aus dem Binary serviert; unbekannte Fontstacks (BKG-Kartenfonts)
  werden an den im Style verzeichneten Upstream-Glyph-Endpoint **proxied**
  (validierte Pfadsegmente, Größen-Limit, begrenzter In-Memory-Cache). Ohne
  aktives `bkg`-Theme bleibt `/glyphs` exakt der bisherige Embedded-Handler.
- **Kachel-Verkehr bleibt Browser → BKG** (öffentlich, schlüsselfrei). Ein
  vollständig eigen-gehosteter Pfad (H3) braucht nur eine andere
  `WAYFINDER_BKG_STYLE_URL`.
- **Defensive-Consumer-Regeln** wie am Feed-Eingang: Upstream nie blind
  vertrauen (LimitReader, Timeouts, JSON-Validierung, Pfad-Validierung gegen
  SSRF/Traversal am Glyph-Proxy).
- `osm` bleibt als **deprecatetes** helles Fallback-Theme bestehen (Ausbau
  frühestens nach H2, wenn `bkg`+dunkler Vektor-Style den Betrieb tragen);
  `dark` (CARTO) bleibt bis H2 der Lotsen-Default.

## Verworfene Alternativen

- **`WAYFINDER_MAP_STYLE_URL` direkt auf das BKG-Style zeigen lassen:**
  zerlegt die Track-Beschriftung (Glyphs-Konflikt, oben Punkt 1). Bleibt als
  Betreiber-Option für Sonderfälle, ist aber kein Migrationspfad.
- **BKG-Fonts zur Build-Zeit ins Binary einbetten** (statt Runtime-Proxy):
  reproduzierbarer, aber die Font-Menge des BKG-Styles ist ein bewegliches
  Ziel (Style-Updates) und die Sandbox/Build-Umgebung hat keinen
  BKG-Netzzugriff. Der Proxy folgt dem Upstream automatisch; die
  Einbettungs-Variante wird mit H3 (Offline-Paket) wieder relevant.
- **Alle Kacheln durch Wayfinder proxien:** löst ein Problem (Browser ohne
  Internet), das H3 sauberer über Selbst-Hosting löst, und zieht die volle
  Kartenlast durch den Track-Server.
- **basemap.de Web Raster (WMS/WMTS):** wäre der kleinste Eingriff, verschenkt
  aber den Vektor-Vorteil (eigener dunkler Radar-Style in H2, scharfe
  Darstellung, freie Gestaltung) und bliebe ein „fertiges Bild dimmen".

## Konsequenzen

- **Für den Lotsen (mit `WAYFINDER_MAP_THEME=bkg`):** helle Karte aus
  amtlichen, monatlich qualitätsgesicherten Daten; Track-Labels unverändert in
  Roboto Mono; Quellenvermerk © basemap.de / BKG in der Attributions-Ecke.
- **Ehrliche Grenzen:** (a) basemap.de endet an der Staatsgrenze — bis zur
  Anbindung von basemap.world (Folge-Häppchen) ist das Umland auf dem
  `bkg`-Theme leer; für grenzüberschreitende Sektoren bleibt bis dahin `dark`
  der praktikable Default. (b) Der dunkle Radar-Modus nutzt weiter CARTO
  (bis H2). (c) Der **Server** braucht Netz zum BKG (oder zum Mirror);
  Browser-seitig kommen Kacheln/Sprite weiter direkt vom BKG.
- **Betrieb:** neue Metriken `wayfinder_basemap_fetch_success_total` /
  `_failures_total` / `wayfinder_basemap_cache_age_seconds`; neue Env-Variable
  `WAYFINDER_BKG_STYLE_URL`; `/ready` bleibt vom Basemap-Pfad unberührt
  (Basiskarte ist nicht der sicherheitsrelevante Kern — das Lagebild ist es).
- **Folge-Häppchen:** **H2** dunkler Radar-Style aus denselben BKG-Kacheln
  (ersetzt den CARTO-Dimm-Trick), **H3** Selbst-Hosting per BKG-Download-Paket
  (macht den Browser-Rand internetfrei), Option **basemap.world** für
  Auslandskontext.

---

## Nachtrag H2 (2026-07-18): Radar-Scope-Dunkelvariante `bkg-dark`

**Entscheidung:** Die dunkle Variante wird **nicht** als hand-gepflegtes
Style-JSON gebaut, sondern als **regelbasierte Farb-Transformation** des zur
Laufzeit geholten BKG-Styles (`pkg/basemap/scope.go`), aktiviert über das neue
Theme **`bkg-dark`**:

- **Flächen/Linien:** Helligkeit invertiert in ein Near-Black-Band
  (L ≈ 0,035–0,38), Sättigung stark reduziert — helles Land wird der
  Scope-Hintergrund, dunkle Grenz-/Straßen-Striche werden zarte helle Struktur;
  die relative Kontrast-Ordnung des amtlichen Styles bleibt erhalten.
- **Kartentext:** in ein gedämpft-helles Band gehoben (L ≈ 0,52–0,72),
  Halos auf Backdrop-Dunkel — Ortsnamen lesbar, ohne mit den Track-Labels zu
  konkurrieren.
- **Symbol-Icons** (Straßenschilder etc.): per `icon-opacity` gedimmt.
- Nur **Farbwerte** werden angefasst (rekursiv auch in Expressions/Stops);
  Layer-Struktur, Filter und Zoom-Verhalten des amtlichen Styles bleiben
  unangetastet. Alpha bleibt erhalten; nicht parsebare Werte (benannte Farben)
  bleiben unverändert (lieber eine Originalfarbe als ein zerstörter Style).

**Warum Transformation statt Hand-Style:** Der BKG-Kachel-Schema-Katalog ist
groß und driftet mit Style-Updates; ein hand-gepflegter Dark-Style müsste jedem
Update nachgeführt werden. Die Transformation ist schema-agnostisch,
deterministisch und als reine Farb-Mathematik vollständig testbar.

**Default bleibt vorerst `dark` (CARTO):** basemap.de endet an der
Staatsgrenze; für grenzüberschreitende Sektoren wäre ein Umland-loser
Scope-Default ein Rückschritt. Der Default-Wechsel auf `bkg-dark` kommt mit dem
basemap.world-Häppchen (Auslandskontext). Register: **FR-UI-031**.

---

## Nachtrag basemap.world (2026-07-18): Umland-Kontext als Default-Quelle

**Entscheidung:** Der Default-Upstream-Style der `bkg`/`bkg-dark`-Themes
wechselt von basemap.de (`bm_web_col.json`, nur Deutschland) auf
**basemap.world Web Vektor** (`bm_web_wld_col.json`). Der Dienst kombiniert
**zwei Kachel-Archive**: innerhalb Deutschlands das unveränderte amtliche
basemap.de-Archiv (monatlich), außerhalb einen vom BKG kuratierten Weltkontext
aus OSM/NaturalEarth (halbjährlich). Damit verschwindet das leere Umland an
der Staatsgrenze — die letzte fachliche Hürde vor dem Wechsel des
Theme-Defaults `dark` → `bkg-dark`.

- **Kein Code-Umbau:** Die H1/H2-Pipeline (Glyph-Weiche, URL-Absolutisierung,
  Attribution, Dunkel-Transformation) ist schema-agnostisch und verarbeitet
  den world-Style unverändert; es ändert sich nur der Default von
  `WAYFINDER_BKG_STYLE_URL`.
- **Ehrliche Einordnung:** Amtlich ist weiterhin **nur der
  Deutschland-Anteil**; der Auslandsteil ist kuratierte Community-/
  NaturalEarth-Kartografie — als Orientierungshintergrund für das ASD bewusst
  akzeptiert (die sicherheitsrelevante Information ist das CAT062-Lagebild).
  Wer strikt amtliche Daten will, pinnt das Nur-Deutschland-Style
  (`GermanyOnlyStyleURL`).
- **Theme-Default-Wechsel** `dark` → `bkg-dark` folgt als eigener
  Mini-Schritt nach dem Betreiber-Smoke-Test des world-Styles.
  Register: **FR-UI-032**.

---

## Nachtrag Ausbau OSM/CARTO (2026-07-18): `bkg-dark` ist der Default, Alt-Themes entfernt

**Entscheidung (Betreiber-Vorgabe: „sauber ausbauen, nichts Altes übrig
lassen"):** Nach bestandenem basemap.world-Smoke-Test wird **`bkg-dark` der
Theme-Default**, und die beiden Alt-Basiskarten werden **vollständig
ausgebaut** — der eingebaute OSM-Raster-Style (`tile.openstreetmap.org`) und
der CARTO-Dunkel-Raster (`basemaps.cartocdn.com`, der „Dimm-Trick" aus
ASD-003a) fliegen samt Inline-Style-Konstanten aus dem Code. Wayfinder
kontaktiert damit **keine OSM-/CARTO-CDNs mehr**; einzige Karten-Quelle ist
der BKG-Dienst (bzw. ein Mirror).

- **Migrations-Freundlichkeit statt Bruch:** Die Env-Werte `dark` und `osm`
  bleiben als **deprecatete Aliase** akzeptiert (`dark` → `bkg-dark`,
  `osm` → `bkg`) und erzeugen eine Startup-Warnung — bestehende Deployments
  starten unverändert, nur eben auf den amtlichen Karten. Unbekannte Werte
  fallen wie bisher auf den Default (FR-CFG-002).
- **Frontend:** Paletten-Vokabular auf `bkg`/`bkg-dark` reduziert (Server
  liefert nach Aliasing nur noch diese Themes).
- **Audit-Spur:** Historische ADRs/Milestones (ASD-003a Radar Dark Mode,
  ADR 0015-Bezüge) bleiben unverändert stehen — bereinigt ist die *aktuelle*
  Doku (README/INSTALLATION/TECHNICAL). Register: **FR-UI-033**.
