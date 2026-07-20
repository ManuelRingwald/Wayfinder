# ASD-023 (E4) — BKG-Element-Auswahl im View-Profil persistieren

> **Register:** FR-UI-047 · **Bezug:** ADR 0023 (View-Profile), ADR 0031, Epic
> #290, Issue #295 (E4). Baut auf **E2** (#293, Element-Schalter).

## Fachlich — warum

Mit E2 kann der Lotse die Basiskarte nach Elementen ausblenden („nur Flüsse",
oder eine entclutterte Ansicht ohne Gebäude/Beschriftung). Diese Auswahl ging aber
bei jedem Reload / Profilwechsel verloren — alle Elemente standen wieder auf an.
E4 **speichert die Element-Auswahl im View-Profil**, sodass eine bewusst
zugeschnittene Karten-Ansicht erhalten bleibt.

## Technisch — dem bestehenden Muster folgend

Reine Ergänzung der View-Profil-Serialisierung (`stores/profileSettings.js`),
exakt gespiegelt zur `airspaceGroups`-Behandlung:

- **`captureSettings`**: neuer Abschnitt `basemapElements: { ...asd.basemapElementVisibility }`.
- **`applySettings`**: neuer, **toleranter** Block — iteriert `settings.basemapElements`
  und schreibt jeden **bekannten** Schlüssel über `asd.setBasemapElement(k, !!v)`.
  Unbekannte Schlüssel werden übersprungen; ein **älteres Profil ohne** den
  Abschnitt lädt fehlerfrei (Elemente behalten ihre All-an-Defaults). Die Karte
  folgt über den bestehenden MapCanvas-`basemapElementVisibility`-Watcher (E2) →
  `applyBasemap`.

Kein neuer Datenfluss, keine Backend-Wirkung — die View-Profile speichern bereits
Layer-/Filter-Präferenzen (ADR 0023); dies ist ein weiterer Abschnitt darin.

## Ehrliche Grenzen

- **Versionstoleranz statt Migration:** `applySettings` ist best-effort (wie
  gehabt) — ein Profil aus der Zeit vor E4 hat keinen `basemapElements`-Abschnitt
  und lädt mit Element-Defaults (alle an). Das ist gewollt.
- Reine Frontend-Persistenz im Profil-`settings`-Blob; kein CAT062-/Backend-Bezug.

## Tests

- `stores/__tests__/profileSettings.test.js`: Fake-Store um `basemapElementVisibility`
  + `setBasemapElement` erweitert; neue Fälle — Capture+Restore der Element-Auswahl
  (ausgeblendete bleiben aus, unberührte bleiben an), Toleranz gegen ein Profil
  **ohne** `basemapElements`-Abschnitt (Defaults bleiben), Überspringen eines
  unbekannten Element-Schlüssels.
