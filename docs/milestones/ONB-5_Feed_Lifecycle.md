# ONB-5 — Feed-Lebenszyklus + Live-Receiver-Join/-Leave

> Teil des Zero-Touch-Onboarding-Epics (ADR 0011). Dieses Paket erlaubt dem
> Betreiber, Daten-Feeds (Firefly-Sender) **anzulegen und zu löschen** — ohne
> Terminal **und ohne Neustart**. Der letzte Betriebsschritt, der bisher einen
> Server-Neustart erzwang, entfällt.
>
> **Lieferung in zwei Commits:** (1) Backend (Feed-Manager, Store, API, Wiring),
> (2) Frontend (Feed-Verwaltungs-UI + Store-Actions). Beide sind umgesetzt.

## Fachlicher Hintergrund

Bisher war die Feed-Menge **statisch**: Sie wurde beim Start aus dem DB-Katalog
geladen und eingefroren. Wollte der Betreiber eine neue Datenquelle anbinden
(z. B. einen zweiten Firefly-Sender mit eigener Multicast-Gruppe) oder einen
defekten Feed dauerhaft abschalten, brauchte er einen Server-Neustart. Das
widerspricht dem Zero-Touch-Ziel „nur die Container starten, alles andere aus der
Oberfläche".

ONB-5 macht die Feed-Menge **live veränderbar**: Beim Anlegen tritt der Server
der Multicast-Gruppe **sofort** bei, beim Löschen verlässt er sie **sofort** —
ohne einen einzigen Datagramm-Verlust für laufende Clients der übrigen Feeds.

## Was umgesetzt wurde (Backend)

### 1. Feed-Manager (`pkg/feedmanager`)

Ein Supervisor, der je Feed-ID **eine laufende Receiver-Goroutine** hält. Kern:
eine **mutex-geschützte Map** `feedID → cancel`. API:

- `Start(Feed)` — baut den Receiver (injizierte `Factory`), `Listen()` (Join),
  startet die `Run`-Goroutine. **Idempotent** (laufender Feed → No-op). Ein Bau-
  oder Join-Fehler registriert **nichts** und wird zurückgegeben.
- `Stop(id)` — `cancel()` + **wartet** auf das saubere Verlassen der Gruppe
  (Socket freigegeben), meldet, ob ein Receiver lief.
- `StopAll()` — Server-Shutdown: alle stoppen und abwarten.
- `Running()` / `IsRunning(id)` — lebende Feed-Menge (für Monitor/Tests).

Ein Receiver, der **von selbst** mit einem echten Fehler endet (Socket-Fehler,
nicht `context.Canceled`), wird vom Manager aus der Map **vergessen** (geloggt),
sodass ein späteres `Start` denselben Feed neu versuchen kann. **Ein einzelner
Feed-Fehler beendet nicht mehr den ganzen Server** (früher: jeder Receiver-Fehler
rief `cancel()` global).

Die `Factory` ist **injiziert** → der Manager ist **UDP-frei unit-testbar** (Fake-
Receiver), inklusive `-race`.

### 2. Prompte Beendigung im Receiver (`pkg/receiver`)

Ein blockiertes `ReadFromUDP` beobachtet eine Kontext-Cancellation **nicht** von
selbst — bei einem *toten* Feed (kein Datagramm) bliebe `Run` hängen und `Stop`
würde nicht zurückkehren. Lösung: ein **Watchdog** setzt bei `ctx.Done()` eine
**vergangene Read-Deadline**; das laufende `ReadFromUDP` kehrt sofort mit
`os.ErrDeadlineExceeded` zurück, die Schleife sieht `ctx.Err()` und endet
**sauber**. So wird die IGMP-Gruppenmitgliedschaft **prompt** freigegeben — auch
bei totem Feed. (Idiomatischer Weg, einen blockierten UDP-Read zu unterbrechen.)

### 3. Store — `FeedRepo.Delete`, `GetByName`, Migration 00008

