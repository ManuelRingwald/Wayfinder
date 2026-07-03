# ADR 0016 — Ausgehende Wetter-Feeds (DWD & NOAA) als Backend-Proxy + Cache

- **Status:** akzeptiert
- **Datum:** 2026-07-02
- **Schnittstellen-relevant:** nein (keine Änderung am CAT062-Draht-Vertrag mit
  Firefly; betrifft **neue, eigenständige** Auxiliar-Datenquellen für die
  Kartendarstellung und eine Header-Infobox)

## Kontext

Der Betreiber wünscht drei wetterbezogene ASD-Erweiterungen:

1. **Wetter-Overlay** — Niederschlags-/Radar-Bild des **Deutschen Wetterdienstes
   (DWD)** unter der Luftlage (Gewitterzellen, Niederschlag).
2. **QNH-Infobox** — aktuelle Höhenmesser-Einstellung (hPa) für den Flugplatz im
   Kopfbereich.
3. **Wetterwarnungen-Overlay** — amtliche DWD-Warnpolygone (Gewitter, Sturm,
   Schnee/Eis …).

Diese Daten gehören **nicht** zum CAT062-Track-Strom von Firefly und sind auch
kein Teil des CAT065/CAT063-Status. Sie kommen von **externen, öffentlichen
HTTP-Quellen** und führen Wayfinders **erste allgemeine ausgehende
Internet-Abhängigkeit außerhalb von OpenAIP (ADR 0004)** ein — gleich zu **zwei**
Anbietern:

- **DWD-„geodienste"** (`https://maps.dwd.de/geoserver/`) — öffentlicher
  GeoServer (WMS/WMTS für Radar, WFS-GeoJSON für Warnungen), **kein API-Key**.
- **NOAA/NWS Aviation Weather Center** (`https://aviationweather.gov/api/data/`)
  — offener METAR-Feed (das `altim`-Feld trägt das QNH in hPa), **kein API-Key**,
  US-Government Public Domain.

Damit stellen sich dieselben Weichenstellungs-Fragen wie bei ADR 0004, plus eine
fachliche QNH-Frage:

1. **Wo läuft der Abruf** — im Browser (jeder Client gegen DWD/NOAA) oder im
   **Go-Backend** (ein Egress-Punkt)?
2. **Verfügbarkeit** — das ASD ist sicherheitsrelevant; was passiert bei
   langsamer/nicht erreichbarer Quelle?
