# ADR 0034 — Rollen-agnostische Konto-Selbstverwaltung (`/api/account`) + Layer-Menü-Bedienbarkeit

- **Status:** akzeptiert (2026-07-21)
- **Kontext-Issues:** #315, #316, #317, #318, #319 (Betreiber-Meldung, UI-/Konto-Bestandsaufnahme)
- **Bezug:** erweitert ADR 0011 (ONB-1 Konto-Selbstverwaltung), verfeinert ADR 0031 (Sidebar-Informationsarchitektur), Spiegel-Muster zu ADR 0023 (View-Profiles) und ADR 0024 (Korrelation) — beides self-scoped, rollen-agnostische Routen hinter `tenantMW`+`pwGate`.

## Worum es geht (in normaler Sprache)

Der Betreiber hat fünf Bedien-Themen an der ASD-Oberfläche gemeldet. Vier betreffen
das **Layer-/Filter-Menü** (ein invertierter Sammel-Schalter, zu enge
Beschriftungen, ein zu langes scrollendes Menü, fehlende Sichtbarkeit aktiver
Layer/Filter). Das fünfte ist die **Konto-Selbstverwaltung**: ein Nutzer soll
„unter Konto" seine **E-Mail-Adresse und sein Passwort selbst** setzen können, und
die E-Mail soll im Admin-Panel sichtbar werden.

Der springende Punkt bei #319 ist eine **Auth-Grenze**: Die bisherige
Selbstverwaltung (ONB-1) lag am `/api/admin/*`-Teilbaum, der komplett hinter
`requireAdmin` gemountet ist — also **nur für Admins** erreichbar. Ein reiner
Mandanten-Nutzer (Lotse) hatte gar keinen erreichbaren Konto-Endpunkt. Da die
geforderte Wirkung „E-Mail erscheint im Admin-Panel" nur für **Mandanten-Nutzer**
gilt (Admins stehen in keiner Mandanten-Zugangstabelle), muss die Selbstverwaltung
für **jeden angemeldeten Nutzer** erreichbar sein.

## Entscheidung

### 1. Rollen-agnostische Konto-Selbstverwaltung über `/api/account/*` (#319)

Neue self-scoped Routen, gemountet hinter `tenantMW(pwGate(...))` — **nicht** dem
Admin-Gate:

- `PUT /api/account/password` (`{current_password, new_password}`)
- `PUT /api/account/email` (`{email}`; leer = löschen)

Beide lesen die **User-ID aus der Session-Identity** (`tenant.FromContext`), **nie**
aus dem Request — genau das etablierte Muster von `/api/view-profiles` (ADR 0023)
und `/api/correlation` (ADR 0024). Damit ist die Selbstverwaltung strukturell auf
das **eigene** Konto begrenzt, unabhängig von der Rolle. Die Handler
(`putMePassword`/`putMeEmail`) sind dieselben, die bisher unter `/api/admin/me/*`
liefen; der neue `Handler.AccountHandler()` exponiert sie an rollen-agnostischer
Stelle.

- **Passwort-Standard:** unverändert `minPasswordLen = 8`, serverseitig erzwungen —
  derselbe Standard wie bei der Admin-Nutzeranlage/-Reset (kein zweiter Maßstab).
- **E-Mail-Validierung:** konservatives Muster (`^[^\s@]+@[^\s@]+\.[^\s@]+$`,
  ≤ 254 Zeichen); ein leerer Wert **löscht** die E-Mail (`NULL`), konsistent mit der
  Admin-Anlage („leer = keine E-Mail"). **Keine** Eindeutigkeits-Pflicht (E-Mail ist
  ein informatives Profilfeld; eindeutig ist der `subject`).
- **Sichtbarkeit im Admin-Panel:** `SetEmail` schreibt dasselbe `users.email`-Feld,
  das die Admin-Zugangstabelle (`AdminUsers.vue`) ohnehin liest → eine
  Selbst-Änderung erscheint dort beim nächsten Laden.
- **Anzeige/Vorbelegung:** `GET /api/whoami` trägt jetzt `email` (fail-soft via
  `GetByID`; immer die **eigene** ID, nie das Impersonation-Ziel). Session- und
  Admin-Store lesen daraus.
