# ORCH-1c — Feed-Quell-Builder (Frontend)

> Drittes und letztes Häppchen von **ORCH-1** (ADR 0012). Macht die in ORCH-1a/-1b
> angelegte Quell-Konfiguration über die Admin-Oberfläche bedienbar — der
> Betreiber stellt pro Feed zusammen, welche Live-Quellen die zugehörige
> Firefly-Instanz später öffnen soll. Damit ist ORCH-1 vollständig.

## Fachlicher Hintergrund

ORCH-1a/-1b lieferten Datenmodell und API; sichtbar war davon noch nichts. Der
Betreiber braucht eine Oberfläche, um pro Feed die Quellen zu pflegen (z. B. „nur
ADS-B in dieser BBox für Speyer", „echter Radar SAC/SIC für Frankfurt") — ohne
JSON von Hand zu schreiben.

## Was umgesetzt wurde

### Store-Actions (`frontend/src/stores/admin.js`)

- **`loadFeedSources(feedId)`** — `GET /api/admin/feeds/{id}/sources`, gibt die
  rohe Antwort zurück (der Dialog hält den transienten Formular-Zustand).
- **`saveFeedSources(feedId, payload)`** — `PUT …/sources`; setzt bei Erfolg den
  Erfolgs-Hinweis, sonst den Fehler (inkl. der server-seitigen
  `400`-Meldung mit Quell-Index).

### Quell-Builder-Dialog (`AdminFeeds.vue`)

Je Feed öffnet ein **„Quellen"**-Button einen Dialog mit:

- **Editierbarer Quell-Liste** — pro Eintrag ein **Typ-Dropdown** (geschlossenes
  Vokabular, gespiegelt vom Server: ADS-B/OpenSky, FLARM/APRS, Radar/ASTERIX) und
  **typ-abhängige Felder**: BBox-Eingabe (min/max lat/lon) für Flächenquellen,
  SAC/SIC für Radar, plus optionale **Credential-Referenz**.
- **Hinzufügen/Entfernen** von Quellen.
- **Coverage-Anzeige:** Die grobe Coverage-BBox wird **server-seitig** abgeleitet
  (ORCH-1b) und nach dem Speichern read-only angezeigt — keine doppelte
  Ableitungs-Logik im Client.

`buildSourcesPayload` reduziert jeden Formular-Eintrag auf die Felder seines Typs
(eine Flächenquelle sendet **nie** sac/sic — was der Server ablehnen würde) und
lässt `cred_ref` weg, wenn leer. `coverage_bbox` wird nicht gesendet → der Server
leitet sie ab.

## Sicherheits-Betrachtung

- **Server bleibt die Grenze:** Die Client-Auswahl/Validierung ist nur UX; der
  Server erzwingt Vokabular, Per-Art-Regeln und `requireAdmin` unabhängig
  (Defense-in-Depth, wie bei `validateView`). Ein abgelehnter `PUT` zeigt die
  server-seitige Meldung (inkl. Quell-Index) im Dialog.
- **Kein Klartext-Secret:** Das Feld heißt bewusst „Credential-**Referenz**" mit
  Hinweis „nie der Schlüssel selbst" — es trägt nur die `cred_ref` (NFR-SEC-004;
  Secret-Speicher folgt ORCH-2).

## Tests

`frontend/src/stores/__tests__/admin.test.js` (ORCH-1c-Block): `loadFeedSources`
(GET), `saveFeedSources` (PUT-Payload + Erfolgs-Hinweis), `saveFeedSources`
`400`-Pfad mit Quell-Index in der Fehlermeldung. Alle 164 Frontend-Tests grün;
der Production-Build (`npm run build`) übersetzt die Komponente fehlerfrei.

Wie bei den übrigen Admin-Komponenten ist die Vue-Komponente selbst über den
getesteten Store + Build abgesichert (kein gesondertes Komponenten-Test-Harness,
vgl. WF2-32).

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-001** (Frontend-Spalte ergänzt; ORCH-1 komplett),
**NFR-SEC-004**.

## Stand ORCH-1

ORCH-1 ist abgeschlossen (1a Schema + Store, 1b Admin-API, 1c Frontend). Der
nächste Schritt im Epic ist **ORCH-2** (`InstanceBackend`-Abstraktion + Docker-
Adapter, getrennte Control-Plane, Secret-Handling je Feed) — eigene Ankündigung
und Freigabe nach Charter §3.
