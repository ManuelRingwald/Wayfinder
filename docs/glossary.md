# Wayfinder – Glossar

Fachbegriffe der Luftfahrtverkehrskontrolle und Wayfinder-Spezifika.

---

## ASD (Air Situation Display)

Die **grafische Darstellung des Luftverkehrs** aus der Perspektive eines
Lotsen oder Fluglotsen. Zeigt Luftfahrzeug-Positionen (Tracks), Flugwege,
Höhe, Geschwindigkeit und Identität (Rufzeichen, Transponder-Code) live auf
einer Karte. Wayfinder ist das ASD-Frontend.

---

## Track

Ein kontinuierlich verfolgtes Flugziel mit:
- **Position** (geographische Koordinaten: Länge/Breite)
- **Höhe** (Flight Level oder Fuß)
- **Geschwindigkeit** (Vektor: Richtung + Betrag)
- **Identität** (Rufzeichen, Flugplan-Nummer, Transponder-Code, ggf. ADS-B-ICAO)

Tracks werden vom Firefly-Tracker berechnet (Fusion von Primär- und
Sekundärradar-Rohdaten) und von Wayfinder visualisiert.

---

## Firefly

Das **Backend-Projekt** — ein Radar-Tracker in Rust, der verrauschte
Einzelmeldungen von Primär- (PSR) und Sekundärradar (SSR) zu kontinuierlichen
Tracks berechnet. Firefly läuft als Dienst und gibt Tracks über WebSocket an
Wayfinder.

---

## WebSocket-Feed

Die Live-Datenverbindung zwischen Firefly (Server) und Wayfinder (Client). Über
WebSocket empfängt Wayfinder kontinuierlich aktualisierte Track-Daten (Position,
Geschwindigkeit, Identität) und kann neue ASD-Frames rendern.

---

## PSR (Primärradar / Primary Surveillance Radar)

Konventionelles Radar: Der Sender pulst elektromagnetische Energie aus, Ziele
reflektieren, das Radar empfängt Echos zurück und misst so Position (Entfernung,
Peilung) und Geschwindigkeit (Doppler). Daten sind verrauscht, brauchen Filterung.

---

## SSR (Sekundärradar / Secondary Surveillance Radar)

Das Flugzeug antwortet aktiv auf eine Abfrage mit einem kodierten Transponder-Signal
(Squawk-Code, Höhe, oft auch Rufzeichen/ADS-B). Viel präziser als PSR, aber
braucht kooperative Flugzeuge. Firefly nutzt SSR zur Identitätskorrelation und
Höhen-Verifikation.

---

## Fusion (Track Fusion)

Die mathematische Verschmelzung von mehreren Datenquellen (PSR + SSR, evtl.
mehrere Radare) zu einem **bestmöglichen Track**. Firefly nutzt Kalman-Filter,
Gating, Global Nearest Neighbor (GNN) etc. — Wayfinder empfängt das Ergebnis.

---

## Karte / Kartenkachel (Map Tile)

Die Hintergrundbild-Schicht einer ASD: Topographische Karte oder Luftfahrtkarte
(mit Lufträumen, Wegpunkten, Funkfrequenzen, etc.). Wayfinder zeigt Kartenkacheln
(z.B. von OpenStreetMap oder spezialisierten Luftfahrt-Quellen) und überlagert
Tracks.

---

## MapLibre GL JS

Open-Source-Bibliothek (WebGL) zum Rendern von Vektor- und Raster-Karten im
Browser — eine Art "Motor" für die Kartendarstellung. Anbieter-neutraler Fork
von Mapbox GL JS, daher gut geeignet für ein Cloud-native-Projekt ohne
Bindung an einen bestimmten Karten-Anbieter. Wayfinder nutzt MapLibre für die
2D-Kartendarstellung; die Kachel-Quelle (z.B. OpenStreetMap) und der
Kartenausschnitt (Mittelpunkt/Zoom) sind über Umgebungsvariablen konfigurierbar.

---

## Zoom / Pan

- **Zoom:** Vergrößerung/Verkleinerung des Kartenausschnitts (z.B. von Heimatflughafen auf ganze Region).
- **Pan:** Verschiebung des Kartenausschnitts (Nutzer schiebt die Karte mit Maus/Touch).

Beide sind Interaktionen, die ein Lotse auf der ASD tätigt — Meilenstein M4.

---

