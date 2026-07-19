# ASD-017 — Sektor-Suche: Straßen und Orte im eigenen Sektor finden (#277)

> **Register:** FR-UI-037 · **Entscheidung:** ADR 0028 · **Issue:** #277

## Fachlich — warum

Operativer Anlass (Betreiber): *„Es soll eine Drohne starten aus der
Friedrichstraße. Nun will der Lotse schnell diese Straße finden."* Bei einer
Drohnen-Meldung, einem Vorkommnis am Boden oder einer Ortsangabe im Funk muss
der Lotse einen **benannten Ort im Sektor** in Sekunden auf dem Scope haben —
manuelles Absuchen der Karte ist zu langsam und bindet Aufmerksamkeit.

Die Suche arbeitet **ausschließlich auf den BKG-Kartendaten, die Wayfinder
ohnehin bezieht** (ADR 0026): kein externer Geocoder (Lizenzfrage — der
BKG-Geokodierungsdienst ist nur für Behörden nach § 14 EGovG kostenfrei; und
externe Dienste rissen im Air-Gap-Betrieb ab), keine neue Datenquelle, keine
neue Vertrauensgrenze. Das Suchgebiet ist bewusst der **eigene Sektor** (AOI,
~30 NM) — genau der Bereich, für den der Lotse verantwortlich ist.

## Technisch — wie

**Backend (`pkg/basemapsearch`, ADR 0028):**

- **Lazy Index je AOI:** Beim ersten Suchaufruf lädt ein Worker-Pool
  (Concurrency 8) die **z14-Vektor-Tiles** der effektiven View-AOI (ohne AOI:
  30-NM-Box um das View-Zentrum). Single-Flight: parallel anfragende Clients
  teilen sich einen Build; der Handler antwortet währenddessen `202
  {status:"building"}`.
- **MVT-Dekodierung** via `github.com/paulmach/orb` (gzip-Sniffing am
  Magic-Byte, 4-MiB-LimitReader je Tile). **Schema-tolerant:** exakter
  `name`-Key, sonst der erste Property-Key, der `name` enthält (z. B.
  `objektname`) — das reale basemap.de-Schema war aus der Sandbox nicht
  live verifizierbar.
- **Normalisierung + Clustering:** lowercase, Umlaut-Faltung (ä→ae, ß→ss),
  „straße/strasse/str." → `str`; gleichnamige Features im 3-km-Umkreis
  verschmelzen zu **einem** Treffer (eine Straße = viele Tile-Features).
- **Ranking:** Präfix-Treffer vor Infix-Treffern (kürzere zuerst), min.
  2 Zeichen, max. 20 Treffer.
- **Limits (fail-safe):** 4096 Tiles je Index — eine übergroße AOI wird
  **Zentrum-erhaltend** geclampt (Sektor-Kern bleibt vollständig suchbar);
  8 Indexe (LRU), 250 k Einträge, Build-Timeout 5 min, TTL 24 h mit
  Stale-Serve + Hintergrund-Rebuild.
- **Endpoint:** `GET /api/basemap/search?q=…` hinter Tenant-Middleware +
  `pwGate` + Impersonations-Read-Scope; **Feature-Gate `basemap`
  fail-closed → 403** — anders als das kosmetische Sidebar-Gate, weil der
  Index-Bau reale Server-Ressourcen kostet (tausende Tile-Fetches).
- **Metriken:** `wayfinder_basemap_search_builds_total{result}`,
  `wayfinder_basemap_searches_total` (TECHNICAL § 5.4c).

**Frontend:**

- **`MapSearch.vue`** im Top-Right-Cluster des Scopes: Debounce 300 ms,
  Building-Poll 1,5 s, Stale-Response-Guard (Sequenznummer), Statushinweise
  („Suchindex wird aufgebaut …" / „Keine Treffer." / „Kein Suchgebiet
  konfiguriert."), Enter wählt den ersten Treffer, Esc/Clear räumt auf.
  Sichtbar nur mit `basemap`-Entitlement (kosmetisches Gate wie die Sidebar,
  fail-open für Admins im Gast-Modus).
- **Marker + Kamera:** Treffer-Klick → `showSearchMarker(lon, lat, name)` in
  der Engine: magenta Ring + Namens-Label (`SEARCH_MARKER_*`,
  `#e040fb` — bewusst distinkt von allen Track-Farben und dem cyanen
  Selektions-Ring) als **oberste** Layer-Ebene (ein gefundener Ort darf nie
  unter dem Lagebild verschwinden) + `easeTo` auf den Ort.

## Ehrliche Grenzen

- **Suchqualität = Tile-Schema auf z14:** Straßen, Plätze, Siedlungen,
  Gewässer — **keine Hausnummern** und keine Adress-Interpolation.
  Adress-genaue Suche bliebe dem dokumentierten Upgrade-Pfad
  BKG-Geokodierungsdienst vorbehalten (nur-Behörden-Lizenz, Issue #277).
- **Reales BKG-Schema nicht live verifiziert:** Die Sandbox erreicht
  `sgx.geodatenzentrum.de` nicht; End-to-End ist gegen orb-kodierte
  MVT-Fixtures getestet. Der Betreiber-Smoke-Test (bekannte Straße im
  Sektor suchen) validiert Schema + Kategorie-Labels am echten Dienst.
- **Erste Suche je Sektor ist kalt** (Index-Bau, typisch Sekunden bis
  wenige Minuten je nach AOI-Größe und Upstream); danach dauerhaft warm
  (TTL 24 h, Stale-Serve).

## Tests

- `pkg/basemapsearch/search_test.go`: TileXY-Referenzpunkt (unabhängig
  verifiziert), Zentrum-erhaltendes Clamping, Normalisierungs-Tabelle,
  3-km-Clustering, Präfix-Ranking, End-to-End gegen einen httptest-Upstream
  mit orb-kodierten MVT-Tiles (gzip + plain, `name`- und
  `objektname`-Features, unbenannte Features bleiben draußen),
  Handler-Gates/Statuscodes (403/503/202/200), LRU-Eviction.
- `frontend/src/components/__tests__/mapSearch.test.js`: gemountete
  Komponente gegen gestubbtes `fetch` (Debounce + Endpoint + Trefferliste +
  Select-Emit, Mindestlänge, 202-Poll bis ready, Esc bricht den Poll ab);
  Source-Guards für Engine-/MapCanvas-/AsdView-Verdrahtung und die
  oberste-Ebene-Position des Markers.
