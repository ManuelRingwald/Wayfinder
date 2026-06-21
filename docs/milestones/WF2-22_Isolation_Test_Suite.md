# WF2-22 — Isolations-Testsuite (Property + Fuzz)

> **Stufe:** 2 (Mandanten-isolierter Datenstrom) · **Paket:** WF2-22 ·
> **Einstufung:** 🔒 S4 · Opus 4.8 · **Grundlage:** ADR 0005, NFR-SEC-003
> („Pflicht-Negativtests als Gate"); baut auf WF2-21 (scoped Fan-out).
> **Test-only — kein Produktivcode geändert** (kein Befund).

## Warum (fachlich)

ADR 0005 / NFR-SEC-003 verlangen für die Cross-Tenant-Isolation —
den Worst-Case eines sicherheitsrelevanten Lagebilds — **Pflicht-Negativtests als
Gate**. WF2-21.1/21.2 lieferten *punktuelle* Negativtests; WF2-22 macht daraus ein
**breites, generiertes Regressions-Gate**, das das Isolations-Prädikat gegen
künftige Änderungen am Fan-out absichert. Es prüft zwei Invarianten:

- **Isolation (kein False-Positive über die Grenze):** ein Client erhält **nie**
  einen Track aus einem nicht-abonnierten Feed **oder** außerhalb seiner Sicht.
- **Safety/Vollständigkeit (kein False-Negative):** ein Track im erlaubten Feed,
  innerhalb der Sicht **oder ohne FL (fail-open)**, wird **zugestellt** — nichts
  wird fälschlich verschluckt.

## Was (technisch)

`pkg/broadcast/isolation_test.go` (Paket-internes Test-File — greift auf das
unexportierte Prädikat `filterView`/`admits`/`AllowsFeed` zu):

- **`viewAdmitsOracle`** — eine **unabhängige, bewusst einfache** Referenz des
  Sicht-Prädikats (inklusive AOI-Grenzen, fail-open FL-Band). Differential-Testing:
  ein Logikfehler in `admits` *oder* in der Referenz fällt gegeneinander auf.
- **`TestFilterViewMatchesOracle`** (deterministisch geseedet, 50 000 Iterationen):
  zufällige Scopes + Batches; `filterView` behält **exakt** die vom Oracle
  zugelassenen Tracks — **beide Richtungen** (kein Über- *und* kein Unter-Filtern).
- **`TestBroadcasterIsolationProperty`** (Ende-zu-Ende durch den echten `Run` +
  `RegisterClient` + `trackChan`): 8 Clients mit zufälligem Scope, 400 zufällige
  Batches über 6 Feeds; **jeder** von einem Client empfangene Track liegt in dessen
  Scope (Feed erlaubt **und** Sicht zulässt). Treibt das reale Fan-out-/Goroutine-/
  Eviction-Verhalten, nicht nur das Prädikat.
- **`FuzzScopeFilter`** (Go-Fuzz) über die **realistische finite Domäne**
  (NaN/Inf werden übersprungen — echte Positionen/FL sind endlich): `filterView`
  stimmt mit dem Oracle überein (Sicht-Dimension) **und** `AllowsFeed` ist exakt
  (Feed-Dimension), kein Panic. In ~8 s lokal **755 000 Ausführungen, 0 Fehler**.

**Determinismus/Reproduzierbarkeit** (zert-tauglich): die Property-Tests nutzen
feste Seeds; die Fuzz-Seeds (`f.Add`) laufen als normale Tests in jedem CI-Lauf
mit, erweitertes Fuzzing ist on-demand:

```bash
go test ./pkg/broadcast/ -run '^$' -fuzz FuzzScopeFilter -fuzztime 30s
```

**Ergebnis:** kein Isolations-Bug gefunden; das Prädikat ist gegen eine
unabhängige Referenz über eine breite generierte Domäne **und** per Fuzzing
abgesichert. Keine Produktivcode-Änderung. Gates grün (`go build/vet/test`,
`gofmt`, `scripts/pg-test.sh`; Fuzz separat); `go 1.25` unverändert.

## Abgrenzung / Nächstes

- **Reine Test-Härtung** — keine neuen ENV-Vars/Metriken/Betriebsmodi (INSTALLATION/
  TECHNICAL unverändert bis auf einen Hinweis zum Fuzz-Lauf).
- **Damit ist der sicherheitskritische Kern (WF2-20/21/22) testseitig abgesichert.**
- **Nächster Schritt: WF2-23** — Pro-Mandant-Metriken & Audit-Log (`tenant`-Label
  an den Metriken, Audit-Event „welcher Tenant sah welchen Scope"), schließt
  Stufe 2 ab.
