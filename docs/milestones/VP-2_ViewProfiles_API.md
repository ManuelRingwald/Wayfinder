# VP-2 — View-Profile: user-gescopte REST-API

> **Kontext:** Zweites Häppchen des Features **View-Profile** (ADR 0023). Setzt auf
> dem VP-1-Store auf und exponiert ihn als **user-gescopte** HTTP-API. **Kein
> CAT062-Bezug**, keine neuen Env-Variablen.

## Fachlich — warum

Damit das Frontend (VP-3/VP-4) Profile speichern, listen, wählen und als Default
setzen kann, braucht es eine API — und zwar eine, die **jedem Nutzer nur seine
eigenen** Profile zeigt und ändern lässt.

## Technisch — wie

### Endpunkte (hinter `tenantMW` + `pwGate`, **kein** Admin-Gate)
| Methode & Pfad | Zweck |
|----------------|-------|
| `GET /api/view-profiles` | eigene Profile listen (leer → `[]`) |
| `POST /api/view-profiles` | anlegen (`{name, settings, make_default?}` → `201`) |
| `PUT /api/view-profiles/{id}` | umbenennen + `settings` ersetzen |
| `DELETE /api/view-profiles/{id}` | löschen (`204`) |
| `POST /api/view-profiles/{id}/default` | als Login-Default setzen |

Gemountet in `main.go` über einen **Sub-Mux** (`ViewProfilesHandler()`), an
`/api/view-profiles` und `/api/view-profiles/`.

### Sicherheit (Kern)
- **Nutzer-Skopus aus der Session:** jeder Handler liest `id.UserID` aus
  `tenant.FromContext` — **nie** aus dem Request-Body. Der Store scoped jede
  Query mit `user_id`, sodass ein fremdes `{id}` als **404** endet (keine
  Existenz-/Daten-Leckage). Kein Admin-Gate: ein Profil ist strikt privat.
- **`pwGate`:** Solange ein Passwortwechsel aussteht, sind die Routen gesperrt
  (wie die übrigen Nutzer-Routen; nur whoami/Passwort-Change sind offen).

### Validierung (`validateViewProfile`, rein/testbar)
- Name: getrimmt, nicht leer, ≤ 60 Zeichen.
- `settings`: **muss ein JSON-Objekt** sein (kein Array/Scalar — das Frontend
  speichert eine Toggle-Map), ≤ 16 KiB; leer/fehlend → `{}`. Die **Schlüssel
  werden nicht** interpretiert (opak, ADR 0023) → neuer Toggle ohne Backend-Änderung.
- Fehlerbilder: kaputtes JSON → `400`, ungültig → `422`, Cap (>3) → `409`
  (aus `store.ErrProfileLimit`), nil-Store → `404`, keine Sitzung → `401`.

### Verdrahtung
- `ViewProfileStore`-Interface (kleiner Store-Slice) + `WithViewProfiles`-Builder
  (nil-safe: ohne Wiring liefern die Routen `404`) — konsistent mit den übrigen
  optionalen Stores der `adminapi.Handler`.
- `main.go`: `store.NewViewProfileRepo(dbPool)` via `WithViewProfiles` + Route-Mount.

## Tests

`adminapi_view_profiles_test.go` (mit Fake-Store + injizierter Identity):
- `validateViewProfile` (leerer/zu langer Name, Nicht-Objekt/oversize `settings`,
  Trim, nil→`{}`), DTO nil→`{}`.
- List/Create/Update/Delete/SetDefault (Happy-Path), inkl. **Scoping auf die
  Session-`user_id`** und `{id}` aus dem Pfad.
- Validierungsfehler (`422`/`400`), **Cap → `409`**, **fremd/unbekannt → `404`**,
  **kein Identity → `401`**, **nil-Store → `404`**, leere Liste → `[]`.

Gates: `go build`/`go vet`/`gofmt`/`golangci-lint` (0 issues) grün; adminapi-Tests
grün.

## Nächste Häppchen

VP-3 (Frontend-Store + reine `captureSettings`/`applySettings`) → VP-4 (UI-
Umschalter + Speichern-Dialog) → VP-5 (Apply-on-Login des Default-Profils, via
whoami oder Profilliste).
