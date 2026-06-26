# ONB-6 — OpenAIP pro Mandant

> Letztes Paket des Zero-Touch-Onboarding-Epics (ADR 0011, Entscheidung **F3**).
> Jeder Mandant nutzt einen **eigenen optionalen OpenAIP-Schlüssel** und ruft die
> Luftraumdaten gegen **seine eigene Area of Interest** ab. Mandanten ohne eigenen
> Schlüssel fallen auf den globalen Schlüssel zurück.
>
> **Lieferung in zwei Commits:** (1) Backend (Schema, Store, Registry, Wiring,
> API, Tests), (2) Frontend (Store-Action + OpenAIP-Abschnitt in der
> Mandanten-Detailseite + vitest).

## Fachlicher Hintergrund

Bisher hielt Wayfinder **einen einzigen, prozess-globalen** OpenAIP-API-Schlüssel
(`WAYFINDER_OPENAIP_API_KEY`) und cachte **eine** Region (eine Box um das
konfigurierte Karten-Zentrum). Im Mehr-Mandanten-Betrieb ist das auf zwei Ebenen
falsch:

1. **Isolation.** Alle Mandanten teilen sich denselben Schlüssel und dieselbe
   Abruf-Quote. Mandant A erschöpft die Quote → Mandant B bekommt keine
   Luftraumdaten mehr. Und der gecachte Datensatz ist nicht mandantenspezifisch:
   im Zweifel sieht Mandant A die mit dem Konto eines anderen abgerufenen Daten.
2. **Region.** Mandant A überwacht Frankfurt, Mandant B Zürich. Mit einem globalen
   AOI-Cache bekommt jeder dasselbe Kompromiss-Rechteck — außerhalb der globalen
   Box gibt es gar keine Treffer.

ONB-6 löst beides: **eigener Schlüssel pro Mandant** + **eigene AOI pro Mandant**
(aus der bereits vorhandenen Mandanten-View-Konfiguration). Der globale Schlüssel
bleibt als **Fallback-Default** für Mandanten ohne eigenen Schlüssel erhalten
(Abwärtskompatibilität).

## Was umgesetzt wurde (Backend)

### 1. Schema — Migration 00009

`tenants.openaip_api_key TEXT` (nullable). `NULL` = kein eigener Schlüssel → der
Mandant nutzt den globalen Schlüssel. Additiv, nicht-destruktiv. Der Schlüssel ist
ein **Geheimnis**: er wird **nicht** in die geteilte `tenantColumns`/`Tenant`-Zeile
aufgenommen, sondern nur über dedizierte Accessoren gelesen/geschrieben — so kann
er nie über ein allgemeines Tenant-DTO an den Browser gelangen.

### 2. Store — `TenantRepo.GetOpenAIPKey` / `SetOpenAIPKey`

- `GetOpenAIPKey(id) (*string, error)` — liest **nur** die Spalte; `nil` =
  Fallback. `ErrNotFound` bei fehlendem Mandanten.
- `SetOpenAIPKey(id, *string)` — setzt (non-nil) oder **löscht** (nil) den
  Schlüssel; `ErrNotFound` bei fehlendem Mandanten.

### 3. Registry (`pkg/aeronautical`) — vom Singleton zum „Service je Mandant"

Der bestehende `Service` (ein Client, eine BBox, ein Cache) bleibt als **globaler
Fallback**. Neu daneben eine **`Registry`** (Muster wie der Feed-Manager aus
ONB-5): eine mutex-geschützte Map `tenantID → laufender Service`. Kern-API:

- `Start(tenantID, apiKey, bbox)` — baut einen Per-Mandant-`Service` (eigener
  Client über die injizierte `ClientFactory`, eigene BBox) und startet seine
  Refresh-Goroutine. **Idempotent** auf unveränderte `(apiKey, bbox)` → ein
  identischer Aufruf ist ein No-op (eine Re-Scope-Auslösung, die die AOI nicht
  bewegt, etwa ein Feed-Grant, startet die Fetch also **nicht** neu). Ein **leerer
  Schlüssel** startet keinen Service und stoppt einen vorhandenen → der Mandant
  fällt transparent auf den globalen Cache zurück.
