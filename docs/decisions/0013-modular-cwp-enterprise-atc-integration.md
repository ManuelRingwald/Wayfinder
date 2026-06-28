# ADR 0013 — Modular CWP & Enterprise ATC Integration

- **Status:** **ENTWURF — zur Freigabe vorgelegt** (kein Produktivcode, bis
  freigegeben). Dieser ADR setzt die **Richtung & die Kontrakte** für ein
  Programm aus mehreren Epics, nicht ein einzelnes Feature.
- **Datum:** 2026-06-28
- **Schnittstellen-relevant:**
  - **Gegenüber dem CAT062/063/065-Draht-Vertrag mit Firefly: nein** — der
    Surveillance-Draht bleibt **unverändert** (Firefly bleibt der autonome
    SDPS-Tracker aus ADR 0012; keine Wayfinder-Spezialfälle).
  - **Neue Kontrakte entstehen** (Wayfinder-intern bzw. extern): das
    **CWP-Bus-Schema** (Browser-lokaler Event-Bus), die **Strip-State-Machine**
    (FDP), das **Workstation/Rollen-Modell** und der **SWIM-Adapter-Rand**
    (AMQP 1.0, AIXM/FIXM/IWXXM). Jeder dieser Kontrakte ist versioniert und
    eigenständig getestet.
- **Bezug:** **ADR 0003** (Browser-Rand, Vertrauensgrenze, fail-closed — gilt
  jetzt zusätzlich für den SWIM-Eingang und den Browser-Bus), **ADR 0005**
  (Multi-Mandanten-Isolation, `tenant_id` bleibt **die** autoritative Grenze —
  Workstation/Rolle ist eine *Verfeinerung darunter*, niemals eine Aufweichung),
  **ADR 0006** (Stateless-Split, Identitäts-/Konfig-Persistenz — Identity wird
  erweitert), **ADR 0007** (NATS-Fan-out — bleibt die **Surveillance**-Ebene;
  SWIM ist eine **zweite, getrennte** Informations-Ebene), **ADR 0012**
  (per-Mandant-Tracker — die CWP-Arbeitsplätze konsumieren genau diese Feeds).
  Firefly-seitig: **Fireflys ADR 0006** (CAT062-Ausgabe-Vertrag, unberührt).
  Anforderungs-Register: neu **FR-CWP-001…**, **FR-FDP-001…**, **FR-IMS-001…**;
  bezogen **NFR-SEC-003/004** (Isolation), **NFR-SEC-005** (neu: untrusted
  SWIM-Ingress), **NFR-TRACE-001** (Strip-Audit).

> ℹ️ **Auslöser:** Betreiber-Vision (2026-06-28). Wayfinder soll vom isolierten
> **ASD** zu einer modularen **Enterprise-CWP** (*Controller Working Position*)
> anwachsen — einer Suite aus **ASD** (Air Situation Display), **EFS**
> (Electronic Flight Strips) und **IMS** (Information Management System). Die
> Module müssen **eigenständig**, **Split-Screen** oder **physisch verteilt auf
> mehreren Monitoren desselben Arbeitsplatzes** betreibbar sein und dabei
> **latenzfrei** interagieren (Klick auf einen EFS-Streifen markiert sofort den
> zugehörigen ASD-Track). Das IMS ist von Tag 1 auf das **SWIM**-Paradigma
> (*System Wide Information Management*) ausgelegt und später gegen die
> öffentlichen **FAA-SCDS**-Live-Feeds validierbar (AIXM/FIXM/IWXXM).

---

## Kontext

### Ist-Zustand (am Code geerdet)

- **Wayfinder ist heute ein Ein-Modul-ASD.** Frontend: zwei Views (`AsdView`,
  `AdminView`), Pinia-Stores `asd.js` / `admin.js` / `impersonation.js`. Der
  ASD-Zustand inkl. **`selectedTrack`/`selectTrack`/`clearTrackSelection`** lebt
  im `asd`-Store — das ist genau der **intermodulare** Zustand, den ein EFS
  mitbedienen müsste.
- **Identität kennt nur Mandant + Autorisierungs-Rolle.** `tenant.Identity` =
  `{TenantID, UserID, Subject, Role, MustChangePassword}`, `Role ∈ {user, admin}`
  (ADR 0009). Es gibt **keinen** Begriff von **Arbeitsplatz (Workstation)** oder
  **operativer Lotsen-Rolle** (Approach/Tower/Ground).
