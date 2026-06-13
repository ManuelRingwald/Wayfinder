# Rückmeldungen an Firefly – Produktionsreife der WebSocket-Schnittstelle

> Diese Datei sammelt Beobachtungen aus der Wayfinder-Entwicklung (Stand: M1,
> 2026-06-13), die für den **produktiven Einsatz von Firefly bei einem ANSP**
> relevant sind. Sie betreffen die `/ws`-Schnittstelle, die Wayfinder als ASD
> konsumiert. Gedacht zum Übertragen ins Firefly-Projekt (eigenes Repo) und dort
> als ADRs/Requirements/Issues weiterzuverfolgen.
>
> Wayfinder baut seinen WebSocket-Client robust gegenüber diesen Lücken (siehe
> ADR 0001), aber die eigentliche Lösung liegt im Firefly-Server.
>
> Siehe `docs/cross-project/README.md` für den Übertragungs-Workflow.

---

## 1. Replay statt echtem Live-Broadcast

**Beobachtung:** `firefly-server` berechnet den Frame-Stream einmal beim Start
(deterministisch, vom `Player`) und spielt ihn **jedem neu verbundenen Client
einzeln von Frame-Index 0 ab** (`pump_frames` in `crates/firefly-server/src/app.rs`).

**Problem für den Produktivbetrieb:** Eine ASD zeigt die *aktuelle* Luftlage.
Bei mehreren gleichzeitigen Arbeitsplätzen (mehrere Lotsen, mehrere ASDs) müsste
jeder Client den **gleichen, aktuellen** Zustand sehen — nicht eine eigene
Wiederholung der gesamten Szenario-Historie ab Sekunde 0. Ein neu verbundener
Client (z.B. nach Reconnect) sollte den **aktuellen Stand** bekommen (Snapshot),
danach die laufenden Updates (Deltas/Broadcast).

**Empfehlung:** Architektur auf Pub/Sub-Fan-out umstellen: ein zentraler
"aktueller Frame" wird an alle verbundenen Clients gebroadcastet; neu
verbindende Clients erhalten zunächst einen Snapshot des aktuellen Zustands.

---

## 2. Keine Authentifizierung/Autorisierung auf `/ws`

**Beobachtung:** Die WebSocket-Route ist offen, jeder kann sich verbinden und
den Frame-Stream empfangen.

**Problem für den Produktivbetrieb:** Eine ASD zeigt sicherheitsrelevante
Live-Luftlage. Der Zugriff muss authentifiziert und autorisiert sein
(wer darf welche ASD/welchen Sektor sehen).

**Empfehlung:** Auth-Schicht vor/auf `/ws` (z.B. Token-basiert), Rollen-/
Sektor-Konzept klären.

---

## 3. Kein Nachrichtentyp-Diskriminator im JSON

**Beobachtung:** Aktuell gibt es zwei Arten von WebSocket-Nachrichten:
- `Frame` (Plots + Tracks) — kein Typ-Feld
- `{"event":"delay_triggered", ...}` — nur dieses hat ein `"event"`-Feld

Ein Konsument muss raten/prüfen, ob `"event"` vorhanden ist, um den Typ zu
bestimmen.

**Problem für den Produktivbetrieb:** Fragil bei Schema-Erweiterungen — neue
Nachrichtentypen sind nicht klar unterscheidbar, Versionierung schwierig.

**Empfehlung:** Einheitliches Hülle-Format mit explizitem Typ-Feld, z.B.
`{"type": "frame", "data": {...}}` bzw. `{"type": "delay_triggered", "data": {...}}`.

---

## 4. `time` ohne Wandzeit-/UTC-Bezug

**Beobachtung:** `Timestamp` (in `crates/firefly-core/src/time.rs`) ist
"Sekunden seit einem willkürlichen, aber festen Epoch" — für die Simulation
einfach Sekunden seit Szenario-Start. Laut Doku-Kommentar später für ASTERIX
"Time of Day" (Sekunden seit UTC-Mitternacht) vorgesehen.

**Problem für den Produktivbetrieb:** Eine ASD muss dem Lotsen eine
**UTC-Uhrzeit** am Track anzeigen können. "Sekunden seit Szenario-Start" ist für
einen Live-Betrieb nicht direkt verwertbar.

**Empfehlung:** Umstellung auf ASTERIX Time-of-Day (UTC-Bezug) wie in den
Doku-Kommentaren bereits angedacht — diese Migration früh einplanen, da
Wayfinder (und jeder andere Konsument) sich darauf verlassen wird.

---

## 5. Keine Protokoll-Versionierung

**Beobachtung:** Die JSON-Wire-Formate (`Frame`, `FrameTrack`, `FramePlot`,
Event-Messages) haben kein Versionsfeld.

**Problem für den Produktivbetrieb:** Server und Client(s) müssen synchron
aktualisiert werden; ohne Versionsfeld ist nicht erkennbar, ob ein Client eine
inkompatible Server-Version anspricht (insbesondere bei mehreren
ASD-Instanzen/Rollouts).

**Empfehlung:** Ein `"schema_version"`-Feld (oder Teil der vorgeschlagenen
Hülle aus Punkt 3) einführen, das Konsumenten prüfen können.

---

## Kontext: Was Firefly schon richtig macht (positiv vermerkt)

- **Sicherheitsrelevante Statusfelder werden bereits durchgereicht**:
  `confirmed`, `coasting`, `update_age_s`, `position_uncertainty_m` (ADR 0008
  bei Firefly) — genau das, was eine ASD für die Darstellung von
  Unsicherheits-Ringen und Tentative/Coasting-Zuständen braucht.
- **Health-/Readiness-Probes** (`/health`, `/ready`) sind vorhanden
  (Kubernetes-tauglich, ADR 0003 bei Firefly).
- **12-Factor-Konfiguration** über Env-Vars (`FIREFLY_PORT`, `FIREFLY_SPEED`,
  `FIREFLY_SCENE`) ist bereits umgesetzt.
- **Format ist flach und selbstbeschreibend** (Feldnamen klar, Newtypes als
  bare scalars) — gute Grundlage für Punkt 3 (Hülle drumherum, statt alles
  umzubauen).
