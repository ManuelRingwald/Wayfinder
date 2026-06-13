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
| [#9](https://github.com/manuelringwald/firefly/issues/9) | `time` ohne Wandzeit-/UTC-Bezug | **bleibt offen, wird zentraler** | CAT062 I062/070 *ist* das ASTERIX-Time-of-Day-Feld, das Wayfinder direkt konsumiert. Solange Firefly dort "Sekunden seit Szenario-Start" statt echter UTC-Tageszeit einträgt, kann Wayfinder dem Lotsen keine korrekte UTC-Uhrzeit am Track anzeigen. |
| [#10](https://github.com/manuelringwald/firefly/issues/10) | Schema-Versionierung | **geschlossen** | Wird Teil der für CAT062 vorgesehenen ICD-Dokumentation (versionierter Schnittstellen-Vertrag) statt eines JSON-Schema-Felds. |

---

## Offen für Firefly: #9 (UTC Time-of-Day)

**Beobachtung:** `Timestamp` (`crates/firefly-core/src/time.rs`) ist aktuell
"Sekunden seit Szenario-Start", auch im CAT062-Adapter (I062/070).

**Problem für den Produktivbetrieb:** Eine ASD muss dem Lotsen eine **UTC-Uhrzeit**
am Track anzeigen können. I062/070 muss dafür echte ASTERIX-Time-of-Day
(Sekunden seit UTC-Mitternacht, 1/128 s) enthalten.

**Empfehlung:** Migration auf echtes UTC-Time-of-Day in I062/070, wie in Fireflys
Roadmap (ADR 0014, Produktions-Phase) bereits vorgesehen.

**GitHub Issue:** [Firefly #9](https://github.com/manuelringwald/firefly/issues/9) `from-wayfinder`

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
