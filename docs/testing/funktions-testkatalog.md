# Funktions-Testkatalog — Wayfinder (manuelles Durchklicken)

> **Ziel.** Für **jede** heute vorhandene Wayfinder-Funktion ein kurzer,
> reproduzierbarer Testablauf: **Vorbedingung → Schritte → erwartetes Ergebnis**,
> plus die **Firefly-Rolle** (liefert die Funktion Daten aus dem ASTERIX-Strom,
> oder läuft sie Firefly-unabhängig?). Gedacht für eine **bereits laufende**
> Wayfinder-Instanz — kein Setup, kein Deployment (das steht in
> `docs/E2E-ABNAHME.md` / `docs/INSTALLATION.md`).

## Konventionen & Firefly-Rolle

- **🔴 braucht Firefly-Strom** — die Funktion zeigt Daten aus CAT062/063/065;
  Vorbedingung ist ein **laufender, abonnierter Feed mit Live-Tracks**.
- **🟡 Firefly beteiligt** — Rückkanal (Kommando-API) oder je-Feed-Instanz.
- **⚪ Firefly-unabhängig** — reine Wayfinder-Funktion, ohne Feed testbar.

**Zwei Grund-Vorbedingungen**, im Text abgekürzt:
- **[V-AN]** = am ASD als Mandanten-Nutzer angemeldet.
- **[V-FEED]** = [V-AN] **und** mindestens ein abonnierter Feed liefert live
  Tracks (die Karte zeigt Flugzeuge, der Feed-Chip ist grün).
- **[V-ADMIN]** = als **Admin** angemeldet (Admin-Dashboard erreichbar).

> „Klick auf einen Track" heißt: ein Track-Symbol auf der Karte anklicken →
> rechts öffnet das **Detail-Panel** (`TrackDetailCard`).

---

## 1) Zugang, Mandanten & Admin

### T-AUTH-01 — Mandanten-Login ⚪
- **Vorbedingung:** abgemeldet; gültige Zugangsdaten eines Mandanten-Nutzers.
- **Schritte:** ASD-URL öffnen → Login-Maske → Benutzername + Passwort → „Anmelden".
- **Erwartet:** Karte lädt, der eigene Sektor ist zentriert; kein Zugriff ohne Login.

### T-AUTH-02 — Erzwungener Passwortwechsel (Zero-Touch) ⚪
- **Vorbedingung:** frisch geseedeter Admin (Standardpasswort noch aktiv).
- **Schritte:** mit Standard-Zugangsdaten anmelden.
- **Erwartet:** sofort die Passwort-ändern-Maske; **keine** andere Route erreichbar, bis das Passwort gewechselt ist.

### T-AUTH-03 — Sliding-Session + Login-Overlay ⚪
- **Vorbedingung:** [V-AN].
- **Schritte:** aktiv bleiben (Karte bedienen) → beobachten; danach lange nichts tun.
- **Erwartet:** bei Aktivität bleibt die Sitzung offen; nach Inaktivität erscheint ein **Login-Overlay** (Re-Login) statt Datenverlust.

### T-AUTH-04 — Absolutes Sitzungs-Maximum ⚪
- **Vorbedingung:** [V-AN], lange laufende Sitzung.
- **Schritte:** über das absolute Maximum hinaus angemeldet lassen (auch bei Aktivität).
- **Erwartet:** die Sitzung endet **spätestens** am absoluten Maximum → Re-Login nötig.

### T-AUTH-05 — Abmelden ⚪
- **Vorbedingung:** [V-AN].
- **Schritte:** Sidebar → Sektion **Account** → „Abmelden".
- **Erwartet:** zurück zur Login-Maske; Karte ohne Login nicht erreichbar.

### T-ADMIN-01 — Admin-Dashboard erreichbar, Nutzer nicht ⚪
- **Vorbedingung:** je einmal als Admin und als normaler Nutzer.
- **Schritte:** `/admin` bzw. den Admin-Einstieg der Navigation Rail öffnen.
- **Erwartet:** Admin sieht das Dashboard; normaler Nutzer wird abgewiesen (Zugriffs-Hinweis, kein Panel). Admin hat **kein** eigenes ASD-Kartenbild.

