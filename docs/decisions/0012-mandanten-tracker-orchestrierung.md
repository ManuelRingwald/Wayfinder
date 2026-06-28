# ADR 0012 — Mandanten-eigene Tracker-Instanzen & Auto-Orchestrierung

- **Status:** **AKZEPTIERT** ✅ (2026-06-27).
- **Datum:** 2026-06-26
- **Schnittstellen-relevant:** **teilweise** — der **CAT062/063/065-Draht-Vertrag
  bleibt unverändert**. Schnittstellen-Wirkung entsteht nur **eingangsseitig bei
  Firefly** (neue generische Quell-Adapter + Coverage-Konfig, ADR-pflichtig in
  Firefly, ORCH-5) und **betrieblich bei Wayfinder** (neue Control-Plane, die
  Tracker-Instanzen startet). Der Strom Firefly→Wayfinder ist davon unberührt.
- **Bezug:** **ADR 0011** (Zero-Touch: Feed-CRUD + `pkg/feedmanager` Live-Join —
  hierauf baut der Lebenszyklus auf), **ADR 0007** (Cloud-Ingest & Feed-Fan-out:
  `FeedSource`-Abstraktion, Ingest-Gateway, NATS — der Orchestrator ist die
  betriebliche Fortschreibung), **ADR 0005** (Multi-Mandanten, Isolationsgrenze),
  **ADR 0003** (Browser-Rand, Vertrauensgrenze, fail-closed), **WF2-21.2**
  (autoritativer AOI/FL-Filter). Firefly-seitig: **Fireflys ADR 0006**
  (CAT062-Ausgabe-Vertrag, format-neutraler Kern, Ports & Adapters), **Fireflys
  SDPS-001 (#19)** (FEP-Sensor-Ingestion), **Fireflys ADR 0021** (konfigurierbarer
  System-Referenzpunkt). Anforderungs-Register: neu **FR-ORCH-001…003**, bezogen
  **NFR-SEC-003/004**, **NFR-SCALE-001**.

> ℹ️ **Auslöser:** Betreiber-Wunsch (2026-06-26). Jeder Mandant soll seinen
> **eigenen Sensor-Mix** sehen (z. B. nur ADS-B für Speyer, PSR+SSR+ADS-B für
> Frankfurt), seinen **eigenen geografischen Ausschnitt** und seine eigenen
> Features. Der Zielzustand: **„Wayfinder + DB starten; pro Kunde einen Mandanten
> und einen Feed über die Oberfläche anlegen — und das Zuordnen des Feeds startet
> automatisch die passende Firefly-Instanz."** Endziel: **1 Firefly-Instanz pro
> Mandant**, Mandant sieht nur seine AOI-Tracks, **Firefly muss mit jedem ASD
> funktionieren und darf keine Wayfinder-Spezialfälle abdecken**.

---

## Kontext

### Ist-Zustand (am Code geerdet)

- **Feed-Katalog + Zuweisung stehen (ADR 0011).** Ein Feed trägt
  `multicast_group`, `port`, `region`, `sensor_mix`. `pkg/feedmanager` joint/
  verlässt die Multicast-Gruppe **zur Laufzeit** beim Anlegen/Löschen eines Feeds.
  Mandanten abonnieren Feeds (`subscriptions`).
- **Mandanten-Isolation steht (Stufe 2).** Ein Mandant sieht **nur** Tracks seiner
  abonnierten Feeds, gefiltert auf **AOI + FL-Band** (`view_configs`,
  `broadcast.Scope.filterView`, WF2-21.2), server-seitig, fail-closed,
  property-/fuzz-getestet (WF2-22).
- **`sensor_mix` ist heute reines Metadatum.** Es beschreibt, *was* ein Feed
  enthält (Audit/Health-Chip via CAT063), filtert aber **nichts** — ein Mandant
  am Feed bekommt **alle** Tracks dieses Feeds. Es gibt **keinen** Per-Track-
  Sensorfilter.
- **Firefly ist heute Simulator + Tracker**, gespeist aus Szenen
  (`FIREFLY_SCENE`), nicht aus **echten Live-Quellen** (OpenSky/FLARM/Radar).
  Output ist CAT062 auf `FIREFLY_CAT062_GROUP:PORT`.
- **Es gibt keine Orchestrierung.** Firefly-Instanzen werden heute **von Hand**
  gestartet (Container/`cargo run`). Wayfinder kennt keine Tracker-Instanzen,
  sondern nur den Multicast-Strom, der „irgendwo" entsteht.

### Spannungsfeld

