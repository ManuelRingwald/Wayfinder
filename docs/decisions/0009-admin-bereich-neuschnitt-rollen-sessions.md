# ADR 0009 — Admin-Bereich-Neuschnitt: Rollen `admin`/`user`, Zugangs-Modell & zustandsbehaftete Sessions

- **Status:** **ENTWURF — zur Freigabe vorgelegt** (kein Produktivcode, bis
  freigegeben).
- **Datum:** 2026-06-24
- **Schnittstellen-relevant:** nein (kein CAT062/065-Draht-Vertrag berührt; rein
  Wayfinder-interner Browser-Rand, Identität, Admin-API und Session-Verwaltung).
- **Bezug:** **ADR 0005** (Multi-Mandanten-Pivot, Hybrid-Modell, NFR-SEC-003
  Isolation), **ADR 0006** (Konfig-/Identitäts-Persistenz, **Stateless-Split**,
  `WAYFINDER_AUTH_MODE`, signiertes Session-Cookie, Rollen), **ADR 0003**
  (Browser-Rand, Auth, fail-closed), **ADR 0008** (Cross-Tenant-Impersonation —
  wird auf die neue Rolle `admin` nachgezogen). Anforderungs-Register: bestehende
  **FR-ADMIN-001/002/003**, **FR-TEN-002/003**, **NFR-SEC-003/004**.

> ℹ️ **Auslöser:** Betriebserfahrung des Plattform-Betreibers (2026-06-24). Die
> Admin-Oberfläche (WF2-31/32) ist aktuell ein *Mandanten-Self-Service*
> (`tenant_admin` konfiguriert die *eigene* Sicht) mit einem aufgesetzten
> `super_admin`-Provisioning-Tab. Der reale Bedarf ist ein anderer: **ein
> Plattform-Betreiber**, der **alle** Mandanten anlegt/konfiguriert/überwacht
> und die Zugänge der Kunden verwaltet. Die Self-Service-Annahme passt nicht zum
> Betriebsmodell.

---

## Kontext

### Ist-Zustand (am Code geerdet)

- **Rollen:** `store.Role` kennt heute **`super_admin`**, **`tenant_admin`** und
  (implizit) Operator/Nutzer. Das Admin-API gatet mit
  `RequireRole(tenant_admin, super_admin)` plus in-Handler `requireSuper` für
  cross-tenant-Pfade (FR-ADMIN-001).
- **Admin-UI:** Drei Tabs — *Ansicht* (der eingeloggte `tenant_admin` editiert
  **seine eigene** View: Zentrum/Zoom/AOI/FL/Layer), *Abos & Feeds* (read-only),
  *Provisioning* (nur `super_admin`: Mandant wählen → Feed grant/revoke).
- **Sessions:** **Stateless** — ein signiertes, ablaufendes Cookie (`wf_session`,
  HMAC-SHA256, ADR 0006). Der Server hält **keinen** Session-Zustand; jeder
  Request wird allein aus dem Cookie + DB-User-Lookup aufgelöst
  (`tenant.Middleware`). Es gibt **kein** serverseitiges Logout, **keine**
  Session-Zählung, **kein** sofort wirksames Sperren.
- **Nutzer-Verwaltung:** keine UI. Nutzer/Mandanten werden per Bootstrap-Script /
  CLI angelegt.

### Operativer Bedarf (zwei Kernaufgaben des Betreibers)

**(a) Technische Aufsicht über die ganze Plattform:** Welche Mandanten gibt es,
welche Features hat jeder freigeschaltet (Lufträume, Range-Rings, History-Dots,
FL-Band, VOR/NDB, …), und sind die Feeds gesund — empfängt Firefly die Daten,
empfängt Wayfinder die Tracks?

