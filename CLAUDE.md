# Wayfinder — Projekt-Charter & Arbeitsregeln

> Dieses Dokument ist die **verbindliche Arbeitsvereinbarung** zwischen dem
> Projektverantwortlichen (Mensch) und dem KI-Assistenten (Claude). Claude liest
> diese Datei zu Beginn jeder Sitzung und hält sich an die hier festgelegten
> Regeln. Es ist zugleich ein menschenlesbares Manifest: *So* arbeiten wir.

> 📌 **Sitzungsstart:** Zuerst `docs/STATUS.md` lesen — dort steht der aktuelle
> Arbeitsstand und der nächste Schritt (wichtig fürs geräteübergreifende
> Weiterarbeiten). Am Sitzungsende `docs/STATUS.md` aktualisieren.

---

## 1. Worum es geht

Wayfinder ist die **Luftlagedarstellung (ASD – Air Situation Display)** – das
Frontend einer modernen Radar-Verkehrskontroll-Anlage. Es verbindet sich mit dem
Firefly-Tracker und zeigt die berechneten Tracks (Position, Geschwindigkeit,
Identität jedes Luftfahrzeugs) live auf einer **2D-Kartendarstellung** an.

Das fachliche Ziel ist anspruchsvoll. Das **didaktische** Ziel ist genauso
wichtig:

> **Der Weg ist das Ziel.** Der Projektverantwortliche ist IT-Projektleiter bei
> einem ANSP (Flugsicherungsorganisation), **ohne formale IT-Ausbildung**, und
> will dieses Projekt nutzen, um Visualisierung und Luftlage-Fachlichkeit
> wirklich zu verstehen. Tempo ist zweitrangig. Verständnis ist erstrangig.

---

## 2. Die goldene Regel: **Erst erklären, dann bauen**

Claude darf **keinen** nennenswerten Code schreiben, ohne vorher den nächsten
Schritt verständlich erklärt und eine Freigabe eingeholt zu haben. Pro
Arbeitsschritt gilt dieser Ablauf:

1. **Ankündigen** — Was kommt als Nächstes? In einfachen Worten, getrennt nach:
   - **Fachlich** (*Warum* braucht eine ASD das? Was ist das Problem
     aus Sicht der Luftlage?)
   - **Technisch** (*Wie* setzen wir es um? Welche Bausteine, welche
     Datenstrukturen, welche Dateien?)
   - **Komplexität** (Einstufung **S1–S5** mit Modell-Empfehlung, siehe unten —
     damit der Projektverantwortliche das passende Modell wählen kann.)
2. **Begriffe klären** — Jeder neue Fachbegriff wird beim ersten Auftreten
   erklärt und in `docs/glossary.md` aufgenommen. Keine unerklärten Abkürzungen.
3. **Freigabe abwarten** — Claude hält an und wartet auf Rückfragen oder ein
   „Go". Erst dann wird implementiert.
4. **In kleinen, testbaren Häppchen umsetzen** — Lieber drei kleine, je für sich
   verständliche Schritte als ein großer Sprung.
5. **Nachbereiten** — Doku aktualisieren (Meilenstein-Erklärung, Glossar, ggf.
   Entscheidungs-Log), Tests grün, dann committen.

**Verboten:** „Durchrattern" — also mehrere Bausteine ungefragt
hintereinanderweg bauen, ohne Erklärung und Freigabe dazwischen.

### Komplexitäts-Skala (für die Modellwahl)

Jeder angekündigte Schritt bekommt eine Einstufung. Sie schätzt, *wie
anspruchsvoll* das saubere Erklären **und** Umsetzen des Schritts ist (Mathe,
Algorithmik, Architektur-Abwägung, Testumfang) — nicht bloß die Zeilenzahl.

| Stufe | Bedeutung | Modell-Empfehlung | Effort-Level |
|-------|-----------|-------------------|--------------|
| **S1** | Trivial/mechanisch (Doku-Kleinkram, Umbenennen, Tippen) | Haiku 4.5 | niedrig |
| **S2** | Leicht (klar umrissen, wenig Logik) | Haiku 4.5 / Sonnet 4.6 | niedrig–mittel |
| **S3** | Mittel (etwas Mathe/Logik, überschaubarer Umfang) | Sonnet 4.6 | mittel |
| **S4** | Anspruchsvoll (subtile Mathe/Algorithmen, Architektur, viele Tests) | Opus 4.8 / Fable 5 | hoch |
| **S5** | Sehr anspruchsvoll (tiefe Mathe, Rendering-Optimierung, große Architektur-Abwägungen) | Fable 5 / Opus 4.8 | hoch–max |

Faustregel: **S1–S2 → Haiku**, **S3 → Sonnet**, **S4–S5 → Opus 4.8 oder Fable 5**.
Das **Effort-Level** mit der Stufe mitziehen (niedrig bei S1, max bei S5). In
einem Lernprojekt, in dem die *Erklärung* zählt, bei Grenzfällen lieber das
stärkere Modell und das höhere Effort-Level.

---

## 3. Sprache

- **Erklärungen, Chat und Dokumentation (`docs/`, `CLAUDE.md`):** Deutsch.
- **Quellcode (Bezeichner, Kommentare im Code):** Englisch — internationaler
  Industriestandard, hält den Code portabel und anschlussfähig. Die *Erklärung*
  des Codes erfolgt dann auf Deutsch in den `docs/` bzw. im Chat.
- Diese Aufteilung ist eine bewusste Entscheidung und kann jederzeit geändert
  werden, wenn der Projektverantwortliche es wünscht.

---

## 4. Dokumentationspflichten

Dokumentation ist in diesem Projekt **kein Nachgedanke, sondern Teil der
Leistung**. Es gibt drei Ebenen:

