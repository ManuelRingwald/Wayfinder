# ADR 0020 — CAT063 SRC-REASON dekodieren + Feed-Health-Chip zeigt den Grund

- **Status:** akzeptiert
- **Datum:** 2026-07-06
- **Schnittstellen-relevant:** ja (konsumiert CAT063 I063/RE, Fireflys ICD 3.1.0,
  **additiv**; Auslöser Firefly ADR 0033 / Firefly-Issue #55 `from-wayfinder`)
- **Bezug:** baut auf ADR 0019 (CAT063-Standard-UAP-Decoder, RE/SP längen-tolerant)
  auf; **schließt Issue #197**.

## Kontext

Der Feed-Health-Chip (ADR 0010) zeigt bei einem degradierten Feed nur **„SENSOR
AUSFALL"** — also **dass** eine Quelle still ist, nicht **warum**. Für den
Betreiber ist das zu wenig: Eine degradierte ADS-B-Quelle kann drei Ursachen
haben, die **unterschiedliche Reaktionen** verlangen — Netz/Firewall
(**unreachable**, Zugangsdaten sind ok), falsche Zugangsdaten (**auth**) oder
Drosselung (**rate_limited**). Ohne diese Unterscheidung tippt der Betreiber im
Zweifel sinnlos Credentials nach, obwohl ein Egress blockiert.

Firefly liefert den Grund seit ICD 3.1.0 (ADR 0033) im **CAT063 I063/RE**
(Reserved Expansion Field, FRN 13) als Vendor-Subfeld **`SRC-REASON`**
(`1=unreachable`, `2=auth`, `3=rate_limited`). Wayfinders Decoder **überspringt**
das RE-Feld bisher nur längen-tolerant (ADR 0019). Dieser ADR wertet es aus und
zeigt den Grund an.

## Entscheidung

1. **Decoder liest SRC-REASON.** `pkg/cat063` parst das I063/RE-Feld statt es nur
   zu überspringen: LEN, Sub-Feld-Spec (`0x80` ⇒ SRC-REASON vorhanden), dann das
   Reason-Oktett. `SensorStatus.Reason` trägt `"unreachable"` / `"auth"` /
   `"rate_limited"` bzw. `""` (kein RE / unbekannter Code — tolerant, kein Fehler).
   Der Rest des RE-Felds wird längen-bewusst übersprungen; SP bleibt reines
   Skippen.
2. **Feed-Ebene: dominanter Grund.** `cat063.DominantReason([]SensorStatus)`
   verdichtet die Gründe der **degradierten** Sensoren eines Blocks auf **einen**
   (Priorität **auth > rate_limited > unreachable** — der am direktesten
   behebbare zuerst). Operationelle Sensoren tragen keinen Grund.
3. **Durchreichen bis zum Browser.** `health.Registry.RecordSensors` bekommt den
   Grund; `FeedSnapshot.DegradedReason` und die WebSocket-`FeedStatusMessage`
   (`degraded_reason`, `omitempty`) tragen ihn; der Admin-Endpoint
   `/api/admin/feeds/health` ebenfalls. Der ASD-Store aggregiert über mehrere
   Feeds (gleiche Priorität) und exponiert `feedDegradedReason`.
4. **Chip zeigt den Grund.** `FeedStatusChip` hängt bei `degraded` ein kurzes
   deutsches Label an (`SENSOR AUSFALL · NICHT ERREICHBAR` / `· AUTH-FEHLER` /
   `· RATENLIMIT`) und trägt einen erklärenden `title`-Tooltip. Ohne bekannten
   Grund bleibt es beim bisherigen `SENSOR AUSFALL`.

## Begründung

- **Operativer Nutzen (#197).** Der Lotse/Betreiber sieht sofort, ob ein
  Credential-Problem (auth) oder ein Netzproblem (unreachable) vorliegt — spart
  sinnloses Nachtippen und beschleunigt die Fehlerbehebung.
- **Additiv, robust.** Der RE-Parser ist eine Erweiterung des bestehenden
  längen-toleranten Skips (ADR 0019); ein fehlendes oder unbekanntes RE-Feld
  führt nie zu einem Decode-Fehler (kein Panic, Charter §7).
- **Wire-Vertrag in einer Hand.** Die Reason-Strings und die Prioritäts-Reihung
  spiegeln Fireflys `SensorReason`/`reasonPriority` 1:1 — Backend (Go) und
  Frontend (JS) teilen dieselbe Rangordnung, damit die Chip-Anzeige nie von der
  Server-Ableitung abweicht.

## Konsequenzen

- **Kein Lockstep-Zwang.** Rein additiv: läuft auch gegen einen älteren Firefly
  (kein RE-Feld ⇒ `Reason=""` ⇒ Chip wie bisher). Deploy jederzeit möglich.
- Neue byte-genaue Referenz-Vektoren (`decoder_test.go`) gegen Fireflys ICD-3.1.0-
  Dump; Store- und Chip-Tests.
- Anforderung **FR-DATA-006** erweitert; Milestone WF-CAT063 ergänzt.
- Schließt **Wayfinder #197** und den H1–H4-Bogen (Firefly #55).

## Ehrliche Grenze

Firefly liefert einen Grund nur für die **HTTP-ADS-B-Quellen** (OpenSky,
adsb_aggregator); FLARM/Radar-Ausfälle kommen ohne Grund (`Reason=""`), der Chip
zeigt dann das generische „SENSOR AUSFALL". Bei mehreren degradierten Feeds mit
verschiedenen Gründen zeigt der Chip nur den **einen** höchstpriorisierten; die
Admin-Feed-Health-Liste (`degraded_reason` je Feed) kann später die volle
Aufschlüsselung anbieten.
