# ADR 0025 — Wayfinder erbringt die Serving-Hälfte der SDPS-Server-Funktion (Verbund mit Firefly)

- **Status:** **AKZEPTIERT** ✅ (2026-07-16, Betreiber-Go). Spiegel-ADR zu
  **Fireflys ADR 0042** (*Arbeitsteilung Firefly + Wayfinder = SDPS-Server-Funktion,
  CAT252-Ersatz*). Dieser ADR trifft **keine** neue Architektur-Entscheidung — er
  schreibt eine seit Monaten **gelebte** Rollenverteilung aus Wayfinder-Sicht
  auffindbar fest, damit der Verbund-Charakter der Server-Funktion auch im
  Wayfinder-ADR-Verzeichnis dokumentiert und auditierbar ist.
- **Datum:** 2026-07-16
- **Schnittstellen-relevant:** **nein.** Der CAT062/063/065-Draht-Vertrag
  (Firefly → Wayfinder, ICD 3.7.0) bleibt byte-identisch. **Rein dokumentarisch:**
  kein Code, keine neuen Env-Variablen, keine geänderten Betriebsmodi. INSTALLATION
  und TECHNICAL sind unberührt (geprüft).
- **Bezug (Wayfinder):** **ADR 0005/0014** (Multi-Mandanten als einziger Modus),
  **ADR 0007** (Cloud-Ingest & Feed-Fan-out: Ingest-Gateway, Bus), **ADR 0012**
  (mandanten-eigene Tracker-Instanzen & Auto-Orchestrierung: 1 Firefly je
  Feed/Mandant), **WF2-21.2** (server-seitiger, fail-closed AOI/FL-View-Filter am
  WS-Rand), **ADR 0021** (AoR vs. AoI/Track-Scope vs. Kartenrahmen), **ADR 0003**
  (Empfangspfad + Browser-Rand als Vertrauensgrenze). **Firefly-seitig:**
  **Fireflys ADR 0042** (diese Entscheidung, Master), **ADR 0006** (CAT062/UDP als
  Ausgabe-Kontrakt, Ports & Adapters), **ADR 0018** (CAT065-Heartbeat =
  Dienst-Status). Referenzrahmen: EUROCONTROL **ARTAS** (Tracker **und** Server)
  mit dem Konsumenten-Protokoll **CAT252**.
- **Anforderungs-Register:** **keine neue Anforderung** — die tragenden
  Anforderungen (Multi-Tenant-Isolation, fail-closed View-Filter, Orchestrierung)
  sind bereits registriert und rückverfolgbar. Dieser ADR bündelt sie nur unter
  der Verbund-Rolle.
- **Auslöser:** Issue **#257** (`from-firefly`) — der Wayfinder-Spiegel, den
  Fireflys ADR 0042 in seinen Konsequenzen anfordert.

> ℹ️ **In normaler Sprache:** Ein vollständiges Luftlage-System nach ARTAS-Vorbild
> hat zwei Hälften — das *Rechenwerk*, das aus Radarmeldungen Tracks macht, und
> den *Server*, der jedem angeschlossenen Nutzer genau seinen Ausschnitt liefert
> (sein Gebiet, seine Höhenschichten, über eine eigene, gesicherte Leitung).
> Firefly ist das Rechenwerk; einen Nutzer-Server hat es bewusst nicht. Diese
> zweite Hälfte macht **Wayfinder** — es verwaltet, wer was sehen darf, filtert
> serverseitig auf das erlaubte Gebiet und liefert das Bild pro Nutzer über eine
> angemeldete Verbindung aus. Das war schon immer so gebaut; hier wird es nur als
> Entscheidung mit Begründung und Grenzen festgehalten.

---

## Kontext

Der ARTAS-**Server** bedient Konsumenten-Systeme individuell: (1)
Subscription-Verwaltung (jeder Nutzer meldet sich an und bestellt einen Dienst),
(2) Zuschnitt je Konsument (Liefergebiet/AOI, Filter, Update-Verhalten pro Abo),
(3) adressierte Punkt-zu-Punkt-Zustellung (klassisch CAT252), (4) Meldung des
Dienst-Status. Firefly erbringt davon **nichts** — es kennt keinen Konsumenten,
sendet ein selbstbeschreibendes ASTERIX-Lagebild (CAT062/063/065) als
Fire-and-Forget-Multicast und hält keinen Empfänger-Zustand.

