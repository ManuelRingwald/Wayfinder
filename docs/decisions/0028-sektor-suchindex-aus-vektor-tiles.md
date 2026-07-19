# ADR 0028 — Sektor-Suche: Server-seitiger Suchindex aus den Vektor-Tiles der Basiskarte

- **Status:** **AKZEPTIERT** ✅ (2026-07-19). Betreiber-Freigabe zu #277 mit
  allen drei Weichen: Suche hinter dem bestehenden `basemap`-Entitlement,
  Tile-Deckel ~4096 mit Zentrum-erhaltendem Clamping, 30-NM-Box um das
  effektive View-Zentrum als Fallback ohne AOI.
- **Datum:** 2026-07-19
- **Schnittstellen-relevant:** nein (kein CAT062-/Firefly-Bezug; ein neuer
  lesender Wayfinder-Endpoint + Browser-UI).
- **Bezug:** ADR 0026/0027 (BKG-Basiskarte als Entitlement-Layer), WF2-50
  (Feature-Entitlements), ADR 0023 (View-Konfiguration liefert AOI/Zentrum),
  Issue #277. Register: **FR-UI-037**.

## Kontext

Operativer Anlass (Betreiber): *„Es soll eine Drohne starten aus der
Friedrichstraße. Nun will der Lotse schnell diese Straße finden."* Der Lotse
braucht eine **Ortssuche im eigenen Sektor** — Straßen, Plätze, Siedlungen —
ohne die Karte manuell abzusuchen.

