# ASD-001 — Erweiterter Data Block

Paket **ASD-001** aus `docs/ROADMAP.md` (#12). Rein im Frontend
(`internal/webui/static/app.js`), kein Backend-Change.

## Fachlich

Der Lotse braucht neben der Track-Position mindestens vier Informationen
auf einen Blick:

- **Rufzeichen** — wen schaue ich an?
- **Flugfläche** — wie hoch?
- **Vertikaltendenz** — steigt, sinkt oder fliegt er im Level?
- **Bodengeschwindigkeit** — wie schnell (für Staffelungsabschätzung)?

Ohne Geschwindigkeit und Tendenz ist der Data Block für echten Lotsen-
Betrieb zu informationsarm.

## Technisch

Das Backend sendet alle nötigen Felder seit Längerem im WebSocket-JSON:
`callsign` (I062/245), `flight_level_ft` (I062/136), `vx`/`vy` (I062/185).
Die Änderungen beschränken sich auf drei Stellen in `app.js`:

### ASD-001a — Bodengeschwindigkeit

`buildLabel(track, vTrend)` erhält eine dritte Zeile:

```
gs = Math.round(Math.hypot(vx, vy) * 1.9438)   // m/s → kt
```

Wird nur angezeigt, wenn `gs > 0` (unterdrückt geparkte/stehende Scheinziele).

### ASD-001b — Steig-/Sinkflug-Indikator

Neuer State-Eintrag `trackFlHistory: Map<track_num, flight_level_ft>`.

In `updateTracksLayer`: für jeden Track mit bekannter FL wird das Delta zum
Vorgänger-Wert berechnet:

| Delta | Anzeige |
|-------|---------|
| > +50 ft | `▲` |
| < −50 ft | `▼` |
| sonst | (leer) |

Schwellwert 50 ft = 2 LSB der I062/136-Kodierung (LSB = 25 ft). Damit
werden Einzelquantisierungs-Rauschen aus der Mode-C-Enkodierung gefiltert,
echte Steig-/Sinkraten (typisch > 200 ft je 4-s-Scan) aber sicher erkannt.

`trackFlHistory` wird am Ende jedes Updates analog zu `trackHistory`
bereinigt (Einträge verschwundener Tracks werden gelöscht).

### Beispiel-Label (volle Information)

```
DLH123
FL350 ▲
247
```

Oder bei fehlender FL (z. B. primärradar-only Track):

```
DLH123
247
```

Oder bei nur Tracknummer und ohne FL:

```
1042
247
```

### Formatierungsstelle

Alle vier Elemente sind in **`buildLabel(track, vTrend)`** gebündelt —
eine einzige Funktion produziert den gesamten Data Block. `vTrend` wird
von `updateTracksLayer` übergeben.

## Tests / Verifikation

- `node --check app.js` (Syntax) ✅
- `go test ./...` — bleibt unberührt, rein Frontend ✅
- Manueller Rauchtest des laufenden Servers: Data Block zeigt alle vier
  Elemente; `▲`/`▼` erscheint nach zwei aufeinanderfolgenden Scans mit
  Höhendifferenz > 50 ft; Tracks ohne FL zeigen nur Callsign + Speed.
