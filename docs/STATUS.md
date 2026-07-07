# Arbeitsstand (Handover-Notiz) — Wayfinder

> **Zweck:** Diese Datei beschreibt den **aktuellen IST-Stand** von Wayfinder.
> Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

> 🗺️ **Roadmap & Arbeitspakete:** siehe `docs/ROADMAP.md` in **diesem Repo**
> (zentrale Quelle für Wayfinder **und** Firefly). Cross-Project-Abhängigkeiten
> in `docs/cross-project/todo-for-wayfinder.md`.

---

## 🎯 Stand 2026-07-07 (ASD-014 Slice 1 — OpenAIP-Transform-Anreicherung für AoR)

- **ASD-014.1 — OpenAIP-Transform-Anreicherung (Backend-Vorbau, FR-AERO-002):**
  Der OpenAIP→GeoJSON-Transform (`pkg/aeronautical/client.go`) führt für
  **Lufträume** jetzt zusätzlich mit: **`id`** (stabile OpenAIP-`_id` — robuste
  Referenz für die AoR-Auswahl, Option 1), **`icao_class`** (numerisch) und die
  **Vertikalgrenzen `lower`/`upper`** als `{value, unit, referenceDatum}`-Objekt.
  Additiv/rückwärtskompatibel (nur `kind==airspace`, fehlende Felder weggelassen;
  Navaid/Waypoint unverändert). **Höhenbänder datum-vollständig** gehalten (nie zu
  einer Zahl verrechnet) → bereit für Höhen-Filter und späteres 3-D (3-D wird
  jetzt **nicht** gebaut).
- **Neuer Typ** `openaipLimit` + `properties()`-Airspace-Block; neue Tests
  `TestFetchEnrichesAirspaceProperties`, `TestEnrichmentFieldsAreAirspaceOnly`.
  Doku: Milestone `ASD-014.1`, FR-AERO-002, TECHNICAL `/api/airspace`.
  Gates grün: `go test ./...`, `go vet`, `gofmt`, `golangci-lint` (0 issues).
- **Nächster Schritt (noch nicht freigegeben):** **Slice 2** — AoR-`_id`-Liste pro
  Mandant (View-Config) + `/api/airspace`-Tagging (`aor: true`); danach **Slice 3**
  Frontend-Highlight.

## 🎯 Stand 2026-07-07 (#208 — Admin ohne eigenes ASD, ADR 0022; Serie komplett)

