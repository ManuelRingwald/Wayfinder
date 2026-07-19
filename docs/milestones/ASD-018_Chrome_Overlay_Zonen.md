# ASD-018 — Overlay-Zonen: Bedien-Chrome fließt, statt zu überlappen

> **Register:** FR-UI-039 · **Entscheidung:** ADR 0029 · **Auslöser:**
> Betreiber-Befund beim Mac-Test 2026-07-19 (Such-Lupe überlagert die
> Vollbild-/Zentrieren-Knöpfe — „schon häufiger passiert").

## Fachlich — warum

Der Scope ist der Arbeitsplatz des Lotsen; Bedien-Elemente, die sich
**überlappen**, sind nicht bloß unschön, sie machen Funktionen unerreichbar.
Das Problem trat **wiederholt** auf: Immer wenn neues Chrome an den rechten
Rand kam (Profil-Schalter + Ereignis-Glocke bei #194, zuletzt die Such-Lupe bei
#277), saßen die Karten-Controls plötzlich darauf. Der Betreiber hat den Kern
benannt: Es braucht ein Layout, in dem **festgelegte Bereiche** definieren, wo
neue Funktionen hinkommen — damit sich das nicht wiederholt.

## Technisch — die Bug-Klasse und ihre Wurzel

Am rechten Rand lagen **zwei unabhängig positionierte** Overlay-Stapel:

1. `.top-right-cluster` (`AsdView`) — Header, Feed-Chip, Aktions-Icons, Suche;
   **wächst nach unten**.
2. `.map-controls` (`MapControls`) — Recenter/Vollbild, mit **fest verdrahtetem**
   `top: calc(… + 140px)`, der die **Höhe** von Stapel 1 nur **riet**.

Ein neues Element in Stapel 1 verschob dessen Unterkante; der geratene Offset in
Stapel 2 stimmte nicht mehr → Überlappung. Ein hart kodierter Offset, der die
Größe eines Nachbarn annimmt, **muss** irgendwann brechen — deshalb kam der Bug
immer wieder (#194 flickte nur `100px`→`140px`).

## Lösung — Overlay-Zonen (ADR 0029)

Chrome wird in **Zonen** organisiert: eine Zone = **ein** positionierter
Flex-Container, jedes Element ein **Flex-Kind** im Fluss. Neues schiebt das
Darunterliegende, statt es zu überlagern. (Gleiches Muster wie MapLibres eigene
Control-Container.)

- Die **rechte Kante** ist die erste konsequente Zone: `.top-right-cluster` ist
  die eine Flex-Spalte, die Viewport-Controls sind ihr **letztes Flex-Kind** —
  sie fließen unter alles darüber, egal wie viele Zeilen der Cluster hat. Der
  geratene `top:140px` ist **weg**.
- Recenter/Vollbild stecken in einer **positions-neutralen**
  `ViewportControls.vue` (kein eigener Offset). Die Zone legt sie aus.
- **Mobil** rendert `MapControls` denselben `ViewportControls` in seinem eigenen
  Stapel unten rechts — **kein Doppel-Code**. `MapCanvas` rendert `MapControls`
  nur für `!mdAndUp` und exponiert `recenter` für den Desktop-Rail.

**Verbindliche Regel:** Neues Chrome kommt als Flex-Kind in eine Zone — nie als
frei-positioniertes `absolute`-Element mit geratenem `top`/`right`. Braucht etwas
wirklich eine neue Position, wird eine **Zone** (ein Container) definiert.

## Ehrliche Grenzen

- **Keine visuelle Zusicherung im CI:** Es gibt keinen WebGL-/Mount-Harness; die
  Struktur ist per Source-Guards festgezurrt (`scopeChromeLayout.test.js`), die
  optische Abnahme macht der Betreiber.
- **Transiente Panels schieben (noch) im Fluss:** Das Ereignis-Log und das
  aufgeklappte Suchfeld liegen weiter im Fluss der Zone — beim Öffnen schieben
  sie die Controls nach unten (korrektes Verhalten, keine Überlappung). Sollen
  die Controls dabei völlig ruhig bleiben, werden diese Panels später zu
  **Overlays** der Zone (absolut, aus dem Fluss) — optionaler Feinschliff.

## Tests

- `frontend/src/views/__tests__/scopeChromeLayout.test.js` (neu): ViewportControls
  ist positions-neutral (kein `absolute`/`top`), sitzt als **letztes** Rail-Kind
  (nach der Suche) und ist desktop-only; `MapControls` hat **keinen** Desktop-
  `top`-Offset mehr und ankert unten; `MapCanvas` rendert `MapControls` nur
  `!mdAndUp` und exponiert `recenter`; Desktop + Mobil teilen **eine**
  `ViewportControls` (kein Doppel-Code).
- Angepasst: `responsive.test.js` (Touch-Sizing jetzt an `ViewportControls`,
  Mobil-Positionierung ist Default-`.map-controls`), `railTools.test.js`
  (Mobil-Gate in `MapCanvas`), `mapCanvasViewCenter.test.js` (Recenter-Label an
  `ViewportControls`).
