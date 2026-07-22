# Arbeitsstand (Handover-Notiz) — Wayfinder

> **Zweck:** Diese Datei beschreibt den **aktuellen IST-Stand** von Wayfinder.
> Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

> 🗺️ **Roadmap & Arbeitspakete:** siehe `docs/ROADMAP.md` in **diesem Repo**
> (zentrale Quelle für Wayfinder **und** Firefly). Cross-Project-Abhängigkeiten
> in `docs/cross-project/todo-for-wayfinder.md`.

---

## 🛬 Stand 2026-07-22 (AMAN-Vorhaben gestartet — Marktanalyse, Recherche-Schritt)

**In normaler Sprache:** Der Betreiber hat entschieden, die Wayfinder-Suite um
einen **AMAN** (Arrival Manager — Planungswerkzeug für die Anflug-Sequenz)
zu erweitern. Als erster Schritt wurde eine **Markt- und Funktionsanalyse**
der führenden AMAN-Produkte erstellt (Thales MAESTRO, Leidos/FAA TBFM+TSAS,
OSYRIS/Orthogon-Linie, NATS/Leidos Intelligent Approach, DLR 4D-CARMA sowie
EUROCONTROL-/SESAR-Grundlagen inkl. E-AMAN/CP1-Pflicht): Funktionskanon
(F1–F12), HMI-/Handling-Muster (Timeline/Ladder, time-to-lose/gain,
Slot-Marker), Kern-Algorithmik (FCFS + Constrained Position Shifting,
Zwei-Horizont-Modell) und Architektur-Baukasten. **Kein Code** — reines
Recherche-Dokument mit ausgewiesener Belegqualität je Aussage:
`docs/research/AMAN-Marktanalyse.md`.

