# AMAN-Marktanalyse — Orientierung für eine Wayfinder-AMAN-Komponente

> **Zweck:** Der Betreiber hat entschieden, die Wayfinder-Suite um einen
> **AMAN** (*Arrival Manager*) zu erweitern. Dieses Dokument ist die
> vorbereitende **Markt- und Funktionsuntersuchung** (2026-07-22): Welche
> Top-AMAN-Produkte gibt es, was können sie, wie sieht das Lotsen-Handling
> aus — als Orientierung für Funktionsumfang und Bedienkonzept unserer
> eigenen Komponente. Die **Integrations-Entscheidung** (wie AMAN in
> ASD/Wayfinder einzieht) ist bewusst **nicht** Teil dieses Dokuments;
> sie folgt als eigener Abstimmungs-Schritt (ADR).

> **Methodik & Belegqualität:** Deep-Research-Lauf über 5 Suchwinkel,
> 20 Quellen, 29 extrahierte Einzelaussagen; die Kernaussagen wurden
> adversarial gegengeprüft (3 unabhängige Prüfer je Aussage). Ein Teil der
> Gegenprüfungen ist einem Sitzungs-Limit zum Opfer gefallen — deshalb
> trägt jede Aussage hier eine ehrliche Beleg-Stufe:
>
> | Marker | Bedeutung |
> |--------|-----------|
> | **[V]** | Verifiziert (3-0-Gegenprüfung, Primärquelle mit Zitat) |
> | **[Q]** | Quellen-belegt (wörtliches Zitat liegt vor, Gegenprüfung ausgefallen) |
> | **[S]** | Such-/Sekundärbefund (Snippet-Ebene, nicht tief gelesen) |
> | **[H]** | Hintergrundwissen des Assistenten (vor Verwendung in ADRs nachprüfen) |

---

## 1. Kurzfassung (in normaler Sprache)

