# Milestone: CAT063 Sensor Status — Gelbes Degradierungs-Banner vollständig aktiviert

**Arbeitspakete:** WF-1 (CAT063-Decoder), WF-2 (Registry-Verdrahtung),
WF-3 (Broadcast-Pfad Option B + Frontend)
**Datum:** 2026-06-25
**Bezug:** Firefly ADR 0022 / Issue #32 (`from-wayfinder`); Wayfinder ADR 0010,
Issue #72 (`from-firefly`)

---

## Fachlicher Hintergrund

Fällt ein Radar aus, läuft Fireflys Tracker ungestört weiter: CAT062 liefert
Tracks aus den verbleibenden Sensoren und aus Coasting, der CAT065-Heartbeat
bleibt „operationell". Das Lagebild wird nur in der **Abdeckung** des
ausgefallenen Sensors ärmer — leiser, schleichender Qualitätsverlust ohne jedes
Warnsignal für den Lotsen.

Dieses Milestone aktiviert das gelbe **Sensor-Degradierungs-Banner**: Sobald
mindestens ein Sensor abgefallen ist, aber noch mindestens einer aktiv meldet,
zeigt der Browser ein gelbes Banner mit der Information welcher Feed und wie
viele Radare betroffen sind.

---

## Was vorher existierte (AP4, ADR 0009)

- `pkg/health.FeedSnapshot.Color()` gab bereits `"yellow"` zurück, wenn
  `0 < SensorsActive < SensorsTotal`.
- `FeedStatusMessage` kannte bereits `SensorsActive`/`SensorsTotal`-Felder.
- Das Frontend zeigte gelb, wenn diese Felder entsprechend gefüllt waren.
- **Problem:** `RecordSensors` wurde nie aufgerufen — CAT063 fehlte als
  Eingabe. `SensorsActive`/`SensorsTotal` blieben immer 0.

---

## WF-1 — CAT063-Decoder (`pkg/cat063`)

**Neues Paket** nach dem Muster von `pkg/cat065`.

### Format (FSPEC `0xB8`, Standard-UAP ab ICD 3.0.0)

> ⚠️ **UAP-Standardisierung (ICD 3.0.0, Fireflys ADR 0032 / Wayfinder ADR 0019,
> BREAKING).** Bis ICD 2.6.1 trug der Record eine kompaktierte UAP (FSPEC `0xE0`;
> Sensor-Identität in I063/010; I063/030 auf FRN 2, I063/060 auf FRN 3). Ab
> 3.0.0 gelten die echten EUROCONTROL-FRN-Slots: I063/010 = **SDPS**-Identität
> (FRN 1), I063/030 (FRN 3), **NEU** I063/050 = **Sensor**-Identität (FRN 4),
> I063/060 (FRN 5) → FSPEC `0xB8`, 9-Oktett-Records. Der Decoder liest die
> Sensor-Identität aus I063/050.

Jeder Record trägt FSPEC `0xB8` (FRN 1 + 3 + 4 + 5), I063/060 ist variabel (FX).

| Byte | Inhalt |
|------|--------|
| 0 | FSPEC `0xB8` |
| 1–2 | I063/010 — SAC/SIC des **SDPS** (25/2 wie I062/010) |
| 3–5 | I063/030 — Time of Day, 24 Bit, 1/128 s seit UTC-Mitternacht |
| 6–7 | I063/050 — SAC/SIC des **Sensors** (SAC 0, SIC = `sensor_id`) |
| 8 | I063/060 — CON-Byte: `0x00`=operationell, `0x40`=degradiert, `0x80`=Initialisierung, `0xC0`=nicht verbunden (variabel via FX) |

**Operationell:** `(CON & 0xC0) == 0x00` → `SensorStatus.Operational = true`. Die
Sensor-Identität steht in `SensorStatus.SAC`/`.SIC` (aus I063/050), die
SDPS-Identität in `.SDPSSAC`/`.SDPSSIC` (aus I063/010).

**Vorwärtskompatibilität.** Der Decoder kennt die Längen-Regeln der übrigen
Standard-Items (I063/015, I063/070–092) und überspringt das Reserved-Expansion-
(RE) und Special-Purpose-(SP)-Feld längen-bewusst — so bricht er nicht, wenn eine
spätere ICD den per-Quelle-Fehlergrund im RE-Feld nachreicht (Fireflys ADR 0033).

### Robustheit

- Längen-Prüfung vor jedem Byte-Zugriff.
- `LEN < 3` (kein Record-Teil): leeres Slice zurückgegeben, kein Fehler.
- Falscher CAT-Wert: sofortiger `DecodeError`.
- Alle Fehlerpfade geben `error`, kein Panic.

