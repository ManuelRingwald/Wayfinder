# ADR 0005 — Wayfinder 2.0: Multi-Mandanten-Pivot

- **Status:** akzeptiert (strategische Richtung; die Detail-Weichen für Persistenz/
  Identität und Cloud-Ingest werden in **ADR 0006** bzw. **ADR 0007** entschieden)
- **Datum:** 2026-06-19
- **Schnittstellen-relevant:** nein (keine Änderung am CAT062-Draht-Vertrag mit
  Firefly). **Hinweis:** Das hier gewählte Hybrid-Feed-Modell kann später Fireflys
  Multicast-Gruppen-/Instanz-Modell berühren (mehrere Feeds) — das ist **keine**
  Wire-Format-Änderung und ist als Cross-Project-Vorwarnung in
  `docs/cross-project/todo-for-firefly.md` vermerkt.
- **Bezug:** Konzept `docs/design/wayfinder-2.0-konzept.md`, ROADMAP §0/§1
  (WF2-00); nimmt den in **ADR 0003 (Ehrliche Grenze)** ausdrücklich vertagten
  Punkt „Autorisierung/Rollen — welcher Lotse darf was" auf; Pendant zu Fireflys
  **ADR 0014** (Produktions-Pivot).

## Kontext

Wayfinder 1.x ist ein **einprozessiges, beim-Start-konfiguriertes Single-Tenant-
ASD** (geerdet am Code):

- **Konfiguration** wird **einmalig beim Start** geladen (`loadConfig()`, ENV >
  `wayfinder.yaml` > Defaults). Parameter-Änderung = Neustart.
- **Verteilung:** **ein** `Broadcaster` fächert **jeden** dekodierten Track an
  **jeden** verbundenen WebSocket-Client (`broadcast.go::broadcast()`,
  all-to-all, kein Filter).
- **Eingang:** **ein** CAT062/CAT065-Multicast-Strom = **eine** Luftlage.
- **Identität:** keine Nutzer, keine Rollen, keine Mandanten; optional **ein**
  geteiltes Bearer-Token (`WAYFINDER_AUTH_TOKEN`, ADR 0003).
- **Persistenz:** keine.

Der Entwurf „Wayfinder 2.0" verlangt den Umbau zu einer **mandantenfähigen,
zur-Laufzeit-konfigurierbaren Plattform**: Mandanten (Tenants) mit
Daten-Isolation, Konfiguration aus einer Datenbank statt aus ENV, Admin-UI,
Sensor-/Stream-Management, optionale Monetarisierung und hochverfügbarer Betrieb.

Das ist ein **größerer strategischer Pivot als ADR 0014** — er ändert
Produktnatur, Bedrohungsmodell und Zertifizierungs-Story — und berührt drei harte
Realitäten der bestehenden Architektur, die der Entwurf nicht auflöst (Konzept
§5):

1. **Ein Feed = ein Himmel.** Heute trägt ein CAT062-Strom genau eine Luftlage.
   „Mehrere Kunden" heißt entweder *mehrere Sichten auf dieselbe Lage* oder *ein
   eigener Feed je Mandant*.
2. **CAT062 liefert fusionierte System-Tracks, keine Roh-Plots.** Der im Entwurf
   skizzierte „Plot-Tagger mit `source=ADS-B/FLARM`" passt nicht zum Vertrag, und
   Firefly-Code wird **nicht** importiert (CLAUDE.md §10).
3. **Multicast überquert keine Cloud-Subnetze.** In der Cloud ist ein
   Multicast→Unicast-Ingest-Gateway Pflicht, nicht Kür.

Es braucht eine bewusste Weichenstellung: den Pivot ratifizieren, das
Mandanten-Modell festlegen, die **Vertrauens-/Isolationsgrenze** definieren, den
Kommerz-Scope und die Zertifizierungs-Haltung setzen — **bevor** Code entsteht.

## Entscheidung

### 1. Der Pivot wird ratifiziert (mit erhaltenem ASD-Kern)

Wayfinder wird zur **mandantenfähigen Plattform**. Der **ASD-Kern** (Track-
Darstellung, Karten-Layer, Data-Block, Filter) bleibt vollständig gültig und wird
in 2.0 zur **mandanten-skopierten Sicht** — er wird nicht ersetzt, sondern
eingebettet. Die Umsetzung erfolgt in **sechs Ausbaustufen (0–5)** mit den
Arbeitspaketen `WF2-xx` (ROADMAP §1, Konzept §7); jede Stufe ist für sich
auslieferbar.

### 2. Mandanten-Modell = **Hybrid**

Ein **Feed-Katalog** (N Upstream-Feeds) plus **Mandant abonniert eine Teilmenge**
der Feeds und legt **Sicht-Filter** (Area-of-Interest-Bounding-Box, Flight-Level-
Band, Track-Kategorie) darüber. Das subsumiert die Reinformen „geteilter Himmel +
Sichten" (Variante A) und „Feed je Mandant" (Variante B) und bricht bei Wachstum
(neue Region/Sensorik je Kunde) nicht.