| Ebene | Ort | Zweck |
|-------|-----|-------|
| **Code-Doku** | Doc-Kommentare (`//`, `/**/`) im Code | Erklären das *Warum* eines Pakets/Typs, nicht nur das *Was*. |
| **Lern-/Fach-Doku** | `docs/milestones/` | Pro Meilenstein eine verständliche Erklärung in Deutsch: Fachlichkeit + Technik + Architektur in Worten. |
| **Glossar** | `docs/glossary.md` | Wächst mit. Jeder Fachbegriff einmal in einfacher Sprache, gern mit Analogie. |
| **Entscheidungen** | `docs/decisions/` | Architecture Decision Records (ADR): *welche* wichtige Entscheidung *warum* getroffen wurde. |

Regeln:
- Jeder neue Meilenstein bekommt **vor dem Abschluss** seine Erklärung in
  `docs/milestones/`.
- Jede architektonisch relevante Weichenstellung bekommt einen kurzen ADR.
- Das Glossar wird bei jedem neuen Begriff gepflegt — nicht „später".

---

## 5. Qualitäts-Gates (vor jedem Commit)

Ein Schritt gilt erst als fertig, wenn:

- [ ] `go test ./...` ist grün.
- [ ] `go vet ./...` ist sauber.
- [ ] `golangci-lint run ./...` ist ohne Warnungen.
- [ ] `go fmt` wurde ausgeführt (oder `gofmt -w .`).
- [ ] Kein `unsafe`-Code ohne ausdrückliche, dokumentierte Begründung.
- [ ] Neue/​geänderte Anforderungen sind im Anforderungs-Register
      (`docs/requirements/`) eingetragen und mit Code/Test rückverfolgbar.
- [ ] Die zugehörige Doku wurde aktualisiert.
- [ ] Der Commit hat eine klare, beschreibende Nachricht.

---

## 6. Git & Branches

- Entwicklung **immer** auf dem vereinbarten Feature-Branch
  (`claude/loving-turing-2obzk6`).
- Niemals ungefragt auf einen anderen Branch pushen.
- **Kein** Pull Request, außer der Projektverantwortliche bittet ausdrücklich
  darum.
- Commits klein und thematisch geschnitten; eine Sache pro Commit.

---

## 7. Inkrementelles Vorgehen — die Meilensteine

| Meilenstein | Inhalt | Status |
|-------------|--------|--------|
| **M1** | WebSocket-Client + Datenmodell (Tracks von Firefly) | ⏳ aktuell |
| **M2** | Statische 2D-Kartendarstellung (Canvas/WebGL) | ⏳ |
| **M3** | Live-Rendering der Tracks auf der Karte | ⏳ |
| **M4** | Interaktion (Zoom, Pan, Layer, Filtering) | ⏳ |
| **M5** | ASD-Elemente (Labels, Trails, Konflikt-Darstellung) | ⏳ |

Innerhalb eines Meilensteins arbeiten wir in kleinen Schritten nach der goldenen
Regel (Abschnitt 2).

---

## 8. Querschnitts-Prinzipien (gelten in *jedem* Meilenstein)

Zwei nicht-funktionale Anforderungen prägen die gesamte Architektur:

### Cloud-nativ (anbieter-neutral, Kubernetes-tauglich)

- **Konfigurierbarkeit:** Firefly-Adresse (Host/Port), Kartenkachel-Source, Kartenmittelpunkt/-zoom via Env-Var oder Config-Datei.
- **Health-/Readiness-Probes:** Wayfinder signalisiert, ob die Verbindung zu Firefly aktiv ist.
- **Strukturierte Logs** (JSON oder Textformat wählbar) für Observability.
- **Sauberes Herunterfahren:** WebSocket-Verbindung ordnungsgemäß schließen.

### Zertifizierungs-fähig (Orientierung ED-153 + ED-109A/DO-278A)

- **Rückverfolgbarkeit**: Anforderung → Design → Code → Test, in beide Richtungen.
- **Verifikationsnachweise**: Tests mit gemessener Abdeckung.
- **Konfigurationsmanagement**: Versionskontrolle, getaggte Baselines, ADRs.
- **Ehrliche Grenze**: Wir bauen *zertifizierungs-fähig*. Die formale
  Zertifizierung selbst ist ein organisatorisch-regulatorischer Schritt
  und nicht Teil dieses Code-Projekts — das versprechen wir nicht.

---

## 9. Cross-Project-Todos (Firefly ↔ Wayfinder)

Firefly und Wayfinder sind getrennte Projekte mit getrennten Claude-Sitzungen.
Eine Claude-Sitzung hat jetzt Zugriff auf **beide** Repos. Das ermöglicht
**direkte Cross-Project-Kommunikation über GitHub Issues**.

### Workflow

1. **Beobachtung** — Während der Arbeit an Wayfinder erkennt Claude ein Thema,
   das Firefly betreffen könnte (z.B. fehlende API-Features, unklarere Daten).
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
  Wayfinder, die Firefly-Arbeit beeinflussen (derzeit 5 offene Issues: #6–#10).

Siehe auch `docs/cross-project/README.md` für die Hintergrund-Erklärung.

---

## 10. Was Claude NICHT tut

- Keine unerklärten Fachbegriffe oder Abkürzungen verwenden.
- Nicht mehrere Bausteine ungefragt hintereinander bauen.
- Keine großen, überraschenden Architektur-Sprünge ohne ADR und Freigabe.
- Nicht „fertig" melden, solange die Qualitäts-Gates (Abschnitt 5) nicht erfüllt
  sind.
- Tempo nicht über Verständnis stellen.
