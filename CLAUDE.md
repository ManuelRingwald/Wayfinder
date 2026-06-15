# Wayfinder — Projekt-Charter & Arbeitsregeln

> Dieses Dokument ist die **verbindliche Arbeitsvereinbarung** zwischen dem
> Projektverantwortlichen (Mensch) und dem KI-Assistenten (Claude). Claude liest
> diese Datei zu Beginn jeder Sitzung und hält sich an die hier festgelegten
> Regeln.

> 📌 **Sitzungsstart:** Zuerst `docs/STATUS.md` lesen (sobald angelegt) — dort
> steht der aktuelle Arbeitsstand und der nächste Schritt. Am Sitzungsende
> `docs/STATUS.md` aktualisieren.

> ⚙️ **Betriebsmodus:** Wayfinder wird **für den realen Betrieb** gebaut, nicht
> als Lernübung. Maßstab ist Produktionsreife: Korrektheit, Robustheit,
> Sicherheit, Betreibbarkeit und Zertifizierungs-Fähigkeit. (Frühere Sitzungen
> hatten einen ausdrücklichen Lern-/Didaktik-Charakter; der ist bewusst
> aufgegeben — siehe Fireflys ADR 0014.)

---

## 1. Worum es geht

Wayfinder ist das **ASD** (*Air Situation Display*) — die Lagedarstellung für den
Lotsen. Es **empfängt, dekodiert und stellt** die von Firefly berechneten
System-Tracks dar: aus dem Datenstrom auf der Leitung wird ein live mitlaufendes,
bedienbares Luftlagebild auf einer 2D-Karte.

**Schwester-Projekt Firefly.** Firefly ist der Radar-Tracker (eigenes Repo,
eigener Charter), der die Tracks rechnet und als **ASTERIX CAT062 über
UDP-Multicast** aussendet. Wayfinder ist der **produktive Konsument** dieses
Stroms. Die einzige Verbindung zwischen beiden Systemen ist der
**CAT062-Draht-Vertrag** — kein gemeinsamer Code, keine Bibliotheks-Abhängigkeit,
keine Punkt-zu-Punkt-Kopplung. Beide Systeme sind unabhängig baubar, testbar und
deploybar.