### Tests (`pkg/cat063/decoder_test.go`, 9 Tests)

Alle gegen byte-genaue Referenz-Vektoren aus Fireflys ICD §9 (gleiche
Vektoren wie Fireflys eigene Encoder-Tests):

| Test | Beschreibung |
|------|-------------|
| `TestDecodeSingleOperational` | 1 Sensor, NOGO=0x00 → `Operational=true` |
| `TestDecodeTwoSensors` | 2 Sensoren, zweiter NOGO=0x40 → `false` |
| `TestDecodeTimeOfDay` | ToD = 3600 s → `0x070800` Ticks |
| `TestDecodeDegradedSensor` | NOGO=0x40 → `Operational=false` |
| `TestDecodeNotConnected` | NOGO=0x80 → `Operational=false` |
| `TestDecodeEmptyBlock` | LEN=3 (nur Header) → leeres Slice, kein Fehler |
| `TestDecodeWrongCategory` | CAT≠0x3F → Fehler |
| `TestDecodeTruncatedInput` | Alle Längen < Minimum → Fehler |
| `TestDecodeLENExceedsData` | LEN > len(data) → Fehler |

---

## WF-2 — Registry-Verdrahtung

### `pkg/health.Registry.RecordSensors`

```go
func (r *Registry) RecordSensors(feedID int64, active, total int)
```

Schreibt `active`/`total` in den `feedEntry` des angegebenen Feeds (legt ihn
ggf. an). `Snapshot()` liest die Werte aus und füllt
`FeedSnapshot.SensorsActive`/`.SensorsTotal`.

### `pkg/receiver.Receiver`

- Neues Feld `sensorStatusHandler func([]cat063.SensorStatus) error`.
- `Config.SensorStatusHandler` (nil → no-op default).
- `dispatch()` bekommt `case cat063.Category` → `handleSensorStatus`.
- `handleSensorStatus` zählt aktive Sensoren und ruft den Handler auf.

### `cmd/wayfinder/feeds.go` — `buildReceivers`

Fünftes Argument `sensorStatusHandler func(int64, []cat063.SensorStatus) error`.
Für jeden Receiver eine Closure, die `feedID` per Capture an den Handler weitergibt.

---

## WF-3 — Broadcast-Pfad (Option B)

### Architektur-Entscheidung: Option B

Statt des bisherigen globalen Aggregat-Status wird ein **per-Feed-Snapshot** an
genau die Clients gesendet, die den jeweiligen Feed abonniert haben:

```go
broadcastFeedSnapshot := func(feedID int64, snap health.FeedSnapshot) {
    _ = broadcaster.Send(broadcast.Message{
        FeedStatus: &broadcast.FeedStatusMessage{
            FeedID:        feedID,
            Color:         snap.Color(),
            SensorsActive: snap.SensorsActive,
            SensorsTotal:  snap.SensorsTotal,
        },
    })
}
```

Der `Broadcaster` liefert eine `FeedStatusMessage` nur an Clients, deren
`Scope.AllowsFeed(feedID)` wahr ist (FeedID = 0 geht an alle → single-tenant
fallback). Das ist derselbe Scoping-Mechanismus wie für Track-Delivery.

### Auslöser

- **CAT065-Heartbeat** → `statusHandler` → `feedRegistry.RecordHeartbeat` →
  `broadcastFeedSnapshot`
- **CAT063-Block** → `sensorStatusHandler` → `feedRegistry.RecordSensors` →
  `broadcastFeedSnapshot`
- **Stale-Monitor** → iteriert alle Feeds und broadcastet je Feed-Snapshot

### Frontend (`index.html` + `app.js`)

- Neues CSS `#feed-status.yellow` (Hintergrund `#b45309`).
- `state.feedStatus: Map<feedId, {color, sensorsActive, sensorsTotal}>`.
- Schlimmste Farbe über alle Feeds bestimmt das Banner
  (`colorRank = {red:2, yellow:1, green:0}`).
- Gelbes Banner: `"▲ SENSOR AUSFALL — Feed X: N/M Radare"`.
- Rotes Banner: `"▲ FEED STALE — kein Heartbeat"`.
- Grünes Banner: `"● FEED OK"`.

---

## Qualitäts-Gates

- `go test ./...` ✅ (alle 21 Pakete, inkl. `pkg/cat063`)
- `go vet ./...` ✅
- `gofmt` ✅
- 9 byte-genaue Referenz-Vektor-Tests ✅
- `feeds_test.go` an neues 5-Argument-Interface angepasst ✅
