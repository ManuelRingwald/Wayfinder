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
