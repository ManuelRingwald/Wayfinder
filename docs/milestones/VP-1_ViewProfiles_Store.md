# VP-1 — View-Profile: Per-Nutzer-Store (Backend-Grundlage)

> **Kontext:** Erstes Häppchen des Features **View-Profile** (ADR 0023):
> persönliche, benannte Anzeige-Profile pro Nutzer (bis zu 3, eins als
> Login-Default). Dieses Häppchen legt nur die **Persistenz-Grundlage** — API,
> Frontend-Store, UI und Apply-on-Login folgen in VP-2…VP-5. **Kein CAT062-Bezug.**

## Fachlich — warum

Ein Lotse wechselt je nach Aufgabe zwischen Kartenbildern (Approach vs.
Überblick). Statt die Toggles jedes Mal neu zu setzen, soll er seine Ansicht
**speichern, benennen und abrufen** können. Voraussetzung ist ein sicherer,
per-Nutzer-gescopter Speicher mit klaren Grenzen — das ist VP-1.

## Technisch — wie

### Migration `00022_user_view_profiles.sql`
Neue Tabelle, getrennt von `view_configs` (Karten-Rahmung):

```
user_view_profiles(
  id, user_id FK→users ON DELETE CASCADE,
  name, settings JSONB (opak, default '{}'),
  is_default, created_at, updated_at)
```
- Index auf `user_id`.
- **Partieller Unique-Index** `WHERE is_default` → **höchstens ein** Default/Nutzer.

### `pkg/store/view_profiles.go` — `ViewProfileRepo`
- `ListByUser` (Erzeugungsreihenfolge), `Create`, `Update`, `Delete`,
  `SetDefault`, `GetDefault`.
- **Cap = 3** (`MaxViewProfilesPerUser`): in `Create` innerhalb einer Transaktion
  mit **per-Nutzer `pg_advisory_xact_lock`** geprüft (count-then-insert kann nicht
  am Cap vorbeirasen) → `ErrProfileLimit`.
- **Single-Default:** `Create(makeDefault)` und `SetDefault` löschen den alten
  Default in derselben Transaktion; der Unique-Index ist die harte Grenze.
- **Ownership:** jede Mutation/Lesung ist `WHERE id AND user_id` → ein fremdes
  Profil ergibt `ErrNotFound`, nie eine Leckage. `user_id` kommt später
  ausschließlich aus der Session (VP-2), nie aus dem Body.
- **`settings` opak:** verbatim gespeichert/zurückgeliefert; `normalizeSettings`
  macht aus nil/leer ein `{}` (nie SQL NULL). Backend interpretiert die
  Toggle-Schlüssel nie → neuer Frontend-Toggle braucht keine Migration.

## Sicherheit

Per-Nutzer-Isolation und die Grenzen (Cap, Single-Default) sind **Store-
Invarianten**, nicht bloß UI-Zusicherungen — die API (VP-2) kann sie nicht
umgehen. Multi-Tenant ist implizit sicher (ein Nutzer gehört zu genau einem
Tenant); Auth bleibt immer an.

## Tests

- **`view_profiles_test.go`** (unit, ohne DB): `normalizeSettings`.
- **`view_profiles_integration_test.go`** (`TestIntegrationViewProfilesCRUD`,
  gegen echte DB): List/Create/Update/Delete/SetDefault/GetDefault, **Cap →
  `ErrProfileLimit`**, **Single-Default-Invariante** über `SetDefault`,
  **Cross-User-Isolation** (fremdes Profil → `ErrNotFound`), nil-`settings` → `{}`.
  `store_integration_test.go`-TRUNCATE um `user_view_profiles` ergänzt.

Gates: `go build`/`go vet`/`gofmt` grün; die neuen Store-Tests **grün gegen eine
echte PostgreSQL-16-DB** (die zwei unverändert roten Integrationstests
`FeedSourceConfig`/`AeroCacheRepo` sind **vorbestehend/umgebungsbedingt**,
timezone-/env-abhängig, unabhängig von diesem Häppchen).

## Nächste Häppchen

VP-2 (REST-API `/api/view-profiles` hinter `tenantMW`) → VP-3 (Frontend-Store +
`captureSettings`/`applySettings`) → VP-4 (UI-Umschalter + Speichern-Dialog) →
VP-5 (Apply-on-Login).