- **Datenmodell** (`00001_init.sql` …): `tenants`, `users`, `feeds`,
  `subscriptions`, `view_configs`, `entitlements`. **Keine** Tabellen für
  Arbeitsplätze, operative Rollen, Flugpläne oder Flight-Strips.
- **Ein Transport-Plane existiert:** Surveillance — CAT062/065/063 über
  UDP-Multicast bzw. NATS-Fan-out (ADR 0007), pro Mandant auf AOI/FL gefiltert
  (WF2-21.2), server-seitig fail-closed. **Kein** Informations-Plane (NOTAM,
  Wetter, Flugplan), **kein** Flugdaten-Plane (Strips).
- **Der Browser-Rand ist heute single-window.** Es gibt keinen Mechanismus, mit
  dem zwei Fenster/Module desselben Arbeitsplatzes Zustand teilen.

### Spannungsfeld

Die Vision erzeugt fünf Spannungen, die der ADR auflösen muss:

1. **Modul-Entkopplung vs. latenzfreie Interaktion.** Völlig eigenständige
   Module (eigene Fenster/Monitore) sollen sich **sofort** gegenseitig markieren
   — ohne monolithischen Shared-Store und **ohne** das Backend für reines
   UI-Highlighting zu belasten.
2. **EFS ist Zustand, nicht Anzeige.** Ein Flight-Strip ist ein **Status-Objekt**
   mit Lebenszyklus und sicherheitskritischen Übergaben (Handover). Das verlangt
   eine **deterministische State-Machine** und ein **Flugdaten-Backend (FDP)** —
   ein neues, zustandsbehaftetes Domänen-Stück neben dem zustandslosen ASD-Pfad.
3. **Login-Kontext wird mehrdimensional.** „Wer bin ich" (Mandant + Autz-Rolle)
   genügt nicht mehr; es kommt „**an welchem Arbeitsplatz, in welcher operativen
   Rolle**" hinzu — ohne die autoritative Mandanten-Isolation aufzuweichen.
4. **Zwei Buswelten.** ADR 0007 hat **NATS** für den Surveillance-Fan-out
   gewählt. SWIM/SCDS spricht **AMQP 1.0**. Das darf nicht zu „ein Bus für
   alles" verschmiert werden — Surveillance und Information sind verschiedene
   Planes mit verschiedenen Garantien.
5. **Neuer untrusted Außenrand.** SWIM-Daten (AIXM/FIXM/IWXXM) sind großes,
   fremdes XML aus dem öffentlichen Netz — eine **neue Vertrauensgrenze**, die
   dieselbe Härte braucht wie der robuste CAT062-Decoder.

### Zielbild (fachlich)

Eine **CWP-Suite** aus drei fachlich getrennten Modulen über einem schlanken
**Shell**:

| Modul | Domäne | Plane | Zustand |
|---|---|---|---|
| **ASD** | Luftlage (Tracks, Karte) | Surveillance (CAT062/NATS) | zustandslos (Live-Sicht) |
| **EFS** | Flight-Strips, Freigaben, Handover | Flugdaten (FDP) | **zustandsbehaftet** (State-Machine, auditiert) |
| **IMS** | NOTAM, Wetter, Lufträume, Flugplan-Info | Information (SWIM/AMQP) | überwiegend Read-Model + Pub/Sub |

Diese drei korrelieren (ein Strip ↔ ein Track ↔ eine Luftraum-Aktivierung), sind
aber **getrennt deploybar und getrennt betreibbar**.

---

## Entscheidung

### D1 — Modulare CWP-Suite: drei Module + Shell, komponierbar (standalone / split / multi-monitor)

Wayfinder wird in **drei fachlich eigenständige Frontend-Module** (ASD, EFS, IMS)
plus eine **CWP-Shell** zerlegt. Jedes Modul hat einen **eigenen Einstiegspunkt
(eigene Route, in eigenem Browser-Fenster öffenbar)** und einen **eigenen,
schlanken Store** für seinen *internen* Zustand.

- **Drei Betriebsformen, eine Codebasis:**
  *standalone* (ein Modul, ein Fenster/Monitor) · *split* (mehrere Module in
  einem Fenster, Layout-Manager der Shell) · *multi-monitor* (jedes Modul in
  einem eigenen Fenster auf eigenem Monitor desselben Arbeitsplatzes).