- `Delete(id)` — `DELETE FROM feeds WHERE id=$1`; Abos, die den Feed
  referenzieren, fallen via `ON DELETE CASCADE` (Migration 00001) automatisch
  weg → **atomar** ohne Transaktion. Fehlende Zeile → `ErrNotFound`.
- `GetByName(name)` — Dup-Pre-Check für ein sauberes 409.
- Migration `00008_feeds_name_unique.sql` — `feeds.name` wird **UNIQUE** (der
  menschliche Schlüssel, den der Betreiber im Dashboard auswählt; spiegelt die
  Slug-Eindeutigkeit der Mandanten). Additiv, nicht-destruktiv.

### 4. API — `POST`/`DELETE /api/admin/feeds`

Beide hinter `requireAdmin`.

- **`POST /api/admin/feeds`** `{name, multicast_group, port, region?, sensor_mix?}`
  → 201. Validierung: `multicast_group` muss **IPv4-Multicast** sein
  (224.0.0.0–239.255.255.255), `port` 1..65535, `sensor_mix` gegen das
  Sensorklassen-Vokabular (unbekannt → 400); doppelter Name → **409**.
  **Atomar über Katalog + Live-Join:** erst Katalogzeile, dann Receiver-Start;
  **scheitert der Beitritt** (z. B. Gruppe/Port belegt), wird die Katalogzeile
  **zurückgerollt** — nie ein katalogisierter, aber stummer Feed.
- **`DELETE /api/admin/feeds/{id}`** → 204. Erst **Receiver stoppen** (Gruppe
  verlassen), dann Zeile löschen (Abos kaskadieren). **Guard C** (Betreiber-
  Entscheidung): **kein** Blockieren bei bestehenden Abos — die Grants
  kaskadieren weg; die Zahl wird nur für die Auditierbarkeit geloggt.

### 5. Verdrahtung (`cmd/wayfinder`)

- `newReceiverFactory(...)` ersetzt `buildReceivers`: liefert eine
  `feedmanager.Factory`, die je Feed einen Receiver mit den geteilten Handlern
  baut und die `feed_id` stempelt. Genutzt **beim Boot** (Katalog) **und** für
  live angelegte Feeds.
- `feedLifecycle`-Adapter setzt `adminapi.FeedLifecycle` auf Feed-Manager +
  Health-Registry um (`Start` → Join; `Stop` → Leave + `registry.Forget`).
- `main.go`: der statische „buildReceivers + Listen-Schleife"-Block ist durch
  den Feed-Manager ersetzt; der **Staleness-Monitor** iteriert die **lebende**
  Menge (`feedManager.Running()`) statt einer eingefrorenen Slice; ein
  **prozessweiter, churn-stabiler Decode-Error-Zähler** (Hook `OnDecodeError`)
  hält die `/metrics`-Zahl monoton, obwohl Receiver kommen und gehen.

### 6. Entkopplung

`adminapi.FeedLifecycle` nimmt **primitive Parameter** (`id, name, group, port`)
— die Admin-API importiert die Transport-/Manager-Schicht **nicht**. Der konkrete
Adapter lebt in `main.go`. Eine `nil`-Lifecycle deaktiviert das Live-Apply
(Single-Tenant / Tests): der Katalog ändert sich, die Receiver-Menge folgt erst
beim Neustart.

## Was umgesetzt wurde (Frontend)

- **Store-Actions** (`admin.js`): `createFeed(payload)` (POST, Erfolgs-Banner;
  409-Duplikatname → verständliche deutsche Meldung), `deleteFeed(id)` (DELETE).
- **`AdminFeeds.vue`** (neue Komponente, Vorlage `AdminPlatformAdmins.vue`):
  Feed-Katalog als Tabelle (Name, `multicast_group:port`, Sensor-Mix-Chips,
  **Gesundheits-Chip** aus `feedsHealth` — grün/gelb/rot wie im Dashboard);
  „Feed anlegen"-Dialog (Name, Multicast-Gruppe, Port, optionaler Sensor-Mix)
  + Löschen-Bestätigung (mit Hinweis, dass Abos kaskadieren). Lädt nach jeder
  Mutation neu (`loadFeeds` + `loadFeedsHealth`).