## Flight Level (FL)

Die **Höhe in 100-Fuß-Schritten**, die in der Luftfahrt verwendet wird
(FL100 = 10.000 ft, FL350 = 35.000 ft). Wird von SSR gemessen oder vom
Flugplan vorgegeben.

---

## Rufzeichen / Callsign

Die **Funkrufzeichen** des Flugzeugs (z.B. „LH417" für Lufthansa 417). Wird
in den Transponder programmiert, vom SSR empfangen und ist die Hauptkennung
für einen Track.

---

## Transponder / Squawk-Code

Ein **4-stelliger Oktal-Code** (z.B. 4271), den das Flugzeug sendet. Eindeutig
auf eine bestimmte Zeit und Region, dient der Radar-/ATC-Identifikation.
„Squawken" = den Transponder einschalten/ändern auf ATC-Anweisung.

---

## ADS-B (Automatic Dependent Surveillance – Broadcast)

Eine **moderne Alternativ-/Zusatz-Technologie** zu SSR: Das Flugzeug sendet
regelmäßig seine GPS-Position, Höhe, Geschwindigkeit an alle Empfänger (nicht
nur den Radar). Hohe Genauigkeit, braucht keine Radar-Abfrage. Wird von
modernen Flugzeugen bevorzugt.

---

## ASTERIX / CAT062 · CAT063 · CAT065

ASTERIX (*All-Purpose Structured EUROCONTROL Surveillance Information Exchange*)
ist das binäre Nachrichtenformat, in dem Firefly seine Ergebnisse über
UDP-Multicast aussendet. Wayfinder empfängt **drei Kategorien** auf demselben
Strom (Dispatch am führenden CAT-Oktett; unbekannte Kategorien werden
übersprungen):
- **CAT062** (`0x3E`) — System-Tracks (*was* fliegt).
- **CAT065** (`0x41`) — SDPS-Heartbeat (*lebt* das Datenverarbeitungssystem?);
  unterscheidet „leerer Himmel" von „totem Feed".
- **CAT063** (`0x3F`) — Per-Sensor-Status (*welche* Sensoren speisen das SDPS?);
  Grundlage des gelben „SENSOR AUSFALL"-Chips.

Der maßgebliche, versionierte Vertrag ist `docs/ICD-CAT062.md` im Firefly-Repo.

---

## SDPS (Surveillance Data Processing System)

Das datenverarbeitende Gesamtsystem — hier Fireflys Tracker-Kern —, das die
Meldungen der Einzelsensoren (Radare, ADS-B) zu einem fusionierten Luftlagebild
verarbeitet und aussendet. Sein Lebenszeichen ist der CAT065-Heartbeat. In CAT063
trägt **I063/010** die **SDPS-Identität** (SAC/SIC, Default 25/2 — *wer* meldet),
das separate **I063/050** die **Sensor-Identität** (SAC 0, SIC = `sensor_id` —
*worüber*; seit Fireflys ADR 0032 / ICD 3.0.0). Diese Trennung erlaubt es, einen
einzelnen ausgefallenen Sensor zu erkennen (gelbes Degradierungs-Banner), auch
wenn das SDPS selbst ungestört weiterläuft.

---

## Konflikte / Separation

Wenn sich zwei Tracks zu nah kommen (horizontal oder vertikal), liegt eine
**Separation-Verletzung** vor. ASD-Systeme warnen den Lotsen (z.B. rote
Hervorhebung). Wayfinder kann das in M5 anzeigen.

---

## Trail

Die **Flugbahnhistorie** eines Tracks: Eine Linie aus den letzten N Positionen,
die zeigt, woher das Flugzeug kommt. Hilft dem Lotsen, Manöver zu erkennen
und Intent zu verstehen. M5-Feature.

---

## Cloud-native / 12-Factor

Die Architektur von Wayfinder soll **anbieter-unabhängig und Kubernetes-tauglich**
sein: Konfigurierbarkeit via Env-Vars, sauberes Herunterfahren, strukturierte
Logs, Health-Probes. Ermöglicht Container-Deployment.

---

## Zertifizierungsfähig (ED-153, ED-109A/DO-278A)

Wayfinder wird mit Blick auf **europäische Luftfahrt-Standards** gebaut:
klare Anforderungen → Design → Code → Test (Rückverfolgbarkeit), saubere
Konfigurationsmanagement, Observability. Formal zertifizieren (Safety Case,
SMS) ist ein anderer Prozess.

---

## M1, M2, ... (Meilensteine)

Wayfinder ist in Meilensteine aufgeteilt:
- **M1:** WebSocket-Client + Datenmodell
- **M2:** Statische 2D-Kartendarstellung
- **M3:** Live-Rendering der Tracks
- **M4:** Interaktion
- **M5:** ASD-Elemente (Labels, Trails, Konflikte)

Jeder Meilenstein hat eine Erklärung in `docs/milestones/`.

## Entitlement / Feature-Flag

Ein **Entitlement** ist die als Daten gespeicherte Berechtigung eines Mandanten,
ein bestimmtes Feature zu nutzen (Tabelle `entitlements`, Schlüssel
`feature_key`). Wayfinder behandelt Features so als **Flags als Daten**: ob ein
Mandant z.B. die STCA-Anzeige sieht, entscheidet ein DB-Eintrag, nicht der Code
oder ein Deploy. Der Feature-Service (`pkg/feature`, WF2-50) liest sie
**fail-closed** und **default-deny**: fehlt das Flag oder schlägt der Zugriff
fehl, gilt das Feature als **nicht** freigeschaltet. Bewusst **von Billing
entkoppelt** (ADR 0005 §4) — die Bezahl-Logik (WF2-51) würde nur Entitlements
*setzen*, der ASD-Kern fragt nur `HasFeature(...)`.

## Sensor-Klasse / Sensor-Mix

Ein Feed (Datenstrom) wird durch seinen **Sensor-Mix** beschrieben — die Menge der
**Sensor-Klassen**, aus denen seine Tracks gespeist werden: `PSR` (Primärradar),
`SSR` (Sekundärradar, Mode A/C), `MODE_S`, `ADS-B`, `MLAT` (Multilateration),
`FLARM`. Wayfinder führt dafür ein **kontrolliertes Vokabular** (`pkg/sensorclass`,
WF2-41): beim Anlegen eines Feeds werden gängige Schreibweisen kanonisiert
(„ads-b" → `ADS-B`) und unbekannte Klassen abgewiesen, damit die Feed-Metadaten
verlässlich und auditierbar bleiben. **Wichtig:** Der Sensor-Mix ist ein
**Feed-Metadatum**, kein Per-Track-Tag (ADR 0005 §6.4) — die *track-abgeleitete*
Herkunft am Symbol (WF2-40) ist davon unabhängig.

## Community-Aggregator (Quell-Typ `adsb_aggregator`)

Ein zweiter ADS-B-Bezugsweg **neben** OpenSky (kein Ersatz): Ein Feed kann aus
einem Community-getriebenen ADS-B-Aggregator gespeist werden — **adsb.lol**
(Default) oder **adsb.fi**. Der Typ ist **auth-frei** (kein Credential-Block, kein
`cred_env`) und daher besonders aus Umgebungen mit Datacenter-IP-Sperre nutzbar
(z. B. Codespaces/Azure-IPs, die OpenSky abweist). Ein **gepollter** Quell-Typ:
konfigurierbar über `poll_interval_secs` (5–3600 s, Default 10 s) und `provider`
(Wire-Werte `adsb_lol`/`adsb_fi`, UI-Labels adsb.lol/adsb.fi). Der
CAT062-Draht-Vertrag bleibt davon unberührt (Firefly-Kontrakt v1.5.0, ADR 0031).
Im Admin-UI heißt der Typ **„ADS-B (Community-Aggregator)"**.

## Ausfallgrund einer Quelle (SRC-REASON)

Wenn eine Quelle keine Daten mehr liefert, sagt der Feed-Health-Chip nicht nur
**dass**, sondern auch **warum**. Firefly schickt den Grund im CAT063-Strom mit
(Feld I063/RE), Wayfinder zeigt ihn am Chip: **nicht erreichbar** (`unreachable` —
Netz/Firewall blockiert; die Zugangsdaten sind vermutlich in Ordnung),
**Auth-Fehler** (`auth` — falsche/fehlende Zugangsdaten) oder **Ratenlimit**
(`rate_limited` — die Quelle drosselt die Abfragen). So weiß der Betreiber sofort,
ob ein Nachtippen der Zugangsdaten überhaupt hilft (nur bei `auth`) oder ob das
Problem woanders liegt. Bei mehreren betroffenen Quellen zeigt der Chip den am
direktesten behebbaren Grund zuerst (`auth` vor `rate_limited` vor `unreachable`).

## Range-Ring (Entfernungsring)

Konzentrische Kreise **konstanter Boden-Distanz** (in nautischen Meilen) um einen
Bezugspunkt — auf dem ASD der konfigurierte Karten-Mittelpunkt. Sie geben dem
Lotsen ein **Distanz-Raster** („~25 NM draußen") für Staffelung und Sequencing.
Wayfinder zeichnet sie **geodätisch** (gleiche Distanz in jede Richtung), damit
sie auf der Web-Mercator-Projektion nicht „gequetscht" erscheinen (ASD-012).
**Abzugrenzen** vom *Coverage-Ring* (Paket 6), der die **Reichweite eines
Sensors** zeigt, nicht ein Distanz-Raster.

## Verantwortungsbereich (AoR / Area of Responsibility)

Das **Luftraum-Volumen, das eine ATC-Einheit tatsächlich kontrolliert** und in
dem sie Verkehr staffelt — für einen Flughafen-ANSP typischerweise die **CTR**
(Tower) und die **TMA/CTA** (Approach). Es ist ein **definiertes Gebiet**
(Koordinaten-Polygon mit Vertikalgrenzen und Luftraumklasse, im AIP publiziert),
**kein Radius**. In Wayfinder ist die AoR eine **Darstellungs-Ebene** (Overlay),
**kein** Track-Filter — sie sagt „das kontrolliere *ich*", nicht „diese Tracks
zeige ich" (ADR 0021). Quelle der Polygone aktuell OpenAIP (ADR 0004).

## Area of Interest (AoI) / Track-Scope

Das **Volumen, das das System verfolgt und dem Lotsen anzeigt**. Die AoI
**schließt die AoR ein, reicht aber bewusst darüber hinaus** (Größenordnung
100–300 NM über die AoR-Grenze, ≈ 15–45 min Flugzeit), damit **anfliegender
Verkehr früh** sichtbar ist — der Kernsatz **„sehen ≠ besitzen":** der Lotse
*sieht* mehr, als er *kontrolliert*. In Wayfinder ist die AoI der **Track-Scope**:
technisch die pro-Mandant **`view_configs.AOI`** (BBox) + FL-Band, server-seitig
erzwungen (WF2-21.2), grob vorgelagert durch Fireflys `FIREFLY_COVERAGE_BBOX`
(ADR 0012). **Das ist der „Radius, der nur die Tracks betrifft"** — bewusst
größer als der Verantwortungsbereich (ADR 0021).

## CTR (Control Zone / Kontrollzone)

Kontrollierter Luftraum um einen (oder mehrere) Flughafen, **vom Boden** bis zu
einer festgelegten Obergrenze; lateral mindestens ~5 NM in Anflugrichtung. Wird
i. d. R. vom **Tower** kontrolliert. Im AIP unter **AD 2.17 „ATS airspace"** je
Flughafen publiziert. Für Wayfinder ein typischer Bestandteil des
**Verantwortungsbereichs** (AoR).

## TMA (Terminal Manoeuvring Area / Terminal Control Area)

Kontrollierter Luftraum **über** einer oder mehreren CTRs, am Zusammenlauf der
ATS-Routen; Untergrenze ist ein **Level über Grund** (nicht der Boden). Wird
i. d. R. von **Approach** kontrolliert. Im AIP zentral unter **ENR 2.1**
publiziert (nicht je Flughafen wiederholt). Zweiter typischer Bestandteil der
**AoR** eines Flughafen-ANSP.

## CTA (Control Area / Kontrollbezirk)

Kontrollierter Luftraum ab einem festgelegten Level über Grund aufwärts
(zwischen CTR/TMA und den Luftstraßen bzw. en-route). Je nach Zuschnitt von
Approach oder Area Control (ACC) kontrolliert. Für Wayfinder als AoR-/Kontext-
Overlay relevant.

## ATZ (Aerodrome Traffic Zone)

Kleine Schutzzone unmittelbar am Flughafen (Boden bis wenige tausend Fuß). Anders
als CTR/TMA ist die ATZ vielerorts ein **echter Zylinder** (Radius um den
Bezugspunkt der Piste) — der Sonderfall, in dem ein Radius tatsächlich die
Geometrie definiert.

