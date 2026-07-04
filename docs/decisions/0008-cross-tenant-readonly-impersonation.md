# ADR 0008 — Cross-Tenant Read-Only-Impersonation („View as Tenant")

- **Status:** **ENTWURF — zur Freigabe vorgelegt** (kein Produktivcode, bis
  freigegeben). Paket **WF2-34**.
- **Datum:** 2026-06-23
- **⚠️ Nachtrag (2026-06-24, ADR 0009):** Mit dem Admin-Bereich-Neuschnitt
  (ADR 0009) wird das Rollen-Modell auf **`admin`/`user`** vereinfacht. Jede
  Nennung von **`super_admin`** in diesem ADR ist bei der Implementierung als
  Rolle **`admin`** zu lesen (`requireSuper → requireAdmin`). Der Grant-
  Mechanismus (signiert, HttpOnly, befristet, read-only, auditiert) bleibt
  unverändert.
- **Schnittstellen-relevant:** nein (kein CAT062/065-Draht-Vertrag berührt; rein
  Wayfinder-interner Browser-Rand + Stream-Lese-Scope).
- **Bezug:** **ADR 0005 / NFR-SEC-003** (harte Cross-Tenant-Isolation — wird hier
  **bewusst, eng und kontrolliert** durchbrochen), **ADR 0003** (Browser-Rand,
  Auth, fail-closed), **ADR 0006** (Identität, signiertes Session-Cookie,
  `super_admin`-Rolle, Stateless-Split); Enforcement-Pakete **WF2-21** (Scope),
  **WF2-22** (Property-/Fuzz-Negativtests), **WF2-23** (Audit + Pro-Tenant-Metriken),
  **WF2-33** (Live-Rescope).

> ℹ️ **Nummerierungs-Hinweis:** In der Planung wurde dieser ADR „ADR 0022" genannt.
> Wayfinders eigene ADR-Reihe läuft jedoch **0001–0007** (die Nummern 0015/0017/
> 0018/0021 in `docs/STATUS.md` sind **Fireflys** ADRs, im Cross-Project-Kontext
> zitiert). Daher ist **0008** die korrekte nächste Wayfinder-Nummer. Falls eine
> projektübergreifend fortlaufende Nummerierung gewünscht ist, benenne ich um.

---

## Kontext

