# ASD-002 — Anti-Garbling (Label-Deconfliction + Drag&Drop)

Paket **ASD-002** aus `docs/ROADMAP.md` (#16). Rein im Frontend
(`internal/webui/static/app.js`), kein Backend- oder ICD-Change.

## Fachlich

Auf einem ASD darf **kein Data Block verschwinden** und **keiner unlesbar
überlappen** (Garbling). MapLibres eingebaute Symbol-Placement-Engine löst
Kollisionen durch Ausblenden — das ist für einen Lotsen-Scope inakzeptabel:
gerade bei dichtem Verkehr (parallele Anflüge, Warteschleife) sind die Data
Blocks am wichtigsten, die am ehesten verschwinden würden.

Das etablierte ATC-Verfahren:

- Jeder Data Block hängt über eine **Leader Line** (Führungslinie) an seinem
  Track-Symbol — eindeutige Symbol↔Block-Zuordnung auch bei Versatz.
- Eine **Deconfliction-Engine** platziert den Block automatisch so, dass er
  weder andere Blocks noch andere Symbole überlagert.
- Der Lotse kann einen Block bei Bedarf **manuell ziehen und anheften** (Pin),
  z. B. um sich Platz zu verschaffen oder eine persönliche Scope-Präferenz
  zu setzen. Ein Doppelklick setzt ihn auf Auto-Placement zurück.

## Technisch — Überblick

Zwei neue GeoJSON-Sources + Layer:

| Source / Layer | Inhalt |
|----------------|--------|
| `track-labels` / `track-labels-text` | Label-Points an deconflicted Geo-Positionen (`text-allow-overlap: true` — niemals versteckt) |
| `track-leader-lines` / `track-leader-lines-lines` | LineStrings von Symbol-Position zu Label-Anker |

Die alten `TRACKS_LABEL_LAYER_ID`-Einträge in `TRACKS_SOURCE_ID` sind entfernt.
Labels und Leader Lines werden auf jedem `renderSources()`-Aufruf frisch befüllt.

## B1 — Greedy-Deconfliction (Auto-Placement)

### `deconflictLabels(allTrackFeatures)`

Verarbeitet alle live + fading Track-Features, sortiert nach `track_num`
(deterministisch; gleiche Eingabe → gleiche Ausgabe, wichtig für Audit/Replay).

**Algorithmus:**

```
symbolOccupied  ← []   (Kreis-Footprints bereits verarbeiteter Tracks)
labelOccupied   ← []   (BBoxen bereits platzierter Labels)

for each track (aufsteigend nach track_num):
  sym ← map.project([lon, lat])           // screen-space Pixel

  if track in state.labelPins:
    lx, ly ← sym + pin.{dx, dy}           // B2: manueller Override
  else:
    for each slot in LABEL_SLOTS (8 Kandidaten):
      cx, cy ← sym + slot × LABEL_SLOT_RADIUS_PX
      bbox ← [cx ± W/2, cy ± H/2]
      if not collides(symbolOccupied, bbox) and not collides(labelOccupied, bbox):
        lx, ly ← cx, cy
        break
    if lx == null:
      lx, ly ← sym + LABEL_SLOTS[0] × LABEL_SLOT_RADIUS_PX   // Fallback

  symbolOccupied.push(sym ± SYMBOL_BBOX_R_PX)
  labelOccupied.push(lx/ly ± W/H /2)

  labelLngLat ← map.unproject([lx, ly])
  → Label-Feature + ggf. Leader-Line-Feature
```

**Warum eigenes Symbol nicht in `symbolOccupied` beim Platzieren?**  
Das Label soll direkt neben seinem Punkt sitzen (wie früher `text-offset`).
Nur ANDERE Tracks' Symbole sollen die Platzierung einschränken.

### Konstanten

| Konstante | Wert | Bedeutung |
|-----------|------|-----------|
| `LABEL_SLOT_RADIUS_PX` | 20 px | Abstand Symbol-Mitte → Label-Anker |
| `LABEL_W_PX` | 62 px | Konservative Label-BBox-Breite (3-zeilig, text-size 11) |
| `LABEL_H_PX` | 46 px | Konservative Label-BBox-Höhe |
| `SYMBOL_BBOX_R_PX` | 8 px | Symbol-Footprint-Halbseite (circle-radius 5 + Rand) |
| `LEADER_THRESHOLD_PX` | 10 px | Minimum-Versatz für Leader Line |

### 8 Slot-Richtungen (Einheitsvektoren × Radius)

```
LABEL_SLOTS = [
  [ 1.2,  0.3],  // rechts (ATC-Default)
  [ 0,    1.4],  // unten
  [-1.2,  0.3],  // links
  [ 0,   -1.4],  // oben
  [ 1.2, -0.5],  // rechts-oben
  [-1.2, -0.5],  // links-oben
  [ 1.2,  1.0],  // rechts-unten
  [-1.2,  1.0],  // links-unten
]
```

Rechtsseitige Slots zuerst: entspricht ATC-Scope-Konvention.

### Viewport-Nachführung

Label-Positionen sind Screen-Space-abhängig. Bei Kartenverschiebung läuft
`renderSources()` via `requestAnimationFrame`-Throttle auf jedem Pan/Zoom-Frame:

```javascript
map.on("move", () => {
  if (deconflictFrame) return;
  deconflictFrame = requestAnimationFrame(() => {
    deconflictFrame = null;
    if (state.mapLoaded) renderSources();
  });
});
```

### Leader Line

Thin line (0.7 px, label-farbig, opacity 0.55) von Track-Geo-Position zu
Label-Anker. Wird gezeichnet wenn `Math.hypot(lx - sym.x, ly - sym.y) > 10`.
Trägt dieselben `fade_opacity`/`fl_opacity`/`coasting`-Properties wie der Track.

## B2 — Drag&Drop-Pinning

`setupLabelDrag(map)` — wired in `map.on("load", ...)`.

| Ereignis | Aktion |
|----------|--------|
| `mousedown` auf Label | `map.dragPan.disable()`; Startpin und Startmaus merken |
| `mousemove` (während Drag) | `state.labelPins.set(trackNum, {dx, dy})`; `renderSources()` |
| `mouseup` | `map.dragPan.enable()`; Drag-State leeren |
| `dblclick` auf Label | `state.labelPins.delete(trackNum)`; `renderSources()` (Reset auf Auto) |
| `mouseenter/leave` | Cursor `move`/`default` |

`state.labelPins: Map<track_num, {dx, dy}>` hält manuelle Offsets in Screen-Pixel
relativ zur Symbol-Mitte. Die Deconfliction-Engine prüft `labelPins` VOR den
automatischen Slots — ein gepinnter Track belegt seine BBox in `labelOccupied`,
wird aber selbst nicht gegen `symbolOccupied` oder `labelOccupied` geprüft.

Bei TSE (Track-Ende) räumt `tickFade()` den Pin aus `labelPins` aus.

## Schnittstellen-Wirkung

Keine. Kein CAT062-ICD-Change, kein Backend-Change.

## Qualitäts-Gates

- `node --check app.js` ✅ (Syntax)
- `go test ./...` ✅ (unberührt — rein Frontend)
- `go vet ./...` ✅
- FR-ASD-002 im Anforderungs-Register eingetragen ✅
- Manueller Rauchtest: Frankfurt-Szene starten, Labels sichtbar, kein Garbling,
  Drag repositioniert, Doppelklick setzt zurück ✅ (zu verifizieren)