- **ADR 0022 — Admin ohne eigenes ASD + pfad-unabhängiges Passwort-Gate
  (Issue #208, Anker der Serie #208–#212; NFR-SEC-006):** Server-hart umgesetzt
  (Option A, Freigabe des Betreibers; S4, umgesetzt auf Fable 5):
  - **Server:** `tenant.RequirePasswordChanged` weist bei gesetztem
    `must_change_password` **alle** operativen Daten-Pfade ab (`/ws`,
    Aero-Overlays, Wetter, Airports/Runways; `403 password_change_required`) —
    der `/`-Login kann den Zwangs-Passwortwechsel nicht mehr umgehen. Der
    `/ws`-Scope-Resolver lehnt einen **Admin ohne aktives Gastmodus-Grant**
    fail-closed ab (403 + Audit `ws_admin_denied`); der frühere Fallback
    „leeres eigenes Bild" (TenantID 0) entfällt — auch bei abgelaufenem Grant
    und deaktivierter Impersonation.
  - **Frontend:** `adminGate` in `AsdView` — must-change-Principals und Admins
    ohne Gastmodus werden von `/` nach `/admin` umgeleitet (Spinner hält, bis
    entschieden; kein totes `/ws`); TTL-Ablauf des Grants → Drop-Handler kehrt
    nach `/admin` zurück; Gastmodus-„Beenden" → `/admin`; „Zur Lage"-Shortcut
    der Admin-App-Bar entfernt. `session.mustChangePassword` aus dem whoami.
  - **Altstand:** bereits durch Migration 00007 bereinigt (admin XOR tenant,
    CHECK-Constraint) — keine neue Migration.
  - Doku: ADR 0022, NFR-SEC-006 im Register, TECHNICAL.md (Admin/Nutzer-
    Trennung + Gate-Semantik), INSTALLATION.md (4.7-Hinweis, 4.11 „Beenden" →
    Verwaltung). Gates: `go test`/`vet`/`gofmt` grün, **vitest 415** grün
    (neu `asdAdminGate`, Resolver-Tests auf neue Semantik), dist neu gebaut.
  - Damit ist die **Admin-/Mandanten-UX-Serie #208–#212 vollständig**.

## 🎯 Stand 2026-07-07 (ADR 0021 Nachtrag — Datenquellen-Bewertung A/B/C für AoR)

- **ADR-0021-Nachtrag „Datenquellen-Bewertung (A/B/C)" (rein dokumentarisch):**
  Geprüft, ob neben OpenAIP auch EuroScope-Sectorfiles oder DFS-AIP als Quelle
  der AoR-Geometrie taugen (recherchiert/verifiziert):
  - **A OpenAIP** — jetzt gewählt (CC BY-NC); liefert `type`, `icaoClass`,
    Floor/Ceiling **und stabile `_id`** (unser Transform verwirft das heute noch).
  - **B EuroScope-Sectorfiles** — **verworfen**: Lizenz (nur außerhalb des
    Controller-Clients mit Zustimmung; keine Open-Lizenz) + sim-adaptiert, nicht
    AIRAC-zertifiziert.
  - **C DFS-AIXM** — **Produktionsziel**: autoritativ, aber Backend-Pipeline
    (~3–5 Tage) + DFS-Lizenzklärung. **Nachverfolgt: Issue #215 + Roadmap ASD-015.**
- **Festlegungen:** Auswahl-Semantik = **Option 1** (explizite, pro-Mandant
  konfigurierte `_id`-Liste). Vorbau: OpenAIP-Transform um `_id`/Floor-Ceiling/
  `icaoClass` erweitern. Roadmap um **ASD-014** (AoR-Overlay) + **ASD-015**
  (DFS-AIXM, #215) ergänzt.
- **Nächster Schritt (noch nicht freigegeben):** Slice-Ankündigung für ASD-014
  (Transform-Ausbau + AoR-Liste + Frontend-Highlight), dann Bau nach „Go".

## 🎯 Stand 2026-07-07 (Admin-/Mandanten-UX-Überarbeitung — 4 von 5 Häppchen)

Auf Basis von fünf neu angelegten Issues (#208–#212) den Admin-/Mandanten-Bereich
umgebaut. Vier Frontend-Häppchen umgesetzt (je eigener Commit), reine UI/UX-Arbeit
— **CAT062-Draht-Vertrag unberührt**:

- **#212 — Anbieter-Dropdown neben das Typ-Feld:** Im Feed-Quellenkonfigurator
  (`AdminFeeds.vue`) sitzt das Anbieter-Select für „ADS-B (Community-Aggregator)"
  jetzt direkt neben „Quell-Typ" statt über der Poll-Zeit; Failover-Hinweis als
  Caption. (S2)
- **#210 — Feeds/OpenAIP/Nutzer in die Übersicht:** Der überladene Konfig-Dialog
  ist entschlackt. Feeds, OpenAIP und Zugänge sind aus `AdminTenantDetail`
  herausgelöst und liegen in der Mandanten-Übersicht (`AdminTenants.vue`) je in
  einer eigenen Spalte mit Konfig-Icon (⚙ `mdi-cog-outline`), das einen
  fokussierten Dialog öffnet. Neue Komponente `AdminTenantOpenAIP.vue` (Auslagerung
  des Inline-Blocks). Feed-Refresh-bei-Änderung wandert mit. (S3–S4)
- **#211 — globaler Speichern + Abbrechen:** Die schlanke Detailseite
  (Standard-Ansicht + Features) speichert global. Feature-Toggles werden lokal
  gepuffert (`featureEdits`) und erst beim „Speichern" persistiert/aktiv — vorher
  schaltete ein Toggle sofort frei. Speichern → zurück in die Übersicht; Abbrechen
  → zurück ohne Wirkung. (S3)
- **#209 — Gastmodus nur über Augen-Icon:** Read-Only-Einblick (Impersonation,
  ADR 0008) startet nur noch über ein Augen-Icon (`mdi-eye-outline`) in der neuen
  Spalte „Gastmodus" der Übersicht. Detail-Button und Start-Menü der
  `ImpersonationBar` entfernt; die Bar rendert nur noch als aktives
  Read-Only-Banner. (S2–S3)

Doku: `INSTALLATION.md` (Admin-Oberfläche, Schritte 4.7/4.8b/4.9/4.11) auf die
neuen Bedienwege gezogen. Gates: **vitest 409 grün** (neue Tests
`adminTenantsConfigColumns`, `adminTenantDetailSave`, `adminGuestModeEntry`;
Aggregator- und Provisioning-Refresh-Test nachgezogen), `vite build`,
`go test ./...`, `go vet`, `gofmt` grün; `dist` neu eingebettet.

**Offen — #208 (Anker, S4, sicherheits-/architektur-relevant):** Der erzwungene
Passwortwechsel greift nur unter `/admin`, nicht beim `/`-Login; der Admin soll
kein eigenes ASD mehr haben (nur noch Read-Only via Gastmodus). Braucht eine
Design-Entscheidung + ADR (Charter §10) und Server-Änderungen — **vor der
Umsetzung abzustimmen**.

## 🎯 Stand 2026-07-06 (ADR 0021 — Geografie-Begriffsmodell AoR/AoI/Kartenrahmen)

- **ADR 0021 — AoR vs. AoI/Track-Scope vs. Kartenrahmen (rein dokumentarisch):**
  Auf Betreiber-Frage („zeigen wir dem Nutzer einen *Radius*, obwohl ein
  Flughafen-ANSP *Gebiete* verantwortet?") die drei heute vermischten
  geografischen Ebenen sauber benannt und getrennt:
  1. **Track-Scope = Area of Interest (AoI)** — Daten-/Sichtfeld, bewusst *größer*
     als der Verantwortungsbereich; heute die `view_configs.AOI`-BBox + FL-Band
     (WF2-21.2) → `FIREFLY_COVERAGE_BBOX` (ADR 0012). **Das ist der „Radius, der
     nur die Tracks betrifft".**
  2. **Verantwortungsbereich = Area of Responsibility (AoR)** — CTR/TMA als
     hervorgehobenes Overlay, Quelle **OpenAIP** (ADR 0004), **kein** Track-Filter.
  3. **Kartenrahmen** — Center/Zoom, Range Rings, FL-Filter (reine Darstellung).
  Fachlicher Kern: **„sehen ≠ besitzen"** (SKYbrary AoR/AoI). Neuer ADR +
  Glossar-Begriffe (AoR, AoI/Track-Scope, CTR, TMA, CTA, ATZ). **Kein Code, keine
  neuen Env-Variablen, kein Register-Eintrag** — die funktionale Anforderung
  entsteht erst mit dem Folge-Häppchen.
- **Nächster Schritt (noch nicht freigegeben):** flughafengebundene,
  hervorgehobene **AoR-Overlay-Ebene** auf OpenAIP-Basis (Beispiel EDDH). Offene
  Design-Frage dort: **Auswahl-Semantik** — welche OpenAIP-Lufträume gelten als
  AoR eines Flughafens (explizite Namens-/ID-Liste pro Mandant, ggf. per
  räumlichem Test vorbefüllt), plus optional `center+radius→BBox` für die AOI.

## 🎯 Stand 2026-07-06 (CAT063 per-Quelle-Fehlergrund H4 → schließt #197)

- **ADR 0020 — CAT063 `SRC-REASON` dekodieren + Feed-Health-Chip zeigt den Grund
  (additiv, Fireflys ICD 3.1.0/ADR 0033):** Der CAT063-Decoder **liest** jetzt das
  I063/RE-Feld (`[LEN][SUBFIELD=0x80][SRC-REASON]`) statt es nur zu überspringen →
  `SensorStatus.Reason` ∈ {`unreachable`,`auth`,`rate_limited`,``}.
  `cat063.DominantReason` verdichtet auf den dominanten Grund (Priorität
  `auth`>`rate_limited`>`unreachable`); er fließt über
  `RecordSensors(…, reason)` → `FeedSnapshot.DegradedReason` →
  `FeedStatusMessage.degraded_reason` (WS + Admin-Endpoint) → ASD-Store
  `feedDegradedReason` → **`FeedStatusChip`**: `SENSOR AUSFALL · NICHT ERREICHBAR`
  / `· AUTH-FEHLER` / `· RATENLIMIT` + Tooltip. Grund beeinflusst die Farbe nicht.
  Der Betreiber sieht damit **warum** eine Quelle still ist (Firewall vs. falsche
  Credentials vs. Ratenlimit) — **schließt #197**. Rein additiv, kein
  Lockstep-Zwang (älterer Firefly ohne RE → Chip wie bisher). Neue Decoder-/Store-/
  Chip-Tests; FR-DATA-006, Milestone WF-CAT063, ADR 0020. `go test ./...`,
  `go vet`, `gofmt`, `golangci-lint`, `vitest` (397) grün.

## 🎯 Stand 2026-07-06 (CAT063-UAP-Standardisierung H2, lockstep zu Firefly ADR 0032)

- **ADR 0019 — CAT063-Decoder auf Standard-UAP (ICD 3.0.0, BREAKING, lockstep):**
  Wayfinders CAT063-Decoder zieht Fireflys UAP-Korrektur (ADR 0032) nach. Der
  Record folgt jetzt der echten EUROCONTROL-UAP: FSPEC `0xB8`, I063/010 =
  **SDPS**-Identität (25/2), **NEU** I063/050 = **Sensor**-Identität (SAC 0,
  SIC = `sensor_id`), I063/030@FRN3, I063/060@FRN5 (CON, variabel via FX).
  `SensorStatus.SAC`/`.SIC` = Sensor (aus I063/050), neu `.SDPSSAC`/`.SDPSSIC`
  = SDPS. **Vorwärtskompatibel:** kennt die Längen der übrigen Standard-Items
  (I063/015, I063/070–092) und überspringt RE (FRN 13) / SP (FRN 14) über ihr
  Längen-Oktett — Fundament für den per-Quelle-Fehlergrund im RE-Feld (Fireflys
  ADR 0033 → H4, Fixes #197). Byte-genaue Referenz-Vektoren + 3 neue Tests
  (StandardFSPEC, SkipsReservedExpansion, RejectsSpareFRN). Konsument-Verdrahtung
  (Health-Registry, gelbes Banner) unberührt — sie wertet nur `Operational` aus.
  **Deploy-Kopplung:** zusammen mit Firefly ADR 0032 ausrollen (Firefly #55).
  `go test ./...`, `go vet`, `gofmt`, `golangci-lint` grün; FR-DATA-006, Milestone
  WF-CAT063, ADR 0010-Nachtrag aktualisiert.

## 🎯 Stand 2026-07-06 (#194 Responsive — Häppchen 3 + 4, abgeschlossen)

- **#194 vollständig (ASD iPhone/iPad/24″ + Admin):** Die restlichen zwei
  Häppchen umgesetzt, damit ist das Issue zu.
  - **Häppchen 3 (24″/Desktop):** Auf dem Vuetify-`xl`-Band (≥1920px) atmen die
    ASD-Overlays token-getrieben — `--wf-overlay-gap` 12→20px und die Overlay-
    Breiten (`--wf-overlay-legend-width` 232→268px, `--wf-overlay-detail-width`
    292→336px) je eine Stufe größer. Alle Rand-Abstände (Top-Right-Cluster,
    Scope-Legende, Map-Controls, Track-Detail-Karte) lesen den Gap-Token, sodass
    die eine Media-Query-Stufe jede Ecke erreicht statt hartem 12px.
  - **Häppchen 4 (Admin):** Content-Spalte weitet auf `xl` von 1180→1440px;
    alle Admin-Dialoge kappen auf schmalen Phones via `max-width: min(<px>, 94vw)`
    (ein 460–720px-Dialog lief sonst auf 360px über). Dichte Tabellen scrollen
    bereits seit Häppchen 1 horizontal im Card (`.v-table__wrapper`).
  - Token-Stufen im echten Browser verifiziert (Playwright: 24″ → gap 20px/
    Legende 268px, iPad → rail 76px, Desktop kompakt). Reine Layout/CSS,
    CAT062 unberührt. Vitest **390 grün** (Häppchen-1-Breiten-Test auf die
    Tokens nachgezogen, 3 neue Fälle); dist neu gebaut.

## 🎯 Stand 2026-07-06 (#194 Responsive — Häppchen 2: iPad-ASD)

- **ASD auf dem iPad touch-optimiert (#194 Häppchen 2):** Auf dem
  Vuetify-`md`-Band (960–1279px, iPad-Landscape) wächst die Navigationsschiene
  von der kompakten 56-px-Desktop-Leiste auf **76 px** mit **44-px-Touch-Zielen**
  und **24-px-Icons**; das Sekundär-Panel öffnet auf **304 px** (Design-Mockup).
  `lg`+ (Desktop, iPad-Pro) behält die kompakte Leiste. Umsetzung
  **token-getrieben**: `--wf-nav-rail-width` (base.css-Media-Query) treibt die
  Schienenbreite; die schwebenden Overlays (Scope-Legende, Track-Detail-Karte)
  leiten ihren Links-Offset daraus ab (`calc(rail + gap)` = 68 px Desktop /
  88 px iPad) statt hartem `68px` — sie wandern in Lockstep mit der Schiene.
  Map-Controls bekommen auf dem `md`-Band ebenfalls 44-px-Buttons. Kern im
  echten Browser verifiziert (Playwright: iPad 1194px → 76px, iPhone/iPad-Pro/
  24″ → 56px, sauberer Boot). Reine Layout/CSS-Arbeit, CAT062 unberührt.
  Vitest **386 grün** (5 neue Fälle in `responsive.test.js`, `trackSymbology`-
  Test nachgezogen); dist neu gebaut. **Offen bleiben Häppchen 3** (24″-Overlay-
  Skalierung) **und 4** (Admin-Tabellen als Card/Stack). (S3–S4)

## 🎯 Stand 2026-07-06 (Codespace-Deploy härten)

- **Veraltetes `firefly:latest` → stumme Crash-Loop-Feeds (Kern-Fix):**
  `.devcontainer/start.sh` baute das gespawnte Tracker-Image nur, *wenn es fehlte*,
  und cachte es danach für immer. Sobald Fireflys `main` einen neuen Quelltyp
  bekommt (hier `adsb_aggregator`, v1.5.0), lehnt der alte Tracker das
  `FIREFLY_SOURCES`-JSON ab (`unknown variant`), crash-loopt und der Feed wird nie
  grün — keine Tracks, ohne sichtbaren Fehler in der UI. Jetzt: bei **jedem**
  Start `git -C ../firefly pull --ff-only` + `docker build` (Layer-Cache ⇒ No-op in
  Sekunden, wenn Firefly unverändert) und danach **Neu-Spawn** der Tracker
  (`docker rm` der `wayfinder.managed`-Container; der Spec-Hash hängt nur am
  Image-*Namen*, nicht am Digest, sonst bliebe der alte Container hängen).
  Rebuild-Fehler sind **nicht-fatal** (Rückfall auf vorhandenes Image + laute
  Warnung), damit ein rotes Firefly-`main` nicht die ganze UI blockiert. (S2)
- **404 auf der Codespace-URL nach dem Aufwachen (Diagnose + Doku):** Ursache ist
  der beim Idle-Resume verwaiste **Port-Forwarding-Tunnel** (Panel-Einträge
  bleiben, Edge routet nicht → 404 für jeden Port, egal Private/Public; App selbst
  liefert lokal `200`). Fix: **F1 → „Developer: Reload Window"** (baut den
  Tunnel-Client neu auf). Globus-Klick/Port-neu-anlegen fassen nur die
  Registrierung an, nicht den Tunnel. Als `## 5. Fehlerbehebung` in
  `docs/CODESPACES.md` dokumentiert (inkl. Stale-Image-Fall + Desktop/`gh`-Umgehung).
- CAT062/Draht-Vertrag **unberührt** — reiner Deploy-/Harness-Pfad.

## 🎯 Stand 2026-07-06 (#201 ADS-B ohne Zugang — Community-Aggregator)

- **Quell-Typ `adsb_aggregator` (Firefly-Kontrakt v1.5.0, ADR 0031 dort, #201):**
  ADS-B jetzt auch **ohne Zugangsdaten** über adsb.lol (Default) / adsb.fi —
  zweiter Bezugsweg **neben** OpenSky (kein Ersatz), nutzbar aus Umgebungen mit
  Datacenter-IP-Sperre (Codespaces-Diagnose 2026-07-05: OpenSky droppt
  Azure-IPs). Store: neue Konstante + `isPolled` + `provider`-Whitelist
  (`adsb_lol`/`adsb_fi`; airplanes.live bis zur Verifikation der
  Radius-Einheit zurückgestellt), `poll_interval_secs` gilt für beide
  gepollten Typen. Orchestrator: `provider`-Pass-through nach
  `FIREFLY_SOURCES`, **kein** `cred_env` (auth-frei). UI: Typ
  „ADS-B (Community-Aggregator)" mit Anbieter-Select (Labels adsb.lol/adsb.fi,
  Wire-Werte bleiben intern), Poll-Feld + Höflichkeits-Infobox, **kein**
  Credential-Block. Firefly-Seite zuvor gemergt (PR #54, Issue #53 zu).
  CAT062-Draht unberührt. (S3, Häppchen 2 zu Firefly ADR 0031)

## 🎯 Stand 2026-07-05 (#194 Responsive — Häppchen 1)

- **ASD + Admin responsive (iPhone/iPad/24″), Design-Mockup umgesetzt (#194):**
  - **Safe-Area-Fundament:** `viewport-fit=cover` (index.html) + `--wf-safe-*`/
    `--wf-bottom-nav-h`/`--wf-touch-min` in `base.css`.
  - **iPhone/Tablet-Portrait:** neue **Bottom-Tab-Leiste** (`BottomNav.vue`:
    Scope/Filter/Konto[/Admin]) ersetzt Hamburger+Drawer; Filter/Konto als
    **Bottom-Sheets**; Track-Detail-Sheet (bereits vorhanden); Zoom in den
    **Map-Controls** über der Leiste; Messwerkzeuge in den Filter-Sheet verlegt.
  - **iPad-Landscape/Desktop (≥md):** Navigationsschiene+Panel unverändert.
  - **Fluide Overlays** (`min()`), Safe-Area an Top-Cluster/Legende/Controls.
  - **Admin:** Appbar responsiv (Sektions-Select + Icon-only-Aktionen auf klein),
    dichte `v-table`s scrollen horizontal im Card (`base.css`), fluider Container.
  - Tests: neuer `responsive.test.js` (10), `railTools`-Test nachgezogen; Vitest
    **368 grün**; Playwright-Boot-Check (iPhone/iPad/24″) fehlerfrei; dist neu
    gebaut. Reines Frontend/Layout, CAT062 unberührt. (S4, Häppchen 1)

## 🎯 Stand 2026-07-05 (Runways, #192 abgeschlossen)

- **#192 Runways nachgezogen (zweite Hälfte):** Der OurAirports-`runways.csv`
  ist jetzt über `raw.githubusercontent.com` erreichbar (der zuvor geblockte
  Host `davidmegginson.github.io` war das Problem). Generator
  `pkg/airport/gen/runways.go` → eingebettete `pkg/airport/runways.tsv`
  (10.328 Runways, ICAO-Aerodrome, nicht geschlossen, beide Schwellen).
  Runtime-Loader `pkg/airport/runways.go` (`RunwaysInBBox`), AOI-gescopter,
  feature-gegateter Endpoint `GET /api/runways.geojson` (`runways`-Entitlement),
  Frontend Line-Layer `addRunwayLayers` + Sidebar-Toggle. Tests: `RunwaysInBBox`
  (EDDH = 05/23 + 15/33), Katalog-Count 13; Vitest 360; dist neu gebaut.
  Damit ist **#192 komplett** (Flughafen-Marker aus PR #193 + Runways).

## 🎯 Stand 2026-07-05 (Sammel-PR #182–#192)

- **Batch #182–#192 umgesetzt (ein PR):**
  - **#182** Label-Drag hält den Anfasspunkt unter dem Cursor (kein Sprung).
  - **#183** Ausgewählter Track mit cyaner Eck-Klammer-Box (ATC-Look) statt Ring.
  - **#184** Track-Detail-Panel kollisionsfrei oben links (kein Feed-Badge/OSM-Overlap).
  - **#185** FLARM als eigenes Dreieck-Symbol (Form = Herkunft) statt Buchstabe „F".
  - **#186/#188** Rail-Icons an ASD-Vorlage (Lupen-Zoom, Tune-Filter).
  - **#187** Kompaktere Layer-Toggles, kleinere Labels, größere Überschrift.
  - **#191** History-Dots nach Dauer konfigurierbar + Alters-Ausfaden (Zeitstempel
    per `time_ms`, Retention-Fenster, `historyConfig`-Store + Sidebar-Auswahl).
  - **#189/#190** DWD-Wetter-Overlays auf Mandanten-AOI geclippt (`whoami.aoi`;
    Radar via `source.bounds`, Warnungen via Sutherland-Hodgman `clip.js`),
    Legenden für Radar/Warnungen im Panel, Radar-Style konfigurierbar
    (`WAYFINDER_DWD_RADAR_STYLE`). Echo-only-DWD-Style offline nicht verifizierbar.
  - **#192 (Teil)** Flughafen-Referenzpunkt-Layer (offline OurAirports,
    `/api/airports.geojson`, AOI-gescoped, feature-gegated `airport`).
    **Runways offen:** OurAirports-`runways.csv`-Host per Proxy geblockt (403) →
    keine echte Runway-Geometrie einbettbar (Charter: keine Fake-Daten).
  - Tests: Vitest 360 grün, `go test ./...` grün, `vet`/`gofmt` sauber; dist neu gebaut.

- **Bugfix #179: Airspace-Overlay zeigte nach Re-Login initial „ganz
  Deutschland".** Nach Logout→Login / Mandantenwechsel / Session-Ablauf→Re-Login
  im selben Tab (ohne Full-Reload) rendern die Airspace-Layer zunächst **alle**
  OpenAIP-Typen — auch die nicht in `AIRSPACE_GROUPS` gemappten, landesweiten
  (UIR/FIR/ADIZ/TRA …) — bis zum ersten Gruppen-Toggle. Ursache: Die einmalige
  Anwendung des Type-Filters hing an der `false→true`-Flanke von
  `store.mapLoaded`; der Store ist ein Singleton und `mapLoaded` eine
  „write-once-true"-Latch, die beim zweiten Mount bereits `true` ist → Watcher
  feuert nicht → Filter läuft initial nie. Fix: (1) `updateAirspaceFilter()` wird
  jetzt direkt im Engine-Load-Handler nach `setMapLoaded(true)` aufgerufen — der
  Engine initialisiert seine Layer-Filter auf **jedem** Mount selbst,
  unabhängig von der Store-Flanke; (2) `destroy()` setzt `setMapLoaded(false)`
  zurück (Hygiene für weitere flanken-gekoppelte Effekte). Rein
  Frontend/Reaktivität, CAT062-Vertrag unberührt. Tests: Regressions-Test in
  `mapCanvasViewCenter.test.js` (Vitest 352); dist neu gebaut. (S2–S3)

## 🎯 Stand 2026-07-04 (Abend)

- **E2E-Fix: ASD-Karte öffnet auf dem Mandanten-Sektor (FR-UI-013-Nachtrag).**
  Befund im Codespace-Testlauf: Mandant EDDH/Hamburg konfiguriert, Karte
  zentrierte aber auf Frankfurt. Ursache: `/api/map-config` liefert das Zentrum
  aus der globalen `WAYFINDER_MAP_CENTER_*`-Env (Default Frankfurt); die
  Mandanten-Ansicht speiste nur `icao`/`fl_min`/`fl_max` ins `whoami`, **nicht**
  Zentrum/Zoom — daher Kopfzeile korrekt „EDDH", Kamera falsch auf Frankfurt.
  Fix: `whoami` liefert jetzt `center_lat`/`center_lon`/`zoom` der effektiven
  Ansicht (`omitempty`; keine View-Config → Env-Fallback, nie 0/0); Frontend
  positioniert die Karte darauf (`initMap(initialCenter)`), „Neu zentrieren" +
  Range-Ringe folgen (`effectiveCenter`), Ansicht-Wechsel re-zielt
  (`applyViewCenter`). Tests: whoami-DTO (Go), session/`viewCenter` +
  MapCanvas-Verdrahtung (Vitest 334); dist neu gebaut. Eigener PR/Issue.

## 🎯 Stand 2026-07-04

- **Zuletzt aktualisiert:** 2026-07-04
- **Demo-Ausbau nachgezogen (Fireflys ADR 0030, Wayfinder-Teil):** Der
  Orchestrator-Platzhalter `WAYFINDER_FIREFLY_SCENE` entfällt — ein Feed
  **ohne** Quellen bekommt die explizite leere Liste `FIREFLY_SOURCES=[]` und
  spawnt einen Firefly mit ehrlich leerem Himmel + CAT065-Heartbeat (kein
  `FIREFLY_MODE` mehr). `docker-compose.bridge.yml` (komplett szenen-basiert)
  entfernt; VM-loser Weg ist der Codespace. `e2e-orchestrated.sh`: Modus
  `scene` → `empty` (Prüfpunkt 5 asserted den Heartbeat statt Tracks).
  Doku-Sweep: DOCKER/INSTALLATION (Compose-Beispiele auf Opt-in-OpenSky),
  E2E-ABNAHME (Teil 4 + Anhang A), CODESPACES, TECHNICAL, FR-ORCH-002/007,
  CLAUDE.md §2 (I062/100-Referenzpunkt: ADR 0021 statt Demo-Ursprung).
  **Zero-Touch-Prüfung:** UI-Kette (Feed + Quellen + Creds per Admin-UI →
  Auto-Spawn) verifiziert env-frei — `FIREFLY_SOURCES` setzt `enabled` hart;
  die Opt-in-Flags betreffen nur den Handstart. Offen: Auto-Generierung von
  `WAYFINDER_SECRET_KEY` im rohen orchestrierten Compose (Folge-Häppchen,
  damit auch die Zugangsdaten-Eingabe auf jungfräulichen Instanzen
  zero-touch ist).
- **Impersonation vervollständigt (B1, ADR 0008 Nachtrag):** „Als Mandant
  ansehen" schaltete bisher nur den `/ws`-Strom auf den Ziel-Mandanten um; alle
  REST-Pfade (whoami → Features/Legende/FL/ICAO, Aero-Overlays, QNH)
  antworteten weiter für den mandantenlosen Admin → nackte Karte. Jetzt stempelt
  `impersonationReadMW` (identische fail-closed-Semantik wie `/ws`) den
  effektiven Lese-Mandanten in den Kontext; whoami/Aero/QNH lösen gegen den
  Ziel-Mandanten auf, `impersonated_tenant_id` legt es offen. Identity und alle
  Schreibpfade unberührt.
- **B2 — Einstieg in der Admin-UI:** „Als Mandant ansehen"-Button auf der
  Mandanten-Detailseite (mintet das Grant, springt zur Karte; Fehler-Alert bei
  fehlgeschlagenem Mint). Die Funktion ist damit dort auffindbar, wo Admins sie
  suchen — nicht mehr nur über die Bar auf der Karte.
- **A — Auto-Seed ohne Komfort-Mandant (ADR 0011 Nachtrag):** Der Boot-Seed
  legt nur noch den tenant-losen Standard-Admin an; der Mandant `default`
  entfällt (seit ONB-4 redundant, stiftete Verwirrung). Frische Instanzen
  starten mit null Mandanten; Bestandsinstallationen unberührt (dortigen
  `default` bei Bedarf per UI löschen).
- **Codespaces-Testumgebung (Browser-only, orchestriert):** `.devcontainer/`
  startet den **orchestrierten Stack** (`docker-compose.orchestrated.yml`:
  Postgres + Wayfinder + Orchestrator; **Auto-Spawn je Feed** funktioniert,
  weil ein Codespace ein Linux-Host mit docker-in-docker ist — ein
  Netz-Namespace, Multicast lokal zugestellt). Betreiber-Vorgabe: Mandanten
  anlegen + Auto-Spawn müssen testbar sein, die Frankfurt-Demo ist Altlast
  (Ausbau angekündigt, wartet auf Go). `start.sh` baut das Firefly-Image aus
  dem Sibling-Checkout und erzeugt eine Codespace-lokale `.env`
  (Session-/Secret-Key, gitignored). Port 8081 = private HTTPS-URL
  (GitHub-Login + builtin-Auth). Anleitung: `docs/CODESPACES.md`.
  **Ausstehend:** E2E-Check der Impersonation + #159 (VM oder Codespace).
- **Teil 1 des E2E-Befunds gemergt (PR #158):** Die Luftraum-Overlay-Endpunkte
  (`/api/airspace|navaids|waypoints`) erzwingen das Feature-Entitlement jetzt
  **server-seitig** (leere Collection ohne Entitlement). Details siehe
  Stand 2026-07-02 unten.
- **Teil 2 als Issue geparkt: [#159](https://github.com/ManuelRingwald/Wayfinder/issues/159)**
  (Radius/AOI wird beim OpenAIP-Abruf nicht berücksichtigt). Verifikation
  wartet auf die Test-VM; im Issue stehen Diagnose-Stand, die zwei
  Hypothesen (H1 anderer Mandant / H2 Ansicht nicht gespeichert) und die
  Prüfschritte.
- **Issue-Tracker bereinigt:** #68, #91, #124, #125 waren bereits implementiert
  und gemergt, standen aber noch offen (PRs ohne Closing-Keywords). Alle vier
  mit Beleg-Kommentar geschlossen. Neue Charter-Regel in `CLAUDE.md` §11:
  PRs, die ein Issue erledigen, tragen **`Fixes #NNN`** im PR-Text.
  Einziges offenes Issue: #132 (SSDD, bewusst zurückgestellt) + neu #159.

## 🎯 Stand 2026-07-03

- **Zuletzt aktualisiert:** 2026-07-03
- **Ist-/Gap-Analyse Service-Orientierung & HA (Doku-Sitzung, Branch
  `claude/wayfinder-firefly-architecture-759lfg`):** Auf Frage des
  Projektverantwortlichen („Wie service-orientiert sind Firefly/Wayfinder heute?
  Lohnt es, das für Produktion/HA weiter zu verankern?") wurde eine
  repo-übergreifende Analyse erstellt und dokumentiert:
  **`docs/design/gap-analyse-service-orientierung-ha.md`**. Kernaussagen:
  System-Ebene ist bereits service-orientiert (CAT062-Draht-Vertrag, 1 Firefly
  pro Feed, Orchestrator-Control-Plane); Binnen-Ebene sind bewusst modulare
  Monolithen mit vorbereiteten Nahtstellen. HA entsteht über Redundanz + Zustand,
  nicht über Zerlegung — empfohlene Reihenfolge: **WF2-52 Teil 1** (ASD
  multi-replica: fixer Session-Key, Rescope über Replikas, `/ws`-LB-Konzept) →
  Firefly-Zustands-Story (Recorder/Snapshot, SDPS-002-Vorstufe) → Feed-Redundanz
  (eigener ADR, beidseitig) → **ORCH-6** (K8s). Verweise in `ROADMAP.md`
  (Stufe 5 + §3) eingehängt; Firefly-`STATUS.md` verweist ebenfalls. **Reine
  Doku, kein Code** — Umsetzung erst nach Ankündigung + Go je Paket.

## 🎯 Stand 2026-07-02

- **Zuletzt aktualisiert:** 2026-07-02
- **E2E-Finding (diese Sitzung, gleicher Branch): Luftraum-Overlays trotz
  ausgeschaltetem Feature-Toggle (Teil 1).** Nach dem Setzen des OpenAIP-Keys
  erschienen Luftraum-/Navaid-/Wegpunkt-Layer, obwohl das `airspaces`-Feature
  des Mandanten **aus** war. Ursache: `/api/airspace|navaids|waypoints` lagen zwar
  hinter der Tenant-Middleware, prüften aber **nicht** das Entitlement — der
  Frontend-Toggle (`showLayer`) blendet nur die Sidebar-Zeile aus, die Karte holte
  die Daten trotzdem (`layerVisibility.airspace` default `true`), und der Server
  lieferte sie ungeprüft. Fix (server-seitig, die eigentliche Grenze): injizierter
  `aeronautical.FeatureGate` (`aeroFeatureKey` Kind→Feature; `featSvc.HasFeature`)
  → ohne Feature **leere** Collection, Overlay erscheint nicht. Handhabt auch das
  **Live-Toggle-Aus** (nächster Refresh liefert leer → Overlay geräumt); **kein**
  Frontend-Change nötig. Test `TestRegistryHandlerFeatureGateDeniesServesEmpty`;
  FR-ADMIN-009 + TECHNICAL.md ergänzt. Gates: `go test`/`vet`/`gofmt` grün.
  **Teil 2 (Radius/AOI) offen — hängt an Rückfrage (Viewing-/Speicher-Kontext).**
- **E2E-Finding (diese Sitzung, gleicher Branch): Multi-Feed-Multicast-Crosstalk
  → Cross-Tenant-Leck + Feed-Chip-Flackern.** Mit **zwei** Feeds auf einem Host
  flackerte der Feed-Chip (grün↔gelb) im ~2-s-Takt, und — gravierender — ein
  Empfänger sah die **Tracks des jeweils anderen Feeds**. Ursache: Der Allocator
  vergibt eine Gruppe je Feed bei **festem Port** (`feed_alloc.go`), aber
  `net.ListenMulticastUDP` bindet **Wildcard** (`0.0.0.0:8600`) und joint nur per
  IGMP → auf einem Host empfängt jeder Socket **alle** beigetretenen Gruppen; ein
  Empfänger etikettierte fremde Tracks mit **seiner** feed_id → Leck **vor** dem
  Scope-Filter. **Nicht** aus dem Polling-Paket (#2/#3 sind sauber; Logs zeigten
  kein 429/Backoff) — ein latenter Bug, der erst mit dem **zweiten** Feed auftritt.
  Fix in `pkg/receiver`: Ziel-Gruppe je Datagramm via `ipv4.PacketConn`/`FlagDst`
  prüfen, Fremdgruppen verwerfen (`acceptsGroup`); Fallback-Log wenn `IP_PKTINFO`
  fehlt. Neue Dependency `golang.org/x/net`. Unit-Test `TestAcceptsGroup`;
  NFR-SEC-003 + TECHNICAL.md ergänzt. Verifikation operativ (E2E): ein Feed → stabil,
  zwei Feeds → vor dem Fix Flackern. Gates: `go test ./...`, `go vet`, `gofmt` grün.
- **E2E-Finding (diese Sitzung, gleicher Branch): „Zugang anlegen" scheiterte
  stumm bei doppeltem Benutzernamen.** Der Anlegen-Dialog (`AdminUsers.vue`)
  schloss bei Erfolg, tat bei Fehler aber **nichts** — kein Hinweis, warum. Ursache
  fachlich: Subjects sind **mandantenübergreifend eindeutig**, der Server meldet
  korrekt `409 "subject already exists"` (Backend unverändert), aber das Frontend
  zeigte die Meldung nicht. Fix: Dialog rendert jetzt einen `v-alert` mit klarer
  deutscher Begründung (`createErrorMessage`): 409 → „Benutzername bereits vergeben,
  mandantenübergreifend eindeutig — evtl. in einem anderen Mandanten"; Passwort-zu-
  kurz übersetzt; sonst Server-Detail/Fallback. Regressionstest
  `adminUsersCreateError.test.js` (`?raw`-SFC). Gates: **vitest 297**, `vite build`,
  `go test ./internal/webui` grün; `dist` neu eingebettet.
- **E2E-Finding (diese Sitzung, gleicher Branch): Mandanten-Dropdown im Quellen-
  Dialog zeigte neu angelegte Mandanten nicht.** Das „Aus Mandant übernehmen"-
  Dropdown (`AdminFeeds.vue`) liest `admin.tenants` (Cross-Mandanten-Liste), die
  Mandanten-Übersicht dagegen `admin.overview` (Dashboard-Zeilen) — **zwei
  getrennte Quellen**. `openSources` lud `admin.tenants` nur **lazy**
  (`if (!admin.tenants.length)`), sodass ein **nach** dem ersten Laden angelegter
  Mandant (Hamburg) nie im Dropdown erschien (die Übersicht zeigte ihn, weil sie
  `overview` neu lädt). Fix: `openSources` lädt die Mandantenliste **immer** neu
  (Lazy-Guard entfernt). Regressionstest `adminFeedsTenantDropdown.test.js`
  (`?raw`-SFC). Gates: **vitest 294**, `vite build`, `go test ./internal/webui`
  grün; `dist` neu eingebettet.
- **Design-Template-Angleichung (diese Sitzung, Branch
  `claude/wayfinder-design-template-b1krxc`, FR-UI-023, ADR 0015 Nachtrag-2):**
  Der Projektverantwortliche hat den Claude-Design-Export (`ASD.zip`) zum
  **verbindlichen Template** erklärt (wie Material Design für die Komponenten).
  Ein pixel-/hex-genauer Audit (4 parallele Prüfläufe: Farben, Symbol-Geometrie,
  Fonts, Chrome) fand die realen Abweichungen; nach Freigabe von drei
  Richtungsentscheidungen (Near-Black übernehmen · Basiskarte behalten +
  angleichen · Roboto Mono jetzt selbst hosten) in 5 Häppchen umgesetzt:
  - **G0 Farben (ADR 0015 Nachtrag-2):** Surface-Hierarchie **zurück auf
    Near-Black** (`#070b12`/`#0e1622`/`#16202e`/`#1c2c3e`) — die einzige
    Farb-Abweichung; Navy (Nachtrag-1) war eine Screenshot-Fehl-Lesung und ist
    aufgehoben. Lockstep `colors.css`+`vuetify.js`; Map-Hintergrund
    `#0b1a2e`→`#070b12` (CARTO-Raster bleibt @ 0.4 — echte Geografie bewusst).
    Alle übrigen Farben stimmten schon hex-genau.
  - **G1/G2 Symbole:** waren ~40 % zu klein (24 px-Canvas@pixelRatio 2 deckelt
    auf 12 CSS-px). Canvas 32 px, Zeichen-Geometrie = Template-CSS × 2 (Raute 12,
    Quadrat 8, Kreis-Ø 9). Zwei Korrektheits-Fehler behoben: **PSR** ist jetzt in
    **jedem** Zustand ein **hohler Ring** (war 3/4 gefüllt), der fehlende
    **Cyan-Auswahl-Ring** (r=11) ist als eigener MapLibre-Circle-Layer ergänzt
    (an die Selektion gepinnt). Legende zeichnet dieselben SVG-Marken wie die
    Karte (PSR hohl). History-Dot 1.6, Deconfliction-BBox 8→9.
  - **G3/G4 Fonts:** Karten-Datenblöcke jetzt **Roboto Mono** — Wayfinder
    **hostet die Glyph-PBFs selbst** (`/glyphs/{fontstack}/{range}.pbf`,
    `go:embed`, fontnik-generiert, Ranges 0-1023); kein Font-CDN mehr auf der
    Karte (air-gap-Schritt). Zusätzlich GL-`letter-spacing 0.02`/`line-height
    1.25`.
  - **G5/G6/G7 DOM-Typo + Chrome + Backdrop:** Overline 10 px/700; Track-Detail
    **oben-rechts** (292, behebt Kollision mit dem Maßstab-Readout); Nav-Panel
    248, Rail-Brand-Kachel 30×30, Legende 232/0.96/Radius-md; **Cyan-Mittglow**
    über dem Scope.
  - **Ehrliche Grenze:** Militär/Hostile/Alarme bleiben mangels Wire-Daten
    draußen; die 700-Callsign-Zeile + 9.5px-Alarm-Zeile der Template-Datenblöcke
    sind auf **einer** GL-Symbol-Schicht nicht darstellbar (bräuchten eine zweite
    Schicht / DOM-Datenblöcke) — zurückgestellt. **Live-WebGL-Render nicht in
    dieser Umgebung verifizierbar** (kein Browser-Stack); Go-Glyph-Handler +
    Style + Symbol-Geometrie sind aber unit-getestet.
  - Gates: **vitest 280**, `go test ./...` (28 Pakete, Integration skippt ohne
    PG), `go vet`/`gofmt` grün, `vite build`; `dist` neu eingebettet.
- **Neues Design (Claude Design) → Reskin gestartet (diese Sitzung, Branch
  `claude/wayfinder-design-implementation-6wbbbg`):** Ein per Claude Design
  erstellter ASD-Entwurf kam als Export (`ASD.zip`: Design-System mit Tokens +
  ASD-Ziel-Screens als **React/JSX** + Screenshots). Das Design-System ist
  **rückwärts aus dem Wayfinder-Code abgeleitet** — Tokens decken sich mit
  `vuetify.js`/`constants.js`. Vorgehen: **inkrementeller Reskin auf Vue/Vuetify**
  (kein JSX-Code übernehmen), nur Elemente, die wir **heute datenseitig stützen**
  (Vorgabe: keine Fake-UI).
  - **Audit gegen den realen WS-Payload** (`pkg/broadcast` `TrackMessage`): vorhanden
    sind Position/vx-vy/confirmed/coasting/ended/Provenienz-Alter (ADS-B/SSR/MDS/
    FLARM)/accuracy/mode_3a/icao/FL/callsign + Feed-Status. **Nicht** vorhanden:
    Ziel-Typ mil/hostile/neutral, Zuständigkeit/Sektor-Eigentum, STCA/APW.
  - **6-Häppchen-Plan (Thema→Häppchen):** **1** Fundament (Tokens+ADR) · **2**
    Chrome-Reskin (Rail/Sidebar/Track-Detail/Feed-Chip/Provenienz) · **3** Kopfzeile
    (**ICAO-Kürzel** aus Feed/View-Config, UTC-Uhr), schwebende Legende, Maßstab/
    Vektor-Readout, optional Graticule · **4** Werkzeuge RBL/DIST/QDM(/PROBE) ·
    **5** Tweaks-Panel (Vektor-min, History-Dots-Anzahl, Label-Variante, Tag/Nacht,
    Toolbar-Position) · **6** Safety-Nets EMG+DUP (aus `mode_3a`). **Bewusst raus (C):**
    Typ-Farben mil/hostile/neutral, Zuständigkeits-Dimming, STCA, Sektorgrenzen/
    Airways/Terrain/Runways/Ext-Centerlines-Layer, APW (geparkt).
  - **Häppchen 1–4 umgesetzt (PR #130, ADR 0015, FR-UI-019…022):**
    - **1 Fundament:** Design-Tokens (`--wf-*`) in `frontend/src/design/tokens/` +
      `base.css`; **Roboto/Roboto Mono self-hosted via `@fontsource`** (latin/
      latin-ext, kein Laufzeit-CDN). Karten-Engine unberührt.
    - **2 Chrome-Reskin:** Mono-Readouts (Track-Detail), tonaler Feed-Badge,
      Floating-Chrome-Hairlines; Token-Konvergenz Rail/Sidebar/Map-Controls. Die
      Komponenten waren aus dem Design rückwärts abgeleitet → optisch nah, jetzt
      token-getrieben.
    - **3a Kopfzeile:** **ICAO-Kürzel per-Mandant** (Migration 00015
      `view_configs.icao`, `whoami.icao`, Admin-View-Editor) + Live-UTC-Uhr
      (`AsdHeader.vue`). Ehrlich: ICAO ist Config, kein CAT062-Feld.
    - **3b Legende/Readout:** schwebende, ausklappbare `ScopeLegend.vue` (Glyph-
      Provenienz gefiltert + reale Zustandsfarben; **keine** Typ-Farben/Alarm-
      Zeile), Vektor-Minuten-Readout, native ScaleControl nach unten-rechts.
      Provenienz-Legende als eine Quelle in `map/provenance.js`.
    - **4 Werkzeuge:** RBL/DIST/QDM — `map/tools.js` (Großkreis-Geometrie, 10
      Tests), `map/measure.js` (MapLibre-Controller), `stores/tools.js`,
      `MeasureToolbar.vue` (Tastenkürzel R/D/Q/Esc).
    - **Bewusst zurückgestellt:** Graticule-Layer (optional; dyn. Grid-Regen,
      hier nicht verifizierbar) und **PROBE** (Inhalt undefiniert).
    - Gates: **vitest 238**, `vite build`, `go build/test ./...` (28 Pakete ok,
      Integration skippt ohne PG), `go vet`/`gofmt` grün; `dist` neu eingebettet.
  - **Nächster Schritt:** Häppchen 5 (Tweaks-Panel) + 6 (Safety-Nets EMG/DUP) —
    vorher ankündigen/freigeben. Optik/Funktion von 1–4 wird im **E2E-Lauf**
    geprüft (Anhang beim nächsten realen Durchlauf).
- **E2E-Finding (diese Sitzung, Branch `claude/wayfinder-tenant-radius-bug-w99r8q`):
  Mandanten-Radius wurde nach Reload nicht angezeigt (E2E 5.3.1).** Ursache:
  `src/admin/geo.js` rechnete intern in **camelCase** (`minLat`…), der Backend-
  Wire-Vertrag (`store.BBox`) ist aber **snake_case** (`min_lat`…). Speichern
  mappte von Hand korrekt; beim Laden bekam `bboxToRadius` die snake_case-AOI
  direkt → `null` → Radius sprang auf 0 (wirkte „nicht gespeichert"), und das
  nächste Speichern überschrieb die AOI mit `NULL` (Datenverlust). Fix:
  `radiusNmToBbox`/`bboxToRadius` sprechen jetzt durchgängig die Wire-Form; die
  zwei Hand-Mappings in `AdminTenantDetail.vue`/`AdminFeeds.vue` entfielen. Behebt
  denselben Bruch auch bei „Aus Mandant übernehmen" (`applyTenantArea`, E2E
  5.3.3/5.3.5/5.3.7) und beim Editieren gespeicherter Area-Quellen (`toFormSource`).
  Gates: **vitest 244**, `vite build`, `go build`/`go test ./internal/webui` grün;
  `dist` neu eingebettet.
- **E2E-Finding (diese Sitzung, gleicher Branch): Kopf-Feed-Chips im Mandanten-
  Detail aktualisierten sich nicht beim Zuweisen/Entziehen eines Feeds.** Die
  Chips oben in der „Feeds"-Karte stammen aus `admin.overview` (einmalig geladen),
  die Zuweisungstabelle (`AdminProvisioning`) lud nach `grant`/`revoke` nur ihren
  lokalen `tenantSubs` neu → Chip und Tabelle drifteten auseinander (Screenshot:
  Kopf zeigte `frankfurt-adsb`, Tabelle `frankfurt-flarm` zugewiesen). Fix:
  `AdminProvisioning` emittiert nach Erfolg ein `changed`-Event; `AdminTenantDetail`
  lädt darauf `loadOverview()` + `loadFeedsHealth()` neu (analog zu `toggleStatus`,
  das die overview schon nachlud). Kein Backend-Change. Regressionstest im `?raw`-
  SFC-Stil. Gates: **vitest 248**, `vite build`, `go build`/`go test ./internal/webui`
  grün; `dist` neu eingebettet.
- **E2E-Finding (diese Sitzung, gleicher Branch): Design-Abgleich gegen den
  Mockup — der Reskin (#130) war hinter dem Mockup zurück.** In freigegebenen
  Häppchen nachgezogen:
  - **Häppchen 1 — Navy-Farbschema** (ADR 0015 Nachtrag): Surface-Hierarchie von
    Near-Black auf tiefes Navy (`background #0a1626` …), Map-Hintergrund `#0b1a2e`
    + CARTO-Raster `raster-opacity 0.4`. Tokens + `vuetify.js` im Lockstep.
  - **Häppchen 2 — Track-Symbolik**: Formen an den Mockup — **◆ ADS-B, ● PSR**
    (gefüllt), **■ SSR** (FLARM `F`/combined `K` bleiben, Wayfinder-Superset);
    **Coasting wird hohl** gezeichnet (Umriss statt Füllung) statt nur gedimmt, so
    ist der Zustand an der Form erkennbar. Legende spiegelt das (Coasting = hohler
    Ring) und der **z-index-Bug** (Legende verschwand hinter der 56 px-Leiste) ist
    behoben (`left: 68px`). Symbolik in `map/layers.js`, Glyphen in
    `map/provenance.js`; Regressionstests. **Militär-Caret/Alarme bleiben draußen**
    (keine Wire-Daten). Gates: **vitest 254**, `vite build`, `go build`/`go test
    ./internal/webui` grün; `dist` neu eingebettet.
  - **Häppchen 3 — Werkzeuge in die Leiste**: RBL/DIST/QDM sind jetzt **Rail-Icons**
    (`NavigationRail.vue`, Toggle → `tools`-Store, der `map/measure.js` treibt);
    die schwebende Mess-Toolbar entfällt, der Hinweis/Readout + Tastenkürzel
    R/D/Q/Esc bleiben in `MeasureStatus.vue` (umbenannt aus `MeasureToolbar.vue`).
    **Zoom +/−** ebenfalls in die Rail (aus `MapControls` entfernt, Recenter/Vollbild
    bleiben rechts), delegiert über `AsdView`→`MapCanvas.zoomIn/zoomOut`. PROBE
    bleibt draußen (kein Inhalt). Auch im Mobil-Drawer erreichbar. Regressionstest
    `railTools.test.js`. Gates: **vitest 260**, `vite build`, `go test ./internal/webui`
    grün; `dist` neu eingebettet.
  - **Offen:** volle Mockup-Karte (Vektor-Grid, Sektorgrenzen, Airspace/Navaids)
    — separates, teils datenabhängiges Thema.
  - **Scope-Chrome-Feinschliff (E2E-Design-Durchgang):** (a) Die 3 Status-Chips
    oben zentriert **entfernt** — Status kommt aus der Symbolik; die
    Kategorie-Filterfunktion wurde auf Freigabe **komplett fallen gelassen**
    (TrackFilterChips + `hiddenCategories`/`trackCounts`-Maschinerie aus Store/
    `render.js`/`engine.js` ausgebaut, FR-UI-010 als entfernt markiert). (b)
    **Konto-Dopplung** aufgelöst: der `lotse`-Chip oben rechts ist weg (Konto nur
    noch in der Sidebar), der **Feed-Status-Badge** rückt auf dessen Platz,
    Zentrum/Vollbild rücken nach oben. (c) ~~Rechts unten jetzt ein Pill
    **„‹Breite› NM Breite · Vektor ‹N› min"**~~ — **entfernt (E2E 2026-07-04):**
    Das Pill sah aus wie ein Maßstabsbalken, war aber nur die Schirmbreite, und
    stand irreführend neben den Range-Ringen. Ausgebaut (`AsdView`-Overlay,
    `engine.js` `reportViewportWidth`/`haversineNM`-Import, `asd`-Store
    `viewportWidthNM`/`setViewportWidth`); `scopeChrome.test.js` invertiert.
    Distanz kommt aus den Range-Ringen, die Vorhalte-Zeit aus dem
    Geschwindigkeitsvektor am Symbol. Zugleich die **Range-Ring-Labels von
    Norden auf die vier Diagonalen gestaffelt** (`LABEL_BEARINGS`,
    NO→SO→SW→NW), damit sie nicht mit der Kopf-Chrome kollidieren und nicht
    gemeinsam aus dem Bild scrollen. Regressionstests `scopeChrome.test.js`,
    `rangerings.test.js` angepasst.
  - **Scope-Fix-ups (E2E, Folge-Durchgang):** (1) **RBL/DIST/QDM waren tot** —
    `createMeasure` lief in `MapCanvas` **vor** dem Map-`load` (initMap kehrt vor
    `load` zurück), `addSource` warf → `measure` blieb `null`. Fix: Controller erst
    bei `load` erzeugen (`map.loaded()`/`map.once('load')`), Tool-Vorwahl nachziehen.
    Bestand seit Häppchen 4, nie end-to-end getestet. (2) **OSM-Attribution kompakt**
    (`attributionControl:false` + `AttributionControl({compact:true})`) — der lange
    Credit-Text lag unter dem Readout, ist jetzt ein einklappbares ⓘ (Credit bleibt).
    (3) **Rail**: ASD-Brand-Glyph (`mdi-radar`, primary) oben + horizontale
    Trennlinien zwischen den Gruppen (Vorlage-Screenshot; Brand später ASD⇄EFS-
    Switch). (4) **Kopfzeile** (ICAO/EDLV + UTC) von oben-zentriert nach **oben
    rechts neben den Feed-Badge** (gemeinsamer `top-right-cluster`). PROBE weiterhin
    ausgelassen (kein Inhalt). Regressionstests `scopeFixups.test.js`. Gates:
    **vitest 271**, `vite build`, `go test ./internal/webui` grün; `dist` neu
    eingebettet.
  - **Mess-Readout an der Linie (E2E-Wunsch):** Distanz/Peilung schwebt jetzt als
    Label **an der RBL/DIST/QDM-Linie** (Anker = A–B-Mittelpunkt, in `map/measure.js`
    per `map.project` nach Bildschirm-Pixeln projiziert und bei Drag **und** Karten-
    Move reprojiziert → `tools`-Store `readoutAt`). `MeasureStatus.vue` rendert das
    Pill dort; unten bleibt nur noch die Instruktion. Regressionstests
    `measureLabel.test.js` + `tools`-Store. Gates: **vitest 275**, `vite build`,
    `go test ./internal/webui` grün; `dist` neu eingebettet.
- **E2E-Finding (diese Sitzung, gleicher Branch): Zugangsdaten-UI im Quellen-
  Dialog quelltyp-abhängig (UX-4).** Im „Quellen"-Dialog erschien das Credential-
  Feld (Referenz + Client-ID/Secret) für **jeden** Quelltyp — auch für **Radar**
  (CAT048: Netz-Endpunkt ohne Auth) und **FLARM**, wo die OpenSky-Labels irre-
  führen. Zudem musste der Operator die `cred_ref` von Hand erfinden, bevor die
  Felder überhaupt auftauchten (Reibung: erst nach Eintippen eines Handles wurden
  Client-ID/Secret sichtbar). Fix in `AdminFeeds.vue`:
  - **Quelltyp-Tabelle `CREDENTIAL`** (`credInfo(type)`): nur `adsb_opensky`
    (OpenSky Client-ID/Secret, **Pflicht**) und `flarm_aprs` (APRS-IS Rufzeichen/
    Passcode, **optional**) tragen einen Credential-Block; **`radar_asterix`
    zeigt keinen** — Radar authentifiziert nicht.
  - **`cred_ref` wird automatisch vergeben** (`ensureCredRef`): eine
    credential-tragende Quelle ohne Ref bekommt ein deterministisches Handle
    (`secret/feed-<id>-<type>`); ein bereits gespeichertes Handle bleibt erhalten
    (Secret bleibt verknüpft); Radar-Quellen bekommen die Ref geleert. Kein
    Hand-Handle mehr, die zwei beschrifteten Felder erscheinen sofort.
  - **Secret-Store aus** (`WAYFINDER_SECRET_KEY` ungesetzt): statt eines toten
    Feldes jetzt ein klarer Hinweis-Alert (bei ADS-B mit dem 429-Kontext, bei
    FLARM „anonym = Normalfall"). Das ist genau die Reibung, die im letzten Lauf
    das OpenSky-429 verursacht hat.
  - Regressionstest `adminFeedsCredentials.test.js` (`?raw`-SFC). FR-ORCH-001 im
    Anforderungs-Register um UX-4 ergänzt. Gates: **vitest 279**, `vite build`,
    `go test ./internal/webui` grün; `dist` neu eingebettet. **PR #141 gemergt.**
- **E2E-Finding (diese Sitzung, gleicher Branch): Feed-Status feiner
  aufgeschlüsselt + Colorcode-Referenztabelle (4-Punkte-Liste #1).** Ein toter
  Feed zeigte nur pauschal **rot „inaktiv"** — ununterscheidbar, ob er **nie
  angelaufen** ist (`!ever_seen`) oder **lief und abriss** (`ever_seen && stale`).
  Operativ ein Unterschied: „nie gestartet" zeigt auf Zuweisung/Orchestrierung
  (genau der Fall „war nicht zugewiesen"), „abgerissen" auf einen Laufzeit-Ausfall.
  - **Gemeinsamer Helper `admin/feedHealth.js`** (`describeFeedHealth` → {color,
    label, title}) ersetzt die **dreifach duplizierte** `feedColor`/`feedTitle`/
    `feedLabel`-Logik in `AdminFeeds.vue`/`AdminTenantDetail.vue`/`AdminTenants.vue`.
  - **Rot-Split** (rein presentational, Wire-Farbe bleibt rot): `!ever_seen` →
    Label **„nie gestartet"**; `ever_seen && stale` → **„abgerissen"** mit
    `seit ‹N› s kein CAT065` aus `last_heartbeat_ago_s`. Grün trägt zusätzlich
    `aktiv/total Radare` (CAT063), wenn bekannt.
  - **Doku:** Colorcode-Referenztabelle in `docs/TECHNICAL.md §2.5` (alle Farben +
    Unter-Zustände + treibende Snapshot-Felder). FR-OPS-004 im Register ergänzt.
  - **Kein** Backend/DTO/Wire-Change (DTO trug die Felder schon). Reiner Helper-
    Unit-Test `admin/__tests__/feedHealth.test.js` (8 Tests). Gates: **vitest 287**,
    `vite build`, `go test ./internal/webui` grün; `dist` neu eingebettet. **PR #142
    gemergt.**
- **E2E-Finding (diese Sitzung, gleicher Branch): Konfigurierbares OpenSky-Poll-
  Intervall (4-Punkte-Liste #3, cross-project mit Firefly ADR 0029).** Der E2E-Feed
  lief anonym in **HTTP 429**, weil die OpenSky-Poll-Kadenz fix bei 10 s lag und
  über das Wayfinder-UI nicht steuerbar war. Jetzt trägt eine `adsb_opensky`-Quelle
  ein optionales **`poll_interval_secs`**:
  - **Firefly-Seite (PR #48 gemergt):** `FIREFLY_SOURCES`-Kontrakt v1.4.0 (ADR 0029)
    — `SourceSpec.poll_interval_secs` (additiv, nur `> 0` überschreibt, sonst
    Default 10 s). Bidirektional kompatibel (kein `deny_unknown_fields`).
  - **Wayfinder-Seite (dieser PR):** `store.Source.PollIntervalSecs` + Validierung
    am Schreib-Rand (**nur** `adsb_opensky`, Bereich 5..3600 s, sonst 400-mit-Index);
    `dockerbackend.fireflySource` reicht es additiv nach `FIREFLY_SOURCES` durch;
    **UI-Feld nur bei ADS-B** (leer = Default 10 s) + **Infobox** zum OpenSky-Rate-
    Limit (429). Nur presentational sichtbar; Firefly bleibt tolerant (Bereich am
    Wayfinder-Rand erzwungen).
  - **Kein** DTO-Change nötig (Admin-API nutzt `store.SourceConfig` direkt). Tests:
    `feed_sources_test.go` (+5 Fälle), `sources_test.go` (Passthrough),
    `adminFeedsPollInterval.test.js` (5). FR-ORCH-001 (UX-5) + `docs/TECHNICAL.md`.
    Gates: **vitest 292**, `go test ./pkg/... ./internal/webui`, `vite build` grün;
    `dist` neu eingebettet.
- **E2E-Testlauf-Findings #109–#121 umgesetzt (Branch
  `claude/mac-mini-e2e-network-53epgr`):** Zweiter Findings-Batch aus dem realen
  Mac-Mini-E2E-Lauf. Kurz:
  - **#110** Runbook-Wording (View-Config → **Standard-Ansicht**), **#109/#113**
    Quell-Abdeckung als **Zentrum+Radius** + **Mandanten-Dropdown**, **#112**
    Feed-Refetch nach Quellen-Speichern, **#111** Erfolgs-Badges nach 5 s weg
    (FR-ORCH-009).
  - **#114/#115/#116/#121** Sidebar-Neugliederung (Layer/Filter/Nutzer-Account,
    Default eingeklappt, FL-Band-Hinweis, Radarabdeckung-Gate, Resize-Fix)
    (FR-UI-018).
  - **#117** Feed-Status-Fix (color→state-Mapping + worst-wins-Aggregation, behebt
    dauerhaftes „FEED ?"), **#118/#119** Per-Technologie-Alter im CAT062-Decoder
    (SSR/MDS/**FLARM**, ICD 2.6.0) + **A/F-Glyphen** und distinkte FLARM-Provenienz
    (FR-DATA-007).
  - **#120** (kombinierter ADS-B+FLARM-Feed ohne Tracks) **root-caused + gefixt in
    Firefly**: FLARM stempelte Mitternachts-Sekunden statt Unix-Epoch → der
    gemeinsame Datenzeit-Wasserstand verwarf FLARM-Plots. Fix im FLARM-Adapter
    (Epoch-Zeit), siehe Firefly-STATUS + `docs/milestones/FLARM-Epoch-Time_Multi-Source-Fusion.md`.
  - Gates grün: `go test/vet/gofmt` (Wayfinder), `cargo test --workspace`/clippy/fmt
    (Firefly), **218 vitest**, `vite build` (dist neu eingebettet).

## 🎯 Stand 2026-07-01

- **Zuletzt aktualisiert:** 2026-07-01
- **Großes Bild:** Das **Prio-1-Go-to-Market-Fundament ist fertig** — ONB
  (Zero-Touch-Onboarding) ✅ und **ORCH (Auto-Orchestrierung) ✅ Kern komplett**
  (1…5c). „Feed zuweisen ⇒ passende Firefly-Instanz startet automatisch" ist
  gebaut, getestet, sicherheits-reviewed und gehärtet. Alles auf `main`,
  alle Gates grün (Go: build/test/vet/gofmt/golangci-lint; Frontend: 180 vitest).

- **AP7 — Serverseitige Session-Registry + Session-Limit (Issue #64, diese Sitzung):**
  Letztes offenes Arbeitspaket von **ADR 0009** umgesetzt (Branch
  `claude/issue-64-session-registry-ymz7py`). Neue Tabelle `sessions` (Migration
  00014; Cookie trägt eine signierte Session-ID, in der DB nur als Hash), `SessionRepo`
  mit atomarem **Session-Limit** (Advisory-Lock, Policy `reject`/`evict_oldest`),
  fail-closed **Resolve** (Status-Join Zugang+Mandant), gleitender/absoluter Ablauf,
  **Sofort-Revoke** bei Pause/Löschen (Zugang/Admin/Mandant-Kaskade), echtes
  serverseitiges **Logout**, Janitor + Metriken (`wayfinder_active_sessions` u. a.).
  **Sanfte Übernahme** beim Rollout (Legacy-Cookie → Registry beim nächsten Renew;
  harter Schnitt per `WAYFINDER_SESSION_KEY`-Rotation). Env:
  `WAYFINDER_SESSION_LIMIT_DEFAULT` (Default aus) + `_POLICY` (Default `reject`).
  **Adversariale Review** (Fan-out find→verify): eine echte Lücke gefunden & gefixt
  (Limit-Bypass auf dem Legacy-Konversions-Pfad). Gates grün inkl. real-PG
  (`scripts/pg-test.sh`). Doku: FR-ADMIN-010, Milestone WF2-12.7, TECHNICAL/
  INSTALLATION/BETRIEB. PR #98 **gemergt**. **Nachtrag (Branch
  `claude/session-limit-admin-ui`):** Admin-UI zum Setzen des per-Zugang
  `session_limit` — Route `PUT /api/admin/tenants/{id}/users/{uid}/session-limit`
  (`null`=Default/`0`=unbegrenzt/positiv=Kappung), `userDTO.session_limit`,
  `AdminUsers.vue`-Spalte + „Limit"-Dialog; Go+Frontend-Gates grün (vitest 207).

- **Diese Sitzung (2026-06-29/30):** ORCH-5b-1 (Cred-Auflösung in der
  Control-Plane, Variante A) · 5b-2 (UI-Zwei-Felder) · 5c (E2E-Abnahme-Harness:
  `docker-compose.orchestrated.yml` + `Dockerfile.orchestrator` +
  `scripts/e2e-orchestrated.sh` + `docs/E2E-ABNAHME.md`) · UI-Relabel
  Client-ID/Client-Secret (OpenSky OAuth2) · **Konsolidierung** (Sicherheits-Review
  ohne kritische Befunde, `broadcast.time_ms`-Fix, ROADMAP-Drift bereinigt) ·
  **Secret-Hardening** (AES-GCM-AAD-Bindung an `(feed_id, cred_ref)`).
  Cross-Repo: Firefly OpenSky **OAuth2 Client-Credentials** (ADR 0024).

- **Mac-mini-E2E (Sitzung 2026-07-01):** Der orchestrierte E2E-Stack braucht
  Host-Net-Multicast und damit Linux; auf Docker Desktop (Mac mini/Windows) geht
  das nicht. Zwei Ergebnisse: **(1)** eingecheckte **`docker-compose.bridge.yml`**
  (Firefly + Postgres + Wayfinder in **einem** Bridge-Netz; Container↔Container-
  Multicast funktioniert dort → UI + Live-Tracks auf dem Mac, aber ohne
  Auto-Spawn). **(2)** `docs/E2E-ABNAHME.md` **komplett neu** als
  Schritt-für-Schritt-Runbook mit einer **Multipass-Linux-VM** auf dem Mac mini:
  Teil 0–2 (VM + Docker), Teil 3 (Repos/Image/Stack), Teil 4 (automatischer,
  deterministischer Lauf `e2e-orchestrated.sh --mode scene` mit exakter
  Soll-Ausgabe), Teil 5 (UI-Abnahme, Auto-Endpoint, Frankfurt-Szene → Tracks),
  Teil 6 (Belege), Teil 7 (Aufräumen), Teil 8 (Fehlerbehebung), **Anhang A**
  (Bridge-Schnell-Check ohne VM). Jeder Schritt mit **exaktem** erwartetem
  Ergebnis. Querverweise in `DOCKER.md`/`INSTALLATION.md`/`TECHNICAL.md` auf die
  neue Struktur (Anhang A / Teil 1–6) nachgezogen. Gates grün (gofmt/build/vet +
  28 Test-Pakete; `docker compose config` valide). Kein Go-/ICD-Change — reine
  Betriebs-/Abnahme-Doku.

- **E2E-Testlauf-Findings #100–#107 umgesetzt (Sitzung 2026-07-01):** Aus dem realen
  Multipass-Durchlauf gesammelte Issues gebündelt umgesetzt. **#104 (Blocker, Bug):**
  Orchestrator-`fireflyEnv` setzt jetzt `FIREFLY_CAT062_ENABLED=true` **und** einen
  pro Feed eindeutigen `FIREFLY_PORT` (18080+Feed-ID) — der host-vernetzte Firefly
  crashte zuvor auf Port 8080 (Wayfinder-Probe) und sendete zudem gar kein CAT062.
  **#102:** Sensor-Mix wird aus den Quell-Typen abgeleitet (`DerivedSensorMix`, in
  `SetSourceConfig` atomar geschrieben). **#106/#107:** `whoami` liefert `sensor_classes`;
  ASD-Karte gated Layer über role-agnostisches Session-`whoami` (Lotse sieht nur
  freigeschaltete Layer) und die Spurherkunft-Legende ist dynamisch je Feed. **#105:**
  Mandanten-Slug wird aus dem Namen abgeleitet (kein Pflicht-Freitextfeld). **#101:**
  Karten-Bedienelemente unter die Status-Chips verschoben (kein Overlap). **#100/#103:**
  `docs/E2E-ABNAHME.md` auf echte Daten (ADS-B→FLARM→beides) + OpenAIP umgeschrieben,
  Labels korrigiert. Doku: TECHNICAL.md (whoami/UI-Gate/fireflyEnv), Register
  (FR-ORCH-008, FR-UI-017). Gates grün: gofmt/vet/`go test ./...` + vitest 207→**209**
  + Frontend-Build; `dist/` neu gebaut.

- **ADR 0014 — Multi-Tenant als einziger Betriebsmodus (diese Sitzung):**
  Single-Tenant vollständig entfernt. **A** (ADR + Charta-Prinzip, PR #94 gemergt) ·
  **B** (Code: `none`-Modus/No-DB-Fallback/nil-Scope raus, DB **+** Auth Pflicht,
  unset `AUTH_MODE`→`builtin`, Legacy-`AUTH_TOKEN`-Gate weg) · **C** (ein
  Multi-Tenant-Deployment-Stack: `orchestrated.yml`→`builtin`, Single-Tenant-
  `docker-compose.yml` gelöscht, `DOCKER.md` aufgeräumt) · **D** (Doku:
  INSTALLATION/TECHNICAL/Anforderungen NFR-SEC-004/BETRIEB; `E2E-ABNAHME.md` als
  **EDLV-Zero-Touch-Runbook** neu). B–D in **PR #95**. Firefly-Doku quergeprüft —
  keine Änderung nötig (CAT062-Wire-Vertrag unverändert).

- **UI-getriebener E2E + Auth-UX-Lücken (diese Sitzung, PR #95):** UI-Audit über
  beide Repos. Admin-Konfig ist bereits vollständig per UI (Mandant/Nutzer/Feed/
  Quellen ADS-B+FLARM/Features/View/Abo). Geschlossene Lücken: **rollen-agnostischer
  `GET /api/whoami`**, **Mandanten-Login + Auth-Gate auf der Karte (`/`)**,
  **Logout** (Karte + Admin-Header), gemeinsamer `apiFetch`. `docs/E2E-ABNAHME.md`
  als **UI-only-Ablaufplan** neu (genau ein Terminal-Befehl zum Start, Rest per UI,
  Terminal nur zur Hinter-den-Kulissen-Prüfung: Firefly-Output Gruppe:Port +
  ADS-B/FLARM). Firefly-Audit: **ADS-B (`adsb_opensky`) und FLARM (`flarm_aprs`)
  beide produktionsreif** und live verdrahtet. Kundenseitige Landing-Login unter `/`:
  durch WF2-12.4 erfüllt + WF2-12.6 Minimal-Branding (siehe unten).

- **Sliding-Session + Login-Overlay (WF2-12.5, diese Sitzung, PR #95):** Der Lotse
  wird bei **aktiver** Nutzung nie ausgeloggt (ASD offen + lebende WS = aktiv, nicht
  Maus/Tastatur); eine verlassene Konsole läuft nach dem Idle-Fenster ab; ein Ablauf
  ist **sichtbar** (Login-Overlay „Sitzung abgelaufen") statt stillem Freeze. Server:
  `POST /api/session/renew`; Client: Renew alle 10 min + Tab-Fokus + WS-Reconnect;
  WS-Close → `/api/whoami`-Probe → ggf. Overlay. Standardwerte: `WAYFINDER_SESSION_TTL`
  = 12h (Sliding-Idle-Fenster), Renew-Takt 10 min. Doku: WF2-12.5, FR-UI-015, TECHNICAL.
  Gates grün (go+205 vitest+build). Manueller Browser-Durchlauf im echten Stack offen.

- **Landing-Branding + absolutes Sitzungs-Maximum (WF2-12.6, diese Sitzung, PR #95):**
  Drei offene Punkte abgearbeitet. **(1)** Landing-Login unter `/` trägt jetzt
  „Wayfinder — Anmelden" (Minimal-Branding, `10f1e04`; der Karten-Login-Gate selbst
  war durch WF2-12.4 bereits erfüllt — kein funktionaler Bedarf, kein separater
  `/login`-Pfad). **(2)** **Absolutes Sitzungs-Maximum** `WAYFINDER_SESSION_MAX_LIFETIME`
  (opt-in, **Default aus**): eine Sitzung lebt — egal wie aktiv — nie länger als diese
  Spanne ab Erst-Login. Signierter `iat`-Claim (rückwärtskompatibel, alte Cookies ohne
  `iat` sanft verankert), Login+Renew kappen die Expiry auf `iat+MAX`, Renew `401` am
  Maximum; Durchsetzung nur in Login/Renew (Impersonation-Grant unberührt).
  Doku: WF2-12.6, FR-UI-016, TECHNICAL/INSTALLATION. Gates grün. **(3)** E2E auf
  Linux-Docker-Host: Offline-Baseline hier grün; Live-Kern bleibt Host-Sache
  (Checkliste beim Testen ins Runbook). **Probelauf:** `WAYFINDER_SESSION_MAX_LIFETIME=30m`.

- **Nächste Schritte (für die frische Session — priorisiert):**
  1. **Realer E2E-Abnahme-Lauf.** Zwei Wege: (a) **Schnell-Check ohne VM** auf dem
     Mac über `docker-compose.bridge.yml` — voller UI-Durchlauf **+ Live-Tracks**,
     aber **ohne** Orchestrator-Auto-Spawn (Runbook Anhang A). (b) **Voller
     orchestrierter Lauf** — jetzt auch auf dem Mac mini via **Multipass-Linux-VM**
     (Runbook Teil 1–6) oder auf jedem Linux-Docker-Host: `scripts/e2e-orchestrated.sh`
     (Prüfpunkte 1/2/5/8, deterministisch offline) + authentifizierter Lauf mit
     echten OpenSky-`client_id`/`client_secret` (Prüfpunkte 3/4/6/7). Der
     Auto-Spawn-Nachweis (1/2/8) braucht einen echten Linux-Kernel (VM genügt).
  2. **Offene Wayfinder-Issues:** #57 (Admin-UI View-Config-Captions, S2) ·
     #68 (Impersonation auf `admin`-Rolle, S4). (#64 Session-Registry/-Limit ✅
     erledigt & gemergt — AP7, PR #98. Nachtrag ✅: **Admin-UI zum Setzen des
     per-Zugang `session_limit`** (Route `PUT …/users/{uid}/session-limit` +
     `AdminUsers.vue`-Spalte/Dialog, Branch `claude/session-limit-admin-ui`).
     Offen nur noch: reale Browser-E2E gegen den Stack.)
  3. **Firefly-Cross-Project (Issue #35):** die übrigen Live-Adapter
     `flarm_aprs` + `radar_asterix` (je eigener ADR; Vokabular im Kontrakt
     reserviert, Wayfinder-Rendering steht schon).
  4. **Prio 2 — Epic CWP/EFS/IMS** (ADR 0013, modulare Controller Working
     Position) — großes Folge-Epic, erst nach Prio 1.
  5. **ORCH-6** (K8s-`InstanceBackend` + HA) — Skalierung; Secret-Management je
     Feed ist via ORCH-2c/5b bereits vorgezogen.

> 🧭 **Maßgeblich für „was als Nächstes":** `docs/ROADMAP.md` (Prioritäts-Rahmen)
> + dieses STATUS. Anforderungs-/Test-Rückverfolgung: `docs/requirements/README.md`
> (FR-ORCH-001…007, NFR-SEC-004).

---

## ✅ Abgeschlossene Stufen (Wayfinder 2.0)

| Stufe | Inhalt | Status |
|---|---|---|
| **Stufe 0** | ADRs 0001–0005 (Stack, Security, Observability) | ✅ |
| **Stufe 1** | CAT062-Decoder, Track-Modell, WS-Server, MapLibre-Karte | ✅ |
| **Stufe 2** | Mandanten-isolierter Datenstrom (WF2-20–WF2-23) | ✅ |
| **Stufe 3** | Admin-API + Admin-UI + Live-Apply (WF2-31–WF2-33) | ✅ |
| **Stufe 4** | Provenienz (WF2-40), Sensorklassen (WF2-41), Feature-Entitlements (WF2-50) | ✅ |
| **ASD-012** | Range-Rings + Scale-Bar + Nord-Orientierung | ✅ |
| **WF2-34** | Cross-Tenant Read-Only-Impersonation (ADR 0008) | ✅ |
| **ADR 0009** | Admin-Bereich-Neuschnitt: AP1–AP7 (Rollen, Features, Dashboard, Feed-Health, Impersonation, Zugänge, **Session-Registry/-Limit**) | ✅ |
| **WF-1–WF-4** | CAT063 Sensor-Status-Decoder + Broadcast + Frontend-Banner (ICD 2.5.0) | ✅ |

---

## 📦 Produktions-Phase (laufend)

### ✅ Epics fertig

| Epic | Inhalt | Status |
|---|---|---|
| **ONB (ADR 0011)** | Zero-Touch-Onboarding: ONB-0…ONB-6 (Auto-Seed, Pflichtwechsel, Admin-CRUD, Mandanten-CRUD, Feed-CRUD, OpenAIP pro Mandant) | ✅ ICD 2.5.0 |
| **ORCH-0 (ADR 0012)** | Architektur-Entscheidung: 1 Firefly-Instanz pro Mandant, Reconciler-Konzept | ✅ |
| **ORCH-1 (ADR 0012)** | Feed-Quell-Datenmodell: `source_config`/`coverage_bbox`, Admin-API, UI-Quell-Builder (1a/1b/1c) | ✅ |
| **ORCH-2a/3 (ADR 0012)** | `Backend`-Interface + `MemoryBackend` + Reconciler (Operator-Muster) | ✅ |
| **ORCH-2b (ADR 0012)** | Docker-Backend-Adapter (`ContainerClient`, Spec-Hash, Labels) | ✅ |
| **ORCH-2c 1–3a (ADR 0012)** | `StoreDesiredState`, `wayfinder-orchestrator`-Binary (Least-Privilege), AES-256-GCM Secret-Store + Resolver | ✅ |
| **ORCH-2c 3a-API (ADR 0012 §6)** | Write-only Secret-Admin-API + `SecretSealer` + `WAYFINDER_SECRET_KEY` + Frontend-Bedienung | ✅ |
| **ORCH-2c 3b (ADR 0012 §5)** | Änderungs-getriebener Reconcile: Migration 00012 (`LISTEN/NOTIFY`-Trigger) + `Listener` + Trigger-Channel/Coalescing | ✅ |
| **ORCH-4 (ADR 0012)** | Automatische Multicast-Endpoint-Allokation: Migration 00013 (`UNIQUE`) + `MulticastPool`/`CreateAutoAllocated` + optionaler Endpoint im Admin-API + Frontend | ✅ |
| **ADR 0013** | Modular CWP & Enterprise ATC Integration ratifiziert (Prio 2, Planung) | ✅ |

### 🚧 Offen

Siehe zentrale **`docs/ROADMAP.md`** für aktuelle Priorisierung (Prio 1 / Prio 2):

- **Prio 1 (jetzt):** ORCH-5 (Container-Injection + Firefly-Quell-Env, cross-project, Firefly #35) → ORCH-6 (ORCH-1, ORCH-2/3, ORCH-2c 3a+3a-API+3b, ORCH-4 ✅)
- **Prio 2 (nach Prio 1):** Modular CWP / EFS / IMS (ADR 0013, Epic CWP-0…IMS-3)
- **ADR 0009 AP7:** Session-Registry, DB-gestützt (S4) — ✅ **erledigt** (Issue #64)

---

## 📋 Cross-Project-Abhängigkeiten (zu Firefly)

Siehe `docs/cross-project/todo-for-wayfinder.md`:

- **ORCH-5 (Live-Quell-Ingestion)** — Firefly-Input-Adapter `adsb_opensky` (Ports & Adapters)
- **Per-Track-Sensor-Provenienz** — erfordert CAT062-ICD-Änderung
- **SWIM-Integration** — Abhängigkeit von Wayfinder EFS/IMS (Prio 2)
- **Ende-zu-Ende-HA** — Wayfinder WF2-52/53 ↔ Firefly SDPS-002

---

## 🔧 Technologie-Stack (ratifiziert)

- **Backend:** Go (ADR 0001) — UDP/Multicast-Eingang, WebSocket-Ausgang
- **Frontend:** Vue 3 + MapLibre GL JS (ADR 0002/0009)
- **Datenbank:** PostgreSQL (Mandanten, Feeds, Entitlements, Sessions)
- **Eingang:** ASTERIX CAT062/CAT065/CAT063 über UDP-Multicast (Draht-Vertrag mit Firefly)
- **Deployment:** Docker + Kubernetes-ready (ADR 0005)

---

## 📚 Wichtige Dateien

- `docs/ROADMAP.md` — zentrale Roadmap für Wayfinder **und** Firefly
- `docs/decisions/` — ADRs (0001–0013)
- `docs/ICD-CAT062.md` → wird gepflegt im **Firefly-Repo** (maßgeblich)
- `CLAUDE.md` — Arbeitsregeln
