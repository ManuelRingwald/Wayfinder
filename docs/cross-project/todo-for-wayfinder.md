# TODOs für Wayfinder (aus dem Firefly-Projekt)

> Hier trägt der Projektverantwortliche Beobachtungen/Wünsche aus der
> Firefly-Arbeit ein, die Wayfinder betreffen. Siehe `README.md` für den
> Workflow.

| Issue | Titel | Status |
|-------|-------|--------|
| [#72](https://github.com/manuelringwald/wayfinder/issues/72) | CAT063 Sensor Status — Wayfinder-Decoder für `0x3F` (aus Firefly ADR 0022 / Issue #32) | ✅ erledigt (2026-06-25, Branch `claude/kind-feynman-kr2co1`) |
| [#245](https://github.com/manuelringwald/wayfinder/issues/245) | CAT062 ICD 3.7.0 — I062/390 Flugplan-Korrelation (Anzeige) + manuelle Korrelation (Kommando-API) | ✅ erledigt (2026-07-16): Teil A Anzeige (FR-DATA-013); Teil B manuelle Korrelation in 4 Häppchen (ADR 0024, FR-ORCH-013) |
| [#257](https://github.com/manuelringwald/wayfinder/issues/257) | Doku-Spiegel: Fireflys ADR 0042 — Verbund-Rolle SDPS-Server-Funktion (CAT252-Ersatz) | ✅ erledigt (2026-07-16): Wayfinder-ADR 0025 + CLAUDE.md §1-Verweis |

**Issue #72** wurde als Folgearbeit aus Fireflys ADR 0022 / Issue #32
(`from-wayfinder`) angelegt. Wayfinder hat umgesetzt:
- `pkg/cat063` CAT063-Decoder (WF-1)
- `pkg/health.Registry.RecordSensors` + Receiver-Dispatch (WF-2)
- Broadcast-Pfad Option B + gelbes Banner (WF-3)
- Doku/ADR 0010 (WF-4)

Das gelbe Sensor-Degradierungs-Banner ist vollständig aktiviert.

**Issue #245** (`from-firefly`, aus Fireflys ADR 0038/0039) trug die
Flugplan-Korrelation über den Draht. **Teil A** (Anzeige I062/390: Plan-Callsign
+ ADEP→ADES, Callsign-Mismatch) ist als FR-DATA-013 erledigt. **Teil B** (manuelle
Korrelation als Bedienhandlung, Rückkanal Wayfinder→Firefly) ist über ADR 0024 in
vier Häppchen gebaut (FR-ORCH-013): H1 Command-Client (`pkg/fireflycmd`), H2
Server-Endpoint + Gating (`pkg/correlationapi`), H3 Detail-Panel-UI, H4
`FIREFLY_WS_TOKEN`-Injektion in die je-Feed gespawnten Firefly-Instanzen.

**Issue #257** (`from-firefly`, Spiegel zu Fireflys ADR 0042) ist rein
dokumentarisch: Wayfinder erbringt die **Serving-Hälfte der SDPS-Server-Funktion**
(Konsumenten-Verwaltung + Zuschnitt), Firefly die Erzeugung + Fire-and-Forget-
Multicast; CAT252 bewusst verworfen. Festgehalten in **ADR 0025** und mit einem
Verweis in `CLAUDE.md` §1.
