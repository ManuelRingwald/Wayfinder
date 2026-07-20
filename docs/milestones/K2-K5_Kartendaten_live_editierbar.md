# K2–K5 — Kartendaten live editierbar (Basiskarte · Wetter · Abdeckung · Aeronautik)

> **Register:** FR-CFG-009 (K2) · FR-CFG-011 (K3) · FR-CFG-010 (K4) ·
> FR-CFG-012 (K5) · **Epic:** #307 (Issues #310/#311/#312/#313). Baut auf **K0**
> (`pkg/mapconfig`, ADR 0033) und **K1** (Rahmen `AdminMapData.vue`) auf.

## Fachlich — warum

Nach dem Rahmen (K1, read-only Status) machen K2–K5 die vier Karten-Datenquellen
**im Admin live bedienbar**. Der Betreiber stellt Basiskarte, Wetter,
Radar-Abdeckung und den OpenAIP-Abruf direkt in der UI ein — ohne Deployment,
ohne Env-Datei anzufassen. Die Umgebungsvariablen bleiben als **Default/
Bootstrap** gültig; ein Override in der UI gewinnt zur Laufzeit.

## Überblick der vier Häppchen

| Häppchen | Reiter | Was der Admin einstellt | Wirkung |
|----------|--------|-------------------------|---------|
| **K2** (#310) | Basiskarte | Style-URL + Theme (`bkg`/`bkg-dark`) | **live** (Server holt Style neu) |
| **K3** (#311) | Wetter | DWD-Radar/-Warnungen/QNH: An/Aus + URL/Layer | An/Aus **live**, URL/Layer **Neustart** |
| **K4** (#312) | Radar-Abdeckung | Sensor-Liste (Lat/Lon/Min/Max/Label) + Ringfarbe | **live** (GeoJSON neu berechnet) |
| **K5** (#313) | Aeronautik | OpenAIP Fetch-Radius + Base-URL | **Neustart** (Key bleibt versiegelt) |

## Technisch

- **Plane `cmd/wayfinder/mapdata.go`** (`mapDataConfig`): jede Einstellung ist
  ein `mapconfig.Setting` (DB-Override ?? Env-Default). `mapConfigHandler` und
  der Coverage-GeoJSON-Handler lesen die **effektiven** Werte pro Request, sodass
  `/api/map-config` live die Overrides widerspiegelt.
- **Admin-Endpunkte** unter `/api/admin/mapdata/*`, alle hinter
  `tenantMW ∘ requireAdmin` (`RequireRole(admin)`). Generischer
  `mapconfig.Resource`-Handler (GET/PUT, leerer Wert = Reset) für Einzelwerte;
  ein eigener Coverage-Handler (GET/PUT/DELETE) für die Sensor-Liste.
- **K2 — Basiskarte (live):** `basemap.Service.Reload(url, dark)` forciert einen
  Re-Fetch des Styles und **behält die letzte gute Konfig** bei Fehler; ein
  Reload-Fehler wird ehrlich als `reload_error` (HTTP 200 = gespeichert, aber
  nicht angewandt) zurückgegeben. Theme validiert gegen `bkg`/`bkg-dark`,
  Style-URL via `ValidateFetchURL` (SSRF).
- **K3 — Wetter (An/Aus live, URL/Layer Neustart):** Die DWD/QNH-Abruf-Dienste
  sind sperrfreie Poll-Schleifen; ein Live-Umbau wäre unsicher. Darum wirken
  **An/Aus + Verfügbarkeit** sofort (`/api/map-config`), während geänderte
  **URLs/Layer** beim Boot angewandt werden: die Dienste werden aus den
  effektiven Werten neu gebaut (`weathertiles`/`weatherwarnings`/`weather`),
  bevor ihre Goroutinen starten. So stimmen Verfügbarkeits-Chip und Poller nach
  einem Neustart überein.
- **K4 — Abdeckung (live):** Sensor-Liste als JSON-Blob-Setting; ein
  **malformter** Override degradiert auf die Env-Sensoren (nie ein kaputtes
  Overlay). Validierung: ≤ 20 Sensoren, Lat/Lon-Bereich, Max > 0, 0 ≤ Min < Max.
  `DELETE` = Reset auf Env (verschieden vom leeren-Liste-Override „null
  Sensoren").
- **K5 — Aeronautik (Neustart):** Fetch-Radius (> 0, ≤ 5000 km) + Base-URL
  (SSRF-geprüft, leer = Anbieter-Standard). Die effektiven Werte werden **vor**
  dem Bau der OpenAIP-Dienste in `cfg` gefaltet, `mapData` behält die echten
  Env-Werte als Reset-Default. Der **API-Key bleibt versiegelt** (`pkg/secret`)
  und wird weiter im eingebetteten OpenAIP-Panel verwaltet.

## Frontend

`AdminMapData.vue` (aus K1) wird je Reiter vom read-only Status zum Editor
erweitert: Formularfelder, Speichern/Reset, „überschrieben/Standard"-Chips und
**ehrliche Neustart-Hinweise** dort, wo ein Wert erst beim Restart greift. Der
Aeronautik-Reiter behält das eingebettete `AdminGlobalOpenAIP`-Panel und bekommt
darüber die Radius/Base-URL-Felder.

## Sicherheit

- Admin-gesetzte, server-seitig gefetchte URLs → `ValidateFetchURL` (nur
  http/https, private/Loopback/Link-Local/Cloud-Metadaten abgelehnt).
- Secrets bleiben versiegelt und **nicht** auf dieser Plane.
- Alle Admin-Routen hinter `RequireRole(admin)`; kein CAT062-Bezug, der
  Track-Rechenpfad und dessen Determinismus bleiben unberührt (reine
  Konfig-Ebene).

## Tests / Gates

- Go: `cmd/wayfinder/mapdata_test.go` (effektives Theme/Sensoren/Wetter-
  Verfügbarkeit/OpenAIP inkl. Malform-Degradation + Reset; `validTheme`/
  `validateSensors`/`validBool`/`validRadiusKM`), `pkg/basemap/reload_test.go`
  (Reload wechselt URL + refetcht). `go test/vet/gofmt` grün.
- Frontend: `adminMapData.test.js` (Quell-Guards je Reiter: Endpunkte, Felder,
  Reset-Semantik, Neustart-Hinweise, Key-versiegelt). `vitest` + `vite build` +
  eingebettetes `dist` neu grün.
