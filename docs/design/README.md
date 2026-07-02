# Wayfinder — Design

Dieser Ordner bündelt die **Design-Grundlagen** von Wayfinder: das visuelle
Konzept, die Farb-/Design-Tokens und die Provenienz des Claude-Design-Exports.

## Inhalt

| Datei | Zweck |
|-------|-------|
| `README.md` | Dieser Überblick. |
| `color-tokens.md` | Beschreibung des Farbschemas (Chrome + Scope). Realisiert als CSS-Tokens, siehe unten. |
| `wayfinder-2.0-konzept.md` | Strategie-/Architektur-Konzept der Plattform (unabhängig vom visuellen Reskin). |

## Maßgebliche Token-Quelle (Design System v1, ADR 0015)

Die Design-Tokens leben **im Frontend** als CSS-Custom-Properties (`--wf-*`):

- `frontend/src/design/tokens/` — `colors.css`, `typography.css`, `spacing.css`,
  `radius.css`, `elevation.css` (+ `index.css` als Sammel-Import).
- `frontend/src/design/base.css` — die wenigen global angewandten Bindungen
  (Grund-Schrift, `.wf-mono`, `.wf-overline`).

**Autoritäts-Nähte** (siehe ADR 0015):

- **Chrome-Farben:** `frontend/src/plugins/vuetify.js` (`asdDarkTheme`) ist der
  Theme-Ursprung; `colors.css` spiegelt dieselben Werte für Nicht-Vuetify-DOM.
  Beide werden im Gleichschritt gepflegt.
- **Scope-/Domänen-Farben:** maßgeblich ist `frontend/src/map/constants.js`
  (MapLibre-GL-Paint). Die Domänen-Sektion in `colors.css` ist nur ein
  DOM-seitiger Spiegel für Legenden-Swatches.

## Schrift

Roboto (UI) + Roboto Mono (tabellarische Zahlen) werden **self-hosted** via
`@fontsource` ins Bundle eingebettet und in `frontend/src/main.js` importiert —
**kein Laufzeit-CDN** (ATC-Konsole, offline-tauglich). Nur die Subsets
**latin + latin-ext** werden geladen.

## Provenienz des Claude-Design-Exports

Das neue Design wurde mit **Claude Design** erstellt und als Projekt-Export
bereitgestellt (`ASD.zip`). Es enthält:

- ein **Design-System** (`_ds/…`) mit Tokens — **rückwärts aus dem
  Wayfinder-Code abgeleitet**, daher decken sich die Werte mit dem bestehenden
  Theme;
- **ASD-Ziel-Screens** als **React/JSX** (`asd/*.jsx`) + Screenshots.

Wayfinders Frontend ist **Vue 3 + Vuetify + MapLibre** (ADR 0002). Der JSX-Code
ist damit **Referenz-Zielbild, kein ausgelieferter Code** — Layout und
Interaktions-Absicht werden nach Vue übersetzt, bestehende Komponenten
weiterverwendet. Der Reskin läuft **inkrementell** in freigegebenen Häppchen
(Reihenfolge/Stand: `docs/STATUS.md`).

Der Export selbst ist **nicht** ins Repo eingecheckt (er lag als Upload vor); die
maßgeblichen, versionierten Artefakte sind die Tokens im Frontend und dieses
Doku-Set.
