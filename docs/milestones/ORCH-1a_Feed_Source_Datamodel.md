# ORCH-1a — Feed-Quell-Datenmodell (Schema + Store)

> Erstes Häppchen des Pakets **ORCH-1** (ADR 0012, Epic „Mandanten-eigene
> Tracker-Instanzen & Auto-Orchestrierung"). Es legt das **Datenmodell** für die
> generische, Firefly-agnostische Quell-Konfiguration eines Feeds an — die
> Grundlage, auf der später Admin-API (ORCH-1b), UI-Quell-Builder (ORCH-1c) und
> der Reconciler (ORCH-3) aufsetzen.
>
> **Lieferumfang ORCH-1a:** DB-Migration + Store-Schicht (Modell, Validierung,
> Coverage-Ableitung, Repo-Accessoren) + Tests. **Noch nicht** Teil dieses
> Häppchens: HTTP-Endpunkte und Frontend (ORCH-1b/-1c).

## Fachlicher Hintergrund

Heute ist ein Feed eine **passive Katalogzeile** (Multicast-Gruppe/Port + rein
informativer `sensor_mix`): Niemand *erzeugt* den Strom — Firefly-Instanzen
werden von Hand gestartet (ADR 0012, Ist-Zustand). Das Ziel des ORCH-Epics ist,
dass das **Zuweisen eines Feeds an einen Mandanten automatisch die passende
Firefly-Instanz startet** — mit genau den Quellen und dem geografischen
Ausschnitt, die dieser Mandant sehen soll (z. B. „nur ADS-B für Speyer").

Damit der Reconciler (ORCH-3) eine Firefly-Instanz konfigurieren kann, muss der
Feed **maschinenlesbar** tragen: *welche* Live-Quellen Firefly öffnen soll und
*welchen groben Ausschnitt* es abdecken soll. ORCH-1a fügt genau dieses
Datenmodell hinzu — ohne Verhalten zu ändern (rein additiv, bestehende Feeds
lesen sich als „keine Quelle konfiguriert").

## Was umgesetzt wurde

### 1. Migration `00010_feed_source_config.sql`

Zwei additive Spalten auf `feeds`:

- **`source_config JSONB NOT NULL DEFAULT '[]'`** — die Quell-Liste. JSONB, damit
  neue Quell-Arten additiv hinzukommen, ohne Schema-Bruch; Default `'[]'` hält
  bestehende Feeds gültig („noch keine Quelle").
- **`coverage_bbox JSONB`** (nullable) — die abgeleitete **grobe äußere** Geo-
  Grenze (`NULL` = noch nicht abgeleitet).

Beide sind nicht-breaking; der `down`-Abschnitt entfernt sie wieder.

### 2. Quell-Modell (`pkg/store/feed_sources.go`)

- **`SourceType`** — geschlossenes Vokabular (wie `pkg/sensorclass`):
  `adsb_opensky`, `flarm_aprs`, `radar_asterix`. Erweiterbar durch Hinzufügen
  einer Konstante; die JSONB-Spalte braucht keine Migration.
- **`Source`** — ein Quell-Eintrag: `Type`, optional `BBox`, optional `SAC`/`SIC`
  (Radar-Sensor-Identität), optional `CredRef`.
- **`SourceConfig []Source`** — die geordnete Liste.

### 3. Validierung am Schreib-Rand (`SourceConfig.Validate`)

Wie der Sensor-Mix-Check (WF2-41) wird eine ungültige Konfiguration **vor** dem
DB-Write abgewiesen, damit der Katalog nie eine Konfiguration speichert, die der
Orchestrator nicht starten kann. Regeln pro Art:

- **Flächengebundene Quellen** (`adsb_opensky`, `flarm_aprs`): **erfordern** eine
  `bbox` und dürfen **keine** `sac`/`sic` tragen (eine Internet-Flächenquelle hat
  keine Sensor-Identität).
- **`radar_asterix`**: **erfordert** `sac` **und** `sic` (jeweils 0..255); `bbox`
  optional.
- **`bbox`** (falls vorhanden): WGS84-gültig (lat ∈ [-90,90], lon ∈ [-180,180],
  min ≤ max).
- **`cred_ref`** (falls vorhanden): nicht-leer (getrimmt), Länge ≤ 200.

Fehler sind `*InvalidSourceError` mit **Index** des betroffenen Eintrags
(`errors.As`-bar) — die spätere Admin-API (ORCH-1b) kann damit auf die
fehlerhafte Zeile zeigen.

### 4. Coverage-Ableitung (`SourceConfig.CoverageBBox(marginKm)`)

Reine, getestete Funktion: bildet die **Union** aller Quell-BBoxen und weitet sie
um `marginKm` auf (lat/lon-geklemmt). Das ist die **grobe äußere** Coverage, die
Wayfinder später an Firefly übergibt (`FIREFLY_COVERAGE_BBOX`) — bewusst **lose**
und **getrennt** von der präzisen inneren Mandanten-AOI (ADR 0012 §3,
coarse-outer vs. precise-inner). Der Längengrad-Rand nutzt die Box-Kante mit dem
größten |Breitengrad| (wo ein Längengrad am kürzesten ist), sodass die Aufweitung
überall im Kasten mindestens `marginKm` beträgt — bewusst konservativ. Ohne
BBox-Quelle (z. B. reiner Radar-Feed) liefert sie `nil` (Coverage bleibt unbesetzt).

### 5. Repo-Accessoren (dedizierte Isolation)

- **`FeedRepo.GetSourceConfig(feedID) → (SourceConfig, *BBox, error)`**
- **`FeedRepo.SetSourceConfig(feedID, SourceConfig, *BBox)`** — validiert zuerst,
  schreibt erst dann; `0` Zeilen → `ErrNotFound`.

Analog zur OpenAIP-Key-Isolation (ONB-6) liegen diese **nicht** in der schlanken
`Feed`-Zeile/DTO, sondern in eigenen Accessoren — die häufigen `List`/`GetByID`-
Abfragen bleiben frei von der JSONB-Last.

## Sicherheits-Betrachtung

- **Credential-Isolation (NFR-SEC-004):** `cred_ref` ist nur ein **Verweis** auf
  ein Pro-Feed-Secret — nie der Klartext. Der eigentliche Secret-Wert (OpenSky-
  Client-Credentials etc.) wird in ORCH-2 in einem getrennten Secret-Speicher
  gehalten und nur dem `InstanceBackend` beim Start gereicht; er erscheint nie in
  einem DTO oder am Browser (ADR 0012 §6).
- **Robuste Validierung:** Jede Quell-Konfiguration wird gegen ein geschlossenes
  Vokabular und WGS84-Grenzen geprüft, bevor sie persistiert wird — kein
  Vertrauen in ungeprüfte Eingaben (Charter §7).
- **Schnittstellen-Wirkung:** Keine. `source_config`/`coverage_bbox` sind rein
  Wayfinder-intern; der CAT062-Draht-Vertrag mit Firefly bleibt unberührt.

## Tests

- **`pkg/store/feed_sources_test.go`** (DB-frei): `TestSourceConfigValidate`
  (Vokabular, Per-Art-Regeln, BBox-Bereiche, blank cred_ref, Fehler-Index);
  `TestCoverageBBox*` (Union, keine-BBox→nil, Marge weitet auf, Pol-Clamp).
- **`pkg/store/feeds_subscriptions_integration_test.go::TestIntegrationFeedSourceConfig`**
  (real-PG): Default leer/`nil`; Round-Trip mit `cred_ref`/`sac`/`sic` +
  abgeleiteter Coverage; ungültige Konfig wird abgewiesen **ohne** Teil-Write
  (vorherige gute Konfig bleibt); Leeren der Konfig; `ErrNotFound` für beide
  Accessoren bei fehlendem Feed.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-001** (Quell-Datenmodell), **NFR-SEC-004**
(Quell-Credential-Isolation).

## Nächste Häppchen

- **ORCH-1b** — Admin-API: `GET/PUT /api/admin/feeds/{id}/sources`,
  Validierung→`400` mit Quell-Index, hinter `requireAdmin`. ✅ erledigt
  (`docs/milestones/ORCH-1b_Feed_Source_Admin_API.md`).
- **ORCH-1c** — Frontend-Quell-Builder im Feed-Dialog (Sensor-Mix-Checkboxen,
  BBox-Vorschlag aus Mandanten-AOI + Marge).
