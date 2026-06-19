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
| **Per-Track-Sensor-Provenienz** (FLARM/SSR/PSR/ADS-B-Diskriminator je Track) | WF2-40/42 (Stufe 4) | CAT062 trägt heute **keinen** sauberen Per-Track-Sensortyp; echte Provenienz wäre eine **ICD-Änderung** (neues Item/Bit). Enabler: Fireflys SDPS-001 (FEP-Ingestion, #19) + Sensor-Registrierung (#8). | Beginn Stufe 4 |
| **Feed-pro-Mandant** (Hybrid-Modell, Variante-B-Anteil) | WF2-20 (Stufe 2) | Mehrere Mandanten mit eigenem „Himmel" ⇒ Fireflys **Multicast-Gruppen-/Instanz-Modell** (eine Gruppe je Feed/Einzugsgebiet) abstimmen. | Beginn Stufe 2 |
| **Konfigurierbarer System-Referenzpunkt** | WF2-20 | Roadmap #4 (Firefly): je Feed ggf. eigener I062/100-Referenzpunkt. | Beginn Stufe 2 |
| **Ende-zu-Ende-HA** | WF2-52/53 (Stufe 5) | Fireflys SDPS-002 (#20, Main/Standby) ↔ Wayfinders stateless Skalierung + Ingest-Gateway-HA für durchgängige Verfügbarkeit. | Beginn Stufe 5 |
| **FHA / Hazard-Analyse** | WF2-21/22 | Roadmap #7 (gemeinsam): muss **Cross-Tenant-Isolations-Hazards** aufnehmen. | mit #7 |

**Wichtig (kein Konflikt mit dem Charter):** Diese Punkte ändern **nicht** das
Prinzip „kein Firefly-Code-Import, Kopplung nur über den CAT062-Draht-Vertrag".
Sensor-Mix bleibt vorerst eine **Feed-Eigenschaft** auf Wayfinder-Seite; eine
ICD-Erweiterung wird nur angestoßen, falls echte Per-Track-Provenienz operativ
nötig wird — dann beidseitig per ADR.
