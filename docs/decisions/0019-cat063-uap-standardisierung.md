# ADR 0019 — CAT063-Decoder auf Standard-UAP (I063/050 Sensor, RE/SP-tolerant)

- **Status:** akzeptiert
- **Datum:** 2026-07-06
- **Schnittstellen-relevant:** ja (CAT063-Eingangs-Vertrag, ICD → 3.0.0, **breaking**;
  Auslöser Fireflys ADR 0032 / Firefly-Issue #55 `from-wayfinder`)
- **Bezug:** Spiegel zu Fireflys ADR 0032; löst das Byte-Format aus ADR 0010 ab.
  Fundament für den per-Quelle-Fehlergrund (#197 → H4, RE-Feld).

## Kontext

Wayfinders CAT063-Decoder (ADR 0010) folgte Fireflys ursprünglicher,
**nicht standardkonformer** CAT063-UAP: FSPEC `0xE0`, Sensor-Identität in
I063/010, I063/030 auf FRN 2, I063/060 auf FRN 3. Firefly hat diese UAP mit
**ADR 0032** (ICD 3.0.0, breaking) auf die echten EUROCONTROL-FRN-Positionen
korrigiert — analog zur CAT062-UAP-Korrektur (Fireflys ADR 0015):

- **I063/010** (FRN 1) = **SDPS**-Identität (SAC/SIC wie I062/010, Default 25/2).
- **I063/030** (FRN 3) = Time of Message.
- **I063/050** (FRN 4, NEU) = **Sensor**-Identität (SAC 0, SIC = `sensor_id`).
- **I063/060** (FRN 5) = Sensor Configuration & Status (CON + GO/NOGO, FX).

→ FSPEC `0xE0` → `0xB8`, Record 7 → 9 Oktette.

Weil sich damit die Byte-Positionen und die Semantik von I063/010 ändern, muss
Wayfinders Decoder in **lockstep** nachziehen — ein alter Decoder liest die neuen
Blöcke nicht (falsche FSPEC, Sensor-Identität am falschen Platz).

## Entscheidung

1. **Standard-UAP-Decode.** `pkg/cat063.DecodeSensorBlock` liest FRN 1 (I063/010
   → SDPS), FRN 3 (I063/030 → ToD), FRN 4 (I063/050 → Sensor) und FRN 5
   (I063/060 → CON, variabel via FX). FSPEC `0xB8`. `SensorStatus.SAC`/`.SIC`
   tragen jetzt die **Sensor**-Identität (aus I063/050); neue Felder
   `SDPSSAC`/`SDPSSIC` tragen die SDPS-Identität (aus I063/010, Rückverfolgbarkeit).
2. **Längen-bewusste Vorwärtskompatibilität.** Der Decoder kennt die
   Längen-Regeln der übrigen Standard-Items (I063/015 = 1 B; I063/070/081/091/092
   = 2 B; I063/080/090 = 4 B) und überspringt das **Reserved-Expansion-(RE, FRN
   13)** und **Special-Purpose-(SP, FRN 14)**-Feld über ihr explizites Längen-
   Oktett. Damit bricht der Decoder nicht, wenn Fireflys ADR 0033 (H3) den
   per-Quelle-Fehlergrund additiv im RE-Feld nachreicht. Ein wirklich
   unbekannter FRN (dessen Länge der Decoder nicht kennt) wird **verworfen**
   statt fehlinterpretiert (robuster Decoder, CLAUDE.md §7).
3. **Verdrahtung unverändert.** Receiver-Dispatch (`0x3F` → `handleSensorStatus`),
   Health-Registry (`RecordSensors`), Broadcast (`FeedStatusMessage`) und das
   gelbe Sensor-Banner bleiben, wie in ADR 0010 beschrieben — sie werten nur
   `SensorStatus.Operational` aus, das unverändert bleibt.

## Begründung

- **Standardtreue & Determinismus (CLAUDE.md §7).** Wayfinder folgt dem echten
  EUROCONTROL-Vertrag; der Decoder ist byte-genau gegen Fireflys 3.0.0-Referenz-
  Dump verifizierbar (Grundwahrheit, CLAUDE.md §6).
- **Lockstep statt still.** Ein Breaking-Wire-Change wird beidseitig per ADR
  nachgezogen (wie ADR 0015/AP2). Firefly-first mergen+deployen, Wayfinder
  unmittelbar danach — dazwischen fällt nur das Sensor-Banner kurz aus
  (CAT062-Tracks/CAT065 unberührt).
- **RE/SP-Toleranz jetzt schon.** So ist H2 vorwärtskompatibel zu H3, ohne dass
  zwischen den beiden Deployments eine dritte Bruchstelle entsteht.

## Konsequenzen

- **Byte-genaue Referenz-Vektoren neu** (`decoder_test.go`): FSPEC `0xB8`,
  SDPS 25/2 in I063/010, Sensor in I063/050; zusätzliche Tests
  `TestDecodeStandardFSPEC`, `TestDecodeSkipsReservedExpansion`,
  `TestDecodeRejectsSpareFRN`, SDPS/Sensor-Split.
- `SensorStatus` bekommt `SDPSSAC`/`SDPSSIC`; `SAC`/`SIC` = Sensor (I063/050).
  Kein Konsument liest heute die Identitäts-Felder (nur `Operational`) — keine
  weitere Verdrahtung betroffen.
- Anforderung **FR-DATA-006** aktualisiert (Standard-UAP, I063/050, RE/SP-Skip).
- **Deploy-Kopplung:** zusammen mit Firefly ADR 0032 ausrollen. Cross-Project via
  Firefly #55.

## Ehrliche Grenze

Wayfinder dekodiert nur die für das Sensor-Banner nötigen Items
(I063/010/030/050/060) und **überspringt** alle übrigen (I063/015, Bias-Items,
RE/SP). Der per-Quelle-Fehlergrund im RE-Feld wird erst in H4 (ADR-Folge)
ausgewertet; hier wird das RE-Feld nur toleriert.