**Konzeptuelles Datenmodell** (Detail-Schema in ADR 0006):
- **Tenant** — die isolierte Organisation (z. B. „Leitstelle Frankfurt").
- **User** — gehört zu genau einem Tenant; hat Rollen (mind. `operator`,
  `tenant-admin`).
- **Feed** — ein Upstream-CAT062/065-Strom mit Metadaten (Quelle, Region,
  Sensor-Mix als **Feed-Eigenschaft**, nicht als Per-Track-Tag).
- **Subscription** — verbindet Tenant ↔ Feed (welche Feeds ein Mandant sehen
  darf).
- **ViewConfig** — mandanten-/nutzer-seitige Sicht (Zentrum, Radius, AOI,
  FL-Band, Layer).
- **Entitlement** — Feature-Freischaltungen je Tenant (Feature-Flags als Daten).

### 3. Vertrauens-/Isolationsgrenze (die sicherheitskritische Kern-Entscheidung)

**Cross-Tenant-Daten-Isolation ist das dominierende Sicherheitsrisiko** des
Pivots: ein Mandant darf **niemals** die Lage eines anderen sehen („Frankfurt
sieht nie Stuttgart"), außer es ist explizit freigegeben. Daraus folgen
**verbindliche Prinzipien** für alle stream-/identitäts-berührenden Pakete:

- **Server-seitige Autorisierung pro Subscription.** Der Broadcaster wird von
  „alle an alle" auf **prädikat-gefilterte Zustellung** umgebaut (ein Track geht
  an einen Client nur, wenn `feed ∈ subscriptions ∧ AOI ∧ FL-Band ∧ Kategorie`).
  Kein Filtern im Browser.
- **Fail-closed.** Ohne auflösbaren, gültigen Tenant-Kontext **keine** Daten.
- **Pflicht-Negativtests.** „Mandant A bekommt nie einen Track von Mandant B" ist
  ein **Gate** (WF2-22): Negativ-, Property- und Fuzz-Tests gegen das
  Subscription-Prädikat.
- **Browser-Rand (ADR 0003) bleibt und wird mandanten-bewusst.** TLS/Auth primär
  am Reverse-Proxy/Ingress; die hier ergänzte **Autorisierung** (Tenant-/Rollen-
  Scope) ist die in ADR 0003 vertagte „Autorisierungs-ADR" — jetzt entschieden.
- **Audit-Spur.** Sicherheitsrelevante Zugriffe (welcher Tenant sah welchen
  Scope) werden auditierbar geloggt (WF2-23), als Observability- **und**
  Zertifizierungs-Nachweis.

### 4. Kommerz-Scope = Feature-Flags ja, Billing zurückgestellt

- **Entitlements/Feature-Flags als Daten** im Store (`tenant.HasFeature(...)`,
  WF2-50) — entkoppelt von Bezahlung.
- **Stripe-Billing ist zurückgestellt** (WF2-51 ruht). Falls es je kommt, **nur
  als separate Plane** (Webhook → Entitlement-Update); der ASD-Kern importiert
  **nie** einen Billing-SDK. Das hält das sicherheitsrelevante ASD frei von
  Billing-Kopplung.

### 5. Zertifizierungs-Haltung

- **Mandanten-Isolation wird Teil der Hazard-Analyse** (ROADMAP #7 FHA): Cross-
  Tenant-Leckage ist ein zu betrachtender Gefährdungsfall.
- Die **Kommerz-Plane bleibt außerhalb** des zertifizierbaren ASD-Kerns.
- **Rückverfolgbarkeit** über neue Anforderungs-Familien `FR-TEN-*` (Tenancy) und
  `NFR-SEC-*` (Isolation): Anforderung → ADR → Code → Test.

### 6. 12-Factor-Grenze: Infrastruktur vs. fachliche Konfiguration

Der Entwurf-Satz „Config wandert ganz aus ENV in die DB" gilt **nur für fachliche
Mandanten-Konfiguration** (Zentrum, Radius, FL-Bänder, Abos, Entitlements). **Infra-
Parameter und Geheimnisse** (DB-URL, Stream-Endpunkt, TLS, OIDC-Client-Secret)
bleiben **ENV/12-Factor** — sie gehören nicht in die Mandanten-DB.

### 7. Rückwärtskompatibilität: Single-Tenant als degenerierter Fall

Der heutige 1.x-Betrieb bleibt als **degenerierter Spezialfall** lauffähig: **ein
Default-Tenant, ein Feed, keine Auth** (Entwicklung/On-Prem-Einzelinstallation).
Das erlaubt eine **schrittweise Migration** ohne Big-Bang und hält die
Stufen 1–2 unabhängig testbar.

### 8. Abgrenzung (was diese ADR NICHT entscheidet)

- **Datastore-Wahl/Schema, Migrations-Tooling, Cache** → **ADR 0006**.
- **Cloud-Ingest-Transport** (direkt-Multicast vs. NATS/Kafka), Gateway-Design →
  **ADR 0007**.
- **Identitäts-Anbindung im Detail** (OIDC@Proxy vs. eingebaut) → ADR 0006.
- **Echte Per-Track-Sensor-Provenienz** (FLARM-Diskriminator) bliebe eine
  CAT062-ICD-Änderung und wird **erst bei Stufe 4** als `from-wayfinder`-Issue
  mit Firefly verhandelt (WF2-42). Bis dahin: Sensor-Mix = **Feed-Eigenschaft**.
- Das Charter-Prinzip **„kein Firefly-Code-Import, Kopplung nur über den
  CAT062-Draht-Vertrag"** bleibt unangetastet.

## Begründung

- **Hybrid skaliert.** A und B sind Sonderfälle des Hybrids; nur er übersteht den
  Übergang „eine Region" → „mehrere Regionen/Sensoren je Kunde" ohne Rearchitektur.
- **Isolation zuerst.** In einem sicherheitsrelevanten Lagebild ist Cross-Tenant-
  Leckage der Worst Case; ihn zum Gate (Negativtests) zu machen, ist
  billiger-jetzt als später.
- **Kein Billing im ASD-Kern.** Entitlements als Daten geben die Produkt-
  Flexibilität (Tiers), ohne ein Bezahl-SDK in den zertifizierungs-relevanten
  Pfad zu ziehen — konsistent mit „kein Krypto-/Auth-Eigenbau im ASD" (ADR 0003).
- **12-Factor bleibt für Infra.** Geheimnisse in der Mandanten-DB wären ein
  Bruch mit Cloud-native-Praxis und ein Sicherheitsrisiko.
- **Degenerierter Single-Tenant-Fall** hält die Migration schrittweise und die
  bestehenden Tests/Deployments am Leben.

### Verworfene Alternativen

- **Variante A allein (nur Sichten auf einen Himmel):** verworfen — bricht, sobald
  Mandanten unterschiedliche Regionen/Sensoren haben. (Als Spezialfall im Hybrid
  enthalten.)
- **Variante B allein (Feed je Mandant):** verworfen als alleiniges Modell —
  schwergewichtig, erzwingt Feed-Vervielfachung auch dort, wo Sichten genügen.
  (Als Spezialfall im Hybrid enthalten.)
- **Voll-SaaS mit Stripe-Billing jetzt:** zurückgestellt — orthogonal zur ASD-
  Mission; eine Billing-Kopplung im sicherheitsrelevanten System ist unerwünscht.
- **Plot-Ebenen-Sensor-Filterung im Wayfinder-Decoder** (Entwurf Epic 3 wörtlich):
  verworfen — CAT062 ist fusioniert, und es bräuchte Firefly-internes Wissen
  (Charter-Verstoß). Ersetzt durch Sensor-Mix auf **Feed-Ebene** + aus dem Vertrag
  *ableitbare* Provenienz (ADS-B ◆ existiert).
- **Kein Pivot (Status quo):** verworfen — erreicht das 2.0-Ziel nicht.

## Konsequenzen

- **Neue Anforderungen im Register** (`docs/requirements/`):
  - **FR-TEN-001** — Mandantenfähigkeit nach Hybrid-Modell (Tenant/Feed/
    Subscription/ViewConfig/Entitlement als Konzept). Implementierung folgt
    WF2-1x/2x.
  - **NFR-SEC-003** — Cross-Tenant-Daten-Isolation: server-seitige AuthZ pro
    Subscription, fail-closed, **Pflicht-Negativtests**. Implementierung folgt
    WF2-21/22.
- **Folge-ADRs:** ADR 0006 (Persistenz/Identität) und ADR 0007 (Cloud-Ingest/
  Fan-out) sind die nächsten Stufe-0-Pakete (WF2-01, WF2-02).
- **ROADMAP/STATUS:** WF2-00 ist erledigt; nächster Schritt = **WF2-01 / ADR
  0006**. (Konsistenz-Regel ROADMAP §7: STATUS „Nächster Schritt" == ROADMAP §0/§1.)
- **Broadcaster-Umbau** (all-to-all → scoped) ist invasiv und sicherheitskritisch
  (WF2-21); neue Persistenz-/Auth-Flächen vergrößern die Angriffsfläche und den
  Betriebsaufwand (DB, evtl. Stream-Bus).
- **Cross-Project:** Hybrid-Feed-Modell und etwaige Per-Track-Provenienz werden
  Firefly **erst beim Erreichen der jeweiligen Stufe** als `from-wayfinder`-Issue
  vorgelegt (Vorwarnung in `todo-for-firefly.md`).

## Ehrliche Grenze

- Diese ADR setzt die **Richtung**; sie garantiert keine Isolation, solange die
  scoped-Zustellung (WF2-21) und ihre Negativtests (WF2-22) nicht implementiert
  und verifiziert sind.
- **Datastore, Identitäts-Anbindung und Ingest-Transport sind bewusst noch
  offen** (ADR 0006/0007) — diese ADR präjudiziert sie nicht über die genannten
  Prinzipien (Stateless-App, Infra-Secrets in ENV) hinaus.
- Die **formale Zertifizierung** bleibt außerhalb des Code-Projekts (CLAUDE.md
  §7) — wir bauen zertifizierungs-*fähig*, inklusive der Mandanten-Isolations-
  Nachweise, aber ohne das Versprechen einer Zulassung.
