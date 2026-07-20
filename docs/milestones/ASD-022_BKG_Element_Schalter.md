# ASD-022 (E2) — BKG-Basiskarte: Element-Schalter im „Karte"-Abschnitt

> **Register:** FR-UI-046 · **Bezug:** ADR 0031 (Sidebar-IA), Epic #290, Issue
> #293 (E2). Baut auf **E0** (#291, Bucketing) und **E1** (#292, Gruppen-Rahmen).

## Fachlich — warum

Der ursprüngliche Betreiber-Wunsch: die amtliche Basiskarte **nach Elementen**
ein-/ausblenden — „nur Flüsse", „nur Straßen". Bislang war die Karte ein einziger
Ein/Aus-Schalter (#274). Jetzt kann der Lotse unter „Karte" **jedes Element
einzeln** schalten (Gewässer, Verkehr, Vegetation, Siedlung, Gebäude, Grenzen,
Beschriftung, Hintergrund) und so das Kartenbild auf das reduzieren, was er
gerade braucht. **Damit ist der Feature-Wunsch erstmals live bedienbar.**

## Technisch — Master × Element

Kein neuer Style-Wechsel, sondern Sichtbarkeits-Schaltung je Element-Gruppe (die
E0-Buckets) — kombiniert mit dem bestehenden #274-Master:

- **Store (`asd.js`)**: neues `basemapElementVisibility` (Gewässer/Verkehr/… je
  Boolean, **alle an per Default** — nichts ändert sich, bis der Lotse ein Element
  ausblendet) + `setBasemapElement`. Der #274-Master `layerVisibility.basemap`
  (ist die Karte an?) bleibt **unverändert** — die Elemente sind eine
  **Verfeinerung**, wenn die Karte an ist, sie schalten die Karte nicht ein.
- **Engine (`engine.js`)**: neue Funktion **`applyBasemap()`** kombiniert beides —
  eine Ebene ist sichtbar **gdw. Master an UND ihre Element-Gruppe an**; eine
  unklassifizierte Gruppe (`other`, nicht in der Element-Liste) folgt still dem
  Master. Die Basiskarte wird **nicht mehr** im flachen Show/Hide-Loop von
  `setLayerVisibility` geschaltet, sondern durchgängig über `applyBasemap` (beim
  Load, bei Master-Änderung und bei Element-Änderung).
- **MapCanvas**: neuer Watcher auf `basemapElementVisibility` → `applyBasemap()`
  (Master-Änderungen laufen weiter über den bestehenden `layerVisibility`-Watcher).
- **Sidebar (`LayerFilterContent.vue`)**: im „Karte"-Abschnitt unter der
  „Basiskarte (BKG)"-Zeile acht **eingerückte Element-Schalter** (`BASEMAP_ELEMENTS`),
  **deaktiviert/ausgegraut, solange die Karte aus ist** (ein Element einer
  verborgenen Karte zu schalten wäre sinnlos).

### Die acht exponierten Elemente

Gewässer · Verkehr · Vegetation · Siedlung · Gebäude · Grenzen · Beschriftung ·
Hintergrund. Die `other`-Catch-all-Gruppe (unerkannte Ebenen aus E0) ist
**bewusst nicht** als Schalter exponiert — sie folgt still dem Master, damit ein
Schalter über unvorhersehbaren Ebenen nicht zu Löchern führt.

## „Nur Flüsse" / „nur Straßen"

Karte an → alle Elemente außer dem gewünschten aus. Weil auch **Hintergrund**
schaltbar ist, lässt sich bis auf den reinen Scope-Grund reduzieren („nur die
Flüsse auf Schwarz"). Beschriftung ist ein eigenes Element, sodass Geometrie und
Namen getrennt schaltbar sind.

## Ehrliche Grenzen

- **Persistenz folgt (E4/#295):** `basemapElementVisibility` liegt im Store, wird
  aber noch **nicht** ins View-Profil gespeichert — nach Reload stehen alle
  Elemente wieder auf an. Das ist E4.
- **Feinjustierung am echten Style:** Die Zuordnung Ebene→Element (E0) ist
  muster-basiert; die letzte Abstimmung am echten `/basemap/style.json` macht der
  Betreiber. Unerkannte Ebenen (`other`) bleiben unter dem Master sichtbar.
- **Kein WebGL-/Mount-Harness** → Wiring per Source-Guards; optische Abnahme durch
  den Betreiber.

## Tests

- `basemapGroups.test.js`: `BASEMAP_ELEMENTS` exponiert die sinnvollen Gruppen mit
  Labels, **nicht** `other`; jedes Element ist eine echte Gruppe.
- `basemapLayer.test.js` (E2-Block): Store-Default (alle an) + Setter;
  `applyBasemap` kombiniert Master × Element (unklassifiziert → folgt Master);
  Basiskarte über `applyBasemap` statt flachem Loop; MapCanvas-Element-Watcher;
  Sidebar rendert einen Schalter je `BASEMAP_ELEMENTS`, deaktiviert bei Karte-aus.
- Bestehende #274-Guards angepasst (Load nutzt `applyBasemap`).
