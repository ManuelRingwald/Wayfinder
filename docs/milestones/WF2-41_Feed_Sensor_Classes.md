# WF2-41 — Feed-Sensorklassen-Katalog & entitlement-gegatete Abos

> **Stufe:** 4 (Sensor-/Stream-Management) · **Paket:** WF2-41 ·
> **Einstufung:** S3 · Sonnet 4.6 · **Grundlage:** ADR 0005 §6.4 (Sensor-Mix als
> Feed-Metadatum; Entitlements binden an **Feeds**, nicht an Per-Track-Sensortypen).
> **Abhängigkeiten:** WF2-20 (Feed-Registry), WF2-50 (Entitlement-Service).

## Warum (fachlich)

Im Hybrid-Mandanten-Modell (Feed-Katalog + Abos) müssen zwei Dinge sauber sein:

1. **Was steckt in einem Feed?** Ein Feed trägt seine **Sensorklassen-
   Zusammensetzung** als Metadatum („Feed A = ADS-B-only", „Feed B =
   PSR+SSR+ADS-B"). Bisher war `sensor_mix` ein **ungeprüftes Freitextfeld** —
   Tippfehler („ADSB" vs „ADS-B") machten die Metadaten unzuverlässig, obwohl
   Betreiber Abos danach auswählen und Auditoren verlässliche Angaben erwarten.
2. **Wer darf wie viele Feeds?** Feed-Zugriff (= Abo) ist die **Entitlement-
   Granularität**. Die **Anzahl** der Feeds ist das Geschäftsmodell: Basis-Mandant
   = ein Feed, zahlender Mandant = mehrere. Bisher war der Grant **ungated**.

## Was (technisch)

### A. Sensorklassen-Katalog (`pkg/sensorclass`, rein + getestet)
- Kontrolliertes Vokabular: `PSR`, `SSR`, `MODE_S`, `ADS-B`, `MLAT`, `FLARM`
  (+ `Describe` je Klasse).
- `Parse(s)` mappt **gängige Legacy-Schreibweisen** kanonisch: normalisiert
  (uppercase, nur A–Z0–9) und schlägt in einer Alias-Tabelle nach —
  `ads-b`/`ADS_B`/`ADS B`/`1090ES` → `ADS-B`, `Mode A/C`/`MODEC` → `SSR`,
  `Mode S`/`mode-s` → `MODE_S`, `WAM`/`multilateration` → `MLAT`. Kanonische
  Werte sind idempotent (round-trip).
- `Canonicalize(raw)` normalisiert + **dedupliziert** (Reihenfolge erhalten) und
  **weist unbekannte Tokens ab** (`*UnknownClassError{Token}`).
- **Erzwungen am Chokepoint** `store.FeedRepo.Create`: ungültige/typo'te Klassen
  erreichen **nie** die DB (analog zum Feature-Katalog-Guard). **Kein
  Schema-Change** — `sensor_mix` bleibt JSONB, nur app-seitig validiert.
- **Surfacing:** `GET /api/admin/sensor-classes` (read-only Katalog für die SPA).

### B. Abos binden an Feeds — die harte Invariante
- Der Grant-Pfad (`adminapi.grantSubscription`, super_admin) prüft **vor**
  `Subscribe`: hält der Mandant bereits ≥ 1 *anderen* Feed und fehlt ihm
  `multi_feed` (WF2-50)? → **409 Conflict**, die DB wird **nicht** berührt
  (fail-early — der invalide Zustand „> 1 Feed ohne Entitlement" kann gar nicht
  erst entstehen).
- **Idempotenz erhalten:** ein Re-Grant desselben bereits gehaltenen Feeds zählt
  nicht hoch → bleibt 204.
- **Ehrliche super_admin-Semantik:** super_admin muss erst das `multi_feed`-
  Entitlement setzen (WF2-50-Endpoint), dann den zweiten Feed granten. Der
  Invariant „Feed-Anzahl ⟂ Entitlement" ist nicht umgehbar.
- **Defense in depth:** Da der ungültige Zustand nie persistiert wird, respektiert
  auch der WF2-21-Fan-out die Grenze automatisch (keine zweite Durchsetzungsstelle
  nötig).

## Sicherheit / Korrektheit
- **Fail-early am Rand:** Validierung (Sensorklassen) und Invariante (Feed-Anzahl)
  greifen, bevor Daten in die DB gelangen — keine „Phantom"-Zustände im Betrieb.
- **Cross-Tenant unberührt:** die neuen/erweiterten Grant-Routen bleiben
  super_admin-only (`requireSuper`); der bestehende Cross-Tenant-403-Test gilt
  weiter.
- Kein CAT062-/ICD-Bezug, kein Frontend-Pflicht-Visual.

## Tests
- `pkg/sensorclass/*_test.go`: Legacy-Wahrheitstabelle, kanonischer Round-Trip,
  Dedup/Leerzeichen, unbekannt → `*UnknownClassError`, `All`/`Describe`.
- `pkg/adminapi/adminapi_test.go`: 1. Feed erlaubt; **2. Feed ohne `multi_feed`
  → 409 und `Subscribe` nicht erreicht**; 2. Feed mit `multi_feed` → 204;
  idempotenter Re-Grant → 204; `GET /api/admin/sensor-classes`.
- `pkg/store/feeds_subscriptions_integration_test.go` (**real-PG**): unbekannte
  Klasse → Fehler; Legacy → kanonisch/dedupliziert.
- `pkg/adminapi/adminapi_integration_test.go` (**real-PG**): Grant-Gating
  Ende-zu-Ende (1. Feed 204 → 2. Feed 409 → `multi_feed` setzen → 2. Feed 204 →
  zwei Abos).
- Gates grün: `go test ./...` + real-PG (`scripts/pg-test.sh`), `go vet`,
  `gofmt`.

## Abgrenzung / Nächstes
- **Keine** Per-Sensorklassen-Entitlements (bewusst: ADR 0005 §6.4 bindet auf
  Feed-Ebene). Echte Per-Track-Sensorprovenienz bleibt WF2-42 (Firefly-ICD).
- Pre-existierende Feeds können noch Legacy-Schreibweisen tragen; sie
  normalisieren beim nächsten `Create`/Edit (Read-Pfad unverändert).
