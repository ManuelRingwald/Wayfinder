# ADR 0017 — Connected-by-default: Wayfinder ist eine Informations-Plattform, kein Steuerungssystem

- **Status:** akzeptiert
- **Datum:** 2026-07-03
- **Schnittstellen-relevant:** nein (kein CAT062-Vertrag betroffen; betrifft die
  Betriebs-/Deployment-Prämisse und die Defaults externer Kontext-Quellen)

## Kontext

Wayfinder ist das ASD — es **stellt die Luftlage und begleitende
Kontext-Informationen dar**. Es ist ausdrücklich ein **System zur
Informationsbereitstellung/Lagedarstellung**, **kein System zur Steuerung von
Flugbewegungen** (keine Freigaben, keine Staffelung, keine Kontrollanweisungen; die
System-Tracks werden von Firefly berechnet und hier nur angezeigt). Aus dieser
Zweckbestimmung folgt eine Betriebs-Prämisse, die bisher **implizit und
inkonsistent** war:

- Frühere Entscheidungen (ADR 0004 OpenAIP, ADR 0016 Wetter) machten externe
  Kontext-Quellen **opt-in**, u.a. mit „luftgespalten/offline"-Begründung.
- Gleichzeitig lädt die **Basiskarte** seit jeher externe Kartenkacheln
  (OSM/CARTO) — das Produkt war also nie wirklich abgeschottet; die Haltung war
  **widersprüchlich**.
- Daraus entstand **Betreiber-Reibung**: eine pro Mandant freigeschaltete
  Overlay-Funktion wirkte „kaputt", weil zusätzlich eine Deployment-Env gesetzt
  sein musste.

## Entscheidung

1. **Zielbild: verbundener Betrieb.** Wayfinder wird für den **vernetzten** Betrieb
   gebaut; ein abgeschottetes/offline Netz ist **keine** Design-Randbedingung mehr.
   Begründung: Wayfinder liefert **Informationen zur Lageunterstützung**, nicht
   flugsicherheits-kritische Steuerung — die für Steuerungs-/Kontrollsysteme
   typische strikte Netz-Isolation ist hier nicht die maßgebliche Randbedingung.
