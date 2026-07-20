# ADR 0031 — Sidebar-Informationsarchitektur: Layer in aufklappbaren Gruppen mit Master

- **Status:** **AKZEPTIERT** ✅ (2026-07-20). Betreiber-Wunsch nach einem
  Konzept, wie die „Fülle an Layer-Optionen" im Sidepanel **sauber gegliedert**
  wird, bevor die BKG-Element-Schalter (#290/#293) die Zahl weiter erhöhen.
- **Datum:** 2026-07-20
- **Schnittstellen-relevant:** nein (reines Browser-Chrome-Layout; kein
  CAT062-/Firefly-Bezug, keine Backend-Wirkung).
- **Bezug:** ASD-020 (Umsetzung, E1). Epic #290 (BKG-Element-Ebenen +
  Sidebar-Gliederung), Issue #292 (E1). Spiegel zu **ADR 0029** (Overlay-Zonen
  am Scope-Rand) — dieselbe Idee für das Sidepanel. Register: **FR-UI-044**.
  Baut auf #176 (flache Airspace-Gruppen), #116 (Rail-Sektionen), #274
  (Basiskarte-Toggle) auf.

## Kontext — die flache Liste skaliert nicht

Der „Layer"-Abschnitt der Sidebar (`LayerFilterContent.vue`, Sektion `layers`)
war eine **flache Liste** von rund einem Dutzend `v-switch`-Zeilen: Basiskarte,
vier Luftraum-Gruppen, AoR, VOR/NDB, Waypoints, Flughäfen, Runways,
Radarabdeckung, DWD-Regenradar (+Legende), DWD-Warnungen (+Legende), History-Dots
(+Dauer), Range-Rings (+Konfig) — dazu die Spurherkunft-Legende. Schon heute ist
das viel; mit den geplanten **BKG-Element-Schaltern** (nur Flüsse / nur Straßen …,
#293) würde die Liste unübersichtlich und der schmale Drawer (248 px offen) käme
an seine Grenze.

Das ist dieselbe **Bug-/Design-Klasse** wie am Scope-Rand (ADR 0029): ohne eine
verbindliche Struktur wächst neues Chrome wild, bis die Bedienbarkeit leidet.

## Entscheidung — Zonen fürs Panel: aufklappbare Gruppen mit tri-state Master

Der Layer-Abschnitt wird in **Gruppen** organisiert. Eine Gruppe ist **ein**
aufklappbarer Block (`LayerGroup.vue`) mit einem **tri-state Master-Schalter** in
der Kopfzeile; jedes Layer ist eine **Zeile** im Slot der Gruppe.

**Die verbindliche Regel:**

> **Neues Layer-Chrome kommt als Zeile in eine bestehende Gruppe — nie als loser
> Schalter in einer flachen Liste.** Passt es in keine Gruppe, wird eine **neue
> Gruppe** definiert (ein `LayerGroup`-Block), nicht eine Einzelzeile frei
> angehängt.

### Die vier Gruppen (Betreiber-Entscheid 2026-07-20)

| Gruppe | Inhalt |
|--------|--------|
| **Aeronautik** | Lufträume (4 Gruppen), AoR, VOR/NDB, Waypoints, Flughäfen, Runways |
| **Karte** | Basiskarte (BKG) — + künftige BKG-Element-Ebenen (#293) |
| **Radar & Reichweite** | Radarabdeckung, History-Dots (+Dauer), Range-Rings (+Konfig) |
| **Wetter** | DWD-Regenradar, DWD-Wetterwarnungen (je +Legende) |

Ordnung nach operativer Häufigkeit: Aeronautik → Karte → Radar & Reichweite →
Wetter. Die **Spurherkunft-Legende** ist ein Referenz-Block (kein Toggle) und
bleibt außerhalb der Gruppen am Fuß des Abschnitts. **Filter** (FL-Band) und
**Nutzer-Account** sind eigene Sektionen (unverändert).

### Der tri-state Master

Der Master reduziert die **sichtbaren, bedienbaren** Mitglieder einer Gruppe auf
einen von drei Zuständen: **an** (alle an), **aus** (alle aus), **teilweise**
(indeterminate). Ein Klick ist ein **Select-all/none**: alles-aus wird alles-an,
alles andere (an oder teilweise) wird alles-aus (`map/layerGroups.js`,
schema-agnostisch + unit-getestet).

**Wichtige Feinheiten:**

- **Ein deaktivierter Toggle** (Quelle nicht verfügbar — z. B. Radarabdeckung
  ohne Radar-Sensor, DWD ohne konfigurierte Quelle) ist aus dem Master-Zustand
  **und** der Bulk-Aktion **ausgeschlossen**. Sonst bliebe der Master ewig
  „teilweise", und der Master dürfte einen Layer ohne Daten nicht einschalten.
- Die Master-Bulk-Aktion **routet über denselben Store-Pfad** wie der jeweilige
  Zeilen-Switch (`onLayerToggle` bzw. `setAirspaceGroup`) — ein Master-Klick ist
  ununterscheidbar davon, jede Zeile einzeln zu schalten. Kein „toter" Toggle.
- Eine Gruppe **ohne sichtbares Mitglied** (alles per Entitlement ausgegated)
  verschwindet ganz (`v-if`).

### Umsetzung (ASD-020 / E1)

- **`LayerGroup.vue`** (neu): positions-neutraler, aufklappbarer Block —
  Chevron + Titel + `v-checkbox-btn`-Master (indeterminate = teilweise); der
  Master-Klick wird an den Elternteil delegiert, damit der **abgeleitete**
  Zustand gewinnt (Vuetifys eigenes Toggle kämpft nicht dagegen). Slot für die
  Zeilen. Gruppen starten **aufgeklappt** — E1 blendet nichts aus, was vorher
  sichtbar war; der Lotse kann eine nicht benötigte Gruppe einklappen.
- **`map/layerGroups.js`** (neu): `masterState(values)` + `nextMaster(state)` —
  die reine Tri-State-Logik, ohne Vuetify-Mount unit-testbar.
- **`LayerFilterContent.vue`**: die vier Gruppen; die Gruppen-**Mitgliedschaft**
  lebt hier neben den Zeilen (`{on, set, enabled}`-Member), also an *einer*
  offensichtlichen Stelle, wenn ein neues Layer dazukommt.

## Konsequenzen

- Die „Fülle" ist strukturell beherrscht: neue Layer (insbesondere die
  BKG-Elemente #293) ordnen sich einer Gruppe unter, statt die Liste zu
  verlängern. Der Master erlaubt gruppenweises Ein-/Ausblenden mit einem Klick.
- **Spätere Stufen (nicht Teil dieser Entscheidung):** Accordion (nur eine
  Gruppe offen) für den schmalen Drawer, **Presets** „Minimal/Standard/
  Detailliert" (#294) und **Persistenz im View-Profil** (#295). Der Rahmen hier
  trägt sie.
- **Ehrliche Grenze:** Es gibt keinen WebGL-/Mount-Harness für eine *visuelle*
  Zusicherung; die Struktur ist per Source-Guards festgezurrt
  (`layerGroups.test.js`, `layerGrouping.test.js`), die optische Abnahme macht
  der Betreiber.
