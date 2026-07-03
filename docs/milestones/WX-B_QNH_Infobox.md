# WX-B — QNH-Infobox (NOAA-METAR)

> Häppchen B der Wetter-Erweiterung. Baut auf WX-0 (ADR 0016) und WX-A auf.
>
> **Nachtrag (CBD-3, ADR 0017):** Das hier beschriebene globale Env-Opt-in
> (`WAYFINDER_METAR_STATIONS`, „ohne Stationen aus") ist überholt. Die NOAA-Quelle
> ist jetzt **default-an** (`WAYFINDER_QNH_ENABLED`), der Flugplatz wird **pro
> Mandant** gesetzt (`view_configs.qnh_icao`), `/api/weather/qnh` ist tenant-scoped;
> `WAYFINDER_METAR_STATIONS` ist nur noch deprecated Fallback. Siehe
> `CBD-3_QNH_per_tenant.md`.

## Fachlich

QNH (Höhenmesser-Einstellung, hPa) ist eine operative Kernangabe. Eine kleine
„QNH 1013"-Anzeige in der Kopfzeile gibt dem Lotsen den aktuellen Wert des
Flugplatzes.

## Technisch — die ehrliche Lage

**QNH ist nicht im CAT062-Vertrag** (weder CAT062/065/063) und **nicht sauber in
offenen DWD-Daten**: die offenen DWD-Produkte (POI `PMSL`, MOSMIX `PPPP`) tragen
nur auf MSL reduzierten Druck (QFF/PMSL) — eine **andere physikalische Größe** als
QNH, und MOSMIX ist eine Vorhersage. Deshalb kommt QNH **ausschließlich aus einem
echten METAR**: **NOAA/NWS Aviation Weather Center** (Public Domain), Feld `altim`
(hPa), hilfsweise die `Qxxxx`/`Axxxx`-Gruppe aus `rawOb`.

**Backend (`pkg/weather`).**
- `client.go` — NOAA-METAR-Client: `FetchMETAR(ids)` mit distinktivem User-Agent
  (Pflicht, sonst AWC-403), `io.LimitReader`, tolerantem Decode; QNH-Extraktion
  `altim` → sonst `Q`/`A`-Regex (inHg→hPa); Plausibilitätsgrenze 850–1100 hPa;
  QNH-lose/fehlerhafte Records werden übersprungen, nicht der ganze Abruf.
- `service.go` — Poll-Loop (Default 15 min) über die konfigurierten Stationen,
  Per-Station-Last-Good-Cache, Stale-Logik (Obs älter als `StaleAfter`, Default
  2 h), `Handler` → `GET /api/weather/qnh` (`{stations,primary}`), ganzzahlige
  hPa-Rundung.

**Verdrahtung (`cmd/wayfinder/main.go`).** Config `WAYFINDER_METAR_STATIONS`
(Kommaliste, Prioritätsreihenfolge — erster = Kopfzeile) / `_URL` / `_USER_AGENT`
/ `WAYFINDER_QNH_REFRESH`; Service + `go weatherQNH.Run(ctx)`; Route hinter
`tenantMW`; Metriken `wayfinder_weather_*{source="noaa_metar"}`.

**Feature-Gate.** Feature-Key `qnh`; die Kopfzeile zeigt die QNH-Anzeige nur bei
Entitlement (`session.hasFeature('qnh')`) und wenn der Backend-Poller eine Lesung
hat.

**Frontend (Path „polled REST").** `stores/weather.js` (5-min-Poller über
`apiFetch`), `AsdHeader.vue` rendert „QNH ‹hPa›" mit Stale-Ausgrauung/`*`-Markierung
und Tooltip (ICAO + Wert). Kein WS-Push nötig (QNH ändert sich ~stündlich).

## Offene Design-Entscheidung (bewusst)

Die Stationen sind **global per Env** konfiguriert (nicht pro Mandant). Eine
per-Tenant-Flugplatz-Auswahl (DB-Feld + Admin-UI) ist eine sinnvolle Folgearbeit,
würde WX-B aber um eine Migration + UI aufblähen — daher hier bewusst
zurückgestellt. Der Endpunkt ist authentifiziert (`tenantMW`); QNH ist ohnehin
öffentliches Wetter.

## Schnittstellen-Wirkung

**Keine.** Kein CAT062/CAT065/CAT063-Eingriff, keine Firefly-Koordination. Neue
ausgehende Abhängigkeit zu `aviationweather.gov` (Vertrauensgrenze, ADR 0016).

## Gates

- `go build/vet/gofmt` sauber; `go test ./...` grün (neue `pkg/weather`-Tests:
  Parsing inkl. inHg + Plausibilität, `altim`-vor-`rawOb`, Fetch-Skip/UA/ids,
  Non-200/Garbage, Snapshot/primary/Stale/Rundung, Last-Good, disabled,
  `normaliseStations`).
- vitest grün (`weather`-Store); `vite build` → `internal/webui/dist` neu eingebettet.

## Ehrliche Grenze

Der NOAA-Endpunkt (`altim`-Feld) konnte in dieser Umgebung nicht live verifiziert
werden (Egress-Policy). Der Poller ist defensiv (kein Feld/kein QNH ⇒ keine
Anzeige, kein Absturz); ein Live-Smoke-Test aus offenem Netz bleibt ein
Deploy-Schritt. Die QNH-Quelle ist ein US-Government-Feed.
