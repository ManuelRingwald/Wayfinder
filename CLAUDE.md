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

- **Transport:** UDP-Multicast. Default-Gruppe `239.255.0.62`, Port `8600`
  (Firefly-Env: `FIREFLY_CAT062_GROUP` / `FIREFLY_CAT062_PORT`). Multicast-TTL
  ist standardmäßig 1 (subnetz-lokal). Ein Datagramm = ein vollständiger
  CAT062-Datenblock = ein Scan; **keine** zusätzliche Anwendungs-Rahmung (keine
  Sequenznummern, keine Extra-Header).
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
  | 11 | I062/380 | ICAO-Adresse (nur wenn vorhanden) | — |
  | 12 | I062/040 | Track-Nummer | — |
  | 13 | I062/080 | Track-Status | variabel mit FX |
  | 14 | I062/290 | Update-Alter | — |
  | 16 | I062/500 | Genauigkeit | — |

- **Koordinaten:** I062/105 liefert **WGS84 direkt** — Wayfinder rendert daraus,
  eine stereografische Rückprojektion ist **nicht** nötig. I062/100 ist die
  zusätzliche System-Ebene (optional verwertbar).
- **Referenz-Spezifikation:** EUROCONTROL CAT062 (ASTERIX) sowie die
  byte-genauen Encoder-Tests in Fireflys `firefly-asterix` (u.a.
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

## 5. Technologie-Stack (Stand: in Ratifizierung)

Wayfinder ist noch Greenfield. Folgende Wahl ist **vorgeschlagen** und wird mit
**ADR 0001 (Wayfinder)** ratifiziert, bevor Code entsteht:

- **Sprache: Go** — gut geeignet für netz-nahe Dienste (UDP/Multicast,
  Nebenläufigkeit, statische Binaries, Container).
- **Karten-Frontend:** offene, anbieter-neutrale 2D-Vektorkarte (Kandidat:
  MapLibre GL, konsistent mit Fireflys Frontend-Linie) — eigener ADR.
- **Transport zum Browser:** offen (z.B. WebSocket-Push der dekodierten Tracks
  vom Wayfinder-Backend an den Browser — getrennt vom CAT062-Eingangs-Pfad) —
  eigener ADR.

Bis ADR 0001 (Wayfinder) steht, gilt keiner dieser Punkte als fixiert.

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