Der Betreiber will **Sensor-Trennung pro Mandant** und **Auto-Start** der Tracker
— ohne dass Firefly Mandanten kennt. Das erzeugt drei Spannungen:

1. **Wo wird der Sensor-Mix realisiert?** Entweder in Wayfinder (Per-Track-Filter,
   Heuristik auf fusionierten Tracks) oder **an der Quelle** (jeder Tracker
   bekommt nur die vereinbarten Quellen). Ersteres legt anwendungs-spezifische
   Logik in den Track-Pfad; Letzteres hält Firefly generisch.
2. **Wo wird der geografische Ausschnitt begrenzt?** In Firefly (Coverage), in
   Wayfinder (Anzeige-AOI) — oder beidseitig? Doppelte Logik wäre eine
   Fehlerquelle; ein einziger Ort an der falschen Stelle wäre teuer/unsicher.
3. **Wer startet die Tracker?** Eine neue Control-Plane in Wayfinder, die
   Container/Pods startet, ist mächtig — und damit ein **Privilegien-Sprung** für
   eine Anwendung, die zugleich dem Browser-Rand zugewandt ist (ADR 0003).

---

## Entscheidung

### 1. Sensor-Trennung an der Quelle: eine dedizierte Firefly-Instanz je Feed/Mandant

Der Sensor-Mix wird **an der Datenquelle** realisiert, **nicht** per Wayfinder-
Track-Heuristik: Bekommt Speyers Firefly nur ADS-B, produziert sie nur ADS-B-
abgeleitete System-Tracks. Damit ist „nur ADS-B für Speyer" eine reine
**Quell-/Feed-Konfiguration**, kein Sonderfall im Track-Pfad.

- **Lebenszyklus-Einheit ist der Feed:** **1 Feed = 1 Multicast-Gruppe = 1
  Firefly-Instanz.** Im Regelfall ist ein Feed einem Mandanten gewidmet → liest
  sich als „1 Firefly pro Mandant". Mehrere Mandanten *können* denselben Feed
  abonnieren (jeder mit eigener AOI) — dann teilen sie eine Tracker-Instanz.
- **Sensor-Upgrade** (Punkt 5 des Betreiber-Use-Cases: Speyer will später SSR+PSR)
  = die **Quell-Konfig des Feeds erweitern**; der Reconciler fährt die Instanz mit
  neuer Konfig nach. Kein neuer Feed, kein Wayfinder-Track-Filter.

### 2. Firefly bleibt autonom & generisch (Ports & Adapters) — keine Tenant-Kenntnis

Firefly bekommt **generische Eingangs-Adapter** (passend zu Fireflys eigener
Ports-&-Adapters-Architektur, ADR 0006 dort). Diese sind **anwendungs-neutral**
und nützen jedem ASD — **kein** Wayfinder-Wissen, **kein** Wayfinder-Import.

- Erster Adapter **`adsb_opensky`**: pollt die **OpenSky-REST-API**
  `GET /states/all?lamin&lomin&lamax&lomax` (~5–10 s, Auth via Client-
  Credentials), wandelt jeden State-Vector (icao24, callsign, lat/lon, baro-/geo-
  alt, velocity, true_track, vertical_rate) in einen Firefly-**Plot/Measurement**
  → Tracker → CAT062.