**Warum CAT062 und nicht JSON/WebSocket (Fireflys ADR 0014).** Wayfinder war
ursprünglich gegen einen JSON/WebSocket-Demo-Pfad geplant (M1: "WebSocket-Client
+ Datenmodell"). Mit dem Pivot auf Produktionsbetrieb gilt: Wayfinder *ist* das
ASD, und der für das ASD definierte Produktions-Kontrakt ist CAT062/UDP-Multicast
(Fireflys ADR 0006). Dieser Kontrakt liefert nativen Multi-Client-Fanout (kein
Replay-Problem), ist selbstbeschreibend (CAT/LEN/FSPEC, kein Typ-Diskriminator
nötig) und liefert WGS84-Positionen direkt (I062/105) — eine
JSON/WebSocket-Implementierung wird **nicht** gebaut.

---

## 2. Der Schnittstellen-Vertrag (CAT062 over UDP-Multicast)

Dies ist das Herzstück und der einzige Berührungspunkt mit Firefly. Wayfinder
**hält sich an diesen Vertrag**, statt Firefly-Code zu importieren.

> 📖 **Maßgebliche Quelle:** `docs/ICD-CAT062.md` im Firefly-Repo (versioniert,
> mit Changelog). Diese Sektion ist eine **Kurzfassung** für den schnellen
> Überblick — bei Abweichungen gilt die ICD. Änderungen an der ICD werden per
> Issue (Label `from-firefly`) angekündigt und hier nachgezogen.

- **Transport:** UDP-Multicast. Default-Gruppe `239.255.0.62`, Port `8600`
  (Firefly-Env: `FIREFLY_CAT062_GROUP` / `FIREFLY_CAT062_PORT`). Multicast-TTL
  ist standardmäßig 1 (subnetz-lokal). Ein Datagramm = ein vollständiger
  ASTERIX-Datenblock **einer Kategorie**; **keine** zusätzliche Anwendungs-Rahmung
  (keine Sequenznummern, keine Extra-Header). Seit ICD 2.3.0 (Fireflys ADR 0018)
  trägt derselbe Strom **zwei Kategorien**: **CAT062** (Tracks, `0x3E`) und
  **CAT065** (SDPS-Service-Status / Heartbeat, `0x41`). Wayfinder **dispatcht am
  führenden CAT-Oktett** und überspringt unbekannte Kategorien (robuster Decoder).
- **Update-Rate:** Keine feste, globale Periode — jeder Sensor hat seine eigene
  `scan_period` (typisch 4–12 s, ADR 0013 in Firefly). Datenblöcke treffen also
  in unregelmäßigem Takt ein, nicht in festen Intervallen.
- **Format:** ASTERIX **CAT062** (System-Tracks). Block = `[CAT=0x3E]
  [LEN: u16 BE] [Record]…`; LEN inkl. 3-Byte-Header. Mehrere Track-Records ohne
  Trenner hintereinander; jeder Record ist über sein **FSPEC** selbst-begrenzend.
- **Kodierte Items** (FRN → Item):

  | FRN | Item | Bedeutung | Kodierung |
  |-----|------|-----------|-----------|
  | 1 | I062/010 | SAC/SIC | — |
  | 4 | I062/070 | Time-of-Day | 1/128 s |
  | 5 | I062/105 | Position **WGS84** lat/lon | signed i32, LSB 180/2²⁵° |
  | 6 | I062/100 | Position System-Stereografisch X/Y | signed i24, LSB 0,5 m |
  | 7 | I062/185 | Geschwindigkeit Vx/Vy | signed i16, LSB 0,25 m/s |
  | 9 | I062/060 | Mode 3/A (nur wenn vorhanden) | — |
  | 10 | I062/245 | Target Identification / Callsign (nur wenn vorhanden) | STI/spare-Oktett + 8 × 6-Bit-IA-5-Zeichen |
  | 11 | I062/380 | ICAO-Adresse (nur wenn vorhanden) | — |
  | 12 | I062/040 | Track-Nummer | — |
  | 13 | I062/080 | Track-Status (CNF Oktett 1, **TSE** Oktett 2 = Track-Ende, CST Oktett 4) | variabel mit FX |
  | 14 | I062/290 | Update-Alter | — |
  | 17 | I062/136 | Flugfläche (nur wenn vorhanden) | signed i16, LSB 1/4 FL = 25 ft |
  | 27 | I062/500 | Genauigkeit | — |

  Die FRNs folgen der **echten EUROCONTROL-CAT062-UAP** (ICD ab v2.0.0,
  Fireflys ADR 0015): I062/500 sitzt auf FRN 27 (nicht 16), I062/295 (FRN 16)
  ist reserviert/ungenutzt. Ein Standard-Record hat damit ≥ 4 FSPEC-Oktette.
  Seit ICD 2.1.0 (additiv, AP7) wird zusätzlich I062/245 auf FRN 10 gesendet —
  FRN 10 liegt im bereits vorhandenen 2. FSPEC-Oktett, also ohne
  Wire-Format-Bruch. Seit ICD 2.2.0 (additiv, ADR 0016 in Firefly) trägt
  I062/080 das **TSE-Bit** (Oktett 2, Bit 7, `0x40`): die letzte Meldung für
  einen gelöschten Track. Wayfinder **entfernt** den Track beim Empfang sofort,
  statt auf einen Timeout zu warten.
- **Koordinaten:** I062/105 liefert **WGS84 direkt** — Wayfinder rendert daraus,
  eine stereografische Rückprojektion ist **nicht** nötig. I062/100 ist die
  zusätzliche System-Ebene (optional verwertbar); ihr Referenzpunkt ist
  aktuell der Firefly-Demo-Ursprung (Frankfurt) und damit nur im Demo-Kontext
  sinnvoll interpretierbar (offener Punkt: "Konfigurierbarer
  System-Referenzpunkt" in Fireflys Roadmap).
- **Zeit (I062/070):** 24-Bit-Zähler in 1/128-s-Ticks seit UTC-Mitternacht —
  **springt bei Mitternacht auf 0 zurück**. Wayfinder darf daraus keinen
  monoton steigenden Zeitstempel über Mitternacht hinweg ableiten; ein Sprung
  von nahe 86 400 s auf einen kleinen Wert ist ein normaler Tageswechsel,
  kein Datenfehler.
- **CAT065 — SDPS-Heartbeat (ICD 2.3.0, ADR 0018):** Derselbe Strom trägt
  periodisch (Default 1 s, wall-clock) eine **CAT065-SDPS-Status-Meldung**
  (`0x41`): I065/010 (SAC/SIC), I065/000 (Message Type = 1), I065/015
  (Service-ID), I065/030 (Time of Day, 1/128 s wie I062/070) und I065/040
  (NOGO operationell/degradiert). Zweck: „leerer Himmel" von „totem Feed"
  unterscheiden. Wayfinder erkennt **Staleness** (kein Heartbeat seit
  > `WAYFINDER_FEED_STALE_TIMEOUT`), zeigt ein Feed-Banner, exponiert
  `wayfinder_feed_stale`/`wayfinder_cat065_heartbeats_received_total` und lässt
  `/ready` bei stale Feed **nicht ready** werden.
- **Referenz-Spezifikation:** EUROCONTROL **SUR.ET1.ST05.2000-STD-09-01,
  Edition 1.10** ("CAT062 System Track Data") sowie die byte-genauen
  Encoder-Tests in Fireflys `firefly-asterix` (u.a.
  `single_track_matches_reference_dump`) sind die Grundwahrheit für den
  Wayfinder-Decoder.
- **Versionierung:** Änderungen am Vertrag sind **schnittstellen-relevant** und
  werden beidseitig per ADR nachgezogen. Der Decoder ist **tolerant** gegenüber
  unbekannten/zusätzlichen FSPEC-Bits (vorwärtskompatibel überspringen).

---

## 3. Arbeitsablauf: **Erst abstimmen, dann bauen**

Claude baut **keinen** nennenswerten Code, ohne vorher den nächsten Schritt
angekündigt und eine Freigabe eingeholt zu haben. Das ist ein **Design-/
Review-Tor**, kein Lern-Ritual: Es verhindert überraschende Architektur-Sprünge
und hält die Richtung abgestimmt. Pro Arbeitsschritt gilt dieser Ablauf:

1. **Ankündigen** — Was kommt als Nächstes? Getrennt nach:
   - **Fachlich** (*Warum* braucht das ASD das? Welches operative Problem löst
     es für den Lotsen?)
   - **Technisch** (*Wie*: Bausteine, Datenfluss, Dateien, Schnittstellen-Wirkung.)
   - **Komplexität & Modell** (Einstufung **S1–S5** mit Modell-Empfehlung, siehe
     unten).
2. **Freigabe abwarten** — anhalten, auf Rückfragen oder „Go" warten.
3. **In kleinen, testbaren Häppchen umsetzen.**
4. **Nachbereiten** — Doku/ADR/Tests, dann committen.

**Verboten:** „Durchrattern" — mehrere Bausteine ungefragt hintereinanderweg
bauen, ohne Abstimmung und Freigabe dazwischen.

### Komplexitäts-Skala & Modell-Angabe (Pflicht)

Jeder angekündigte Schritt bekommt eine Einstufung, **und das verwendete bzw.
empfohlene Modell wird genannt** — sowohl für den Schritt selbst als auch für
jede an einen Subagenten/Task delegierte Arbeit (Werkzeug-Läufe). Die Einstufung
schätzt, *wie anspruchsvoll* das saubere Umsetzen ist (Mathe, Algorithmik,
Architektur-Abwägung, Testumfang) — nicht bloß die Zeilenzahl.

| Stufe | Bedeutung | Modell-Empfehlung | Effort-Level |
|-------|-----------|-------------------|--------------|
| **S1** | Trivial/mechanisch (Doku-Kleinkram, Umbenennen) | Haiku 4.5 | niedrig |
| **S2** | Leicht (klar umrissen, wenig Logik) | Haiku 4.5 / Sonnet 4.6 | niedrig–mittel |
| **S3** | Mittel (etwas Logik, überschaubarer Umfang) | Sonnet 4.6 | mittel |
| **S4** | Anspruchsvoll (subtile Logik, Architektur, viele Tests) | Opus 4.8 / Fable 5 | hoch |
| **S5** | Sehr anspruchsvoll (große Architektur-Abwägungen) | Fable 5 / Opus 4.8 | hoch–max |

Faustregel: **S1–S2 → Haiku**, **S3 → Sonnet**, **S4–S5 → Opus 4.8 oder Fable 5**.
Bei Sicherheits- oder Schnittstellen-Wirkung lieber das stärkere Modell.

---

## 4. Sprache

- **Chat und Dokumentation (`docs/`, `CLAUDE.md`):** Deutsch.
- **Quellcode (Bezeichner, Kommentare im Code):** Englisch.
- (Konsistent mit Firefly, ADR 0002 dort.)

---

## 5. Technologie-Stack (ADR 0001: ratifiziert ✅)

Folgende Wahl ist **akzeptiert** (ADR 0001, 2026-06-13):

- **Sprache: Go** — netz-nativ (UDP/Multicast, Goroutines), statische Binaries,
  Cloud-native Standard.
- **Karten-Frontend: MapLibre GL JS** — anbieter-neutral, Open-Source, WebGL,
  konsistent mit Firefly (ADR 0009 dort).
- **Transport zum Browser: WebSocket-Server-Push** — decodierte Tracks vom
  Backend asynchron an den Browser, getrennt vom CAT062-Eingangs-Pfad.

Damit kann der erste Code entstehen.

**Noch nicht ratifiziert (folgen später):**
- Frontend-UI-Framework (Vue/React/Svelte) — ADR 0002
- Browser-Authentifizierung / Sicherheit — ADR 0003
- Observability (Logs, Metriken) — ADR 0004
- Deployment & Konfiguration — ADR 0005

---

## 6. Qualitäts-Gates (vor jedem Commit)

Ein Schritt gilt erst als fertig, wenn (Go-Variante; bei abweichendem Stack
sinngemäß):

- [ ] `go test ./...` ist grün.
- [ ] `go vet ./...` und `gofmt`/`golangci-lint` ohne Befunde.
- [ ] Der CAT062-Decoder ist gegen **byte-genaue Referenz-Vektoren** getestet
      (Grundwahrheit: Fireflys Encoder-Tests / Referenz-Dump).
- [ ] Neue/​geänderte Anforderungen sind im Anforderungs-Register
      (`docs/requirements/`) eingetragen und mit Code/Test rückverfolgbar.
- [ ] Sicherheits-relevante Pfade (Feed-Eingang, Auth am Browser-Rand) sind
      bewertet (Abschnitt 7).
- [ ] Die zugehörige Doku wurde aktualisiert.
- [ ] Der Commit hat eine klare, beschreibende Nachricht.

---

## 7. Querschnitts-Prinzipien (gelten in *jedem* Schritt)

### Sicherheit (kritisch — ASD ist sicherheitsrelevant)
- **Robuster Decoder:** Niemals einem Datagramm vertrauen. Längen prüfen,
  Grenzen einhalten, fehlerhafte Records verwerfen statt abstürzen
  (kein Panic auf Eingabe-Daten). Fuzzing des Parsers vorsehen.
- **Feed-Authentizität:** Multicast hat keine eingebaute Authentifizierung —
  Eingangs-Pfad und Vertrauensgrenze explizit dokumentieren (Netz-Isolation
  und/oder anwendungsseitige Absicherung; eigener ADR).
- **Browser-Rand absichern:** Wenn Wayfinder die Lage an Browser verteilt, ist
  dieser Rand (Auth/TLS) abzusichern — das ASD-Bild ist nicht öffentlich.

### Cloud-nativ (anbieter-neutral, Kubernetes-tauglich)
- **Deterministische Verarbeitung nach Datenzeit** (ASTERIX Time-of-Day, nicht
  Wanduhr), passend zu Fireflys Determinismus.
- **12-Factor-Konfiguration**, Health-/Readiness-Probes, sauberes Herunterfahren.
- **Observability** (strukturierte Logs, Metriken, Tracing) ist Pflicht.

### Zertifizierungs-fähig (Orientierung ED-153 + ED-109A/DO-278A)
- **Rückverfolgbarkeit** Anforderung → Design → Code → Test.
- **Konfigurationsmanagement**: Versionskontrolle, getaggte Baselines, ADRs.
- **Ehrliche Grenze:** Wir bauen *zertifizierungs-fähig*; die formale
  Zertifizierung selbst ist nicht Teil dieses Code-Projekts.

---

## 8. Dokumentationspflichten

| Ebene | Ort | Zweck |
|-------|-----|-------|
| **Code-Doku** | Doc-Kommentare | *Warum*, nicht nur *Was*. |
| **Feature-Doku** | `docs/milestones/` | Pro Baustein eine präzise Erklärung. |
| **Glossar** | `docs/glossary.md` | Domänen-Referenz (ASTERIX, CAT062, ASD …). |
| **Entscheidungen** | `docs/decisions/` | ADRs. |

ADR 0001 (Wayfinder) hält den **Stack** fest; ein früher ADR hält die
**CAT062-Schnittstelle** als bewusst gewählten Vertrag fest (Spiegel zu Fireflys
ADR 0006).

---

## 9. Cross-Project-Todos (Firefly ↔ Wayfinder)

Firefly und Wayfinder sind getrennte Projekte mit getrennten Claude-Sitzungen.
Eine Claude-Sitzung hat Zugriff auf **beide** Repos. Das ermöglicht **direkte
Cross-Project-Kommunikation über GitHub Issues**.

### Workflow

1. **Beobachtung** — Während der Arbeit an Wayfinder erkennt Claude ein Thema,
   das Firefly betreffen könnte (z.B. Schnittstellen-Fragen zum CAT062-Vertrag).
2. **Issue erstellen** — Claude erstellt ein Issue im anderen Repo mit Label
   `from-wayfinder` (oder `from-firefly`).
3. **Dokumentieren** — Die Issue wird referenziert in
   `docs/cross-project/todo-for-<anderes-projekt>.md`.
4. **Checken** — Beim Sitzungsstart: Offene Issues aus der anderen Sitzung
   ansehen (GitHub-Issues mit `from-firefly` oder `from-wayfinder`).
5. **Aktualisieren** — Wenn ein Issue erledigt ist, wird es geschlossen und
   die Referenz in der `.md`-Datei aktualisiert.

### Dateien

- **`docs/cross-project/todo-for-wayfinder.md`** — Probleme/Wünsche aus
  Firefly, die Wayfinder-Arbeit beeinflussen.
- **`docs/cross-project/todo-for-firefly.md`** — Probleme/Wünsche aus
  Wayfinder, die Firefly-Arbeit beeinflussen.

Siehe auch `docs/cross-project/README.md` für die Hintergrund-Erklärung.

### Stand nach Fireflys ADR 0014 (CAT062-Pivot)

Die ursprünglichen Issues #6–#10 (`from-wayfinder`) waren gegen den
JSON/WebSocket-Pfad formuliert. Nach dem Pivot auf CAT062/UDP:

- **#6** (Pub/Sub-Fan-out), **#8** (Typ-Diskriminator), **#10** (Schema-
  Versionierung) — **geschlossen**, durch die Multicast-/ASTERIX-Architektur
  gegenstandslos.
- **#7** (Auth auf `/ws`) — **transformiert**: Sicherheitsfrage verschiebt sich
  auf Netz-Isolation des Multicast-Pfads + Browser-Rand von Wayfinder (siehe
  Abschnitt 7).
- **#9** (UTC Time-of-Day) — **bleibt offen und wird zentraler**: CAT062
  I062/070 ist das Time-of-Day-Feld, das Wayfinder konsumiert. Firefly arbeitet
  an der Migration auf echte UTC-Tageszeit.

Details siehe `docs/cross-project/todo-for-firefly.md`.

---

## 10. Was Claude NICHT tut

- Nicht mehrere Bausteine ungefragt hintereinander bauen.
- Keine großen, überraschenden Architektur-Sprünge ohne ADR und Freigabe.
- **Keinen Firefly-Code importieren** — die Kopplung läuft ausschließlich über
  den CAT062-Draht-Vertrag.
- Einem Netz-Datagramm nicht blind vertrauen (robuster, getesteter Decoder).
- Nicht „fertig" melden, solange die Qualitäts-Gates (Abschnitt 6) nicht erfüllt
  sind.
- Korrektheit und Sicherheit nicht dem Tempo opfern.
