# WF2-40 — Spur-Herkunft als Sicht-Layer (Provenienz-Symbole)

> **Stufe:** Betriebs-Härtung / ASD-Feinschliff · **Paket:** WF2-40 ·
> **Einstufung:** S3 · Sonnet 4.6 (Frontend-Logik + Symbol-Umbau, keine
> Architektur-Wirkung) · **Grundlage:** CAT062-Vertrag (CLAUDE.md §2), Fireflys
> ICD 2.4.0 / ADR 0019 (ADS-B-Alter I062/290). **Kein Backend-/ICD-Change.**

## Warum (fachlich)

Bisher sah der Lotse jeden Track als denselben Punkt — unabhängig davon, ob die
Spur von **ADS-B** (kooperativ, hochpräzise Selbstmeldung mit ID/Höhe),
**SSR/Mode S** (kooperativer Sekundär-Reply) oder nur **Primärradar (PSR)** (reine
Skin-Paint: Position ohne Identität/Höhe, geringere Integrität) gestützt wird.

Die Herkunft ist eine **Daten-Qualitäts- und Vertrauensinformation**: Ein
primär-only-Track neben kontrollierter Lage verdient einen anderen Blick als ein
ADS-B-Track. WF2-40 macht die Herkunft **auf einen Blick** sichtbar, ohne den
Scope zuzukleistern.

### Ehrliche Grenze (wichtig)

Die Provenienz ist **track-abgeleitet**, *nicht* zertifizierte
Per-Plot-Provenienz. CAT062 trägt keine explizite Sensor-Quelle pro Plot auf der
Leitung; wir schließen ausschließlich aus den Items, die der Vertrag ohnehin
liefert (`adsb_age_s`, `icao_addr`, `mode_3a`, Callsign). Eine echte
Per-Track-Sensorprovenienz wäre eine ICD-Änderung bei Firefly — offen als
**WF2-42**.

## Was (technisch)

**Form = Herkunft, Farbe = Zustand.** Der Track-Layer wurde von einem
`circle`-Layer auf einen `symbol`-Layer umgestellt. Die **Form** des Icons
kodiert die Herkunft, die **Farbe** unverändert den Track-Zustand:

| Herkunft | Symbol | Bedeutung |
|----------|--------|-----------|
| `adsb`   | ◆ gefülltes Karo   | ADS-B-Anteil **aktuell frisch** (kooperativ, hochpräzise) |
| `ssr`    | ▢ gefülltes Quadrat | kooperativer Sekundär-Reply (Mode S / Mode 3A / Callsign) |
| `psr`    | ○ offener Ring      | Primär-only Skin-Paint (keine ID/Höhe) — „hohl" = datenärmer |