### T-ADMIN-02 — Mandanten-Lebenszyklus ⚪
- **Vorbedingung:** [V-ADMIN].
- **Schritte:** Mandant anlegen → Nutzer/Zugang vergeben → Mandant bearbeiten.
- **Erwartet:** Mandant erscheint in der Liste; Zugang funktioniert; Änderungen werden gespeichert.

### T-ADMIN-03 — Feed-Lebenszyklus + Abo ⚪→🔴
- **Vorbedingung:** [V-ADMIN].
- **Schritte:** Feed anlegen (Quelle konfigurieren) → einem Mandanten zuweisen/abonnieren → Abo wieder entfernen.
- **Erwartet:** Feed erscheint; nach Abo sieht der Mandant den Feed (bei laufender Quelle live Tracks); nach Entfernen nicht mehr. *(Sichtbarwerden der Tracks = 🔴.)*

### T-ADMIN-04 — Live-Apply von Sicht-/Abo-Änderungen ⚪
- **Vorbedingung:** [V-ADMIN] + parallel ein angemeldeter Mandanten-Client.
- **Schritte:** Sicht (Zentrum/FL-Band/AoR) des Mandanten im Admin ändern, speichern.
- **Erwartet:** die Änderung wirkt beim Mandanten **ohne Neuladen** (Karte springt / Filterband passt sich an).

### T-ADMIN-05 — „View as Tenant X" (Read-only-Impersonation) ⚪/🟡
- **Vorbedingung:** [V-ADMIN].
- **Schritte:** einen Ziel-Mandanten „ansehen als" wählen → Impersonation-Leiste erscheint → eine Schreibaktion versuchen (z. B. manuelle Korrelation).
- **Erwartet:** man sieht die Lage des Ziel-Mandanten; **Schreibaktionen sind gesperrt** (403), die Leiste macht die Impersonation sichtbar; Beenden kehrt zur Admin-Sicht zurück.

### T-SEC-01 — Cross-Tenant-Isolation 🔴
- **Vorbedingung:** zwei Mandanten mit **unterschiedlichen** Feeds/Sektoren, live.
- **Schritte:** parallel je einen Nutzer beider Mandanten anmelden.
- **Erwartet:** jeder sieht **nur** seine abonnierten Feeds/Tracks; kein Fremd-Track leckt durch (server-seitig erzwungen).

---

## 2) Live-Luftlagebild

### T-LIVE-01 — Tracks auf der Karte 🔴
- **Vorbedingung:** [V-FEED].
- **Schritte:** Karte betrachten.
- **Erwartet:** Flugzeuge erscheinen an ihren WGS84-Positionen und bewegen sich mit den Feed-Updates.

### T-LIVE-02 — Erweiterter Data Block 🔴
- **Vorbedingung:** [V-FEED].
- **Schritte:** Labels der Tracks ansehen.
- **Erwartet:** Label zeigt Kennung/FL und — sofern vorhanden — Vertikaltendenz ▲/▼ und Kurven-Indikator →/←.

### T-LIVE-03 — Spur-Herkunft als Symbol-Form 🔴
- **Vorbedingung:** [V-FEED] mit gemischten Quellen (z. B. ADS-B + SSR).
- **Schritte:** Symbol-Formen vergleichen; Legende in der Sidebar (Sektion Layer) heranziehen.
- **Erwartet:** die Symbol-Form unterscheidet PSR/SSR/ADS-B usw. gemäß Legende.

### T-LIVE-04 — Track-Ende (TSE) → sofortiges Entfernen 🔴
- **Vorbedingung:** [V-FEED]; ein Track, der endet (Landung/Verlassen).
- **Schritte:** einen Track beim Verschwinden beobachten.
- **Erwartet:** bei Track-Ende (TSE-Bit) verschwindet der Track **sofort mit sanftem Ausblenden**, nicht erst nach Timeout.

### T-LIVE-05 — History/Trails + Dots 🔴/⚪
- **Vorbedingung:** [V-FEED]; History-Dots aktiv (Sidebar).
- **Schritte:** einen Track eine Weile verfolgen; History-Retention (Minuten) in der Sidebar ändern.
- **Erwartet:** Trail/Dots zeigen die zurückgelegte Spur; das Zeitfenster ändert sich live mit der Einstellung.

### T-LIVE-06 — Bewegungsvektor 🔴
- **Vorbedingung:** [V-FEED].
- **Schritte:** bewegte Tracks ansehen.
- **Erwartet:** ein Voraus-Vektor zeigt Kurs/Geschwindigkeit (aus Vx/Vy).