- **Geteiltes Fundament, keine geteilte Logik:** gemeinsames Design-System,
  gemeinsame Auth/Session, gemeinsamer **CWP-Bus** (D2). **Verboten:** ein
  modulübergreifender Monolith-Store für intermodulare States.
- **Kein harter Komponenten-Bezug** zwischen Modulen (kein `import { asdStore }
  from efs`). Die einzige modulübergreifende Kopplung ist der **Bus** (D2) plus
  geteilte, *stateless* Domänen-Typen (Korrelations-IDs, DTO-Schemata).

### D2 — Client-Koordination über `BroadcastChannel` — das Backend trägt **kein** UI-Highlighting

Modulübergreifende **UI-Intentionen** (Auswahl, Hover, Fokus) laufen
**ausschließlich** über einen **Browser-lokalen, domänenübergreifenden
Event-Bus** auf Basis der **nativen `BroadcastChannel API`**. Das Backend wird
für reines Highlighting **nicht** angefasst.

- **Ein versioniertes Bus-Schema** (`cwp-bus`, same-origin). Skizze:

  ```ts
  // cwp-bus message contract (versioned; unknown types are ignored forward-compat)
  type CwpBusMessage = {
    v: 1
    origin: 'asd' | 'efs' | 'ims'
    session: string        // per-login guard: hash(tenant_id, workstation_id, sid)
    type:
      | 'track.selected' | 'track.hovered' | 'track.cleared'
      | 'flight.selected' | 'strip.assumed' | 'strip.handover'
      | 'ims.airspace.focus' | ...
    correlationId?: string // ICAO 24-bit hex (preferred), fallback callsign
    payload: unknown
    ts: number
  }
  ```

- **Korrelations-Vertrag (der Kern):** Module sprechen über eine **stabile
  Korrelations-Identität**. Primär die **ICAO-24-Bit-Adresse** (I062/380, im
  ASD vorhanden), Fallback **Callsign** (I062/245), zuletzt **Track-Nummer**
  (I062/040, nur innerhalb einer Surveillance-Quelle eindeutig). Ein EFS-Strip
  trägt dieselbe Korrelations-ID → Klick im EFS → `flight.selected{correlationId}`
  → ASD setzt `selectedTrack` lokal. **Null Backend-Roundtrip.**
- **Refactor des bestehenden Zustands:** `asd.selectedTrack` wird vom **lokalen
  alleinigen Wahrheitsort** zu einem Zustand, der **auch** aus Bus-Events
  gespeist wird und Auswahl-Änderungen **auf** den Bus spiegelt. Realisiert als
  schlanke Composable `useCwpBus()` — **kein** neuer globaler Store.
- **Sicherheits-Leitplanken am Bus:** `BroadcastChannel` ist **same-origin**
  (gut). Zusätzlich trägt jede Nachricht ein **Session-Guard-Token**
  (`session`); Empfänger **verwerfen** Nachrichten mit fremdem Token (schützt
  gegen Kontext-Bleed bei Impersonation/Mandantenwechsel, ADR 0008). Der Bus
  transportiert **nur UI-Intent**, **nie** autoritative Daten oder Secrets.

### D3 — EFS = zustandsbehaftete Strips, getrieben von einer FDP-State-Machine mit Rollen & Handover

Ein Flight-Strip ist ein **Status-Objekt**. Es entsteht ein neues, **zustands­behaftetes**
Backend-Domänenstück **FDP** (*Flight Data Processing*, `pkg/fdp`), getrennt vom
zustandslosen Surveillance-Pfad. Kern ist eine **explizite, auditierte
State-Machine**.

- **Strip-Lebenszyklus (deterministische State-Machine, Skizze):**

  ```
  proposed ─▶ pending ─▶ assumed(role) ─▶ handover_offered(→role') ─┬▶ assumed(role')   (accept)
                                                                    └▶ assumed(role)    (reject)
  assumed(role) ─▶ released ─▶ terminated        ·        any ─▶ cancelled
  ```

- **Operative Rollen sind erststeklassig:** Strips sind **dedizierten
  Lotsen-Rollen** zugeordnet (z. B. **Approach / Tower / Ground / Center**,
  erweiterbar). Nur die **besitzende Rolle** darf einen Strip ändern oder zur
  Übergabe anbieten (Transitions-Guard).
