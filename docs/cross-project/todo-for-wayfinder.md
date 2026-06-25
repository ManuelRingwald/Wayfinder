# TODOs für Wayfinder (aus dem Firefly-Projekt)

> Hier trägt der Projektverantwortliche Beobachtungen/Wünsche aus der
> Firefly-Arbeit ein, die Wayfinder betreffen. Siehe `README.md` für den
> Workflow.

| Issue | Titel | Status |
|-------|-------|--------|
| [#72](https://github.com/manuelringwald/wayfinder/issues/72) | CAT063 Sensor Status — Wayfinder-Decoder für `0x3F` (aus Firefly ADR 0022 / Issue #32) | ✅ erledigt (2026-06-25, Branch `claude/kind-feynman-kr2co1`) |

**Issue #72** wurde als Folgearbeit aus Fireflys ADR 0022 / Issue #32
(`from-wayfinder`) angelegt. Wayfinder hat umgesetzt:
- `pkg/cat063` CAT063-Decoder (WF-1)
- `pkg/health.Registry.RecordSensors` + Receiver-Dispatch (WF-2)
- Broadcast-Pfad Option B + gelbes Banner (WF-3)
- Doku/ADR 0010 (WF-4)

Das gelbe Sensor-Degradierungs-Banner ist vollständig aktiviert.
