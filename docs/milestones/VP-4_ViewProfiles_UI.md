# VP-4 — View-Profile: UI-Umschalter + Speichern-Dialog

> **Kontext:** Viertes Häppchen des Features **View-Profile** (ADR 0023) — das
> sichtbare Stück. Verdrahtet den VP-3-Store zu einer Bedienung im ASD.
> **Kein Backend-/CAT062-Bezug**; `dist` neu gebaut. Apply-on-Login ist VP-5.

## Fachlich — warum

Der Lotse soll seine Ansicht **benennen und speichern**, sie **durch Auswahl
abrufen** und eine als **Default** markieren — direkt aus dem Lagebild heraus,
ohne die Toggles jedes Mal neu zu setzen.

## Technisch — wie

### `ViewProfileMenu.vue` (im ASD-Header-Cluster)
- Ein Button (Label = aktuell angewandtes Profil, sonst „Ansicht") öffnet ein
  **`v-menu`** mit der Profilliste:
  - **Klick auf einen Eintrag** → `store.apply(id)` (schreibt die Anzeige-Prefs in
    den asd-Store; die Karte folgt über die bestehenden MapCanvas-Watcher). Das
    aktive Profil ist hervorgehoben (`store.activeId`).
  - **Stern-Icon** → `store.setDefault(id)` (spiegelt `is_default`; gefüllter Stern
    = Login-Default).
  - **Stift** → Umbenennen-Dialog (`store.rename`), **Papierkorb** →
    `store.remove(id)`.
  - „**Aktuelle Ansicht speichern…**" (deaktiviert bei Cap, `store.canCreate`,
    Hinweis „Maximal 3 Profile").
- **`v-dialog`** für Speichern/Umbenennen: Namensfeld (≤ 60, Counter) + Checkbox
  „Als Standard beim Login" (nur beim Speichern) → `store.saveCurrent(name,
  makeDefault)`. Store-Fehler werden im Dialog als `v-alert` gezeigt.
- Lädt die Profile `onMounted` (`store.load()`). `pointer-events: auto` auf dem
  Control (der Header-Cluster hat `pointer-events: none`); `v-menu`/`v-dialog`
  rendern als Overlay und sind davon unberührt.

### Verdrahtung
`AsdView.vue` importiert `ViewProfileMenu` und rendert es im `top-right-cluster`
zwischen Feed-Status-Chip und Ereignis-Glocke (nur im authentifizierten Scope).

## Ehrliche Grenze

Der Server bleibt autoritativ (Cap/Single-Default/Ownership); die UI spiegelt
optimistisch. **Apply-on-Login** (Default beim Einloggen anwenden) ist bewusst
**VP-5** und noch nicht enthalten — hier wählt der Nutzer manuell.

## Tests

`viewProfileMenu.test.js` (Source-Guard, Projektkonvention wie `eventPanel.test.js`
— kein Component-Mount-Harness fürs ASD-Chrome): Store-Verdrahtung, `load` on
mount, `apply`/`setDefault`/`remove`/`saveCurrent`/`rename`, Default-Stern,
Cap-Gating („Maximal 3"), AsdView-Mount. Die Store-Logik selbst deckt VP-3 ab
(`profiles.test.js`/`profileSettings.test.js`).

Gates: **vitest 510 grün**, `vite build` + eingebettetes `dist` neu
(deterministisch), Go unberührt.

## Nächstes Häppchen

VP-5 — Apply-on-Login: nach dem Karten-Init das Default-Profil anwenden
(orthogonal zur Tenant-Karten-Rahmung; kein Default → heutiges Verhalten).
