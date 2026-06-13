# Arbeitsstand (Handover-Notiz)

> **Zweck:** Diese Datei ist der schnelle Wiedereinstieg — egal ob am PC oder
> Handy. Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

- **Zuletzt aktualisiert:** 2026-06-13 (Branch `claude/loving-turing-2obzk6`:
  Charter-Pivot Lernprojekt → Produktion; Schnittstellen-Wechsel auf CAT062/UDP)
- **Branch:** `claude/loving-turing-2obzk6` — Projekt ist noch Greenfield, kein
  Code vorhanden.

> 🔁 **Pivot vollzogen: Wayfinder konsumiert CAT062/UDP-Multicast statt
> JSON/WebSocket.** `CLAUDE.md` wurde komplett neu gefasst (Produktionsbetrieb,
> Modell-Angabe pro Schritt jetzt Pflicht, Abschnitt 2 = vollständiger
> CAT062-Draht-Vertrag mit FRN/Item-Tabelle). Begründung und Konsequenzen stehen
> in Fireflys `docs/decisions/0014-produktionsbetrieb-statt-lernprojekt-wayfinder-cat062.md`.
>
> Cross-Project-Status (`docs/cross-project/todo-for-firefly.md`): Issues
> **#6, #8, #10** geschlossen (durch CAT062-Architektur gegenstandslos), **#7**
> transformiert (Netz-Isolation Multicast + Wayfinder-Browser-Rand), **#9** (UTC
> Time-of-Day) bleibt offen und wird zentraler.

---

## 1. Wo wir gerade stehen

- **Noch kein Code.** Bisher existieren nur `CLAUDE.md`, `LICENSE`, `README.md`
  und `docs/cross-project/todo-for-firefly.md`.
- Die alte M1–M5-Planung (WebSocket-Client, 2D-Canvas-Rendering, …) ist mit dem
  Pivot **obsolet** — sie war gegen den JSON/WebSocket-Pfad formuliert. Eine neue
  Meilenstein-Planung steht noch aus (siehe Abschnitt 3).

## 2. Gesetzte Entscheidungen (Fundament)

| Thema | Entscheidung | Quelle |
|-------|--------------|--------|
| Betriebsmodus | Produktionsbetrieb (nicht Lernprojekt) | Fireflys ADR 0014 |
| Schnittstelle | **CAT062 over UDP-Multicast** (nicht JSON/WebSocket) | Fireflys ADR 0006 + 0014, `CLAUDE.md` Abschnitt 2 |
| Sprache | Code Englisch, Doku/Chat Deutsch | `CLAUDE.md` Abschnitt 4 |
| Stack | **Go vorgeschlagen**, noch nicht ratifiziert | `CLAUDE.md` Abschnitt 5, ADR 0001 ausstehend |

## 3. Nächster Schritt (hier geht es weiter!)

➡️ **ADR 0001 (Wayfinder) ratifizieren** — Stack-Entscheidung (Go, MapLibre GL,
Transport zum Browser) festigen, bevor der erste Code entsteht.

Danach, in kleinen Häppchen (Ankündigung + Freigabe, `CLAUDE.md` Abschnitt 3):
1. Projekt-Skelett (Go-Modul, Tooling: `go vet`, `golangci-lint`, `gofmt`).
2. CAT062-Decoder-Grundgerüst gegen Fireflys byte-genaue Referenz-Vektoren
   (FSPEC-Parsing, dann FRN-Items aus der Tabelle in `CLAUDE.md` Abschnitt 2).
3. UDP-Multicast-Empfänger (Loopback-Test gegen Fireflys `firefly-multicast`-Sender).
4. Health-/Readiness-Probes, 12-Factor-Konfiguration.

Erst Erklärung → Rückfragen/Go → dann kleine, testbare Umsetzung.

## 4. So steige ich wieder ein (Kurzbefehle)

```bash
# Noch kein Code — Einstieg ist CLAUDE.md + dieses STATUS.md lesen.
```

Doku-Einstieg: `CLAUDE.md` (Abschnitt 2 = CAT062-Vertrag),
`docs/cross-project/todo-for-firefly.md` (Cross-Project-Status).