**Nachtrag (gleicher Tag, Betreiber-Auftrag):** Die im ersten
Recherche-Lauf einem Sitzungs-Limit zum Opfer gefallenen Gegenprüfungen
wurden in einer **zweiten Runde nachgeholt** (Workflow-Wiederaufnahme +
Direkt-Nachprüfung an den Primärquellen). Ergebnis: 10 weitere Aussagen
adversarial verifiziert (u.a. Intelligent Approach seit 2015 operativ,
E-AMAN validiert bis 200 NM + CP1-Pflicht, AMAN/DMAN = SESAR Solution #54
V3/TRL6), die übrigen Kernaussagen per Primärquellen-Korroboration
bestätigt (TBFM-Freeze-Mechanik, TTL/TTG-„L2"-Label-Muster, CPS,
SINOPTICA-Kern); eine Kennzahl („30 kg Fuel je Flug" für E-AMAN) war
nicht belegbar und wurde verworfen. Beleg-Marker im Dokument nachgezogen.

**Nächster Schritt:** Abstimmung mit dem Betreiber, wie der AMAN in die
Suite integriert wird (eigener Dienst vs. Backend-Modul, Datenbedarf ggü.
Firefly, Mandanten-Zuschnitt, v1-Umfang) → mündet in einen ADR. Erst nach
Freigabe wird gebaut.

---

## ✅ Stand 2026-07-22 (Pro-Mandant-Basiskarte — Bedien-UI, T3; ADR 0035 komplett)

**In normaler Sprache:** „Basiskarte pro Mandant" ist jetzt **end-to-end** fertig.
Im Admin unter **Mandanten → [Mandant]** ist die Konfiguration in **Tabs**
gegliedert (**Sicht · Freigaben · Kartendaten**). Im Reiter **Kartendaten** stellt
der Betreiber die **eigene Basiskarte** des Mandanten ein (Theme hell/dunkel +
Style-URL); „Auf Standard" setzt auf die globale Karte zurück. Der Lotse dieses
Mandanten sieht daraufhin seine Karte (Serving-Pfad aus T2).

**Fachlich/technisch:** T3 von ADR 0035 (rein Frontend). `AdminTenantDetail.vue`
in `v-tabs`/`v-window` umgebaut; neuer Kartendaten-Reiter mit Theme-Select +
Style-URL-Feld, „überschrieben/Standard"-Chip und Reset — ruft die T2-Endpunkte
`GET/PUT /api/admin/tenants/{id}/mapdata/basemap/{theme,style-url}`. Der bestehende
globale Speichern-Knopf (Sicht + Freigaben) bleibt und ist auf dem Kartendaten-
Reiter ausgeblendet (der speichert eigenständig). **Verifikation:** 760
Frontend-Tests grün (neu `adminTenantBasemap.test.js`), `vite build` + Dist neu;
Backend unverändert (T2). Doku: ADR 0035 (T3 ✅), FR-CFG-013.

**Epic-Stand:** T1 (#326) + T2 (#327) + T3 = **Pro-Mandant-Basiskarte komplett**.
Nächste mögliche Schritte (nicht begonnen): weitere Subsysteme pro Mandant nur bei
Bedarf (Hybrid-Scope hält Wetter/Sensoren global).

---

## 🗺️ Stand 2026-07-22 (Pro-Mandant-Basiskarte — Serving-Pfad, T2; ADR 0035)

**In normaler Sprache:** Jeder Mandant kann jetzt eine **eigene Basiskarte**
bekommen — eigenes Theme (hell/dunkel) und eigenen BKG-Style — passend zu seinem
Gebiet. Der Betreiber wählte Variante **A** (echte Pro-Mandant-Karte). T2 baut den
**Serving-Pfad**: der Karten-Style-Proxy liefert je Mandant die passenden
Karten-Bytes; ohne eigene Einstellung sieht ein Mandant unverändert die globale
Karte.

**Fachlich/technisch:** ADR 0035, Hybrid-Scope (nur Basiskarte pro Mandant; Wetter/
Sensoren bleiben global, Overlays bleiben Entitlement). Der `basemap.Service`
bekam einen **gekeyten Varianten-Cache** je (Style-URL, dark) — die Default-Variante
behält ihre Last-Good-Garantie, nur Abweicher landen im (auf 32) begrenzten Cache
(`StyleJSONFor`). `/basemap/style.json` + `/api/map-config` laufen jetzt hinter der
Tenant-Middleware und liefern das mandanten-effektive Theme + Style
(`Cache-Control: private`). Rangfolge **Mandant ?? global ?? Env**
(`tenant_map_settings` neben `platform_settings`, T1). Admin-Schreibpfad
`GET/PUT /api/admin/tenants/{id}/mapdata/basemap/{theme,style-url}` (SSRF-geprüft).
Der Frontend holt den Style same-origin (Session-Cookie); bei Ausfall greift der
synthetische Fallback. **Isolation getestet** (Mandant A ändert nie B).
**Verifikation:** `go test ./...` + `vet`/`gofmt` + `golangci-lint` (0 issues) grün;
neue Tests in `pkg/basemap/variant_test.go`, `pkg/mapconfig/tenant_test.go`,
`cmd/wayfinder/mapdata_tenant_test.go`. FR-CFG-013, ADR 0035, TECHNICAL §6.7.
**Nächster Schritt:** **T3** — Mandanten-Detail (`AdminTenantDetail.vue`) auf
**Tabs** umbauen + Pro-Mandant-Basiskarte-Editor (ruft den Admin-Schreibpfad).

---

## 🧱 Stand 2026-07-21 (Pro-Mandant-Kartendaten — Fundament, T1; Entscheidung offen)

**In normaler Sprache:** Vorarbeit, damit die Basiskarte (Theme/Style) später
**pro Mandant** einstellbar wird. Dieser Schritt baut nur das **Fundament**: eine
neue DB-Tabelle für Pro-Mandant-Overrides und die Logik, die den effektiven Wert
in der Rangfolge **Mandant → global → Env** auflöst — **isoliert** (Mandant A
ändert nie Mandant B, per Test bewiesen). Noch **keine** UI und noch **keine**
sichtbare Wirkung.

**Fachlich/technisch:** T1 zum Hybrid-Scope (Epic #307): neue Tabelle
`tenant_map_settings` (Migration 00023, getrennt von der globalen
`platform_settings` → globale Werte/versiegelter OpenAIP-Key unberührt),
`store.TenantMapSettingsRepo`, `mapconfig.TenantSetting` (Rangfolge Mandant ??
global ?? Env; Store-Fehler degradiert auf global). Tests:
`pkg/mapconfig/tenant_test.go` (Rangfolge, **Isolation** A≠B, Reset, tenantID 0 /
nil-Store, Degradation). Rein Library (wie K0), noch nicht in den Binärpfad
verdrahtet — kein CAT062-Bezug.

**Offene Entscheidung (Betreiber):** „Basiskarte pro Mandant" braucht einen
mandanten-fähigen Style-Proxy (die Karten-Bytes hell/dunkel + Style entstehen
heute **einmal global**). Optionen **A** (voller Pro-Mandant-Proxy, S4),
**B** (nur Vordergrund-Palette pro Mandant, S2–S3) oder **C** (global lassen, nur
Tabs-UI). Das Fundament passt zu A und B. **Nächster Schritt** hängt an der Wahl.

---

## 🌧️ Stand 2026-07-21 (Wetter an der AOI-Kante beschnitten — Bugfix #324, FR-UI-051)

**In normaler Sprache:** Die amtliche Basiskarte endet exakt am Kundengebiet
(AOI), das Wetter (v. a. das DWD-Regenradar) ragte aber darüber hinaus — es passte
nicht zur Kartenkante. Jetzt werden **alle** Kartendaten (Basiskarte + Radar +
Warnungen) an **derselben** scharfen AOI-Kante abgeschnitten; Tracks, Labels,
Coverage-Ringe und Aeronautik (ohnehin serverseitig aufs Gebiet zugeschnitten)
bleiben wie gewohnt sichtbar.

**Fachlich/technisch:** Ursache war die Z-Ordnung — die AOI-Maske (#289) lag
**unter** dem Wetter (bewusst: „begrenzt nur die Karte"), und das Radar-Raster
wird nur über kachel-granulare `bounds` beschnitten → „Bleed" über die Kante.
Fix (rein Frontend): Maske **über** Radar+Warnungen einhängen
(`engine.js`), Radar-Re-Add bei AOI-Wechsel stabil unter die Warnungen setzen
(`layers.js`, `beforeId`). `bounds`/Warnungs-Clip bleiben (weniger Overdraw). Doku:
ADR 0032 (Nachtrag), ASD-025-Milestone, FR-UI-051. **Verifikation:** 756
Frontend-Tests grün, `vite build` + Dist neu. **Nächste Schritte** (abgestimmt,
Hybrid-Scope): ADR + Pro-Mandant-Override-Modell für die „Kartendaten"-Konfig
(nur Basiskarte-Theme/Style echt pro Mandant) + Isolations-Negativtest, dann
Mandanten-Detail auf Tabs umbauen.

---

## 🛰️ Stand 2026-07-21 (CAT065-NOGO sichtbar — „Dienst degradiert", #261, FR-OPS-009)

**In normaler Sprache:** Wenn Fireflys Tracker hängt, sendet er weiter „Herzschlag",
markiert sich darin aber als **degradiert** (NOGO). Bisher sah der Lotse das nicht —
der Feed blieb grün. Jetzt wird ein solcher Feed **gelb** angezeigt und der
Status-Chip liest **„DIENST DEGRADIERT"**: das Lagebild dahinter kann eingefroren
sein, und das ist sofort erkennbar (nicht erst, wenn der Feed ganz verstummt).

**Technisch:** #261 (Konsumenten-Seite zu Fireflys SAFE.4). Der CAT065-NOGO-Zustand
(`ServiceStatus.Operational`, I065/040) wurde bereits dekodiert, aber im Konsumenten
verworfen. Jetzt: `health.Registry.RecordHeartbeat(feedID, now, operational)` →
`FeedSnapshot.SdpsDegraded` → `Color()` gelb; WS-Feld `feed_status.sdps_degraded`;
Frontend `asd.js` (`feedSdpsDegraded`) + `FeedStatusChip` („DIENST DEGRADIERT",
Vorrang vor „SENSOR AUSFALL"). Der Heartbeat setzt weiter die Staleness-Uhr zurück
(der Feed **lebt**), zählt aber nicht mehr als gesund. Kein Wire-Bruch (NOGO seit
ICD 2.3.0; kein ADR — Verwertung bestehender Semantik). **Verifikation:** volle
`go test ./...` + `vet`/`gofmt`/`golangci-lint` (0 issues) grün, 755 Frontend-Tests
grün, Dist neu gebaut/eingebettet. Register FR-OPS-009, Meilenstein-Doku, TECHNICAL
ergänzt.

**Nächster Schritt:** Review + Merge (schließt #261).

---

## 🎯 Stand 2026-07-21 (UI-/Konto-Bestandsaufnahme #315–#319 — ADR 0034, FR-UI-050 + FR-ADMIN-011)

**In normaler Sprache:** Fünf vom Betreiber gemeldete Bedien-Themen umgesetzt.
Vier am Layer-/Filter-Menü: (#315) ein Klick auf eine Gruppe wie „Aeronautik"
**schaltet ihre Layer an**, statt sie zu de-selektieren; (#316) das Menü-Panel ist
breiter, sodass die Beschriftungen („Minimal/Standard/Detailliert") passen; (#317)
es ist immer nur **eine** Gruppe aufgeklappt (kein Scrollen in der 2. Ebene);
(#318) die Menü-Icons **leuchten blau**, sobald ein Layer/Filter aktiv ist. Und
(#319): Jeder Nutzer kann jetzt **unter „Konto" E-Mail und Passwort selbst setzen**
— auch ein Lotse, nicht nur der Admin; die E-Mail wird im Admin-Panel sichtbar.

**Technisch:** #315 `nextMaster` „fill-then-clear" (`state !== 'on'`); #316 Panel
248→288 px + flexible Preset-Buttons; #317 `LayerGroup` kontrolliert
(`:expanded`/`@toggle`) + `openGroup`-Akkordeon in `LayerFilterContent`; #318
`hasActiveLayers`/`hasActiveFilter` + `nav-rail__btn--engaged`-Glow (Cyan). #319
(**ADR 0034**): neue **rollen-agnostische**, self-scoped Routen
`PUT /api/account/{password,email}` hinter `tenantMW`+`pwGate` (User-ID aus der
Session, nie aus dem Request) — der Admin-gegatete Zwangswechsel-Pfad
`/api/admin/me/password` bleibt unangetastet; `GET /api/whoami` trägt jetzt
`email`; UI: neuer `AccountSelfServiceDialog.vue` (ASD-Sidebar „Konto") +
E-Mail-Feld im Admin-„Mein Konto".

**Verifikation:** 736 Frontend-Tests grün (58 Dateien), volle Go-Suite grün,
`go vet` + `gofmt` + `golangci-lint` (0 issues) sauber, Dist neu gebaut/eingebettet.
Register: FR-UI-050, FR-ADMIN-011; Endpunkte in `TECHNICAL.md` ergänzt.

**Nächster Schritt:** Review durch den Betreiber; danach PR-Merge (schließt
#315–#319).

---

## 🗺️ Stand 2026-07-20 (Kartendaten live editierbar — K2–K6, Epic #307 abgeschlossen)

**In normaler Sprache:** Der „Kartendaten"-Bereich im Admin ist jetzt **voll
bedienbar** statt nur Anzeige. Der Betreiber stellt alle vier Quellen direkt in
der Oberfläche ein — **ohne Deployment, ohne Env-Datei**: die **Basiskarte**
(Style + hell/dunkel, wirkt sofort), das **Wetter** (DWD-Radar/-Warnungen/QNH
an/aus + Server-URLs), die **Radar-Abdeckung** (Sensor-Standorte + Ringfarbe,
wirkt sofort) und die **Aeronautik** (OpenAIP Fetch-Radius + Server-URL; der
Schlüssel bleibt sicher versiegelt). Wo ein Wert erst beim nächsten Neustart
greift, sagt die UI das **ehrlich** dazu. Die Umgebungsvariablen bleiben als
**Standardwerte** gültig — ein Override in der UI gewinnt, „Auf Standard" fällt
zurück.

**Fachlich/technisch:** K2–K5 des Epics **#307** auf der K0-`mapconfig`-Plane
(ADR 0033), plus K6 (Doku-Abschluss). Neue Plane `cmd/wayfinder/mapdata.go`
(Settings = DB-Override ?? Env-Default; `/api/map-config` + Coverage-GeoJSON
lesen effektiv pro Request); Admin-Endpunkte `/api/admin/mapdata/*` hinter
`RequireRole(admin)`. **Live:** Basiskarte (`basemap.Service.Reload`, letzte gute
Konfig bei Fehler, ehrlicher `reload_error`), Wetter-An/Aus + Verfügbarkeit,
Abdeckung. **Beim Neustart:** Wetter-URLs/Layer + OpenAIP-Radius/Base-URL (die
sperrfreien Poll-Dienste werden aus den effektiven Werten neu gebaut, ein
laufender Feed wird nicht im Betrieb umkonfiguriert). SSRF-Leitplanken für
gefetchte URLs; OpenAIP-Key bleibt versiegelt (`pkg/secret`). **K6-Doku:**
INSTALLATION (Env = Default, überschreibbar-Marker + Callout), TECHNICAL §6.7
(Precedence DB > Env, Hot-Reload vs. Restart, `platform_settings`-Schlüssel-
Tabelle, SSRF, Verfügbarkeits-Signale), Glossar-Eintrag, Milestone
`K2-K5_Kartendaten_live_editierbar.md`, Register FR-CFG-009…012. **Verifikation:**
`go test/vet/gofmt` grün, 737 Frontend-Tests grün, `vite build` + Dist neu
eingebettet. **Nächster Schritt:** Epic #307 nach dem Merge abschließen
(#308–#314 schließen).

---

## 🗂️ Stand 2026-07-20 (Admin-Bereich „Kartendaten": Rahmen + Status — K1, FR-CFG-008)

**In normaler Sprache:** Im Admin gibt es jetzt einen neuen Bereich **„Kartendaten"**
mit vier Reitern — **Basiskarte, Wetter, Radar-Abdeckung, Aeronautik**. Er zeigt
auf einen Blick, **was konfiguriert und verfügbar** ist (z. B. „DWD-Regenradar
verfügbar", „3 Sensoren konfiguriert", Theme/Style der Karte). Der bisherige
„OpenAIP"-Bereich ist als Reiter **Aeronautik** hier eingezogen. In diesem Schritt
ist alles **nur Anzeige** — das direkte Bearbeiten der Werte folgt je Quelle.

**Fachlich/technisch:** K1 des Epics **#307** (Issue #309), auf K0 (`pkg/mapconfig`)
aufbauend. Neue `AdminMapData.vue` (vier `v-tabs`), Status aus **demselben**
`/api/map-config`, das das ASD beim Start liest (Single Source of Truth); Reiter
Aeronautik bettet das bestehende `AdminGlobalOpenAIP.vue` ein (keine Doppelung).
Read-only. **Verifikation:** 731 Frontend-Tests grün, `go build` grün, Dist neu
eingebettet. **Nächster Schritt:** K2 (#310) — Basiskarte live editierbar
(Style-URL/Theme), dann K3 Wetter · K4 Abdeckung · K5 Aeronautik-Felder · K6 Doku.

---

## 🛠️ Stand 2026-07-20 (Config-Plane für Kartendaten — K0, ADR 0033, FR-CFG-007)

**In normaler Sprache:** Fundament für ein neues Admin-Vorhaben (#307): Wetter,
Basiskarte, Radar-Abdeckung und Aeronautik sollen sich später **im Admin live
einstellen** lassen (heute meist nur über Env + Neustart). K0 baut noch **keine
Oberfläche**, sondern die wiederverwendbaren Bausteine darunter: eine Einstellung
kann in der Datenbank überschrieben werden (mit dem Env-Wert als Rückfall), eine
Änderung lädt den betroffenen Dienst **ohne Neustart** neu (und behält bei Fehler
die letzte gute Konfig), und vom Admin gesetzte URLs werden vor dem Speichern
**auf Sicherheit geprüft** (kein Zugriff auf interne Adressen — SSRF-Schutz).

**Fachlich/technisch:** K0 des Epics **#307** (Issue #308), ADR 0033. Neues Paket
`pkg/mapconfig` (rein, unit-getestet): `Setting` (DB-Override ?? Env-Default,
Reset, Store-Fehler → Default), `Registry`+`ReloadFunc` (defensives Hot-Reload je
Domain), `ValidateFetchURL` (SSRF-Leitplanken), `Resource.Handler` (generischer
GET/PUT-Admin-Endpunkt, Reload-Fehler ehrlich als `reload_error`). **12-Factor
bleibt gültig** (frisches Deployment ohne DB-Config = wie bisher). Secrets bleiben
versiegelt (nicht in dieser Plane). **Verifikation:** `go test ./pkg/mapconfig`
grün, `go vet`/`gofmt` sauber, `go build ./...` grün. **Ehrliche Grenze:**
DNS-Rebinding-SSRF nicht abgedeckt (Trusted-Admin-Modell, dokumentiert).
**Nächster Schritt:** K1 (#309) — Admin-Abschnitt „Kartendaten" + Status-Anzeige,
dann K2–K5 (Subsysteme live) → K6 (Doku).

---

## 🎯 Stand 2026-07-20 (BKG-Basiskarte auf die Mandanten-AOI begrenzt — ASD-025, #289, ADR 0032, FR-UI-049)

**In normaler Sprache:** Bisher war die amtliche Basiskarte flächig (Deutschland/
Welt) zu sehen — auch weit außerhalb des Kundengebiets. Tracks und Wetter waren
aber schon auf das **Zuständigkeitsgebiet (AOI)** begrenzt. Jetzt endet auch die
**Karte am Sektorrand**: außerhalb der AOI wird sie ausgeblendet (mit der dunklen
Scope-Farbe überdeckt). So bleibt der Blick auf das eigene Gebiet konzentriert und
Karte/Tracks/Wetter enden an derselben Kante. Ist beim Kunden keine AOI hinterlegt,
bleibt die Karte vollflächig.

**Fachlich/technisch:** ASD-025 / ADR 0032. **Form = Rechteck aus der vorhandenen
AOI-BBox** — deckt sich exakt mit dem server-seitigen Track-/Wetter-Zuschnitt und
braucht kein neues Konfig-Feld. (Der Betreiber denkt die AOI als **Radius/Kreis**;
das ist als Folge-Option festgehalten — der Code ist vorbereitet, ein Kreis tauscht
nur den Loch-Ring in `aoiMaskFeature`. Die Zuschnitt-Form-Rückfrage war wegen einer
Tool-Störung nicht interaktiv möglich; gewählt wurde die konsistente, reversible
Rechteck-Variante.) **Mittel:** eine Masken-Fill-Ebene (Welt-Polygon mit AOI-Loch,
`map/clip.js`), über der Karte, unter allen Overlays — begrenzt nur die Karte, nie
Tracks/Wetter/Aeronautik. **Verifikation:** 725 Frontend-Tests grün, `go build`/
`vet` grün, Dist neu eingebettet. **Ehrliche Grenze:** harte rechteckige Kante
(Kreis/Radius + weicher Rand = dokumentierte Folge-Optionen #289); optische Abnahme
durch den Betreiber.

---

## ✨ Stand 2026-07-20 (BKG-Element-Presets „Minimal/Standard/Detailliert" — ASD-024, E3, FR-UI-048; Epic #290 abgeschlossen)

**In normaler Sprache:** Statt die acht Element-Schalter der Basiskarte einzeln
zu stellen, gibt es jetzt drei **1-Klick-Profile** darüber: **Minimal** (nur
Gewässer/Grenzen/Beschriftung auf dem Scope), **Standard** (dazu Verkehr +
Hintergrund) und **Detailliert** (alles an). Ändert der Lotse danach einen
Schalter, steht die Auswahl auf „Benutzerdefiniert". Damit ist das BKG-Element-
Feature rund: nach Elementen schalten, per Preset schnell wählen, und alles bleibt
im View-Profil erhalten.

**Fachlich/technisch:** ASD-024, E3 (letzte Komfort-Stufe) des Epics **#290**
(Issue #294). `BASEMAP_PRESETS` + `matchPreset` in `map/basemapGroups.js`
(unit-getestet), Store-Action `applyBasemapPreset` (gebündelte Mutation → ein
`applyBasemap`), kompakte Preset-Button-Reihe in der Sidebar (nur bei Karte-an,
aktiver Preset hervorgehoben). Persistenz kostenlos über E4 (Preset setzt nur
Element-Zustände). **Verifikation:** 719 Frontend-Tests grün, `go build`/`vet`
grün, Dist neu eingebettet.

**Epic #290 (BKG-Karte in Elemente unterteilen) ist damit abgeschlossen** —
E0 (Bucketing) → E1 (Sidebar-Gruppen) → E2 (Element-Schalter) → E4 (Persistenz)
→ E3 (Presets). Optionaler Rest E5 (Panel-Suche/Straßenklassen) ist nicht geplant.

---

## 💾 Stand 2026-07-20 (BKG-Element-Auswahl im View-Profil gespeichert — ASD-023, E4, FR-UI-047)

**In normaler Sprache:** Die Element-Schalter der Basiskarte (aus E2) blieben nach
einem Reload nicht erhalten — es standen wieder alle auf an. Jetzt **speichert das
View-Profil die Auswahl mit**: Wer sich eine entclutterte Karte einstellt (z. B.
Gebäude und Beschriftung aus), findet sie nach Reload oder Profilwechsel genauso
wieder. Standardmäßig (und bei älteren Profilen) sind weiterhin alle Elemente an.

**Fachlich/technisch:** ASD-023, E4 des Epics **#290** (Issue #295). Reine
Ergänzung der Profil-Serialisierung (`stores/profileSettings.js`), gespiegelt zur
`airspaceGroups`-Behandlung: `captureSettings` schreibt `basemapElements`,
`applySettings` liest sie tolerant zurück (unbekannte Schlüssel übersprungen,
älteres Profil ohne Abschnitt lädt fehlerfrei). Die Karte folgt über den
bestehenden MapCanvas-Element-Watcher. **Verifikation:** 710 Frontend-Tests grün,
`go build`/`vet` grün, Dist neu eingebettet. **Damit ist der praktische Kern des
BKG-Element-Features rund** (E0–E2 + E4). **Optional offen:** Presets
„Minimal/Standard/Detailliert" (E3/#294) — reine Bequemlichkeit.

---

## 🗺️ Stand 2026-07-20 (BKG-Basiskarte: Element-Schalter „nur Flüsse/Straßen" — ASD-022, E2, FR-UI-046)

**In normaler Sprache:** Der ursprüngliche Wunsch ist erfüllt — die amtliche
Basiskarte lässt sich jetzt **nach Elementen** ein-/ausblenden. Unter „Karte" in
der Seitenleiste gibt es acht Unterschalter (**Gewässer, Verkehr, Vegetation,
Siedlung, Gebäude, Grenzen, Beschriftung, Hintergrund**). So kann der Lotse z. B.
„nur Flüsse" oder „nur Straßen" zeigen. Die Unterschalter sind ausgegraut, solange
die Karte insgesamt aus ist (der bekannte #274-Schalter „Basiskarte" bleibt der
Hauptschalter). Standardmäßig sind alle Elemente an — es ändert sich also nichts,
bis man bewusst etwas ausblendet.

**Fachlich/technisch:** ASD-022, E2 des Epics **#290** (Issue #293), auf E0
(#291, Bucketing) + E1 (#292, Gruppen) aufbauend. Kein Style-Wechsel: neue
Engine-Funktion `applyBasemap()` schaltet jede Ebene sichtbar **gdw. Master an UND
Element-Gruppe an** (unerkannte `other`-Ebenen folgen dem Master). Store
`basemapElementVisibility` (alle an Default) + `setBasemapElement`; MapCanvas-
Watcher wendet Element-Änderungen sofort an; Sidebar zeigt die acht Unterschalter
(deaktiviert bei Karte-aus). **Verifikation:** 707 Frontend-Tests grün, `go build`/
`vet` grün, Dist neu eingebettet. **Offen:** Persistenz der Element-Auswahl im
View-Profil (E4/#295) — nach Reload sind wieder alle Elemente an; danach optional
Presets (E3/#294). **Ehrliche Grenze:** Ebene→Element-Zuordnung muster-basiert,
Feinjustierung am echten `/basemap/style.json` durch den Betreiber; optische
Abnahme durch den Betreiber (kein WebGL-Test).

---

## 🧩 Stand 2026-07-20 (BKG-Basiskarte: Element-Bucketing — ASD-021, E0, FR-UI-045)

**In normaler Sprache:** Vorbereitung dafür, die amtliche Basiskarte später in
**einzelne Elemente** zerlegbar zu machen (nur Flüsse, nur Straßen …). Damit die
kommenden Schalter (E2/#293) wissen, *welche* Karten-Ebene zu *welchem* Element
gehört, ordnet dieser Schritt jede Ebene der Vektorkarte einer **Element-Gruppe**
zu (Gewässer, Verkehr, Vegetation, Siedlung, Gebäude, Grenzen, Beschriftung,
Hintergrund). **Noch ohne Bedienoberfläche** — reine Grundlage. Weil die
Ebenen-Namen der BKG mit Updates wechseln, geschieht die Zuordnung **per Muster**
(nicht per fester Namensliste), und alles Unerkannte landet sicher in „Sonstiges",
sodass nie eine Ebene verloren geht.

**Fachlich/technisch:** ASD-021, E0 des Epics **#290** (Issue #291). Neu
`map/basemapGroups.js` (`classifyBasemapLayer`/`bucketBasemapLayers`, rein +
unit-getestet gegen einen Fixture aus basemap.de- **und** basemap.world-Namen);
`engine.js` bucketet beim Style-Load (`state.basemapGroups`) und exponiert
`setBasemapGroupVisibility(group, visible)` — den Schalthebel für E2, **noch von
keiner UI aufgerufen**. Ein Symbol-Layer ist immer „Beschriftung" (Regel zuerst).
**Verifikation:** 701 Frontend-Tests grün (neu `basemapGroups.test.js` + E0-Block
in `basemapLayer.test.js`), `go build`/`vet` grün, Dist neu eingebettet. **Ehrliche
Grenze:** Live-BKG-Style von hier nicht prüfbar (Proxy 403) → Feinjustierung der
Muster am echten `/basemap/style.json`, bis dahin fängt „Sonstiges" alles auf.
**Nächster Schritt:** E2/#293 — die Element-Schalter im „Karte"-Abschnitt an
`setBasemapGroupVisibility` hängen.

---

## 🗂️ Stand 2026-07-20 (Layer-Sidebar in aufklappbare Gruppen — ASD-020, ADR 0031, FR-UI-044)

**In normaler Sprache:** Der „Layer"-Bereich der Seitenleiste war eine lange,
flache Reihe von Schaltern. Jetzt sind sie in **vier benannte, aufklappbare
Gruppen** einsortiert — **Aeronautik**, **Karte**, **Radar & Reichweite**,
**Wetter** — jede mit einem **Sammel-Schalter** oben, der die ganze Gruppe auf
einen Klick ein-/ausblendet (drei Zustände: alles an / aus / teilweise). Man kann
Gruppen zuklappen, die man nicht braucht. Das ist die Vorbereitung dafür, die
Basiskarte später in einzelne Elemente (nur Flüsse, nur Straßen …) zerlegbar zu
machen (#293) — dann gibt es mehr Schalter, und die Gliederung hält das bedienbar.

**Fachlich/technisch:** ASD-020 / ADR 0031, erster Schritt (E1) des BKG-Element-
Epics **#290** (Issue #292). Spiegel zu den Overlay-Zonen am Scope-Rand (ADR 0029),
jetzt fürs Panel: neue `LayerGroup.vue` (aufklappbarer Rahmen + tri-state Master),
schema-agnostische Tri-State-Logik in `map/layerGroups.js` (unit-getestet), die
vier Gruppen in `LayerFilterContent.vue` mit Member-Modell. Der Master routet über
**denselben** Store-Pfad wie die Zeilen-Switches (kein toter Toggle); ein
deaktivierter Toggle (Quelle nicht verfügbar) ist aus Master-Zustand + Bulk-Aktion
ausgeschlossen. **Verbindliche Regel (ADR 0031):** neues Layer-Chrome kommt als
Zeile in eine Gruppe, nie als loser Schalter. **Verifikation:** 669 Frontend-Tests
grün (neu: `layerGroups.test.js`, `layerGrouping.test.js`), `go build`/`vet` grün,
Dist neu eingebettet; optische Abnahme durch den Betreiber. **Offen (spätere
E-Stufen):** Accordion, Presets (#294), Persistenz im View-Profil (#295), dann die
BKG-Element-Schalter selbst (E2/#293, braucht E0/#291-Bucketing).

---

## 🎨 Stand 2026-07-19 (Gruppierte Rail, Orange/Blau-Farbcode, Zoom auf die Karte — ASD-019, ADR 0030, FR-UI-040)

**In normaler Sprache:** Die schmale Werkzeug-Leiste links am Lagebild war eine
flache Reihe gleich aussehender Symbole. Jetzt ist sie **in zwei benannte Gruppen
geteilt** — **MEASURE** (die Mess-Werkzeuge RBL/DIST/QDM) und **MAP** (die
Karten-Panels Layer/Filter) — jede unter einem kleinen Titel. Die beiden Familien
sind **farblich getrennt**: Ein scharf gestelltes **Mess-Werkzeug leuchtet
bernstein/orange** (Warnfarbe — passt, weil es dann die Karten-Klicks „an sich
zieht", ein besonderer Modus), ein offenes **Karten-Panel leuchtet cyan/blau**
(normaler Zustand). Das aktive Symbol bekommt einen weichen Schein, damit man den
aktiven Zustand sofort sieht; das **Konto** sitzt klar abgesetzt ganz unten. Die
**Zoom-Knöpfe (+/−)** sind aus der Leiste **auf die Karte gewandert** — an die
**untere rechte Ecke**, dort wo sie wirken. Die Leiste wird dadurch kürzer und
trägt nur noch echte Lotsen-Werkzeuge.

**Fachlich/technisch:** ADR 0030. Ein Mess-Werkzeug ist **modal** (fängt
Karten-Klicks), ein Panel **nicht-modal** — vorher sahen beide gleich aus (cyan).
Die **zwei Aktiv-Farben** kodieren genau diesen Unterschied (Sicherheit: scharfer
Mess-Modus ≠ offener Layer). `NavigationRail` bekommt `.nav-rail__section`-Mikro-
Labels + `--tool`/`--panel`-Gruppen; `--tool`-aktiv → `--wf-warning` +
`--wf-state-armed` + `--wf-glow-armed`, `--panel`/Konto → Cyan + `--wf-glow-
selected`; dezente Dauer-Akzentleiste je Familie; Konto per Push-Divider
(`margin-top:auto`) an den Fuß. Zoom: neue positions-neutrale `ZoomControls.vue`,
gehostet von `MapControls` (jetzt die **bottom-right-Zone** auf Desktop **und**
Mobil, erweitert ADR 0029); `MapCanvas` rendert sie unbedingt und verdrahtet Zoom
an die Engine; Recenter/Vollbild bleiben desktop-seitig in der top-right-Zone.
`TrackDetailPanel` reserviert die Zoom-Höhe (`--wf-map-controls-reserve`), damit
sich Karte und Zoom am rechten Rand nie überlappen. Neue Tokens in
`colors.css`/`spacing.css`. Kein CAT062-/Backend-/ICD-Bezug (reines Chrome).
Tests: `railTools.test.js` neu geschnitten, `scopeChromeLayout.test.js` +
`responsive.test.js` angepasst; **vitest 648 grün, vite build grün, dist neu**.

**Offen (ehrliche Grenze):** Kein WebGL-/Mount-Harness in der Sandbox — die
**optische Abnahme** macht der Betreiber nach `git pull` + Frontend-Rebuild:
Farb-Codierung (amber Werkzeug / cyan Panel), Glow der aktiven Symbole,
Zoom-Position unten rechts, keine Überlappung mit der Track-Detail-Karte,
sauberer Abstand zur Attribution-ⓘ in der Ecke.

**Nächster Schritt:** Betreiber-Sicht-Abnahme des neuen Rail-/Zoom-Layouts.
Ansonsten weiter mit dem Bridge-Orchestrator-Test.

---

## 🧩 Stand 2026-07-19 (ASD-Chrome-Overlay-Zonen: Schluss mit überlappenden Bedien-Elementen — ASD-018, ADR 0029, FR-UI-039)

**In normaler Sprache:** Beim Mac-Test fiel dem Betreiber auf, dass die
Such-Lupe die Vollbild-/Zentrieren-Knöpfe überlagert — und dass das schon
mehrfach passiert ist, wenn neue Funktionen dazukamen. Ursache war
strukturell: Am rechten Rand lagen zwei **unabhängig** positionierte
Element-Stapel, und die Karten-Controls hingen an einem **fest verdrahteten**
Abstand, der die Höhe des oberen Clusters nur riet — jedes neue Icon brach
diese Annahme. Statt wieder einen Zahlenwert nachzuziehen (das haben wir bei
#194 schon getan), ist der rechte Rand jetzt **eine durchgehende Spalte**: Die
Controls sind das letzte Element darin und rutschen automatisch unter alles,
was oben dazukommt. Überlappung strukturell erledigt. Dazu eine dokumentierte
**Regel**: Neues Bedien-Chrome kommt immer in so eine Zone hinein, nie als
frei schwebendes Einzel-Element mit geratenem Abstand.

**Fachlich/technisch:** Overlay-Zonen (ADR 0029). Recenter/Vollbild in eine
positions-neutrale `ViewportControls.vue` extrahiert (kein eigener Offset); die
Zone (Desktop-Rail in `AsdView`) legt sie als letztes Flex-Kind aus. `MapControls`
ist jetzt mobil-only (Zoom + `ViewportControls`, unten rechts), der geratene
`top:140px` ist weg; `MapCanvas` rendert `MapControls` nur `!mdAndUp` und
exponiert `recenter`. Kein Doppel-Code (mobil + Desktop teilen `ViewportControls`).
Register FR-UI-039, ADR 0029, Milestone ASD-018. Tests: neuer
`scopeChromeLayout.test.js` (Struktur-Garantien) + angepasste
responsive/railTools/mapCanvasViewCenter; vitest 643.

**Offen (ehrliche Grenze):** Kein WebGL-Render in der Sandbox — die **optische
Abnahme** macht der Betreiber (Such-Lupe + Controls überlappen nicht mehr,
Controls sitzen sauber unter dem Cluster). Transiente Panels (Ereignis-Log,
aufgeklappte Suche) schieben die Controls beim Öffnen im Fluss nach unten —
korrektes Verhalten; optional könnte man sie später als Overlays aus dem Fluss
nehmen, damit die Controls völlig ruhig bleiben.

**Nächster Schritt:** Betreiber-Sicht-Abnahme des Layouts (nach `git pull` +
Frontend-Rebuild). Ansonsten weiter mit dem Bridge-Orchestrator-Test.

---

## 🌉 Stand 2026-07-19 (Portables Auto-Orchestrierungs-Profil im Bridge-Netz — FR-CFG-006, Opus 4.8)

**In normaler Sprache:** Beim Mac-mini-Aufbau fiel auf: Der gewohnte
Auto-Spawn-Betrieb (Feed zuweisen → Firefly startet automatisch mit passender
Adresse) lief nur auf Codespace, weil das orchestrierte Profil
Host-Networking braucht — das kann macOS nicht. Auf dem Mac blieb nur das
statische Bridge-Compose (eine feste Firefly, Adresse von Hand angleichen).
Der Betreiber will aber **eine** Setup-Weise, die auf Mac **und** Codespace
gleich läuft. Neu: **`docker-compose.orchestrated-bridge.yml`** — der volle
Orchestrator-Betrieb (Server + Orchestrator + auto-gespawnte Firefly je Feed),
aber alles in **einem Bridge-Netz** (`asd-net`) statt Host-Networking. Bridges
leiten Multicast zwischen ihren Mitgliedern weiter, also erreichen die
gespawnten Firefly den Server ohne Host-Netz — und die automatische
Gruppenvergabe je Feed bleibt. Läuft damit auf Mac, Windows und Codespace/Linux
identisch.

**Fachlich/technisch:** Kein Go-Umbau nötig — der Orchestrator konnte das
Zielnetz der gespawnten Container schon frei setzen
(`WAYFINDER_FIREFLY_NETWORK` → `dockerbackend.New(networkMode)` →
`HostConfig.NetworkMode`, nimmt einen Netznamen). Das neue Profil setzt es auf
`asd-net` (`networks.asd.name`), legt Server/Orchestrator/DB in dasselbe Netz,
veröffentlicht 8081/8080. Firefly wird als `firefly:latest` gespawnt
(Voraussetzung: `docker build -t firefly:latest ../firefly`). Per-Feed-Gruppe
(`feed_alloc.go`) + Weitergabe an die Firefly (`fireflyEnv`) unverändert →
Sender/Empfänger passen je Feed automatisch. Doku: INSTALLATION Schritt 4.C
(inkl. Codespace↔Mac-Hinweis, Aufräumen), Orchestrierungs-Hinweise nachgezogen,
Register FR-CFG-006. Host-Net-Profil bleibt für reine Linux-Hosts.

**Offen (ehrliche Grenze):** Die Sandbox kann keinen Docker-Build/Runtime
ausführen — `docker compose config` validiert die Datei, aber die
**End-to-End-Abnahme** (Feed zuweisen → Tracker spawnt im Bridge → Tracks
kommen an) macht der Betreiber auf dem Mac. Falls dabei etwas klemmt
(Multicast im Bridge, Netz-Attach der gespawnten Container), nachsteuern.

**Nächster Schritt:** Betreiber-Abnahme des Bridge-Orchestrierungs-Profils auf
dem Mac: `docker build -t firefly:latest ../firefly` +
`docker compose -f docker-compose.orchestrated-bridge.yml up -d --build`, Feed
zuweisen, prüfen dass der Tracker spawnt und Tracks erscheinen.

---

## 📘 Stand 2026-07-19 (INSTALLATION.md nachgezogen: BKG-Theme in den Beispiel-Composes + Karte/Suche einschalten — Doku-Currency, Opus 4.8)

**In normaler Sprache:** Betreiber-Befund bei der Mac-mini-Umzugsplanung: Die
Beispiel-Compose-Dateien in `INSTALLATION.md` setzten `WAYFINDER_MAP_THEME`
nicht, und der Einrichtungs-Walkthrough erklärte nirgends, dass die Karte per
Default aus ist (synthetischer Scope, #274) — wer der Anleitung folgte, sah am
Ende einen schwarzen Scope und wunderte sich. Die Env-Referenz-Tabellen waren
zwar aktuell, die Beispiele + der Walkthrough aber veraltet. Nachgezogen:
(a) `WAYFINDER_MAP_THEME`/`WAYFINDER_BKG_STYLE_URL` in **beide** Beispiel-
Composes (Schritt 4.A + 4.2) aufgenommen; (b) neuer **Schritt 4.10a**
„Basiskarte (BKG) + Sektor-Suche einschalten" (Feature `basemap` freigeben →
Layer togglen → Suche nutzen); (c) zwei Fehlersuch-Zeilen („Scope schwarz",
„Such-Icon fehlt"). Klargestellt außerdem: `docker-compose.orchestrated.yml`
lässt sich **nicht** auf den Mac verschieben (Host-Networking + Auto-Spawn +
repo-relative Build-Kontexte) — der portable Weg ist das Bridge-Master-Compose.

**Nächster Schritt:** Keiner offen aus diesem Doku-Schritt; der Betreiber
richtet den Mac-mini-Stack nach der aktualisierten Anleitung ein.

---

## 🔍 Stand 2026-07-19 (Sektor-Suche: nur bei aktivem BKG-Layer + aufklappbares Icon — Nachtrag 3 zu FR-UI-037, Opus 4.8)

**In normaler Sprache:** Zwei Bedien-Wünsche des Betreibers, damit der Scope
frei auf die Tracks bleibt: (1) Die Suche soll nur erscheinen, wenn die
BKG-Karte tatsächlich *eingeschaltet* ist — ohne sichtbare Karte gibt es
nichts zu verorten. (2) Im Ruhezustand soll nur ein kleines Lupen-Icon zu
sehen sein; ein Klick fährt das Suchfeld aus, nach der Treffer-Wahl klappt es
wieder zusammen. Beides umgesetzt: Die Suche ist jetzt an den Layer-Schalter
gekoppelt (nicht mehr an die bloße Freigabe), und das Suchfeld ist
standardmäßig zu einem Icon eingeklappt.

**Fachlich/technisch:** `showSearch = store.layerVisibility.basemap === true`
(reaktiv am Sidebar-Toggle); ein `watch` räumt beim Abschalten den Treffer-
Marker auf. `MapSearch.vue` hat einen Ausklapp-Zustand (`expanded`): Icon-Button
→ `v-expand-x-transition` → Feld mit Auto-Fokus; Zuklappen bei Treffer-Wahl
(`onSelect`), Esc/× (`onClose`) und leerem Blur (`onBlur`, guard verhindert
Klau des Treffer-Klicks). Tests: Gate-Source-Guard (Layer-Kopplung +
Marker-Aufräumen), Icon→Feld-Aufklappen, Zuklappen nach Treffer (via
exponiertem `expanded`). Register: FR-UI-037-Nachtrag-3. vitest 638, dist neu.

**Nächster Schritt:** Betreiber-Sicht-Abnahme: Icon nur bei aktivem
BKG-Layer, Klick fährt Feld aus, Treffer-Wahl klappt zu. Weiterhin offen:
Label-Flacker-Fix (Sicht-Abnahme), Treffer-Kontext/Zoom (Nachtrag 2).

---

## 🎯 Stand 2026-07-19 (Sektor-Suche: Treffer unterscheidbar + Zoom auf Ziel — Nachtrag 2 zu FR-UI-037, Opus 4.8)

**In normaler Sprache:** Zwei Bedien-Rückmeldungen des Betreibers nach der
ersten funktionierenden Suche: (1) Fünf identische „Forststraße"-Zeilen waren
nicht auseinanderzuhalten. (2) Ein Treffer wurde zwar zentriert, aber die
Karte blieb herausgezoomt — die Straße war unauffindbar. Beides behoben:
Jede Trefferzeile trägt jetzt ein Ortsmerkmal — den nächstgelegenen Ort
(„bei Wegberg") plus Peilung und Entfernung vom Sektorzentrum („8,2 NM ·
295°"). Und ein Klick auf einen Treffer fährt die Kamera nicht nur hin,
sondern stellt einen festen, sinnvollen Zoom ein (Straßenebene) — egal ob du
vorher zu weit draußen oder zu nah dran warst.

**Fachlich/technisch:** Server-seitig reichert `enrichHits` jeden Treffer an:
Radial (Haversine-Entfernung NM + Anfangs-Peilung ° vom bbox-Zentrum, immer
verfügbar) und der nächste Ort ≤ 8 km aus einer schema-tolerant gefilterten
Siedlungs-Teilmenge (`filterPlaces`/`isPlaceCategory` — best-effort, leer bei
abweichendem Tile-Schema → Zeile zeigt dann nur das Radial, genau die vom
Betreiber gewählte graceful degradation). Ergebnisfelder additiv
`near`/`dist_nm`/`bearing_deg`. Frontend: `hitDetail(h)` baut
`Kategorie · bei Ort · NM · Peilung`, fehlende Teile fallen weg;
`showSearchMarker` nutzt `flyTo` mit **absolutem** `SEARCH_RESULT_ZOOM=14`
(zoomt in beide Richtungen). Tests: Distanz/Peilung, `isPlaceCategory`,
`enrichHits` (Ort + Radial + Nicht-Anhängen entfernter Orte), Frontend-Zeile
mit/ohne Kontext, Zoom-Source-Guard. Register: FR-UI-037-Nachtrag-2.

**Nächster Schritt:** Betreiber-Sicht-Abnahme („Forststraße" → unterscheidbare
Zeilen, Klick zoomt aufs Ziel). Ggf. Kategorie-/Ort-Labels ans reale
BKG-Schema feinschleifen. Label-Flacker-Fix (weiter unten) ebenfalls noch
offen zur Sicht-Abnahme.

---

## 🔧 Stand 2026-07-19 (Sektor-Suche: TileJSON-Fix + ehrlicher Fehler-Status — Nachtrag zu FR-UI-037)

**In normaler Sprache:** Der Betreiber-Smoke-Test der Sektor-Suche schlug fehl —
„Suchindex wird aufgebaut …" ohne Ende. Die Server-Logs zeigten die Ursache
sauber: Der echte basemap.world-Stil gibt seine Kachel-Adresse als **Verweis
auf eine TileJSON-Datei** an (`url`), nicht direkt eingebettet (`tiles`) —
der Index-Builder las nur die eingebettete Form und scheiterte bei jedem
Versuch sofort („style has no vector tile source"), während die UI den
Fehler nie zu sehen bekam. Beides ist behoben: Der Builder folgt jetzt dem
TileJSON-Verweis (defensiv: größen-limitiert, zeit-begrenzt), und ein
fehlgeschlagener Erst-Bau erscheint in der UI als ehrliches „Suche derzeit
nicht verfügbar — neuer Versuch läuft …" statt als ewiger Aufbau-Hinweis.

**Fachlich/technisch:** `tilesTemplate` löst beide Quell-Formen auf
(inline `tiles` zuerst, sonst TileJSON-`url`-Fetch); `Search` meldet einen
nie erfolgreich gebauten Index mit Fehler stabil als `status:"error"`
(sticky über Hintergrund-Retries — kein Flackern error↔building), Handler
liefert das als 200 mit Status-Feld; `MapSearch.vue` zeigt den neuen
Zustand und pollt gedrosselt (3 s) weiter. Genau diese Lücke war die
dokumentierte „ehrliche Grenze" (Sandbox erreicht das BKG nicht — die
TileJSON-Indirektion war in den Fixtures nicht abgebildet); die Fixtures
decken jetzt beide Formen ab. Register: FR-UI-037-Nachtrag; TECHNICAL
aktualisiert. **Offen:** Betreiber-Smoke-Test Wiederholung („Forststraße").

**Nächster Schritt:** Betreiber wiederholt den Such-Smoke-Test; parallel
steht die Sicht-Abnahme des Label-Flacker-Fixes (unten) aus.

---

## ✨ Stand 2026-07-19 (Label-Flacker-Fix: `fadeDuration: 0` — FR-UI-038)

**In normaler Sprache:** Betreiber-Meldung: Bei jedem Track-Update wurden die
Datenblock-Labels für einen kurzen Moment leer. Verdacht war das neue
Live-Detail-Panel — tatsächlich war es MapLibres eingebaute
Label-**Einblend-Animation**: Die Karten-Engine behandelt ein Label mit
geändertem Text (neue Flugfläche) als „neues" Symbol und blendet es über
300 ms ein; sichtbar wurde das erst durch die viel höhere Beschriftungs-Last
der BKG-Vektor-Basiskarte. Fix: Die Blende ist jetzt abgeschaltet
(`fadeDuration: 0`) — getauschte Labels stehen im selben Frame. Das ist die
saubere Lösung, kein Workaround: Der Track-Label-Layer verzichtet seit
ASD-002 bewusst auf MapLibres Karten-Label-Logik (eigene Deconfliction,
Kollisions-Placement aus); die Zeitachse war das letzte fehlende Drittel
dieses Opt-outs. Nebenwirkung: Auch Basiskarten-Beschriftung poppt beim
Zoomen hart ein statt weich zu blenden — scope-konsistent.

**Geprüft und verworfen:** differentielle Quell-Updates (beheben das Blinken
nicht — MapLibre matcht Symbole über den Inhalt; bleibt als reine
Effizienz-Option), DOM-Marker/Custom-WebGL-Layer (falsche Flughöhe).
Register: **FR-UI-038**. **Offen:** Sicht-Abnahme durch den Betreiber (die
Sandbox kann WebGL nicht rendern) — Scope beobachten: Labels müssen bei
jedem Update stehen bleiben, ohne Leer-Moment.

**Nächster Schritt:** Betreiber-Sicht-Abnahme Flacker-Fix + Smoke-Test
Sektor-Suche (#277, unten). Danach Betriebs-Härtung laut Roadmap.

---

## 🔎 Stand 2026-07-19 (Sektor-Suche über die Basiskarten-Daten — #277, ADR 0028, FR-UI-037)

**In normaler Sprache:** Der Lotse kann jetzt im Scope nach Straßen und Orten
in seinem Sektor **suchen** (Use Case des Betreibers: „Eine Drohne startet aus
der Friedrichstraße — wo ist die?"). Ein Suchfeld oben rechts liefert nach
2+ Zeichen Treffer aus dem eigenen Sektor; ein Klick auf einen Treffer setzt
einen magenta Marker mit Namen und fährt die Kamera dorthin. Beim allerersten
Suchen eines Sektors baut der Server sich einmalig ein Suchregister aus den
Kartendaten auf („Suchindex wird aufgebaut …", wenige Sekunden), danach ist
die Suche sofort. Kein externer Suchdienst: Es werden ausschließlich die
BKG-Kartendaten benutzt, die Wayfinder ohnehin lädt — funktioniert damit auch
im Air-Gap-Betrieb und ohne Lizenzfrage (BKG-Geokodierung wäre nur für
Behörden kostenfrei).

**Fachlich/technisch:** Neues `pkg/basemapsearch` — lazy je AOI gebauter Index
aus den z14-Vektor-Tiles des konfigurierten Styles (Worker-Pool, Single-Flight,
MVT-Dekodierung via `github.com/paulmach/orb`, schema-tolerante
Namens-Extraktion, Normalisierung ä→ae/ß→ss/„straße"→`str`, 3-km-Clustering,
Präfix-vor-Infix-Ranking, max. 20 Treffer). Limits fail-safe: 4096 Tiles
(übergroße AOI Zentrum-erhaltend geclampt), 8 Indexe (LRU), 250 k Einträge,
4 MiB/Tile, Build-Timeout 5 min, TTL 24 h mit Stale-Serve; ohne AOI 30-NM-Box
ums View-Zentrum. Endpoint `GET /api/basemap/search?q=…` (202 building → UI
pollt / 200 ready / 503 ohne Gebiet), **Feature-Gate `basemap` fail-closed
(403)** — der Index-Bau kostet reale Ressourcen. UI: `MapSearch.vue` im
Top-Cluster (Debounce 300 ms, Building-Poll, Esc/Clear), Marker + `easeTo` in
der Engine (`SEARCH_MARKER_*`, oberste Layer-Ebene). Metriken
`wayfinder_basemap_search_builds_total{result}` /
`wayfinder_basemap_searches_total`. Doku: **ADR 0028**, FR-UI-037, TECHNICAL
(Endpoint + § 5.4c). Betreiber-Weichen 1–3 wie freigegeben umgesetzt.
**Offen:** Betreiber-Smoke-Test gegen echte BKG-Tiles (die Sandbox erreicht
`sgx.geodatenzentrum.de` nicht — das reale Tile-Schema konnte nur
schema-tolerant, nicht live verifiziert werden): Suche nach einer bekannten
Straße im Sektor, Treffer-Klick, Marker prüfen.

**Nächster Schritt:** Betreiber-Smoke-Test #277; danach ggf. Feinschliff
(Kategorie-Labels ans reale BKG-Schema anpassen). Ansonsten Betriebs-Härtung
laut Roadmap.

---

## 🌍 Stand 2026-07-18 (BKG-Basiskarte: basemap.world als Default-Quelle — Umland-Kontext; ADR 0026 Nachtrag, FR-UI-032)

**In normaler Sprache:** Die amtliche Karte endete bisher hart an der
Staatsgrenze — hinter Lübeck Richtung Ostsee war einfach Leere. Jetzt ist die
Standard-Quelle der `bkg`/`bkg-dark`-Themes **basemap.world**: innerhalb
Deutschlands weiterhin exakt die amtlichen basemap.de-Daten, außerhalb ein vom
BKG kuratierter Weltkontext (aus OSM/NaturalEarth). Grenzüberschreitende
Sektoren sehen damit ihr Umland — die letzte fachliche Hürde vor dem Wechsel
des Standard-Themes auf den dunklen Amtsdaten-Scope ist gefallen.

**Fachlich/technisch:** Reiner Default-Tausch — `pkg/basemap.DefaultStyleURL`
zeigt auf `bm_web_wld_col.json` (zwei Kachel-Archive: amtliches DE monatlich,
Welt halbjährlich); die schema-agnostische H1/H2-Pipeline (Glyph-Weiche,
URL-Absolutisierung, Attribution, Dunkel-Transformation) verarbeitet den
world-Style ohne Code-Änderung. `GermanyOnlyStyleURL` bleibt als dokumentierte
Pin-Option für strikt-amtliche Deployments (`WAYFINDER_BKG_STYLE_URL`).
Register: **FR-UI-032**. Ehrliche Grenze: amtlich ist nur der DE-Anteil.

**Nachtrag (2026-07-18, Korrektur):** Ein zwischenzeitlich hier verbuchter
„Smoke-Test bestanden"-Vermerk war **falsch** und ist zurückgenommen. Der
Betreiber-Screenshot (Sektor Niederrhein/NL, `bkg-dark`) zeigte den **alten**
Nur-Deutschland-Stand: Links der Grenze liegt Leere; die sichtbaren Umrisse
über NL sind Wayfinders eigene Luftraum-Overlays (EHAA/CTA-EHAM/EHV), keine
Welt-Kacheln — der Test-Build stammte von `main` **vor** dem Merge dieses
Nachtrags und konnte die world-Default-URL gar nicht enthalten (vom Betreiber
erkannt, 2026-07-18).

**Nachtrag (2026-07-18, Smoke-Test jetzt wirklich ✅):** Nach Merge von
PR #270 + Rebuild bestätigt der Betreiber am laufenden System: links der
deutschen Grenze erscheint NL/BE-Kartografie statt Schwarz — der
world-Kontext greift, Dunkel-Transformation inklusive. (Zwischenzeitliche
Verwirrung war reiner **Browser-Cache** auf dem localhost-Origin des
`gh`-Tunnels; Hard-Reload löste es. Lehre aus der Fehl-Abnahme davor bleibt:
**vor jedem Abnahme-Vermerk Versions-Verifikation am laufenden System**.)
**basemap.world ist abgenommen.**

**Nächster Schritt:** Richtungs-Entscheid des Betreibers: **(a)**
Theme-Default-Wechsel `dark` → `bkg-dark` (S1, server-weit — schneller
Schlussstein, OSM/CARTO ab Werk abgelöst) und #274 später obendrauf, oder
**(b)** direkt das **#274-Entitlement-Modell** (BKG als mandanten-freigebbare
Layer-Option mit Nutzer-Toggle, S4) ohne die Zwischenstufe. Außerdem offen:
ASD-Bedienbarkeits-Trio #271–#273, H3 Selbst-Hosting, #267 DB-Volume.

---

## 🏁 Stand 2026-07-18 (Ausbau OSM/CARTO — `bkg-dark` ist der Default; ADR 0026 Nachtrag, FR-UI-033)

**In normaler Sprache:** Der Betreiber hat Richtung **(a) mit Verschärfung**
entschieden: nicht nur Default-Wechsel, sondern **sauberer Ausbau** der alten
Karten. Die OSM- und CARTO-Raster-Basiskarten sind komplett aus dem Code
entfernt — Wayfinder startet ab Werk mit dem **dunklen amtlichen Radar-Scope**
(`bkg-dark`, inkl. Umland via basemap.world) und kontaktiert **keine
OSM-/CARTO-Server mehr**. Wer die alten Theme-Namen (`dark`/`osm`) noch in
seiner Konfiguration hat, bekommt automatisch die passende BKG-Variante plus
eine Hinweis-Warnung im Log — nichts bricht.

**Fachlich/technisch:** `defaultMapStyle`/`darkMapStyle` (Inline-Raster-Styles)
ersatzlos entfernt; Theme-Vokabular `bkg`/`bkg-dark` (Default `bkg-dark`),
Legacy-Aliase `dark`→`bkg-dark` / `osm`→`bkg` mit
`MapThemeDeprecatedInput`-Startup-Warnung; map-config liefert ohne
Custom-Style-URL immer `/basemap/style.json`; basemap-Service läuft immer
(außer Custom-Style). Frontend-Paletten auf `bkg`/`bkg-dark` reduziert,
Compose-Defaults nachgezogen. Historische ADRs/Milestones (ASD-003a etc.)
bleiben als Audit-Spur; aktuelle Doku (README/INSTALLATION/TECHNICAL)
bereinigt. Register: **FR-UI-033**. Gates grün (go test/vet/gofmt,
golangci-lint, vitest 603, `npm run build`, dist neu).

**Nachtrag (2026-07-18, Ab-Werk-Test ✅):** Betreiber-Bestätigung nach Merge
+ Rebuild ohne gesetzte Theme-Env: der dunkle Amtsdaten-Scope (`bkg-dark`)
erscheint ab Werk. **Die BKG-Migration (ADR 0026, H1/H2/basemap.world/Ausbau
OSM-CARTO) ist damit vollständig abgeschlossen und abgenommen.**

**Nachtrag (2026-07-18, H3 Selbst-Hosting/Air-Gap — FR-CFG-004):** Auf
Betreiber-Wahl als Abschluss des BKG-Themas umgesetzt — als reiner
**Deployment-Baustein ohne Code-Umbau**: INSTALLATION §8.0a beschreibt den
Spiegel-Aufbau (monatliches BKG-Download-Paket `fonts/sprites/styles/tiles`,
statisch serviert, `WAYFINDER_BKG_STYLE_URL` auf den Spiegel — die
H1-Pipeline macht die von der BKG-Hosting-Anleitung verlangten
URL-Umschreibungen automatisch); Referenz-Spiegel
`docker-compose.basemap-mirror.yml` + `deploy/basemap-mirror/nginx.conf`
(CORS, gzip-Content-Encoding, Cache). Ehrliche Grenzen: world-Offline-Paket
beim BKG-DLZ zu klären; Paket-Verifikation betreiberseitig offen
(zweistelliger GB-Download). Nebenbei geklärt + als **#277** erfasst:
Orts-/Straßensuche wäre über den amtlichen BKG-Geokodierungsdienst machbar
(Lizenzfrage!), nicht über die Kacheln selbst.

**Nachtrag (2026-07-18, #267 DB-Volume ✅ — FR-CFG-005):** Der `db`-Dienst in
`docker-compose.orchestrated.yml` nutzt jetzt das benannte Volume
`wayfinder-db` — Mandanten/Nutzer/Feeds/Abos überleben Container-Neuanlegen;
Reset nur noch bewusst via `docker volume rm`. Befund-Korrektur: die
onboarding-Compose **hatte bereits** ein Volume (Issue-Annahme „beide Dateien"
war halb falsch); Volume-Name konsistent übernommen. Ehrlicher
Umstiegs-Preis (einmaliger Verlust des volume-losen Bestands) in INSTALLATION
dokumentiert. Nebenbei geklärt: #277 hat durch die Betreiber-Idee
„AOI-begrenzter Sektor-Suchindex aus den Kacheln" (Kandidat D) einen
lizenzfreien, Air-Gap-tauglichen Vorzugsweg (S4, Kommentar im Issue).

**Nachtrag (2026-07-19, ASD-Bedien-Trio #271–#273 ✅ — FR-UI-034):** Drei
Betreiber-gemeldete Bedien-Reibungen behoben: **(#271)** Klick auf den
Datenblock (Label) selektiert den Track wie ein Symbol-Klick — Label-Drags
bleiben unterscheidbar (MapLibre unterdrückt Clicks jenseits der
clickTolerance), Werkzeug-Guard unverändert. **(#272)** Das Detail-Panel
läuft **live** mit: jeder WS-Batch ersetzt den Selektions-Snapshot
(`refreshSelectedTrack`); verschwundener Track (TSE) behält den letzten
Stand (Panel bleibt offen). Subtile Stelle gelöst: der
Korrelations-Prefill-Watch keyt jetzt auf `track_num` — sonst hätte jede
Live-Meldung das getippte Callsign-Feld zurückgesetzt. **(#273)** Klick auf
freie Kartenfläche deselektiert (Panel zu, Halo weg); Mess-Werkzeuge und
Kamera-Pan sind ausgenommen. Rein Frontend. Register: **FR-UI-034**. Gates
grün (vitest 613, `npm run build`, dist neu; Go unberührt).

**Nachtrag (2026-07-19, „Track beendet"-Banner — FR-UI-035):** Auf
Betreiber-Rückfrage („Wird ein verschwundener Track im Panel angezeigt?" —
Antwort war Nein) als kleines Folge-Häppchen zu #272: Das Live-Panel zeigt
bei verschwundenem Track (TSE) jetzt ein Warn-Banner „Track beendet —
letzte bekannte Werte" (Liveness aus `liveTrackNums`; Coasting zählt als
lebend) und deaktiviert die drei Korrelations-Kommandos (Kommando auf einen
gelöschten Track liefe ins Leere). Rein Frontend. Gates grün (vitest 616,
build, dist neu).

**Nachtrag (2026-07-19, #274 Basiskarte als Entitlement-Layer ✅ — ADR 0027,
FR-UI-036):** Betreiber-Weichen **W1=b / W2=aus** ratifiziert: Die BKG-Karte
ist ein freigebbares Nice-to-have — der Scope startet **rein synthetisch**
(Near-Black + Overlays); erst Admin-Freigabe (`basemap`-Entitlement, Katalog
jetzt 14 Keys) plus bewusstes Zuschalten in der Sidebar (Default aus,
View-Profile persistieren) zeigt die Karte. Mechanik ohne Style-Wechsel
(Layer-Snapshot + Sichtbarkeit, immer sichtbarer Near-Black-Grund); dazu der
**synthetische Fallback-Style** bei nicht ladbarem Karten-Upstream (lokale
Glyphs — ein BKG-Ausfall kostet nie das Lagebild). Migrations-Wirkung von
W1=b bewusst: Bestandsmandanten sehen bis zur Freigabe den synthetischen
Scope (INSTALLATION-Hinweis). Gates grün (go test, vitest 623, build, dist).

**Nächster Schritt:** offen — Kandidat: #277 (Sektor-Suche, S4 —
Design-Ankündigung nötig). Wird wie üblich angekündigt (Freigabe abwarten).

---

## 🌒 Stand 2026-07-18 (BKG-Basiskarte H2: Radar-Scope-Dunkelvariante `bkg-dark`; ADR 0026 Nachtrag, FR-UI-031)

**In normaler Sprache:** Der dunkle Radar-Modus konnte bisher nur ein fremdes
Kartenbild (CARTO) auf 40 % dimmen. Jetzt gibt es mit
`WAYFINDER_MAP_THEME=bkg-dark` erstmals einen **echten dunklen Scope aus den
amtlichen BKG-Daten**: fast schwarzer Grund, zarte Küsten/Grenzen/Straßen,
gedämpft helle Ortsnamen, gedimmte Straßenschilder — dieselben
qualitätsgesicherten Vektordaten wie beim hellen `bkg`, nur dunkel gezeichnet.
Der bisherige `dark`-Default bleibt vorerst (basemap.de endet an der
Staatsgrenze — für grenzüberschreitende Sektoren wäre ein Umland-loser Default
ein Rückschritt; Wechsel kommt mit basemap.world).

**Fachlich/technisch:** Kein zweites hand-gepflegtes Style-JSON (BKG-Schema
driftet mit Updates), sondern eine **regelbasierte HSL-Transformation** in der
H1-Pipeline (`pkg/basemap/scope.go`, `Config.Dark`): Farben je Rolle in
Scope-Bänder gemappt — Flächen/Linien helligkeits-invertiert nach Near-Black
(Kontrast-Ordnung erhalten, Sättigung ×0,35), Kartentext gedämpft hell, Halos
backdrop-dunkel, `icon-opacity` gedimmt; rekursiv durch Expressions/Stops,
Alpha erhalten, Unparsebares unverändert. Theme-Vokabular
`dark|osm|bkg|bkg-dark`; `bkg-dark` teilt `/basemap/style.json` und die dunkle
Frontend-Palette. Register: **FR-UI-031**. Gates grün (go test/vet/gofmt/
golangci-lint, vitest, `npm run build`, dist neu).

**Nachtrag (2026-07-18, Sichttest ✅):** Betreiber-Sichttest am echten
BKG-Dienst bestanden (Screenshot Raum Hamburg, Zugang via `gh`-Port-Tunnel):
Near-Black-Grund mit zarter Geografie-Struktur, Ortsnamen gedämpft hell im
Zielband, Straßenschilder gedimmt (schwach sichtbar — bewusster
`icon-opacity`-Wert 0,35; „ganz aus" wäre ein S1-Nachschlag), ASD-Overlays
(Sektorringe/CTR/TMA/AoR) und Track-Datenblock klar dominant. Die
HSL-Bänder passen auf dem realen BKG-Farbspektrum ohne Feintuning —
**H2 ist abgenommen.**

**Nächster Schritt:** H3 (Selbst-Hosting), #267 (DB-Volume) oder
basemap.world (Auslandskontext; danach Default-Wechsel `dark`→`bkg-dark`) —
wie üblich mit Ankündigung + Freigabe.

---

## 🗺️ Stand 2026-07-18 (Amtliche Basiskarte BKG basemap.de — H1, Theme `bkg`; ADR 0026, FR-UI-030, ASD-016)

**In normaler Sprache:** Die Basiskarte unter dem Luftlagebild kann jetzt aus
**amtlichen, qualitätsgesicherten Daten** kommen: basemap.de Web Vektor, der
gemeinsame Kartendienst von Bund und Ländern (BKG) — statt der bisherigen
OSM-/CARTO-Bilder ohne QS-Zusage. Der Betreiber schaltet das mit
`WAYFINDER_MAP_THEME=bkg` ein (Style-Wahl des Betreibers: **Farbe**). Die
Track-Beschriftung bleibt dabei intakt: Wayfinder veredelt das BKG-Kartenrezept
server-seitig, sodass **alle** Schriften — Städtenamen wie Callsigns — weiter
aus Wayfinders eigener Schriftquelle kommen (ein MapLibre-Style kennt nur
**eine** Glyph-Quelle; unveredelt eingebunden wären die Track-Labels stumm
geblieben — genau deshalb ist die Migration mehr als ein URL-Tausch).

**Fachlich/technisch:** Neues Paket `pkg/basemap` (Muster `pkg/weathertiles`):
`/basemap/style.json` holt das Upstream-Style (`WAYFINDER_BKG_STYLE_URL`,
Default BKG-„Farbe"), schreibt `glyphs` auf `/glyphs` um, absolutisiert
relative Sprite-/Kachel-URLs (inkl. `{z}`-Template-Reparatur), injiziert die
Pflicht-Attribution © basemap.de / BKG falls fehlend; Cache 12 h,
stale-on-error, ohne Cache ehrliches 502 (`/ready` unberührt). `/glyphs` wird
mit aktivem `bkg`-Theme zur **Weiche**: eingebettete Fontstacks (Roboto Mono)
lokal, BKG-Kartenfonts via validiertem, größen-limitiertem Proxy (kein `..`,
Range-Regex, `PathEscape`, 2-MiB-Limit, Cache-Bound 512). Metrik-Trio
`wayfinder_basemap_fetch_*`/`_cache_age_seconds`. Frontend: `PALETTES.bkg` =
helle Palette. `dark` bleibt Default (CARTO, bis H2), `osm` deprecated.
Register: **FR-UI-030**. Gates grün (`go test ./...`, vet, gofmt,
golangci-lint; vitest 603, `npm run build`, `dist` neu eingebettet).

**Ehrliche Grenzen / offen:** (a) basemap.de endet an der Staatsgrenze —
Auslandskontext via basemap.world ist ein Folge-Häppchen; darum wechselt H1 den
Default nicht. (b) **Verifikation am echten BKG-Dienst steht aus** — die
Entwicklungs-Sandbox hatte keinen Netzzugriff auf `sgx.geodatenzentrum.de`
(Proxy-Policy); die Pipeline ist gegen einen realistisch geformten
httptest-Upstream verifiziert. **Betreiber-Smoke-Test (H0/H1):**
`WAYFINDER_MAP_THEME=bkg` setzen → Karte lädt, Track-Labels intakt,
Attribution sichtbar.

**Nachtrag (2026-07-18, Smoke-Test ✅):** Betreiber-Smoke-Test am **echten
BKG-Dienst** erfolgreich (Codespace, `WAYFINDER_MAP_THEME=bkg`, Screenshot
Raum Hamburg): amtliche „Farbe"-Karte lädt vollständig (Kacheln + Sprite),
die **Basemap-Ortsnamen rendern** — d. h. die BKG-Kartenfonts kommen
nachweislich durch die `/glyphs`-Proxy-Weiche —, und die ASD-Overlays
(Sektorringe, TMA/CTR, AoR, Sektor-Labels) sitzen lesbar auf der hellen
Basis (bkg-Palette greift). **Vollständig bestätigt** (Betreiber-Rückmeldung):
Track-Labels rendern in **Roboto Mono**, die ⓘ-Attribution zeigt
„© 2026 basemap.de / BKG | Datenquellen: © GeoBasis-DE" (das Upstream-Style
bringt seinen eigenen Quellenvermerk mit — unsere Injektion bleibt reines
Sicherheitsnetz für den Fall eines attributionslosen Styles). Damit ist die in
„Ehrliche Grenzen (b)" offene End-zu-End-Verifikation erbracht; **H1 ist
komplett abgenommen**.

**Nachtrag (2026-07-18, H1-Lücke):** Die Compose-Dateien
(`docker-compose.orchestrated.yml`/`.onboarding.yml`) reichten
`WAYFINDER_MAP_THEME`/`WAYFINDER_BKG_STYLE_URL` nicht in den
Wayfinder-Container durch — der Betreiber konnte das `bkg`-Theme im
Compose-Betrieb gar nicht aktivieren (beim Smoke-Test aufgefallen). Beide
Dateien tragen jetzt die Passthrough-Zeilen (`${WAYFINDER_MAP_THEME:-dark}`,
`${WAYFINDER_BKG_STYLE_URL:-}`); Aktivierung damit z. B.
`WAYFINDER_MAP_THEME=bkg docker compose -f docker-compose.orchestrated.yml up -d wayfinder`.

**Nächster Schritt:** Betreiber-Smoke-Test am echten Netz; danach Ankündigung
**H2** (eigener dunkler Radar-Style aus den BKG-Vektorkacheln, ersetzt den
CARTO-Dimm-Trick als `dark`-Default) bzw. **H3** (Selbst-Hosting/Air-Gap via
BKG-Download-Paket). Wird wie üblich angekündigt (Freigabe abwarten).

---

## 🧩 Stand 2026-07-16 (Verbund-Rolle dokumentiert: Serving-Hälfte der SDPS-Server-Funktion — #257, ADR 0025)

**In normaler Sprache:** Rein dokumentarisches Häppchen — kein Code. Ein
vollständiges Luftlage-System nach ARTAS-Vorbild hat zwei Hälften: das *Rechenwerk*
(macht aus Radarmeldungen Tracks) und den *Server* (liefert jedem Nutzer genau
seinen Ausschnitt über eine gesicherte Leitung). Firefly ist das Rechenwerk und hat
bewusst keinen Nutzer-Server; diese zweite Hälfte macht **Wayfinder** (Mandanten/
Abos verwalten, serverseitig aufs erlaubte Gebiet filtern, pro Nutzer über einen
angemeldeten WebSocket ausliefern). Das war immer so gebaut — jetzt ist es als
**Entscheidung mit Begründung und Grenzen** festgehalten, damit die Verbund-Rolle
auch im Wayfinder-ADR-Verzeichnis auffindbar ist (Spiegel zu Fireflys ADR 0042).

**Fachlich/technisch:** Neuer **ADR 0025** („Wayfinder erbringt die Serving-Hälfte
der SDPS-Server-Funktion") mit der Leistungstabelle (welche ARTAS-Server-Leistung
durch welchen Wayfinder-Baustein erbracht wird: ADR 0005/0007/0012, WF2-21.2,
ADR 0003/0021) und der Konsumenten-Matrix **K1–K5** inkl. der bewussten Absage an
einen CAT252-Server. Verweis-Absatz in `CLAUDE.md` §1. Cross-Project-Todo
aktualisiert (#245 **und** #257 als erledigt). Kein Wire-/ICD-Bezug, keine
Env-Variablen, keine Code-Änderung — Go-/Frontend-Gates unberührt (nur Doku).

**Nächster Schritt:** Die `from-firefly`-Kette dieser Sitzung ist damit vollständig
abgearbeitet (#239/#240, #241, #242, #245 Teil A + Teil B H1–H4, #257). Offene
`from-firefly`-Issues: **keine** mehr. Nächster Punkt wäre die **Betriebs-Härtung**
(Observability/Last/Deployment) oder ein neuer Cross-Project-Impuls — wird wie
üblich angekündigt (Freigabe abwarten).

---

## 🔑 Stand 2026-07-16 (Manuelle Korrelation Häppchen 4: Token-Injektion — #245 Teil B **abgeschlossen**, FR-ORCH-013)

**In normaler Sprache:** Häppchen 4 schließt die letzte Lücke, damit die manuelle
Korrelation im **echten Mehr-Feed-Betrieb** funktioniert. Fireflys Kommando-API
ist tokengeschützt: Ohne das richtige Passwort (Bearer-Token) lehnt Firefly jeden
Korrelations-Befehl ab. Bisher **sendete** Wayfinder zwar das Token (seit H1/H2),
aber die je Feed automatisch gestarteten Firefly-Instanzen kannten es gar nicht —
im Docker-orchestrierten Betrieb wären die Befehle also an `401` gescheitert.
Häppchen 4 sorgt dafür, dass der Orchestrator dasselbe Deployment-Token beim
Starten **in jede Firefly-Instanz hineinreicht** (`FIREFLY_WS_TOKEN`). Damit passt
das Passwort auf beiden Seiten, und **#245 Teil B ist komplett**.

**Neu nutzbar:** Im vollen orchestrierten Aufbau (Postgres + Server + Orchestrator,
der pro Feed eine Firefly-Instanz spawnt) greift die manuelle Korrelation jetzt
Ende-zu-Ende: Setzt der Betreiber `WAYFINDER_FIREFLY_COMMAND_TOKEN` auf **beiden**
Prozessen (Server **und** Orchestrator), verlangen die Firefly-Instanzen genau das
Token, das der Server sendet — die Korrelations-Knöpfe aus H3 wirken real bis in
den Tracker durch.

**Fachlich/technisch:** `pkg/dockerbackend` bekommt ein `commandToken`-Feld
(`Backend`) + `New`-Parameter; `fireflyEnv` hängt `FIREFLY_WS_TOKEN=<token>` an die
Container-Env, sobald das Token gesetzt ist (leer ⇒ keine Injektion, Feature aus).
`cmd/wayfinder-orchestrator` liest `WAYFINDER_FIREFLY_COMMAND_TOKEN` (dasselbe
deployment-weite Token wie der Server) in seine Config und reicht es an
`dockerbackend.New` durch. **Kontrakt verifiziert** gegen Fireflys Quelle
(`crates/firefly-server/src/main.rs`: `FIREFLY_WS_TOKEN` gated `authorize_command`
und `/ws`; Server-zu-Server passiert die Origin-Prüfung, es zählt nur das Bearer).
Das Hinzufügen der Env ändert den Spec-Hash → laufende Instanz wird beim nächsten
Reconcile ersetzt (übernimmt das Token). Token wird **nie geloggt** (Config nie
als Ganzes ausgegeben). Rein Backend/Orchestrator, keine CAT062-Wirkung, kein
Frontend. Register: **FR-ORCH-013** (Stand H4 ✅, Teil B vollständig). Gates grün
(`go test ./...`, vet, gofmt, golangci-lint).

**Test-Kern:** `backend_test.go::TestFireflyEnvInjectsCommandToken` (Token gesetzt →
`FIREFLY_WS_TOKEN` in der Env, leer → nicht injiziert),
`main_test.go::TestLoadConfigCommandToken` (Orchestrator parst die Env, leer wenn
unset). Doku: INSTALLATION (Token nun auch am Orchestrator nötig), TECHNICAL
(H4-Absatz), requirements/README (FR-ORCH-013 H4 ✅).

**Nächster Schritt:** #245 Teil B ist damit erledigt — Issue **#245** kann
geschlossen werden (der PR trägt das Closing-Keyword). Danach den Cross-Project-
Nachzug (`from-firefly`) fortsetzen bzw. den nächsten Punkt aus der Roadmap
abstimmen. Wird wie üblich angekündigt (Freigabe abwarten).

---

## 🧭 Stand 2026-07-16 (Manuelle Korrelation Häppchen 3: Frontend-Bedienung im Detail-Panel — #245 Teil B, FR-ORCH-013)

**In normaler Sprache:** Ab jetzt **sieht und bedient** der Lotse die manuelle
Korrelation. Im Track-Detail-Panel gibt es einen neuen Abschnitt **„Korrelation"**
mit einem Callsign-Feld (vorbelegt mit der besten bekannten Kennung des Tracks)
und drei Knöpfen: **Korrelieren** (Track an den gefileten Plan binden),
**Unkorreliert** (die Automatik-Zuordnung unterdrücken) und **Zurücksetzen** (den
manuellen Eingriff lösen, Automatik übernimmt wieder). Jeder Klick zeigt sofort
eine **ehrliche Rückmeldung** direkt darunter — grün bei Erfolg, gelb mit
Klartext-Grund bei Ablehnung („Kein Flugplan mit dieser Kennung gefunden",
„Für diesen Feed nicht berechtigt" …). Das ist die **erste Bedienhandlung im
ASD, die etwas bei Firefly verändert** — bisher konnte der Lotse nur zuschauen.

**Neu nutzbar:** Der Korrelations-Abschnitt erscheint **nur**, wenn (a) der
Betrieb die Funktion aktiviert hat (Command-Token gesetzt, neues
`map-config.correlation_available`) **und** (b) der Track über einen echten
Katalog-Feed kam (`feed_id` vorhanden — der ENV-Fallback-Feed hat keinen
Command-Kanal). So sieht der Lotse nie Knöpfe, die ohnehin nur 503 liefern würden.

**Fachlich/technisch:** (1) `feed_id` wird jetzt auf jedes Track-Feature gebacken
(`frontend/src/map/tracks.js`) — der Endpoint adressiert per `(feed_id,
track_num)`. (2) Store-Aktionen in `stores/asd.js` (`correlate` /
`setUncorrelated` / `clearOverride`) posten über `apiFetch` an
`POST/DELETE /api/correlation` und übersetzen die HTTP-Statuslage in eine
einheitliche `{ ok, message }`-Form (deutsche Controller-Meldungen je Status,
Fallback auf den rohen Fehler). (3) `TrackDetailCard.vue` bekommt den
Korrelations-Abschnitt (Callsign-Feld + drei Knöpfe + synchrone `v-alert`-Zeile;
`correlationBusy` sperrt während des Kommandos). (4) Neuer map-config-Schalter
`correlation_available` (= Token gesetzt), vom Engine in
`store.correlationAvailable` gespiegelt. **Reine UI-/Frontend-Arbeit plus ein
Read-only-Backend-Flag** — kein neuer Env-Eintrag (Token seit H2 dokumentiert),
keine CAT062-Wirkung, das Sicherheits-Gating bleibt komplett serverseitig (H2).
Register: **FR-ORCH-013** (Stand H3 ✅). Gates grün (`go test ./...`, vet, gofmt,
golangci-lint; `vitest` 600 grün, `npm run build`, `dist` neu eingebettet).

**Test-Kern:** `asd.test.js` — Verfügbarkeits-Gate (Default aus, Boolean-Coercion)
+ die drei Kommandos gegen ein gestubbtes `fetch` (korrekte URL/Methode/Body:
`correlate` POSTet `{feed_id, track_number, callsign}`, `setUncorrelated` einen
`null`-Callsign, `clearOverride` DELETEt den Pfad; Status→Meldung-Mapping 204/422/
409/403 + Fallback). `tracks.test.js` — `feed_id`-Bake (Wert bzw. `null` beim
ENV-Feed). `main_test.go::TestMapConfigHandlerCorrelationAvailable` — Flag spiegelt
Token gesetzt/leer.

**Nächster Schritt:** **Häppchen 4** — `fireflyEnv`-Injektion des
`FIREFLY_WS_TOKEN` in die je-Feed gespawnten Firefly-Container
(`pkg/dockerbackend`), damit der Command-Rückkanal im echten Multi-Feed-Betrieb
authentifiziert durchkommt; danach ist **#245 Teil B** komplett und das Issue
kann geschlossen werden. Wird wie üblich angekündigt (Freigabe abwarten).

---

## 🛂 Stand 2026-07-16 (Manuelle Korrelation Häppchen 2: Server-Endpoint + Gating — #245 Teil B, FR-ORCH-013)

**In normaler Sprache:** Häppchen 2 baut die **Bedien-Schnittstelle** für die
manuelle Flugplan-Korrelation: den Server-Endpoint `POST/DELETE /api/correlation`.
Wenn der Lotse (ab Häppchen 3) im Kontextmenü „mit DLH123 korrelieren" klickt, geht
die Anfrage hierher — der Server **prüft streng, ob der Nutzer das darf**, ruft dann
über den H1-Client das richtige Firefly und gibt sofort eine ehrliche Antwort
zurück. **Noch kein sichtbares UI** (das ist Häppchen 3), aber der Endpoint ist
funktionsfähig und per `curl` testbar.

**Das Sicherheits-Herzstück:** Dies ist Wayfinders **erste Schreib-Aktion eines
Mandanten-Nutzers auf einen Feed**. Deshalb steht das Gating im Zentrum — drei
Schleusen in `pkg/correlationapi.authorize`: (1) nicht eingeloggt → **401**; (2)
**unter „Als Mandant X ansehen"** (Read-only-Impersonation, ADR 0008) → **403**
(eine lesende Sitzung darf nichts schreiben); (3) **nicht auf den Feed abonniert**
→ **403** (das fängt auch den scope-losen Admin, ADR 0022, ohne Sonderfall). Der
Firefly-Fehler wird ehrlich durchgereicht: unbekannter Callsign → 422, keine Pläne
→ 409, Instanz unerreichbar/Token-Fehlkonfig → 502. Jedes Kommando wird auditiert.

**Fachlich/technisch:** Neues Paket `pkg/correlationapi` (`Service` mit
`SetHandler`/`ClearHandler`, Interfaces `Commander`/`SubscriptionChecker` für volle
Unit-Testbarkeit ohne Netz). Verdrahtet in `cmd/wayfinder` hinter `tenantMW`+`pwGate`
(**nicht** admin-gegated), Config `WAYFINDER_FIREFLY_COMMAND_TOKEN` (leer ⇒ 503).
Body-Limit + `DisallowUnknownFields`. **Neue Env-Variable** in `INSTALLATION.md`/
`TECHNICAL.md` eingetragen. Rein backend, kein Frontend, keine CAT062-Wirkung.
Register: **FR-ORCH-013** (Stand H2 ✅). Gates grün (`go test ./...`, vet, gofmt,
golangci-lint).

**Test-Kern (AuthZ-Tabelle):** unauth→401, Nicht-Abonnent→403, Impersonation-
trotz-Abo→403, scope-loser-Admin→403, Subs-Fehler→500, Feature-aus→503, Body-
Validierung→400, Firefly-Fehler→422/409/502 — und in **jedem** Ablehnungsfall wird
der Commander **nie** aufgerufen (kein Kommando ohne bestandenes Gating).

**Nächster Schritt:** **Häppchen 3** — Frontend-Kontextmenü am Track (korrelieren /
entkorrelieren / Pin lösen) gegen `/api/correlation`, mit synchroner Fehleranzeige.
Danach **Häppchen 4** — `fireflyEnv`-Injektion des `FIREFLY_WS_TOKEN` in die
Firefly-Container. Wird wie üblich angekündigt (Freigabe abwarten).

## 🔌 Stand 2026-07-16 (Manuelle Korrelation Häppchen 1: Firefly-Command-Client — #245 Teil B, FR-ORCH-013)

**In normaler Sprache:** Der Lotse soll Fireflys automatische Flugplan-Zuordnung
per Hand korrigieren können (ADR 0024). Das braucht erstmals einen **Rückkanal**
von Wayfinder zu Firefly — bisher hört Wayfinder nur zu. Häppchen 1 baut die
**Rohrleitung** dafür: einen kleinen, voll getesteten Baustein, der ein
Korrelations-Kommando an das richtige Firefly schicken *könnte*. **Noch nichts
sichtbar** für den Lotsen und **noch nicht im Betrieb aktiv** — der Baustein wird
erst in Häppchen 2 an einen Server-Endpoint angeschlossen.

**Fachlich/technisch:** Neues Paket `pkg/fireflycmd` — `Client` mit `Correlate` /
`SetUncorrelated` / `ClearOverride` / `ListOverrides` gegen Fireflys echte
Kommando-API (`POST/DELETE/GET /correlation`, verifiziert gegen
`firefly-server/src/app.rs`). Best-effort nach `pkg/weather`-Muster (getakteter
`*http.Client`, `context`, `io.LimitReader`, `Authorization: Bearer`); typisierte
Fehler `ErrUnknownCallsign` (422) / `ErrNoFlightPlans` (409) / `ErrUnauthorized`
(401) / `ErrUnreachable` (Netz) fürs spätere synchrone Menü-Feedback. Adressierung
über `HostLoopbackAddresser` → `instance.FireflyHTTPPort` — die Port-Formel ist
dabei aus `pkg/dockerbackend` (orchestrator-privat) in `pkg/instance` als
**geteilte Single-Source-of-Truth** verschoben, damit der Server sie ohne den
schweren Docker-Import nutzen kann; `dockerbackend` delegiert nun dorthin. Token-
Konstante `WAYFINDER_FIREFLY_COMMAND_TOKEN` definiert; **Server-Verdrahtung +
Endpoint + Gating = Häppchen 2** (daher noch kein `INSTALLATION.md`-Env-Eintrag —
die Variable wird erst dort wirksam). Rein backend-intern, keine CAT062-Wirkung,
kein Frontend. Register: **FR-ORCH-013** (Stand H1 ✅). Gates grün (`go test
./...`, vet, gofmt, golangci-lint).

**Nächster Schritt:** **Häppchen 2** — Server-Endpoint (`POST/DELETE /api/correlation`)
+ Gating (`IsSubscribed`, kein scope-loser Admin, nicht unter Impersonation) +
422/409-Mapping, inkl. Config-Verdrahtung von `WAYFINDER_FIREFLY_COMMAND_TOKEN`.
Wird wie üblich angekündigt (Freigabe abwarten).

## 🗺️ Stand 2026-07-15 (CAT062-Flugplan-Korrelation I062/390 — #245 Teil A, FR-DATA-013)

**In normaler Sprache:** Firefly weiß jetzt zentral, **welcher gefilte Flugplan**
zu einem Track gehört, und schreibt das auf den Draht (ICD 3.7.0). **Neu sichtbar
für den Lotsen:** Im Detail-Fenster stehen jetzt der **Plan-Callsign** und die
**Route** (z. B. „EDDF → EDDM"). Und ein wichtiges Betriebssignal: Weicht die vom
Flugzeug **gesendete** Kennung (I062/245) vom **gefileten** Plan-Callsign ab, wird
das hervorgehoben — am Label mit einem dezenten „≠" und im Panel farblich. Das
deutet auf einen falschen Squawk oder eine falsche Plan-Zuordnung hin.

**Fachlich/technisch:** Decoder liest FRN 21 (I062/390) **subfeld-getrieben** (wie
schon I062/380, #238): CSN (#2, 7 Okt. ASCII → Plan-Callsign), DEP/DST (#7/#8, je
4 Okt. ICAO). Bekannte fixe Subfelder werden längen-übersprungen (Vorwärts-
Kompatibilität für Fireflys additives Wachstum), das variable #12 (TOD) wird
abgelehnt. → `DecodedTrack`-Felder → WS-JSON (`plan_callsign`/`plan_departure`/
`plan_destination`) → Label-Mismatch-Marker + Detail-Panel (Plan-Callsign, Route,
Mismatch-Highlight). Additiv, kein Wire-/ICD-Bruch (unkorrelierter Track byte-
identisch). Grundwahrheit: Fireflys ICD §4.10-Referenz-Vektoren (`43 80 …`,
`40 …`). Register: **FR-DATA-013**. Gates grün (`go test ./...`, vitest,
`npm run build`, gofmt/vet/golangci-lint). dist neu.

**Scope-Abgrenzung (wichtig):** #245 ist damit **Teil A** (Anzeige) erledigt.
**Teil B — manuelle Korrelation** (`POST/DELETE /correlation`, ein Rückkanal
Wayfinder→Firefly) ist ein **architektonischer Neubau** (Wayfinder ist bisher
reiner Multicast-Konsument ohne Steuerverbindung) und bekommt einen **eigenen ADR
+ eigene Freigabe** — bewusst nicht in diesem PR. Auch `identity_conflict` (nur in
Fireflys WS-Pfad) ist über CAT062 nicht verfügbar.

**Stand Cross-Project-Nachzug:** Die decoder-/anzeige-seitige `from-firefly`-Reihe
(#235–#242, #245 Teil A) ist damit **abgeschlossen**. Offen bleibt nur der
Bedien-Rückkanal (#245 Teil B) als eigenes Vorhaben.

## 🧭 Stand 2026-07-15 (CAT062-Kinematik-Kette I062/200/210 — #242, FR-DATA-012)

**In normaler Sprache:** Firefly rechnet nicht nur *wo* ein Flugzeug ist, sondern
auch *wie es sich bewegt* — dreht es gerade nach links/rechts, wird es schneller
oder langsamer, steigt oder sinkt es, und wie stark beschleunigt es. Diese
Bewegungs-Info schickt Firefly jetzt mit (ICD 3.6.0). **Neu sichtbar für den
Lotsen:** (1) ein **Kurven-Indikator** (→ Rechtskurve / ← Linkskurve) direkt am
Track-Label — ein manövrierendes Flugzeug fällt sofort auf. (2) Im Detail-Fenster:
**Kurventrend**, **Geschwindigkeitstrend** (zunehmend/abnehmend) und die
**Beschleunigung**. Der Steig-/Sinkpfeil bleibt wie bisher aus der quantitativen
Rate (#241) — die neue qualitative Vertikal-Achse wird nicht doppelt gezeigt.

**Wichtige Klärung zur Reihenfolge:** Der ursprüngliche Eindruck „Firefly hat 3.6.0
noch nicht geliefert" war ein **veralteter lokaler Firefly-Checkout** — auf Firefly
`main` liegen 3.6.0 (I062/200/210) **und** 3.7.0 (I062/390) bereits. Nach `git pull`
lagen die byte-genauen §4.9-Referenz-Vektoren vor, gegen die getestet wurde.

**Fachlich/technisch:** Decoder liest FRN 8 (I062/210: Ax/Ay je i8 × 0,25 m/s²,
Ost/Nord) und FRN 15 (I062/200: TRANS/LONG/VERT je 2 Bit; Wert 3 = unbestimmt →
nil; Item entfällt bei komplett unbestimmter Lage) → getypte `DecodedTrack`-Enums
+ Beschleunigungs-Felder → WS-JSON (`course_trend`/`speed_trend`/`vertical_motion`/
`accel_ax_ms2`/`accel_ay_ms2`, nur bestimmte Achsen). Frontend: `label.js`
(Kurven-Indikator →/←), `trackDetail.js` + `TrackDetailCard` (Kurven-/
Geschwindigkeitstrend + Beschleunigungs-Betrag). Der WS-Feldname `vertical_motion`
ist bewusst vom rate-getriebenen ▲/▼-Glyph (`vertical_trend`) getrennt. Additiv,
kein Wire-/ICD-Bruch (Track ohne Kinematik byte-identisch). Grundwahrheit: Fireflys
ICD §4.9-Referenz-Vektoren (`04 FE`/`7F 80`/`54`/`B0`). Register: **FR-DATA-012**.
Gates grün (`go test ./...`, vitest, `npm run build`, gofmt/vet/golangci-lint).
dist neu.

**Nächster Schritt:** Cross-Project-Nachzug weiter — **#245** (Flugplan I062/390,
ICD 3.7.0, liegt ebenfalls auf Firefly `main`; durch #244 entsperrt).

## 🛗 Stand 2026-07-15 (CAT062-Vertikal-Kette I062/130/135/220 — #241, FR-DATA-011)

**In normaler Sprache:** Das Label zeigte die Höhe bisher als **gemessene**
Flugfläche — ein Rohwert, der von Meldung zu Meldung springen kann. Firefly
rechnet jetzt eine **saubere Vertikal-Lösung** und schickt sie mit: eine
geglättete Höhe, eine echte Steig-/Sinkrate und die geometrische (WGS-84-)Höhe.
**Neu sichtbar für den Lotsen:** (1) eine **ruhigere Anzeige-Höhe** im Label
(bevorzugt der geglättete Wert). (2) Eine ehrliche **„A" vs. „FL"-Kennzeichnung**:
`A030` = 3000 ft auf das echte regionale QNH bezogene Altitude, `FL350` =
Druckhöhe/Flugfläche — der Lotse sieht die Bezugsgröße direkt. (3) Ein **echter
Steig-/Sinkpfeil** (▲/▼) aus der Rate des Trackers statt aus dem bisherigen,
rausch-anfälligen Höhen-Differenz-Schätzer. (4) Geometrische Höhe + Steigrate im
Detail-Fenster.

**Korrektheits-Teil:** Die eine subtile Stelle ist I062/135 — Bit 16 ist ein
**QNH-Bit**, die restlichen 15 Bits sind ein **15-Bit-Zweierkomplement** (nicht
i16). Diese Vorzeichen-Erweiterung ist exakt gegen Fireflys byte-genaue
Referenz-Vektoren getestet.

**Fachlich/technisch:** Decoder liest FRN 18/19/20 (drittes FSPEC-Oktett):
I062/130 (i16 × 6,25 ft), I062/135 (QNH-Bit + 15-Bit-ZK × 25 ft), I062/220
(i16 × 6,25 ft/min) → `DecodedTrack`-Felder → WS-JSON (`geometric_altitude_ft`/
`barometric_altitude_ft`/`qnh_corrected`/`rocd_ft_min`; QNH-Flag nur mit baro.
Höhe). Frontend: `tracks.js` (Pfeil primär aus `rocd_ft_min`, ±300 ft/min-Totband,
Fallback FL-Differenz), `label.js` (A/FL-Anzeige-Höhe), `trackDetail.js` +
`TrackDetailCard` (baro/geo/ROCD-Zeilen). Additiv, kein Wire-/ICD-Bruch (Track
ohne Vertikal-Daten byte-identisch; I062/136 bleibt **gemessen** daneben).
Grundwahrheit: Fireflys ICD §4.8-Referenz-Vektoren. Register: **FR-DATA-011**.
Gates grün (`go test ./...`, vitest, `npm run build`, gofmt/vet/golangci-lint).
dist neu.

**Nächster Schritt:** Cross-Project-Nachzug weiter — **#242** (I062/200 Mode of
Movement + I062/210 Beschleunigung), danach **#245** (Flugplan I062/390, durch
#244 entsperrt).

## 📥 Stand 2026-07-15 (Lokale ASTERIX-über-UDP-Quelltypen `adsb_asterix` + `mlat_asterix` — #239/#240, FR-ORCH-012)

**In normaler Sprache:** Bisher konnte ein Feed seine Flugdaten aus dem Internet
(OpenSky, Community-Aggregatoren) oder von einem klassischen Radar beziehen. Jetzt
kommen die **beiden Produktions-Bezugswege** dazu, die man im echten Betrieb nutzt:
eine **eigene ADS-B-Bodenstation** (die Antenne, die die Flugzeug-Selbstmeldungen
direkt empfängt) und ein **WAM/MLAT-System** (das die Position aus Laufzeit-
differenzen mehrerer Bodenstationen rechnet). Beide liefern ihre Daten lokal per
Netzwerk-Push (ASTERIX über UDP), nicht per Internet-Abfrage. **Neu nutzbar:** Der
Betreiber wählt im Admin-„Quellen"-Dialog jetzt diese zwei Typen und trägt nur den
**Netzwerk-Endpoint** (`group:port`), optional die Stations-Kennung (SAC/SIC) und
eine Sensor-ID ein — **kein** Kartenausschnitt, **keine** Zugangsdaten (der rohe
UDP-Strom ist durch die Netz-Isolation geschützt, nicht durch ein Passwort).

**Fachlich/technisch:** Zwei neue Werte im geschlossenen Quell-Vokabular —
`adsb_asterix` (ADS-B-Bodenstation, **CAT021/UDP**, Firefly FEP.3, Kontrakt v1.6.0)
und `mlat_asterix` (WAM/MLAT, **CAT020 + CAT019 über UDP**, FEP.5, v1.7.0). Sie
bilden eine **dritte Formkategorie** neben flächen-begrenzt und Radar: das
Bodensystem rechnet die Position selbst, daher **kein `bbox`, kein Standort, kein
`cred_ref`** — nur optional `listen`/`sac`/`sic`/`sensor_id`. `Source.validate`
lehnt `sensor_id` auf Fremdtypen ab und verbietet für die UDP-Typen
BBox/Standort/Credential (Bereichs-Check SAC/SIC 0..255, non-negative `sensor_id`).
`dockerbackend.fireflySource` reicht `sensor_id` additiv nach `FIREFLY_SOURCES`
durch. Sensor-Mix-Ableitung: `adsb_asterix→ADS-B`, `mlat_asterix→MLAT`. Admin-UI:
zwei Typ-Einträge mit eigener Formular-/Payload-Kategorie (`ASTERIX_UDP_TYPES`).
**Betriebshinweis:** ist eine solche UDP-Quelle die **einzige** eines Feeds, fehlt
die Union-BBox für Fireflys System-Referenzpunkt (nur I062/100, das Wayfinder nicht
rendert) — Betreiber setzt dann `FIREFLY_SYSTEM_REF_*` an der Firefly-Instanz; kein
Auto-Wert ableitbar. **Rein orchestrierungs-seitig** — kein Decoder-Eingriff, keine
CAT062-Ausgabe-Wirkung, Wire-Vertrag additiv. Register: **FR-ORCH-012**. Gates grün
(`go test ./...`, vitest 554, `npm run build`, gofmt/vet/golangci-lint). dist neu.

**Nächster Schritt:** Cross-Project-Nachzug weiter — **#241** (Vertikal-Kette
I062/130/135/220) bzw. **#242** (I062/200 Mode of Movement + I062/210
Beschleunigung), danach **#245** (Flugplan I062/390, durch #244 entsperrt).

## 🛬 Stand 2026-07-15 (I062/380 Mode-S-DAPs + Selected Altitude / Level-Bust — #238, FR-DATA-010)

**In normaler Sprache:** Moderne Flugzeuge senden über Mode-S die Höhe, die der
Pilot im **Autopiloten eingedreht** hat (Selected Altitude). Wayfinder zeigt sie
jetzt — im Label als „S350" neben der Ist-Flugfläche und im Detail-Panel. Weicht
die eingedrehte Höhe deutlich von der Ist-Höhe ab, wird das **hervorgehoben** —
das **Level-Bust-Signal**: der Lotse sieht auf einen Blick, dass ein Flugzeug eine
andere Höhe ansteuert. Dazu Steuerkurs, IAS und Mach im Detail-Panel.

**Korrektheits-Teil (wichtig):** Diese Daten stecken **in I062/380** — dem Feld,
das bisher nur die ICAO-Adresse trug. Der alte Decoder ignorierte die FX-Kette und
hätte einen DAP-tragenden Track **fehl-geparst** (Desync im restlichen Datagramm).
Der Nachzug ist damit **korrektheitskritisch**, nicht nur ein Feature.

**Fachlich/technisch:** I062/380 auf **subfeld-getrieben** umgestellt (FX-Spec +
Subfelder aufsteigend): ADR (#1), MHG (#3), SAL (#6, 13-Bit-Zweierkomplement × 25 ft),
IAR (#26), MAC (#27) → `DecodedTrack`-Felder → WS-JSON → Label (`S<FL>`) +
`TrackDetailCard` (Level-Bust-Hinweis ab 300 ft Abweichung, `isLevelBust`).
Bekannte fixe Subfelder werden längen-übersprungen, variable/unbekannte (#8/#9/#25)
**abgelehnt** (robuster Decoder). DAP-loser Track byte-identisch, kein Wire-/ICD-Bruch.
Ehrliche Grenze: Wayfinder vergleicht SAL vs. Ist-FL — die *Freigabe* kennt es
nicht, die Bust-Bewertung bleibt beim Lotsen. Grundwahrheit: Fireflys ICD §4.7
(ADR 0x3C65AC, MHG 270°, SAL 35 000 ft, IAS 250 kt, Mach 0,784). Register:
**FR-DATA-010**. Gates grün (`go test ./...`, vitest 548, `npm run build`,
gofmt/vet/golangci-lint). dist neu.

**Nächster Schritt:** Cross-Project-Nachzug weiter — **#239/#240** (Quell-Typen
`adsb_asterix`/`mlat_asterix`, Orchestrator/Admin-UI) bzw. **#241/#242** (Vertikal,
Mode of Movement) nach abgestimmter Reihenfolge.

## 📡 Stand 2026-07-15 (CAT063 Registrierungs-Bias je Sensor — #237, FR-DATA-009)

**In normaler Sprache:** Jedes Radar misst systematisch ein bisschen falsch (z. B.
„immer 150 m zu weit"). Firefly schätzt diesen Fehler laufend und rechnet ihn vor
der Fusion heraus. Ab jetzt **zeigt Wayfinder den angewandten Wert je Sensor** —
im Feed-Chip (aufklappbar: „SIC 2 · Δr +145 m · Δθ +0,30°") und im Admin-Feed-
Panel. Nutzen: Ein Bias, der plötzlich **wächst**, ist ein **Frühwarnsignal** —
das Radar läuft aus der Kalibrierung oder hat ein Hardware-Problem, bevor das
Lagebild sichtbar leidet.

**Umfang-Entscheidung:** Wayfinders Feed-Health war bisher **rein aggregiert**
(grün/gelb/rot + „2/3 Radare") — es gab **keinen Per-Sensor-Eintrag**. Auf
Betreiber-Wahl (voller Chip-Ausbau) wurde ein **Per-Sensor-Detailpfad neu gebaut**.

**Fachlich/technisch:** Decoder liest I063/080 **SRB** (i16 × 1/128 NM → m) +
I063/081 **SAB** (i16 × 360/2¹⁶°) nach `SensorStatus.RangeBiasM`/`.AzimuthBiasDeg`
(**nil = keine Korrektur**, nie 0; FRN 7/8 wurden bisher nur übersprungen). Neuer
Pfad: `health.SensorDetail` je Feed → `FeedSnapshot.Sensors` → WS
`FeedStatusMessage.sensors[]` **und** Admin `/api/admin/feeds/health` `sensors[]`.
Frontend: geteilte Helfer (`formatSensorBias`/`describeSensor`/`sensorNeedsAttention`
in `admin/feedHealth.js`), Store (`feedSensors`/`sensorDetails`), operativer
**FeedStatusChip** als Menü, `AdminFeeds`-Zeile. **Bewusst kein Prometheus-Metrik**
(Kardinalitäts-Regel WF2-23). Additiv, kein Wire-/ICD-Bruch. Grundwahrheit:
Fireflys `sensor_with_bias_matches_reference_dump` (Dump SIC 1, +150 m / +0,30°).
Register: **FR-DATA-009**. Gates grün (`go test ./...`, vitest 537, `npm run build`,
gofmt/vet/golangci-lint). dist neu.

**Nächster Schritt:** Cross-Project-Nachzug weiter — **#238** (I062/380 Mode-S-DAPs,
Selected Altitude fürs Level-Bust-Bild) bzw. nach abgestimmter Reihenfolge.

## ✈️ Stand 2026-07-15 (ASD-Features: Monosensor-Hinweis + Ident-Highlight — #236, FR-DATA-008)

**In normaler Sprache:** Zwei kleine, aber operativ nützliche Signale, die schon
auf dem Draht lagen, aber bisher weggeworfen wurden, sind jetzt für den Lotsen
sichtbar:

- **Ident-Highlight:** Wenn ein Lotse „…, squawk ident" sagt und der Pilot den
  **IDENT-Knopf** drückt, leuchtet **genau dieses eine Flugzeug** jetzt mit einem
  amberfarbenen Ring auf — die klassische „welcher bist du?"-Bestätigung. Der
  Ring erscheint und verschwindet von selbst mit dem Puls (~15–30 s), ohne
  eigenen Timer.
- **Monosensor-Hinweis:** Tracks, die gerade nur **eine** Quelle bestätigt
  (keine Kreuz-Prüfung → anfälliger für Geister/Bias), tragen einen dezenten
  „*"-Marker am Label und eine erklärende Zeile im Detail-Panel.

**Fachlich/technisch:** Beide Bits sitzen in Oktett 1 von **I062/080** (MON `0x80`,
SPI `0x40`) — dem immer vorhandenen Oktett; **additiv**, kein Wire-/ICD-Bruch.
Decoder liest sie nach `TrackStatus.Monosensor`/`.SPI` (`pkg/cat062`), Broadcaster
reicht sie als `mono`/`spi` (`omitempty`) an die SPA. Frontend: MON-Marker im
Label (`label.js`) + Feature-Properties (`tracks.js`); **SPI-Ring** als
gefilterter Circle-Layer auf der Track-Quelle (`addSpiHighlightLayer`,
`spi==true`, `layers.js`/`engine.js`); Detail-Panel-Zeilen „Ident aktiv" /
„Monosensor" (`TrackDetailCard.vue`). Grundwahrheit: Fireflys ICD §4.1 (3.2.0),
Encoder-Test `track_status_carries_mon_and_spi_in_octet_one` (Status-Oktett
`0xC0` = MON|SPI). Register: **FR-DATA-008**. Tests: `status_test.go` (byte-genau),
`label.test.js`, `tracks.test.js`. Gates grün (`go test ./...`, vitest 535, `npm
run build`, gofmt/vet). dist neu gebaut.

**Nächster Schritt:** Cross-Project-Nachzug weiter — **#237** (CAT063
Registrierungs-Bias I063/080/081) bzw. nach abgestimmter Reihenfolge.

## 🛡️ Stand 2026-07-15 (Sicherheit: Empfangspfad gegen bösartige Datagramme gehärtet — #235, NFR-SAFE-001)

**In normaler Sprache:** Wayfinder empfängt das Luftlagebild als ungeschützten
Netzwerk-Strom. Ein einzelnes absichtlich kaputtes Datenpaket durfte den
Empfänger bisher **einfrieren** — und weil an einem Empfänger alle Lotsen-Bildschirme
hängen, wäre damit das Lagebild für alle ausgefallen. Diese Lücke ist geschlossen:
kaputte Pakete werden jetzt sauber verworfen, der Empfänger läuft weiter. Für den
Lotsen ändert sich **nichts Sichtbares** — es ist reine Absicherung, kein neues
Feature.

**Auslöser:** Firefly hat denselben Fehlerklassen-Bug in seinem eigenen Decoder
gefunden (Fuzzing, QW.2) und uns per Cross-Project-Issue **#235** zum Nachziehen
gebeten.

**Konkreter Befund (Fachdetail):** Die drei ASTERIX-Decoder (`pkg/cat062`,
`pkg/cat063`, `pkg/cat065`) parsen die FX-verkettete **FSPEC** ohne Obergrenze. In
**cat063** und **cat065** lief die FRN-Iteration über einen `uint8`-Zähler — eine
feindliche FSPEC von ≥ 37 Oktetten ließ ihn bei 255 überlaufen (Wrap → **Endlos­schleife**,
DoS am unauthentifizierten Multicast-Rand). cat062 war durch seine feste FRN-Liste
vor dem Wrap geschützt, las die überlange FSPEC aber ebenfalls unbegrenzt.

**Fix:** harte Obergrenze `maxFSPECOctets = 36` in allen drei Decodern (deckt FRN
1…252, ein Vielfaches jeder realen UAP) → Überlänge = Decode-Fehler; zusätzlich die
FRN-Schleife in cat063/065 auf `int` umgestellt (Wrap unmöglich, unabhängig vom Cap).
**Dauerhaft abgesichert** durch drei **Go-Fuzz-Targets** (`FuzzDecode*`, Seeds aus den
Referenz-Vektoren; ~0,8–0,9 M Ausführungen je 8 s ohne Fund) + Endlosschleifen-Regressions­tests
mit 2-s-Timeout-Wächter + neuen **CI-Fuzz-Job** (30 s je Target). Kein Wire-/ICD-Bezug,
kein Lockstep. Register: **NFR-SAFE-001**. Gates grün (`go build`/`vet`/`gofmt`/`go test ./...`).

**Nächster Schritt:** Cross-Project-Nachzug-Reihenfolge weiterarbeiten — als Nächstes
**#236** (I062/080 MON/SPI-Flags) bzw. nach abgestimmter Reihenfolge. #244 (FPL.0) ist
bestätigt und geschlossen.

## 🐞 Stand 2026-07-08 (UI-Fix-Batch — Sidebar-Animation, Icon-Überlappung, Profil-Icon, Ereignis→Track; FR-UI-029)

Vier Betreiber-Mängel (Video + Foto) behoben + eine Bedien-Erweiterung:

- **Sidebar-Reflow (Bug 1) + Scrollbalken-Blitzen (Bug 2):** Das ausklappende
  Nav-Panel baute die Schrift sichtbar auf / stauchte sie beim Einklappen, und
  ein Scrollbalken tauchte kurz auf. Ursache: `.nav-panel` war `flex:1`, wuchs
  also während der Drawer-**Breiten**-Animation mit und brach den Text neu um.
  Fix: **feste Panelbreite** (offene Drawer-Breite − Rail; 248 px Desktop / 304 px
  iPad-Band), Inhalt liegt sofort final, `.nav-two-col overflow:hidden` clippt →
  sauberer Wisch-Reveal statt Neu-Layout; `.nav-panel__body overflow-x:hidden`.
- **Icon-Überlappung (Bug 3):** Profil-Schalter + Ereignis-Glocke stapelten als
  zwei Extra-Zeilen im Top-Right-Cluster → die Map-Controls (`top:100px`) saßen
  darauf. Fix: Profil + Glocke in **eine** kompakte Aktionszeile
  (`.cluster-actions`); `MapControls` → `top:140px`, `TrackDetailPanel` →
  `top:220px` (gleiche Controls→Detail-Distanz wie zuvor).
- **Profil nur als Icon (Bug 4):** `ViewProfileMenu` ist ein Icon-Button mit
  **Hover-Tooltip** (aktiver Profilname) statt sichtbarem Label — hält den
  Lotsen-Scope aufgeräumt.
- **Ereignis→Track (Bug 5):** Klick auf eine **noch aktive** „Track N
  erschienen"-Zeile selektiert den Track (Detail-Panel + Halo, Kamera-`easeTo`).
  Store spiegelt das Live-Track-Set (`liveTrackNums` aus `liveTrackFeatures`);
  nur Zeilen mit noch aktivem Track sind klickbar (Fadenkreuz-Affordanz);
  Engine-`selectTrackByNum` (No-op, wenn Track weg → Panel bleibt offen).
  Ring-Puffer bleibt `MAX_EVENTS=200` (≫50) mit vorhandenem Scroll.
- **Kein CAT062-/Backend-Bezug** (reine Frontend-Chrome).
- **Tests:** `asd.test.js` (`liveTrackNums`), `eventPanel.test.js`
  (Selektierbarkeit/`select-track`/Engine), `viewProfileMenu.test.js`
  (Icon-only + Tooltip), `layerSidebarCleanup.test.js` (feste Panelbreite).
  **vitest 525 grün**, `vite build` + `dist` neu, `go build ./...` grün.
- **Nächster Schritt:** offen — auf Betreiber-Input warten.

## 🐞 Stand 2026-07-08 (UI-Fix — Fluginfo rechts + Sidebar-Trennlinie)

- **Fluginfo-Karte (`TrackDetailPanel`, FR-UI-005):** lag oben **links** (Offset
  = Rail-Breite) und wurde vom **ausgeklappten** Navigation-Panel (LAYER/FILTER)
  überdeckt (Betreiber-Meldung + Screenshot). Jetzt **am rechten Rand** verankert,
  **unter** dem Top-Right-Cluster + den Map-Controls (top ~180px), sodass sie diese
  Chrome nicht überlappt und das linke Panel sie nie verdeckt.
- **Rail↔Panel-Trennlinie (`NavigationRail`, #176/FR-UI-008):** die vertikale
  `v-divider` streckte sich in der Flex-Zeile nicht zuverlässig auf volle Höhe →
  kaum sichtbar. Ersetzt durch einen **immer voll-hohen 1px-Streifen**
  (`--wf-border-strong`, dezent aber klar sichtbar) zwischen schmaler Sidebar und
  ausgeklapptem Panel.
- **Tests:** `responsive.test.js` (Karte rechts, kein Links-Offset),
  `layerSidebarCleanup.test.js` (Trennlinie voll-hoch + Border-Token). **vitest
  514 grün**, `vite build` + `dist` neu; Go unberührt.

## 🎯 Stand 2026-07-08 (View-Profile VP-5 — Apply-on-Login; **Feature komplett**)

- **VP-5 (FR-PROFILE-005):** Nach dem Login wird das **Default-Profil** automatisch
  angewandt. `profiles`-Store: `applyDefaultOnce()` (Guard `defaultApplied`) wendet
  das `is_default`-Profil **genau einmal pro App-Load** an (setzt `activeId`); ohne
  Default latcht der Guard nicht (retrybar). `ViewProfileMenu.vue` triggert es
  **erst wenn `asd.mapLoaded`** (Live-Watcher greifen) — nach `store.load()` und via
  `watch(mapLoaded)`. **Orthogonal** zur Tenant-Karten-Rahmung; überschreibt keine
  spätere manuelle Wahl. **Kein Backend-/CAT062-Bezug.**
- **Tests:** `profiles.test.js` (`applyDefaultOnce`: einmalig/No-op/retrybar),
  `viewProfileMenu.test.js` (mapLoaded-Gating). **vitest 513 grün**, `vite build` +
  `dist` neu.
- **✅ Feature View-Profile komplett (VP-1…VP-5):** bis zu 3 persönliche Anzeige-
  Profile benennen/speichern/abrufen, eins als Default beim Login. Server-seitig
  per-Nutzer gescopt + begrenzt.
- **Nächster Schritt:** offen — auf Betreiber-Input warten (Backlog: 2.0-SaaS-Pfad,
  DFS-AIXM #215, weitere ASD-Design-Angleichung).

## 🎯 Stand 2026-07-08 (View-Profile VP-4 — UI-Umschalter + Speichern-Dialog)

- **VP-4 (FR-PROFILE-004):** `ViewProfileMenu.vue` im ASD-Header-Cluster — Button
  (Label = aktives Profil) → `v-menu` mit Profilliste (Klick = **anwenden**,
  Stern = **Default**, Stift = **umbenennen**, Papierkorb = **löschen**) +
  „**Aktuelle Ansicht speichern…**"-`v-dialog` (Name + „Als Standard beim Login").
  Cap-Gating (≤3, „Maximal 3 Profile"), lädt `onMounted`. Verdrahtet den
  VP-3-Store; **kein** Backend-/CAT062-Bezug; `dist` neu.
- **Tests:** `viewProfileMenu.test.js` (Source-Guard: Store-Verdrahtung, Aktionen,
  Default-Stern, Cap, AsdView-Mount). **vitest 510 grün**, `vite build` + `dist` neu.
- **Nächster (letzter) Schritt:** **VP-5** — Apply-on-Login des Default-Profils
  (nach Karten-Init, orthogonal zur Tenant-Karten-Rahmung).

## 🎯 Stand 2026-07-08 (View-Profile VP-3 — Frontend-Store + Capture/Apply)

- **VP-3 (FR-PROFILE-003):** Pinia-`profiles`-Store (`load`/`saveCurrent`/`update`/
  `rename`/`overwrite`/`remove`/`setDefault`/`apply`, `canCreate`≤3,
  `defaultProfile`) gegen die VP-2-API. Reine, testbare Serialisierung in
  `profileSettings.js`: `captureSettings`/`applySettings` fangen/spielen die
  **Anzeige-Präferenzen** des asd-Stores (Layer/Airspace-Gruppen/Range-Rings/
  History/FL-Filter; **kein** Zentrum/Zoom — Option A), tolerant (unbekannte Keys
  übersprungen, `airspace` aus Gruppen abgeleitet). Karte folgt über bestehende
  MapCanvas-Watcher.
- **Noch keine UI** (VP-4) → keine Komponente importiert die Module → `dist`
  unverändert. **Kein CAT062-Bezug.**
- **Tests:** `profileSettings.test.js` (Capture/Apply/Toleranz/Round-Trip),
  `profiles.test.js` (CRUD gegen gemocktes fetch). **vitest 504 grün.**
- **Nächster Schritt:** **VP-4** — UI-Umschalter im ASD-Header + „Ansicht
  speichern"-Dialog (verdrahtet den Store, baut `dist` neu).

## 🎯 Stand 2026-07-08 (View-Profile VP-2 — user-gescopte REST-API)

- **VP-2 (FR-PROFILE-002):** Fünf Endpunkte hinter `tenantMW`+`pwGate` (kein
  Admin-Gate): `GET/POST /api/view-profiles`, `PUT/DELETE /api/view-profiles/{id}`,
  `POST /api/view-profiles/{id}/default`.
  - **Streng auf Session-`user_id` gescopt** (nie aus dem Body) → fremdes Profil =
    404 (keine Leckage). Validierung (`validateViewProfile`, rein/testbar): Name
    ≤60, `settings` **JSON-Objekt** ≤16 KiB, Toggle-Schlüssel opak. Cap→409,
    ungültig→422, kaputt→400, nil-Store→404, kein-Identity→401.
  - `ViewProfileStore`-Interface + `WithViewProfiles`-Builder (nil-safe),
    `ViewProfilesHandler()` Sub-Mux in `main.go` gemountet.
- **Tests:** `adminapi_view_profiles_test.go` (Validierung, CRUD, Scoping,
  Fehler-Codes, nil/401). `go build`/`vet`/`gofmt`/`golangci-lint` (0 issues) grün.
- **Nächster Schritt:** **VP-3** — Frontend-`profiles`-Store + reine
  `captureSettings`/`applySettings` (serialisiert die asd-Store-Toggles).

## 🎯 Stand 2026-07-08 (View-Profile VP-1 — Per-Nutzer-Store, Backend)

- **View-Profile (ADR 0023)** — neues Feature: persönliche, benannte Anzeige-
  Profile pro Nutzer (bis zu **3**, eins als **Login-Default**), Umfang **nur
  Anzeige-Präferenzen** (Layer/Airspace-Gruppen/Range-Rings/History/FL-Filter/
  Basiskarte; Betreiber-Wahl „Option A"). Getrennt von `view_configs`
  (Karten-Rahmung).
- **VP-1 (FR-PROFILE-001):** Persistenz-Grundlage. Migration `00022_user_view_profiles.sql`
  (opakes `settings JSONB`, partieller Unique-Index für Single-Default) +
  `ViewProfileRepo` (List/Create/Update/Delete/SetDefault/GetDefault). **Cap=3**
  per Transaktion + `pg_advisory_xact_lock` (→ `ErrProfileLimit`), **Single-Default**
  als Store-Invariante, **strikte Per-`user_id`-Ownership** (fremd → `ErrNotFound`).
  `settings` verbatim (Backend interpretiert nie). **Kein CAT062-Bezug.**
- **Tests:** `normalizeSettings` (unit) + `TestIntegrationViewProfilesCRUD` (CRUD,
  Cap, Single-Default, Cross-User-Isolation) **grün gegen echte PostgreSQL-16**.
  `go build`/`vet`/`gofmt` grün.
- **Nächster Schritt:** **VP-2** — user-gescopte REST-API `/api/view-profiles`
  (GET/POST/PUT/DELETE + `/default`) hinter `tenantMW`.
- **Betriebshinweis:** GitHub-MCP war zeitweise abgemeldet → PR ggf. manuell/nach
  Re-Autorisierung anlegen.

## 🎯 Stand 2026-07-08 (ASD-011b — Selektions-Umrandung des Labels)

- **ASD-011b — Selektions-Umrandung des Datenblock-Labels (FR-UI-028):** Bei
  Selektion bekommt das Datenblock-Label zusätzlich zum Symbol-Halo eine
  **abgerundete Rahmen-Box** in **neutralem Hellton** (`#f2f7fc`) — angeglichen
  ans Claude-Design-Template (Betreiber-Screenshot 2026-07-08), Farbe = Betreiber-
  Wahl „weiß/neutral hell".
  - **Technik:** `deconflictLabels` erzeugt für den selektierten Track aus der
    Label-Screen-Bbox einen **abgerundeten Ring** (reine `roundedRectRing`), jeder
    Punkt per **`map.unproject`** exakt zurückprojiziert → Box sitzt pixelgenau ums
    Label (gleicher Round-Trip wie der Drag-Fix). Eigene Line-Ebene über den
    Labels; nur 0/1 Feature. **Kein CAT062-Bezug.**
  - **Zuschnitt:** nur die Selektions-Umrandung; „alle Labels boxen" + Alarmfarben
    (STCA/EMG/DUP) bleiben separate Häppchen (STCA bräuchte Wire-Daten I062/340).
- **Tests:** `deconflict.test.js` (`roundedRectRing` Bounds/Clamp; Selektions-Box
  rahmt Label-Bbox exakt, nur selektierter Track). **vitest 489**, `vite build` +
  `dist` neu; Go unberührt.

## 🐞 Stand 2026-07-08 (Bugfix — Label-Drag springt weg / versetzt zur Maus)

- **Symptom:** Klick auf ein Track-Label (das per Leader-Linie mit dem Track
  verbundene Datenblock-Label) ließ das Label beim ersten Drag-Schritt
  **wegspringen** und danach **versetzt zur Maus** ziehen.
- **Ursache:** `deconflictLabels` rechnete die Label-Geo-Position aus dem
  Pixel-Offset per **hand-gerollter Web-Mercator-Formel mit `256·2^zoom`** —
  MapLibres Welt ist aber **`512·2^zoom`**. Das Label wurde dadurch am **doppelten**
  Pixel-Offset gerendert, während `drag.js` in exakten Pixeln (`sym+pin`, 1×)
  rechnete. Beim ersten Move las der Drag die 2×-Position zurück und verdoppelte
  den Pin → Sprung + konstanter Cursor-Versatz.
- **Fix:** `deconflictLabels` platziert das Label jetzt per **`map.unproject([lx,ly])`**
  (exakte Umkehr von `map.project`, gültig für jede Tile-Größe/Zoom/Breite) →
  `project(labelGeo) === sym+offset` exakt. Auto-Platzierung sitzt am gewollten
  Offset, Drag ist pixelgenau (kein Sprung, kein Versatz).
- **Tests:** neuer Round-Trip-Regressionstest in `deconflict.test.js` (Label-Geo
  projiziert exakt auf `sym+pin`, inkl. Leader-Endpunkt); `drag.test.js`
  unverändert grün. **vitest 485**, `vite build` + `dist` neu; Go unberührt.

## 🎯 Stand 2026-07-08 (ASD-013 — Alarm-/Ereignis-Panel)

- **ASD-013 — Alarm-/Ereignis-Panel (FR-UI-027):** Zuschaltbares Ereignis-Panel
  (Glocke oben rechts mit Ungesehen-Badge) protokolliert **Feed-Ausfall/-Degradation/
  -Erholung**, **Verbindungsverlust/-wiederherstellung** und **Track erschienen/
  beendet** — alles **client-seitig aus dem WS-Strom abgeleitet** (kein
  Wire-Change), automatisch mandanten-skopiert.
  - **Reine Ableitung** in `map/events.js` (`feedStatusEvent`/`connectionEvent`/
    `trackLifecycleEvents` + `SEVERITY_META`), **Ring-Puffer-Store**
    `stores/events.js` (`MAX_EVENTS=200`, neueste zuerst, Ungesehen-Zähler),
    `EventPanel.vue`, Engine-WS-Handler-Verdrahtung, Glocke/Badge in `AsdView.vue`.
  - **Rausch-Vermeidung:** erste Frame nach (Re)Connect **primet** nur die
    Baseline (kein „erschienen"-Flut); „beendet" **nur** per TSE (I062/080).
  - **Ehrliche Grenze:** keine Wire-Alarme (STCA/Militär/Hostile mangels Feld
    draußen, vgl. ASD-006/#18) — nur beobachtbare Zustandsübergänge.
- **Tests:** `events.test.js` (Ableitung), Store-Test (Ring-Puffer/Cap/unseen),
  `eventPanel.test.js` (Verdrahtung). **vitest 485**, `vite build` + `dist` neu;
  Go unberührt.
- **Damit ist „für beides go" (ASD-011 + ASD-013) abgeschlossen.**

## 🎯 Stand 2026-07-08 (ASD-011 — Erweitertes Track-Detail-Panel)

- **ASD-011 — Erweitertes Track-Detail-Panel (FR-UI-026):** Das Detail-Panel
  eines angeklickten Tracks zeigt zusätzlich zu Callsign/FL/Bodengeschwindigkeit/
  Mode 3-A/Status nun **Vertikaltendenz**, **Kurs über Grund** (aus Vx/Vy),
  **Position (WGS84)**, **Sensor-Aktualität** (Chips je Technologie mit
  Update-Alter + Frische-Farbe), **ICAO-Adresse**, **Positionsgenauigkeit** und
  **System (SAC/SIC)**.
  - **Formatierer** als reine, testbare Funktionen in `map/trackDetail.js`;
    Felder in `updateTracksLayer` auf die Feature-Properties gebacken, sodass das
    Panel sie direkt aus `store.selectedTrack` liest. **Kein CAT062-Bezug** — alle
    Felder bereits im WS-JSON.
  - **Ehrliche Grenze:** PSR erscheint nicht in „Sensor-Aktualität" (kein sauberes
    Per-Track-`psr_age`-Frische-Signal) → getragen über die „Herkunft"-Zeile.
- **Tests:** `trackDetail.test.js` (Formatierer, 28 Fälle), `tracks.test.js`
  (`extended detail fields (ASD-011)`). **vitest 456**, `vite build` + eingebettetes
  `dist` neu; Go unberührt (`go build ./...`).
- **Nächster Schritt:** **ASD-013** (Alarm-/Event-Panel, S3) als eigener PR.

## 🎯 Stand 2026-07-08 (ASD-014 Slice 4 — AoR-Namens-Picker; Thema rund)

- **ASD-014.4 — Namens-Picker für den AoR-Editor (FR-AERO-006):** Löst die
  „ID-Eingabe"-Grenze aus Slice 3 auf. Der Admin wählt die Lufträume **nach
  Namen**; gespeichert wird weiter die stabile `id`.
  - **Backend:** neuer Endpunkt `GET /api/admin/tenants/{id}/airspaces` (hinter
    `requireAdmin`) → Luftraum-Liste des Mandanten aus dem **vorhandenen**
    Aeronautik-Cache (`Registry.Serve`), projiziert auf `{id,name,type?,icao_class?}`,
    nach Name sortiert. Kein neuer Fetch; `pkg/adminapi` bleibt transport-agnostisch
    (Projektion im `cmd/wayfinder`-Adapter, robust gegen int/float64).
  - **Frontend:** `v-autocomplete` mit Items aus dem Endpunkt; gewählte, aber nicht
    (mehr) gecachte IDs bleiben als synthetische Items erhalten (kein stiller
    Verlust). Leerer Cache → Hinweis „erst OpenAIP konfigurieren".
- **Tests:** adminapi (Optionen/404/403), `projectAirspaces`/`propInt`, Store
  (`loadTenantAirspaces`), Editor-Wiring. **vitest 429**, `vite build` + `dist` neu;
  Go grün (`go test ./...`/`vet`/`gofmt`/`golangci-lint`).
- **ASD-014 (ADR 0021) damit vollständig rund:** .1 Transform, .2 AoR-Liste, .3
  Karten-Highlight + Editor, .4 Namens-Picker.

## 🎯 Stand 2026-07-07 (ASD-014 Slice 3 — AoR-Kartendarstellung + Editor; Thema abgeschlossen)

- **ASD-014.3 — AoR-Kartendarstellung + Editor (Frontend, FR-UI-025):** Schließt
  ADR 0021 end-to-end ab.
  - **Karte:** eigene AoR-Linien-Ebene über der Airspace-Quelle, gefiltert auf die
    `id`s aus `whoami.aor_airspace_ids` (Akzent `#00e676`); `session.aorAirspaceIds`
    → `engine.updateAoR`; `MapCanvas` reconcilet nach `initMap` (#219-Race) +
    watcht die Liste; Legenden-Toggle „Verantwortungsbereich (AoR)".
  - **Editor:** `AdminTenantDetail.vue` Chips-Feld für die stabilen OpenAIP-IDs,
    über die bestehende `saveTenantView` gespeichert; `validateView.js`-Parität
    (≤ 500 / ≤ 64 / keine Steuerzeichen).
  - **Ehrliche Grenze:** ID-Eingabe, noch kein Namens-Picker (bräuchte eine
    mandantenübergreifende Luftraum-Liste — Folgearbeit).
- **Tests:** session (`aorAirspaceIds`), validateView (AoR-Grenzen), Map-/Editor-
  Source-Guards. **vitest 427 grün**, `vite build` + eingebettetes `dist` neu; Go
  unberührt grün.
- **Nebenbei behoben:** FR-AERO-ID-Kollision (ASD-014 → FR-AERO-004/005; die IDs
  002/003 gehörten schon AERO-2/AERO-3).
- **Zusammenfassung ASD-014 (ADR 0021) komplett:** .1 Transform (`id`/Höhenbänder),
  .2 AoR-Liste am View-Config + whoami, .3 Karten-Highlight + Editor.

## 🎯 Stand 2026-07-07 (ASD-014 Slice 2 — AoR-Auswahl pro Mandant, Backend)

- **ASD-014.2 — AoR-Auswahl pro Mandant (Backend, FR-AERO-005):** Der Mandant
  konfiguriert seinen **Verantwortungsbereich** (CTR/TMA) als **explizite Liste
  stabiler OpenAIP-`id`s** (Auswahl-Semantik **Option 1**). Umgesetzt als **Variante
  A (whoami-Surfacing)** — `pkg/aeronautical` bleibt unangetastet:
  - **Store:** neue Spalte `view_configs.aor_airspace_ids` (JSONB, Migration
    `00021`, nullable = keine AoR); `ViewConfig.AoRAirspaceIDs` in Columns/Upserts/
    `viewJSONParams`/`scanViewConfig`.
  - **Admin-API:** `viewDTO`/`whoamiDTO`-Feld `aor_airspace_ids` (`omitempty`);
    `validateView` (Anzahl ≤ 500, id ≤ 64, keine Steuerzeichen), `normalizeAoRIDs`
    (Trim/Dedup/Reihenfolge). Editierbar über die bestehenden View-Routen (kein
    neuer Endpunkt). whoami liefert die effektive Liste an die ASD-SPA.
- **Tests:** Store-Round-Trip (real-PG) + `TestViewJSONParams`;
  `TestValidateViewAoRAirspaceIDs`/`TestNormalizeAoRIDs`/
  `TestWhoamiIncludesAoRAirspaceIDs`/`TestWhoamiOmitsAoRWhenUnset`.
  Doku: Milestone `ASD-014.2`, FR-AERO-005, TECHNICAL (`whoami`/`00021`).
  Gates grün: `go test ./...`, `go vet`, `gofmt`, `golangci-lint` (0 issues).
- **Nächster Schritt (noch nicht freigegeben):** **Slice 3 (Frontend)** —
  Highlight-Styling der AoR-Lufträme (Match `id` ∈ `aor_airspace_ids` aus whoami)
  + Legende + Editor (Lufträume nach Namen wählen → `id` speichern, mit
  Client-`validateView`-Parität); optional Höhenband-Label/-Filter aus `lower`/`upper`.

## 🎯 Stand 2026-07-07 (#219 — Gastmodus: „Ansicht zurücksetzen" springt auf Frankfurt)

- **Bugfix #219 (Regression aus #208 / ADR 0022; S2–S3, rein Frontend,
  CAT062-Draht-Vertrag unberührt):** Im Read-Only-Gastmodus (Impersonation,
  ADR 0008) öffnete die Karte nicht auf dem angesehenen Mandanten und
  „Ansicht zurücksetzen" zentrierte auf den globalen `WAYFINDER_MAP_CENTER_*`-
  Default (Demo: Frankfurt) statt z. B. auf EDDH.
  - **Ursache:** `initMap` ist asynchron. Beim Betreten des Gastmodus hält der
    Session-Store beim `MapCanvas`-Mount noch die veraltete, nicht-impersonierte
    Sicht (leeres `viewCenter`); das impersonation-aware `GET /api/whoami` landet
    erst *während* des `await initMap`. Der `watch(session.viewCenter →
    applyViewCenter)` feuert dann gegen ein noch `null`es `mapEngine` → die
    Nach-Zielung auf EDDH geht verloren, `effectiveCenter` (Ziel von `recenter()`)
    bleibt auf Frankfurt. Gastmodus-spezifisch, da nur der Admin den `adminGate`
    aus #208 durchläuft.
  - **Fix:** `MapCanvas` gleicht **nach** dem Auflösen von `initMap` die Karte auf
    die aktuelle effektive Sicht ab (`applyViewCenter(session.viewCenter)` +
    `applyWeatherAOI(session.aoi)`; No-op bei unveränderter Sicht). Deckt alle
    Race-Reihenfolgen ab (früh aufgelöst → `initMap`-Argument; während `initMap`
    → Reconcile; nach `initMap` → bestehender Watcher).
  - Doku: FR-UI-013 (Nachtrag #219), Regressionstest
    `mapCanvasViewCenter.test.js`. Gates: **vitest 416 grün** (+1),
    `go test`/`vet`/`gofmt` grün, `vite build` + eingebettetes `dist` neu.

## 🎯 Stand 2026-07-07 (ASD-014 Slice 1 — OpenAIP-Transform-Anreicherung für AoR)

- **ASD-014.1 — OpenAIP-Transform-Anreicherung (Backend-Vorbau, FR-AERO-004):**
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
  Doku: Milestone `ASD-014.1`, FR-AERO-004, TECHNICAL `/api/airspace`.
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