- **Zwangswechsel bleibt unangetastet:** Der Pflicht-Passwortwechsel des
  auto-seedeten Admins läuft weiter über das Admin-gegatete, allowlisted
  `/api/admin/me/password`. Nur der Seed-Admin trägt je `must_change_password`
  (`cmd/wayfinder/seed.go`) → für normale Mandanten-Nutzer entsteht durch `pwGate`
  auf `/api/account/*` **kein** Deadlock.

**Kontoöschung** (`DELETE`) bleibt bewusst **admin-seitig** (Dashboard „Mein
Konto"), nicht in der ASD-Selbstverwaltung — eine schwerere Aktion mit
„letzter-Admin"-Guard.

**UI:** Die ASD-Sidebar-Sektion „Konto" bekommt einen Dialog
(`AccountSelfServiceDialog.vue`, Session-Store) für E-Mail + Passwort; das
Admin-Dashboard-„Mein Konto" (`MyAccountPanel.vue`) bekommt zusätzlich das
E-Mail-Feld.

### 2. Layer-Menü-Bedienbarkeit (#315–#318)

- **#315 — Gruppen-Master „fill-then-clear":** `nextMaster` gibt jetzt
  `state !== 'on'` zurück (vorher `state === 'off'`). Ein Klick auf eine leere
  **oder teilweise** aktive Gruppe schaltet **alle** Unterpunkte an („selektieren");
  nur eine bereits vollständig aktive Gruppe wird geleert. Behebt „Selektion von
  Aeronautik de-selektiert alles".
- **#316 — Panelbreite:** Das Sidebar-Panel wird von 248 auf **288 px** verbreitert
  (Desktop; Tablet bleibt 304), und die Karten-Preset-Buttons werden flexibel
  (`min-width:0`), damit „Minimal/Standard/Detailliert" vollständig passen.
- **#317 — Akkordeon (verfeinert ADR 0031):** ADR 0031 ließ Gruppen „start
  expanded". Das führte zu einer scrollenden 2. Ebene. Neu: **nur eine** Gruppe ist
  offen; `LayerGroup` ist jetzt **kontrolliert** (`:expanded` + `@toggle`),
  `LayerFilterContent` hält die einzelne `openGroup`-ID. Öffnen einer Gruppe klappt
  die anderen zu.
- **#318 — Aktiv-Leuchten:** Die Rail-Icons „Layer"/„Filter" leuchten **blau**
  (`--wf-glow-selected`), sobald ihre Sektion ≥ 1 aktives Element hat
  (`hasActiveLayers`/`hasActiveFilter`) — analog zum Amber-Leuchten „scharfer"
  Measure-Tools, unabhängig davon, ob das Panel offen ist.

## Konsequenzen

- **Sicherheit:** Die neue Auth-Fläche ist **eng** — self-scoped, session-basierte
  User-ID, `pwGate` aktiv, keine Rollen-Eskalation möglich (ein Nutzer kann
  ausschließlich sein eigenes Konto ändern). Die E-Mail-Änderung braucht kein
  lokales Passwort und ist damit auch für OIDC/Proxy-Konten nutzbar (anders als die
  Passwort-Änderung, die das aktuelle Passwort verifiziert).
- **Kein Wire-/CAT062-Vertrag betroffen** — rein Browser-Rand + interne Admin-/
  Konto-API. Keine Firefly-Wirkung.
- **Rückverfolgbarkeit:** FR-ADMIN-011, FR-UI-050 im Anforderungs-Register; Tests
  server- und frontendseitig (siehe Register).
- **Determinismus/Multi-Tenant** unberührt (Konto-Ebene, kein Track-Rechenpfad).

## Verworfene Alternativen

- **Nur Admin-Dashboard erweitern (E-Mail in „Mein Konto"):** kleiner, aber
  verfehlt die Kern-Wirkung „E-Mail im Admin-Panel sichtbar" — die gilt nur für
  Mandanten-Nutzer, die das Admin-Dashboard gar nicht erreichen.
- **`me`-Routen komplett aus dem Admin-Gate lösen:** würde den bewährten,
  allowlisteten Zwangswechsel-Pfad anfassen; stattdessen eine **zusätzliche**,
  klar benannte rollen-agnostische Fläche (`/api/account/*`), Admin-Pfad unverändert.
- **E-Mail-Eindeutigkeit erzwingen:** würde Migration + Unique-Constraint
  erfordern; E-Mail ist bewusst ein informatives Feld (eindeutig ist der `subject`).