**(b) Read-only-Aufschaltung auf einen Mandanten** zur Fehlerdiagnose („sehe ich,
was der Kunde sieht") — bereits in **ADR 0008** entworfen.

Dazu die **Zugangs-Verwaltung**: der Betreiber legt pro Kunde einen oder mehrere
**Zugänge** an (Login-Konten), kann sie **pausieren** (Login gesperrt, Konfig
bleibt), **löschen**, **Passwörter** setzen, und je Zugang ein **Limit gleich­
zeitiger Sessions** vergeben.

---

## Entscheidung

### 1. Rollen-Modell auf **zwei** Rollen reduzieren: `admin` und `user`

`store.Role` wird auf genau zwei Werte vereinfacht:

| Rolle | Wer | Darf |
|-------|-----|------|
| **`admin`** | Plattform-Betreiber | Alles im Admin-Bereich: Mandanten/Feeds/Features/Zugänge verwalten, Feed-Gesundheit sehen, „View as Tenant" |
| **`user`** | Endnutzer (Lotse) eines Mandanten | Nur das ASD des eigenen Mandanten bedienen (Layer/Filter im ASD) |

- **`super_admin` → `admin`** (umbenannt; behält die Plattform-Reichweite).
- **`tenant_admin` → entfällt.** Die bisherige Self-Service-Konfiguration (eigene
  View editieren) wird **zentral** zum `admin` verlagert (Punkt 3). Bestehende
  `tenant_admin`-Konten werden per Migration auf **`user`** gesetzt (sie verlieren
  den Admin-Zugang — gewollt; der Betreiber ist künftig die einzige Admin-Instanz).
- **Datenmigration:** eine Schema-/Daten-Migration bildet
  `super_admin → admin`, `tenant_admin → user`, `operator/null → user` ab. Der
  Role-Constraint/Enum wird auf `{admin, user}` verengt.

### 2. Domänenmodell: **Mandant → Zugänge**

| Ebene | Trägt |
|-------|-------|
| **Mandant** (tenant) | Feed-Abo(s), View-Konfig (Center/Radius→AOI, FL-Band min/max), Feature-Set, Status (aktiv/pausiert) |
| **Zugang** (account = `users`-Zeile, Rolle `user`) | Subject, Passwort-Hash, **Status** (aktiv/pausiert), **Session-Limit**, Mandanten-Zugehörigkeit |
| **Admin** | Plattform-global, **nicht** an einen Kunden-Mandanten gebunden |

- Ein Mandant hat **1..n** Zugänge; alle Zugänge eines Mandanten teilen dessen
  Feed/View/Feature-Konfig (**Annahme A1**, vom Betreiber bestätigt).
- **Pausieren auf beiden Ebenen** (**A2**): Zugang pausieren (nur dieser Login
  gesperrt) **und** Mandant pausieren (kaskadiert auf alle Zugänge). Konfig bleibt
  jeweils erhalten. Umsetzung über ein `status`-Feld (`active`/`paused`), das der
  Login-Pfad **und** die Session-Auflösung prüfen (fail-closed: `paused` ⇒ kein
  Login, bestehende Sessions werden invalidiert — siehe Punkt 5).
- **Admin-Verortung (A3):** Der `admin` ist ein User mit Rolle `admin` ohne
  Kunden-Mandanten-Bindung (technisch: dedizierter Plattform-Mandant **oder**
  `tenant_id NULL` — die Detail-Festlegung erfolgt im AP1-Häppchen am Schema, ohne
  weitere Architektur-Entscheidung).

### 3. Admin-Bereich wird **mandantenzentriert** und zentral konfigurierend

Statt „eigene View + Provisioning-Tab" bietet der Admin-Bereich eine
**Mandanten-Übersicht** und pro Mandant eine **Detailansicht**, in der der `admin`
**zentral** konfiguriert:

- **Feed-Verknüpfung** (welche Feeds: ADS-B / PSR/SSR / …),
- **Sicht**: **Center-Punkt + Radius (NM)** im UI → **clientseitig** in die
  bestehende **AOI-Bounding-Box** (`PUT /api/admin/view`, WF2-21.2) konvertiert.
  Das **Backend bleibt unverändert** AOI-basiert; nur das UI rechnet Center+Radius
  ↔ Bbox (Begründung: UX-freundlicher; kein neues API-Feld). Rückrichtung: aus der
  gespeicherten Bbox wird beim Laden ein Näherungs-Radius zurückgerechnet.
- **FL-Band min/max** pro Mandant (**A4**; bleibt Min/Max, kein Boolean),
- **Feature-Set** (Entitlements, Punkt 4),
- **Zugänge** des Mandanten (Punkt 5).

Der **Nutzer** schaltet im ASD weiterhin Layer/Filter über die **bestehende
Filterfunktion** ein/aus — die Admin-Konfig setzt nur, **was** verfügbar ist und
**wo** (Center/Radius) die Lage liegt.

### 4. Feature-Katalog erweitern (parametrierbar, fail-closed bleibt)

Der getypte Katalog (`pkg/feature`, default-deny, fail-closed) wächst um die real
benötigten ASD-Features: `airspaces` (Lufträume/CTR …), `range_rings`,
`history_dots`, `vor_ndb`, `waypoints` (Liste wächst weiter). **FL-Band** ist
**kein** Boolean, sondern ein **parametrierter Wert** (min/max) und wird daher in
der **View-Konfig** des Mandanten geführt (wie Center/Zoom), nicht als
Boolean-Entitlement. Der bestehende fail-closed-Mechanismus bleibt unangetastet.

### 5. **Zustandsbehaftete Sessions** — bewusste, eng begrenzte Abkehr vom Stateless-Split

Session-Limit (**„max N gleichzeitig"**) **und** sofort wirksames Pausieren/Löschen
verlangen, dass der Server **aktive Sessions kennt, zählt und widerrufen kann**.
Ein rein signiertes, stateless Cookie (ADR 0006) kann das **prinzipiell nicht**
(es lässt sich weder zählen noch vor Ablauf invalidieren).

**Entscheidung:** Einführung einer **serverseitigen Session-Registry**
(DB-gestützt, neue Tabelle `sessions`: `id`, `user_id`, `created_at`,
`last_seen_at`, `expires_at`, ggf. `client_meta`). Das Cookie trägt künftig eine
**Session-ID** (weiterhin signiert/HttpOnly/Secure/SameSite), der Request wird
gegen die Registry aufgelöst.

Damit erschlägt **ein** Mechanismus drei Anforderungen:

1. **Session-Limit:** beim Login wird die Zahl aktiver Sessions des Zugangs
   geprüft; bei Überschreitung **fail-closed** (Login abgelehnt) **oder** älteste
   Session verdrängt — **Policy pro Zugang konfigurierbar**, Default „ablehnen".
2. **Sofort-Pause/-Löschung:** ein pausierter/gelöschter Zugang ⇒ seine Sessions
   werden invalidiert, der nächste Request fliegt fail-closed raus.
3. **Echtes serverseitiges Logout** (Nebengewinn).

**Warum DB-gestützt (nicht in-memory):** Wayfinder zielt auf **Kubernetes /
horizontale Skalierung** (ADR 0007). Mehrere Instanzen müssen denselben
Session-Zustand sehen — ein In-Memory-Registry pro Pod würde Limit und Revoke
unterlaufen. PostgreSQL ist bereits die Zustands-Heimat (ADR 0006); Redis bleibt
wie dort **zurückgestellt** (eine Option, falls die Lookup-Last es später
rechtfertigt).

**Bewusste Konsequenz für ADR 0006:** Der „Stateless-Split" wird für **Sessions**
aufgegeben (ein DB-Lookup pro Request statt rein stateless). Das ist bei der
Zielgröße (Plattform-Betreiber + Kunden-Lotsen, nicht Massen-Internet) unkritisch
und der **Preis für hartes Session-Limit + Sofort-Revoke**, die anders nicht
erreichbar sind. **ADR 0008** (Impersonation-Grant) bleibt ein **separater**,
weiterhin stateless signierter Grant — die Session-Registry betrifft nur die
Authentifizierungs-Session, nicht den Impersonation-Lese-Scope.

### 6. ADR 0008 wird auf `admin` nachgezogen

Alle `super_admin`-Vorkommen in ADR 0008 / `pkg/impersonation` / Endpunkten
(`POST/DELETE/GET /api/admin/impersonation`) werden auf die Rolle **`admin`**
umgestellt. Der Grant-Mechanismus selbst bleibt unverändert (signiert, HttpOnly,
befristet, read-only, auditiert).

---

## Begründung

- **Zwei Rollen bilden das reale Betriebsmodell ab.** Es gibt genau einen
  Plattform-Betreiber-Typ und einen Endnutzer-Typ; die Drei-Rollen-Konstruktion
  war ein Self-Service-Erbe, das nie zum Einsatz kam.
- **Mandant→Zugang** trennt sauber: Konfiguration lebt am Mandanten (einmal je
  Kunde), Login/Limit/Status leben am Zugang (mehrere je Kunde). Das deckt den
  Paderborn-Workflow direkt ab.
- **Center+Radius im UI, AOI im Backend** ist die UX-richtige Schicht-Trennung:
  Betreiber denkt in „Umkreis um den Platz", das Backend bleibt bei der erprobten,
  getesteten AOI-Filterung (WF2-21.2) — kein Eingriff in den isolations-kritischen
  Pfad.
- **DB-Session-Registry** ist der einzige Weg zu echtem Limit + Sofort-Revoke und
  passt zur ohnehin vorhandenen PostgreSQL-Zustandsheimat und zum K8s-Ziel.

### Verworfene Alternativen

- **Drei Rollen behalten / nur umbenennen:** löst den fachlichen Fehlschnitt nicht
  (Self-Service vs. zentrale Betreiber-Verwaltung). **Verworfen.**
- **Stateless bleiben + „weiches" Limit:** technisch unmöglich — stateless Cookies
  sind weder zählbar noch widerrufbar. **Verworfen.**
- **In-Memory-Session-Registry:** bricht unter horizontaler Skalierung (Limit/
  Revoke pro Pod statt global). **Verworfen** zugunsten DB; Redis als spätere
  Option offen.
- **FL-Band als Boolean-Entitlement:** verliert die Min/Max-Parameter, die der
  Betreiber pro Mandant setzen will. **Verworfen** — FL-Band bleibt in der
  View-Konfig.
- **Center/Radius als neues Backend-API-Feld:** unnötiger Eingriff in den
  getesteten AOI-Pfad; die Konversion ist reine UI-Ergonomie. **Verworfen.**

---

## Konsequenzen

Umsetzung in **Arbeitspaketen** (eigene Ankündigung + Freigabe je Paket, Charter
§3). Reihenfolge nach Abhängigkeit:

| AP | Inhalt | Stufe |
|----|--------|-------|
| **AP1** | Rollen `admin`/`user`: Schema-/Daten-Migration, `requireSuper→requireAdmin`, `RequireRole`-Anpassung, Frontend entgittert; ADR 0008 nachgezogen | S3 |
| **AP6** | Zugangs-Verwaltung: CRUD je Zugang (anlegen, pausieren, löschen, Passwort setzen/zurücksetzen), Mandant-Pause kaskadiert | S3 |
| **AP7** | Session-Registry (`sessions`-Tabelle), Limit-Enforcement beim Login, Sofort-Pause/-Revoke/-Logout | S4 |
| **AP2** | Feature-Katalog erweitern (`airspaces`/`range_rings`/`history_dots`/`vor_ndb`/`waypoints`); FL-Band min/max in View-Konfig | S3 |
| **AP3** | Admin-Dashboard: Mandanten-Übersicht (Features+Feeds+Zugänge), Detail-Edit, Center+Radius↔AOI-Konversion | S3 |
| **AP4** | Feed-Gesundheit pro Feed/Mandant (sendet Firefly? empfängt Wayfinder?), Ampel im Dashboard | S3–S4 |
| **AP5** | „View as Tenant" (ADR 0008 implementieren, auf `admin`) | S4 |

- **Schema:** Migration für Role-Verengung; neue `sessions`-Tabelle; `users` um
  `status` + `session_limit` erweitert; ggf. `tenants.status`.
- **Backend:** `pkg/auth`/`pkg/tenant` (Session-Registry, Status-Prüfung),
  `pkg/adminapi` (Zugangs-/Mandanten-Endpunkte, Feed-Health), `pkg/feature`
  (Katalog), `pkg/impersonation` (Rolle `admin`).
- **Frontend:** Admin-Bereich neu (Mandanten-Übersicht + Detail), Zugangs-
  Verwaltung, Center/Radius-Eingabe; `super_admin`/`tenant_admin`-Gating entfernt.
- **Doku:** Register (neue FR-ADMIN-/FR-TEN-/NFR-SEC-Einträge, rückverfolgbar),
  `docs/TECHNICAL.md` (neue Endpunkte/Tabellen/Env), `docs/INSTALLATION.md` +
  `docs/BETRIEB.md` (geänderter Admin-Workflow, Session-Limit), Milestone-Doku je
  AP. **GitHub-Issues** je AP als Backlog-Spiegel.
- **Env:** ggf. `WAYFINDER_SESSION_LIMIT_DEFAULT` (Default-Limit je Zugang) und
  eine Limit-Überschreitungs-Policy (`reject`/`evict_oldest`).

---

## Ehrliche Grenze

- **Breaking für bestehende Konten:** `tenant_admin`-Konten verlieren den
  Admin-Zugang (werden `user`). Das ist gewollt, aber ein Datenmigrations-Schritt,
  der vor dem Rollout kommuniziert werden muss.
- **Session-Lookup-Last:** ein DB-Lookup pro authentifiziertem Request. Bei der
  Zielgröße unkritisch; sollte die Last wachsen, ist ein Session-Cache/Redis die
  dokumentierte nächste Stufe (nicht Teil dieses ADR).
- **Kein SSO-/OIDC-Umbau:** Der Proxy-Auth-Pfad (ADR 0006 §5) bleibt bestehen; die
  Session-Registry betrifft primär den `builtin`-Modus. Wie Session-Limit und
  externes OIDC zusammenspielen, ist hier **nicht** entschieden (offener Punkt,
  falls künftig OIDC produktiv wird).
- Dieser ADR entscheidet die **Richtung**; die genaue Schema-Form je Tabelle und
  die Limit-Policy-Defaults werden im jeweiligen AP-Häppchen festgelegt.