Ein Deutschland-weiter Geocoder wäre der klassische Weg, scheidet aber aus:
Der **BKG-Geokodierungsdienst** ist nur für Behörden (§ 14 EGovG) kostenfrei,
externe Dienste (Nominatim u. a.) verlassen die amtliche BKG-Datenbasis
(Sinn der Migration, ADR 0026) und reißen im Air-Gap-Betrieb (INSTALLATION
§ 8.0a) ab. Die Betreiber-Idee („Kandidat D"): Das Suchgebiet ist ohnehin
**klein und bekannt** — die AOI des Mandanten (~30 NM um den Flughafen). Die
Namen stehen bereits in den **Vektor-Tiles der Basiskarte**, die Wayfinder
sowieso bezieht (und im Air-Gap-Fall selbst hostet).

## Entscheidung

Wayfinder baut sich **selbst einen Suchindex aus den Vektor-Tiles** des
konfigurierten Basiskarten-Styles (`pkg/basemapsearch`):

1. **Lazy je AOI, server-seitig:** Beim ersten Suchaufruf eines Mandanten
   lädt ein Worker-Pool die z14-Tiles der AOI-BBox (Single-Flight: parallel
   anfragende Clients teilen sich EINEN Build), dekodiert die MVT-Layer
   (`github.com/paulmach/orb`), extrahiert benannte Features
   (schema-tolerant: exakter `name`-Key, sonst jeder Key, der `name`
   enthält) und clustert gleichnamige Einträge im 3-km-Umkreis (eine Straße
   = viele Tile-Features → EIN Treffer).
2. **Begrenzt und gedeckelt (fail-safe):** Max. **4096 Tiles** je Index —
   eine übergroße AOI wird **Zentrum-erhaltend** auf den Deckel geclampt
   (der Kern des Sektors bleibt vollständig durchsuchbar, die Ränder fallen
   weg). Max. 8 Indexe (LRU), max. 250 k Einträge, 4 MiB je Tile
   (LimitReader), 5-min-Build-Timeout, TTL 24 h mit Stale-Serve +
   Hintergrund-Rebuild.
3. **Ohne AOI: 30-NM-Box** um das effektive View-Zentrum (Weiche 3) — jeder
   Mandant kann suchen, auch bevor eine AOI konfiguriert ist.
4. **Hinter dem `basemap`-Entitlement (Weiche 1), fail-closed am Server:**
   Der Index-Bau kostet reale Ressourcen (tausende Tile-Fetches) — ein nicht
   berechtigter Mandant darf ihn nicht auslösen (`403`). Das UI-Gate
   (Suchfeld nur mit Entitlement sichtbar) ist wie in der Sidebar rein
   kosmetisch.
5. **Protokoll:** `GET /api/basemap/search?q=…` → `202 {status:"building"}`
   während des Baus (UI pollt), `200 {status:"ready", results:[…]}` mit max.
   20 Treffern (Präfix-Treffer vor Infix-Treffern, Normalisierung
   ä→ae/ß→ss/„straße|strasse|str." → `str`), `503` ohne auflösbares
   Suchgebiet.
6. **UI:** Suchfeld im Top-Cluster des Scopes (`MapSearch.vue`, Debounce
   300 ms); ein gewählter Treffer setzt einen **magenta Marker** (Ring +
   Namens-Label, oberste Ebene — ein gefundener Ort darf nie unter dem
   Lagebild verschwinden) und fährt die Kamera hin. Esc/Leeren entfernt ihn.

## Verworfene Alternativen

- **BKG-Geokodierungsdienst:** Adress-genau, aber lizenzpflichtig für
  Nicht-Behörden; als späterer Upgrade-Pfad dokumentiert (Issue #277).
- **Externe Geocoder (Nominatim u. a.):** verlassen die amtliche Datenbasis
  und den Air-Gap.
- **Client-seitige Suche über geladene Tiles
  (`queryRenderedFeatures`/`querySourceFeatures`):** findet nur, was der
  Browser gerade geladen hat — beim Use Case (Straße JETZT finden, egal wo
  im Sektor) genau falsch.

## Konsequenzen

- Neue Abhängigkeit **`github.com/paulmach/orb`** (MVT-Dekodierung; reine
  Go-Bibliothek, BSD-3). Erste Nicht-Stdlib-Abhängigkeit mit
  Protobuf-Parsing im Eingangs-Pfad → der Decoder läuft nur über
  entitlement-geschützte, größen-limitierte Fetches gegen den konfigurierten
  Style-Upstream (nicht gegen beliebige Nutzereingaben).
- Die Suchqualität ist durch das Tile-Schema begrenzt (Namen auf z14): gut
  für Straßen/Orte/Gewässer, keine Hausnummern. Adress-Suche bliebe dem
  Geokodierungsdienst-Upgrade vorbehalten.
- Metriken: `wayfinder_basemap_search_builds_total{result}`,
  `wayfinder_basemap_searches_total` (TECHNICAL § 5.4b).

## Nachtrag (2026-07-19) — Betreiber-Smoke-Test: TileJSON, Fehler-Status, Treffer-Kontext, Zoom

Der erste Live-Betrieb gegen den echten BKG-Stil deckte drei Punkte auf:

1. **TileJSON-Quellform.** Die realen basemap.de/world-Styles deklarieren die
   Vektor-Quelle als TileJSON-Verweis (`url`), nicht inline (`tiles`) — jeder
   Bau scheiterte mit „style has no vector tile source". `tilesTemplate` löst
   nun beide Formen auf (`tilesFromTileJSON`, defensiv: ctx-bound, 1-MiB-Limit,
   Status-Check). Fixtures decken beide Formen ab.
2. **Ehrlicher Fehler-Status.** Ein fehlgeschlagener Erst-Bau erschien als
   endloses „wird aufgebaut". `Search` meldet ihn stabil als `status:"error"`
   (sticky über Hintergrund-Retries); die UI zeigt „nicht verfügbar" und pollt
   gedrosselt weiter.
3. **Treffer-Kontext + Zoom (Bedienbefund).** Gleichnamige Treffer waren nicht
   unterscheidbar und der Klick zentrierte ohne Zoom. Additive Ergebnisfelder
   `near`/`dist_nm`/`bearing_deg` (Ort ≤ 8 km best-effort + Radial vom
   Sektorzentrum, `enrichHits`) machen die Zeilen unterscheidbar; das Frontend
   fährt den Treffer mit festem absolutem Fokus-Zoom (`SEARCH_RESULT_ZOOM=14`,
   `flyTo`) an. Betreiber-Entscheidung zur Unterscheidung: **Ort + Radial
   kombiniert** mit Radial als schema-unabhängiger Rückfalllinie.