- **Handover ist eine sicherheitskritische, geführte Transition:** `offer` →
  Ziel-Rolle sieht den angebotenen Strip → `accept`/`reject`. **Jeder** Übergang
  wird **lückenlos auditiert** (wer, wann, von→nach, Rolle) — Zertifizierungs-
  Anforderung (Rückverfolgbarkeit, Charter §7).
- **Persistenz (neu):** `flights`, `flight_strips`, `strip_assignments`,
  `strip_transitions` (Audit-Log). Verarbeitung **deterministisch nach
  Datenzeit**, konsistent mit dem Determinismus-Prinzip.
- **Flugdaten-Quelle gestuft:** **Stufe A** — provisorische Flight-Objekte aus
  korrelierten Tracks (ICAO/Callsign), damit das EFS früh nutzbar ist. **Stufe
  B** — echte Flugpläne (FIXM via SWIM / externer Flugplan-Feed). Stufe B ist die
  große, eigene Abhängigkeit (siehe Ehrliche Grenzen).

### D4 — Workstation- & Rollen-Modell: Login-Kontext = Mandant **und** operative Rolle **und** Arbeitsplatz

Die Plattform lernt den Begriff **CWP-Arbeitsplatz**. Der Admin konfiguriert:
„**Arbeitsplatz 1 gehört zu Mandant Speyer, hat Rolle *Approach*, sieht Feed X**".

- **Zwei orthogonale Rollen-Achsen — strikt getrennt:**
  - **Autorisierungs-Rolle** (`user` | `admin`, bestehend, ADR 0009) — *darf ich
    administrieren?*
  - **Operative Rolle / Lotsen-Position** (`approach` | `tower` | `ground` |
    `center` | …, **neu**) — *welche Flugverkehrskontroll-Funktion übe ich aus?*
    Diese beiden werden **nie** vermischt.
- **Neue Entitäten:** `controller_roles` (kontrolliertes, erweiterbares
  Vokabular) und `workstations` (`id`, `tenant_id`, `name`, `controller_role`,
  Feed-Bindung, optional Sektor/AOI-Bezug). Ein Arbeitsplatz gehört **genau einem
  Mandanten** (Isolation bleibt am `tenant_id` verankert) und trägt **genau eine**
  operative Rolle.
- **Login etabliert den kombinierten Kontext:** Der Nutzer meldet sich **an einem
  Arbeitsplatz** an (feste Zuordnung oder Auswahl). `tenant.Identity` wird um
  **`WorkstationID`** und **`ControllerRole`** erweitert. Dieser Kontext speist
  **beide** Module: das ASD (welcher Feed/AOI) **und** das EFS (welche
  Strips/Positionen diese Rolle besitzt).
- **Admin-UI:** Arbeitsplätze CRUD, Rolle zuweisen, Feed binden — als
  Erweiterung des bestehenden Admin-Bereichs (`pkg/adminapi`, `AdminView`).

### D5 — IMS auf dem SWIM-Paradigma von Tag 1: Pub/Sub-Informations-Plane, SCDS-validierbar

Das IMS wird als **SWIM-orientierter Informations-Plane** gebaut — Pub/Sub-fähig
und so geschnitten, dass eine spätere **FAA-SCDS**-Anbindung „nur ein weiterer
Adapter" ist.

- **Transport: AMQP 1.0** als SWIM-Transport (deckt die FAA-SCDS-Realität /
  Solace ab; MQTT als möglicher zweiter Adapter für leichtgewichtige Quellen).
  **Bewusst getrennt von NATS** (ADR 0007): NATS bleibt der **Surveillance**-
  Fan-out, AMQP ist der **Informations**-Plane. Zwei Buswelten, zwei Zwecke.
- **Ports & Adapters am Eingang:** ein **internes, kanonisches Informations-
  Modell**; **Format-Adapter** parsen **AIXM** (Lufträume/NOTAM), **FIXM**
  (Flug), **IWXXM** (Wetter, METAR/TAF/SIGMET). Die SCDS-Anbindung ist ein
  AMQP-Quell-Adapter, der dieselben kanonischen Objekte erzeugt.
