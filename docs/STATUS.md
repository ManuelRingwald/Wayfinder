# Arbeitsstand (Handover-Notiz)

> **Zweck:** Diese Datei ist der schnelle Wiedereinstieg — egal ob am PC oder
> Handy. Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

> 🗺️ **Roadmap:** Arbeitspakete, Findings und empfohlene Reihenfolge stehen in
> `docs/ROADMAP.md` (Stichwort „Roadmap" im Chat zeigt diese Liste).

- **Zuletzt aktualisiert:** 2026-06-19 — **WF2-10.1 „Persistenz-Schicht & Migrationen"
  abgeschlossen (S3 · Sonnet 4.6 / Opus-Review) — ERSTES PRODUKTIVCODE-PAKET.**
  Neues `pkg/store`: `store.go` (`Open` pgxpool + Ping, `DSNFromEnv` aus
  `WAYFINDER_DB_URL`), `migrate.go` (minimaler **In-House-Migrationsrunner**:
  eingebettete `migrations/*.sql`, `-- +migrate up/down`-Marker, `schema_migrations`-
  Tracking, je Migration eine Transaktion, idempotent, forward-only),
  `migrations/00001_init.sql` (ADR-0006-Schema: tenants/users/feeds/subscriptions/
  view_configs/entitlements). Tests `store_test.go` DB-frei (kein Docker-Daemon
  hier; Schema-Apply folgt WF2-10.3 in CI). **Zwei bewusste Entscheidungen
  (ADR 0006 Nachtrag):** (1) **goose verworfen** — zog transitiv
  `modernc.org/sqlite` (volle SQLite-Engine) in einen Postgres-only-Dienst; (2)
  **Go-Baseline 1.23 → 1.25** (pgx + modernes `golang.org/x/*` verlangen es;
  `go.mod` + Dockerfile `golang:1.25-bookworm` gebumpt). Abhängigkeit:
  `github.com/jackc/pgx/v5` (sonst lean, kein Migrations-Framework). Register
  FR-TEN-002 (Implementierung/Tests aktualisiert), Milestone
  `docs/milestones/WF2-10.1_Persistence_Layer.md`. Gates grün (`go build/vet/test`,
  `gofmt`). `WAYFINDER_DB_URL` noch nicht von `main` gelesen (Library) → kein
  INSTALLATION-Eintrag nötig. **Nächster Schritt:** WF2-10.2 (Repositories
  tenants/users) nach Ankündigung & „Go".
- **Vorherige Aktualisierung:** 2026-06-19 — **WF2-02 / ADR 0007 „Cloud-Ingest &
  Feed-Fan-out" abgeschlossen — STUFE 0 KOMPLETT (S4 · Opus 4.8, Doku).** Neue
  ADR `docs/decisions/0007-cloud-ingest-und-feed-fan-out.md`. Zielumgebung vom
  Projektverantwortlichen gesetzt: **Public Cloud + Kubernetes**. Entscheidungen:
  (1) **`FeedSource`-Abstraktion** — `MulticastFeedSource` (On-Prem/Dev) vs.
  `StreamFeedSource` (Cloud), via `WAYFINDER_FEED_SOURCE`; (2) **Ingest-Gateway**
  (`cmd/wayfinder-ingest`) als eigener Minimal-Dienst: tritt Multicast-Gruppe(n)
  bei, republisht **Roh-Datagramme** auf **Subject pro Feed** (kein Decode im
  Gateway); (3) **Stream-Bus = NATS JetStream** — Core-Subject-Fan-out („jede
  Instanz sieht alles"), JetStream nur als Late-Join-Puffer, Replay bleibt
  Firefly (SDPS-005). **RabbitMQ vs Kafka geprüft** (auf Wunsch): für dieses
  Profil RabbitMQ > Kafka, beide < NATS → verworfen; RabbitMQ bleibt AMQP-
  Fallback. Bus trägt Roh-ASTERIX (einziger Decode-Punkt erhalten). Register
  **FR-FEED-001** + **NFR-SCALE-001**. ROADMAP §0/§1/§6 + STATUS §1/§2/§3
  fortgeschrieben. `go build/vet/test` grün. Reine Doku. **Damit ADR 0005/0006/
  0007 = Stufe 0 abgeschlossen.** **Nächster Schritt:** WF2-10 (Persistenz-
  Schicht, **erstes Produktivcode-Paket**, S3 · Sonnet 4.6 +Opus-Review) nach
  Ankündigung & „Go".
- **Vorherige Aktualisierung:** 2026-06-19 — **WF2-01 / ADR 0006 „Konfig-/Identitäts-
  Persistenz" abgeschlossen (S4 · Opus 4.8, Doku).** Zweiter Baustein von
  Wayfinder 2.0 (Stufe 0). Neue ADR `docs/decisions/0006-konfig-identitaets-persistenz.md`:
  (1) Datastore = **PostgreSQL**; (2) Zugriff = **`pgx` + `sqlc`** (typsicher,
  kein ORM, auditierbar); (3) Migrationen = **`goose`** (eingebettet, getaggte
  Baselines); (4) **Schema-Skizze** (tenants/users/feeds/subscriptions/
  view_configs/entitlements; feeds = globaler Katalog, sensor_mix als
  Feed-Eigenschaft); (5) **Identität = OIDC@Proxy primär** (Wayfinder validiert
  Token, mappt subject→tenant) **+ eingebauter Fallback** (argon2id) **+ none**
  (Single-Tenant), via `WAYFINDER_AUTH_MODE`; Tenant-Kontext **fail-closed**
  (Muster aus ADR 0003); (6) **Stateless-Split** (State in DB, Infra/Secrets in
  ENV); (7) **Redis zurückgestellt** (In-Proc-TTL zuerst). Register **FR-TEN-002**
  (Persistenz/Schema) + **NFR-SEC-004** (Identität/Session), je mit Vorwärts-
  Referenz auf WF2-10/11/12. Neue ENV-Variablen (`WAYFINDER_DB_URL`,
  `WAYFINDER_OIDC_*`, `WAYFINDER_SESSION_KEY`, `WAYFINDER_AUTH_MODE`) kommen in
  INSTALLATION/TECHNICAL, **sobald WF2-10/11 sie einlesen** (heute noch
  wirkungslos). ROADMAP §0/§1/§6 + STATUS §1/§2/§3 fortgeschrieben (WF2-01 ✅,
  nächster = WF2-02). `go build/vet/test` grün (keine Code-Änderung). Reine Doku.
  **Nächster Schritt:** WF2-02 / ADR 0007 „Cloud-Ingest & Feed-Fan-out"
  (S4 · Opus 4.8) nach „Go".
- **Vorherige Aktualisierung:** 2026-06-19 — **WF2-00 / ADR 0005 „Multi-Mandanten-Pivot"
  abgeschlossen (S4 · Opus 4.8, Doku).** Erster Baustein von Wayfinder 2.0.
  Neue ADR `docs/decisions/0005-multi-mandanten-pivot.md`: (1) Pivot zur
  mandantenfähigen Plattform ratifiziert, ASD-Kern bleibt als mandanten-skopierte
  Sicht; (2) **Mandanten-Modell = Hybrid** (Feed-Katalog + Abos + Sicht-Filter)
  mit konzeptuellem Datenmodell (Tenant/User/Feed/Subscription/ViewConfig/
  Entitlement); (3) **Isolationsgrenze** als sicherheitskritischer Kern: server-
  seitige AuthZ pro Subscription, Broadcaster all-to-all → prädikat-gefiltert,
  fail-closed, **Pflicht-Negativtests** (A sieht nie B); nimmt die in ADR 0003
  vertagte „Autorisierungs-ADR" auf; (4) Kommerz-Scope (Feature-Flags ja, Billing
  zurückgestellt); (5) Zert-Haltung (Isolation in FHA #7); (6) 12-Factor-Grenze
  (Infra-Secrets ENV, fachliche Config DB); (7) Single-Tenant als degenerierter
  Fall (schrittweise Migration); (8) Abgrenzung zu ADR 0006/0007. Register:
  **FR-TEN-001** (Mandantenfähigkeit/Hybrid) + **NFR-SEC-003** (Cross-Tenant-
  Isolation), beide mit Vorwärts-Referenz auf WF2-1x/2x. ROADMAP §0/§1/§6 +
  STATUS §1/§2/§3 fortgeschrieben (WF2-00 ✅, nächster = WF2-01). `go test ./...`
  grün (keine Code-Änderung). Reine Doku, kein Produktivcode, keine ICD-Änderung.
  **Nächster Schritt:** WF2-01 / ADR 0006 „Konfig-/Identitäts-Persistenz"
  (S4 · Opus 4.8) nach „Go".
- **Vorherige Aktualisierung:** 2026-06-19 — **Paket 6 Coverage-Werkzeug (Radar-Ringe) abgeschlossen (S3 · Sonnet 4.6).**
  Neues Go-Paket `pkg/coverage`: `ParseEnv()` liest `WAYFINDER_COVERAGE_SENSOR_N_*`
  (max. 20 Sensoren); `RingsGeoJSON()` erzeugt GeoJSON-FeatureCollection mit äußerem
  Ring (outer), innerem Ring (inner, nur bei MinRangeM > 0) und Mittelpunkt-Dot
  (center). Kreisapproximation: 128 Punkte, Flat-Earth (< 1 % Fehler bei ≤ 250 km).
  Neuer Endpoint `/api/coverage/rings` (statisch, einmal berechnet, `application/geo+json`).
  `WAYFINDER_COVERAGE_RING_COLOR` (Default `#5B8DEF`) — einheitliche Farbe für alle Sensoren.
  Frontend: neues `COVERAGE_*` Quell-/Layer-ID-Paar in `constants.js`; `addCoverageLayer()`
  + `updateCoverageSource()` in `layers.js`; Engine lädt Layer und fetched Ringe beim
  Map-Load; `setLayerVisibility` kennt `coverageRings`; ASD-Store `layerVisibility.coverageRings: true`;
  Toggle-Schalter „Radarabdeckung" im Layer-Panel. 6 Tests in `pkg/coverage/coverage_test.go`.
  INSTALLATION.md §7.5 + TECHNICAL.md §6.5 ergänzt. Firefly-Seite: `SensorModel` erhält
  `min_range_m`/`max_range_m` (serde-kompatibel, rein informational); `with_sensor_coverage()`
  chainbar; Frankfurt + Demo-Scene setzen Reichweiten. `cargo test --workspace` grün.
  Paket 6a (Firefly-UI-Aufräumen) als separates TODO in Roadmap vermerkt.
  Nächster Schritt: Paket 7 (FHA/Hazard-Analyse) oder Paket 6a nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-19 — **Roadmap zentral auf Wayfinder 2.0
  ausgerichtet; Widersprüche aufgelöst (S2–S3 · Sonnet/Opus, Doku).** Aus dem
  Entwurf „Wayfinder 2.0" wurde ein ausführliches Konzept entwickelt
  (`docs/design/wayfinder-2.0-konzept.md`, **auf `main` via PR #25**): 6
  Ausbaustufen (0–5), ~28 Arbeitspakete (`WF2-xx`), Schwierigkeitsgrad→Modell-
  Tabelle, zwei ratifizierte Leitentscheidungen — **Mandanten-Modell = Hybrid**
  und **Kommerz-Scope = Feature-Flags ja, Stripe-Billing zurückgestellt**.
  Danach **`docs/ROADMAP.md` komplett neu strukturiert** als zentrale, auf 2.0
  ausgerichtete Quelle: §0 Strategie, §1 WF2-Backlog (Stufen 0–5), §2 ASD-Kern
  (ASD-011/012/013 als **mandanten-unabhängige Parallel-Spur** mit 2.0-Abgleich),
  §3 Firefly-Backlog mit 2.0-Bezug, §4 Begründung, §6 Erledigt. **Kollision
  aufgelöst:** bisher zeigte STATUS auf „ASD-011 zuerst", das Konzept auf „ADR
  0005" — neuer gemeinsamer nächster Schritt = **WF2-00 / ADR 0005**.
  Cross-Project-Abhängigkeiten in `docs/cross-project/todo-for-firefly.md`
  vermerkt. Reine Doku, kein Produktivcode, keine ICD-Änderung. Gates n/a
  (Markdown). Nächster Schritt: **WF2-00 — ADR 0005 „Multi-Mandanten-Pivot"**
  (S4 · Opus 4.8) nach „Go".
- **Vorherige Aktualisierung:** 2026-06-18 — **AP9.9 „ADS-B-Badge im Track-Label"
  abgeschlossen (S3 · Opus 4.8).** Wayfinder-Seite von AP9 (ADS-B-Integration).
  **Decoder:** `pkg/cat062/types.go` um `UpdateAge.ESAge *float64` erweitert
  (nil = rein Radar, Pointer = ADS-B-Anteil vorhanden). `pkg/cat062/decoder.go`
  Fall 14 (I062/290) durch bit-walking Loop ersetzt: Bits 7→1 MSB-first,
  je gesetztes Bit ein Age-Byte (LSB = 1/4 s); PSR=0x40, ES=0x08 — tolerant
  gegenüber zukünftigen Subfeldern. **Broadcaster:** `TrackMessage.AdsbAgeS
  *float64` (`json:"adsb_age_s,omitempty"`) hinzugefügt; `tracksToMessage` mapt
  `UpdateAge.ESAge`. **Frontend (`app.js`):** `ADSB_FRESH_THRESHOLD_S = 30`,
  `ADSB_BADGE = "◆"`, `isAdsbFresh(adsbAgeS)` Helper;
  `buildLabel` zeigt `◆` im Label-Ident wenn `isAdsbFresh` (age ≤ 30 s).
  **Tests:** `TestDecodeAdsbAge` + `TestDecodeNoAdsbAge` (byte-exakt,
  Mirror von Fireflys `single_track_with_adsb_hit_matches_reference_dump`,
  ICD 2.4.0); `TestTracksToMessageMapsAdsbAge` in Broadcast-Tests.
  **Anforderungen:** FR-DATA-005 (ES-Age Decoder), FR-ASD-006 (ADS-B Badge)
  im Register. Gates grün (`go test ./...` ✅, `go vet ./...` ✅, `gofmt` ✅,
  `node --check app.js` ✅). AP9 (ADS-B, ICD 2.4.0) auf Wayfinder-Seite
  damit vollständig abgeschlossen. Nächster Schritt: nächstes Roadmap-Paket
  nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-17 — **Phase 1 der ASD-Optik-Verbesserung
  (ASD-007–010) abgeschlossen.** Branch `claude/vue-md3-asd-006`.

  **ASD-007 Farbschema:** Cyan-Primary-Theme aus ASD-Mockup (Command-Center-
  Ästhetik). `vuetify.js`: background `#070b12`, surface `#0e1622`, primary
  `#23d3e6`. `constants.js`: neues `TRACK_COLORS`-Objekt (friendlyCivil
  `#41c4e8`, hostile `#ff4338`, unknown `#ffd23e`, neutral `#43c66b`, friendlyMilitary
  `#ffa726`); PALETTES.dark aktualisiert (label, vector, trail, airspaceFillColor,
  airways). Design-Spec in `docs/design/color-tokens.md`.

  **ASD-008 Navigation Rail:** `NavigationRail.vue` ersetzt die monolithische
  `LayerSidebar.vue`. Permanent-schmale Schiene (56 px Icons + Tooltips) auf
  Desktop; Klick → 240-px-Panel für Layer-/FL-Filter-Controls; Collapse-Button;
  Mobile bleibt Hamburger-Temporary-Drawer. sections-Array vorbereitet für
  ASD-013 Alarm-Panel.

  **ASD-009 Karten-Controls:** `MapControls.vue` — zwei schwebende Button-
  Gruppen rechts (Zoom +/−; Recenter, Nord-up, Fullscreen). `engine.js` um
  `zoomIn/zoomOut/recenter/resetNorth` erweitert.

  **ASD-010 Kategorie-Filter-Chips:** `TrackFilterChips.vue` top-center über
  dem Canvas. Live-Zähler (Confirmed/Coasting/Tentative) aus Pinia
  `trackCounts`. Klick togglet `hiddenCategories`; `render.js` filtert alle
  Feature-Typen (Symbole, Vektoren, Dots, Trails) für ausgeblendete Kategorien.

  Gates: `npm run build` ✅ · `vitest 39/39` ✅ · `go test ./...` ✅.
  S2–S3 · Sonnet 4.6.

  **Nächster Schritt:** Phase 2 beginnen — Reihenfolge ASD-011 → ASD-012 →
  ASD-013. ASD-011 (Erweitertes Track-Detail-Panel) ist S2, gut umsetzbar mit
  Sonnet 4.6. Oder: PR #16 erst mergen lassen und dann auf neuem Branch weiter.

- **Vorherige Aktualisierung:** 2026-06-17 — **ASD-006 „Vue 3 + Vuetify 3
  (Material Design 3)" abgeschlossen.** Branch `claude/vue-md3-asd-006`.
  ADR 0002 ratifiziert. AP0–AP6 vollständig umgesetzt (ADR-Doku, Scaffold,
  Karten-Engine als ES-Module, 39 Vitest-Tests, Pinia-Store, App-Shell,
  Track-Detail-Panel). wayfinder.yaml.example + FR-CFG-003 (YAML-Config).
  Gates: npm run build ✅ · vitest 39/39 ✅ · go test ./... ✅.

- **Vorherige Aktualisierung:** 2026-06-16 — **Paket #16 / ASD-002 „Anti-Garbling
  (Label-Deconfliction + Drag&Drop)" abgeschlossen.** Rein Frontend (`app.js`),
  kein Backend- oder ICD-Change. **B1 Auto-Deconfliction:** `deconflictLabels()`
  berechnet in Screen-Space für jeden Track (deterministisch nach `track_num`) die
  optimale Label-Position per greedy 8-Slot-Algorithmus (Slots rechts-priorisiert,
  ATC-konform); Kollision gegen BBoxen bereits platzierter Labels und anderer
  Tracks' Kreis-Footprints geprüft; eigenes Symbol absichtlich ausgeschlossen damit
  Label neben seinem Punkt sitzen kann; Fallback auf Slot 0 — kein Label verschwindet
  je. Labels in neuer `LABELS_SOURCE_ID` (`text-allow-overlap:true`,
  `text-ignore-placement:true`). Leader Lines (`LEADER_LINES_SOURCE_ID`, 0.7 px,
  label-farbig) wenn Abstand > 10 px. Viewport-Nachführung via
  `requestAnimationFrame`-Throttle auf `map.on("move")`. Alle Opacity-Properties
  (`fade_opacity`, `fl_opacity`, `coasting`) aus Track-Features durchgereicht.
  `TRACKS_LABEL_LAYER_ID` aus `addTracksLayer()` entfernt; neue Funktionen:
  `addLeaderLinesLayer`, `addLabelsLayer`, `bboxCollides`, `deconflictLabels`.
  **B2 Drag&Drop-Pinning:** `setupLabelDrag()` — `mousedown` auf Label →
  `map.dragPan.disable()` + Offset in `state.labelPins`; `mousemove` → Live-Update
  + `renderSources()`; `mouseup` → commit; `dblclick` → Pin löschen (Auto-Reset).
  `tickFade()` räumt Pins für abgelaufene Tracks aus. FR-ASD-002 im Register.
  Milestone `docs/milestones/ASD-002_Anti_Garbling.md`. `node --check app.js` ✅,
  `go test ./...` ✅, `go vet ./...` ✅. S4 · Opus 4.8.
  Nächster Schritt: nächstes Roadmap-Paket nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-16 — **Paket #15 / ASD-005 „Höhen- und
  Filter-Tools" abgeschlossen.** Frontend-only (`index.html` + `app.js`). Min/Max-FL
  Number-Inputs + Ausblenden-Checkbox in `#layer-control`. `isFlFiltered(flightLevelFt)`
  prüft ob bekannte FL außerhalb [minFL, maxFL] liegt (unbekannte FL = immer passiert).
  `flOpacity()` liefert 0.0 (hide) / 0.15 (dim) / undefined (nicht gefiltert).
  `setupFlFilter()` verdrahtet die Inputs und ruft bei Änderung sofort `renderSources()`
  auf — Filteränderungen wirken ohne WSS-Update. `flight_level_ft` nun in
  `liveTrackFeatures`-Properties gespeichert. `fl_opacity`-Bedingung (`["has",
  "fl_opacity"] → ["get", "fl_opacity"]`) in allen 5 Layer-Paint-Expressions
  ergänzt (Priorität: fade_opacity > fl_opacity > coasting > normal).
  `filtered: boolean` auf Track-Symbol-Features für circle-color-Expression
  (blau-grau für gefilterte Tracks). Firefly-ROADMAP synchronisiert.
  Anforderung FR-ASD-005 im Register. S2 · Sonnet 4.6.
  Nächster Schritt: nächstes Roadmap-Paket nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-16 — **Paket #14 / ASD-004 „Track-Lebenszyklus
  & History-Darstellung" abgeschlossen.** Rein Frontend (`app.js`), kein Backend-Change.
  **ASD-004a History-Dots:** Neuer Source `track-history-dots` + `circle`-Layer
  `track-history-dots-circles` (Radius 2 px, Trail-Farbe); rendert jeden
  `state.trackHistory`-Eintrag als einzelnen Punkt zwischen Trail-Linie und
  Speed-Vector — klassisches Radar-Instrument (Punktabstand = Geschwindigkeit,
  Krümmung = Drehrate). **ASD-004b Coasting-Abdunkeln:** Alle fünf Track-Layer
  erhalten datengesteuerte Opacity-`case`-Expressions: coasting → circle-opacity
  0.5, text/vector-opacity 0.35, trail/dot-opacity 0.2; `state.trackCoasting:
  Map<track_num, boolean>` führt den Zustand für Trail/Dot-Features mit.
  **ASD-004c Graceful Fade-Out bei TSE:** TSE-Tracks landen in
  `state.fadingTracks: Map<track_num, {deadline, track}>` statt sofort weggefiltert
  zu werden; `renderSources()` mischt sie mit `fade_opacity`-Property (0–1) in alle
  vier GeoJSON-Sources; `tickFade()` läuft per `setInterval` (~50 ms) und räumt
  abgelaufene Tracks + ihre History auf. Paint-Expressions: `["has", "fade_opacity"]`
  hat Vorrang vor Coasting-Dimming. `updateTrackHistory` lässt Fading-Track-
  History stehen. Anforderung FR-ASD-004 im Register. Milestone
  `docs/milestones/ASD-004_Track_Lifecycle_History.md`. Gates grün
  (`go test ./...`, `go vet ./...`). S3 · Sonnet 4.6.
  Nächster Schritt: nächstes Roadmap-Paket nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-16 — **Paket #12 / ASD-001 „Erweiterter
  Data Block" abgeschlossen.** Rein Frontend (`app.js`), kein Backend-Change.
  **ASD-001a Ground Speed:** `buildLabel(track, vTrend)` erhält neue dritte
  Zeile mit Bodengeschwindigkeit in Knoten (`Math.hypot(vx, vy) × 1.9438`,
  gerundet, nur wenn > 0). **ASD-001b Steig-/Sinkflug-Indikator:**
  `state.trackFlHistory: Map` speichert letzte bekannte FL pro Track;
  `updateTracksLayer` berechnet FL-Delta, zeigt `▲` bei > +50 ft oder `▼`
  bei < −50 ft gegenüber dem Vorgänger-Scan (Schwellwert 50 ft = 2 LSB,
  filtert Mode-C-Quantisierungsrauschen). History wird parallel zu
  `trackHistory` bereinigt (Einträge verschwundener Tracks gelöscht).
  Alle vier Data-Block-Elemente gebündelt in `buildLabel`:
  `DLH123 / FL350 ▲ / 247`. Anforderung FR-ASD-001 im Register.
  Meilenstein `docs/milestones/ASD-001_Extended_Data_Block.md`. Gates grün
  (`go test ./...`, `go vet ./...`, `node --check app.js`). S2 · Sonnet 4.6.
  Nächster Schritt: nächstes Roadmap-Paket nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-15 — **Paket #13 / ASD-003 „Aeronautical
  Map Layer" abgeschlossen.** Vier Häppchen: **3a Radar Dark Mode** —
  `WAYFINDER_MAP_THEME` (`dark`|`osm`, Default dark), `darkMapStyle` (CARTO
  `dark_nolabels`, key-frei), `mapConfigHandler` liefert Style + `theme`;
  Frontend `PALETTES` wählt helle Labels/Vektoren/Trails auf dunklem Grund
  (FR-MAP-001). **3b OpenAIP-Backend** (ADR 0004) — neues Paket
  `pkg/aeronautical`: defensiver OpenAIP-Client (Timeout, 32-MiB-Limit,
  `validGeometry`), best-effort `Service` mit Last-Good-Cache + nicht-
  blockierendem Refresh (`WAYFINDER_OPENAIP_*`), Endpoints
  `/api/airspace|navaids|waypoints`, `/metrics`-Kennzahlen
  `wayfinder_openaip_*` (FR-MAP-002, NFR-OPS-004/SEC-002/OBS-004). Track-Pfad
  und `/ready` bleiben **vollständig entkoppelt**. **3c/3d Overlays** —
  Luftraum (fill/line/label, schaltbar), VOR/NDB + Waypoints als Symbol-Layer
  mit laufzeit-gezeichneten Icons (kein Sprite-Asset), Zoom-Böden gegen
  Clutter, Layer-Steuerungs-Panel (`#layer-control`); `loadAeronautical`
  zieht alle 5 min nach, Fehler nicht-fatal (FR-MAP-003/004). Gates grün
  (`go build/vet/test`, `gofmt`, `node --check app.js`); Rauchtest des Binaries
  bestätigt Dark-Theme, leere Collections ohne Key (graceful) und die Metriken.
  Modell: Opus 4.8 (S4 wegen 3b). Datenquellen-Entscheidung „Live-OpenAIP" vom
  Projektverantwortlichen getroffen. Nächster Schritt: nächstes Roadmap-Paket
  nach Abstimmung.
- **Vorherige Aktualisierung:** 2026-06-15 — **Paket #3 „CAT065 Heartbeat"
  abgeschlossen (beide Seiten).** Wayfinder-Teil: neues Paket `pkg/cat065`
  (Decoder für CAT065 SDPS-Status, byte-genau gegen Fireflys Referenz-Dump,
  robust gegen Truncation/falsche Kategorie). Receiver dispatcht den
  gemeinsamen Multicast-Strom am führenden **CAT-Oktett** (`0x3E` → Track,
  `0x41` → Status, sonst Decode-Fehler) — neuer `dispatch`/`handleStatus`,
  `StatusHandler` in der Config, Test `TestDispatchRoutesByCategory`. Neues
  Paket `pkg/health` (`FeedHealth`): verfolgt Heartbeat-Ankunft, erkennt
  Staleness (kein Heartbeat seit > `WAYFINDER_FEED_STALE_TIMEOUT`, Default 3 s),
  `Observe` liefert nur Zustandswechsel. `main.go`: StatusHandler füttert
  Health + Heartbeat-Zähler, Monitor-Goroutine erkennt Staleness ohne Verkehr,
  `broadcastFeedStatus` pusht `feed_status`-WS-Nachricht (separater Pfad, leert
  **nicht** das Lagebild). Frontend: Feed-Banner (grün/rot/grau,
  `updateFeedBanner` in `app.js`, `#feed-status` in `index.html`). `/ready`
  wird bei stale Feed **nicht ready** (nur wenn je Heartbeat gesehen); `/metrics`
  um `wayfinder_cat065_heartbeats_received_total` + `wayfinder_feed_stale`
  ergänzt. `Message.FeedStatus`/`FeedStatusMessage` im Broadcaster. Doku:
  CLAUDE.md §2 (CAT065-Kurzfassung), Register FR-DATA-004/FR-OPS-004/NFR-OBS-003,
  ROADMAP/STATUS. Architektur-Entscheidung (gleiche Multicast-Gruppe, Dispatch
  am CAT-Oktett) vom Projektverantwortlichen bestätigt. **Firefly-Teil** (Sender:
  `firefly-asterix::cat065`, `run_heartbeat`, ADR 0018, ICD 2.3.0) ebenfalls
  fertig. Alle Gates grün (`go build/vet/test`, `gofmt`). Cross-Project-Issue
  (`from-firefly`) zum CAT065-Vertrag wird erstellt + nach beidseitiger
  Umsetzung geschlossen. Nächster Schritt: nächstes Roadmap-Paket nach
  Abstimmung (z. B. #4 Konfigurierbarer System-Referenzpunkt).
- **Vorherige Aktualisierung:** 2026-06-15 — Paket #2 „Observability-Grundgerüst"
  **abgeschlossen** mit Häppchen 2.3: gemeinsamer `/metrics`-Endpoint
  (Prometheus-Textformat). Wayfinder-Teil (NFR-OBS-002): neues Paket
  `pkg/metrics` (`Handler`/`Counter`/`Gauge`, hand-gerollte Prometheus-
  Exposition ohne externe Abhängigkeit). `Broadcaster` bekommt
  `EvictedCount()` (Eviction-Zähler, `pkg/broadcast/broadcast.go`),
  `Receiver` bekommt `DecodeErrorCount()` (`pkg/receiver/receiver.go`).
  `startProbeServer` (Port `:8080`) bekommt eine neue `/metrics`-Route neben
  `/health`/`/ready`: `wayfinder_cat062_blocks_received_total`/
  `wayfinder_cat062_tracks_received_total` (Counter),
  `wayfinder_cat062_decode_errors_total` (Counter),
  `wayfinder_tracks_current` (Gauge), `wayfinder_ws_clients_connected`
  (Gauge), `wayfinder_ws_clients_evicted_total` (Counter). Neue Tests:
  `pkg/metrics/metrics_test.go::TestHandlerRendersPrometheusExpositionFormat`,
  `pkg/broadcast/broadcast_test.go::TestBroadcastEvictsClientWithFullSendChannel`
  (jetzt zusätzlich `EvictedCount()`-Assertion),
  `pkg/receiver/receiver_test.go::TestReceiverDecodeErrorCountStartsAtZero`.
  Neue Anforderung NFR-OBS-002 im Register. Alle Gates grün
  (`go build`/`go vet`/`go test ./...`; `gofmt` clean außer dem
  vorbestehenden, unveränderten Befund in `pkg/receiver/receiver_test.go`).
  Firefly-Teil (Häppchen 2.2, `tracing` in `firefly-multicast`, und 2.3,
  `firefly-server::metrics`) ist ebenfalls erledigt — **Paket #2 vollständig
  abgeschlossen.** Nächster Schritt: nächstes Roadmap-Paket nach Abstimmung
  mit dem Projektverantwortlichen.
- **Vorherige Aktualisierung:** 2026-06-15 — Paket #1 „Multicast-Feed-Sicherheit",
  Häppchen 1.3: **Browser-Rand-Implementierung gemäß ADR 0003.**
  `pkg/ws/handler.go`: globales `CheckOrigin: func(r) bool { return true }`
  entfernt; `Handler` bekommt ein `allowedOrigins []string`-Feld und eine neue
  `checkOrigin`-Methode — Requests ohne `Origin`-Header (Nicht-Browser-Clients)
  und Same-Origin-Requests sind weiterhin erlaubt, Cross-Origin-Requests nur
  noch, wenn der `Origin`-Header in `WAYFINDER_ALLOWED_ORIGINS` steht (sonst
  fail-closed mit Warn-Log). `cmd/wayfinder/main.go`: neue `Config`-Felder
  `AllowedOrigins`, `AuthToken`, `TLSCertFile`, `TLSKeyFile`, alle per
  `loadConfig()` aus `WAYFINDER_ALLOWED_ORIGINS` (kommasepariert),
  `WAYFINDER_AUTH_TOKEN`, `WAYFINDER_TLS_CERT`/`_KEY` gelesen (Default: leer).
  Neue `authMiddleware`: greift nur, wenn `WAYFINDER_AUTH_TOKEN` gesetzt ist
  (sonst Pass-through + Warn-Log "relies on network isolation / reverse
  proxy"); prüft Bearer-Header oder `?token=`-Query-Param (Browser-WS kann
  keine Custom-Header beim Handshake setzen) via
  `crypto/subtle.ConstantTimeCompare`, sonst `401` + `WWW-Authenticate:
  Bearer`. Server-Setup von globalem `http.Handle`/`DefaultServeMux` auf
  lokalen `http.NewServeMux()` umgestellt, durch `authMiddleware` gewrappt;
  optionales TLS (`http.ListenAndServeTLS`, wenn `WAYFINDER_TLS_CERT`/`_KEY`
  beide gesetzt sind, sonst Klartext-HTTP wie bisher). Health-/Readiness-Probes
  (`:8080`) bleiben bewusst unauthentifiziert (separater Mux). Neue Tests:
  `pkg/ws/handler_test.go` (`TestCheckOrigin*`, 6 Fälle: ohne Origin,
  Same-Origin, Cross-Origin ohne/mit Allowlist, ungültiger Origin-Header) und
  `cmd/wayfinder/main_test.go` (`TestAuthMiddleware*` — deaktiviert/fehlender
  Token/falscher Token/Query-Param/Bearer-Header; `TestLoadConfig*SecurityEnvVars*`
  — Parsing und Default-Leerwerte). `docs/requirements/README.md` (NFR-SEC-001):
  Implementierung/Tests für den Browser-Rand jetzt eingetragen. Alle Gates
  grün (`go build`/`go vet`/`go test ./...`; `gofmt` clean außer dem
  vorbestehenden, unveränderten Befund in `pkg/receiver/receiver_test.go`).
  Damit ist **Paket #1 inhaltlich abgeschlossen** (1.4 — optionale
  Sender-Härtung in Firefly — bleibt als unabhängiges Nice-to-have offen).
  Nächster Schritt: mit dem Projektverantwortlichen das nächste Paket
  abstimmen (Vorschlag: Paket #2 „Observability-Grundgerüst", **S3 · Sonnet
  4.6**) oder optional 1.4 angehen.
- **Vorherige Aktualisierung:** 2026-06-15 — Paket #1 „Multicast-Feed-Sicherheit",
  Häppchen 1.2: **ADR 0003 „Sicherheit: Vertrauensgrenze des Empfangspfads und
  Browser-Rand"** erstellt (`docs/decisions/0003-sicherheit-empfangspfad-und-browser-rand.md`).
  Zwei Entscheidungen: (1) **Empfangspfad** spiegelt Fireflys ADR 0017 — Netz-
  Isolation auf der Netzwerk-Schicht, kein App-Krypto auf CAT062, robuster
  Decoder bleibt App-Schutzschicht (keine Code-Änderung). (2) **Browser-Rand**
  (`/`, `/ws`, `/api/map-config` auf `:8081`, heute ohne TLS/Auth, `CheckOrigin
  → true`): TLS+Auth primär am Reverse-Proxy/Ingress (OIDC/mTLS, cloud-native,
  kein Krypto-Eigenbau im ASD); ergänzend fail-closed in Wayfinder — strikter
  Origin-Check (`WAYFINDER_ALLOWED_ORIGINS`), optionale Token-Middleware
  (`WAYFINDER_AUTH_TOKEN`, Default aus + Warn-Log), optionales TLS
  (`WAYFINDER_TLS_CERT`/`_KEY`); Health-/Readiness-Probes (`:8080`) bleiben
  unauthentifiziert. Schließt das transformierte ehem. Issue #7. Neue
  Anforderung **NFR-SEC-001** im Register (Empfangspfad: dokumentiert;
  Browser-Rand: Implementierung folgt Häppchen 1.3). Reine Doku, kein
  Code-Diff. Nächster Schritt: Häppchen 1.3 — Implementierung Browser-Rand
  (Origin-Check, Token-Middleware, optionales TLS) in
  `pkg/ws/handler.go`/`cmd/wayfinder/main.go`, **S4 · Opus 4.8**.
- **Vorherige Aktualisierung:** 2026-06-15 (Branch `claude/tse-i062-080`, nach
  `main` gemergt — PR #8 (Wayfinder) / PR #16 (Firefly):
  **T5 — CAT062 Track-Ende (TSE, I062/080) dekodiert + Track-Entfernung, ICD
  2.2.0.** AP8 (Callsign) war bereits zuvor nach `main` gemergt — PR #7.) `decodeTrackStatus`
  liest die FX-Kette jetzt oktett-genau (CNF Oktett 1, **TSE Oktett 2 Bit 7
  `0x40`**, CST Oktett 4) und füllt `TrackStatus.Ended`; robust gegen früher
  endende Records. Durchgereicht via `broadcast.TrackMessage.Ended`
  (`json:"ended,omitempty"`); das Frontend (`updateTracksLayer`) **filtert**
  Ende-Records heraus → Symbol/Label/Vektor/Trail verschwinden sofort (statt
  Timeout). Test: `pkg/cat062/decoder_test.go::TestDecodeTrackEnd` (Referenz
  aus Fireflys `track_status_carries_tse_when_ended`). `CLAUDE.md` §2 und
  `docs/requirements/README.md` (FR-DATA-003) aktualisiert. Gates grün
  (`go build`/`go vet`/`go test ./...`; `gofmt` für geänderte Dateien). **TSE
  (Firefly T1–T4 + Wayfinder T5) damit beidseitig abgeschlossen.**
- **Vorherige Aktualisierung:** 2026-06-15 (Branch `claude/callsign-i062-245`:
  **AP8 — CAT062 Target Identification I062/245 (Callsign) dekodiert, ICD
  2.1.0.**) `pkg/cat062/decoder.go` zieht FRN 10 nach: 7-Byte-Item
  (STI/spare-Oktett + 8 × 6-Bit-IA-5), `decodeTargetIdentification`/
  `ia5Decode` (fremde Codes defensiv → Leerzeichen, robust gegen
  Fehl-Datagramme). `DecodedTrack.Callsign *string` (trailing spaces
  getrimmt), durchgereicht über `broadcast.TrackMessage.Callsign`
  (`json:"callsign,omitempty"`) bis ins Frontend. `app.js::buildLabel` zeigt
  das Callsign jetzt als primäre Label-Zeile (Track-Nummer als Fallback), FL
  weiterhin als zweite Zeile. FRN 10 liegt im bereits vorhandenen 2.
  FSPEC-Oktett → additiv, kein Wire-Format-Bruch. Test:
  `pkg/cat062/decoder_test.go::TestDecodeCallsign` (Referenzwerte aus Fireflys
  `target_identification_packs_eight_six_bit_ia5_codes`). `CLAUDE.md`
  Abschnitt 2 (FRN-Tabelle) und `docs/requirements/README.md` (FR-DATA-002)
  aktualisiert. Alle Gates grün (`go build`/`go vet`/`go test ./...`/`gofmt`
  für die geänderten Dateien; ein vorbestehender `gofmt`-Befund in
  `pkg/receiver/receiver_test.go` ist unverändert und nicht Teil dieser
  Änderung). **AP7 (Firefly-Encoder) und AP8 (dieser Schritt) sind damit
  beide abgeschlossen.**
- **Frühere Aktualisierung:** 2026-06-15 (Branch `claude/callsign-i062-245`:
  Doku-/Docker-Vorbereitung fürs Testen. `README.md` komplett neu (Quickstart
  Docker/lokal, Architektur-Übersicht, Konfig-Tabelle, Build & Test, Links).
  Neu: `Dockerfile` (Multi-Stage `golang:1.23-bookworm` → `debian:bookworm-
  slim`, Healthcheck `/health`), `docker-compose.yml` (`network_mode: host` —
  notwendig für CAT062-Multicast-Empfang), `.dockerignore`, `DOCKER.md`
  (Standalone + End-to-End mit Firefly, inkl. `FIREFLY_CAT062_ENABLED=true` und
  macOS/Windows-Docker-Desktop-Hinweis). Firefly-seitig analoger Abschnitt in
  README/DOCKER.md ergänzt. Docker-Build konnte in dieser Sitzung nicht
  getestet werden (kein Docker-Daemon verfügbar) — `go build`/`go vet`/
  `go test ./...` sind grün.)
- **Frühere Aktualisierung:** 2026-06-14 (Branch `claude/serene-heisenberg-xq4rla`:
  AP2 — Vertikallage I062/136 + UAP-Standardtreue; davor Kurs-Pfeile + Trails)
- **Branch:** `claude/serene-heisenberg-xq4rla` — **M1.1–M1.3 abgeschlossen**
  (CAT062 Multicast → Decoder → Broadcaster → WebSocket-Clients, in `main`).
  **M1.4.a/b/c abgeschlossen**: `internal/webui` (eingebettetes Frontend),
  MapLibre GL JS Karte, WebSocket-Client mit Reconnect, Live-Tracks als
  farbige Kartensymbole (grün=confirmed, grau=tentativ, orange=coasting) mit
  Track-Nummern-Labels. Siehe `docs/milestones/M1.4.c_Track_Rendering.md`.
  **M1 ist funktional abgeschlossen** (Backend-Pipeline + Live-Kartendarstellung).
  **Neu (post-M1, UI-Häppchen A.1)**: Kurs-Pfeile (ASD-Speed-Vector-Line,
  60s-Vorausschau) je Track in `internal/webui/static/app.js` — eigene
  GeoJSON-Quelle `track-vectors`/Layer `track-vectors-lines`, berechnet aus
  `vx`/`vy` (m/s, Ost/Nord) per flacher Erdnäherung. Live gegen Firefly
  (CAT062-Multicast) verifiziert.
  **Neu (post-M1, UI-Häppchen A.2)**: Track-Trails — die letzten 20 Positionen
  je Track werden im Frontend-State (`state.trackHistory`) gehalten und als
  blassgraue Spur (`track-trails`/`track-trails-lines`) gerendert; History wird
  bereinigt, sobald ein Track aus dem Update verschwindet. Live gegen Firefly
  verifiziert.
  **Neu (AP2, ICD-Thema): Vertikallage I062/136 + UAP-Standardtreue** (lockstep
  zu Fireflys ADR 0015 / ICD 2.0.0, Issue #5 `from-firefly`). Decoder
  (`pkg/cat062`) zieht nach: **I062/500 von FRN 16 → FRN 27** und neues
  optionales **I062/136** (FRN 17, signed i16, LSB 1/4 FL = 25 ft).
  `DecodedTrack.FlightLevelFt` + `broadcast.TrackMessage.flight_level_ft`
  durchgereicht; `app.js` zeigt die Flugfläche als zweite Label-Zeile „FLnnn"
  (ASD-Datablock-Stil). Referenz-Vektor-Test aktualisiert (FSPEC
  `[0x9F,0x0F,0x01,0x04]`, LEN 40) + neuer `TestDecodeFlightLevel`. Live gegen
  Firefly verifiziert (FL372/FL340 im WS-Strom). → Issue #5 kann nach Merge
  geschlossen werden.
  Nächster Schritt: AP7/AP8 (Callsign I062/245), AP5/AP6 (CAT065 Heartbeat),
  weitere UI-Häppchen.

---

## 1. Wo wir gerade stehen

**AP9.9 ADS-B-Badge (ICD 2.4.0): ✅ Abgeschlossen** (PR #22, gemergt)
**ASD-006 (Vue 3 + Vuetify 3 MD3): ✅ Abgeschlossen**
**ASD-007 Farbschema: ✅ Abgeschlossen**
**ASD-008 Navigation Rail: ✅ Abgeschlossen**
**ASD-009 Karten-Controls: ✅ Abgeschlossen**
**ASD-010 Kategorie-Filter-Chips: ✅ Abgeschlossen**

**Strategische Ausrichtung: Wayfinder 2.0** (Multi-Mandanten-Plattform) — siehe
`docs/ROADMAP.md` §0/§1 (zentral) und `docs/design/wayfinder-2.0-konzept.md`
(Begründung). Kritischer Pfad: **Stufe 0 (ADRs) → 1 (Identität/Persistenz) → 2
(mandanten-isolierter Stream, 🔒) → 3 (Config/Admin) → 4 (Sensorik) → 5
(Kommerz/HA)**.

**✅ Stufe 0 (Entscheidung & Fundament) abgeschlossen:**

| AP | Inhalt | Stufe | Status |
|----|--------|-------|--------|
| **WF2-00** | ADR 0005 „Multi-Mandanten-Pivot" | S4 · Opus 4.8 | ✅ erledigt |
| **WF2-01** | ADR 0006 „Konfig-/Identitäts-Persistenz" | S4 · Opus 4.8 | ✅ erledigt |
| **WF2-02** | ADR 0007 „Cloud-Ingest & Feed-Fan-out" (NATS JetStream) | S4 · Opus 4.8 | ✅ erledigt |

**Stufe 1 — in Arbeit:** **WF2-10.1 ✅** (`pkg/store`: pgx-Pool + eingebetteter
In-House-Migrationsrunner + Schema `00001_init` + DB-freie Tests). **➡️ Nächster:
WF2-10.2** (Repository-Zugriffe tenants/users mit pgx), S3 · Sonnet 4.6
(+Opus-Review).

Offen, **ASD-Kern (mandanten-unabhängig, parallel möglich** — nicht im kritischen
Pfad, Details/Abgleich in ROADMAP §2):

| AP | Inhalt | Stufe |
|----|--------|-------|
| **ASD-011** | Erweitertes Track-Detail-Panel (Ausbau TrackDetailCard.vue) | S2 · Sonnet 4.6 |
| **ASD-012** | Range-Rings + Scale-Bar + Nord-/Track-up | S3 · Opus 4.8 |
| **ASD-013** | Alarm-/Ereignis-Panel | S3 · Sonnet 4.6 |

## 2. Gesetzte Entscheidungen (Fundament)

| Thema | Entscheidung | Quelle | Status |
|-------|--------------|--------|--------|
| Betriebsmodus | Produktionsbetrieb (nicht Lernprojekt) | Fireflys ADR 0014 | ✅ |
| Schnittstelle | **CAT062 over UDP-Multicast** | Fireflys ADR 0006 + 0014, `CLAUDE.md` §2 | ✅ |
| Stack | Go + MapLibre GL JS + WebSocket-Server-Push | ADR 0001 | ✅ |
| Frontend-Framework | Vue 3 + Vuetify 3 (MD3), Vite, Vitest, Pinia | ADR 0002 | ✅ |
| Farbschema | Cyan-Primary aus ASD-Mockup | `docs/design/color-tokens.md` | ✅ |
| **Wayfinder 2.0 — Pivot/Mandanten-Modell** | **Hybrid** (Feed-Katalog + Abos + Sicht-Filter); Pivot ratifiziert | **ADR 0005** | ✅ ratifiziert |
| **Wayfinder 2.0 — Kommerz-Scope** | **Feature-Flags ja, Stripe-Billing zurückgestellt** | **ADR 0005** (Konzept §6.5) | ✅ (WF2-51 ruht) |
| **Wayfinder 2.0 — Isolationsgrenze** | Server-seitige AuthZ pro Subscription, fail-closed, Pflicht-Negativtests | **ADR 0005**, NFR-SEC-003 | ✅ Prinzip gesetzt (Umsetzung WF2-21/22) |
| **Wayfinder 2.0 — Persistenz** | PostgreSQL + `pgx`; **In-House-Migrationsrunner** (goose verworfen, ADR 0006 Nachtrag); Stateless-Split; Redis zurückgestellt; **Go-Baseline 1.25** | **ADR 0006**, FR-TEN-002 | 🔵 Umsetzung WF2-10 (10.1 ✅) |
| **Wayfinder 2.0 — Identität** | OIDC@Proxy primär + eingebauter Fallback + none (`WAYFINDER_AUTH_MODE`); Tenant-Kontext fail-closed | **ADR 0006**, NFR-SEC-004 | ✅ entschieden (Umsetzung WF2-11/12) |
| **Wayfinder 2.0 — Cloud-Ingest/Transport** | Public Cloud + K8s; `FeedSource` (Multicast/Stream) + Ingest-Gateway; Bus = **NATS JetStream** (RabbitMQ/Kafka verworfen) | **ADR 0007**, FR-FEED-001/NFR-SCALE-001 | ✅ entschieden (Umsetzung WF2-20/52/53) |

## 3. Nächster Schritt

➡️ **WF2-10.2: Repository-Zugriffe (tenants/users)** — S3 · Sonnet 4.6
(+Opus-Review), nach Ankündigung & „Go".

Baut auf `pkg/store` (10.1) auf: typsichere CRUD-Zugriffe für `tenants`/`users`
mit pgx (ggf. `sqlc`-Codegen — sqlc-Binary ist hier nicht installiert, daher
entweder `go install` oder handgeschriebene pgx-Queries), inkl. Tests. DB-
Integrationstests (Schema-Anwendung) folgen in WF2-10.3 (CI/Testcontainers, da
**kein Docker-Daemon** in dieser Umgebung).

Danach WF2-10.3 (restliche Repositories), dann WF2-11 (AuthN), WF2-12
(Tenant-Context), WF2-13 (Admin-Bootstrap).

**Erledigt in dieser Sitzung:** **Stufe 0 komplett** (WF2-00/01/02, ADR
0005/0006/0007) **+ WF2-10.1** (`pkg/store`: pgx-Pool, In-House-Migrationsrunner,
Schema, DB-freie Tests). Dabei ADR-0006-Nachtrag: **goose verworfen** (zog
`modernc.org/sqlite`), **Go-Baseline 1.23 → 1.25** (Dockerfile gebumpt). Register
FR-TEN-001/002, FR-FEED-001, NFR-SEC-003/004, NFR-SCALE-001.

**Parallel möglich (nicht kritischer Pfad):** ASD-011/012/013 (ASD-Kern,
ROADMAP §2) — widerspruchsfrei zu 2.0, von einem leichteren Modell ziehbar.

## 4. Schnell-Einstieg

```bash
cd /home/user/Wayfinder
git log --oneline | head -10
npm run build          # in frontend/
npm run test -- --run  # in frontend/
go test ./...
```
