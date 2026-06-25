# ADR 0011 — Zero-Touch-Onboarding: Auto-Admin beim Boot, Selbst- & Plattform-Verwaltung über die Admin-Oberfläche

- **Status:** **ENTWURF — zur Freigabe vorgelegt** (kein Produktivcode, bis
  freigegeben).
- **Datum:** 2026-06-25
- **Schnittstellen-relevant:** nein (kein CAT062/063/065-Draht-Vertrag mit
  Firefly berührt; rein Wayfinder-interner Bootstrap, Browser-Rand, Admin-API
  und Konfiguration).
- **Bezug:** **ADR 0009** (Admin-Bereich-Neuschnitt: Rollen `admin`/`user`,
  Mandant→Zugang, zustandsbehaftete Sessions — dieser ADR baut direkt auf der
  dort entworfenen Admin-API auf), **ADR 0006** (Konfig-/Identitäts-Persistenz,
  `WAYFINDER_AUTH_MODE`, Session-Cookie), **ADR 0005** (Multi-Mandanten),
  **ADR 0003** (Browser-Rand, fail-closed), **ADR 0004** (OpenAIP-Datenquelle —
  wird hier von *global* auf *pro Mandant* erweitert). Anforderungs-Register:
  bestehende **FR-ADMIN-001/002/003**, **FR-TEN-002/003**, **NFR-SEC-003/004**.

> ℹ️ **Auslöser:** Betriebserfahrung des Plattform-Betreibers (2026-06-25). Eine
> frische Instanz ist heute **nicht** ohne Terminal benutzbar: Erst
> `wayfinder bootstrap` (erster Mandant + Admin), dann `wayfinder feed add`
> (Feed-Katalog), dann pro Kunde ein Mandant — alles über die CLI. Der erklärte
> Zielzustand ist: **„nur die Container von Wayfinder und Firefly starten, und
> alles andere aus der Admin-Oberfläche heraus konfigurieren."**

---

## Kontext

### Ist-Zustand (am Code geerdet)

- **Bootstrap nur über CLI.** `wayfinder bootstrap` (`cmd/wayfinder/bootstrap.go`)
  legt idempotent Mandant + Admin-User + Credential an; `wayfinder feed add`
  (`cmd/wayfinder/feedcmd.go`) legt einen Feed im Katalog an. Beide verlangen
  `WAYFINDER_DB_URL` und einen Terminal-Zugriff auf den Container.
- **Auth-Default ist `none`** (`auth.ParseMode`, `WAYFINDER_AUTH_MODE`). In
  `none` ist jeder Request derselbe feste Subject — es gibt **keinen Login**,
  also auch keine Selbstverwaltung. Multi-Mandant/Admin-UI verlangen `builtin`
  (Session-Cookie, `WAYFINDER_SESSION_KEY`) **plus** eine Datenbank.
- **Admin-API (ADR 0009, implementiert).** Vorhanden sind: Mandanten-Liste,
  Mandanten-Detail (View/Abos/Entitlements/Status), **User-CRUD pro Mandant**
  (`POST/PATCH/DELETE .../users`, `PUT .../users/{id}/password`), Feed-Liste,
  Feed-Gesundheit, Overview.
- **Es fehlen** für den Zielzustand genau fünf Bausteine:
  1. **Auto-Seed beim Boot** (kein Terminal nötig),
  2. **Selbstverwaltung** des eingeloggten Kontos (`/api/admin/me/*`:
     Passwort ändern, eigenes Konto löschen),
  3. **Mandanten anlegen/löschen** über die API (heute nur Liste + Status),
  4. **Feeds anlegen/löschen/zuweisen** über die API (heute nur CLI + Liste),
  5. **OpenAIP pro Mandant** (heute ein globaler `WAYFINDER_OPENAIP_API_KEY`,
     ADR 0004).

### Spannungsfeld