- Weitere Adapter ohne Architektur-Bruch: **`adsb_beast`** (dump1090/Beast),
  **`flarm_aprs`** (OGN/APRS-IS), **`radar_asterix`** (echtes CAT048/CAT001 =
  Fireflys SDPS-001 #19; Sensor-Identität über **SAC/SIC**).
- **Konfiguration rein über Fireflys Env/Config** (z. B. `FIREFLY_SOURCES=…`,
  `FIREFLY_COVERAGE_BBOX=…`, `FIREFLY_CAT062_GROUP/PORT`). Firefly weiß nicht, dass
  ein „Mandant" existiert — es ist ein Tracker mit Quellen, einem Coverage-Bereich
  und einem Ausgabe-Multicast.

> **Diese Adapter sind überwiegend Firefly-Arbeit** (eigenes Repo, eigener
> Charter). Wayfinder stößt sie per `from-wayfinder`-Issue an und stimmt die
> Konfig-Oberfläche ab (ORCH-5). Bis sie existieren, kann der Orchestrator gegen
> Fireflys **Szenen-Modus** (`FIREFLY_SCENE`) als Platzhalter-Quelle entwickelt
> werden.

### 3. Geografie sauber getrennt: Coverage in Firefly, autoritative AOI in Wayfinder

Es gibt **zwei verschiedene** geografische Begriffe — bewusst an **zwei
verschiedenen** Stellen:

| Begriff | Ort | Charakter |
|---|---|---|
| **Coverage-/Quell-BBox** (z. B. das OpenSky-Abfragefenster) | **Firefly** | **grobe äußere Grenze**, generische Tracker-Konfig. Man kann/soll nicht „ganz Europa" für einen Speyer-Mandanten von OpenSky ziehen; jede begrenzte Quelle hat eine BBox, ARTAS hat ein definiertes Coverage-Volumen. **Kein Wayfinder-Spezialfall.** |
| **Anzeige-/Isolations-AOI** (Kreis + Radius + FL, live verstellbar) | **Wayfinder** | **präziser innerer Filter**, autoritative Billing-/Sicherheits-Grenze (WF2-21.2). Unverändert. |

**Resolution (der Einwand des Assistenten, ratifiziert):** Firefly erhält eine
**grobe äußere** Coverage-BBox; Wayfinder behält den **präzisen inneren** AOI/FL-
Filter. Wayfinder **leitet** die Coverage-BBox beim Provisionieren aus der
Mandanten-AOI **+ Marge** ab und übergibt sie Firefly **in rein generischen
Begriffen** (BBox-Koordinaten, kein Tenant-Bezug). Coarse-outer-bound vs.
precise-inner-filter — **komplementär, defense-in-depth, keine doppelte Logik**,
und Firefly bleibt ahnungslos über Mandanten.

### 4. Wayfinder wird Orchestrator: `InstanceBackend` (Docker → Kubernetes)

Wayfinder bekommt eine **Tracker-Instanz-Abstraktion**:

```
InstanceBackend interface { Start(spec) ; Stop(id) ; Status(id) }
```

- **Docker-Adapter zuerst** (lokal/Dev/Einzelhost): startet je Feed einen
  Firefly-Container mit der abgeleiteten Env (Quellen, Coverage-BBox, Multicast-
  Gruppe/Port).
- **Kubernetes-Adapter später** (Produktion, skaliert): erzeugt je Feed ein
  Deployment/Pod mit Resource-Requests/Limits.
- **Multicast-Gruppen-/Port-Allokation** liegt bei Wayfinder (der Feed hält die
  Werte bereits) — kollisionsfrei pro Feed.

### 5. Reconciler (Operator-Muster): Soll-aus-Feed-Aktivität

Ein **Reconciler** hält **Soll = Ist**, idempotent:

- Feed hat **≥ 1 aktives Abo** → **genau eine** Firefly-Instanz läuft mit dessen
  Quell-/Coverage-Konfig.
- Feed wird **idle** (kein Abo mehr) oder **gelöscht** → Instanz wird **abgebaut**.
- **Crash-Recovery & Orphan-Cleanup:** Der Reconciler vergleicht periodisch
  gewünschte Feeds gegen laufende Instanzen und korrigiert Drift (abgestürzte
  Instanz neu starten, verwaiste Instanz ohne Feed stoppen). Instanz-Identität ↔
  `feed_id`.

> **Verhältnis zu ADR 0007:** ADR 0007 entwarf `FeedSource` + Ingest-Gateway +
> NATS für **viele Konsumenten an einem Feed**. ORCH adressiert die
> **Erzeugung** der Feeds (Tracker-Instanzen). Beide sind kompatibel: der
> Orchestrator startet die Quelle, das Fan-out/Gateway verteilt sie. NATS bleibt
> die Skalierungs-Option für die Verteilung (ORCH-6/WF2-53), nicht für das
> Starten.

### 6. 🔒 Sicherheits-Leitplanke: getrennte, least-privilege Control-Plane

Prozesse/Container zu starten ist ein **Privilegien-Sprung** (Docker-Socket /
K8s-API). Verbindliche Invariante:

- Der **Orchestrator/Reconciler läuft als eigene Control-Plane-Komponente**,
  **getrennt** vom browser-/WS-zugewandten Wayfinder-Prozess (ADR 0003).
- Der Internet-/Browser-Rand darf **nie** direkt Container/Pods starten; er
  schreibt nur **Soll-Zustand** (Feed + Quell-Konfig in die DB). Der Reconciler
  liest Soll und wirkt auf das `InstanceBackend` — **Least-Privilege**, ein
  schmaler, auditierbarer Pfad.
- **Quell-Credentials** (OpenSky-Client-Credentials, FLARM-/Radar-Zugänge) werden
  als **Secrets je Feed** geführt (nie im Klartext-DTO, nie an den Browser; vgl.
  OpenAIP-Key-Isolation ONB-6), und nur dem `InstanceBackend` beim Start gereicht.

---

## Begründung

- **Sensor-Trennung an der Quelle** hält Firefly generisch und den Wayfinder-
  Track-Pfad frei von Heuristik. Sie ist **deterministisch** (die Quelle liefert
  nur, was sie liefert) statt heuristisch (ADS-B-Alter raten). Sie trifft exakt
  den Betreiber-Wunsch „Firefly = autonomer ARTAS-artiger Tracker".
- **Coverage vs. AOI getrennt** vermeidet doppelte Filter-Logik und legt jede
  Verantwortung an ihren natürlichen Ort: die Quelle begrenzt grob (Effizienz/
  Datenmenge), Wayfinder filtert präzise (Isolation/Billing/Live-Sicht).
- **Feed als Lebenszyklus-Einheit** baut bruchlos auf ADR 0011 (`pkg/feedmanager`,
  Feed-CRUD) auf — der Feed trägt schon Gruppe/Port; ORCH ergänzt nur die
  **Erzeugung** der Quelle hinter dem Multicast.
- **Reconciler statt imperativem Start** ist das erprobte cloud-native Muster
  (Kubernetes-Operator): idempotent, crash-fest, drift-korrigierend — passend zur
  12-Factor-/K8s-Ausrichtung (Charter §7).
- **Getrennte Control-Plane** ist die zentrale Sicherheits-Invariante: das
  Privileg „Container starten" darf nicht am selben Prozess hängen, der das
  öffentliche Browser-Bild ausliefert.
- **Ports & Adapters in Firefly** ist dort bereits die Architektur-Linie (ADR 0006
  Firefly) — Eingangs-Adapter fügen sich symmetrisch zu den vorhandenen Ausgangs-
  (Encoder/Transport-)Adaptern.

### Verworfene Alternativen

- **Per-Track-Sensorfilter in Wayfinder** (frühere „Option B": `sensor_filter`
  auf dem Abo, Go-Port von `trackProvenance()`): heuristisch (rät die Herkunft aus
  ADS-B-Alter/ICAO-Adresse), legt anwendungs-spezifische Logik in den
  sicherheitskritischen Track-Pfad und liefert nie 100 % Trennschärfe.
  **Verworfen** zugunsten der Quell-Trennung.
- **ICD-`source_type` pro Track** (frühere „Option C": Firefly stempelt die
  Herkunft je Track in CAT062): sauber, aber eine **Schnittstellen-Last** für ein
  Problem, das die Quell-Trennung gegenstandslos macht. **Verworfen** (bleibt als
  separater Provenienz-Wunsch WF2-42 unabhängig bestehen).
- **AOI nur in Firefly** (Firefly filtert auf den Mandanten-Ausschnitt): verlagert
  die **autoritative Isolations-/Billing-Grenze** aus Wayfinder heraus und zwingt
  Firefly, einen Mandanten-Begriff zu kennen. **Verworfen** — verletzt die
  Firefly-Autonomie. Coverage-BBox (generisch) ja, Mandanten-AOI nein.
- **AOI doppelt (Firefly + Wayfinder identisch):** zwei Wahrheiten desselben
  Filters = Drift-/Fehlerquelle. **Verworfen** zugunsten coarse-outer/precise-
  inner.
- **Orchestrator im browser-zugewandten Prozess:** spart eine Komponente, hängt
  aber „Container starten" an den Internet-Rand. **Verworfen** (Sicherheit).
- **Mandant startet Tracker selbst / manuell wie heute:** widerspricht dem
  Auto-Start-Wunsch und skaliert nicht. **Verworfen.**

---

## Konsequenzen

Umsetzung in **Arbeitspaketen** (eigene Ankündigung + Freigabe je Paket,
Charter §3). Reihenfolge nach Abhängigkeit:

| AP | Inhalt | Stufe · Modell |
|----|--------|----------------|
| **ORCH-0** | Dieser ADR 0012 (Richtung + Entscheidungen 1–6) | S4 · Opus 4.8 |
| **ORCH-1** | Feed-Quell-Datenmodell (Wayfinder): `source_config` (erweiterbare Quell-Liste `adsb_opensky`/`flarm_aprs`/`radar_asterix` mit BBox/SIC-SAC/Cred-Ref) + abgeleitete `coverage_bbox`; Migration, Admin-API, UI-Quell-Builder (BBox-Vorschlag aus Mandanten-AOI + Marge); Feed-UX (Sensor-Mix als Checkboxen, Default-Template) | S3–S4 · Sonnet/Opus |
| **ORCH-2** 🔒 | `InstanceBackend`-Abstraktion + **Docker-Adapter**; separater Control-Plane-Prozess, Least-Privilege; Multicast-Allokation; Secret-Handling je Feed | S4–S5 · Opus/Fable |
| **ORCH-3** 🔒 | Reconciler: Soll-aus-Feed-Aktivität, idempotent, Crash-Recovery, Orphan-Cleanup; Instanz-Identität ↔ `feed_id` | S4–S5 · Opus/Fable |
| **ORCH-4** | Orchestrierungs-UX: „Feed zuweisen → Instanz startet", Instanz-Status-Chip (provisioning/running/failed), Start/Stop, Anbindung an Feed-Health (AP4) | S3 · Sonnet |
| **ORCH-5** | **Cross-Project (Firefly):** generische Live-Quell-Ingestion (`SourceAdapter`: `adsb_opensky`, `flarm_aprs`, Radar CAT048/CAT001 = SDPS-001 #19) + Coverage-BBox-Konfig, via Env/Config, null Wayfinder-Kopplung; `from-wayfinder`-Issue + Abstimmung | S5 · Fable/Opus |
| **ORCH-6** 🔒 | Skalierung & HA: K8s-`InstanceBackend`, Resource-Limits, Autoscaling, Secret-Management; koppelt an WF2-52/53 | S4–S5 · Opus/Fable |

- **Schema:** `feeds.source_config` (JSONB) + `feeds.coverage_bbox` (ORCH-1);
  Secret-Referenzen je Feed (ORCH-2, analog OpenAIP-Key-Isolation).
- **Backend (Wayfinder):** neue Control-Plane-Komponente (Reconciler +
  `InstanceBackend`), `pkg/adminapi` (Quell-Konfig-CRUD), `pkg/store`
  (Feed-Quell-/Secret-Repo). **Kein** Eingriff in `pkg/broadcast` (Isolation
  unverändert).
- **Backend (Firefly, cross-project):** Eingangs-Adapter + Coverage-Konfig —
  eigener Firefly-ADR, eigener Charter.
- **Frontend:** Quell-Builder im Feed-Dialog, Instanz-Status in Mandant-/Feed-
  Ansichten.
- **Doku:** Register (FR-ORCH-001…003), `docs/INSTALLATION.md` (Orchestrator-
  Deployment, Quell-Konfig), `docs/TECHNICAL.md` (Control-Plane, `InstanceBackend`,
  Reconciler, neue Env/Secrets), `docs/BETRIEB.md` (Instanz-Lebenszyklus betreiben),
  Milestone-Doku je AP.

---

## Ehrliche Grenze

- **Dieser ADR entscheidet die Richtung**, nicht die Detailmechanik. Goroutine-/
  Container-Lifecycle, exakte Reconcile-Schleife, Secret-Backend (K8s-Secrets vs.
  Vault) und die genaue `source_config`-Schemaform werden im jeweiligen AP-Häppchen
  festgelegt und isoliert getestet — **kein** weiterer Architektur-Sprung nötig.
- **Firefly-Live-Quellen (ORCH-5) sind die größte Abhängigkeit** und überwiegend
  **Firefly-Arbeit**. Ohne sie liefert der Orchestrator nur Szenen-Tracker
  (Demo/Platzhalter). Echte ADS-B/FLARM/Radar-Ingestion ist ein **eigener,
  nicht-trivialer** Firefly-Meilenstein (SDPS-001).
- **OpenSky** hat **Rate-/Credit-Limits** (anonym strenger, authentifiziert
  großzügiger) und ist eine **Crowdsourced-Quelle** — Abdeckung/Latenz sind nicht
  ATC-grade. Für Demo/Lagebewusstsein geeignet; **nicht** als zertifizierte
  Surveillance-Quelle. Das ist eine bewusste Einstiegs-Wahl, kein Betriebsversprechen.
- **Ressourcen:** N Mandanten = N Tracker-Instanzen. Der Ressourcenhunger ist
  real; horizontale Skalierung (ORCH-6/K8s) ist die Antwort, hat aber Grenzen
  (CPU/RAM je Instanz, Multicast-Gruppen-Budget). „Beliebig viele" Mandanten sind
  durch Cluster-Kapazität begrenzt — ehrlich zu kommunizieren.
- **Bekannte offene Punkte:** Mehr-Mandanten-an-einem-Feed (geteilte Instanz) ist
  möglich, aber die Coverage-BBox müsste dann die **Vereinigung** der AOIs decken
  — Detail für ORCH-1/-3.