Ein **`super_admin`** (Plattform-Betreiber) braucht für **Support und Diagnose**
die Möglichkeit, die ASD-Sicht eines konkreten Mandanten **so zu sehen, wie dieser
Mandant sie sieht** („View as Tenant X") — etwa um ein gemeldetes Darstellungs-
oder Scope-Problem nachzuvollziehen, ohne den Kunden um Screenshots zu bitten.

Das steht in **direktem, gewolltem Spannungsverhältnis zu NFR-SEC-003** (harte
Cross-Tenant-Isolation): Genau die Grenze, die **WF2-21/22** server-seitig
erzwingen und mit **Pflicht-Negativtests** absichern („Mandant A bekommt **nie**
einen Track von Mandant B"), soll hier — **kontrolliert, eng begrenzt, nur lesend,
nur `super_admin`, auditiert und zeitlich befristet** — durchlässig werden.

**Geerdet am heutigen Code:**

- `tenant.Identity = {TenantID, UserID, Subject, Role}` ist die **server-
  autoritative Authentifizierungs-Wahrheit**, aufgelöst aus *Session → User-
  Lookup* (`tenant.Middleware`, `pkg/tenant/tenant.go`). Sie ist **nicht** vom
  Client setzbar — der Vertrauensanker.
- Authentifiziert wird über ein **signiertes Session-Cookie**
  (`auth.BuiltinAuthenticator` + `MintSession`/`ParseSession`, HMAC-SHA256 mit
  Server-Key, eingebaute Expiry) **oder** einen Reverse-Proxy-Header
  (`ProxyAuthenticator`, OIDC, ADR 0006 §5).
- Der **WS-Lese-Scope** wird aus `id.TenantID` aufgelöst (`newScopeResolver` →
  `resolveScope`), der **View-Filter** (Zentrum/Zoom/AOI/FL-Band) aus
  `view_configs.GetEffective(id.TenantID)`.
- **Alle** mutierenden `/api/admin/*`-Pfade sind auf `id.TenantID` **und**
  `RequireRole`/`requireSuper` verankert.

**Die präzise Frage:** Wie schaltet man eine **lesende** Fremd-Mandanten-Sicht
frei, ohne (a) die Identität (Authentifizierungs-Wahrheit) zu verfälschen, (b) die
bestehenden Negativtests zu entwerten, (c) Schreibpfade zu öffnen, (d) die
Billing-/SLA-Metriken des Ziel-Mandanten zu verschmutzen — und das **fail-closed**,
nur für `super_admin`, mit lautem Alarm bei Missbrauch?

---

## Entscheidung

### 1. Identität bleibt unangetastet; Impersonation ist ein *getrennter, expliziter Lese-Scope*

Die `tenant.Identity` wird **niemals** überschrieben. Impersonation ist ein
**zweites, getrenntes Signal** — ein *effektiver Lese-Mandant* (`effReadTenant`) —,
das **ausschließlich** den **Lese-Scope** (Stream + View) überlagert, nie die
Identität und nie einen Schreibpfad.

Der **Default-Pfad** (kein Impersonation-Signal) bleibt **byte-identisch** zu
heute. Genau dadurch bleiben die teuren **WF2-22-Property-/Fuzz-Negativtests
unverändert gültig** — Impersonation ist rein additiv.

### 2. Trägermechanismus: signierter, kurzlebiger Grant als **HttpOnly-Cookie**

„View as Tenant X" ruft **`POST /api/admin/impersonation {tenant_id}`** auf. Der
Server prüft `Role == super_admin` (`requireSuper`) und die **Existenz** des
Ziel-Mandanten und mintet einen **signierten, zeitlich befristeten Grant**
(HMAC-SHA256, **Wiederverwendung des `auth.MintSession`-Musters**; Nutzlast
`{target_tenant_id, exp}`), gesetzt als Cookie **`wf_impersonation`** mit
**`HttpOnly; Secure; SameSite=Strict`** und `Max-Age = TTL`.

**Warum Cookie statt Header/Query-Param:**

- Der **WS-Handshake kann keine Custom-Header** setzen (Browser-`WebSocket`-API);
  ein Cookie reist **nativ** sowohl auf REST als auch auf den **WS-Upgrade** mit.
- Ein `?impersonate=`-**Query-Param leakt** in Zugriffs-Logs, Referrer und
  Browser-History.
- `HttpOnly` verhindert JS-Zugriff (kein XSS-Diebstahl), **signiert** =
  manipulationssicher, **`exp`** = eingebaute **Zeit-Box/Auto-Expiry**.

Dies **ersetzt** die frühere Skizze (`X-Impersonate-Tenant`-Header +
`?impersonate=`-WS-Query) — siehe Verworfene Alternativen.

**Beenden:** **`DELETE /api/admin/impersonation`** löscht das Cookie; der ASD
schließt die WS und verbindet ohne Grant neu (zurück zur echten Sicht).

### 3. Validierung: server-seitig, fail-closed, `super_admin`-only, **laut**

Eine neue Schicht (`pkg/impersonation`, **nach** `tenant.Middleware`) liest das
Grant-Cookie und legt **nur dann** einen `effReadTenant` in den Context, wenn
**alle** Bedingungen erfüllt sind:

1. Signatur gültig **und**
2. nicht abgelaufen **und**
3. **die echte `Identity.Role` ist `super_admin`** (bei jedem Request neu geprüft —
   nicht im Token „eingefroren") **und**
4. der Ziel-Mandant existiert.

- **Verletzung von (3)** — ein Nicht-`super_admin` trägt ein Grant-Cookie: **lautes
  `403` (REST) bzw. WS-Handshake-Reject** **+ Audit-Event
  `event=impersonation_denied`** (Entscheidung: Spoofing-Versuche **sichtbar**
  machen). **Kein** stilles Ignorieren.
- **Fehlt das Cookie** ganz → Default-Pfad, keine Impersonation, **kein** Fehler.
- Ungültige/abgelaufene Signatur → behandelt wie „kein gültiger Grant"
  (Default-Pfad bzw. Reject je nach Pfad), Audit-Notiz.

### 4. Wirkungsbereich: **BEIDES** — Stream-Scope **und** View *(deine Entscheidung 1)*

Unter aktiver Impersonation löst der Lesepfad **vollständig** gegen den
**Ziel-Mandanten** auf:

- WS-Feed-Scope: `resolveScope(..., effReadTenant, ...)`
- View-Filter: `view_configs.GetEffective(effReadTenant)` (Zentrum/Zoom/AOI/FL-Band)

→ Der `super_admin` sieht die Lage **exakt** wie der Ziel-Mandant (gleiche Feeds,
gleicher Ausschnitt, gleiche Filter).

**Snapshot bei Connect (v1)** *(deine Entscheidung 3)*: Der `effReadTenant` wird
**einmal beim WS-Connect** aufgelöst; **kein** Live-Rescope mitten in der Sitzung.
Grant-Wechsel ⇒ Reconnect.

### 5. Schreibpfade bleiben hart auf der echten Identität — **Read-Only-Invariante**

**Alle** mutierenden Endpunkte (`PUT/POST/DELETE /api/admin/*`) verwenden
**ausschließlich** `id.TenantID` (echte Identität), **nie** `effReadTenant`.
Impersonation ist **strukturell read-only**: es existiert **kein** Codepfad, in dem
`effReadTenant` in einen Schreibvorgang fließt. Eine eigene Negativtest-Klasse
sichert das (siehe Konsequenzen).

### 6. Metriken: Impersonation aus den Pro-Tenant-Billing/SLA-Serien **ausschließen** *(deine Entscheidung 2)*

Eine Impersonation-WS-Sitzung darf die `tenant_id`-gelabelten **Billing-/SLA-
Metriken** des Ziel-Mandanten **nicht** erhöhen (sonst verfälschter Verbrauch /
falsche SLA-Zahlen). Impersonation-Connects werden in den Pro-Tenant-Serien
**übersprungen** und stattdessen unter einer **getrennten** Serie gezählt:
`wayfinder_impersonation_sessions_total` (gelabelt mit dem **Akteur**-Tenant, nicht
dem Ziel-Mandanten).

### 7. Audit: jede Impersonation ist lückenlos nachvollziehbar

`logScopeAudit` wird um **`impersonated_tenant_id`** + **`actor_user_id`/
`actor_subject`** (der echte `super_admin`) erweitert; zusätzlich die Events
`impersonation_start` / `impersonation_denied` / `impersonation_end`. Die
Audit-Spur beantwortet jederzeit: *welcher `super_admin` sah wann welchen
Mandanten*.

### 8. UI: Read-Only-Zustand mit **persistentem Banner + Exit + Switcher** *(deine Entscheidungen)*

- Bei aktiver Impersonation zeigt der ASD einen **persistenten Banner**: „Sie
  betrachten Mandant **X** — **nur Lesen**", mit **Exit**-Button und
  **Tenant-Switcher** (Wechsel = neuer Grant + Reconnect).
- **Kein** farbiger Viewport-Rahmen *(deine Entscheidung)*.
- Read-Only an der UI: etwaige mandanten-mutierende Bedienelemente sind im
  Impersonation-Modus deaktiviert; der ASD-Kern ist ohnehin lesend.
- Landing-Flow: `super_admin` startet im Admin-Bereich, wählt dort „View as
  Tenant X" und wird in den (read-only) ASD des Ziel-Mandanten geführt; der Banner
  bringt ihn jederzeit zurück.

---

## Begründung

- **Identität ⟂ Lese-Scope getrennt:** Die Authentifizierungs-Wahrheit bleibt
  unverrückbarer Anker — die Grundlage dafür, dass die Durchbrechung *kontrolliert*
  bleibt und nie versehentlich zu Schreibrechten oder Identitäts-Verwischung führt.
- **Cookie-Grant nutzt vorhandene, geprüfte Krypto** (`MintSession`/HMAC-SHA256):
  kein neues Sicherheits-Primitive, kein Token in URLs/Logs, **nativ WS-tauglich**,
  **eingebaute Expiry** (erfüllt die Zeit-Box-Anforderung ohne Zusatzmechanik).
- **Default-Pfad byte-identisch:** Die WF2-22-Property-/Fuzz-Negativtests bleiben
  **ohne Änderung** gültig; die neue Fähigkeit ist additiv und durch ein eigenes,
  **lautes** Gate abgesichert (Punkt B der Anforderung erfüllt).
- **Lautes Fail-closed** statt stillem Ignorieren macht Missbrauchs-/Spoofing-
  Versuche im Audit sichtbar — passend zur Sicherheitskritikalität eines ASD.

### Verworfene Alternativen

- **Identität überschreiben („`super_admin` *wird* Tenant X"):** Verwischt die
  Authentifizierungs-Wahrheit, riskiert versehentliche Schreibrechte auf den
  Fremd-Mandanten und **entwertet die Negativtests** (der Default-Pfad wäre nicht
  mehr byte-identisch). **Verworfen.**
- **`X-Impersonate-Tenant`-Header + `?impersonate=`-WS-Query (frühere Skizze):**
  Header reisen **nicht** auf dem WS-Handshake; Query-Param **leakt** in
  Logs/Referrer/History; der rohe Tenant-Id ist **unsigniert** (manipulierbar) und
  trägt **keine Expiry**. Durch den signierten HttpOnly-Cookie-Grant **abgelöst**.
- **Server-seitiger Impersonation-State (DB-/Session-Tabelle) mit TTL:**
  Funktioniert und erlaubt **Sofort-Revoke**, fügt aber geteilten Zustand gegen den
  Stateless-Split (ADR 0006) hinzu. Der stateless signierte Grant ist leichter und
  horizontal skalierbar. Für **v1 verworfen**; Option für v2, falls Sofort-Revoke
  einzelner Grants nötig wird.
- **Stilles Ignorieren eines Grants bei Nicht-`super_admin`:** Verbirgt
  Missbrauchsversuche. **Verworfen** zugunsten **lautem 403 + Audit**.

---

## Konsequenzen

- **Neues Paket `pkg/impersonation`:** Grant-Mint/Parse (HMAC-Reuse aus
  `pkg/auth`), Resolve-Middleware, Context-Wert `effReadTenant` (getrennt von
  `tenant`-Context-Key).
- **`cmd/wayfinder`:** `newScopeResolver` + View-Filter nutzen `effReadTenant`,
  falls gesetzt; neue Endpunkte `POST`/`DELETE /api/admin/impersonation`
  (`requireSuper`); Audit-Felder + Metrik-Ausschluss.
- **Neue Negativtests (Gate, NFR-SEC-003):**
  - (a) Nicht-`super_admin` + Grant ⇒ **403/Reject** (nicht still geehrt, nicht
    still ignoriert);
  - (b) `super_admin`@TenantX sieht **genau** TenantX-Scope **und** -View, nichts
    darüber hinaus;
  - (c) Schreibpfad unter aktiver Impersonation trifft **immer den echten** Tenant,
    nie den imitierten;
  - (d) abgelaufener/ungültiger Grant ⇒ Default-Pfad bzw. Reject;
  - die bestehenden **WF2-22-Tests bleiben unverändert grün**.
- **Frontend:** `impersonation`-Pinia-Store + State, Banner-Komponente, Tenant-
  Switcher, Reconnect-Logik bei Grant-Wechsel; `whoami`/Identity liefert die
  Tenant-Liste für den Switcher (nur `super_admin`).
- **Doku:** Anforderungs-Register (FR-SEC-/FR-ADMIN-Einträge, rückverfolgbar),
  `docs/TECHNICAL.md` (neue Endpunkte, Metrik, Env), `docs/INSTALLATION.md` (Env),
  Milestone-Doku `WF2-34_*`.
- **Env:** Der Grant nutzt den **vorhandenen** Session-Signing-Key (kein neues
  Secret zwingend); **TTL** via `WAYFINDER_IMPERSONATION_TTL` (Default z. B.
  `30m`). Ist kein Signing-Key konfiguriert (ModeNone/degenerierter Single-Tenant),
  ist Impersonation **deaktiviert** (fail-closed; ergibt dort auch fachlich keinen
  Sinn).

---

## Ehrliche Grenze

- **Snapshot-v1:** Entzieht man dem Ziel-Mandanten *während* einer laufenden
  Impersonation ein Abo (oder ändert seinen View), sieht der `super_admin` das erst
  **nach Reconnect**. Die **Grant-TTL begrenzt das Fenster**. Live-Revokation /
  Live-Rescope unter Impersonation ist bewusst **v2** (kann später an WF2-33
  andocken).
- **Keine server-seitige Sofort-Revoke einzelner Grants** (Preis der Stateless-
  Lösung): Ein ausgestellter Grant gilt bis `exp`. **Mitigation:** kurze TTL +
  Schlüssel-Rotation als Notbremse (invalidiert schlagartig **alle** Grants und
  Sessions).
- Diese ADR **durchbricht NFR-SEC-003 bewusst und eng begrenzt**. Sie ist die
  **dokumentierte, auditierbare, befristete Ausnahme** für `super_admin`-Support —
  sie hebt die Isolations-Garantie für den **Normalbetrieb nicht** auf und ersetzt
  sie nicht.

---

## Nachtrag (2026-07-04): Grant gilt auch auf den Read-only-REST-Endpunkten der Karte

**Befund (E2E):** Nur der `/ws`-Lesepfad honorierte den Grant. Alle REST-Pfade,
aus denen die Karte ihr Bild zusammensetzt, antworteten weiter für den
(mandantenlosen) `admin` selbst: `/api/whoami` lieferte leere Features →
sämtliche Layer-Schalter verschwanden; `/api/airspace|navaids|waypoints` und
`/api/weather/qnh` liefen gegen Tenant 0 → leer. Ergebnis war eine nackte Karte
mit Tracks — nicht „exakt das, was der Mandant sieht".

**Entscheidung:** Der Grant wird mit **identischer Semantik** (dieselbe
`impersonation.Resolve`-Entscheidung, fail-closed) auf die **rein lesenden**
REST-Endpunkte der Karte ausgedehnt:

- Eine Middleware (`impersonationReadMW`, innerhalb der Tenant-Middleware)
  stempelt bei aktivem Grant den **effektiven Lese-Mandanten** in den
  Request-Kontext (`tenant.WithReadTenant`); die **Identity bleibt unberührt**.
- `/api/whoami` löst Features/Sensor-Klassen/FL-Band/ICAO gegen den
  Ziel-Mandanten auf und legt den Zustand offen
  (`impersonated_tenant_id`, omitempty); Identitätsfelder bleiben die echten.
- `/api/airspace|navaids|waypoints` bedienen den Cache des Ziel-Mandanten;
  das Feature-Gate (PR #158) urteilt über **dessen** Entitlements.
- `/api/weather/qnh` löst den Flugplatz gegen die View des Ziel-Mandanten.
- **Unverändert:** `/api/admin/*` (Admin-UI = echte Identität), alle
  Schreibpfade, `/ws` (hatte die Auflösung bereits), Radar-Kacheln und
  Warnungen (mandanten-unabhängig).

**Fail-closed wie am Handshake:** fehlender/abgelaufener Grant → Pfad
byte-identisch wie ohne Impersonation; gültiger Grant von Nicht-`admin` oder
auf gelöschten Mandanten → **403 + Audit** (`impersonation_denied`);
DB-Fehler bei der Mandanten-Prüfung → 500 (nie stiller Fallback).

**Warum kein Risiko für die Nutzer des Mandanten:** Die Aufschaltung bleibt
strukturell read-only — FL-Filter und Layer-Schalter der Karte sind reiner
Client-Zustand, das ASD schreibt serverseitig nichts außer Session-Renew/
Logout, und jeder Mandanten-Nutzer hat seine eigene View(-Override)-Zeile.
