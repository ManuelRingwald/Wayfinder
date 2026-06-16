# ASD-005 — Höhen- und Filter-Tools

**Datum:** 2026-06-16
**Anforderung:** FR-ASD-005
**Dateien:** `internal/webui/static/index.html`, `internal/webui/static/app.js`
**Komplexität:** S2 · Sonnet 4.6

---

## Fachliche Motivation

Der Lotse arbeitet in einem definierten Verantwortlichkeitsbereich (z. B. TMA,
FL 50–195). Traffic außerhalb dieses Bereichs ist für ihn zwar prinzipiell
sichtbar, aber nicht handlungsrelevant. Ohne Filter füllt sich das Scope-Bild mit
Überflugverkehr und ablenkenden Tracks. Der FL-Filter erlaubt es, Traffic
außerhalb des eigenen FL-Bereichs entweder auszublenden (keine Ablenkung) oder
grafisch abzustufen (entsättigen / dimmen), sodass er nicht in die Konfliktanalyse
einfließt, aber bei Bedarf noch abgelesen werden kann.

Tracks mit **unbekannter FL** (I062/136 nicht vorhanden) passieren den Filter
immer — sie nicht zu zeigen wäre operativ riskant.

---

## Technische Umsetzung

### UI-Panel (`index.html`)

Erweiterung des bestehenden `#layer-control`-Panels um einen neuen Abschnitt:
- Zwei `<input type="number">` für Min-FL und Max-FL (FL-Einheiten, z. B. `50`
  = FL050, `195` = FL195).
- Checkbox „Ausblenden": umschalten zwischen Entsättigen (dim) und Verbergen.

### Filter-Logik (`app.js`)

```
state.flFilter = { minFL: null, maxFL: null, hide: false }
```

**`isFlFiltered(flightLevelFt)`** — gibt `true` zurück wenn die FL bekannt und
außerhalb [minFL, maxFL] liegt. Null-Grenzen werden ignoriert.

**`flOpacity(flightLevelFt)`** — gibt den `fl_opacity`-Wert zurück:
- Nicht gefiltert → `undefined` (Property nicht gesetzt, CoAlesce-Fallback)
- Gefiltert + hide → `0.0` (unsichtbar)
- Gefiltert + dim → `0.15` (stark gedimmt)

**`setupFlFilter()`** — verdrahtet die Inputs mit `state.flFilter` und ruft bei
jeder Änderung `renderSources()` auf, sodass die Filteränderung sofort
sichtbar ist, ohne auf das nächste WSS-Update zu warten.

### Integration in `renderSources()`

`fl_opacity` wird bei jedem `renderSources()`-Aufruf frisch berechnet — nicht
beim Empfang des WSS-Updates — damit Schieberbewegungen des Benutzers sofort
wirken. Dafür ist `flight_level_ft` nun in den precomputed `liveTrackFeatures`
gespeichert.

Das `filtered`-Boolean wird als Feature-Property auf Track-Symbol-Features
gesetzt (für die `circle-color`-Expression, die gefilterte Tracks blau-grau
einfärbt).

### Priorität der Opacity-Expressions

Alle fünf Track-Layer prüfen in dieser Reihenfolge:

1. `["has", "fade_opacity"]` → TSE-Fade (höchste Priorität, ASD-004c)
2. `["has", "fl_opacity"]` → FL-Filter (ASD-005)
3. `["get", "coasting"]` → Coasting-Dimming (ASD-004b)
4. Normalwert

### Opacity-Werte

| Layer | Gefiltert + dim | Gefiltert + hide |
|-------|-----------------|-----------------|
| Track-Kreis | 0.15 | 0.0 |
| Track-Label | 0.15 | 0.0 |
| Speed-Vector | 0.15 | 0.0 |
| Trail-Linie | 0.15 | 0.0 |
| History-Dot | 0.15 | 0.0 |

---

## Schnittstellen-Wirkung

Keine. Ausschließlich Frontend; kein neuer Backend-Endpoint, keine CAT062-Änderung.

---

## Qualitäts-Gates

- `go test ./...` ✅ (kein Go-Code geändert)
- `go vet ./...` ✅
- Frontend: manuell gegen Firefly-CAT062-Feed verifiziert (Min/Max-Filter wirkt
  sofort, Ausblenden/Entsättigen-Toggle korrekt, unbekannte FL passiert Filter).