- **Interner Pub/Sub bis zum Browser:** das IMS-Frontend abonniert Themen
  (Topic-Filter) über den bestehenden WS-Rand bzw. einen dedizierten Kanal —
  mandanten-skopiert wie der Track-Strom.
- **Korrelation statt Kopplung:** IMS-Information bleibt **getrennt** von Tracks
  und Strips, **korreliert** aber (eine AIXM-Luftraum-Aktivierung färbt einen
  ASD-Layer; ein IWXXM-METAR annotiert einen Flugplatz; ein FIXM-Flugplan
  reichert einen EFS-Strip an) — die Korrelation läuft über D2/Domänen-IDs.

### D6 — Plane-Trennung & Sicherheits-/Isolations-Invarianten (verbindlich)

- **Drei Server-Planes, bewusst getrennt:** Surveillance (CAT062 / NATS) ·
  Flugdaten (FDP / intern, zustandsbehaftet) · Information (SWIM / AMQP).
  Plus **ein** Client-Plane (BroadcastChannel, reiner UI-Intent). Keine
  Vermischung der Transporte.
- **`tenant_id` bleibt die autoritative Isolationsgrenze** (ADR 0005). Workstation
  und operative Rolle sind **Verfeinerungen darunter** — sie können Sicht
  **verengen** (eine Approach-Position sieht ihre Strips), aber **nie** über die
  Mandantengrenze hinaus **erweitern**. Fail-closed bleibt Pflicht.
- **SWIM-Eingang ist untrusted external data** (NFR-SEC-005, neu): robustes,
  fehler-tolerantes XML-Parsing, Größen-/Tiefen-Limits (XXE/Billion-Laughs
  abgewehrt), fehlerhafte Nachrichten verwerfen statt absturz — derselbe Maßstab
  wie der robuste CAT062-Decoder (Charter §7).
- **FDP-Transitions sind autorisierungspflichtig:** nur die besitzende operative
  Rolle (D4) darf einen Strip ändern/anbieten; serverseitig erzwungen,
  auditiert.
- **Control-Plane-Grenze (ADR 0012) bleibt:** der browser-zugewandte Prozess
  startet weiterhin keine Privileg-Aktionen.

---

## Begründung

- **BroadcastChannel statt Backend für Highlighting** ist die direkte Antwort auf
  „latenzfrei **und** entkoppelt": same-origin, In-Browser, keine Netz-Latenz,
  keine Backend-Last, nativ (keine Bibliothek). Multi-Monitor an **einem**
  Arbeitsplatz ist genau der Sweet-Spot der API.
- **Korrelations-ID als Vertrag** (ICAO→Callsign→Tracknr.) macht die Module
  unabhängig: keiner kennt die Interna des anderen, beide kennen nur die ID.
- **Explizite FDP-State-Machine** ist die einzige zertifizierungs-fähige Art,
  sicherheitskritische Strip-Übergaben zu führen: deterministisch, auditiert,
  rollen-bewacht — statt impliziter Zustände im UI.
- **Zwei Rollen-Achsen getrennt** verhindert die klassische Verwechslung
  „Admin = Tower"; Autorisierung und operative Funktion sind verschiedene Dinge.
- **AMQP für SWIM, NATS für Surveillance** respektiert die Realität: SCDS spricht
  AMQP; der Track-Fan-out ist ein anderes Problem mit anderen Garantien. Ein
  gemeinsamer Bus wäre eine falsche Vereinfachung.
- **SWIM von Tag 1** (kanonisches Modell + Adapter-Rand) heißt: die spätere
  SCDS-Anbindung ist additiv, kein Umbau — „providable validation" gegen echte
  öffentliche Feeds wird zum Schalter, nicht zum Projekt.
- **Modul-Schnitt baut bruchlos** auf dem Bestehenden auf: ASD ist bereits ein
  eigenständiges Modul; EFS/IMS treten daneben, nicht hinein.

### Verworfene Alternativen

- **Monolithischer Cross-Modul-Pinia-Store für Auswahl/Hover:** koppelt die
  Module hart, bricht Multi-Window (ein Store lebt pro Fenster) und zwingt zu
  Backend-Sync. **Verworfen** zugunsten des BroadcastChannel-Bus.