- `Stop(tenantID)` — Goroutine beenden + abwarten + aus der Map entfernen.
- `Serve(tenantID, kind)` — liefert den Cache des Mandanten-Service; **fällt** auf
  den globalen Cache **zurück**, wenn kein eigener läuft (und auf eine leere
  Collection, wenn es auch keinen globalen gibt — graceful, nie ein Fehler).
- `Register(mux, mw, tenantOf)` — montiert die drei GeoJSON-Endpunkte
  **tenant-aware** (hinter der Middleware `mw`, Mandant via `tenantOf`).
- `StopAll()` — Shutdown. `FetchSuccessCount`/`FetchFailureCount` — **Summe** über
  global + alle Mandanten-Services (monotone Prozess-Metrik trotz Churn).

Die `ClientFactory` ist **injiziert** → die Registry kennt keine HTTP-/Transport-
Details und ist ohne echtes OpenAIP unit-testbar.

### 4. Endpunkte mandanten-aufgelöst

`/api/airspace`, `/api/navaids`, `/api/waypoints`:

- **Multi-Mandant:** hinter der Tenant-Middleware registriert; jeder Handler liest
  die `tenant_id` aus der Identity und liefert `registry.Serve(tenantID, kind)`.
  Kein Identity → leere Collection (graceful). Die Endpunkte sind damit jetzt
  **authentifiziert** — konsistent mit `/ws`, das ohnehin hinter der Middleware
  liegt; das Lagebild ist nicht öffentlich.
- **Single-Tenant:** unverändert der globale Cache, unauthentifiziert wie bisher.

### 5. API — Schlüssel je Mandant (hinter `requireAdmin`)

- `GET /api/admin/tenants/{id}/openaip` → `{"configured": <bool>}`. Meldet **nur**,
  ob ein Schlüssel gesetzt ist — **nie den Schlüssel selbst**.
- `PUT /api/admin/tenants/{id}/openaip` `{"api_key": "<schlüssel>"|null}` → 204.
  Leer/Whitespace/`null` = **löschen** (Rückfall auf den globalen Schlüssel). Nach
  dem Persistieren **Live-Apply**: der Per-Mandant-Refresh wird sofort (neu)
  gestartet — kein Neustart.

### 6. Live-Wirkung & Lebenszyklus

- **AOI-Änderung:** `putView` (Selbstbedienung) und `putTenantView` (cross-tenant)
  lösen zusätzlich zum Re-Scope (WF2-33) ein `triggerAeroApply` aus → der
  Mandanten-Refresh holt gegen die **neue** AOI. Dank Idempotenz ist das ein
  No-op, wenn sich die BBox nicht bewegt hat.
- **Mandant löschen** (ONB-4): `deleteTenant` ruft `triggerAeroStop` → der
  Per-Mandant-Service wird abgebaut.
- **Boot:** main.go iteriert beim Start den Mandanten-Katalog und ruft je Mandant
  `Apply` → die Per-Mandant-Caches sind warm, ohne auf eine Admin-Bearbeitung zu
  warten.

### 7. Entkopplung

`adminapi.TenantAeroLifecycle` (`Apply(ctx, id)` / `Stop(id)`) ist eine kleine
Schnittstelle — die Admin-API importiert die OpenAIP-/Transport-Schicht **nicht**.
Der konkrete Adapter (`tenantAeroLifecycle` in `cmd/wayfinder/aero.go`) löst den
effektiven Schlüssel (eigener, sonst global) und die AOI (View-AOI, sonst Box um
das View-Zentrum, sonst globale Karten-Box) auf und treibt die Registry. Eine
`nil`-Lifecycle (Single-Tenant / Tests) deaktiviert das Live-Apply.

## Byte-/Verhaltens-Vertrag

