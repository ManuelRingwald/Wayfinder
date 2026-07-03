# Arbeitsstand (Handover-Notiz) — Wayfinder

> **Zweck:** Diese Datei beschreibt den **aktuellen IST-Stand** von Wayfinder.
> Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

> 🗺️ **Roadmap & Arbeitspakete:** siehe `docs/ROADMAP.md` in **diesem Repo**
> (zentrale Quelle für Wayfinder **und** Firefly). Cross-Project-Abhängigkeiten
> in `docs/cross-project/todo-for-wayfinder.md`.

---

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
    Zentrum/Vollbild rücken nach oben. (c) Rechts unten jetzt ein Pill
    **„‹Breite› NM Breite · Vektor ‹N› min"** — die native Maßstabsleiste wurde
    durch die aus den Kartengrenzen berechnete Viewport-Breite ersetzt
    (`engine.js` `reportViewportWidth` → `asd`-Store `viewportWidthNM`).
    Regressionstests `scopeChrome.test.js` + `asdViewAuthGate` angepasst. Gates:
    **vitest 267**, `vite build`, `go test ./internal/webui` grün; `dist` neu
    eingebettet.
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
