# AP2 — Feature-Katalog erweitern

**Paket:** ADR 0009 · **Stufe:** S3 · **Modell:** Sonnet 4.6
**Issue:** #65 · **Abgeschlossen:** 2026-06-25

---

## Fachliche Motivation

Der Feature-Entitlement-Service (WF2-50) wurde mit drei initialen Keys
eingeführt (`stca`, `multi_feed`, `premium_layers`). Die ASD-Kartenoverlays
(Lufträume, Range-Rings, History-Dots, VOR/NDB, Waypoints) waren bereits als
Layer implementiert, aber noch nicht über das Entitlement-System steuerbar —
ein Mandant konnte sie weder aktivieren noch deaktivieren.

AP2 schließt diese Lücke: Die fünf aeronautischen/operativen Layer werden als
vollwertige Feature-Keys in den Katalog aufgenommen und die Sichtbarkeit ihrer
Steuerelemente im ASD-Seitenbereich wird kosmetisch per `hasFeature()` gegatet.

---

## Technische Umsetzung

### Backend: `pkg/feature/catalog.go`

Fünf neue Keys wurden dem geschlossenen Katalog hinzugefügt:

| Key | Konstante | Beschreibung |
|-----|-----------|--------------|
| `airspaces` | `Airspaces` | Luftraum-Overlays (CTR, TMA, restricted, info) — ASD-011 |
| `range_rings` | `RangeRings` | Range-Ring-Overlay — ASD-012 |
| `history_dots` | `HistoryDots` | Track-History-Punkte — ASD-004a |
| `vor_ndb` | `VorNdb` | VOR/NDB-Navaid-Overlay — ASD-003 |
| `waypoints` | `Waypoints` | Wegpunkt-Overlay — ASD-003 |

Der Katalog umfasst nun **8 Keys**. Fail-closed-Mechanik, `IsKnown`, `All`,
`Describe` und `Set`-Validierung sind unverändert.

`whoami` liefert automatisch alle 8 Keys mit ihrem effektiven Wert (kein
Backend-Change am Admin-API nötig).

### Frontend: Layer-Sichtbarkeit & Steuerelemente

**`frontend/src/stores/asd.js`**
- `historyDots: true` in `layerVisibility` — History-Dots hatten zuvor keinen
  Toggle; jetzt vollständig in den Layer-Visibility-Mechanismus integriert.

**`frontend/src/map/engine.js`**
- `HISTORY_DOTS_LAYER_ID` importiert und als Gruppe `historyDots` in
  `setLayerVisibility` eingetragen. Der bestehende `watch()`-Spread in
  `MapCanvas.vue` deckt den neuen Key automatisch ab.

**`frontend/src/components/LayerFilterContent.vue`**
- `useAdminStore` importiert.
- Alle fünf neuen Feature-Keys werden als UI-Gate verwendet:
  ```
  v-if="!admin.isAuthorized || admin.hasFeature('<key>')"
  ```
- Für `coverageRings` (Radarabdeckung) bleibt **kein Gate** — es ist
  Infrastruktur-Overlay, kein mandantenspezifisches Aerodaten-Feature.
- Neuer **History-Dots-Switch** (`label="History Dots"`,
  `v-model="store.layerVisibility.historyDots"`) mit `historyDots`-Gate.

### Gate-Formel

```js
!admin.isAuthorized || admin.hasFeature(key)
```

- **Nicht-Admin-Nutzer** (403 auf `whoami` → `isAuthorized = false`): sehen
  alle Layer-Steuerelemente (Kurzschluss über `!isAuthorized`).
- **Admin-Nutzer**: sehen nur Steuerelemente für freigeschaltete Features.
- **Kein Server-Enforcement** auf aeronautische Daten — das Gating ist rein
  kosmetisch (die Daten liegen öffentlich über OpenAIP vor).

---

## Tests

### Go (`pkg/feature`)

- `TestIsKnown`: prüft alle 8 Keys (vorher 3).
- `TestAllSortedAndComplete`: `len(All()) == 8`, alle 8 Keys in der sortierten
  Rückgabe (vorher 3).
- `TestEffectiveDefaultDeny` / `TestEffectiveStoreErrorFailsClosed`: Längen-
  Check auf 8 (vorher 3).

### Vitest (`admin.test.js`)

Neuer `describe`-Block **„feature catalog (AP2)"**:

| Test | Was wird geprüft |
|------|-----------------|
| Airspace-Overlay-Keys aus `whoami` | `airspaces=true`, `vor_ndb=false`, `waypoints=true` |
| Display-Layer-Keys aus `whoami` | `range_rings=true`, `history_dots=false` |
| Alle 5 Keys default `false` bei leerem `features`-Objekt | Fail-closed |
| `isAuthorized=false` auf 403 | Gate-Formel ergibt `true` für alle 5 Keys (Nicht-Admin sieht alles) |

---

## Qualitäts-Gates

- `go test ./...` ✅ (alle Pakete grün)
- `go vet ./...` ✅
- `vitest run` ✅ (112 Tests in 8 Suiten)
- `docs/requirements/README.md` — FR-TEN-003 auf 8 Keys und AP2-Gating erweitert ✅
- `docs/TECHNICAL.md` — Abschnitt 5.5 um vollständige Katalog-Tabelle ergänzt ✅
- kein Schema-Change, keine neue Env-Var ✅
