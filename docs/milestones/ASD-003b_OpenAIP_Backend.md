# ASD-003b — OpenAIP-Backend (Client + Cache + Endpoints)

Teil von **ASD-003 (Aeronautical Map Layer)**, Häppchen 3b. Entscheidung:
**ADR 0004**. Siehe `docs/ROADMAP.md` Paket #13.

## Fachlich

Die Luftraum- und Navigations-Strukturen (Lufträume, VOR/NDB, Waypoints) sind
Luftfahrt-Kontext und gehören **nicht** zum CAT062-Track-Strom. ASD-003 bezieht
sie **live von OpenAIP**. Damit das sicherheitsrelevante ASD nicht von einer
externen Quelle abhängt, läuft der Abruf server-seitig, gecached und
best-effort.

## Technisch

Neues Paket **`pkg/aeronautical`**:

- **`client.go`** — OpenAIP-Client:
  - `Fetch(ctx, kind, bbox)` ruft `/airspaces`, `/navaids` bzw.
    `/reporting-points` ab (API-Key im Header `x-openaip-api-key`), liest die
    Antwort **längen-begrenzt** (`maxResponseBytes`, 32 MiB) und transformiert
    sie in eine GeoJSON-`FeatureCollection`.
  - **Defensiver Konsument** (ADR 0004 / CLAUDE.md §7): Timeout (vom Aufrufer
    gesetzt), Größen-Limit, `validGeometry` verwirft Items mit fehlender/
    ungültiger Geometrie statt den ganzen Abruf scheitern zu lassen; Nicht-200
    und Decode-Fehler werden als Fehler gemeldet (→ Last-Good-Cache greift).
  - `BoundingBoxFromCenter(lat, lon, radiusKM)` baut das Abfragefenster um den
    Kartenmittelpunkt (Längengrad-Spanne wächst mit der Breite).
  - **Properties** je Feature: `kind`, `name`, `ident`, `type` (OpenAIP-Enum)
    und für Navaids `navaid_kind` (VOR/NDB/DME/… best-effort), `frequency`.
- **`service.go`** — Cache + Refresh + Endpoints:
  - In-Memory-Cache je Kind (`atomic.Pointer[FeatureCollection]`), **Last-Good-
    Fallback**: ein fehlgeschlagener Refresh behält den letzten guten Stand.
  - `Run(ctx)`: einmaliger Refresh beim Start + periodisch
    (`WAYFINDER_OPENAIP_REFRESH`, Default 24 h). **Nicht-blockierend**, meldet
    nie fatale Fehler; deaktiviert (kein Key) kehrt es nach einem Warn-Log
    sofort zurück.
  - `Register(mux)`: `/api/airspace`, `/api/navaids`, `/api/waypoints` liefern
    den gecachten Stand — oder eine **leere FeatureCollection** (HTTP 200),
    solange nichts gecached ist (graceful degradation).
  - Metriken: `FetchSuccessCount`/`FetchFailureCount`/`CacheAgeSeconds`.
- **`geojson.go`** — minimale GeoJSON-Ausgabetypen + `EmptyCollection`.

**Verdrahtung** (`cmd/wayfinder/main.go`):
- Neue Config: `WAYFINDER_OPENAIP_API_KEY` (Secret; ohne Key Feature aus),
  `WAYFINDER_OPENAIP_BASE_URL` (Override), `WAYFINDER_OPENAIP_REFRESH`
  (Go-Duration, Default 24 h), `WAYFINDER_OPENAIP_RADIUS_KM` (Default 250).
- `aeroService.Run(ctx)` als eigene Goroutine; Endpoints am `:8081`-Mux
  (hinter der bestehenden Auth-/Origin-Absicherung); `/metrics` (`:8080`) um
  `wayfinder_openaip_fetch_success_total`/`_failures_total`/
  `wayfinder_openaip_cache_age_seconds` ergänzt.

## Architektur-Hinweis

Der Track-Pfad (CAT062 → WebSocket → Karte) ist von OpenAIP **vollständig
entkoppelt**: kein gemeinsamer Zustand, eigener Goroutine-Lebenszyklus, und
`/ready` bleibt unberührt. Ein OpenAIP-Ausfall lässt das Lagebild unverändert
weiterlaufen (ADR 0004).

## Tests

- `pkg/aeronautical/client_test.go`: Transform inkl. API-Key-Header,
  bbox-Query, Pfad; Überspringen kaputter Geometrie; Nicht-200 → Fehler;
  Toleranz gegenüber fehlendem `items`; bbox-Geometrie; `validGeometry`-Tabelle.
- `pkg/aeronautical/service_test.go`: leere Collection vor erstem Refresh;
  Cache-Befüllung; **Last-Good bei Fehler**; deaktivierter `Run` kehrt sofort
  zurück; `CacheAgeSeconds`.

## Nächste Schritte

- **3c**: Luftraum-Layer (`/api/airspace`) im Frontend + Layer-Steuerung.
- **3d**: Waypoints + VOR/NDB (`/api/navaids`, `/api/waypoints`).