Wayfinder ist **sicherheitsrelevant** (ASD). „Zero-Touch" darf die
Vertrauensgrenze nicht aufweichen. Der bisherige Default `none` setzt bewusst
auf **Netz-Isolation** (ADR 0003): kein Login, weil der Browser-Rand als
isoliert angenommen wird. Sobald die Instanz ohne Terminal **konfigurierbar**
sein soll, braucht sie einen **Login** — und ein Login mit einem **bekannten
Default-Passwort** ist nur dann vertretbar, wenn der erste Schritt nach dem
Login der **erzwungene Passwortwechsel** ist.

---

## Entscheidung

### 1. Gebündeltes Deployment fährt `builtin` + Auto-Seed (F2 = docker-compose mit Postgres)

Wayfinder bekommt ein **`docker-compose.yml`** im Repo, das zwei Dienste startet:

- **`db`** — PostgreSQL (gepinnte Version, benanntes Volume für Persistenz),
- **`wayfinder`** — die Anwendung, vorkonfiguriert mit
  `WAYFINDER_AUTH_MODE=builtin`, `WAYFINDER_DB_URL` auf den `db`-Dienst und einem
  beim Start erzeugten/zu setzenden `WAYFINDER_SESSION_KEY`.

Damit gilt: **`docker compose up` genügt.** Der Default-Auth-Modus des **Binaries
bleibt `none`** (Netz-Isolation als bewusster Default, ADR 0003 unverändert) —
nur das **Compose-Profil** wählt `builtin`. So ändert sich für bestehende
`none`-Deployments nichts; das Zero-Touch-Erlebnis lebt im Compose-Profil.

> **Session-Key-Bequemlichkeit:** Ist in `builtin` kein `WAYFINDER_SESSION_KEY`
> gesetzt, erzeugt Wayfinder beim Start einen zufälligen Key und **warnt** (die
> Sessions überleben dann keinen Neustart und sind nicht multi-Replica-fähig).
> Für Produktion/horizontale Skalierung bleibt ein **fest gesetzter** Key die
> dokumentierte Empfehlung. Das Compose-File setzt einen festen Key vor.

### 2. Auto-Seed beim Boot: Default-Mandant + Default-Admin (F1 = admin/admin + Pflichtwechsel)

Beim Start führt Wayfinder **in `builtin`-Modus mit gesetzter DB** einen
idempotenten **Auto-Seed** aus (Wiederverwendung von `runBootstrap`):

- Existiert **noch kein einziger Admin**, wird angelegt:
  - ein **Default-Mandant** (Slug `default`),
  - ein **Default-Admin** mit Subject **`admin`**, Passwort **`admin`** und dem
    neuen Flag **`must_change_password = true`**.
- Existiert bereits irgendein Admin, passiert **nichts** (idempotent, kein
  Zurücksetzen eines schon gewechselten Passworts).

**Pflicht-Passwortwechsel (fail-closed).** Ein Konto mit
`must_change_password = true` darf sich zwar **einloggen**, aber **jeder**
Admin-API-Zugriff außer „eigenes Passwort ändern" (`PUT /api/admin/me/password`)
und „abmelden" wird mit **HTTP 403 + Marker** abgewiesen. Das Frontend leitet
nach dem Login zwingend auf die Passwort-ändern-Maske. Der erfolgreiche Wechsel
setzt das Flag auf `false`. Damit ist das bekannte Default-Passwort `admin/admin`
**nur für genau einen Schritt** gültig — den, der es ersetzt.

> **Warum ein bekanntes Default-Passwort und kein Env-Pflicht-Secret?** Der
> Betreiber-Wunsch ist „nur Container starten" — ein Pflicht-Env-Secret würde
> genau diesen Reibungspunkt wieder einführen. Der Pflichtwechsel beim ersten
> Login schließt die Lücke des bekannten Defaults, ohne eine
> Start-Voraussetzung zu erzwingen. Begründung der Sicherheits-Abwägung in
> Abschnitt **Begründung**.

### 3. Selbstverwaltung des eigenen Kontos (`/api/admin/me/*`)

Neue, **rollen-unabhängige** Endpunkte für das **eingeloggte** Konto (kein
`requireAdmin`; jeder authentifizierte Nutzer verwaltet sein eigenes Konto):