### T-LIVE-07 — Anti-Garbling: Label-Deconfliction + Pinning ⚪
- **Vorbedingung:** [V-FEED] mit **dicht** beieinander liegenden Tracks.
- **Schritte:** überlappende Labels beobachten; ein Label per Drag&Drop verschieben.
- **Erwartet:** Labels weichen sich automatisch aus; ein gezogenes Label bleibt an der neuen Stelle gepinnt.

---

## 3) Track-Detail-Panel (Klick auf einen Track)

### T-DTL-01 — Erweitertes Detail-Panel 🔴
- **Vorbedingung:** [V-FEED].
- **Schritte:** einen Track anklicken.
- **Erwartet:** Panel zeigt Position (WGS84), Kennung (Mode 3/A, ICAO), SAC/SIC, Genauigkeit, Bodengeschwindigkeit/Kurs, Status und die Sensor-Aktualität je Technologie.

### T-DTL-02 — Vertikal-Kette 🔴
- **Vorbedingung:** [V-FEED]; Track mit frischer Vertikalschätzung.
- **Schritte:** Track anklicken → Zeilen „Barometrische/Geometrische Höhe" und „Steig-/Sinkrate" ansehen.
- **Erwartet:** gefilterte Baro-Höhe (mit A/FL je QNH-Bit), geometrische Höhe und Steig-/Sinkrate erscheinen; die Vertikaltendenz ▲/▼ passt zur Rate.

### T-DTL-03 — Kinematik-Kette 🔴
- **Vorbedingung:** [V-FEED]; kurvender/beschleunigender Track.
- **Schritte:** Track anklicken → „Kurventrend", „Geschwindigkeitstrend", „Beschleunigung".
- **Erwartet:** Kurven-/Geschwindigkeitstrend + Beschleunigung erscheinen; im Label der Kurven-Indikator →/←.

### T-DTL-04 — Mode-S-DAPs + Level-Bust 🔴
- **Vorbedingung:** [V-FEED]; Track mit Mode-S-DAPs (Selected Altitude).
- **Schritte:** Track anklicken → „Selected Altitude", „Magnetischer Steuerkurs", „IAS", „Mach".
- **Erwartet:** die Werte erscheinen; weicht Selected Altitude deutlich von der Ist-Höhe ab, wird das als **Level-Bust** hervorgehoben.

