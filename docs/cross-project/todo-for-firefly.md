# Rückmeldungen an Firefly – Produktionsreife der Schnittstelle

> Diese Datei sammelt Beobachtungen aus der Wayfinder-Entwicklung, die für den
> **produktiven Einsatz von Firefly bei einem ANSP** relevant sind. Gedacht zum
> Übertragen ins Firefly-Projekt (eigenes Repo) und dort als ADRs/Requirements/
> Issues weiterzuverfolgen.
>
> Siehe `docs/cross-project/README.md` für den Übertragungs-Workflow.

---

## Stand nach Fireflys ADR 0014 (Pivot Lernprojekt → Produktion, CAT062-Konsum)

Die ursprünglichen Issues #6–#10 wurden formuliert, als Wayfinder noch gegen den
JSON/WebSocket-Pfad (`firefly-server`, `/ws`) geplant war. Mit ADR 0014
konsumiert Wayfinder stattdessen **ASTERIX CAT062 über UDP-Multicast** (Fireflys
ADR 0006) als produktiven ASD-Kontrakt. Das verändert die Relevanz der Punkte
unten grundlegend:

| # | Thema | Status | Begründung |
|---|-------|--------|------------|
| [#6](https://github.com/manuelringwald/firefly/issues/6) | Pub/Sub-Fan-out statt Replay | **geschlossen** | Multicast ist nativ Fan-out — mehrere ASD-Instanzen hören unabhängig dieselbe Gruppe; das Replay-Problem entsteht für CAT062 nicht. |
| [#7](https://github.com/manuelringwald/firefly/issues/7) | Auth/Autorisierung auf `/ws` | **transformiert** | Multicast hat keine Verbindungs-/Token-Authentifizierung. Die Sicherheitsfrage verschiebt sich auf **Netz-Isolation des Multicast-Pfads** (Firefly-seitig) und den **Browser-Rand von Wayfinder** (Wayfinder-seitig, eigener ADR dort). |
| [#8](https://github.com/manuelringwald/firefly/issues/8) | Nachrichtentyp-Diskriminator im JSON | **geschlossen** | ASTERIX ist selbstbeschreibend (CAT/LEN/FSPEC) — ein zusätzlicher Typ-Diskriminator ist für CAT062 gegenstandslos. |
| [#9](https://github.com/manuelringwald/firefly/issues/9) | `time` ohne Wandzeit-/UTC-Bezug | **erledigt** | Firefly implementiert echtes ASTERIX UTC-Time-of-Day in I062/070 (Firefly PR #11, Commit `a2449cf`). |
| [#10](https://github.com/manuelringwald/firefly/issues/10) | Schema-Versionierung | **geschlossen** | Wird Teil der für CAT062 vorgesehenen ICD-Dokumentation (versionierter Schnittstellen-Vertrag) statt eines JSON-Schema-Felds. |

---

## #9 (UTC Time-of-Day) — erledigt ✅

Firefly liefert jetzt echtes ASTERIX UTC-Time-of-Day in I062/070 (statt
"Sekunden seit Szenario-Start"). Damit kann eine ASD dem Lotsen eine korrekte
UTC-Uhrzeit am Track anzeigen. **GitHub Issue:**
[Firefly #9](https://github.com/manuelringwald/firefly/issues/9) `from-wayfinder`
— kann geschlossen werden.

---

## Stand nach Wayfinder M1 (CAT062-Pipeline + Live-Karte, abgeschlossen 2026-06-13)

Wayfinder konsumiert jetzt den vollen CAT062-Vertrag produktiv: `latitude`,
`longitude`, `track_num`, `confirmed`, `coasting`, `vx`, `vy` werden auf einer
MapLibre-Karte als farbige Symbole mit Labels dargestellt (M1.4.a–c). Keine
neuen Schnittstellen-Probleme dabei aufgefallen — der CAT062-Vertrag (ICD
v1.0.0) deckt den aktuellen Wayfinder-Bedarf vollständig ab. Noch nicht
genutzt, aber bereits im Vertrag vorhanden: `update_age_s`,
`position_uncertainty_m`, `mode_3a`, `icao_addr` (geplant für spätere
ASD-Elemente wie Unsicherheits-Ringe und Label-Inhalte).

---

## Kontext: Was Firefly schon richtig macht (positiv vermerkt)

- **Sicherheitsrelevante Statusfelder werden bereits durchgereicht**:
  `confirmed`, `coasting`, `update_age_s`, `position_uncertainty_m` (ADR 0008
  bei Firefly, kodiert u.a. in I062/080, I062/290, I062/500) — genau das, was
  eine ASD für die Darstellung von Unsicherheits-Ringen und
  Tentative/Coasting-Zuständen braucht.
- **Health-/Readiness-Probes** (`/health`, `/ready`) sind vorhanden
  (Kubernetes-tauglich, ADR 0003 bei Firefly).
- **12-Factor-Konfiguration** über Env-Vars (`FIREFLY_CAT062_GROUP`,
  `FIREFLY_CAT062_PORT`, …) ist bereits umgesetzt.
- **CAT062-Empfänger-Seite bereits bewiesen** (Fireflys ADR 0006, Häppchen
  D.1–D.3): Decoder, Rückprojektion und echter Multicast-Empfänger sind
  Ende-zu-Ende getestet — gute Referenz für den Wayfinder-Decoder.

---

## Wayfinder 2.0 (Multi-Mandanten-Plattform) — künftige Schnittstellen-Bezüge

> Stand 2026-06-19. Wayfinder richtet sich auf **Wayfinder 2.0** aus (Konzept:
> `docs/design/wayfinder-2.0-konzept.md`; zentrale Roadmap: `docs/ROADMAP.md`
> §0–§1). Mandanten-Modell = **Hybrid** (Feed-Katalog + Abos + Sicht-Filter).
> Die folgenden Punkte **können** Firefly betreffen. Sie sind **noch nicht**
> akut — `from-wayfinder`-Issues werden **erst beim Erreichen der jeweiligen
> Stufe** erstellt (nicht prophylaktisch), damit keine verfrühten/gegensätzlichen
> Anforderungen entstehen. Hier nur als Vorwarnung dokumentiert.

| Thema | Wayfinder-Paket | Mögliche Firefly-Wirkung | Auslöser für Issue |
|-------|-----------------|--------------------------|--------------------|
| **Per-Track-Sensor-Provenienz** (FLARM/SSR/PSR/ADS-B-Diskriminator je Track) | WF2-40/42 (Stufe 4) | CAT062 trägt heute **keinen** sauberen Per-Track-Sensortyp; echte Provenienz wäre eine **ICD-Änderung** (neues Item/Bit). Enabler: Fireflys SDPS-001 (FEP-Ingestion, #19) + Sensor-Registrierung (#8). | ✅ Issue [#30](https://github.com/manuelringwald/firefly/issues/30) (ICD v2.5.0) |
| **Feed-pro-Mandant** (Hybrid-Modell, Variante-B-Anteil) | WF2-20 (Stufe 2) | Mehrere Mandanten mit eigenem „Himmel" ⇒ Fireflys **Multicast-Gruppen-/Instanz-Modell** (eine Gruppe je Feed/Einzugsgebiet) abstimmen. | Beginn Stufe 2 |
| **Konfigurierbarer System-Referenzpunkt** | WF2-20 | Roadmap #4 (Firefly): je Feed ggf. eigener I062/100-Referenzpunkt. | Beginn Stufe 2 |
| **Ende-zu-Ende-HA** | WF2-52/53 (Stufe 5) | Fireflys SDPS-002 (#20, Main/Standby) ↔ Wayfinders stateless Skalierung + Ingest-Gateway-HA für durchgängige Verfügbarkeit. | Beginn Stufe 5 |
| **FHA / Hazard-Analyse** | WF2-21/22 | Roadmap #7 (gemeinsam): muss **Cross-Tenant-Isolations-Hazards** aufnehmen. | mit #7 |

**Wichtig (kein Konflikt mit dem Charter):** Diese Punkte ändern **nicht** das
Prinzip „kein Firefly-Code-Import, Kopplung nur über den CAT062-Draht-Vertrag".
Sensor-Mix bleibt vorerst eine **Feed-Eigenschaft** auf Wayfinder-Seite; eine
ICD-Erweiterung wird nur angestoßen, falls echte Per-Track-Provenienz operativ
nötig wird — dann beidseitig per ADR.

---

## WF2-42 (Stufe 4) — Explizite Per-Track-Provenienz → ICD v2.5.0 ⏳

**GitHub Issue:** [Firefly #30](https://github.com/manuelringwald/firefly/issues/30)
`from-wayfinder` (angelegt 2026-06-22) — **offen**.

**Auslöser:** WF2-40 (gemerged, PR #52) zeigt die Track-Herkunft als Symbol-Form
(◆ ADS-B / ▢ SSR/Mode S / ○ PSR), klassifiziert sie aber **heuristisch im
Frontend** (`frontend/src/map/provenance.js`): ADS-B aus `adsb_age_s`-Frische,
SSR/Mode S aus Präsenz von `icao_addr`/`mode_3a`/Callsign, sonst PSR. Die
Präsenz von `icao_addr` ist nur ein **Proxy** für „kooperativ" — keine echte
Sensor-Provenienz, und **FLARM** ist damit gar nicht erkennbar.

**Vorschlag (entschieden: Enum + `source_ages`; additiv, ICD-Minor v2.5.0):**

| Feld | Typ | Form | Zweck |
|------|-----|------|-------|
| `provenance` | string-Enum (erweiterbar) | `psr`·`ssr`·`mode_s`·`adsb`·`flarm`·`combined` | Autoritative, dominante Herkunft (Tracker-Präzedenz) |
| `source_ages` | Objekt | `{ "psr": 4.0, "mode_s": 2.0, "adsb": 1.5 }` | Per-Sensortyp-Alter (nur vorhandene Typen) |

**Begründung (im Issue ausgeführt):** (1) Fusions-Tracks (`combined`) — Enum
allein lässt das Frontend die **Frische der Teilsignale** (PSR vs. ADS-B) nicht
validieren, dafür braucht es `source_ages`. (2) **ASTERIX-Treue** — `source_ages`
bildet **I062/290 (System Track Update Ages)** ab (PSR ✓/SSR/MDS/ADS ✓); der
Tracker führt die Daten ohnehin, die Weitergabe ist sauber und auditierbar.
(3) **FLARM** — kein Standard-I062/290-Subfeld; das Enum schließt die Lücke, das
Alter kommt als dokumentiertes Vendor-Subfeld (I062/SP).

**Wayfinder-Folge (nach Lieferung):** `trackProvenance()` →
`track.provenance ?? <Heuristik-Fallback>` (vorwärts-/rückwärtskompatibel);
Frische aus `source_ages.adsb` statt `adsb_age_s`-Proxy. Decoder zieht
byte-genau gegen Fireflys Referenz-Dump nach (Charter §2/§6).

---

## Feed-Gesundheit: Sensor-Teilausfall → Gelb (FF-1 + CAT063) ⏳

**GitHub Issue:** [Firefly #32](https://github.com/manuelringwald/firefly/issues/32)
`from-wayfinder` (angelegt 2026-06-25) — **offen**.

**Auslöser:** Wayfinder AP4 (Feed-Gesundheit pro Feed, PR #71) zeigt eine Ampel
pro Feed, gespeist nur aus dem CAT065-Heartbeat. Die **Gelb-Semantik wird
umgewidmet**: „leerer Himmel" ist kein Warnzustand. Gelb soll künftig
**degradierte Fusion** bedeuten — von N konfigurierten Radaren liefern nur noch
M < N. Künftige Ampel: grün = alle Sensoren liefern · gelb = 1…N−1 still ·
rot = kein Heartbeat oder alle still.

**Befund:** Firefly hat das Sensor-Modell strukturell (`SensorId`, drei Radare
in der Multi-Radar-Scene, `contributing_sensors` pro Track), aber **keinen
Laufzeit-Begriff von „Sensor ausgefallen"** (Sensoren statisch via
`with_sensor`; kein „Sensor 2 liefert seit X s keine Plots"). CAT065 kann das
nicht sauber tragen (I065/040 NOGO ist binär; I065/020 ist Batch Number, kein
Sensor-Count).

**Mit dem Projektverantwortlichen abgestimmt (2026-06-25):**

| AP (Firefly) | Inhalt | Hinweis |
|--------------|--------|---------|
| **FF-1** | Laufzeit-Sensor-Health: per `SensorId` „last plot at", Silent-Timeout aus `scan_period`, globaler Aktiv/Silent-Snapshot, Metrik `firefly_sensor_active{sensor_id}` | **zuerst Umfang/Semantik abstimmen** (Voraussetzung) |
| **FF-2/3** | **CAT063 Sensor Status Messages** (Kategorie `0x3F`, ein Record/Sensor) auf derselben Gruppe/Port; periodische Aussendung (`FIREFLY_CAT063_*`) | Format **entschieden: CAT063** (standard-treu, „M von N"-fähig) |
| **ICD/ADR** | Neue Kategorie in `docs/ICD-CAT062.md` (Changelog v2.4.0) + ADR „CAT063 Sensor Status" + byte-genaue Referenz-Vektoren | — |

**Abgrenzung zu #30:** verwandt (beide brauchen Sensor-Awareness), aber #30 ist
Per-**Track**-Provenienz in CAT062 (welcher Sensortyp erzeugte *diesen* Track),
dieses Issue ist System-/Feed-Gesundheit in CAT063 (welche Sensoren leben
gerade). Nicht vermischen.

**Wayfinder-Folge (nach Lieferung):** WF-1 robuster CAT063-Decoder
(`pkg/cat063`) + Referenz-Vektor-Test · WF-2 Health-Registry um Sensor-Soll/-Ist
erweitern (`Color()`: gelb = `0 < aktiv < gesamt`, „leerer Himmel" raus aus
gelb) · WF-3 UI „2 / 3 Radare aktiv (Sensor 2 still)" in `FeedStatusChip` +
Admin-Ampel.

---

## ORCH-5 (Auto-Orchestrierung) — Quell-Eingangs-Kontrakt + Live-Input-Adapter ⏳

**GitHub Issue:** [Firefly #35](https://github.com/manuelringwald/firefly/issues/35)
`from-wayfinder` (angelegt 2026-06-29) — **offen**.

**Auslöser:** Wayfinders Auto-Orchestrierung (ADR 0012) fährt pro Feed eine
eigene Firefly-Instanz. In `main` bereits: ORCH-1 (generische `source_config` +
`coverage_bbox`), ORCH-2/2b/3 (`InstanceBackend` + Docker-Adapter + Reconciler),
ORCH-2c 3a + 3a-API (AES-256-GCM-Secret-Speicher + write-only Admin-API +
`SecretResolver`). **Was fehlt:** der Orchestrator kann eine Instanz starten,
ihr aber **nicht sagen, woraus** sie rechnen soll — der Docker-Adapter setzt
bewusst **kein** `FIREFLY_SOURCES`, weil der **Eingangs-Kontrakt auf
Firefly-Seite** noch nicht ratifiziert ist.

**Bitte um (Firefly-Seite):**

| AP | Inhalt | Hinweis |
|----|--------|---------|
| **Env-Kontrakt** | Env-getriebene Quell-Konfig (Var. A: eine JSON-`FIREFLY_SOURCES`; Var. B: indizierte `FIREFLY_SOURCE_N_*`) | Firefly wählt Form/Namen; Wayfinder mappt im Docker-Adapter |
| **Secret-Env** | Konvention, **welche Env** je Quelle das (beim Start aufgelöste) Credential trägt | z. B. `FIREFLY_SOURCE_0_SECRET` |
| **Input-Adapter** | `adsb_opensky` (OpenSky-Client), `flarm_aprs` (OGN/APRS), `radar_asterix` (ASTERIX-Eingang CAT048/001) — Ports & Adapters, Kern format-neutral | vermutlich eigener ADR + Meilenstein (SDPS/FEP-Familie) |
| **Versionierung** | Eingangs-Kontrakt versionieren (analog zur Ausgabe-ICD?) | — |

**Abgrenzung (kein Charter-Konflikt):** Der **Ausgabe**-Vertrag (CAT062/UDP)
bleibt unberührt; dies ist ein **neuer Eingangs-Kontrakt** auf Firefly-Seite,
kein Code-Import. Quell-Vokabular bleibt Wayfinder-seitig (`adsb_opensky`/
`flarm_aprs`/`radar_asterix`); Firefly definiert nur die Env-Form + Adapter.

**Wayfinder-Folge (nach Ratifizierung):** Docker-Adapter übersetzt
`source_config` → die von Firefly gewählten Envs; `SecretResolver` injiziert die
aufgelösten Werte in die vereinbarten Secret-Envs; End-to-End-Abnahme
(Quelle → Firefly-Instanz → CAT062 → Wayfinder-Lagebild).
