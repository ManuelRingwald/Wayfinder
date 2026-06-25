# AP4 — Feed-Gesundheit pro Feed im Admin-Dashboard

**Paket:** ADR 0009 · **Stufe:** S3–S4 · **Modell:** Opus 4.8
**Issue:** #67 · **Abgeschlossen:** 2026-06-25 · baut auf AP3 auf

---

## Fachliche Motivation

Im Dashboard (AP3) sieht der Plattform-Betreiber *welche* Feeds ein Mandant
abonniert — aber nicht, ob diese Feeds **gesund** sind. Im Betrieb ist das die
erste Diagnose-Frage bei einer Störungsmeldung: „Kunde sieht keine Flugzeuge" —
liegt es am **leeren Himmel** (Feed lebt, gerade kein Verkehr) oder am **toten
Feed** (Firefly-Sender aus, Netz-Pfad gestört, kein Heartbeat)? Bisher musste man
dafür in `/metrics` schauen, und auch dort nur **global** (ein einziges
`wayfinder_feed_stale`), nicht pro Feed.

AP4 macht die Feed-Gesundheit **pro Feed** im Dashboard sichtbar — als farbiger
Ampel-Chip an jedem Feed, in Übersicht und Detailseite:

- **grün** — Heartbeat kommt an **und** Tracks fließen (gesund),
- **gelb** — Heartbeat kommt an, aber kein Verkehr (**leerer Himmel**),
- **rot** — kein Heartbeat (**toter Feed**) oder nie gesehen.

---

## Technische Umsetzung

### Backend

1. **`pkg/health` — Per-Feed-Registry.** Die bisherige `FeedHealth` trackte
   Heartbeat-Staleness **global** (eine Instanz für alle Feeds). Neu:
   - `FeedHealth.LastHeartbeat()` — Accessor für Zeit + „je gesehen", damit die
     Registry `last_heartbeat_ago` berechnen kann.
   - `Registry` (neu, `registry.go`): hält pro `feedID` eine `FeedHealth` **und**
     die zuletzt empfangene CAT062-Block-Größe (Proxy für „kommen gerade
     Tracks?"). Lazy-Registrierung beim ersten Heartbeat/Track. Liefert
     `Snapshot(feedID, now) → FeedSnapshot{EverSeen, Stale, LastHeartbeatAgoS,
     TrackCountRecent}` mit `Color()`-Ableitung. Zusätzlich aggregiert sie alle
     Feeds in eine globale `FeedHealth` und exponiert `Status`/`Observe` als
     **Drop-in-Ersatz** für die bisherige Single-Instanz (Readiness-Probe und
     Browser-Feed-Banner bleiben unverändert).

2. **`cmd/wayfinder`** — `feedHealth *health.FeedHealth` → `feedRegistry
   *health.Registry`. Der `trackHandler` meldet pro Block `RecordTracks(feedID,
   len)`, der `statusHandler` bekommt über eine Per-Feed-Closure in
   `buildReceivers` jetzt die `feedID` mit (`func(int64,
   cat065.ServiceStatus)`) und ruft `RecordHeartbeat(feedID, now)`. Readiness
   (`/ready`) und `/metrics` lesen `feedRegistry.Status(...)` — Verhalten
   identisch.

3. **`pkg/adminapi`** — neues `FeedHealthSource`-Interface (`Snapshot`),
   befüllt von `*health.Registry`. Neuer Endpunkt **`GET
   /api/admin/feeds/health`** (admin-only, `requireAdmin`): listet je Feed des
   globalen Katalogs `{feed_id, color, stale, ever_seen, last_heartbeat_ago_s,
   track_count_recent}`. Eine **nil**-Quelle (z. B. Tests, die nur DB-Pfade
   prüfen) liefert eine leere Liste statt zu krachen.

### Frontend

- **Store** (`admin.js`): `feedsHealth` (Map `feedId → DTO`) +
  `loadFeedsHealth()`. Bei Fehler bleibt der letzte Stand stehen (kein
  Flackern auf rot bei einem transienten 503).
- **`AdminTenants.vue`** / **`AdminTenantDetail.vue`**: die Feed-Chips bekommen
  die Ampel-Farbe (`success`/`warning`/`error`) + Tooltip; die Übersicht lädt
  `loadOverview` + `loadFeedsHealth` parallel.

**Schnittstellen-Wirkung:** rein additiv, **kein** CAT062/ICD-Bezug, **kein**
Schema-Change, **keine** neue Env-Variable (`WAYFINDER_FEED_STALE_TIMEOUT` gilt
weiter). Die Ampel ist eine **In-Memory-Sicht** des Empfangsprozesses — sie ist
nicht über Replikas hinweg aggregiert (eine Wayfinder-Instanz kennt nur ihre
eigenen Empfänger; Konsistenz mit dem Single-Process-Empfangsmodell).

---

## Tests

### Go
- `pkg/health/registry_test.go` (9): unbekannter Feed → rot; Heartbeat+Tracks →
  grün; Heartbeat ohne Tracks → gelb; stale → rot; `last_heartbeat_ago`
  (positiv / negativ wenn nie gesehen); Feed-Isolation; Aggregat-`Status`/
  `Observe`.
- `pkg/adminapi/adminapi_test.go` AP4-Block (4): nil-Quelle → leere Liste;
  Farben + Felder (grün/gelb); stale → rot; `user` → 403. Cross-Tenant-Test um
  `/feeds/health` erweitert.

### Vitest
- `admin.test.js` AP4-Block (4): `loadFeedsHealth` keyt nach `feed_id`; stale →
  rot; leere Liste leert die Map; Fehler lässt vorhandene Daten stehen.

---

## Qualitäts-Gates

- `go test ./...` ✅ · `go vet`/`gofmt` ✅
- `vitest run` ✅ (132 Tests) · `npm run build` ✅
- Doku: Register **FR-OPS-004** (AP4-Erweiterung), TECHNICAL §3 (Endpunkt),
  INSTALLATION (Schritt 5.7 Ampel-Erklärung), dieser Milestone.
