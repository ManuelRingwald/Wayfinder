# ASD-004 — Track-Lebenszyklus & History-Darstellung

**Datum:** 2026-06-16
**Anforderung:** FR-ASD-004
**Dateien:** `internal/webui/static/app.js`
**Komplexität:** S3 · Sonnet 4.6

---

## Fachliche Motivation

Ein echtes Air Situation Display unterscheidet drei Aspekte des Track-Lebenszyklus,
die bisher fehlten:

1. **History-Dots** — Vergangene Positionen erscheinen als diskrete Punkte, nicht
   als durchgehende Linie. Der Abstand zwischen zwei aufeinanderfolgenden Punkten
   kodiert die Momentangeschwindigkeit; die Krümmung des Punktebogens kodiert die
   Drehrate. Beides liest der Lotse ohne Messung ab — eine Linie macht diese
   Information unsichtbar.

2. **Coasting-Abdunkeln** — Ein coastender Track (CST-Bit in I062/080) hat seit
   mehreren Scans keinen frischen Plot mehr erhalten; seine Position ist extrapoliert
   und damit unsicherer. Heute sah der Lotse nur den orangen Kreis; Label, Vektor
   und Trail hatten volle Opazität. Das täuschte Aktualität vor.

3. **Graceful Fade-Out bei TSE** — Wenn ein Track endet (TSE-Bit in I062/080 Oktett 2),
   verschwand er bisher augenblicklich. Real-ASD-Systeme zeigen ein kurzes
   Ausblenden (~1,5 s), damit das Verschwinden wahrnehmbar und vorhersehbar ist
   und den Lotsen nicht überrascht.

Alle drei Änderungen sind rein im Frontend (kein Backend-Change, keine
CAT062-Vertragsänderung).

---

## Technische Umsetzung

### 4a — History-Dots

Neuer MapLibre-Source `track-history-dots` (GeoJSON Point-Features) und ein
`circle`-Layer `track-history-dots-circles` (Radius 2 px, Trail-Farbe aus
Palette). In `renderSources()` wird jeder Eintrag aus `state.trackHistory` als
eigenes Point-Feature emittiert, mit `coasting`-Property aus `state.trackCoasting`
und — für fading Tracks — mit `fade_opacity`.

Layer-Reihenfolge von unten: Airspace → **Trail-Linie** → **History-Dots** →
Speed-Vectors → Track-Symbole → Labels.

### 4b — Coasting-Abdunkeln

Alle Track-Layer erhalten datengesteuerte Opacity-Expressions:

```
["case",
  ["has", "fade_opacity"], ["get", "fade_opacity"],   // ASD-004c: TSE-Fade
  ["get", "coasting"], DIM_VALUE,                      // ASD-004b: Coasting
  NORMAL_VALUE
]
```

Dimm-Werte:

| Layer | Normal | Coasting |
|-------|--------|----------|
| `circle-opacity` (Track-Symbol) | 1.0 | 0.5 |
| `text-opacity` (Label) | 1.0 | 0.35 |
| `line-opacity` (Speed-Vector) | 1.0 | 0.35 |
| `line-opacity` (Trail) | 0.6 | 0.2 |
| `circle-opacity` (History-Dot) | 0.6 | 0.2 |

Da `state.trackHistory` keine Track-Metadaten speichert, führt `state.trackCoasting:
Map<track_num, boolean>` den Coasting-Zustand pro Track mit — parallel zu
`trackFlHistory` und `trackHistory`.

### 4c — Graceful Fade-Out bei TSE

**State:**
- `state.fadingTracks: Map<track_num, {deadline: number, track: object}>` —
  Tracks zwischen TSE-Empfang und dem Ende der Fade-Periode.
- `state.liveTrackFeatures`, `state.liveVectorFeatures` — vorberechnete GeoJSON-
  Features für den aktuellen Live-Frame, werden von `renderSources()` wiederverwendet.
- `state.fadeInterval: number|null` — `setInterval`-Handle für die Fade-Schleife.

**Ablauf:**

1. `updateTracksLayer(msg)` extrahiert Tracks mit `ended=true` und trägt sie in
   `fadingTracks` ein (Deadline = `Date.now() + 1500 ms`), bevor die Live-Liste
   gefiltert wird.
2. Alle vier GeoJSON-Sources werden in `renderSources()` befüllt, das Live-Features
   und Fading-Features zusammenführt. Fading-Features tragen `fade_opacity =
   max(0, (deadline - now) / 1500)`.
3. Beim ersten Eintrag in `fadingTracks` startet `updateTracksLayer` den Loop:
   `state.fadeInterval = setInterval(tickFade, 50)`.
4. `tickFade()` entfernt abgelaufene Einträge aus `fadingTracks`, `trackHistory`
   und `trackCoasting`, dann ruft es `renderSources()` auf. Wenn `fadingTracks`
   leer ist, löscht es sich selbst (`clearInterval`).

**History während des Fade:** `updateTrackHistory()` überspringt Tracks in
`fadingTracks` beim Bereinigen, sodass Trail und Dots während der Fade-Periode
sichtbar bleiben und mit dem Track mitverblassen.

---

## Schnittstellen-Wirkung

Keine. Ausschließlich Frontend; kein neuer Backend-Endpoint, keine CAT062-Änderung.

---

## Qualitäts-Gates

- `go test ./...` ✅ (kein Go-Code geändert)
- `go vet ./...` ✅
- Frontend: manuell gegen Firefly-CAT062-Feed verifiziert (History-Dots sichtbar,
  Coasting-Tracks gedimmt, TSE-Tracks ausgeblendet statt sofortigem Verschwinden).
