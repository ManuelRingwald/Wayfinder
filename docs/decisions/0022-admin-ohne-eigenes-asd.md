# ADR 0022 — Admin ohne eigenes ASD: Gastmodus als einziger Lagebild-Zugang, Passwort-Gate pfad-unabhängig

- **Status:** **AKZEPTIERT** ✅ (2026-07-07).
- **Datum:** 2026-07-07
- **Schnittstellen-relevant:** **nein** — der **CAT062/063/065-Draht-Vertrag mit
  Firefly bleibt unverändert**. Betroffen ist ausschließlich der Browser-Rand
  (Auth/Autorisierung am `/ws`- und REST-Zugang).
- **Bezug:** **Issue #208** (Anker der Admin-/Mandanten-UX-Serie #208–#212),
  **ADR 0008** (Cross-Tenant-Read-Only-Impersonation — der „Gastmodus"),
  **ADR 0009** (Admin-Bereich-Neuschnitt, Rollenmodell), **ADR 0011 / ONB-1**
  (Zero-Touch-Auto-Seed mit `must_change_password`), **Migration 00007**
  (admin XOR tenant als DB-Invariante), **#209** (Gastmodus-Augen-Icon als
  einziger Einstieg), CLAUDE.md §7 (Sicherheit: Browser-Rand absichern).
- **Anforderungs-Register:** **NFR-SEC-006** (dieser Schritt).