- **Backend-Roundtrip / WebSocket-Echo für Highlighting:** belastet das Backend
  für reine UI-Intention und fügt Netz-Latenz hinzu. **Verworfen** (explizite
  Leitplanke des Betreibers).
- **`window.postMessage` / SharedWorker statt BroadcastChannel:** `postMessage`
  verlangt harte Fenster-Referenzen (bricht „unabhängig geöffnete" Monitore);
  SharedWorker ist mächtiger, aber schwergewichtiger als nötig.
  `BroadcastChannel` ist die minimale, native Lösung. (SharedWorker bleibt als
  spätere Option, falls geteilte Rechen-Last entsteht.)
- **EFS als reines Read-Display über Tracks:** verkennt, dass ein Strip ein
  Status-Objekt mit Übergaben ist; ohne State-Machine keine Handover-Sicherheit.
  **Verworfen.**
- **Operative Rolle in die bestehende `users.role`-Spalte quetschen:** vermischt
  Autorisierung und Funktion, bricht das ADR-0009-Modell. **Verworfen** zugunsten
  zweier Achsen.
- **SWIM über NATS erzwingen / Tracks über AMQP:** ignoriert die Protokoll-
  Realität (SCDS=AMQP) und die unterschiedlichen Garantien. **Verworfen**
  zugunsten getrennter Planes.
- **„Erst alles bauen, dann integrieren":** widerspricht Charter §3. Der ADR
  setzt nur Richtung+Kontrakte; Umsetzung in kleinen, je freigegebenen Paketen.

---

## Konsequenzen

Umsetzung als **Programm aus Epics**, jedes in kleinen, einzeln angekündigten und
freigegebenen Paketen (Charter §3). Die Tabelle ist eine **Vorschau** — die genaue
Paket-Schneidung wird nach diesem ADR und je Epic abgestimmt.

| Epic / AP | Inhalt | Stufe · Modell |
|---|---|---|
| **CWP-0** | Dieser ADR 0013 (Richtung + D1–D6) | S5 · Opus 4.8 / Fable 5 |
| **CWP-1** | **CWP-Shell + Bus-Fundament:** App-Shell, Modul-Routing (eigenes Fenster je Modul), `useCwpBus()`-Composable, versioniertes `cwp-bus`-Schema, Session-Guard; Refactor `asd.selectedTrack` auf den Bus | S4 · Opus 4.8 |
| **CWP-2** | **Workstation/Rollen-Modell (Backend):** Migration `controller_roles`/`workstations`, `Identity`-Erweiterung (`WorkstationID`/`ControllerRole`), Login-Kontext, Admin-API CRUD | S4 · Opus 4.8 |
| **CWP-3** | **Workstation-Admin-UI:** Arbeitsplätze anlegen/zuweisen, Rolle + Feed binden, Login-Auswahl | S3 · Sonnet 4.6 |
| **EFS-1** | **FDP-State-Machine (Backend):** `pkg/fdp`, Strip-Lebenszyklus + Transitions-Guards, Persistenz (`flights`/`flight_strips`/`strip_transitions`), Audit; Flight-Objekte Stufe A (track-korreliert) | S5 · Fable 5 / Opus 4.8 |
| **EFS-2** | **EFS-Modul (Frontend):** Strip-Bay, Statusdarstellung, Auswahl↔ASD über den Bus | S3–S4 · Sonnet/Opus |
| **EFS-3** | **Handover-Flow:** Anbieten/Annehmen/Ablehnen rollen-bewacht, UI + Backend, vollständig auditiert | S4 · Opus 4.8 |
| **IMS-1** | **SWIM-Informations-Modell + AMQP-Adapter-Rand (Backend):** `pkg/ims`, kanonisches Modell, AMQP-1.0-Subscriber-Gerüst, **ein** dünner vertikaler Schnitt (z. B. IWXXM-METAR) end-to-end | S5 · Fable 5 / Opus 4.8 |
| **IMS-2** | **IMS-Modul (Frontend):** Topic-Abos über WS, Read-Model-Ansichten (NOTAM/Wetter), Korrelation zu ASD-Layern/EFS | S3–S4 · Sonnet/Opus |
| **IMS-3** | **AIXM/FIXM-Adapter + SCDS-Anbindung:** weitere Format-Adapter; Anbindung an die öffentlichen FAA-SCDS-Feeds zur Validierung (Registrierung nötig) | S5 · Fable 5 / Opus 4.8 |
| **CWP-🔒** | **Querschnitt Sicherheit:** untrusted-SWIM-Ingress-Härtung (XXE/Limits/Fuzzing), Bus-Session-Guard-Tests, FDP-Transitions-Authz, Isolations-Negativtests | S4–S5 · Opus/Fable |

- **Schema (neu):** `controller_roles`, `workstations`, `flights`,
  `flight_strips`, `strip_assignments`, `strip_transitions`; perspektivisch
  `information_objects` (IMS Read-Model). Migrationen ab `00010_*`.
- **Backend (Wayfinder):** neue Domänen `pkg/fdp` (zustandsbehaftet) und
  `pkg/ims` (SWIM-Adapter + Pub/Sub); `tenant.Identity`-Erweiterung; `pkg/ws`
  bekommt zusätzliche, mandanten-skopierte Topics (Strips, IMS). **Kein** Eingriff
  in den CAT062-Decoder/`pkg/broadcast`-Isolationskern (Surveillance unverändert).
- **Frontend:** App-Shell + `useCwpBus()`-Composable; Module ASD (Refactor),
  EFS (neu), IMS (neu) mit je eigenem Store; **kein** Monolith-Store.
- **Doku:** Register (FR-CWP/FDP/IMS, NFR-SEC-005, NFR-TRACE-001),
  `docs/INSTALLATION.md` (AMQP/SWIM-Env, Workstation-Setup), `docs/TECHNICAL.md`
  (Planes, Bus-Schema, State-Machine, Identity-Erweiterung), Milestone-Doku je
  Epic, Glossar (CWP, EFS, IMS, FDP, SWIM, SCDS, AIXM, FIXM, IWXXM, Handover).
- **Cross-Project (Firefly):** **keine** Pflicht-Änderung — der CAT062-Vertrag
  bleibt. Optionaler späterer Berührungspunkt nur, falls FDP echte Flugpläne aus
  einer Firefly-nahen Quelle bezöge (offen, nicht Teil dieses ADR).

---

## Ehrliche Grenzen

- **Dies ist ein Programm, kein Sprint.** Der ADR entscheidet **Richtung &
  Kontrakte** (Bus-Schema, Strip-State-Machine, Workstation/Rollen-Modell,
  SWIM-Adapter-Rand). Jede Detailmechanik (genaue Zustände/Transitions, AMQP-
  Client, XML-Parser-Wahl, Topic-Layout) wird im jeweiligen Paket festgelegt und
  isoliert getestet — **kein** weiterer großer Architektur-Sprung nötig.
- **`BroadcastChannel` ist same-origin und same-browser.** Multi-Monitor an
  **einem** Arbeitsplatz (ein Browser, mehrere Fenster) ist abgedeckt.
  **Physisch getrennte Maschinen** (zwei PCs, ein Lotse) liegen **außerhalb** —
  das bräuchte einen Backend-Relay und ist bewusst nicht Teil dieser Stufe.
- **Echte Flugpläne (FDP Stufe B) sind die größte Abhängigkeit.** Ohne echte
  Flugplan-Quelle (FIXM/AFTN/extern) bleibt das EFS auf track-abgeleitete
  Provisorien beschränkt — nützlich, aber nicht voll operativ. Das ist ein
  eigener, nicht-trivialer Meilenstein.
- **FAA SCDS** erfordert **Registrierung/Onboarding** und liefert großes, echtes
  AIXM/FIXM/IWXXM (umfangreiche GML-Schemata). Wir bauen **SCDS-ready**; die
  tatsächliche Anbindung ist ein eigener Schritt (IMS-3), die Voll-Abdeckung der
  Schemata ein langfristiges Ziel — wir starten mit **einem** dünnen vertikalen
  Schnitt.
- **Zwei Buswelten kosten Betrieb.** NATS (Surveillance) **und** AMQP
  (Information) parallel zu betreiben erhöht die Betriebs-Komplexität; das ist
  der bewusste Preis für korrekte Plane-Trennung und SCDS-Kompatibilität.
- **Kein Zertifizierungs-Versprechen.** Wir bauen *zertifizierungs-fähig*
  (deterministische State-Machine, Audit, Rückverfolgbarkeit); die formale
  Zertifizierung selbst bleibt außerhalb dieses Code-Projekts (Charter §7).
