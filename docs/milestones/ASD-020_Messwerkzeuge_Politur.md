# ASD-020 — Messwerkzeuge-Politur: Tooltips, Label-Pick + Highlight, Track-Following

> **Register:** FR-UI-041 · FR-UI-042 · FR-UI-043 · **Entscheidung:** kein ADR
> (reine ASD-Chrome-/Interaktions-Politur, keine Architektur-Weiche, kein
> CAT062-Bezug) · **Auslöser:** Betreiber-Wünsche 2026-07-19 (GitHub #296/#298
> Wünsche, #297 Bug) rund um die Messwerkzeuge RBL/DIST/QDM.

## In normaler Sprache — was sich sichtbar ändert

Die Messwerkzeuge am linken Rand des Lagebilds (RBL, DIST, QDM) bekommen drei
Verbesserungen, die das Messen im Alltag deutlich runder machen:

1. **Tooltips (#296).** Fährt der Lotse mit der Maus über eines der drei
   Werkzeuge, erscheint ein kurzer Erklärtext — z. B. „QDM — Peilung von einem
   Track zu einem beliebigen Punkt". Die kryptischen Kürzel sind damit auch
   ohne Vorwissen verständlich, **bevor** man klickt.

2. **Track per Label wählen + Markierung (#298).** Um einen Track zu messen,
   musste man bisher genau das **kleine Symbol** treffen. Jetzt genügt ein Klick
   auf den **Datenblock (das Text-Label)** daneben — das größere, natürlichere
   Ziel. Der gewählte Track wird zusätzlich mit einem **cyan Ring** markiert, so
   sieht der Lotse sofort, worauf seine Messung zeigt.

3. **Messlinie folgt dem Track (#297).** Vorher blieb die Messlinie am
   Ausgangspunkt kleben: bewegte sich der gemessene Track weiter, driftete er von
   der Linie weg und die Zahl stimmte nicht mehr. Jetzt **folgt** der Messpunkt
   dem Track — Linie und der Distanz-/Peilungs-Wert laufen live mit.

## Fachlich — welches Problem das löst

Ein Messwerkzeug im ASD ist ein Lotsen-Grundwerkzeug (Abstand/Peilung schnell
abgreifen). Drei Reibungspunkte standen dem im Weg: die Kürzel waren nicht
selbsterklärend; das Klickziel (nacktes Symbol) war zu klein und ungewohnt (die
normale Selektion konnte das Label längst, #271 — nur im Werkzeug-Modus fehlte
es); und — am gravierendsten — eine Messung auf einen **bewegten** Track war
schlicht falsch, weil sie an der Startkoordinate einfror. Für ein Lagebild, in
dem sich alles bewegt, ist „die Linie folgt dem Track" die eigentlich erwartete
Grundfunktion.

## Technisch

### #296 — Tooltips (`NavigationRail.vue`)
- Jeder `measureTools`-Eintrag trägt ein neues `description`-Feld (ein Satz je
  Werkzeug).
- Desktop-Rail: `<v-tooltip activator="parent" location="right">` als Kind des
  `.nav-rail__btn`; mobiler Drawer: derselbe Tooltip `location="bottom"` am
  `v-btn` (Haus-Konvention wie `ZoomControls`/`ViewProfileMenu`). Auslösung per
  Hover **und** Tastatur-Fokus (A11y).
- Abgrenzung: der `hint` (tools store) bleibt die Bedien-Anweisung **während**
  ein Werkzeug scharf ist; der Tooltip erklärt **vor** der Auswahl.

### #298 — Track per Label-Klick + Highlight (`measure.js`)
- `trackAt()` fragt jetzt `TRACKS_LAYER_ID` **und** `LABELS_LAYER_ID` ab. Ein
  Treffer (Symbol oder Label) wird über `track_num` aufgelöst; die Position ist
  bewusst die **echte Symbol-Position** (Live-Position aus `liveById`, sonst die
  Symbol-Geometrie) — nicht die versetzte Label-Geometrie, sonst säße der
  Messpunkt am Datenblock statt am Flugzeug.
- Ein Endpunkt, der ein Track ist, bekommt `role='track'`; die neue Ebene
  `measure-track-highlight` (hohler cyan Ring, r=11) markiert genau diese
  Endpunkte. Bewusst **Ring** (nicht der Corner-Bracket-Kasten der normalen
  Selektion) und **cyan** (nicht amber SPI / magenta Suche) → auf einen Blick als
  „Mess-Anker" lesbar, ohne mit einer normalen Selektion verwechselt zu werden.
- Der Werkzeug-Guard aus #271 (`AsdView.onTrackClick`, kein Detail-Panel im
  Werkzeug-Modus) bleibt unberührt — der Label-Pick lebt vollständig im
  Mess-Controller, nicht im Selektionspfad.

### #297 — Track-Following (`measure.js` + `engine.js` + `MapCanvas.vue`)
- Endpunkte sind jetzt `{ lng, lat, trackNum }`; `trackNum=null` = freier Punkt.
- Neuer, entkoppelter Engine-Callback **`onTracks(liveTrackFeatures)`** (Spiegel
  zu `onTrackClick`/`onEmptyClick`), gefeuert nach jeder Track-Charge **und** beim
  Pending-Flush auf `load`. `MapCanvas` reicht ihn an `measure.refreshTracks()`.
- `refreshTracks(features)` baut `liveById` (track_num → Position) neu auf und
  zieht jeden track-referenzierten Endpunkt auf die Live-Position nach; nur bei
  echter Änderung wird neu gerendert. Freie Punkte bleiben fest.
- **Edge-Case:** Ist der Track nicht mehr im Anzeige-Satz (TSE / Scope-Austritt),
  fehlt er in `liveById` → der Endpunkt behält die letzte Position (eingefroren),
  konsistent zur `refreshSelectedTrack`-Konvention (FR-UI-029).
- #297 und #298 teilen sich die **Track-Referenz** als gemeinsame Basis (deshalb
  in einem Zug umgesetzt).

## Tests

- `railTools.test.js` (#296-Block): jedes Werkzeug hat eine `description`; der
  Tooltip ist an die Description gebunden (Haus-Konvention `activator="parent"`).
- `measureTrack.test.js` (neu, verhaltensbasiert über einen Fake-Map):
  - #298 — DIST pickt den Track am **Symbol** und ringt ihn (`role='track'`);
    QDM löst einen **Label**-Klick auf denselben Track auf; der QDM-Zielpunkt
    bleibt ein **freier** Punkt (kein Ring).
  - #297 — der Endpunkt **folgt** der Live-Position bei der nächsten Charge und
    **friert** ein, wenn der Track den Anzeige-Satz verlässt.
  - Wiring-Guards: `measure.js` fragt beide Layer ab + exponiert `refreshTracks`;
    `engine.js` feuert `onTracks`, `MapCanvas` leitet es an den Controller.
- `measureLabel.test.js` (unverändert grün): schwebender Readout am Messstrich.
- Ergebnis: `vitest run` **657 grün** (54 Dateien), `vite build` grün, `dist` neu
  gebaut.

## Betriebs-/Doku-Prüfung (Qualitäts-Gates)

- **INSTALLATION.md / TECHNICAL.md:** keine Änderung — keine neuen
  Umgebungsvariablen, Startbefehle, Metriken oder Betriebsmodi (reine
  Frontend-Interaktion).
- **CAT062-Vertrag:** unberührt — keine Schnittstellen-Wirkung auf Firefly.

## Ehrliche Grenzen

Kein WebGL-/Mount-Harness in der Sandbox: Struktur/Verdrahtung sind per
Source-Guards und einem verhaltensbasierten Fake-Map-Test gezurrt, die **optische
Abnahme** (Tooltip-Text, cyan Ring am gewählten Track, Linie folgt dem bewegten
Track, Readout läuft mit) macht der Betreiber nach `git pull` + Frontend-Rebuild.
Der Highlight-Ring ist bewusst schlicht (ein Ring, keine animierten Brackets);
eine reichere Mess-Anker-Darstellung wäre optionaler Feinschliff.