> ℹ️ **Auslöser:** Betreiber-Befund (Issue #208): (a) Der auto-geseedete Admin
> (`admin`/`admin`, `must_change_password`) konnte sich über die ASD-Login-Karte
> unter `/` anmelden, **ohne** zur Passwortänderung gezwungen zu werden — die
> Zwangs-Maske existierte nur unter `/admin`, und die Allowlist-Sperre griff nur
> für `/api/admin/*`. (b) Der Admin erreichte über `/` ein eigenes (leeres)
> Lagebild — ein mandantenloser Betreiber-Account soll aber **gar kein eigenes
> ASD** besitzen.

---

## Kontext

Wayfinder ist multi-tenant (ADR 0014): jedes Lagebild ist die Sicht **eines
Mandanten** (dessen Feeds, dessen AOI/FL-Band, dessen Features). Ein
Plattform-Admin ist seit ONB-3/Migration 00007 **mandantenlos** (DB-Invariante
`admin ⇒ tenant_id IS NULL`). Ein „eigenes" Admin-ASD wäre damit strukturell ein
ungescoptes Bild — genau das, was das Multi-Tenant-Modell ausschließt. Bisher
wurde das weich gelöst: `TenantID 0` ergab ein leeres Bild („gewollt"), und der
Read-Only-Gastmodus (ADR 0008) war ein optionaler Zusatz.

Für den erzwungenen Passwortwechsel (ONB-1) galt: Die bekannten
Seed-Zugangsdaten sollen „für genau eine Aktion gültig" sein — die, die sie
ersetzt. Durchgesetzt war das nur in `pkg/adminapi` (Allowlist
`whoami`/`me`/`me/password` für `/api/admin/*`); `/ws`, `/api/whoami` und die
Daten-Routen (Wetter, Overlays, Airports/Runways) liefen daran vorbei.

## Entscheidung

**1. Ein Plattform-Admin hat kein eigenes Lagebild — der Server erzwingt das.**
Der `/ws`-Scope-Resolver lehnt den Handshake eines Admins **ohne aktives**
Impersonation-Grant fail-closed ab (403, Audit-Event `ws_admin_denied`). Das
gilt auch bei **abgelaufenem** Grant und bei **platform-weit deaktivierter**
Impersonation (kein `WAYFINDER_SESSION_KEY`): Der frühere Fallback „leeres
eigenes Bild" entfällt ersatzlos. Der **einzige** Lagebild-Zugang des Admins
ist der Read-Only-Gastmodus (ADR 0008) über einen echten Mandanten, gestartet
über das Augen-Icon der Mandanten-Übersicht (#209).

**2. Das Passwort-Gate ist pfad-unabhängig.** Die neue Middleware
`tenant.RequirePasswordChanged` weist jede Anfrage eines Principals mit
gesetztem `must_change_password` mit `403 password_change_required` ab und ist
vor **alle operativen Daten-Pfade** montiert: `/ws`, `/api/airspace|navaids|
waypoints`, `/api/weather/*`, `/api/airports.geojson`, `/api/runways.geojson`.
Erreichbar bleiben nur: `/api/login`/`/api/logout` (Sitzungsauf-/-abbau),
`GET /api/whoami` (der SPA-Probe-Punkt, der den Flag **meldet**),
`POST /api/session/renew` (hält die Sitzung während der Maske) und die
bestehende adminapi-Allowlist (`/api/admin/whoami`, `/api/admin/me`,
`PUT /api/admin/me/password`). Damit ist die Zusage aus `seed.go` wieder wahr —
egal über welche URL sich der Principal anmeldet.

**3. Das Frontend führt, der Server erzwingt.** Das ASD unter `/` gated die
Karte **nach** der Authentifizierung (`adminGate`): ein Principal mit
`must_change_password` wird nach `/admin` (Zwangs-Maske) umgeleitet; ein Admin
ohne aktiven Gastmodus ebenso. Der Spinner hält, bis das Gate entschieden ist,
sodass nie ein zum Scheitern verurteiltes `/ws` öffnet. „Beenden" im
Gastmodus-Banner führt nach `/admin` zurück; läuft das Grant während der
Sitzung ab (TTL), erkennt der Connection-Drop-Handler das und kehrt ebenfalls
nach `/admin` zurück. Der „Zur Lage"-Shortcut der Admin-App-Bar entfällt.
Diese UI-Führung ist Komfort — **die Grenze ist der Server** (Punkte 1–2).

**4. Altstand.** Bereits erledigt: Migration 00007 hat bestehende Admins von
ihrem Mandanten gelöst und erzwingt die Invariante per CHECK-Constraint. Es
gibt keinen Bestands-Admin mit Mandanten-ASD; eine neue Migration ist nicht
nötig.

## Begründung

- **„Sehen heißt aufschalten":** Der Blick des Betreibers auf die Lage ist
  immer der Blick *eines Mandanten* — auditiert (`impersonation_start`/`_end`,
  `ws_connect` mit `impersonated_tenant_id`), zeitlich befristet
  (`WAYFINDER_IMPERSONATION_TTL`) und strukturell read-only. Ein ungescoptes
  oder leeres Sonder-Bild hat keinen operativen Wert und wäre ein
  Sonder-Codepfad genau der Art, die ADR 0014 verbietet.
- **Fail-closed statt Konvention:** Die Passwortwechsel-Zusage hing an der
  URL-Wahl des Nutzers. Sicherheitszusagen, die von der Einstiegsroute
  abhängen, sind keine — daher ein Gate an jedem Daten-Pfad.
- **Defense-in-Depth:** UI-Redirect (Komfort) + `/ws`-Ablehnung (Datengrenze) +
  Passwort-Gate (Credential-Grenze) + DB-Invariante (Datenmodell-Grenze).

## Konsequenzen

- Ein Admin, der `/` aufruft, landet in `/admin`; die Karte sieht er nur im
  Gastmodus. Im **Proxy-Auth-Modus ohne Session-Key** (Impersonation
  deaktiviert) hat ein Admin **gar keinen** Lagebild-Zugang — dokumentierte
  Betriebsbedingung, kein Bug.
- Semantik-Änderung am `/ws`-Verhalten: Admin + abgelaufenes/fehlendes Grant →
  **403** statt leeres Bild. Betroffene Tests wurden entsprechend nachgezogen
  (`TestScopeResolverAdminWithoutGrantRejected`,
  `TestScopeResolverImpersonationExpiredFallsBack` — Fallback nur noch für
  User, `TestScopeResolverImpersonationDisabledWithoutKey`).
- `GET /api/whoami` bleibt für geflaggte Principals erreichbar (liefert
  `must_change_password`, damit die SPA die Maske ansteuern kann) — es gibt
  keine Track-/Lagedaten preis.
- Kein Wire-/ICD-Einfluss; Firefly unberührt.

## Verifikation

- `pkg/tenant/authz_test.go::TestRequirePasswordChanged` (4 Fälle, Marker).
- `cmd/wayfinder/scope_test.go` (Admin-Ablehnung ohne/mit abgelaufenem Grant,
  Admin-Zulassung mit aktivem Grant, User-Fallback, Impersonation-aus-Fall).
- `frontend/src/views/__tests__/asdAdminGate.test.js` (Redirect-Gate, TTL-Drop,
  Gastmodus-Exit, entfernter Shortcut).