Bei uns existiert die Server-Leistung dennoch — sie liegt in **Wayfinder**, am
Code geerdet:

| ARTAS-Server-Leistung | Bei uns erbracht durch (Wayfinder) | Beleg |
|-----------------------|------------------------------------|-------|
| Subscription-Verwaltung | Mandanten + Feed-Katalog + Abos (`subscriptions`), Feed-Join zur Laufzeit | ADR 0005/0012 |
| Zuschnitt je Konsument (AOI/FL) | server-seitiger, **fail-closed** View-Filter am WS-Rand (AOI-BBox + FL-Band je Mandant), property-/fuzz-getestet | WF2-21.2, ADR 0021 |
| Zuschnitt je Konsument (Sensor-Mix/Coverage) | Orchestrierung an der Quelle: 1 Firefly-Instanz je Feed mit eigenem `FIREFLY_SOURCES`-Mix + `FIREFLY_COVERAGE_BBOX` | ADR 0012 |
| Adressierte Zustellung | WebSocket = authentifizierte Punkt-zu-Punkt-Verbindung je Client (Browser-Rand, TLS/Auth) | ADR 0003/0014 |
| Zustellung über Netzgrenzen | Ingest-Gateway + Bus transportiert die **Roh-Datagramme** unverändert in die Cloud (Fan-out an N Instanzen) | ADR 0007 |
| Dienst-Status | Firefly-CAT065-SDPS-Heartbeat (I065/040 NOGO) an alle — im Strom statt je User | Fireflys ADR 0018 |

---

## Entscheidung

### 1. Die SDPS-Server-Funktion ist eine Verbund-Leistung aus Firefly + Wayfinder

Das Gesamtsystem „Firefly + Wayfinder" erbringt die ARTAS-Server-Funktion
**arbeitsteilig**, mit harter Rollengrenze:

- **Firefly = Erzeugung + Verteilung an alle.** Ein Lagebild je Instanz als
  ASTERIX-Multicast (CAT062/063/065), fire and forget; keine Konsumenten-
  Verwaltung, kein Empfänger-Zustand. Der native Multicast-Fanout *ist* die
  Verteil-Schicht (N Konsumenten kosten den Tracker nichts).
- **Wayfinder = Konsumenten-Verwaltung + Zuschnitt.** Anmeldung, Berechtigung,
  Abo (Mandant ↔ Feeds), Liefergebiet (AOI-BBox + FL-Band, server-seitig,
  fail-closed) und adressierte Zustellung (authentifizierter WebSocket je Client)
  — **außerhalb** des Track-Rechenpfads. Der Sensor-Mix je Konsument entsteht **an
  der Quelle** (eine dedizierte Firefly-Instanz je Feed, ADR 0012); Firefly bleibt
  mandanten-blind.

### 2. Konsumenten-Matrix K1–K5 (Master: Fireflys ADR 0042)

Welcher Konsument bekommt den Strom **wie**? Adressierte Dienste sind **Optionen**,
kein Pflichtausbau:

| # | Konsument | Anschlussweg | Status |
|---|-----------|--------------|--------|
| K1 | ASTERIX-fähiges System **im Multicast-Segment** (weiteres ASD, Recorder) | Direkt der Gruppe beitreten — die ICD ist der vollständige Vertrag; Staleness via CAT065 | ✅ heute |
| K2 | ASTERIX-fähiges System **ohne Multicast-Zugang** (Cloud) | Ingest-Gateway/Bus (ADR 0007): unveränderte Roh-Datagramme Punkt-zu-Punkt | ✅ Architektur steht |
| K3 | System, das einen **zugeschnittenen** Dienst braucht (nur mein Gebiet/FL, Web) | **Wayfinder als Serving-Punkt**: Mandant + View-Config + WS-Abo (WF2-21.2) — Wayfinders **JSON-Vertrag**, kein ASTERIX | ✅ heute |
| K4 | Konsument mit eigenem **Sensor-Mix-/Coverage-Vertrag** | Eigene Firefly-Instanz je Feed (ADR 0012) — der adressierte Dienst entsteht an der Quelle | ✅ Architektur steht |
| K5 | Legacy-System, das **nur CAT252** spricht | **Nicht bedienbar** — bewusst; siehe Punkt 3 | ❌ bewusst nicht gebaut |

