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