- `GET /api/admin/tenants/{id}/openaip`: 200 `{"configured": bool}`; 404 bei
  unbekanntem Mandanten. Der Schlüssel erscheint **nie** in einer Antwort.
- `PUT /api/admin/tenants/{id}/openaip`: 204; 400 bei ungültigem Body oder zu
  langem Schlüssel; 404 bei unbekanntem Mandanten.
- `/api/airspace|navaids|waypoints` (Multi-Tenant): Cache des Request-Mandanten;
  leere Collection ohne Identity. Best-effort — nie ein Fehler-Status aus einem
  OpenAIP-Ausfall (ADR 0004 unverändert).

## Sicherheits-Bewertung (CLAUDE.md §7)

- **Geheimnis-Hygiene:** Der Schlüssel wird isoliert gelesen, nie in ein
  allgemeines DTO aufgenommen und nie zum Browser zurückgegeben (GET meldet nur
  Präsenz). Setzen/Löschen ist **admin-only**.
- **Mandanten-Isolation:** Jeder Mandant fetcht mit seinem eigenen Schlüssel und
  Cache; der Endpunkt liefert ausschließlich den Cache des Request-Mandanten
  (`Serve(tenantID, …)`). Ohne eigenen Schlüssel: globaler Fallback (bewusst,
  abwärtskompatibel).
- **ADR-0004-Eigenschaften bleiben:** Schlüssel server-seitig, best-effort mit
  Last-Good-Cache, nicht-blockierender Start, Größengrenzen, „kein Schlüssel ⇒
  Feature still aus" — jetzt **pro Mandant**.
- **Browser-Rand:** Die GeoJSON-Endpunkte liegen im Multi-Mandanten-Betrieb jetzt
  hinter der Tenant-Middleware (vorher unauthentifiziert) — eine **Verschärfung**,
  konsistent mit `/ws`.

## Qualitäts-Gates (Backend-Commit)

- `scripts/pg-test.sh -p 1 ./...` (real-PG) ✅, `go vet`/`gofmt` ✅,
  `pkg/aeronautical` zusätzlich unter `-race` ✅.
- Tests:
  - `pkg/store/store_integration_test.go::TestIntegrationTenantOpenAIPKey`
    (real-PG: Get/Set/Clear-Roundtrip, nil-Default, ErrNotFound).
  - `pkg/aeronautical/registry_test.go`: Per-Tenant-Cache, Empty-Key-Fallback auf
    global, Start-Idempotenz (ein Client-Bau bei unveränderten Inputs, Neustart
    bei Schlüssel-Wechsel), Stop-Fallback, Count-Aggregation, Handler-Auflösung +
    No-Identity-Empty.
  - `pkg/adminapi/adminapi_openaip_test.go`: `GET …configured`/`…NeverLeaksKey`,
    `Set …Applies`/`…NullClears`/`…UnknownIs404`/`…InvalidBodyIs400`,
    `OpenAIPRoutesForbidNonAdmin`, `DeleteTenantStopsAero`.

## Bekannte Grenze

Wie ONB-5 wirkt der Live-Apply im **lokalen** Prozess. In einem Mehr-Replica-
Deployment würde jede Replica die geänderte Schlüssel-/AOI-Konfig erst bei ihrem
nächsten `Apply` (Boot / Admin-Bearbeitung an dieser Replica) übernehmen; ein
replica-übergreifendes Konfig-Watch ist nicht Teil von ONB-6 (offener Betriebs-
Härtungs-Punkt).

## Rückverfolgbarkeit

- **Anforderung:** FR-ADMIN-009 (`docs/requirements/README.md`).
- **Design:** ADR 0011 (`docs/decisions/0011-zero-touch-onboarding.md`), F3;
  ADR 0004 (OpenAIP-Datenquelle, Eigenschaften unverändert).
- **Vorgänger:** ONB-5 (`ONB-5_Feed_Lifecycle.md`).
- **Abschluss:** ONB-6 ist das **letzte** Paket des Zero-Touch-Epics.
