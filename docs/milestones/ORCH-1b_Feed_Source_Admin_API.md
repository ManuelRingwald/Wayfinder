# ORCH-1b — Feed-Quell-Konfiguration über die Admin-API

> Zweites Häppchen von **ORCH-1** (ADR 0012). Baut auf dem Datenmodell aus
> ORCH-1a auf und macht die Quell-Konfiguration eines Feeds **über die
> Admin-API bedien- und schreibbar** — die Grundlage für den UI-Quell-Builder
> (ORCH-1c) und, später, für den Reconciler (ORCH-3).
>
> **Lieferumfang ORCH-1b:** zwei Admin-Endpunkte + DTO + server-seitige
> Validierung + Coverage-Ableitung am Rand. **Noch nicht** Teil: das Frontend
> (ORCH-1c).

## Fachlicher Hintergrund

Nach ORCH-1a trägt ein Feed ein `source_config`/`coverage_bbox`-Datenmodell, das
aber nur über den Store erreichbar war. Damit der Betreiber pro Feed festlegen
kann, *welche* Live-Quellen die zugehörige Firefly-Instanz öffnen soll (und mit
welchem groben Ausschnitt), braucht es einen bedienbaren API-Rand — analog zum
Feed-Lebenszyklus (ONB-5), aber für die Quell-Konfiguration.

## Was umgesetzt wurde

### Endpunkte (hinter `requireAdmin`)

- **`GET /api/admin/feeds/{feedID}/sources`** → `200`
  `{sources: [...], coverage_bbox: {...}|null}`. `sources` serialisiert als `[]`
  (nie `null`). Unbekannter Feed → `404`.
- **`PUT /api/admin/feeds/{feedID}/sources`** → `200` (kanonisch zurückgelesen).
  Body: `{sources: [...], coverage_bbox?: {...}}`.

Beide sind **Plattform-Operationen** (`requireAdmin`) — die Quell-Konfiguration
ist interne Orchestrierungs-Metadaten, nicht mandanten-seitig.

### Server-seitige Validierung (Defense am Schreib-Rand)

`PUT` validiert die Quell-Liste mit `SourceConfig.Validate` (ORCH-1a): geschlossenes
Vokabular, Per-Art-Regeln (Flächenquelle erfordert `bbox` und trägt keine
`sac`/`sic`; `radar_asterix` erfordert `sac`/`sic` 0..255), WGS84-BBox, `cred_ref`
als Verweis. Ein Verstoß liefert **`400` mit dem Quell-Index** (`InvalidSourceError`
→ `"invalid sources: store: source[1]: …"`) und schreibt **nichts** (kein
Teil-Write). Die Client-Validierung in ORCH-1c ist nur UX — der Server bleibt die
Grenze (Defense-in-Depth, wie `validateView`).

### Coverage: Ableitung mit Operator-Override

Fehlt `coverage_bbox` im Body, **leitet der Server** die grobe äußere BBox aus den
Quell-BBoxen + `defaultCoverageMarginKm` (50 km) ab — die kanonische Ableitung
liegt damit **einmal** in Go (`SourceConfig.CoverageBBox`), nicht doppelt im
Frontend. Eine explizit gesetzte `coverage_bbox` gewinnt (Operator-Override) und
wird WGS84-validiert (`validateBBox` → `400` statt `500` bei Unfug).

### DTO

`feedSourcesDTO{Sources store.SourceConfig, CoverageBBox *store.BBox}` — `Sources`
nutzt die JSON-Tags des Store-Modells (`type`/`bbox`/`sac`/`sic`/`cred_ref`)
direkt, kein paralleler Typ.

## Sicherheits-Betrachtung

- **Admin-gated:** Beide Routen liegen hinter `requireAdmin`; ein Nicht-Admin
  bekommt `403` und schreibt nie (`TestFeedSourcesRoutesForbidNonAdmin`).
- **Kein Klartext-Secret:** Es wandert nur `cred_ref` (ein Verweis) über die
  Leitung — der Secret-Wert selbst ist nicht Teil des DTO (NFR-SEC-004; der
  Secret-Speicher folgt ORCH-2).
- **Robuste Eingabe:** Body-Größe begrenzt (`MaxBytesReader`), Quell- und
  Coverage-Validierung vor jedem Write.
- **Schnittstellen-Wirkung:** Keine auf CAT062; rein Wayfinder-intern.

## Tests

`pkg/adminapi/adminapi_feeds_test.go`:
- `TestGetFeedSourcesDefaultsEmpty` (leere Konfig → `[]`/`null`),
  `TestGetFeedSourcesUnknownIs404`.
- `TestPutFeedSourcesRoundTripAndDerivesCoverage` (Round-Trip + abgeleitete
  Coverage padded die Quell-BBox), `…ExplicitCoverageOverrides` (Operator-BBox
  gewinnt), `…InvalidIsRejected` (4 Fälle → `400`, kein Write),
  `…UnknownIs404`, `TestFeedSourcesRoutesForbidNonAdmin`.

Der `fakeFeeds`-Store wurde um `Get`/`SetSourceConfig` erweitert.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-001** (Admin-API-Spalte ergänzt), **NFR-SEC-004**.

## Nächstes Häppchen

- **ORCH-1c** — Frontend-Quell-Builder im Feed-Dialog: Quell-Liste editieren
  (Typ-Dropdown, BBox-Eingabe, Sensor-Mix-Checkboxen als Template), BBox-Vorschlag
  aus der Mandanten-AOI + Marge, `GET`/`PUT` gegen die ORCH-1b-Endpunkte.
