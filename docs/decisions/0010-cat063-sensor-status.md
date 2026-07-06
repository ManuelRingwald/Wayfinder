# ADR 0010 — CAT063 Sensor Status Decoder (Per-Sensor-Liveness)

- **Status:** akzeptiert; UAP-Format **abgelöst durch ADR 0019**
- **Datum:** 2026-06-25

> ⚠️ **Nachtrag (ADR 0019, 2026-07-06):** Das hier beschriebene Record-Format
> (FSPEC `0xE0`; Sensor-Identität in I063/010; I063/030 auf FRN 2, I063/060 auf
> FRN 3) folgte Fireflys damaliger, **nicht standardkonformer** CAT063-UAP. Mit
> Fireflys ADR 0032 (ICD 3.0.0, breaking) wurde die UAP auf die echten
> EUROCONTROL-FRN-Slots gebracht; Wayfinders Decoder zieht in **ADR 0019** nach
> (Sensor aus I063/050, FSPEC `0xB8`, RE/SP längen-tolerant). Zweck und
> Verdrahtung (WF-2/WF-3, Health-Registry, Banner) bleiben gültig; nur das
> Byte-Format ist überholt.
- **Schnittstellen-relevant:** ja (CAT063/`0x3F` neu auf dem Multicast-Strom,
  ICD → 2.5.0, additiv; Auslöser Firefly ADR 0022 / Issue #32 `from-wayfinder`)
- **Bezug:** Spiegel zu Fireflys ADR 0022; schließt den Blocker für das gelbe
  Sensor-Degradierungs-Banner (AP4-Farbsemantik, ADR 0009).

## Kontext

Seit ICD 2.5.0 sendet Firefly auf demselben UDP-Multicast-Strom eine dritte
ASTERIX-Kategorie:

- **CAT062** (`0x3E`) — System-Tracks (was fliegt).
- **CAT065** (`0x41`) — SDPS-Heartbeat (lebt das Datenverarbeitungssystem, ADR
  0003 / Wayfinder ADR → FR-DATA-004).
- **CAT063** (`0x3F`) — Per-Sensor-Status (welche Sensoren speisen das SDPS).

CAT063 beantwortet eine Frage, die CAT062 und CAT065 gemeinsam **offenlassen**:
Fällt ein Radar aus, läuft Fireflys Tracker (und damit CAT065) ungestört weiter.
Das Lagebild wird nur in der Abdeckung des ausgefallenen Sensors schlechter —
stiller, schleichender Qualitätsverlust ohne jedes Warnsignal für den Lotsen.

Wayfinder hat das gelbe **Sensor-Degradierungs-Banner** seit AP4 (ADR 0009)
strukturell vorbereitet (`FeedSnapshot.Color()` gibt `"yellow"` zurück, wenn
`0 < SensorsActive < SensorsTotal`), konnte es aber nicht aktivieren, solange
kein CAT063 auf dem Draht lag. Dieser ADR und der dazugehörige Code räumen den
Blocker.

## Entscheidung

1. **Neues Paket `pkg/cat063`** mit `DecodeSensorBlock(data []byte)` nach dem
   Muster von `pkg/cat065`. Der Decoder ist gegen **byte-genaue
   Referenz-Vektoren** aus Fireflys ICD §9 verifiziert (9 Tests, CLAUDE.md §6).

2. **FSPEC und Items.** Jeder CAT063-Record trägt FSPEC `0xE0`:
   - **I063/010** (FRN 1, 2 Byte) — SAC/SIC des Sensors.
   - **I063/030** (FRN 2, 3 Byte) — Time of Day, 1/128 s wie I062/070.
   - **I063/060** (FRN 3, 1 Byte) — Sensor Configuration & Status (NOGO-Feld);
     Bits `0xC0`: `0x00` = operationell, `0x40` = degradiert, `0x80` = nicht
     verbunden, `0xC0` = nicht initialisiert. Wayfinder wertet
     `(NOGO & 0xC0) == 0x00` als aktiv; jeder andere Wert gilt als inaktiv.

3. **Dispatch im Receiver.** `pkg/receiver.Receiver.dispatch()` bekommt einen
   dritten `case cat063.Category` neben den bestehenden CAT062/CAT065-Fällen.
   `handleSensorStatus` zählt aktive vs. gesamte Sensoren und ruft
   `sensorStatusHandler(feedID, statuses)` auf. Unbekannte Kategorien werden
   wie bisher verworfen (Decode-Fehler-Zähler, kein Panic).

4. **Registry-Verdrahtung.** `pkg/health.Registry.RecordSensors(feedID, active,
   total int)` schreibt `sensorsActive`/`sensorsTotal` in den `feedEntry`.
   `Snapshot()` liest sie aus und gibt sie im `FeedSnapshot` zurück. Damit
   aktiviert sich `FeedSnapshot.Color()` → `"yellow"` automatisch, sobald der
   erste CAT063-Block mit Teilausfall eintrifft.

5. **Broadcast-Pfad (Option B).** Jeder CAT065-Heartbeat *und* jeder
   CAT063-Block löst `broadcastFeedSnapshot(feedID, snap)` aus. Der Broadcaster
   liefert die `FeedStatusMessage{FeedID, Color, SensorsActive, SensorsTotal}`
   nur an Clients, deren `Scope.AllowsFeed(feedID)` wahr ist (per-Feed-Scoping,
   Multi-Tenant-tauglich). FeedID = 0 (single-tenant fallback) passiert
   uneingeschränkt.

6. **Keine neuen Env-Variablen.** CAT063 kommt automatisch mit dem bestehenden
   `FIREFLY_CAT062_GROUP:FIREFLY_CAT062_PORT`-Strom; es gibt keine eigene
   Aktivierung.

## Konsequenzen

**Positiv:**
- Das gelbe Sensor-Degradierungs-Banner ist jetzt vollständig aktiviert: Fällt
  ein Radar aus, sieht der Lotse sofort das gelbe Banner mit der Information,
  welcher Feed und wie viele Sensoren betroffen sind.
- Drei klar getrennte Liveness-Ebenen auf einem einzigen Strom:
  CAT062 (Tracks), CAT065 (SDPS lebt), CAT063 (welche Sensoren speisen).
- Der Decoder ist vorwärtskompatibel: unbekannte FSPEC-Bits werden
  übersprungen, kein Panic auf unbekannte Eingaben (CLAUDE.md §7).

**Zu beachten:**
- Der gemeinsame Strom enthält jetzt drei Kategorien. Konsumenten, die nur
  auf CAT062/CAT065 eingestellt sind, müssen den `default`-Fall ihres
  Dispatch (verwerfen) korrekt implementieren, damit CAT063-Datagramme nicht
  als Decode-Fehler gezählt werden.
- Firefly sendet CAT063 nur im **Live-Modus** (OpenSky, Echtzeit); im
  **Replay** sind alle Sensoren per Definition aktiv (Determinismus-Regel,
  Fireflys ADR 0003). Das Frontend zeigt in diesem Fall kein gelbes Banner —
  das ist gewollt.

## Rückverfolgbarkeit

- **Anforderung:** FR-DATA-006 (`docs/requirements/README.md`).
- **Code:**
  - `pkg/cat063/decoder.go` + `pkg/cat063/decoder_test.go` (WF-1)
  - `pkg/health/registry.go` `RecordSensors` (WF-2)
  - `pkg/receiver/receiver.go` `handleSensorStatus` (WF-2)
  - `cmd/wayfinder/feeds.go` `buildReceivers` `sensorStatusHandler` (WF-2)
  - `cmd/wayfinder/main.go` `sensorStatusHandler`, `broadcastFeedSnapshot` (WF-3)
  - `pkg/broadcast/broadcast.go` `FeedStatusMessage` + FeedID-Scoping (WF-3)
  - `internal/webui/static/index.html` + `app.js` (WF-3, gelbes Banner)
- **Firefly:** ADR 0022, Issue #32 (`from-wayfinder`), ICD 2.5.0.
- **Cross-Project:** `docs/cross-project/todo-for-wayfinder.md` (Issue #72).
