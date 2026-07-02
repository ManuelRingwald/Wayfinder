# AP3 — Admin-Dashboard: Mandanten-Übersicht + zentrale Konfig

**Paket:** ADR 0009 · **Stufe:** S3 · **Modell:** Sonnet 4.6
**Issue:** #66 · **Abgeschlossen:** 2026-06-25

---

## Fachliche Motivation

Der Admin-Bereich war bis AP2 **ich-zentriert**: Top-Level-Tabs „Ansicht / Abos &
Feeds / Provisioning / Zugänge", jeder ein Ausschnitt der Konfiguration. Der
Plattform-Betreiber denkt aber **mandantenzentriert**: „Zeig mir Kunde X — welche
Features, welche Feeds, welche Sicht, wie viele Zugänge." AP3 dreht die
Perspektive um: eine **Übersichtstabelle aller Mandanten** und pro Mandant eine
**Detailseite**, die Status, Sicht, Features, Feeds und Zugänge an einem Ort
bündelt. Das macht Onboarding und Audit („wer hat was") auf einen Blick bedienbar.

Der Betreiber hat die Variante **„Voll neu, Tabs ersetzen"** gewählt: die alten
Top-Level-Tabs verschwinden zugunsten des Master/Detail-Flows.

---

## Technische Umsetzung

### Backend (`pkg/adminapi`) — additiv

1. **Aggregat-Endpoint** `GET /api/admin/overview` (admin-only): Liste aller
   Mandanten, je mit `status`, aktiven `features[]` (Katalog-Reihenfolge),
   abonnierten `feeds[]` und `user_count`. Ein Call statt N+1 (4 Calls × N
   Mandanten) beim Rendern der Tabelle. Fan-out sequenziell pro Mandant;
   jeder Backend-Fehler lässt den ganzen Call fehlschlagen (kein
   teil-befülltes Dashboard).

2. **Cross-Tenant-Sicht** `GET/PUT /api/admin/tenants/{tenantID}/view`
   (admin-only): die Selbstbedienungs-Route `/api/admin/view` arbeitet nur auf
   dem **eigenen** Mandanten (aus der Identity). Die Detailseite braucht aber
   Lesen/Schreiben der Sicht **jedes** Mandanten. `GetTenantDefault` (Tenant-
   Default, kein User-Override) wurde dafür ins `ViewStore`-Interface
   aufgenommen. Gleiche `validateView`-Server-Validierung; Ziel-`tenantID` aus
   dem Pfad; ein Schreibvorgang löst Live-Rescope aus (WF2-33).

Beide Routen sind `requireAdmin`-gegated (Defence-in-Depth zum äußeren
`RequireRole(admin)`).

### Frontend

- **`src/admin/geo.js`** (reine Funktion + Unit-Tests): `radiusNmToBbox` und
  `bboxToRadius`. Center+Radius (NM) ↔ AOI-Bbox wird **clientseitig** umgerechnet
  (Issue-Formel: `lat_delta=R/60`, `lon_delta=R/(60·cos φ)`); Pol-Singularität
  geklemmt, Rückrichtung aus der Breiten-Halbhöhe (round-trip-stabil). **Das
  Backend bleibt AOI-basiert** (WF2-21.2 unberührt) — die Umrechnung ist UX vor
  dem PUT. Beide Funktionen sprechen die **Backend-Wire-Form** der Bbox
  (`min_lat`/`min_lon`/`max_lat`/`max_lon`, wie `store.BBox` sie serialisiert) auf
  beiden Enden, damit die AOI ohne Schlüssel-Umbenennung an den Aufrufstellen
  gespeichert **und** zurückgelesen wird. (Ein früherer camelCase/snake_case-Bruch
  ließ `bboxToRadius` beim Laden `null` liefern → der Radius sprang nach Reload auf
  0 und wirkte „nicht gespeichert"; beim nächsten Speichern wurde die AOI zudem
  auf `NULL` überschrieben.)
- **`AdminTenants.vue`** (neu): Übersichtstabelle aus `loadOverview()`; Spalten
  Mandant/Status/Features/Feeds/Zugänge + „Konfigurieren" (emittiert `select`).
- **`AdminTenantDetail.vue`** (neu): pro Mandant Status-Umschalter, Standard-
  Ansicht (Zentrum + Radius + FL-Band, via `geo.js`), Feature-Toggles
  (`loadTenantEntitlements`/`setTenantEntitlement`) und die eingebetteten
  Abschnitte Feeds + Zugänge.
- **`AdminProvisioning.vue` / `AdminUsers.vue`**: optionaler `tenantId`-Prop —
  ist er gesetzt, entfällt der eigene Mandanten-Wähler und die Komponente
  arbeitet auf dem übergebenen Mandanten (Wiederverwendung der getesteten CRUD-
  Logik in der Detailseite).
- **`AdminView.vue`**: Tabs → Master/Detail (`selectedTenant === null` →
  Übersicht, sonst Detailseite mit „Übersicht"-Zurück).
- **Entfernt:** `AdminViewConfig.vue` und `AdminSubscriptions.vue` (durch die
  Detailseite abgelöst, keine Referenzen mehr).
- **Store-Actions:** `loadOverview`, `loadTenantView`, `saveTenantView`,
  `loadTenantEntitlements`, `setTenantEntitlement`.

**Schnittstellen-Wirkung:** rein additiv, **kein** CAT062/ICD-Bezug, **kein**
Schema-Change, **keine** neue Env-Variable. Der Server bleibt die Autorität (die
Geo-Umrechnung und das UI-Gating sind kosmetisch).

---

## Tests

### Go (`pkg/adminapi`)
- `TestGetOverviewAggregates`: Status, `user_count`, Feeds und nur **aktive**
  Features in stabiler Katalog-Reihenfolge.
- `TestGetTenantViewReadsDefault` / `…UnknownTenantIs404`.
- `TestPutTenantViewUpsertsAndRescopes` (Upsert auf Ziel-Mandant + Rescope) /
  `…RejectsInvalid` (ungültige Sicht erreicht den Store nicht).
- `TestCrossTenantRoutesForbidUser` um overview + tenant-view erweitert (403 für
  Rolle `user`).
- **real-PG** (`adminapi_integration_test.go`): overview-Aggregat (1 Zugang,
  2 Feeds, stca+multi_feed) + tenant-view PUT→GET-Round-Trip.

### Vitest
- `geo.test.js` (10): Breiten-Halbhöhe, Längen-Spreizung mit `1/cos φ`,
  null bei nicht-positivem/nicht-endlichem Radius, Pol-Clamp, Round-Trip,
  **Wire-Form (snake_case) auf beiden Enden** (Regressions-Schutz gegen den
  Radius-Reset).
- `admin.test.js` AP3-Block (8): `loadOverview` (Erfolg + Fehler),
  `loadTenantView` (200 + 404), `saveTenantView` (DTO + Validierungsfehler),
  `loadTenantEntitlements`, `setTenantEntitlement`.

---

## Qualitäts-Gates

- `go test ./...` ✅ · `go vet`/`gofmt` ✅ · real-PG `scripts/pg-test.sh` ✅
- `vitest run` ✅ (128 Tests) · `npm run build` ✅
- Doku: Register **FR-ADMIN-002** (AP3-Erweiterung), TECHNICAL §3 (Endpunkte),
  INSTALLATION (mandantenzentrierter Workflow), dieser Milestone.
