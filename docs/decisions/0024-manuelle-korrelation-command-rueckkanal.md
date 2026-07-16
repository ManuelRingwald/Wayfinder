# ADR 0024 — Manuelle Flugplan-Korrelation: Command-Rückkanal Wayfinder → Firefly

- **Status:** **VORGESCHLAGEN** 🟡 (2026-07-16) — wartet auf Ratifizierung. Bis
  zur Annahme wird **kein** Implementierungs-Code gebaut (Design-/Review-Tor,
  CLAUDE.md §3). Die beiden Kern-Weichenstellungen (Kommando-Weg, Token-Modell)
  sind unten mit Empfehlung **und** Gegenoption ausformuliert; die finale
  Entscheidung trifft der Betreiber beim Review.
- **Datum:** 2026-07-16
- **Schnittstellen-relevant:** **ja, aber nicht am CAT062-Ausgabe-Vertrag.** Der
  Strom Firefly → Wayfinder (CAT062/063/065) bleibt **unverändert**. Neu ist ein
  **Rückkanal** Wayfinder → Firefly gegen Fireflys **Kommando-API**
  (`POST/DELETE/GET /correlation`, Fireflys ADR 0038/0039). Dieser Rückkanal
  existiert heute **nicht** — Wayfinder ist bisher reiner Multicast-Konsument.
- **Bezug:** **ADR 0012** (Mandanten-Tracker-Orchestrierung, getrennte
  Control-Plane, §6 Least-Privilege), **ADR 0003** (Browser-Rand als
  Vertrauensgrenze, fail-closed), **ADR 0005/0014** (Multi-Tenant-Isolation),
  **ADR 0008** (Read-only-Impersonation), **ADR 0022** (Admin ohne eigenes ASD).
  Firefly-seitig: **Fireflys ADR 0038** (zentrale Korrelation im SDPS),
  **Fireflys ADR 0039** (Kommando-API). Issue **#245 (Teil B)**; Teil A
  (I062/390-Anzeige) ist bereits erledigt (FR-DATA-013). Anforderungs-Register:
  neu geplant **FR-ORCH-013** (nach Ratifizierung).

> ℹ️ **Auslöser:** Der Lotse braucht eine **Bedienhandlung**, um Fireflys
> automatische Flugplan-Korrelation zu korrigieren — einen Plan an einen Track
> pinnen, einen Track auf „unkorreliert" zwingen, einen Pin lösen. „Manuell
> schlägt Automatik." Vorstufe zu elektronischen Flugstreifen (EFS-1).

---

## Kontext

Fireflys Kommando-API (Fireflys ADR 0039), gegen die Wayfinder sprechen würde:

| Endpunkt | Wirkung |
|----------|---------|
| `POST /correlation` `{track_number, callsign}` | Plan auf Draht-Track pinnen (422 bei unbekanntem Callsign) |
| `POST /correlation` `{track_number}` | Track auf **unkorreliert** pinnen (Automatik gesperrt) |
| `DELETE /correlation/{N}` | Pin lösen, Automatik übernimmt (idempotent) |
| `GET /correlation` | Pins auflisten |

Auth: `Authorization: Bearer <FIREFLY_WS_TOKEN>`; `409` ohne konfigurierte
Flugpläne; Pins sind flüchtig und sterben mit dem Track-Ende (TSE).

Eine Code-Analyse (2026-07-16) hat die **harten Randbedingungen** ermittelt, die
dieser ADR auflösen muss:

1. **Kein Rückkanal heute.** Der `/ws`-Pfad ist push-only und **verwirft**
   eingehende Frames (`pkg/ws/handler.go`). Es gibt keinen Code, der eine
   Firefly-Instanz adressiert oder anspricht.
2. **Erreichbarkeit existiert, ist aber nicht portabel.** Jede Firefly-Instanz
   bindet ihren HTTP-Port **deterministisch** aus der Feed-ID
   (`dockerbackend.fireflyHTTPPort(feedID)` = `18080 + (feed_id mod 40000)`) im
   **Host-Netz**; im Einzelhost-Harness ist er also von **beiden** Wayfinder-
   Prozessen per `127.0.0.1:<port>` erreichbar. **Aber:** der Kubernetes-Backend
   (ORCH-6) ist ungebaut — dort gibt es **keine** localhost-Adresse, kein stabiles
   Cross-Tier-Adressierungsschema. Die Adressierungs-Helfer sind zudem
   **unexportiert** in `pkg/dockerbackend` (orchestrator-privat).
3. **Es gibt noch kein Token.** `fireflyEnv` setzt heute `FIREFLY_CAT062_*`,
   `FIREFLY_PORT`, `FIREFLY_SOURCES` (+ Cred-Envs) — **kein** `FIREFLY_WS_TOKEN`
   (repo-weit bestätigt). Fireflys HTTP-Server gilt bislang als „totes Gewicht"
   (nur binden, damit kein Crash-Loop). Ein Command-Token muss **neu** erzeugt,
   bereitgestellt und rotierbar gemacht werden — und eine Rotation über
   `fireflyEnv` ändert den **Spec-Hash** → der Reconciler startet die Instanz neu.
