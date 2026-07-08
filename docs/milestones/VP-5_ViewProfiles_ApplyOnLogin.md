# VP-5 — View-Profile: Apply-on-Login

> **Kontext:** Fünftes und **letztes** Häppchen des Features **View-Profile**
> (ADR 0023). Wendet das Default-Profil des Nutzers beim Login automatisch an.
> **Kein Backend-/CAT062-Bezug.**

## Fachlich — warum

Der Nutzer soll seine bevorzugte Ansicht nicht bei jedem Login neu wählen müssen:
markiert er ein Profil als **Default**, wird es beim Einloggen **automatisch
aktiviert**.

## Technisch — wie

- **`profiles`-Store:** `applyDefaultOnce()` (mit Guard `defaultApplied`) wendet
  das Profil mit `is_default` **genau einmal pro App-Load** an
  (`applySettings` → asd-Store) und setzt `activeId`. Gibt es (noch) kein
  Default, **latcht der Guard nicht** — die Aktion ist retrybar, sobald die Liste
  geladen ist. So überschreibt sie auch **keine spätere manuelle Wahl** des
  Nutzers innerhalb der Session (nur einmal, ganz am Anfang).
- **`ViewProfileMenu.vue`** triggert es **erst wenn die Karte bereit ist**
  (`asd.mapLoaded`), damit die angewandten Toggles über die **Live-MapCanvas-
  Watcher** auf die Karte durchschlagen: nach `store.load()` (onMounted) und
  zusätzlich via `watch(() => asd.mapLoaded)`. Wer zuletzt fertig wird (Profile
  geladen / Karte bereit) löst aus; der Once-Guard verhindert Doppel-Anwendung.

## Präzedenz / ehrliche Grenze

- **Orthogonal** zur Tenant-Karten-Rahmung: das Profil setzt nur Anzeige-
  Präferenzen (Layer/Ringe/Filter/…), **nicht** Zentrum/Zoom/AOI — die bleiben
  von der effektiven View-Config getrieben (whoami).
- **Kein Default** → heutiges Verhalten (Store-Defaults).
- Der Server bleibt autoritativ (VP-1/VP-2); der Client wendet nur an.

## Tests

- **`profiles.test.js`** — `applyDefaultOnce` wendet den Default **einmal** an
  (setzt `activeId`/`defaultApplied`), ist beim zweiten Aufruf ein **No-op**
  (überschreibt eine spätere manuelle Änderung nicht), und ist **retrybar**, wenn
  noch kein Default geladen ist.
- **`viewProfileMenu.test.js`** — Source-Guard für die `mapLoaded`-Gating-
  Verdrahtung (`watch(asd.mapLoaded)` + `store.applyDefaultOnce()`).

Gates: **vitest 513 grün**, `vite build` + eingebettetes `dist` neu
(deterministisch), Go unberührt.

## Ergebnis — Feature „View-Profile" komplett

VP-1 (Store) → VP-2 (API) → VP-3 (Frontend-Store/Capture/Apply) → VP-4 (UI) →
**VP-5 (Apply-on-Login)**. Der Nutzer kann bis zu **3** persönliche Anzeige-
Profile benennen, speichern, jederzeit abrufen, eine als **Default** setzen — und
dieses wird beim Login automatisch aktiv. Server-seitig durchgehend per-Nutzer
gescopt und begrenzt; **kein CAT062-Bezug**.
