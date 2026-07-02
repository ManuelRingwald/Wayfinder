# WF2-31 — Tenant-skopiertes Admin-API (REST)

> **Stufe:** 3 (Dynamische Konfiguration & Admin-UI) — **erster Baustein** ·
> **Paket:** WF2-31 · **Einstufung:** 🔒 S3 · Sonnet 4.6 (+Opus-Review,
> Isolations-/Validierungs-Pfad) · **Grundlage:** ADR 0005/0006, NFR-SEC-003;
> baut auf WF2-10 (Repos) + WF2-13 (`RequireRole`).

## Reihenfolge-Entscheidung (Projektverantwortlicher)

Stufe 3 startet **mit der Admin-API statt mit dem Config-Service (WF2-30)**:
sichtbarer Business-Value und testbare Endpunkte haben Vorrang vor vorzeitiger
Infrastruktur-Optimierung. Die REST-Endpunkte gehen **direkt auf die Repos**; die
**Caching-Schicht (WF2-30) wird später** eingezogen, wenn die Metriken den Bedarf
zeigen. WF2-31s Abhängigkeit „WF2-30" entfällt damit.

## Warum (fachlich)

Mandanten-Konfiguration (Sicht-Ausschnitt, Abos) wurde bisher per DB-Poking /
CLI gesetzt. WF2-31 gibt einem **Admin** eine echte API, um die **eigene**
Konfiguration zu lesen und zu ändern — die Grundlage für die Admin-UI (WF2-32) und
das Live-Apply (WF2-33). Headline-Nutzen: ein Admin stellt den **Sicht-Ausschnitt
seines Sektors** (Zentrum/Zoom/AOI/FL) selbst ein.

## Was (technisch)

Neues Paket **`pkg/adminapi`** — `Handler` (interner `http.ServeMux` mit
Methode+Pfad-Mustern, automatische `405`):

| Methode + Pfad | Rolle | Wirkung |
|---|---|---|
| `GET /api/admin/view` | admin | Effektive Sicht des eigenen Mandanten (`view_configs.GetEffective`), `404` wenn keine. |
| `PUT /api/admin/view` | admin | Tenant-Default-Sicht upserten (`UpsertTenantDefault`), **server-validiert**. |
| `GET /api/admin/subscriptions` | admin | Eigene abonnierte Feeds (`ListFeedsByTenant`). |
| `GET /api/admin/feeds` | admin | Feed-Katalog (read-only). |

**Isolation per Konstruktion (der Kern):** jeder Handler nimmt die `tenant_id`
**aus der Identity** (`tenant.FromContext`, von der Middleware gesetzt) — **nie**
aus Pfad oder Body. Ein Admin kann damit ausschließlich die **eigene**
Mandanten-Config berühren (NFR-SEC-003). Ohne Identity → `401` (fail-closed).

**Server-Validierung** (`validateView`): `center_lat ∈ [-90,90]`,
`center_lon ∈ [-180,180]`, `zoom ∈ [0,24]`; AOI im Bereich und `min ≤ max`;
`fl_min/fl_max ≥ 0` und `fl_min ≤ fl_max`. Ungültiges → `400`, erreicht den Store
**nicht**.

**DTOs** statt roher Store-Structs: `viewDTO` (center/zoom/aoi/fl/layers), `feedDTO`
(id/name/region/sensor_mix — Infra-Felder wie multicast_group/port bewusst **nicht**
im Admin-Surface).

**Verdrahtung** (`cmd/wayfinder/main.go`): `mux.Handle("/api/admin/",
tenantMW(requireAdmin(adminapi.New(viewRepo, subRepo, feedRepo, logger))))` —
derselbe `RequireRole(admin)`-Gate wie `/admin` (WF2-13), nur
bei aktiver Multi-Tenancy. Kleine Interfaces (`ViewStore`/`SubscriptionStore`/
`FeedStore`) machen die Handler fake-bar.

**Kein Schema-Change, keine neue Abhängigkeit.**

## Tests

- **DB-frei** (`pkg/adminapi/adminapi_test.go`): **Tenant-Scoping** —
  `TestPutViewIsTenantScoped`/`…SubscriptionsIsTenantScoped` beweisen, dass der
  Store mit der `tenant_id` **aus der Identity** (7) aufgerufen wird, nicht aus dem
  Body; `validateView`-Tabelle (Lat/Lon/Zoom/AOI invertiert/Bereich/FL invertiert/
  kaputtes JSON → `400`, erreicht den Store nicht); `401` ohne Identity; `404` ohne
  View; `405` bei falscher Methode; `GET feeds`.
- **Real gegen PostgreSQL 16** (`adminapi_integration_test.go`): `PUT view` →
  `GET view` round-trippt AOI + FL-Band; `GET subscriptions` zeigt den gegrantenen
  Feed; `GET feeds` den Katalog.

Gates grün (`go build/vet/test`, `gofmt`, `scripts/pg-test.sh`); `go 1.25`
unverändert. Doku: INSTALLATION §7 + TECHNICAL §6 (Admin-API); Register
FR-ADMIN-001.

## Abgrenzung / Nächstes

- **Subscription-Writes** (Feed-Grant/-Revoke) bewusst **nicht** hier: das ist
  eine **Billing-/admin-Entscheidung** und cross-tenant (Ziel-Mandant aus
  Pfad statt Identity) — eigener Schritt mit eigenem Rollen-/Billing-Design.
- **Caching (WF2-30)** kommt später (Reihenfolge-Entscheidung oben).
- **Live-Apply:** ein `PUT view` wirkt auf **neue** Connects (der Scope wird am
  Handshake aufgelöst, WF2-21). Bestehende WS-Verbindungen werden erst durch
  **WF2-33** re-skopiert.
- **Nächster Schritt:** **WF2-32 — Admin-UI** (`/admin`, Vue 3 + Vuetify:
  Formulare/Slider für die View-Config auf diesem API) **oder** der
  Subscription-Write-/admin-Pfad — nach Abstimmung.
