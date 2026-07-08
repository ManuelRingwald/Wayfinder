# VP-3 — View-Profile: Frontend-Store + Capture/Apply

> **Kontext:** Drittes Häppchen des Features **View-Profile** (ADR 0023). Baut die
> Frontend-Logik auf der VP-2-API auf: den Zustands-Store und die reine
> Serialisierung der Anzeige-Präferenzen. **Noch keine UI** (VP-4). **Kein
> CAT062-Bezug**; keine Komponente importiert die neuen Module → `dist`
> unverändert.

## Fachlich — warum

Die UI (VP-4) und das Apply-on-Login (VP-5) brauchen einen Store, der die eigenen
Profile lädt, anlegt, wählt und als Default setzt — und eine verlässliche Art, die
aktuelle Ansicht in ein Profil zu **fangen** und ein Profil wieder **anzuwenden**.

## Technisch — wie

### Reine Serialisierung — `frontend/src/stores/profileSettings.js`
Bewusst **seiteneffektfrei** (operiert auf dem übergebenen Store, kein Pinia-
Import) und damit isoliert testbar:
- **`captureSettings(asd)`** → versioniertes Objekt der **Anzeige-Präferenzen**:
  `layers` (ganze `layerVisibility`), `airspaceGroups`, `rangeRings`
  (`spacingNM`/`count`), `history` (`durationS`), `flFilter`. Whatever Layer-Keys
  der Store trägt, werden mitgenommen (**vorwärtskompatibel**). **Kein**
  Karten-Zentrum/Zoom (Option A — die Sektor-Rahmung bleibt Tenant-View/AOI).
- **`applySettings(asd, settings)`** → spielt es über die asd-Store-Setter zurück,
  **tolerant**: unbekannte/fehlende Sektionen und Keys werden übersprungen, Zahlen
  validiert (`finiteOrNull`), der **abgeleitete `airspace`-Layer wird nicht direkt
  gesetzt**, sondern folgt den angewandten Airspace-Gruppen. So kann ein partielles
  oder veraltetes Profil nie den Zustand beschädigen. Die Karte folgt über die
  bestehenden MapCanvas-Watcher (Layer/Filter/Ringe/History).

### Zustands-Store — `frontend/src/stores/profiles.js`
Pinia-Store gegen die VP-2-API (`apiFetch`): `load`, `saveCurrent(name,
makeDefault)` (fängt die aktuelle Ansicht), `update`/`rename`/`overwrite`,
`remove`, `setDefault`, `apply(id)` (wendet ein Profil an, setzt `activeId`).
Der lokale Zustand wird nach jedem Call konsistent gehalten — u. a. bleibt lokal
**genau ein** `is_default` (`markDefaultLocally`). `canCreate` (≤ `MAX_PROFILES`
= 3) und `defaultProfile` als Computed für die UI.

## Ehrliche Grenze

Der Server bleibt **autoritativ** (Cap, Single-Default, Ownership — VP-1/VP-2);
der Store spiegelt nur optimistisch für flüssige UI. Basiskarte (dark/OSM) ist
**bewusst nicht** Teil des Profils in v1 (bräuchte einen Karten-Restyle) — kann
additiv folgen.

## Tests

- **`profileSettings.test.js`** — `captureSettings` (Snapshot, forward-kompatible
  Layer-Keys, kein Zentrum/Zoom), `applySettings` (Setter-Durchreichung, `airspace`
  aus Gruppen abgeleitet, Toleranz gegen unbekannte Keys/Junk, FL-Coercion auf
  `null`), **Round-Trip** `apply(capture(a))`.
- **`profiles.test.js`** — `load`/`saveCurrent`/`rename`/`remove`/`setDefault`/
  `apply`/`canCreate` gegen ein gemocktes `fetch` (Body/Method-Assertions, Fehler-
  Pfad, lokales Single-Default).

**vitest 504 grün**; `dist` unverändert (keine Komponente importiert die Module).

## Nächste Häppchen

VP-4 (UI-Umschalter + „Ansicht speichern"-Dialog, verdrahtet den Store, baut
`dist` neu) → VP-5 (Apply-on-Login des Default-Profils).