### 3. CAT252 wird nicht implementiert

Ein CAT252-artiger Subscription-Server ist verworfen — nicht aufgeschoben:
Konsumenten-Zustand (Sessions, Reconnects, Abo-Mutationen) im Tracker koppelte die
Lagebild-Erzeugung an das Verhalten einzelner Empfänger — genau die Kopplung, die
Fireflys ADR 0006 („Entkopplung über Datenstrom") ausschließt; ohne Replay-/
Fanout-Gewinn (Multicast skaliert im Netz) und ohne Bedarf im Zielbild (Wayfinder
+ K1–K4 kommen ohne aus). **Falls** ein künftiger Konsument CAT252 vertraglich
erzwingt, entsteht ein **Protokoll-Adapter am Rand** (eigener Dienst, der als
gewöhnlicher K1/K2-Konsument den Multicast liest und CAT252 nach außen spricht) —
per neuem ADR, ohne Änderung am Tracker-Kern oder an Wayfinders Serving-Schicht.

---

## Begründung

- **Die Leistung existiert — nur der Nachweis fehlte.** Jede Zeile der
  Server-Leistungstabelle ist mit gelebtem Wayfinder-Code/ADR belegt; dieser ADR
  macht Wayfinders Anteil an der Verbund-Rolle explizit und auditierbar.
- **Sicherheitsargument:** Der Track-Rechenpfad bleibt frei von Empfänger-Zustand.
  Ausfall/Überlast/Fehlverhalten eines Konsumenten kann die Lagebild-Erzeugung
  **strukturell nicht** beeinflussen (Fire-and-Forget-Multicast statt Session-Server).
- **Zuschnitt lebt genau einmal:** der operative AOI/FL-Filter sitzt fail-closed in
  Wayfinder (WF2-21.2), getrennt von der quell-seitigen Coverage-Begrenzung
  (ADR 0012, Rechenlast an der Quelle) — keine zweite Filter-Wahrheit.

### Verworfene Alternativen

- **Eigenständiger Track-Server-Dienst zwischen Firefly und Konsumenten** (dritte
  Komponente, die abonniert/filtert/adressiert): reproduziert Wayfinders
  Serving-Schicht als Parallelbau und schüfe eine zweite Filter-Wahrheit.
  Verworfen — **Wayfinder *ist* dieser Dienst.**
- **CAT252-Server in Firefly / Per-Konsument-Unicast aus Firefly:** zieht
  Empfänger-Zustand in den kritischen Pfad, ohne Gewinn. Verworfen (siehe Punkt 3).

---

## Konsequenzen

- **Rein dokumentarisch:** dieser ADR + der CLAUDE.md-§1-Verweis auf die
  Verbund-Rolle. Kein Code, keine Env-Variablen, kein Wire-/ICD-Bezug.
- **Leitplanke für die Zukunft:** Wer einen neuen Konsumenten anbindet, ordnet ihn
  zuerst in die Matrix K1–K5 ein. Ein Vorschlag, der auf „Firefly merkt sich
  Empfänger" oder „ein zweiter Serving-/Filter-Dienst neben Wayfinder" hinausläuft,
  widerspricht diesem ADR (und Fireflys ADR 0042) und braucht deren explizite
  Ablösung.
- **Laufzeit-Steuerung bleibt eigenes Vorhaben:** Fireflys SRV.2 (Sensor an/aus,
  Service-Kommandos via API) entsteht getrennt; Wayfinder zieht dort ggf. mit einem
  eigenen Häppchen nach — außerhalb dieses ADR.

## Ehrliche Grenzen

- **Kein ARTAS-kompatibler CAT252-Endpunkt.** Ein Bestands-System, das
  ausschließlich CAT252 spricht, kann heute **nicht** andocken (K5). Der Ausweg
  (Adapter am Rand) ist ein Konzept, kein Code.
- **Zugeschnittene Dienste liefern Wayfinders JSON, kein gefiltertes ASTERIX.** Wer
  „gefiltertes CAT062" braucht, fällt auf K1/K2 (ungefiltert) oder einen künftigen
  Adapter zurück.
- **Dienst-Status ist Broadcast, nicht Abo-Quittung.** CAT065 sagt „Dienst
  lebt/degradiert" an alle; eine per-User-Bestätigung existiert nur am
  Wayfinder-WS-Rand, nicht im Strom.