| Methode & Pfad | Wirkung |
|---|---|
| `GET /api/admin/me` | eigene Konto-Stammdaten (Subject, Rolle, Mandant, `must_change_password`) |
| `PUT /api/admin/me/password` | eigenes Passwort ändern (verlangt aktuelles Passwort; setzt `must_change_password=false`) — **auch im Pflichtwechsel-Zustand erlaubt** |
| `DELETE /api/admin/me` | eigenes Konto löschen — **mit „letzter aktiver Admin"-Guard** (Abschnitt 4) |

### 4. Admins anlegen/löschen/pausieren über die API — mit „letzter Admin"-Guard

Die bestehende `createUser`/`deleteUser`/`setUserStatus`-Maschinerie (ADR 0009)
wird so erweitert, dass der Betreiber **weitere Admins** anlegen, pausieren und
löschen kann. Verbindlicher Sicherheits-Invariant:

- **„Letzter aktiver Admin"-Guard.** Das Löschen oder Pausieren des **letzten
  aktiven Admins** wird **fail-closed** abgewiesen (HTTP 409). Es darf nie ein
  Zustand entstehen, in dem sich niemand mehr administrativ einloggen kann
  (Selbst-Aussperrung). Gilt für `DELETE /api/admin/me`, `DELETE …/users/{id}`
  und das Pausieren via `PATCH …/users/{id}`.

### 5. Mandanten anlegen/löschen über die API

| Methode & Pfad | Wirkung |
|---|---|
| `POST /api/admin/tenants` | neuen Mandanten anlegen (Slug + Name), `requireAdmin` |
| `DELETE /api/admin/tenants/{tenantID}` | Mandanten löschen, `requireAdmin` |

- `store.TenantRepo` bekommt **`Delete`**; das Löschen **kaskadiert** sauber auf
  abhängige Zeilen (Zugänge, Abos, Entitlements, View-Konfig, OpenAIP-Konfig) —
  per `ON DELETE CASCADE` bzw. expliziter Transaktion.
- **Guard:** Der **Default-Mandant** (oder ein Mandant mit noch aktiven Admins)
  wird gegen versehentliches Löschen geschützt (fail-closed, HTTP 409), damit
  der „letzter Admin"-Guard nicht über den Umweg „Mandant löschen" umgangen wird.

### 6. Feeds anlegen/löschen/zuweisen über die API — mit Live-Receiver-Wirkung

| Methode & Pfad | Wirkung |
|---|---|
| `POST /api/admin/feeds` | neuen Feed im Katalog anlegen (Name, Gruppe, Port, Region, Sensor-Mix) — Logik aus `feedAddCommand` als gemeinsame Funktion |
| `DELETE /api/admin/feeds/{feedID}` | Feed aus dem Katalog entfernen |

- `store.FeedRepo` bekommt **`Delete`**. Die **Zuweisung** eines Feeds an einen
  Mandanten existiert bereits (`POST/DELETE …/tenants/{id}/subscriptions`,
  ADR 0009) und wird wiederverwendet.
- **Live-Wirkung auf den Receiver.** Heute werden die Receiver **einmalig beim
  Start** aus dem Katalog gebaut (`buildReceivers`). Anlegen/Löschen über die UI
  muss **ohne Neustart** wirken: ein **Feed-Manager** joint/verlässt die
  Multicast-Gruppe zur Laufzeit (Receiver-Goroutine starten/stoppen). Das ist
  der **anspruchsvollste** Baustein (Nebenläufigkeit, sauberes Goroutine-
  Lifecycle, kein Datagramm-Verlust/Doppel-Join) → **ONB-5, S3–S4**.

### 7. OpenAIP pro Mandant (F3 = Pro-Mandant Key + eigener AOI-Cache)

ADR 0004 hält den OpenAIP-Key **global** (`WAYFINDER_OPENAIP_API_KEY`) und cached
**eine** Region. Für Multi-Mandant wird das **pro Mandant** geführt:

- Neue Konfiguration **pro Mandant** (DB): OpenAIP-**API-Key** und **Area of
  Interest** (die ohnehin pro Mandant vorhandene View-AOI wird wiederverwendet).