4. **Prozessgrenze (ADR 0012 §6).** Das **Container-Start-Privileg** (Docker-
   Socket / K8s-API) ist exklusiv beim **Orchestrator**; der browser-zugewandte
   Server schreibt nur Soll-Zustand in die DB. **Aber:** §6 verbietet dem Wortlaut
   nach nur die **Container-Laufzeit** — nicht das **Erreichen eines Service-Ports**
   eines bereits laufenden Firefly. Und Host-Loopback macht den Port vom Server aus
   erreichbar. Ob ein `/correlation`-Kommando ein „Control-Plane-Privileg" ist,
   ist damit eine **echte offene Entscheidung**.
5. **Kommando ≠ Reconciler.** Der Reconciler ist **idempotent/Soll-Zustand**; ein
   einmaliges imperatives „korreliere jetzt Track X" passt **nicht** in
   Konvergiere-zum-Soll und bräuchte einen **eigenen** Transport. Vor allem wegen
   der **synchronen Rückmeldung** (422/409), die der Lotse **sofort** im
   Kontextmenü sehen muss.
6. **Gating-Grundlage vorhanden.** `SubscriptionRepo.IsSubscribed(tenant, feed_id)`
   ist genau das Prädikat für „darf dieser Nutzer auf diesem Feed korrelieren".
   Jeder Track trägt seine `feed_id` bereits zum Browser (`TrackMessage.FeedID`).

---

## Entscheidung

### E1 — Kommando-Weg: **Server-direkt** (empfohlen)

Der **browser-zugewandte Server** ruft Fireflys `/correlation`-API **direkt**
(synchroner HTTP-Call, Bearer-Token). Begründung:

- Die **Rückmeldung muss synchron** sein — der Lotse klickt „korreliere" und
  erwartet sofort „erledigt / Callsign unbekannt (422) / keine Pläne (409)". Ein
  direkter Call liefert das in einem Sprung.
- Ein Korrelations-Kommando an ein **laufendes** Firefly ist eine **gegatete
  per-Track-Datenaktion**, **kein** Container-Lebenszyklus. Es kreuzt die von
  ADR 0012 §6 gezogene Linie **nicht**: das gefährliche Privileg (Container
  starten, Docker-Socket) bleibt exklusiv beim Orchestrator. Der Blast-Radius
  eines Server-Rand-Exploits über diesen Kanal ist „Korrelationen auf bereits
  zugänglichen Feeds durcheinanderbringen" — **nicht** „beliebige Container
  starten".

**Gegenoption (verworfen, dokumentiert): Orchestrator-vermittelt.** Der Server
schreibt ein Kommando in die DB (Queue + `NOTIFY`), der privilegierte
Orchestrator führt den Call aus und meldet das Ergebnis zurück. Vorteil: **jeder**
Firefly-Kontakt bliebe in der Control-Plane (strengste §6-Auslegung). Nachteile:
**asynchron** (Antwort-Rundreise Firefly → Orchestrator → DB → Server → Browser),
**deutlich mehr Maschinerie** (Kommando-Queue-Tabelle, Status-Rückkanal,
Timeouts), passt **nicht** zum Soll-Zustand-Reconciler und liefert eine spürbar
schlechtere Bedien-Erfahrung. Der Isolations-Gewinn ist gering, weil das
wirklich gefährliche Privileg so oder so beim Orchestrator bleibt.

### E2 — Token-Modell: **Deployment-weit** (empfohlen, mit Härtungspfad)

Ein deployment-weites `WAYFINDER_FIREFLY_COMMAND_TOKEN` — **gleicher Wert** auf
dem Server (zum Senden) und via `fireflyEnv` in **jeder** Firefly-Instanz (zum
Prüfen). Muster wie `WAYFINDER_SECRET_KEY` (schon heute auf Server **und**
Orchestrator gesetzt). Begründung:

- **Einfachste** Bereitstellung/Adressierung; keine Pro-Feed-Erzeugungs-/
  Lesepfad-/Rotations-Mechanik.
- Die **feine** Zugangskontrolle liegt ohnehin am **Wayfinder-Rand**
  (`IsSubscribed(tenant, feed_id)`, E3). Das Token ist ein **zweiter, grober**
  Zaun gegen ungebetenen Netz-Verkehr, nicht die Mandanten-Trennung.

**Gegenoption (dokumentierte spätere Härtung): Per-Feed-Token.** Der Orchestrator
erzeugt je Feed ein eigenes Token, injiziert es via `fireflyEnv` und persistiert
es (die `feed_secrets`+AES-GCM+AAD-Maschinerie aus ADR 0012 ist wiederverwendbar).
Kleinerer Blast-Radius, aber deutlich mehr bewegliche Teile (Erzeugung,
verschlüsselte Ablage, Lesepfad für den Sender, Rotation ⇒ Spec-Hash-Neustart).
**Empfehlung:** deployment-weit **starten**, Per-Feed als dokumentierte Härtung
für einen späteren Schritt.