3. **Vertrauen in externe Daten** — CLAUDE.md §7 („niemals einem Datagramm
   vertrauen") gilt sinngemäß für jede externe HTTP-Quelle.
4. **QNH-Datenwahrheit** — QNH ist **nicht** dasselbe wie der auf MSL reduzierte
   Druck (QFF/PMSL), den die offenen DWD-Produkte (POI `PMSL`, MOSMIX `PPPP`)
   liefern; welche Quelle liefert **echtes** QNH?
5. **Lizenz/Attribution** — welche Nutzungsbedingungen gelten, und was muss
   sichtbar attribuiert werden?

## Entscheidung

### 1. Abruf im Backend (Proxy + Cache), nicht im Browser

Wayfinder ruft DWD und NOAA **server-seitig** ab, cached die Ergebnisse und
liefert sie dem Frontend über **interne Endpoints** aus:

- Radar-Kacheln als getunnelter Tile-Proxy (`/api/weather/radar/{z}/{x}/{y}.png`
  → DWD-WMS `GetMap` in EPSG:3857),
- Warnungen als GeoJSON (`/api/weather/warnings.geojson`),
- QNH als kleines JSON (`/api/weather/qnh`).

Begründung wie ADR 0004: **ein Egress-Punkt** statt N Browser; das Frontend
spricht **nur denselben Origin** an (kein zusätzlicher CORS-/Mixed-Content-Rand,
kein Browser→Drittanbieter); die Attribution wird **zentral** gesetzt; und der
Rand ist **per-Tenant gatebar** (Feature-Entitlement) statt im Client.

### 2. Graceful Degradation — das ASD hängt nicht am Wetter

- Alle drei Features sind **best-effort**. Der **Track-Pfad
  (CAT062 → WebSocket → Karte) ist davon vollständig unabhängig** und rendert
  immer.
- Ein DWD-/NOAA-Ausfall darf **`/ready` nicht umkippen** und den **Start nicht
  blockieren**. Bei Fehler wird der **letzte gute Cache** weiter ausgeliefert;
  ohne Cache liefert der Endpoint eine **leere/transparente** Antwort (HTTP 200)
  statt eines Fehlers — Overlay/QNH fehlen dann einfach.
- **Periodischer Refresh** je nach Produkt-Takt (Radar/Warnungen ~5 min,
  QNH/METAR ~30 min) plus einmalig beim Start, **nicht-blockierend**.

### 3. Robuster, misstrauischer Konsument

- **Timeouts** auf alle Requests; **Antwort-Größen begrenzen** (`io.LimitReader`,
  Schutz gegen Speicher-Exhaustion) — analog `pkg/aeronautical`.
- **Tolerantes Parsen**: fehlerhafte Einzel-Records (ein Warn-Feature, ein
  METAR) werden **verworfen** statt den ganzen Abruf scheitern zu lassen; **kein
  Panic** auf Eingabe-Daten; **Fuzzing** des METAR-/GeoJSON-Parsers ist
  vorgesehen (CLAUDE.md §7).
- **Observability:** Erfolg/Fehler-Zähler und Cache-Alter als Metriken
  (`wayfinder_weather_*`), Fehler als strukturierte Logs.

### 4. Kein Endpoint konfiguriert ⇒ Feature still aus

Ohne die jeweilige Quell-URL bleibt das Feature **deaktiviert** (Warn-Log beim
Start); die Endpoints liefern leer, das ASD läuft normal weiter. `Enabled` wird
— wie bei OpenAIP (`Enabled: cfg.OpenAIPAPIKey != ""`) — aus der gesetzten URL
abgeleitet. Jedes Feature ist zusätzlich per **Feature-Entitlement** pro Mandant
schaltbar (`weather_radar`, `qnh`, `weather_warnings`; ADR 0014 Multi-Tenant).

### 5. QNH-Datenwahrheit — nur echtes METAR-QNH, nie PMSL

- QNH wird **ausschließlich** aus einem **METAR** bezogen (NOAA-Feld `altim`,
  bzw. die `Qxxxx`-Gruppe in `rawOb`) — das ist der operative, von Piloten/Lotsen
  gesetzte Wert.
- Der auf MSL reduzierte Druck aus den offenen DWD-Produkten (POI `PMSL`, MOSMIX
  `PPPP`) ist eine **andere physikalische Größe** (Reduktion über Ist- statt
  ISA-Standardtemperatur; MOSMIX ist zudem eine **Vorhersage**) und wird **nicht**
  als QNH angezeigt.
- **Kein erfundener Wert im UI** (CLAUDE.md, „keine Fake-UI"): fehlt ein aktuelles
  METAR, wird die QNH-Anzeige ausgegraut/ausgeblendet, nicht geschätzt. Das
  METAR-Alter wird mitgeführt; ein zu altes QNH wird als stale markiert.

### 6. Lizenz & Attribution

- **DWD:** frei, inkl. kommerzieller Nutzung, unter **GeoNutzV** bzw. **CC BY
  4.0** — Pflicht-Quellenangabe **„© Deutscher Wetterdienst"** (bzw. „Quelle:
  Deutscher Wetterdienst"), plus **Änderungshinweis**, falls Daten verändert
  werden. Die Attribution wird im Karten-Overlay (`attribution` der MapLibre-
  Source) bzw. der Overlay-Legende gesetzt; für die Warn-Grenzgeometrie
  zusätzlich „© GeoBasis-DE/BKG".
- **NOAA/NWS:** Werk der US-Bundesregierung → **Public Domain**; Höflichkeits-
  Attribution „METAR/QNH: NOAA/NWS Aviation Weather Center".

## Begründung

- **Sicherheit:** Backend-Proxy gibt einen einzigen, kontrollierbaren,
  auditierbaren Egress; kein Browser→Drittanbieter, kein CORS-Rand, per-Tenant
  gatebar.
- **Verfügbarkeit des ASD:** Eine externe Abhängigkeit darf die Kernfunktion
  (Tracks anzeigen) eines sicherheitsrelevanten Lagebilds nie gefährden —
  best-effort + Last-Good-Cache + nicht-blockierender Start setzen das durch.
- **Misstrauen gegenüber externen Daten** ist die konsequente Fortschreibung des
  „robusten Decoder"-Prinzips (CLAUDE.md §7) auf zwei HTTP-Quellen.
- **Datenwahrheit:** QNH nur aus METAR schützt vor einer fachlich falschen
  Anzeige auf einem sicherheitsrelevanten Display.

### Verworfene Alternativen

- **Browser ruft DWD/NOAA direkt:** N-faches Abrufen, CORS-/Rate-Limit-Probleme
  (NOAA verlangt einen distinktiven User-Agent und limitiert ~100 req/min),
  Attribution verstreut, schwerer abzusichern. Verworfen.
- **DWD-PMSL/MOSMIX als QNH:** physikalisch **falsche** Größe für ein ASD;
  verworfen (siehe Entscheidung 5).
- **DWD pc_met / FlugWetter (echtes deutsches METAR):** kostenpflichtige,
  geschlossene Nutzergruppe — passt nicht zur quellenoffenen Grundhaltung.
  Zurückgestellt; NOAA liefert dasselbe QNH offen.
- **Wetter als Readiness-Voraussetzung:** würde das ASD an eine externe Quelle
  koppeln (fail-closed an der falschen Stelle). Verworfen.

## Konsequenzen

- **Neue Go-Pakete:** `pkg/weathertiles` (DWD-Radar-Tile-Proxy + Cache),
  `pkg/weather` (NOAA-METAR-Poller + QNH-Cache), Warnungen-Fetch (WFS-GeoJSON,
  OpenAIP-Muster).
- **Neue interne Endpoints:** `/api/weather/radar/{z}/{x}/{y}.png`,
  `/api/weather/warnings.geojson`, `/api/weather/qnh`.
- **Neue Konfiguration (12-Factor):** `WAYFINDER_DWD_WMS_URL`,
  `WAYFINDER_DWD_RADAR_LAYER`, `WAYFINDER_DWD_REFRESH`, `WAYFINDER_DWD_WARN_URL`,
  `WAYFINDER_METAR_URL`, `WAYFINDER_METAR_USER_AGENT`, `WAYFINDER_QNH_REFRESH`
  (jeweils Default im cfg-Literal; `Enabled` aus gesetzter URL abgeleitet).
- **Neue Feature-Keys:** `weather_radar`, `qnh`, `weather_warnings` in
  `pkg/feature/catalog.go` (getippt, fail-closed; pro Mandant schaltbar).
- **Neue Metriken:** `wayfinder_weather_fetch_success_total`,
  `wayfinder_weather_fetch_failures_total`, `wayfinder_weather_cache_age_seconds`
  (nach Quelle gelabelt). `metrics.Gauge` nimmt nur `int64` — QNH wird nicht als
  Float-Gauge exponiert, sondern nur über den REST-Endpoint (ganzzahlig hPa).
- **Neue Anforderungen** im Register: `FR-WX-001` (Radar-Overlay),
  `FR-WX-002` (QNH-Infobox), `FR-WX-003` (Warnungen-Overlay),
  `NFR-SEC-005` (Wetter-Vertrauensgrenze/robuster Wetter-Decoder).
- **Betrieb:** Das Deployment-Netz muss **ausgehend** `maps.dwd.de` und
  `aviationweather.gov` (HTTPS/443) erreichen dürfen — nach
  `docs/INSTALLATION.md`/`docs/BETRIEB.md`.

## Ehrliche Grenze

- Die konkreten Endpunkte (exakte GeoServer-Layer-Namen, tatsächlich angebotene
  EPSG:3857-Unterstützung, das METAR-`altim`-Feld) konnten in der
  Entwurfs-Umgebung **nicht live verifiziert** werden (Egress-Policy blockierte
  `*.dwd.de` und `aviationweather.gov`). Der Code ist deshalb **defensiv gegen
  Fehlschläge** gebaut (best-effort, leeres/transparentes Fallback), sodass ein
  falscher Layer-Name „nur" ein leeres Overlay ergibt, keinen Absturz; ein
  **Live-Smoke-Test aus offenem Netz** bleibt ein Deploy-Schritt.
- Wetter-Overlays und QNH sind **Orientierungs-/Kontext-Information**, **kein
  zertifizierter meteorologischer/aeronautischer Datensatz**. Für einen realen
  operativen Einsatz wäre eine zertifizierte Wetter-/QNH-Quelle nötig (analog zur
  „ehrlichen Grenze" von ADR 0004).
- Die QNH-Quelle ist ein **US-Government-Feed** (NOAA); die Entscheidung sichert
  die Verfügbarkeit des ASD-Kerns gegen dessen Ausfall, nicht die
  ununterbrochene Verfügbarkeit des QNH selbst.
