# CAT065-NOGO sichtbar machen — „Dienst degradiert" (#261, Fireflys SAFE.4)

> **Kurz:** Ein hängender Tracker sendet seit Fireflys SAFE.4 weiter CAT065-
> Heartbeats, flaggt sich darin aber **NOGO** (degradiert, I065/040). Wayfinder
> hat das NOGO-Bit zwar dekodiert, im Konsumenten aber **verworfen** — ein
> degradierter, aber lebender Feed blieb **grün/unsichtbar**. Jetzt treibt ein
> NOGO-Heartbeat die Feed-Ampel auf **gelb** und der Chip liest **„DIENST
> DEGRADIERT"**. Rückverfolgbarkeit: **FR-OPS-009**.

## Fachlich — welches Problem löst es für den Lotsen?

Ein „gesunder" Heartbeat und ein „hängender Tracker, der noch Heartbeats sendet"
sahen für den Lotsen bisher **gleich** aus (beide grün). Fireflys FHA (SAFE.4)
schloss die Sende-Lücke: Bleibt der Rechen-Tick des Trackers zu lange aus, meldet
der Heartbeat **NOGO/degradiert** statt operationell. Damit der Lotse das auch
**sieht**, zeigt Wayfinder einen NOGO-Feed jetzt als **degradiert** an — das
Lagebild dahinter kann eingefroren oder unvollständig sein, und das ist am
Feed-Chip sofort erkennbar (nicht erst, wenn der Feed ganz still wird).

## Technisch — wie umgesetzt?

**Vorher:** `cmd/wayfinder/main.go`'s `statusHandler` rief
`feedRegistry.RecordHeartbeat(feedID, now)` **unabhängig** vom NOGO-Bit auf;
`FeedSnapshot.Color()` wurde nur bei CAT063-**Sensor**ausfall gelb. Der CAT065-
`ServiceStatus.Operational` (bereits korrekt dekodiert, `b[0]&0xC0==0`) wurde nie
ausgewertet.

**Nachher:**
- `pkg/health/registry.go` — `RecordHeartbeat(feedID, now, operational)` führt pro
  Feed ein `sdpsDegraded`-Flag; `FeedSnapshot.SdpsDegraded` treibt `Color()` auf
  **`yellow`** (zusätzlich zur bestehenden Sensor-Gelb-Semantik). Der Heartbeat
  setzt weiterhin die Staleness-Uhr zurück — der Feed **lebt**, ist nur degradiert.
- `pkg/broadcast/broadcast.go` — `FeedStatusMessage.SdpsDegraded`
  (`json:"sdps_degraded,omitempty"` → gesunder Draht byte-unverändert).
- `cmd/wayfinder/main.go` — `status.Operational` wird durchgereicht und ins
  Snapshot/die Nachricht übernommen.
- Frontend: `stores/asd.js` (`setFeedHealth(..., sdpsDegraded)` +
  `feedSdpsDegraded`-Getter), `map/engine.js` (WS-Feld `sdps_degraded`),
  `components/FeedStatusChip.vue` — bei SDPS-NOGO Label **„DIENST DEGRADIERT"** +
  eigener Tooltip (dienst- statt sensor-bezogen; hat Vorrang vor „SENSOR AUSFALL").

**Kein Wire-Vertragsbruch:** I065/040 NOGO ist seit ICD 2.3.0 spezifiziert und
wurde bereits dekodiert — dieser Schritt **verwertet** ihn nur (kein ICD-Bump;
Fireflys ICD-Bump 3.7.1 war rein dokumentarisch).

## Verifikation
- **Backend:** `pkg/health` — `TestRegistrySdpsNogoIsYellowButNotStale` (NOGO →
  `SdpsDegraded`, „yellow", **nicht** stale; operationeller Folge-Heartbeat → grün).
  Volle `go test ./...`, `go vet`, `gofmt`, `golangci-lint` (0 issues) grün.
- **Frontend:** `feedStatusChip.test.js` (+SDPS-Label/-Vorrang), `asd.test.js`
  (`feedSdpsDegraded`-Flag + Reset). 755 Tests grün. Dist neu gebaut/eingebettet.