### E3 — Gating & Akteur

- **Prädikat:** `IsSubscribed(tenant, track.feed_id)` — nur ein **echter
  Mandanten-Nutzer** mit Abo auf den Feed des Tracks darf korrelieren. Der Track
  liefert seine `feed_id` + `track_num` bereits im WS-DTO.
- **Kein Admin ohne Scope** (ADR 0022): ein Admin ohne aktive Sicht hat keinen
  Feed-Scope und darf nicht korrelieren.
- **Nicht unter Read-only-Impersonation** (ADR 0008): Korrelation **schreibt**
  Firefly-Zustand; ein Schreib-Kommando unter einem ausdrücklich **lesenden**
  Impersonations-Grant ist unzulässig. Korrelation ist damit die **erste**
  authentifizierte, mandanten-gescopte **Feed-Schreib-Aktion** eines
  Mandanten-Nutzers — der Akteur ist der Feed-abonnierte Nutzer selbst, nie ein
  impersonierender Admin.

### E4 — Adressierung (portabel)

Eine **backend-agnostische Abstraktion** liefert je `feed_id` die Command-Base-URL
der zugehörigen Firefly-Instanz (analog zur `instance.Backend`-Abstraktion):

- **Docker/Host-Netz (heute):** `http://127.0.0.1:<fireflyHTTPPort(feed_id)>` —
  der deterministische Port wird über einen **exportierten** Helfer bereitgestellt
  (statt der heute orchestrator-privaten Funktion zu duplizieren), Host optional
  konfigurierbar.
- **Kubernetes (später, ORCH-6):** Pod-/Service-Adresse — die Abstraktion lässt
  das offen; **kein** localhost hartkodiert im Server.

### E5 — Transport

Direkter, **best-effort**, misstrauischer HTTP-Client nach dem Haus-Muster
(`pkg/weather`): injizierter `*http.Client` mit **Timeout** (Haus-Default 15 s),
`context`, `io.LimitReader`, `Authorization: Bearer`-Header. Fehler-Mapping:
Fireflys 422/409 werden als aussagekräftige Bedienfehler an den Browser
weitergereicht; Netz-/Timeout-Fehler als „Instanz nicht erreichbar". **Nie** den
Track-Pfad oder `/ready` blockieren.

---

## Konsequenzen

**Positiv**
- Synchroner, gegateter Bedienweg für den Lotsen; unmittelbare 422/409-Rückmeldung.
- ADR 0012 §6 bleibt intakt: Container-Laufzeit-Privileg exklusiv beim
  Orchestrator; der Server bekommt nur eine **schmale, gegatete Datenaktion**.
- Wiederverwendung etablierter Muster (Outbound-HTTP `pkg/weather`,
  `IsSubscribed`-Gating, `WAYFINDER_SECRET_KEY`-artige Env-Konfig).

**Kosten / Risiken**
- **Erste Feed-Schreib-Aktion** am Browser-Rand — braucht saubere AuthZ-Tests
  (Nicht-Abonnent → 403, Impersonation → 403, Admin ohne Scope → 403).
- **Token-Rotation** über `fireflyEnv` ⇒ Spec-Hash-Drift ⇒ Reconciler-Neustart
  der Instanzen. Bewusst in Kauf genommen (selten); dokumentieren.
- **K8s-Adressierung** bleibt offen, bis ORCH-6 existiert — die Abstraktion (E4)
  hält die Naht dafür frei, aber der erste Wurf funktioniert nur im
  Host-Netz-Harness. **Ehrliche Grenze**, im Betriebshandbuch vermerken.
- **Grober Token** (deployment-weit) — Per-Feed-Härtung dokumentiert, nicht sofort.

**Ausdrücklich NICHT Teil dieses Vorhabens**
- `identity_conflict` (SPEC.1-Duplikat-Flag): lebt nur in Fireflys **WS-Pfad**,
  **nicht** über CAT062 — für Wayfinder über den Draht nicht verfügbar.
- Persistente Pins über Track-Ende hinaus (Fireflys HA.1).

---

## Umsetzungs-Reihenfolge (nach Ratifizierung, je eigenes Häppchen + Freigabe)

1. **Backend-Command-Client + Adressierung** (`pkg/…` Firefly-Command-Client,
   exportierter Port-Helfer, Config `WAYFINDER_FIREFLY_COMMAND_TOKEN`).
2. **Server-Endpoint + Gating** (`POST/DELETE /api/correlation`, `IsSubscribed`,
   422/409-Mapping; AuthZ-Tests).
3. **Frontend-Kontextmenü** am Track (korrelieren / entkorrelieren / Pin lösen;
   synchrone Fehleranzeige).
4. **Orchestrator/`fireflyEnv`**: `FIREFLY_WS_TOKEN` injizieren (+ Doku
   INSTALLATION/TECHNICAL, Spec-Hash-Hinweis).

Jeder Schritt: Anforderungs-Register, Tests, Doku, Qualitäts-Gates (CLAUDE.md §6).
