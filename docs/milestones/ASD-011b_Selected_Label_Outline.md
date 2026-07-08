# ASD-011b — Selektions-Umrandung des Datenblock-Labels

> **Kontext:** Angleichung an das **Claude-Design-Template** (FR-UI-023: der
> Export ist verbindlich). Im Designkonzept ist das **selektierte** Label
> umrandet; heute hebt Wayfinder nur das **Symbol** hervor (Halo-/Bracket-Box,
> FR-ASD-007/#183). **Rein Frontend, kein CAT062-Bezug.** Keine neuen
> Env-Variablen.

## Fachlich — warum

Bei der Selektion soll der Lotse Symbol **und** zugehörigen Datenblock als
zusammengehörig „ausgewählt" erkennen — besonders in dichter Lage, wo das Label
über eine Leader-Linie versetzt sitzt. Die Umrandung des Blocks macht die
Auswahl am Text eindeutig, ergänzend zur Symbol-Hervorhebung.

## Technisch — wie

- **Farbe/Stil (Betreiber-Entscheidung 2026-07-08):** **neutraler Hellton**
  `#f2f7fc`, leicht **abgerundete** Ecken — distinkt zu Cyan-Chrome und den
  Zustands-/Alarmfarben (rot/amber), damit „Selektion" nicht mit „Alarm"
  verwechselt wird.
- **Geometrie:** `deconflictLabels` kennt die Label-Screen-Bbox bereits
  (`LABEL_W_PX`/`LABEL_H_PX` um `(lx, ly)`). Für den selektierten Track wird
  daraus mit Padding ein **abgerundeter Ring** erzeugt — reine, testbare
  `roundedRectRing(cx, cy, halfW, halfH, r, segsPerCorner)` (Ecken als kurze
  Arcs, geschlossener Ring). Jeder Ringpunkt wird per **`map.unproject`**
  zurück nach Geo projiziert, sodass die Box **pixelgenau** um das Label sitzt
  (derselbe exakte Round-Trip, der auch den Label-Drag-Fix trägt) und ihm bei
  jedem Render folgt.
- **Ebene:** eigene Line-Ebene `SELECTION_LABEL_LAYER_ID` (`line-join`/`-cap:
  round` für die weichen Ecken), registriert **über** der Labels-Ebene, damit die
  Box den Text rahmt. Die Quelle trägt **0 oder 1** Feature (nur das selektierte
  Label). `render.js` reicht `selectedTrackNum` durch und setzt die Quelle;
  `deconflictLabels` gibt `selectionBoxFeatures` zusätzlich zurück.

## Zuschnitt / ehrliche Grenze

Umgesetzt ist **nur** die Selektions-Umrandung (der konkrete Wunsch). Die
breitere Design-Angleichung bleibt **bewusst** in separaten Häppchen:

- **Jedes** Label boxen (nicht nur das selektierte) — reiner Sicht-Reskin.
- **Alarm-/Zustands-Rahmen** STCA (rot), EMG (rot), DUP (amber): STCA bräuchte
  **Wire-Daten** (I062/340), die Wayfinder noch nicht konsumiert (vgl. ASD-006/
  Firefly #18); EMG (Squawk 7700) und DUP (doppelter Squawk) wären client-seitig
  ableitbar, aber eigenständige Schritte.

## Schnittstellen-Wirkung

**Keine** am CAT062. Reines Frontend; `dist` neu gebaut und eingebettet.

## Tests

- **`frontend/src/map/__tests__/deconflict.test.js`:**
  - `roundedRectRing` — geschlossener Ring, Extrema erreichen die (gepolsterte)
    Rechteck-Grenze, Radius-Clamping auf die kürzere Seite.
  - `deconflictLabels selection outline (ASD-011b)` — kein Box-Feature ohne
    Selektion; **nur** der selektierte Track wird umrandet; die Box rahmt die
    Label-Bbox **exakt** (Round-Trip über die Mock-`project`/`unproject`).

Gates grün: **vitest 489**, `vite build` + eingebettetes `dist` neu; Go
unberührt (`go build ./...`).