- Der `pkg/aeronautical`-Service wird von „ein globaler Client + ein Cache" auf
  **„ein Client + Cache je Mandant"** erweitert (Key je Mandant, Cache-Bucket je
  Mandant-AOI). Die Endpunkte (`/api/airspace`, `/api/navaids`,
  `/api/waypoints`) liefern künftig die Daten **des Mandanten des Requests**.
- **Alle Schutz-Eigenschaften aus ADR 0004 bleiben:** Key server-seitig,
  best-effort/Last-Good-Cache, nicht-blockierender Start, Timeouts/Größen-
  grenzen, „kein Key ⇒ Feature still aus" (jetzt pro Mandant).
- Der globale `WAYFINDER_OPENAIP_API_KEY` bleibt als **Fallback-Default** für
  Mandanten ohne eigenen Key erhalten (Abwärtskompatibilität).

---

## Begründung

- **Pflichtwechsel statt Pflicht-Secret** trifft den Betreiber-Wunsch „nur
  Container starten" exakt und hält die Sicherheits-Grenze: Das bekannte
  Default-Passwort ist nur bis zum ersten, **erzwungenen** Wechsel gültig; bis
  dahin ist jede andere Aktion fail-closed gesperrt. Das ist das etablierte
  „Default-Credential + forced rotation"-Muster (Router/Appliance-Onboarding).
- **Builtin nur im Compose-Profil, Binary-Default bleibt `none`** vermeidet eine
  stille Default-Änderung für bestehende isoliert betriebene Instanzen
  (ADR 0003) und lokalisiert das Zero-Touch-Verhalten dort, wo es gewünscht ist.
- **Auto-Seed wiederverwendet `runBootstrap`** — kein zweiter, divergierender
  Provisioning-Pfad; die idempotente Logik ist schon getestet.
- **„Letzter Admin"-Guard** ist die zentrale Sicherheits-Invariante des ganzen
  Epics: Selbstverwaltung + Lösch-Endpunkte dürfen nie zur Total-Aussperrung
  führen.
- **Feed-Live-Join** ist die einzige echt neue Architektur-Frage (Receiver-
  Lifecycle zur Laufzeit) und bekommt darum die höchste Einstufung und einen
  eigenen, isoliert testbaren Schritt.
- **OpenAIP pro Mandant** ist die konsequente Multi-Mandant-Fortschreibung von
  ADR 0004; die robusten Eigenschaften (Misstrauen, Degradation) bleiben
  unverändert, nur die Schlüssel-/Cache-Dimension kommt hinzu.

### Verworfene Alternativen

- **Auto-Admin per Pflicht-Env-Secret** (`WAYFINDER_ADMIN_PASSWORD`, sonst kein
  Start): sauber, aber führt genau die Terminal-/Konfig-Reibung wieder ein, die
  beseitigt werden soll. **Verworfen** zugunsten admin/admin + Pflichtwechsel.
- **Generiertes Passwort ins Startup-Log:** bequem, aber verlagert das Secret in
  die Logs (Aufbewahrung/Zugriff schwer kontrollierbar) und scheitert, wenn der
  Erststart-Log nicht greifbar ist. **Verworfen.**
- **Binary-Default auf `builtin` umstellen:** stille Sicherheits-/Verhaltens-
  Änderung für alle bestehenden `none`-Deployments. **Verworfen** — Profil-lokal.
- **Externe DB als einziger Weg** (F2-Alternative A): legitim und bleibt für
  K8s/Produktion der empfohlene Weg, erfüllt aber „nur Container starten" nicht
  out-of-the-box. **Compose-Postgres** wird der Standard-Einstieg; externe DB
  via `WAYFINDER_DB_URL` bleibt **vollständig** unterstützt.
- **OpenAIP global lassen:** verletzt die Mandanten-Isolation (ein Kunde sähe
  die per-Key abgerufenen Daten eines anderen). **Verworfen.**

---

## Konsequenzen

Umsetzung in **Arbeitspaketen** (eigene Ankündigung + Freigabe je Paket,
Charter §3). Reihenfolge nach Abhängigkeit:

| AP | Inhalt | Stufe · Modell |
|----|--------|----------------|
| **ONB-0** | Dieser ADR 0011 (Richtung + drei Entscheidungen F1/F2/F3) | S2 · Sonnet/Opus |
| **ONB-1** | Auto-Seed beim Boot (Default-Mandant + Admin, idempotent, `runBootstrap`); `must_change_password`-Flag (Schema + Login-Gate fail-closed); `docker-compose.yml` mit Postgres | S3 · Opus |
| **ONB-2** | Selbstverwaltung `/api/admin/me` (GET/Passwort/Delete) + Frontend-Pflichtwechsel-Maske | S3 · Sonnet |
| **ONB-3** | Admins anlegen/löschen/pausieren über die API; „letzter aktiver Admin"-Guard; Admin-UI | S3 · Sonnet |
| **ONB-4** | Mandanten anlegen/löschen: `POST/DELETE /api/admin/tenants`, `TenantRepo.Delete` (Cascade), Guards + UI | S3 · Sonnet |
| **ONB-5** | Feeds anlegen/löschen/zuweisen: `POST/DELETE /api/admin/feeds`, `FeedRepo.Delete`, **Live-Receiver-Join/-Leave** + UI | S3–S4 · Opus |
| **ONB-6** | OpenAIP pro Mandant: Schema (Key/AOI je Mandant), Service-Umbau auf Cache je Mandant, Endpunkte mandanten-aufgelöst, globaler Key als Fallback + UI | S4 · Opus |

- **Schema:** `users.must_change_password` (ONB-1); ggf. `tenants`-Lösch-
  Kaskade prüfen (ONB-4); neue Mandanten-OpenAIP-Konfig-Tabelle (ONB-6).
- **Backend:** `cmd/wayfinder` (Auto-Seed, Feed-Manager), `pkg/adminapi`
  (`/me/*`, Tenant-/Feed-CRUD, Guards), `pkg/store` (`TenantRepo.Delete`,
  `FeedRepo.Delete`, OpenAIP-Konfig-Repo), `pkg/receiver`/Feed-Manager
  (Live-Join), `pkg/aeronautical` (Cache je Mandant).
- **Frontend:** Pflichtwechsel-Maske, Konto-Selbstverwaltung, Admin-/Mandanten-/
  Feed-Verwaltungs-Ansichten, OpenAIP-Konfig je Mandant.
- **Doku:** Register (neue/aktualisierte FR-/NFR-Einträge, rückverfolgbar),
  `docs/INSTALLATION.md` (Compose-Schnellstart ersetzt CLI-Pflichtschritte,
  Default-Login + Pflichtwechsel), `docs/TECHNICAL.md` (neue Endpunkte/Flag/
  Tabellen/Env), Milestone-Doku je AP. Die CLI-Befehle `bootstrap`/`feed add`
  **bleiben** als Skript-/Automatisierungs-Pfad erhalten (nur nicht mehr Pflicht).

---

## Ehrliche Grenze

- **Bekanntes Default-Passwort** ist ein bewusst eingegangenes Restrisiko für
  das **Zeitfenster zwischen Erststart und erstem Login**. Wer in diesem Fenster
  Netzzugriff auf den Browser-Rand hat, kann sich als `admin/admin` einloggen —
  allerdings nur, um sofort das Passwort zu wechseln (alles andere ist gesperrt).
  Für höher exponierte Deployments bleibt die Netz-Isolation (ADR 0003) bzw. ein
  vorab gesetztes Passwort der härtere Weg; beides bleibt möglich.
- **Compose-Postgres** ist ein **Einstiegs-/Demo-Komfort**, kein Ersatz für eine
  betriebene, gesicherte, gesicherte-Backups-Datenbank im Produktivbetrieb.
- **Feed-Live-Join** verändert den Receiver-Lebenszyklus zur Laufzeit; der ADR
  entscheidet die **Richtung**, die genaue Goroutine-/Shutdown-Mechanik wird im
  ONB-5-Häppchen festgelegt und isoliert getestet.
- Dieser ADR entscheidet die **Richtung**; Schema-Detailform je Tabelle, die
  Cascade-Strategie und die UI-Feinheiten werden im jeweiligen AP-Häppchen
  festgelegt (kein weiterer Architektur-Sprung nötig).