2. **Externe Kontext-Quellen sind default-an.** Basiskarte (schon immer),
   DWD-Wetter/Warnungen, NOAA-QNH und OpenAIP-Aeronautik sind **standardmäßig
   aktiv**. Abschaltbar bleibt jede Quelle über einen **expliziten**
   `WAYFINDER_..._ENABLED=false`-Schalter (klare Opt-out-Absicht statt „URL
   leeren").
3. **NFR-SEC-001 bleibt unberührt.** Der **CAT062/065/063-Multicast-Eingang**
   bleibt auf einem isolierten Segment/VLAN — das ist die **Feed-Datenebene**, kein
   Internet. Die Connected-by-default-Prämisse betrifft **ausschließlich ausgehende
   Kontext-Quellen**, nicht den Feed-Eingang.
4. **Best-effort und robuster Decoder bleiben.** Alle externen Quellen bleiben
   best-effort (nie `/ready` blockieren, Last-Good bzw. leer bei Ausfall,
   Größenlimits, tolerantes Parsen, kein Panic auf Eingabe). Der **Default kippt —
   die Schutz-Eigenschaften nicht.**
5. **Egress als dokumentierte Betriebsvoraussetzung.** Die ausgehenden Ziele
   (`maps.dwd.de`, `aviationweather.gov`, `api.core.openaip.net`, Karten-Tile-CDNs;
   jeweils HTTPS/443) werden als **First-Class-Netzwerk-Anforderung** dokumentiert
   (INSTALLATION/BETRIEB).
6. **Self-hosted Fonts/Glyphen bleiben.** Die Begründung wechselt von „offline/
   luftgespalten" (ADR 0015) auf **Robustheit / keine Laufzeit-CDN-Abhängigkeit**
   (weniger externe Fehlerquellen, schneller Erststart). Kein Rückbau.
7. **Per-Tenant-Entitlements bleiben opt-in (default-deny).** „Quelle default-an" ≠
   „jeder Mandant sieht alles": die **Sichtbarkeit** steuert weiter das
   Feature-Entitlement pro Mandant. Das sind **zwei getrennte Ebenen**
   (Quell-Konfiguration vs. mandanten-bezogene Freischaltung).

## Amendments an bestehenden ADRs

- **ADR 0004 (OpenAIP):** Die „kein Schlüssel ⇒ Feature still aus"-Opt-in-Haltung
  wird durch dieses ADR abgelöst. OpenAIP wird auf ein **persistentes
  On-Demand-Modell** mit **global über die Admin-UI setzbarem Schlüssel**
  umgestellt (Details in den Folge-Häppchen). Die Schutz-Eigenschaften (Schlüssel
  server-seitig, best-effort, Größenlimits) bleiben.
- **ADR 0016 (Wetter):** Die „Feature still aus ohne URL"-Opt-in-Haltung wird zu
  **default-an mit `..._ENABLED`-Opt-out** (Entscheidung: expliziter Schalter).
  NFR-SEC-005 (robuster Wetter-Decoder / Vertrauensgrenze) bleibt vollständig
  gültig — nur der Default kippt.

## Begründung

- **Zweck-Ehrlichkeit:** Ein Informations-/Darstellungs-System hat andere
  Betriebs-Randbedingungen als ein Steuerungssystem; „connected" ist für
  Kontext-Daten die realistische und einzige konsistente Haltung (die Karte war es
  ohnehin immer).
- **Betreibbarkeit:** kein Env-Gefummel für Standard-Overlays; der Admin steuert
  die Sichtbarkeit pro Mandant über Entitlements.
- **Konsistenz:** behebt den Widerspruch (Kartenkacheln extern, Rest opt-in).

### Verworfene Alternativen

- **Weiter opt-in mit Offline-Default:** widerspricht der Zweckbestimmung und der
  bereits extern ladenden Basiskarte; erzeugt Betreiber-Reibung. Verworfen.
- **„URL leer = aus" als Opt-out:** unklar/fragil; ein expliziter
  `..._ENABLED`-Schalter macht die Abschalt-Absicht selbsterklärend.
- **Mitgelieferter OpenAIP-Schlüssel:** Nutzungsbedingungen-/Rate-Limit-Risiko auf
  einem geteilten Schlüssel. Verworfen — der Admin setzt einen globalen Schlüssel
  über die UI.

## Konsequenzen

- **Folge-Häppchen:** DWD default-an + `..._ENABLED` (Häppchen 2); QNH default-an +
  per-Mandant-Flugplatz (Häppchen 3); OpenAIP persistentes On-Demand + globaler
  Schlüssel via UI (AERO-1/AERO-2); optional AIRAC-Kalender + Änderungs-Diff
  (AERO-3).
- **Neue Doku:** Netzwerk-Anforderungen (ausgehende Verbindungen) in
  INSTALLATION/BETRIEB.
- **Register:** `NFR-OPS-005` (Connected-by-default-Posture) neu; die per-Feature-
  Umformulierungen („still aus" → „default-an mit Opt-out") werden **im jeweiligen
  Code-Häppchen** nachgezogen, damit das Register den echten Code-Stand spiegelt.

## Ehrliche Grenze

- Wayfinder bleibt ein **System zur Informationsbereitstellung/Lagedarstellung**,
  **kein zertifiziertes System zur Steuerung von Flugbewegungen**. Die
  dargestellten externen Kontext-Daten (Karte, Wetter, Warnungen, QNH, Aeronautik)
  sind **Orientierungs-Information**, keine zertifizierten aeronautischen/
  meteorologischen Datensätze (Fortführung der „ehrlichen Grenze" aus ADR 0004/
  0016).
- Ein Betreiber mit besonderen Isolations-Anforderungen kann jede Kontext-Quelle
  per `..._ENABLED=false` abschalten — dann entfällt nur die jeweilige
  Kontext-Anzeige; der ASD-Kern (CAT062 → Karte) läuft unverändert weiter.