- **Navigation** (`AdminView.vue`): dritter Eintrag **„Feeds"** im Header-Toggle
  (Mandanten ↔ Feeds ↔ Plattform-Administratoren), im Pflichtwechsel-Zustand
  ausgeblendet.

## Byte-/Verhaltens-Vertrag

- `POST /api/admin/feeds`: 201 + `{id, name, multicast_group, port, region?,
  sensor_mix}`; 400 bei ungültiger Multicast-Adresse/Port/Sensor-Mix; 409 bei
  doppeltem Namen; 500 (mit Rollback) wenn der Live-Join scheitert.
- `DELETE /api/admin/feeds/{id}`: 204; 404 bei unbekanntem Feed.

## Qualitäts-Gates (Backend-Commit)

- `go test -p 1 ./...` gegen real-PG (`scripts/pg-test.sh`) ✅, `go vet`/`gofmt` ✅,
  `pkg/feedmanager` zusätzlich unter `-race` ✅.
- Tests:
  - `pkg/feedmanager/feedmanager_test.go`: Start/Stop (sauberes Leave),
    Idempotenz, Listen-/Factory-Fehler, Self-Error-Forget + Retry, `StopAll`,
    Base-Ctx-Cancel.
  - `pkg/store/store_integration_test.go`: `TestIntegrationFeedDeleteCascades`
    (Abo-Cascade, Tenant überlebt, Re-Delete → `ErrNotFound`, Namen-Reuse),
    `TestIntegrationFeedNameUnique` (UNIQUE-Constraint).
  - `pkg/adminapi/adminapi_feeds_test.go`: `TestCreateFeed`, `…Validation`
    (8 Fälle), `…DuplicateName`, `…InvalidSensorMix`, `…RollsBackOnJoinFailure`,
    `TestDeleteFeedSucceeds`, `…UnknownIs404`, `TestFeedLifecycleRoutesForbidNonAdmin`,
    `…WithoutLifecycle`.
  - `cmd/wayfinder/feeds_test.go::TestNewReceiverFactory`.

**Frontend-Commit:**
- `npm run test` (157 Tests) ✅, `npm run build` aktualisiert `dist/` ✅.
- Tests: `frontend/src/stores/__tests__/admin.test.js` (ONB-5-Block:
  `createFeed` 201/409/400, `deleteFeed` 204).

## Sicherheits-Bewertung (CLAUDE.md §7)

- **Robuster Eingang unberührt:** Der Decoder verwirft fehlerhafte Datagramme
  weiterhin (kein Panic); der neue Watchdog ändert nur die **Beendigung**, nicht
  die Auswertung.
- **Feed-Authentizität:** Multicast bleibt unauthentifiziert — der neue Pfad legt
  nur fest, **welche** Gruppen der Server abonniert; die Vertrauensgrenze
  (Netz-Isolation) ist unverändert (ADR 0003). Die Feed-Anlage ist
  **admin-only** (`requireAdmin`).
- **Validierung am Rand:** Nur IPv4-Multicast-Adressen im gültigen Bereich werden
  katalogisiert — der Katalog kann nie einen Feed halten, den der Receiver nicht
  binden kann.

## Bekannte Grenze

Der Live-Join/-Leave wirkt im **lokalen** Prozess. In einem Mehr-Replica-Deployment
würde jede Replica der Gruppe eines neu angelegten Feeds erst bei ihrem nächsten
`Start` beitreten; ein replica-übergreifendes Katalog-Watch ist nicht Teil von
ONB-5 (offener Betriebs-Härtungs-Punkt).

## Rückverfolgbarkeit

- **Anforderung:** FR-ADMIN-008 (`docs/requirements/README.md`).
- **Design:** ADR 0011 (`docs/decisions/0011-zero-touch-onboarding.md`), Guard C.
- **Vorgänger:** ONB-4 (`ONB-4_Tenant_Lifecycle.md`).
- **Folgepaket:** ONB-6 (OpenAIP pro Mandant).
