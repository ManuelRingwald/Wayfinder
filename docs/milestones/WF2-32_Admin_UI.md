# WF2-32 — Admin-UI (Frontend)

> **Stufe:** 3 · **Paket:** WF2-32 — **Consumer des Admin-Backends** ·
> **Einstufung:** S3 · Sonnet 4.6 · **Grundlage:** ADR 0001 (Vue/Vuetify-Stack),
> baut auf WF2-31/31b (Admin-API) + WF2-21.2 (View-Filter-Semantik).

## Warum (fachlich)

Das gesamte Mandanten-Provisioning lag bisher nur als REST-API/DB vor — bedienbar
nur per `curl`. Für ein SaaS untauglich. WF2-32 gibt den beiden Admin-Rollen ihre
bedienbare Oberfläche unter `/admin`:

- **tenant_admin** — Self-Service der **eigenen** Sicht: Kartenzentrum/Zoom, **AOI-
  Bounding-Box**, **FL-Band**, Standard-Layer; dazu ein Lese-Blick auf gebuchte
  Feeds und den Katalog. Kein Ticket mehr für eine AOI-Anpassung.
- **super_admin** — **Cross-Tenant-Provisioning** über die Oberfläche: Mandanten
  listen, Feeds zuweisen/entziehen (grant/revoke) ohne DB-Zugriff.

Damit schließt sich die Schleife Backend-API (WF2-31/31b) → menschlich bedienbares
Kontroll-Surface.

## Kurskorrekturen des Projektverantwortlichen

1. **History-Mode statt Hash-Routing.** Saubere Routen (`/` ASD, `/admin`
   Dashboard), kein `/#/admin`. Konsequenzen serverseitig nachgezogen (siehe
   „Technisch").
2. **Kompletter Komponenten-Austausch, kein Overlay.** Auf `/admin` wird die ASD-
   Karte **vollständig unmounted** (MapLibre-Canvas + WebSocket + Timer frei) —
   das Dashboard ist eine losgelöste, eigenständige View.

## Was (technisch)

### Routing & SPA-Serving
- **`vue-router` (History-Mode)** — `src/router/index.js`: `/` → `AsdView`
  (eager), `/admin` → `AdminView` (**lazy import** → eigener Bundle-Chunk
  `AdminView-*.js`, ~14 kB; belastet den operativen ASD-Load nicht).
- **App-Umbau:** `App.vue` ist jetzt eine dünne Shell (`<v-app><router-view/>`); der
  bisherige Vollbild-ASD-Inhalt wandert nach `src/views/AsdView.vue`. **Kein
  `<keep-alive>`** — Verlassen der Lage unmountet die Karte wirklich; `MapCanvas.
  onUnmounted → mapEngine.destroy()` schließt WS, Reconnect-Timer und Intervalle
  und entfernt die WebGL-Karte. Das erfüllt den geforderten Komponenten-Austausch.
- **Backend-Namespace bereinigt:**
  - Rollen-Probe **`/admin` → `GET /api/admin/whoami`** verschoben (in
    `pkg/adminapi`, hinter demselben Rollen-Gate).
  - **SPA-History-Fallback** in `internal/webui/webui.go`: unbekannte Pfade liefern
    `index.html` aus (Deep-Links wie `/admin` überleben Reload/Bookmark). Echte
    Assets werden weiter ausgeliefert; das API-Surface ist über speziellere
    Mux-Pattern registriert und wird vom Fallback **nie** beschattet. Shell wird
    `no-cache` ausgeliefert (gehashte Assets bleiben cachebar).

### Frontend-Bausteine
- **`stores/admin.js`** (Pinia): `apiFetch`-Wrapper (JSON, normalisiert auf
  `{ok,status,data,error}`, 401/403/network-tolerant); Actions `loadIdentity`
  (whoami), `loadView`/`saveView`, `loadFeeds`/`loadSubscriptions`,
  `loadTenants`/`loadTenantSubscriptions`/`grant`/`revoke`; Getter `role`/
  `isSuperAdmin`/`isAuthorized`.
- **`views/AdminView.vue`** — Shell: App-Bar (Identität + „Zur Lage"), Rollen-Probe
  beim Mount, Tabs **Ansicht / Abos & Feeds / Provisioning**. Der Provisioning-Tab
  ist `v-if="isSuperAdmin"`-gegated. Fehler/Erfolg als schließbare Banner.
- **`components/admin/AdminViewConfig.vue`** — View-Editor (Zentrum/Zoom, AOI-
  Toggle+BBox, FL-Band-Toggle, Layer-Switches). **Validierungs-Parität vor dem
  PUT** über `src/admin/validateView.js` (spiegelt `pkg/adminapi.validateView`).
- **`components/admin/AdminSubscriptions.vue`** — read-only: gebuchte Feeds +
  Katalog (mit „gebucht"-Markierung).
- **`components/admin/AdminProvisioning.vue`** (super_admin) — Mandant wählen →
  Feed-Tabelle mit Zuweisen/Entziehen; nach jeder Aktion Re-Fetch.

### Sicherheit
Das Rollen-Gating der UI ist **kosmetisch** — der Server erzwingt jede Grenze
unabhängig (`requireSuper → 403`, Tenant-ID aus der Identity). Die Client-
Validierung ist eine UX-Höflichkeit, **nie** die Sicherheitsgrenze: der Server
bleibt die Wahrheit (Defense-in-Depth).

**Kein Schema-Change.** Eine neue Frontend-Abhängigkeit (`vue-router`).

## Tests

- **Validierungs-Parität** (`src/admin/__tests__/validateView.test.js`, Vitest):
  wohlgeformte Config besteht; jede Server-Regel (Lat/Lon/Zoom-Bereiche, AOI
  out-of-range/invertiert, FL negativ/invertiert) wird abgelehnt.
- **Store** (`src/stores/__tests__/admin.test.js`, Vitest, gemocktes `fetch`):
  `loadIdentity` setzt Identität + `isSuperAdmin`; 403 → `accessError`, unautorisiert;
  `saveView` PUTtet die **exakte DTO** und setzt State/Notice; 400 → Fehlerbanner;
  `grant` POSTtet `{feed_id}` an die Pfad-Route, `revoke` DELETEt; 403 aus `grant`
  (tenant_admin-Versuch) wird gemeldet.
- **SPA-Fallback** (`internal/webui/webui_test.go`, Go): Deep-Links (`/admin`,
  `/admin/tenants/5/…`) liefern die Shell (200, HTML); echte Assets (`/favicon.svg`)
  werden **nicht** beschattet; Root liefert `index.html`.
- **whoami** (`pkg/adminapi/adminapi_test.go`): `GET /api/admin/whoami` meldet die
  Identität/Rolle (200) bzw. `401` ohne Identity.

Gates grün: Vitest (62 Tests), `npm run build`, `go build/vet/test`, `gofmt`,
`scripts/pg-test.sh`.

## Abgrenzung / Nächstes

- **Live-Apply (WF2-33)** bewusst vertagt: eine View-/Abo-Änderung wirkt auf **neue**
  `/ws`-Verbindungen (Reconnect genügt vorerst).
- **Nicht enthalten:** Anlegen/Löschen von Tenants/Feeds/Usern über die UI (heute
  `bootstrap`/`feed`-CLI); Entitlements/Feature-Flags.
- **Nächster Schritt:** WF2-33 (Live-Apply, laufende Subscriptions re-skopieren) oder
  WF2-30 (Config-Cache, bei gemessenem Bedarf) — Reihenfolge nach Abstimmung.