- **Klassifizierer** (`src/map/provenance.js`, rein + unit-getestet):
  `trackProvenance(track)` mit Präzedenz **adsb → ssr → psr**. ADS-B zählt nur,
  wenn `adsb_age_s` vorhanden **und frisch** ist (`isAdsbFresh`,
  ≤ `ADSB_FRESH_THRESHOLD_S` = 30 s) — übernimmt die Frische-Semantik des früheren
  ADS-B-Data-Block-Badges (siehe „Beziehung zu FR-ASD-006"). Eine stale ADS-B-Spur
  fällt auf ihre verbleibende kooperative/primäre Quelle zurück. Präsenz wird mit
  `!= null` geprüft, damit ein Null-Wert (z. B. `adsb_age_s === 0`, ein frisches
  Update; oder Squawk `0o0000`) korrekt als vorhanden zählt.
- **Symbol-Rendering ohne SDF** (`src/map/layers.js`): `addTrackIcons` rendert
  **12 Icons** (3 Formen × 4 Zustandsfarben) zur Laufzeit auf ein Canvas
  (`makeTrackIcon`, gleiches Muster wie die Navaid-Icons). Die Zustandsfarbe wird
  beim Zeichnen **eingebacken** — so bleibt die alte `circle-color`-Semantik
  erhalten, ohne die Antialiasing-Fallstricke des SDF-`icon-color`-Tintings. Der
  Layer wählt das Icon per datengetriebenem Ausdruck:
  `["concat","wf-trk-",["coalesce",["get","provenance"],"psr"],"-",<state-case>]`.
  `icon-opacity` ist identisch zur früheren `circle-opacity` (Vorrang
  fade > FL > coasting > normal); `icon-allow-overlap` + `icon-ignore-placement`
  stellen sicher, dass Tracks nie durch Symbol-Kollision verschwinden.
- **Zustandsfarben** wurden aus dem alten `circle-color`-Ausdruck in
  `TRACK_STATE_COLORS` (`constants.js`) faktorisiert — eine Wahrheitsquelle für die
  Icon-Generierung.
- **Datenfluss:** `provenance` wird in `updateTracksLayer` (live) und
  `renderSources` (fading) auf **jedes** Track-Feature gelegt.
- **Textuelle/zugängliche Darstellung** (statt Glyph an *jedem* Datenblock, das
  den Scope zukleistern würde): Das **Track-Detail-Panel**
  (`TrackDetailCard.vue`) zeigt die Herkunft im Klartext („Herkunft: ADS-B
  (kooperativ)" …); die **Sidebar** (`LayerFilterContent.vue`) trägt eine
  statische **Form-Legende** mit der Notiz „Form = Herkunft · Farbe = Status".

## Beziehung zu FR-ASD-006 (gelöste Regression)

FR-ASD-006 hatte ein ADS-B-`◆`-**Badge im Data-Block** spezifiziert (mit
30-s-Frische-Schwelle), implementiert in der **alten** `internal/webui/static/app.js`.
Beim Port auf das Vue-Frontend ging dieses Badge **verloren** — es war im
ausgelieferten Bundle (`dist/`) nicht vorhanden (Doku-vs.-Code-Lücke). WF2-40
**stellt die ADS-B-Kennzeichnung wieder her** — mit identischer Frische-Schwelle,
nun als Symbol-**Form** ◆ statt Label-Glyph — und **löst FR-ASD-006 ab**
(Registry aktualisiert). Die alte `static/app.js` bleibt toter Referenz-Code.

## Sicherheit / Korrektheit

- **Kein Vertrauen in fremde Daten:** Der Klassifizierer wirft nie (Null-/
  Undefined-Eingaben → `psr`); der `coalesce`-Guard im `icon-image`-Ausdruck
  fängt ein fehlendes `provenance`-Property ab.
- **Keine Schnittstellen-Wirkung:** rein clientseitig, alle Felder lagen bereits
  im WS-JSON. Kein Backend-, kein ICD-, kein Vertrags-Change.
- **Zustands-Semantik unangetastet:** Farbe, Coasting-Dimming, FL-Filter und
  TSE-Fade verhalten sich exakt wie zuvor (gleiche Opazitäts-Ausdrücke).

## Tests

- `src/map/__tests__/provenance.test.js` (15 Tests): Wahrheitstabelle der
  Präzedenz (adsb > ssr > psr), Frische-Grenze (≤ 30 s, Null-Alter, stale →
  Fallback), Squawk `0o0000` als vorhanden, leeres Callsign als „keine ID",
  Null-/Undefined-Robustheit, Label-Vollständigkeit.
- `src/map/__tests__/tracks.test.js` (`updateTracksLayer provenance`): verankert,
  dass jedes Live-Feature die korrekte `provenance` trägt.
- **Symbol-Optik:** manuell verifiziert (kein WebGL-Test-Harness im Projekt, vgl.
  FR-ASD-001/003/004). `npm run build` grün, `vitest run` grün (78 Tests).

## Abgrenzung / Nächstes

- **Nicht** Teil von WF2-40: echte Per-Plot-Sensorprovenienz (bräuchte einen
  CAT062/ICD-Zusatz bei Firefly) → **WF2-42**.
- „mehr I062/080" (zusätzliche Track-Status-Bits über CNF/TSE/CST hinaus) ist
  separat zu bewerten, sobald die ICD entsprechende Felder exponiert.
