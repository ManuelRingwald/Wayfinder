# Arbeitsstand (Handover-Notiz)

> **Zweck:** Diese Datei ist der schnelle Wiedereinstieg — egal ob am PC oder
> Handy. Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

- **Zuletzt aktualisiert:** 2026-06-13 (Branch `claude/loving-turing-2obzk6`:
  **M1.1 abgeschlossen** — CAT062-Decoder vollständig implementiert (FRN
  1,4,5,6,7,9,11,12,13,14,16), alle Tests grün, gegen byte-genauen
  Firefly-Referenz-Vektor validiert)
- **Branch:** `claude/loving-turing-2obzk6` — CAT062-Decoder-Grundgerüst fertig
  und validiert; nächster Schritt M1.2 (UDP-Multicast-Receiver)

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

**M1.1 (CAT062-Decoder-Grundgerüst): ✅ Abgeschlossen**

Implementiert:
- ✅ `pkg/cat062/types.go` — DecodedTrack, DataSourceID, TimeOfDay, WGS84Position, CartesianPosition, Velocity, TrackStatus, UpdateAge, PositionAccuracy
- ✅ `pkg/cat062/fspec.go` — FSPEC-Parser mit FX-Chaining
- ✅ `pkg/cat062/decoder.go` — DecodeDataBlock, DecodeRecord mit FRN 1,4,5,6,7,9,11,12,13,14,16
  - FRN1 (I062/010): SAC/SIC ✅
  - FRN4 (I062/070): Time-of-Day ✅
  - FRN5 (I062/105): WGS84-Position ✅
  - FRN6 (I062/100): System-Cartesian (i24 **sign-extension**) ✅
  - FRN7 (I062/185): Velocity ✅
  - FRN9 (I062/060): Mode 3/A ✅
  - FRN11 (I062/380): ICAO-Adresse ✅
  - FRN12 (I062/040): Track-Nummer ✅
  - FRN13 (I062/080): Track-Status (variable FX, vereinfacht) ✅
  - FRN14 (I062/290): PSR-Age ✅
  - FRN16 (I062/500): Position-Genauigkeit (APC) ✅
- ✅ `pkg/cat062/decoder_test.go` — **alle 10 Tests grün** (TestSignExtendI24,
  TestFSPECParser, TestDecodeDataSourceID, TestDecodeTimeOfDay,
  TestDecodeWGS84Position, TestDecodeVelocity, TestDecodeCartesianPosition,
  TestDecodeMultipleTracks, TestReferenceVector, BenchmarkDecodeRecord)

**Validierung gegen Firefly (M1.1.d):**
- `TestReferenceVector` dekodiert den byte-exakten Dump aus Fireflys
  `single_track_matches_reference_dump` (firefly-asterix/src/cat062.rs) und
  prüft alle Felder gegen die dort erzeugten Werte — der Wire-Vertrag zwischen
  Firefly (Encoder) und Wayfinder (Decoder) ist somit Ende-zu-Ende verifiziert.

**Qualitäts-Gates:** `go test ./...` ✅, `go vet ./...` ✅, `gofmt` ✅

## 2. Gesetzte Entscheidungen (Fundament)

| Thema | Entscheidung | Quelle | Status |
|-------|--------------|--------|--------|
| Betriebsmodus | Produktionsbetrieb (nicht Lernprojekt) | Fireflys ADR 0014 | ✅ |
| Schnittstelle | **CAT062 over UDP-Multicast** (nicht JSON/WebSocket) | Fireflys ADR 0006 + 0014, `CLAUDE.md` Abschnitt 2 | ✅ |
| Sprache | Code Englisch, Doku/Chat Deutsch | `CLAUDE.md` Abschnitt 4 | ✅ |
| Stack | Go + MapLibre GL JS + WebSocket-Server-Push | `CLAUDE.md` Abschnitt 5, ADR 0001 | ✅ ratifiziert 2026-06-13 |

## 3. Nächster Schritt (hier geht es weiter!)

➡️ **M1.2 (UDP-Multicast-Receiver)**
- Netzwerk-Binding auf `239.255.0.62:8600` (oder Env-Konfiguration via
  `FIREFLY_CAT062_GROUP`/`FIREFLY_CAT062_PORT`)
- Empfangsschleife: ein Datagramm = ein CAT062-Datenblock → `DecodeDataBlock`
- Loopback-Test gegen Fireflys `firefly-multicast`-Sender
- Fehlerbehandlung für truncated/malformed Datagramme (Decoder ist bereits
  robust — verwerfen statt Panic)
- Health-/Readiness-Probes (12-Factor, `CLAUDE.md` Abschnitt 7)

Danach M1.3 (WebSocket-Server) und M1.4 (Frontend/MapLibre).

Erst Erklärung → Rückfragen/Go → dann kleine, testbare Umsetzung
(`CLAUDE.md` Abschnitt 3).

## 4. So steige ich wieder ein (Kurzbefehle)

```bash
# Tests laufen und Dekodierung prüfen
go test ./pkg/cat062 -v

# Oder einzelne Tests:
go test ./pkg/cat062 -run TestDecodeCartesianPosition -v

# Code ansehen:
ls -la pkg/cat062/
# decoder.go        (Haupt-Dekoder + i24-Helfer)
# decoder_test.go   (Tests)
# types.go          (Domain-Typen)
# fspec.go          (FSPEC-Parser)
```

Einstieg:
- `CLAUDE.md` Abschnitt 2 = CAT062-Draht-Vertrag + FRN-Tabelle
- `docs/cross-project/todo-for-firefly.md` = Cross-Project-Status
- Commit-Log: `git log --oneline | head` (letzte Arbeit + Messaging)