Ein **AMAN** ist das Planungswerkzeug für den **Anflugverkehr**: Er
beobachtet alle Anflüge auf einen Flughafen lange bevor sie im Endanflug
sind, berechnet für jeden Flug eine **Soll-Landezeit** und eine stabile
**Landereihenfolge**, und sagt den Lotsen frühzeitig, **wie viel Zeit ein
Flug verlieren oder gewinnen muss**, damit die Reihenfolge ohne Warteschleifen
und hektisches Vectoring aufgeht. Sichtbar wird das klassisch als
**Timeline** („Zeitleiter") neben dem Radarbild plus **Hinweise (Advisories)**
am oder neben dem Track-Label.

Der Weltmarkt wird von wenigen Systemen geprägt:

- **MAESTRO** (Thales) — der De-facto-Standard in Europa/Asien/Australien,
  integrierter AMAN/DMAN. **[V]**
- **TBFM + TSAS** (Leidos/FAA) — das US-Gegenstück, landesweites
  Time-Based-Metering-Rückgrat. **[V]**
- **OSYRIS-/Orthogon-Linie** (Barco-Orthogon → heute Frequentis-Orthogon)
  — die deutsche/nordeuropäische Produktlinie (u.a. DFS-Umfeld). **[V/Q]**
- **Intelligent Approach** (NATS/Leidos) — Spezialist für den Endanflug
  (zeitbasierte Staffelung), kein Voll-AMAN. **[S]**
- **DLR 4D-CARMA** — das wichtigste frei publizierte Forschungs-AMAN
  (Algorithmen-Referenz). **[S/Q]**

Der gemeinsame Funktionskern aller Top-Systeme ist überall gleich
(Abschnitt 4): **ETA-Prädiktion → Scheduler (Sequenz + Sollzeiten) →
Horizont-Modell (aktiv/eingefroren) → Advisories (time to lose/gain,
Speed) → Timeline-HMI mit manuellen Eingriffen.** In Europa ist die
Ausdehnung des Planungshorizonts (**Extended AMAN**, bis 180–200 NM)
für die größten Flughäfen inzwischen **regulatorisch vorgeschrieben**. **[S]**

---

## 2. Normative Grundlagen (was „AMAN" offiziell heißt)

| Dokument | Inhalt / Relevanz | Beleg |
|----------|-------------------|-------|
| **EUROCONTROL — Arrival Manager: Implementation Guidelines / AMAN Status Review** (SKYbrary Bookshelf, 2010) | *Das* Grundlagendokument: Betriebskonzept (Sequence Planner in der TMA, AMAN-Anzeige an jeder E-TMA/TMA-Position), Referenz-Funktionsliste (Sequenz-/Metering-Berechnung, Delay-Absorption, Advisories, Horizonte), Lessons Learned der AMAN-Pioniere. | [S] |
| **SESAR Solution #05 — Extended AMAN (TS AMAN)** | Technische Spezifikation: Anforderungen an Sequenzierung, Trajektorien-Prädiktion, Advisory-Erzeugung (time-to-lose/gain), Verteilung an Upstream-Sektoren über **SWIM**-Services. Direkt als Anforderungsbasis verwertbar. | [S] |
| **SESAR JU — E-AMAN Solution** | Betriebskonzept Extended Horizon: AMAN-Horizont von TMA-Nähe auf bis zu **200 NM** upstream ausgedehnt; Delay-Absorption per Speed Advisories im En-route-Segment statt Holding. | [S] |
| **EU Common Project One (CP1)** (SESAR Deployment Manager) | **E-AMAN ist Pflicht**: innerhalb 180 NM für die 20 verkehrsreichsten EU-Flughäfen (Zieltermine Ende 2024 Legacy, Ende 2025 via SWIM), inkl. Cross-Border-/XMAN-Dimension. Maßstab für Zukunftsfähigkeit. | [S] |
| **FAA Order JO 7210.3, Kap. 18 §25** | Normatives US-Betriebsdokument zu TBFM (Details in Abschnitt 3.2). | [V] |
| **ICAO ASBU-Modul B0-RSEQ** („Improved Traffic Flow through Runway Sequencing, AMAN/DMAN") | Globale ICAO-Einordnung von AMAN/DMAN als Basis-Baustein der ATM-Modernisierung. | [H] |

---

## 3. Die Top-Produkte im Steckbrief

### 3.1 Thales **MAESTRO** (TopSky-ATC/Maestro)

- **Hersteller/Herkunft:** Thales; ursprünglich aus dem französischen
  Umfeld (Egis Avia / DSNA), von Thales übernommen. [S]
- **Positionierung:** Voll **integrierte AMAN/DMAN-Lösung** — Arrival-,
  Departure- und Runway-Management in einem System. **[V]**
  („The MAESTRO ATM system offers a fully integrated AMAN/DMAN solution
  for automated support of arrival, departure and runway management.")
- **Marktstellung:** Thales bezeichnet Maestro als das **meistgenutzte
  Sequencing-/Metering-System der Welt** — Hersteller-Superlativ, deckt
  sich aber mit dem breiten Referenz-Fußabdruck. **[V]** (als
  Hersteller-Aussage verifiziert)
- **Referenz-Deployments:** **28 Flughäfen weltweit** (Zahl von der
  aktuellen Thales-Seite, taucht aber schon 2014 auf — möglicherweise
  veraltet) [Q]; Australien-Rollout Sydney → Melbourne → Brisbane →
  **Perth (live 3. April 2014**, Betreiber Airservices Australia,
  Implementierung in 7 Monaten) [Q]; erstes kombiniertes AMAN+DMAN-
  Deployment in China (Beijing TMA) [Q].
- **Funktionskern (Thales-Formulierung):** „sequence and balance the
  traffic flow of the airport and airspace" — Sequenzberechnung **plus
  Runway-/Flow-Balancing**, Ziel Delay-Reduktion + Kapazität. [Q]
- **Handling-Merkmal:** **Alle Arbeitspositionen (ACC, Approach, Tower)
  teilen dieselbe Timeline** — eine gemeinsame Sequenz-Sicht, inkl.
  kollaborativem Handling von Runway-Konfigurationswechseln. [S]
- **Stabilitäts-Modell:** Vierstufig **UNSTABLE / STABLE / SUPERSTABLE /
  FROZEN** je Flug (aus Community-Doku eines MAESTRO-Trainings-Wikis —
  niedrige Quellen-Autorität, aber konsistent mit dem Horizont-Modell
  anderer Systeme). [S]

### 3.2 Leidos/FAA **TBFM** + **TSAS** (USA)

- **Rolle:** TBFM ist laut FAA Order JO 7210.3 das **grundlegende
  Decision Support Tool** der FAA für zeitbasiertes Verkehrsmanagement
  (En-route **und** Terminal). **[V]**
- **Kernfunktion:** Terminierung von Flügen auf definierte
  **Constraint-Punkte (Meter Fix / Meter Arc)** zu festgelegten Zeiten →
  eine zeitlich geordnete Sequenz. **[V]**
- **Warum zeitbasiert:** Die Sollzeiten (STAs) führen Verkehrsströme
  zusammen bei **minimalem Koordinationsaufwand, weniger
  Vectoring/Holding** und effizienter Kapazitätsnutzung — die
  Kernbegründung für Time-Based Metering. **[V]**
- **Freeze-Mechanik:** Passiert die ETA eines Flugs den voreingestellten
  **Freeze Horizon** seiner *Stream Class*, wird die STA **eingefroren**
  (keine automatische Neuberechnung mehr) — der Lotse kann die Verspätung
  kontrolliert abbauen. Freeze Horizon für Jets typisch **150–350 NM**
  vor dem Meter Fix, je Konfiguration/Stream Class. [Q]
- **Schedule-Inputs:** aktuelle **ETA je Constraint-Punkt** aus
  **Windvorhersage + Flugplan + gewünschtem Intervall** am Punkt —
  Trajektorien-Prädiktion aus Surveillance-, Flugplan- und Wetterdaten. [Q]
- **TSAS:** erweitert Scheduling/Metering **in den Terminal-Luftraum**
  und gibt Terminal-Lotsen eigene Metering-Werkzeuge. **[V]**
  HMI-seitig (NASA-Entwicklung, an die FAA transferiert): **Slot Markers**
  (Soll-Positions-Kreise) und **Speed Advisories direkt auf dem
  Radardisplay** des Terminal-Lotsen, damit STAs entlang RNAV-Arrivals
  ohne Downwind-Verlängerung getroffen werden. [S]
- **DMAN-Seite:** Kopplung im US-Kontext über **TFDM** (Terminal Flight
  Data Manager). [S]

### 3.3 **OSYRIS-/Orthogon-Linie** (Barco-Orthogon → Harris → Frequentis-Orthogon)

- **Bedeutung:** Die in Deutschland/Nordeuropa verbreitete AMAN-Produktlinie
  (Orthogon GmbH, Bremen; über Barco und Harris zu Frequentis gewandert). [H]
  NATS' Extended-AMAN-Programm lief auf Orthogon-Technik. [H]
- **Horizont-Modell (verifiziert am EUROCONTROL-Experiment):** Zwei
  Zeithorizonte — **aktiver Horizont ~40 min vor Touchdown**
  (automatische Sequenzierung + Advisory-Berechnung) und **eingefrorener
  Horizont ~20 min vor Touchdown** (keine Umsequenzierung mehr, Sequenz
  gilt als stabil). **[V]**
- **Timeline-Inhalt (CoSpace-Experiment, EUROCONTROL Experimental
  Centre):** beidseitig der Zeitachse je Flug **IAF-Kennung, Callsign,
  Flugzeugtyp, Wirbelschleppen-Kategorie, time-to-lose/gain**; Ansicht
  referenziert auf die **Runway-Schwelle**. [Q]
- **Advisory-Muster:** **„time to lose" / „time to gain"** wird den
  Lotsen **stromaufwärts (E-TMA)** angezeigt — das Kern-Interaktionsmuster
  für Delay-Absorption vor dem TMA-Eintritt. [Q]
- **Lotsen-Interaktionen (aus derselben Studie):** Position in der
  Sequenz ändern, Reihenfolge zweier Flüge **erzwingen (force)**, Sequenz
  **„packen"** (Lücken reduzieren). [S]
- **DFS-Umfeld:** DFS Aviation Services vertreibt einen eigenen
  AMAN (Produkt-Flyer 2020, operativ an deutschen Flughäfen) —
  Feature-Katalog: Sequenzberechnung, Timeline-HMI, Systemintegration. [S]

### 3.4 NATS/Leidos **Intelligent Approach** (Endanflug-Nische)

- **Was es ist:** Time-Based-Separation-Werkzeug für den **Endanflug**:
  berechnet das optimale **Zeitintervall zwischen Anflügen aus Live-Wind
  und Flugzeugtyp** und zeigt es als **Spacing-/Separation-Marker direkt
  im Radarbild** zwischen den Anflügen. [S]
- **Deployments:** Heathrow (2015), Gatwick, Schiphol, Toronto Pearson. [S]
- **Einordnung:** **Kein Voll-AMAN**, sondern das letzte Glied der
  Anflug-Kette — aber das wichtigste Referenz-HMI-Muster dafür, wie
  Advisory-Grafik **direkt ins ASD** integriert wird (statt in ein
  Nebenfenster).

### 3.5 **DLR 4D-CARMA** (+ SINOPTICA) — Forschungsreferenz

- **ICAS-2010-Papier:** Sequenzbildung, Zuweisung von **Landezeiten und
  Runways**, Projektion horizontaler/vertikaler **4D-Anflugtrajektorien** —
  eine der wenigen frei publizierten Quellen mit Algorithmus-Details eines
  vollständigen AMAN. [S]
- **SINOPTICA (H2020, Aerospace 2023):** 4D-CARMA zum **Extended AMAN**
  erweitert (Flughafen Mailand-Malpensa): **Unwetter-Nowcasting als
  Constraint direkt in der Sequenzplanung**, eigenes Modul für
  **4D-Ausweichtrajektorien** bei Gewitterzellen, **zwei
  Darstellungsvarianten im Primär-Display (ASD)** des Lotsen; Nachweis
  per Schnellzeit-/Echtzeit-Simulation → Reifegrad Forschung, nicht
  operativ. [Q]

### 3.6 Übrige Kandidaten (ehrliche Korrektur der Erwartungsliste)

- **Saab:** breites ATM-Portfolio (100+ Standorte, 45+ Länder), aber
  **kein prominent vermarktetes Standalone-AMAN-Produkt** auffindbar. [S]
- **Indra / Leonardo / Adacel / L3Harris:** In diesem Recherche-Lauf
  **keine substanziellen AMAN-Produktbelege** gefunden; Arrival-Funktionen
  stecken dort typischerweise in den ATM-Suiten (z.B. iTEC-Umfeld) statt
  als eigenständiges Produkt. Lücke ist dokumentiert — bei Bedarf gezielt
  nachrecherchieren. [S/H]

---

## 4. Funktions-Kanon: Was einen „Top-AMAN" ausmacht

Destillat über alle untersuchten Systeme — das ist die Checkliste, an der
sich der Funktionsumfang einer Wayfinder-AMAN-Komponente messen lässt:

| # | Funktion | Kern | Beleg-Anker |
|---|----------|------|-------------|
| F1 | **Trajektorien-Prädiktion / ETA** | ETA je Flug an Referenzpunkten (Meter Fix, IAF, Threshold) aus Flugplan/Route + Wind + aktueller Track-Lage | TBFM [Q], 4D-CARMA [S] |
| F2 | **Sequenz + Sollzeiten (Scheduler)** | Landereihenfolge + STA je Flug; Ziel: Durchsatz + Stabilität + Fairness | TBFM [V], MAESTRO [V/Q] |
| F3 | **Horizont-Modell** | Aktiver Horizont (Neuberechnung erlaubt) → **Freeze** (Sequenz stabil, keine Umplanung) | OSYRIS 40/20 min [V], TBFM Freeze Horizon 150–350 NM [Q] |
| F4 | **Delay-Advisories** | „time to lose / time to gain" je Flug, an Upstream-Positionen verteilt | CoSpace/E-TMA [Q], SESAR TS [S] |
| F5 | **Speed Advisories / zeitbasierte Staffelung** | konkrete Geschwindigkeits-Hinweise (TSAS) bzw. dynamische Spacing-Marker im Endanflug (Intelligent Approach) | TSAS [S], IA [S] |
| F6 | **Runway-Zuweisung / Balancing** | Verteilung auf Bahnen, Balancing der Flows Flughafen ↔ Luftraum | MAESTRO [Q], 4D-CARMA [S] |
| F7 | **Constraint-Handling** | Wirbelschleppen-Matrix, Kapazität, Runway-Konfiguration, Wetter (bis hin zu Nowcasting-Integration) | CPS-Modell [Q], SINOPTICA [Q] |
| F8 | **Manuelle Eingriffe des Lotsen** | Umsequenzieren, Reihenfolge erzwingen (force), Sequenz packen, Runway ändern — der Lotse bleibt Entscheider, AMAN ist **Advisory-System** | CoSpace [S] |
| F9 | **Extended Horizon (E-AMAN/XMAN)** | Horizont 180–200 NM, Advisory-Verteilung an Upstream-/Nachbar-Sektoren via **SWIM**; in der EU (CP1) Pflicht für Top-20-Flughäfen | SESAR/CP1 [S] |
| F10 | **AMAN/DMAN-Kopplung** | integrierte An-/Abflug-Muster für dieselbe/abhängige Runways (Mixed-Mode); belegte Effekte u.a. −9 % Taxi-Zeiten, bis 7 % Taxi-out-Fuel, +7,8 % Off-Block-Prädiktabilität | MAESTRO [V], SESAR [S], TFDM [S] |
| F11 | **A-CDM-Anbindung** | Abgleich mit Airport-CDM-Meilensteinen/Slots | SESAR [S/H] |
| F12 | **Konfigurierbare Metering-Punkte** | Referenzpunkt der Planung wählbar (Threshold vs. Meter Fix vs. IAF) — Systeme unterscheiden sich hier | TBFM Meter Fix [V] vs. OSYRIS Threshold-Ansicht [Q] |

**Ein „Basis-AMAN" ist F1–F4 + F8 + Timeline-HMI.** F5–F7 sind die
Ausbaustufe „operativ vollwertig", F9–F11 die Verbund-/Netzwerk-Stufe.

---

## 5. HMI- und Handling-Muster (Dimension „Bedienung")

1. **Timeline/Ladder als Leitmetapher.** Alle Systeme zeigen die Sequenz
   als vertikale Zeitleiter mit den Flügen als Einträgen; die Zeitachse
   referenziert je nach System die **Runway-Schwelle** (OSYRIS-Ansicht [Q])
   oder **Meter Fixes** (TBFM [V]). Mehrere Bahnen = mehrere Spalten/Leitern.
2. **Label-Inhalt der Timeline-Einträge** (Beispiel CoSpace [Q]):
   IAF · Callsign · Typ · Wake-Kategorie · **time-to-lose/gain**. Das ist
   eine direkt übernehmbare Vorlage für den Informationsgehalt.
3. **Zustands-Kodierung ist produkt­spezifisch, nicht standardisiert.**
   Die adversariale Prüfung hat ausdrücklich **widerlegt**, dass es eine
   produktübergreifende Farb-Konvention gäbe (z.B. „grau = forced/frozen" —
   das ist eine OSYRIS-/Experiment-spezifische Legende). MAESTRO nutzt
   ein eigenes vierstufiges Stabilitätsmodell, TBFM eigene
   T-GUI/P-GUI-Konventionen. → **Gestaltungsfreiheit für Wayfinder**, aber
   die *Kategorien* (instabil/stabil/eingefroren/manuell fixiert) sind
   überall dieselben. **[V-Refutation]**
4. **Advisories gehören dorthin, wo der Lotse hinschaut.** Zwei bewährte
   Muster, die sich ergänzen:
   - **Timeline + Advisory-Spalte** (klassisch, OSYRIS/MAESTRO): TTL/TTG
     neben dem Timeline-Eintrag; Upstream-Positionen sehen „ihre" Flüge. [Q]
   - **Advisory im Radarbild selbst** (moderner Trend): TSAS-**Slot-Marker**
     (Soll-Positions-Kreise) + Speed Advisories im Terminal-Radardisplay [S];
     Intelligent-Approach-**Spacing-Marker** zwischen den Anflügen im ASD [S];
     SINOPTICA integrierte auch Wetter-/Umleitungs-Darstellung ins
     Primär-Display [Q].
5. **Interaktionsmuster des Lotsen:** manuelles Umsequenzieren
   (Position verschieben), **Force** (Reihenfolge zweier Flüge festschreiben),
   **Pack** (Lücken komprimieren), Runway-Änderung, ggf. manueller Freeze.
   Jeder Eingriff macht den Flug für die Automatik „gepinnt". [S]
6. **Eine gemeinsame Sequenz-Sicht für alle Rollen** (MAESTRO-Muster):
   ACC-, Approach- und Tower-Positionen arbeiten auf derselben Timeline —
   wichtig für Konsistenz, auch im Mandanten-Kontext von Wayfinder. [S]

---

## 6. Kern-Algorithmik (Was im Scheduler steckt)

- **Baseline FCFS** (First-Come-First-Served nach ETA) — fair, aber
  durchsatz-suboptimal.
- **Constrained Position Shifting (CPS)** [Q]: Optimierung der
  Landereihenfolge, wobei jeder Flug höchstens *k* Positionen von seinem
  FCFS-Platz abweichen darf (Fairness + operationelle Stabilität).
  Referenz: Balakrishnan/Chandran (AIAA 2006) — **Dynamic-Programming-
  Ansatz, linear in der Anzahl Flüge**, prototypisch auf realen
  Denver-Daten erprobt → **echtzeitfähig**. [Q]
- **Constraint-Klassen des Optimierungsmodells** [Q]: Mindest-Staffelung
  (Wirbelschleppen-/Runway-Occupancy-Matrix je Typ-Paar),
  Präzedenz-Beziehungen (kein Überholen, Airline-/ATC-Vorgaben),
  Zeitfenster je Flug. Zielfunktion: Durchsatz maximieren (Makespan
  minimieren) — in der Praxis abgewogen gegen Verspätungs-Fairness.
- **Zwei-Horizont-Stabilisierung** (siehe F3): ereignisgetriebene
  Neuberechnung bei jedem Track-/Plan-Update **nur** im aktiven Horizont;
  im Freeze-Bereich bleibt die STA stehen. **[V]**
- **ETA-/Trajektorien-Prädiktion** als eigenständiges Modul: Route +
  Geschwindigkeitsprofil + Wind; TBFM rechnet ETAs an Constraint-Punkten
  aus Windvorhersage + Flugplan. [Q] 4D-CARMA projiziert volle horizontale
  + vertikale 4D-Trajektorien. [S]

---

## 7. Architektur- und Integrationsmuster

Wiederkehrender Baukasten aller untersuchten Systeme:

```
Surveillance (Tracks) ─┐
Flugplan-/Routendaten ─┤   ┌─────────────┐   ┌───────────┐   ┌──────────────────┐
Wind/Met ──────────────┼──▶│ Trajektorien-│──▶│ Scheduler │──▶│ Advisory-Erzeugung│
Runway-Konfig/Kapazität┘   │ Prädiktion   │   │ (Seq+STA) │   │ (TTL/TTG, Speed)  │
                           └─────────────┘   └───────────┘   └────────┬─────────┘
                                                                      ▼
                                              Timeline-HMI (alle Rollen) + ASD-Overlay
                                              + Verteilung an Upstream-CWPs (E-AMAN, SWIM)
```

- **Datenquellen:** Surveillance-Tracks, Flugplandaten (Route, ADEP/ADES,
  Typ, Wake), Wind/Met, Runway-Konfiguration/Kapazität. (TBFM-Beleg [Q];
  generisch [S/H].)
- **Schnittstellen-Standards:** Track-Eingang klassisch ASTERIX;
  E-AMAN-Advisory-Verteilung an andere Zentren/Sektoren per
  **SWIM-Services** (CP1-Zieltermin Ende 2025) [S]; Flugplandaten im
  SWIM-Umfeld als FIXM [H].
- **Kopplung:** DMAN (integrierte Runway-Nutzung, Mixed-Mode) [V/S],
  A-CDM (Meilensteine) [S/H], im US-Kontext TFDM [S].
- **Wetter als Constraint** ist die aktuelle Forschungs-Frontlinie
  (SINOPTICA: Nowcasting → Sequenzplanung + 4D-Ausweichtrajektorien) —
  operativ noch nicht Standard. [Q]

---

## 8. Erste Ableitungen für Wayfinder (Diskussionsgrundlage, keine Entscheidung)

**Was die Suite heute schon mitbringt:**

- Live-System-Tracks mit Position, Geschwindigkeit, Höhe + Steig-/Sinkrate
  (CAT062: I062/105/185/135/220), Callsign (I062/245) und — entscheidend —
  **Flugplan-Grunddaten ADEP/ADES + Plan-Callsign (I062/390)** für
  korrelierte Tracks: die Zutat, um Anflüge auf einen Zielflughafen
  überhaupt zu erkennen.
- Mandanten-gescopte WebSocket-Verteilung, AOI-Zuschnitt, ein
  Vue/MapLibre-Frontend, in das eine Timeline-Komponente und
  ASD-Overlays passen.

**Was für einen echten AMAN fehlt (Kandidaten für den Zuschnitt):**

1. **ETA-Engine** (F1): Prädiktion auf einen Referenzpunkt (pragmatisch
   zuerst: Runway-Schwelle/IAF des Ziel-Flughafens) aus Track-Lage +
   einfachem Performance-Modell; Wind später.
2. **Scheduler** (F2/F3): FCFS + CPS mit Wake-Staffelungsmatrix,
   Zwei-Horizont-Modell (aktiv/frozen — die 40/20-min-Werte von OSYRIS
   sind ein guter Startpunkt).
3. **Timeline-HMI** (F8 + Abschnitt 5): Zeitleiter je Runway,
   Label-Inhalt nach CoSpace-Vorlage, TTL/TTG-Advisory im Track-Label
   und/oder Detail-Panel; manuelle Eingriffe (Umsequenzieren/Force) als
   spätere Stufe.
4. **Flughafen-/Runway-Modell**: Stammdaten (Runways, Richtungen,
   Kapazität) — heute nirgends in der Suite vorhanden.

**Offene Grundsatzfragen für die Integrations-Diskussion:**

- **Wo lebt der AMAN?** Eigener Dienst neben Wayfinder (konsumiert
  denselben CAT062-Multicast — sauber nach ARTAS-Denkmodell) vs. Modul im
  Wayfinder-Backend (kurzer Weg zur bestehenden WS-/Mandanten-Infrastruktur).
- **Braucht er mehr Daten von Firefly?** (z.B. angereicherte
  Flugplan-/Routendaten über I062/390 hinaus, Wind) → wäre ein
  ICD-Thema mit `from-wayfinder`-Issue.
- **Mandanten-Zuschnitt:** AMAN je Mandant/Feed oder global je Flughafen?
- **Anzeige-Ambition v1:** nur Timeline + TTL/TTG (Basis-AMAN) oder
  gleich Advisory-Grafik im ASD (Slot-Marker-Muster)?

---

## 9. Quellen

| Quelle | Typ/Qualität |
|--------|--------------|
| Thales — TopSky-ATC/Maestro Produktseite | Primär (Hersteller) |
| FAA Order JO 7210.3 Kap. 18 §25 (TBFM) | Primär (normativ) |
| EUROCONTROL — AMAN Implementation Guidelines / Status Review (SKYbrary Bookshelf 2416) | Primär (normativ), Snippet-Ebene |
| SESAR Solution #05 Extended AMAN TS; SESAR-JU-Lösungsseiten (E-AMAN, AMAN/DMAN); SESAR Deployment Manager (CP1) | Primär/Programm, Snippet-Ebene |
| EUROCONTROL-EEC CoSpace-Studie (AMAN-Timeline-Figur, OSYRIS/Barco-Orthogon; via ResearchGate) | Primär (Forschung, ~2005) |
| Balakrishnan/Chandran — Scheduling Aircraft Landings under CPS (AIAA 2006) | Primär (Forschung) |
| SINOPTICA — Severe-Weather-Integration in einen E-AMAN (Aerospace 2023, DOI 10.3390/aerospace10030210) | Primär (Forschung) |
| DLR 4D-CARMA (ICAS 2010, Paper 628) | Primär (Forschung), Snippet-Ebene |
| NATS + Leidos — Intelligent-Approach-Produktseiten | Primär (Hersteller), Snippet-Ebene |
| FAA TSAS-Storyboard; NASA-NTRS-TSAS-Papier; NATCA-TBFM-Artikel | Sekundär, Snippet-Ebene |
| ATC-Network — „Maestro AMAN goes live in Perth" (2014) | Sekundär (Pressemitteilungs-Wiedergabe) |
| DFS Aviation Services — AMAN-Produkt-Flyer (2020) | Primär (Hersteller), Snippet-Ebene |
| Saab — ATM-Portfolio-Seite | Primär (Hersteller), Snippet-Ebene |

**Recherche-Statistik:** 5 Suchwinkel · 20 Quellen · 29 extrahierte
Aussagen · 6 dreifach verifiziert · 1 als Verallgemeinerung widerlegt
(und hier korrigiert eingearbeitet) · 18 quellen-belegt ohne
abgeschlossene Gegenprüfung (Sitzungs-Limit). Aussagen mit [H] sind
Assistenten-Hintergrundwissen und vor Übernahme in einen ADR nachzuprüfen.