### T-DTL-05 — Flugplan-Korrelation (Anzeige) + Callsign-Mismatch 🔴
- **Vorbedingung:** [V-FEED]; ein **korrelierter** Track (Firefly sendet I062/390).
- **Schritte:** Track anklicken → „Plan-Callsign" + „Route (ADEP → ADES)".
- **Erwartet:** Plan-Callsign und Route erscheinen; weicht der gefilete Plan-Callsign von der gesendeten Kennung (I062/245) ab, erscheint ein **Mismatch-Hinweis** („≠") am Label und im Panel.

### T-DTL-06 — Manuelle Korrelation: pinnen 🟡
- **Vorbedingung:** [V-FEED]; Funktion aktiv (`correlation_available`); Track mit **feed_id** (Katalog-Feed, nicht ENV-Fallback); **nicht** unter Impersonation.
- **Schritte:** Track anklicken → Abschnitt **„Korrelation"** → Feld „Plan-Callsign" prüfen/setzen → **„Korrelieren"**.
- **Erwartet:** grüne Erfolgszeile „Track N mit … korreliert."; Firefly übernimmt die Zuordnung (der Plan-Callsign erscheint anschließend am Track). *Firefly-Rolle: die Kommando-API `/correlation` nimmt den Befehl an.*

### T-DTL-07 — Manuelle Korrelation: „unkorreliert" erzwingen 🟡
- **Vorbedingung:** wie T-DTL-06.
- **Schritte:** Abschnitt „Korrelation" → **„Unkorreliert"**.
- **Erwartet:** Erfolgszeile „Track N als unkorreliert markiert."; die Automatik-Zuordnung ist unterdrückt.

### T-DTL-08 — Manuelle Korrelation: zurücksetzen 🟡
- **Vorbedingung:** ein Track mit zuvor gesetztem manuellem Pin.
- **Schritte:** Abschnitt „Korrelation" → **„Zurücksetzen"**.
- **Erwartet:** Erfolgszeile „… aufgehoben."; Fireflys Automatik übernimmt wieder (idempotent).

### T-DTL-09 — Manuelle Korrelation: ehrliche Fehlermeldungen 🟡
- **Vorbedingung:** wie T-DTL-06.
- **Schritte:** einen **unbekannten** Callsign eingeben → „Korrelieren".
- **Erwartet:** **gelbe** Meldung „Kein Flugplan mit dieser Kennung gefunden." (422); analog „… keine Flugpläne konfiguriert" (409) bzw. „Für diesen Feed nicht berechtigt" (403). Kein Absturz, klarer Klartext.

### T-DTL-10 — Korrelations-Steuerung korrekt ausgeblendet 🟡/⚪
- **Vorbedingung:** einmal mit deaktivierter Funktion (kein Command-Token) **oder** ein Track vom ENV-Fallback-Feed (ohne feed_id).
- **Schritte:** Track anklicken → nach dem Abschnitt „Korrelation" suchen.
- **Erwartet:** der Abschnitt ist **nicht** sichtbar (keine Knöpfe, die ohnehin nur 503 lieferten).

---

## 4) Filter, Werkzeuge & Ereignisse

### T-TOOL-01 — FL-Filter (ausblenden/entsättigen) ⚪
- **Vorbedingung:** [V-FEED].
- **Schritte:** Sidebar → Sektion **Filter** → FL-Band setzen (min/max) → zwischen „ausblenden" und „entsättigen" wechseln.
- **Erwartet:** Tracks außerhalb des Bands verschwinden bzw. werden gedimmt; **unbekannte Höhen bleiben sichtbar** (Safety).

### T-TOOL-02 — FL-Band-Hinweis der Sicht ⚪
- **Vorbedingung:** [V-AN] mit konfiguriertem FL-Band der Sicht.
- **Schritte:** FL-Filter öffnen.
- **Erwartet:** das zulässige FL-Band der effektiven Ansicht wird als grauer Hinweis/Platzhalter angezeigt.

### T-TOOL-03 — Range-Rings + Scale-Bar + Kompass ⚪
- **Vorbedingung:** [V-AN].
- **Schritte:** Range-Rings in der Sidebar einschalten → Abstand (5/10/15 NM) + Anzahl ändern; Karte drehen → Kompass klicken.
- **Erwartet:** konzentrische Ringe **konstanter Boden-Distanz** um den Sicht-Mittelpunkt; Nautik-Maßstab unten; Kompass zeigt Bearing, Klick stellt Nord-up.

### T-TOOL-04 — Mess-Werkzeuge RBL/DIST/QDM ⚪
- **Vorbedingung:** [V-AN].
- **Schritte:** Mess-Werkzeug (Karten-Controls rechts) wählen → zwei Punkte setzen.
- **Erwartet:** Peilung/Distanz (bzw. QDM) werden als Linie + Readout angezeigt.

### T-TOOL-05 — Alarm-/Ereignis-Panel + Sprung zum Track 🔴
- **Vorbedingung:** [V-FEED].
- **Schritte:** Ereignis-Panel öffnen → auf eine „Track N erschienen"-Zeile klicken.
- **Erwartet:** Ereignisse (erschienen/verschwunden) werden gelistet; Klick auf eine **noch aktive** Zeile springt zum Track und selektiert ihn.

---

## 5) Karte & aeronautischer Kontext (OpenAIP)

### T-MAP-01 — Lufträume + Gruppenfilter ⚪
- **Vorbedingung:** [V-AN].
- **Schritte:** Sidebar → Layer → Luftraum-Gruppen CTR/TMA/Restricted/Info einzeln schalten.
- **Erwartet:** die jeweilige Gruppe erscheint/verschwindet; sind **alle** aus, ist der Luftraum-Layer aus.

### T-MAP-02 — Navaids & Waypoints ⚪
- **Vorbedingung:** [V-AN].
- **Schritte:** Navaids- bzw. Waypoints-Layer schalten.
- **Erwartet:** die Symbole/Marker erscheinen bzw. verschwinden.

### T-MAP-03 — Flughäfen & Runways ⚪
- **Vorbedingung:** [V-AN].
- **Schritte:** Airport- und Runway-Layer (opt-in) einschalten.
- **Erwartet:** Flughafen-Marker und Pisten-Mittellinien erscheinen.

### T-MAP-04 — AoR (Verantwortungsbereich) ⚪
- **Vorbedingung:** [V-AN] mit konfiguriertem AoR der Sicht.
- **Schritte:** AoR-Layer betrachten; als Admin im View-Editor AoR-Lufträume per Namens-Picker ändern.
- **Erwartet:** die zugeordneten Lufträume (CTR/TMA) sind hervorgehoben; Änderungen werden gespeichert und dargestellt.

### T-MAP-05 — ICAO-Flugplatzsuche im View-Editor ⚪
- **Vorbedingung:** [V-ADMIN].
- **Schritte:** View-Editor → Flugplatz per ICAO (z. B. `EDDF`) suchen → als Zentrum übernehmen.
- **Erwartet:** Treffer erscheinen; die Sicht zentriert auf den Flugplatz.

### T-MAP-06 — Theme + Startausschnitt der Sicht ⚪
- **Vorbedingung:** [V-AN].
- **Schritte:** ASD öffnen (Dark/OSM je Konfiguration).
- **Erwartet:** Basiskarte im konfigurierten Theme; die Karte öffnet auf dem **eigenen** Sektor (Zentrum/Zoom der Sicht).

### T-MAP-07 — Coverage-Ringe (Sensor-Reichweite) ⚪
- **Vorbedingung:** [V-AN]; serverseitig Coverage-Sensoren konfiguriert.
- **Schritte:** Sidebar → „Radarabdeckung" schalten.
- **Erwartet:** Sensor-Reichweiten als Ringe; **ohne** konfigurierte Sensoren ist der Schalter deaktiviert (statt wirkungslos).

---

## 6) Wetter & Betriebskontext

### T-WX-01 — DWD-Regenradar-Overlay ⚪
- **Vorbedingung:** [V-AN]; Radar-Quelle aktiv (Entitlement vorhanden).
- **Schritte:** Sidebar → Wetter-Radar schalten.
- **Erwartet:** Niederschlags-/Radarbild unter der Luftlage, auf den eigenen Sektor geclippt; Attribution „© Deutscher Wetterdienst". Ohne Quelle/Entitlement ist der Schalter deaktiviert.

### T-WX-02 — DWD-Wetterwarnungen-Overlay ⚪
- **Vorbedingung:** [V-AN]; Warn-Quelle aktiv.
- **Schritte:** Wetterwarnungen schalten.
- **Erwartet:** amtliche Warnpolygone nach Warnstufe eingefärbt, über dem Radar, unter Aeronautik/Tracks.

### T-WX-03 — QNH-Infobox ⚪
- **Vorbedingung:** [V-AN]; QNH-Quelle an + Entitlement + Flugplatz in der Sicht (`qnh_icao`).
- **Schritte:** Kopfzeilen-Infobox ansehen.
- **Erwartet:** aktueller QNH-Wert für den konfigurierten Flugplatz.

---

## 7) Feed-Gesundheit & Sensorstatus

### T-FEED-01 — Feed-Staleness-Banner 🔴
- **Vorbedingung:** [V-FEED].
- **Schritte:** den Feed/die Quelle stoppen (Heartbeat aussetzen) und warten (> Stale-Timeout); danach wieder starten.
- **Erwartet:** bei ausbleibendem CAT065-Heartbeat erscheint das **Stale-Banner** und `/ready` wird **nicht ready**; „leerer Himmel" (Heartbeat da, keine Tracks) löst das Banner **nicht** aus. Nach Neustart verschwindet es.

### T-FEED-02 — Sensor-Status-Chip (CAT063) 🔴
- **Vorbedingung:** [V-FEED] mit mehreren Sensoren.
- **Schritte:** Feed-Chip ansehen und aufklappen.
- **Erwartet:** Aggregat grün/gelb/rot; aufgeklappt pro Sensor der Status + (bei Degradierung) der **aktionierbarste Grund** (auth > rate_limited > unreachable) und ggf. Bias-Werte.

### T-FEED-03 — Herkunft/Aktualität je Technologie 🔴
- **Vorbedingung:** [V-FEED] mit gemischten Quellen.
- **Schritte:** Track anklicken → „Sensor-Aktualität".
- **Erwartet:** je beitragende Technologie ein Chip mit Alter; frische Quellen sind hervorgehoben.

---

## 8) Feeds, Quelltypen & Orchestrierung

### T-ORCH-01 — Per-Feed-Tracker-Orchestrierung 🟡
- **Vorbedingung:** [V-ADMIN]; orchestrierter Betrieb (Docker-Backend).
- **Schritte:** einen Feed anlegen/abonnieren → beobachten, dass eine Firefly-Instanz startet; Abo/Feed entfernen → Instanz verschwindet.
- **Erwartet:** je Feed genau **eine** Firefly-Instanz (Auto-Spawn durch den Reconciler); Aufräumen beim Entfernen. *Firefly-Rolle: Firefly = der gespawnte Tracker je Feed.*

### T-ORCH-02 — Quelltypen konfigurieren 🟡
- **Vorbedingung:** [V-ADMIN].
- **Schritte:** einen Feed mit verschiedenen Quelltypen anlegen: `adsb_aggregator` (auth-frei), `adsb_asterix`/`mlat_asterix` (lokal, ASTERIX/UDP), `radar_asterix` (mit Standort), OpenSky (mit Credentials).
- **Erwartet:** die gespawnte Firefly-Instanz konsumiert die konfigurierten Quellen (Env-Kontrakt); bei laufender Quelle erscheinen Tracks.

### T-ORCH-03 — Quell-Credential-Isolation 🟡
- **Vorbedingung:** [V-ADMIN]; ein Feed mit credential-behafteter Quelle (z. B. OpenSky).
- **Schritte:** Credential über die UI hinterlegen (versiegelt) → Feed starten.
- **Erwartet:** das Secret liegt **verschlüsselt** (kein Klartext at rest); nur der privilegierte Orchestrator entschlüsselt und injiziert es in die Firefly-Instanz; ohne Deployment-Key laufen credentialled Quellen anonym / Secret-Routen sind deaktiviert (503).

### T-ORCH-04 — Feed-Gesundheit bei quellenlosem Feed 🔴
- **Vorbedingung:** [V-ADMIN]; ein Feed **ohne** Quellen.
- **Schritte:** Feed anlegen/abonnieren (keine Quelle) → Lage ansehen.
- **Erwartet:** „leerer Himmel" mit laufendem CAT065-Heartbeat (Feed-Chip grün, keine Tracks) — kein Stale-Banner, keine Fake-Tracks.

---

## 9) Sichten & Profile

### T-VIEW-01 — Zugeschnittener Track-Dienst (fail-closed) 🔴
- **Vorbedingung:** zwei Mandanten mit unterschiedlichem AOI/FL-Band, gleicher Feed live.
- **Schritte:** beide anmelden und die sichtbaren Tracks vergleichen.
- **Erwartet:** jeder Mandant sieht **nur** Tracks in seiner AOI-BBox + seinem FL-Band; der Filter sitzt **server-seitig** (kein Fremd-Track im Browser).

### T-VIEW-02 — View-Profile speichern & umschalten ⚪
- **Vorbedingung:** [V-AN].
- **Schritte:** Kartenausschnitt/Filter einstellen → über das View-Profile-Menü als Profil **speichern** → anderes Profil wählen → zurückschalten.
- **Erwartet:** Profile werden gespeichert (per Nutzer); Umschalten wendet Zentrum/Zoom/Filter an.

### T-VIEW-03 — Apply-on-Login ⚪
- **Vorbedingung:** [V-AN] mit gespeichertem Standard-Profil.
- **Schritte:** abmelden → erneut anmelden.
- **Erwartet:** das hinterlegte Profil wird beim Login automatisch angewandt.

---

## Kurz-Checkliste (zum Abhaken)

| Bereich | Tests | Braucht Feed? |
|---|---|---|
| 1 Zugang/Admin | T-AUTH-01…05, T-ADMIN-01…05, T-SEC-01 | tw. 🔴 |
| 2 Live-Bild | T-LIVE-01…07 | 🔴 |
| 3 Detail-Panel | T-DTL-01…10 | 🔴 (Korrelation 🟡) |
| 4 Filter/Werkzeuge | T-TOOL-01…05 | tw. 🔴 |
| 5 Karte/Aeronautik | T-MAP-01…07 | ⚪ |
| 6 Wetter | T-WX-01…03 | ⚪ |
| 7 Feed-Gesundheit | T-FEED-01…03 | 🔴 |
| 8 Orchestrierung | T-ORCH-01…04 | 🟡/🔴 |
| 9 Sichten/Profile | T-VIEW-01…03 | tw. 🔴 |
