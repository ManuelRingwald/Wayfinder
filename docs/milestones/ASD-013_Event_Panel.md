# ASD-013 — Alarm-/Ereignis-Panel

> **Kontext:** ASD-Kern-Sicht-Feature aus der ROADMAP. **Rein Frontend, kein
> CAT062-Bezug** — jedes protokollierte Signal existiert bereits im WS-Strom, den
> Wayfinder ohnehin konsumiert. Keine neuen Env-Variablen, keine Wire-Änderung.

## Fachlich — warum

Der Lotse braucht eine chronologische Liste betrieblich relevanter
**Zustandsübergänge**, die sonst leicht untergehen, weil sie flüchtig sind:

- **Feed fällt aus / degradiert / erholt sich** — „habe ich noch ein Lagebild?"
- **Verbindung zum ASD-Server verloren / wiederhergestellt** — dito, auf
  Transport-Ebene.
- **Track erschienen / beendet** — was ist neu, was ist weg.

Ein Ereignis-Panel macht diese Übergänge nachvollziehbar (Situationsbewusstsein)
und ergänzt die reine Momentaufnahme der Karte um eine kurze Historie.

## Technisch — wie

### Reine Ableitung (`frontend/src/map/events.js`)
Bewusst **seiteneffektfrei** (kein `Date`, kein Store) und damit isoliert
testbar:

| Funktion | Quelle | Ereignis(se) |
|----------|--------|--------------|
| `feedStatusEvent(prev, curr)` | Aggregat-`feedStatus` (asd-Store) | `feed-stale` (error) / `feed-degraded` (warn) / `feed-recovered` (success) |
| `connectionEvent(prev, curr)` | WebSocket-Lebenszyklus | `connection-lost` (error) / `connection-restored` (success) |
| `trackLifecycleEvents(prev, curr, ended)` | Live-Track-Set-Diff + I062/080 TSE | `track-appeared` / `track-disappeared` (info) |

`SEVERITY_META` bildet jede Severity (`info`/`warning`/`error`/`success`) auf
Icon + Vuetify-Farbe ab (eine Quelle für Panel und Badge). Zwei bewusste
**Rausch-Grenzen** stecken in der Semantik: `feedStatusEvent` unterdrückt den
gutartigen Anfangs-Aufstieg `unknown → ok` (sonst „wiederhergestellt" bei jedem
Reconnect); `trackLifecycleEvents` meldet „beendet" **nur** für per TSE
(I062/080) explizit gelöschte Tracks — ein bloßer Scan-Ausfall (eine
Track-Nummer fällt ohne TSE weg) erzeugt **kein** Ereignis.

### Ring-Puffer-Store (`frontend/src/stores/events.js`)
Pinia-Store mit beschränktem Puffer (`MAX_EVENTS = 200`, neueste zuerst). `add`
stempelt eine monotone `id` + Wall-Clock-`ts` und trimmt; `addMany` für Batches;
`unseenCount` speist das Glocken-Badge; `markSeen`/`clear` für die Bedienung.
Weil jeder WS-(Re)Connect den Strom neu skopiert, ist der Ereignis-Log
**implizit mandanten-gescopt** (WF2-21) — kein Extra-Aufwand.

### Engine-Verdrahtung (`frontend/src/map/engine.js`)
Im WS-Handler:
- **Feed:** `feedStatus` **vor** `setFeedHealth` merken, danach vergleichen →
  `feedStatusEvent`.
- **Track:** `recordTrackEvents(msg)` leitet aus dem rohen Batch (live vs.
  `ended`) die Lebenszyklus-Ereignisse ab — **unabhängig vom Map-Load-Zustand**
  (kein Verlust während des Style-Ladens). Die **erste Frame nach einem
  (Re)Connect primet** nur die Baseline (`trackEventsPrimed`), damit das
  Anfangsbild den Log nicht mit „erschienen" flutet.
- **Connection:** in den Open-/Close-Handlern → `connectionEvent`; der erste
  Connect ist still.

### UI (`EventPanel.vue` + `AsdView.vue`)
Eine **Glocke** oben rechts (im Top-Right-Cluster) mit **Ungesehen-Badge**
toggelt ein schwebendes Panel; Öffnen setzt den Log auf „gesehen". Das Panel
listet die Ereignisse neueste-zuerst (Severity-Icon/-Farbe, Meldung, lokale
Uhrzeit), mit „Leeren"-Aktion und Leer-Zustandshinweis.

## Ehrliche Grenze

**Keine Wire-Alarme.** STCA/Militär/Hostile-Alarme haben mangels Feld kein
Vorbild im aktuellen CAT062-Strom (vgl. ASD-006/#18, das Fireflys I062/340
konsumieren würde) — das Panel protokolliert ausschließlich **beobachtbare
Zustandsübergänge**, keine erfundenen Alarme. Track-Lebenszyklus-Ereignisse sind
info-level und können auf dichten Feeds häufig sein; der Puffer ist gedeckelt.

## Schnittstellen-Wirkung

**Keine** am CAT062. Reines Frontend; `dist` neu gebaut und eingebettet.

## Tests

- **`frontend/src/map/__tests__/events.test.js`** — Ableitung: Feed-/
  Connection-Transitionen (inkl. unterdrückter Anfangsfälle), Track-Diff/TSE
  (kein „beendet" ohne TSE), Severity-Meta vollständig.
- **`frontend/src/stores/__tests__/events.test.js`** — Ring-Puffer:
  Prepend/Stempel/unseen, Cap bei `MAX_EVENTS`, `addMany`-Reihenfolge,
  `clear`/`markSeen`.
- **`frontend/src/components/__tests__/eventPanel.test.js`** — Panel-/AsdView-/
  Engine-Verdrahtung (Source-Guard, Projektkonvention).

Gates grün: **vitest 485**, `vite build` + eingebettetes `dist` neu; Go
unberührt (`go build ./...` grün).
