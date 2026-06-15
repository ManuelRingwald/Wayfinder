# ADR 0003 — Sicherheit: Vertrauensgrenze des Empfangspfads und Browser-Rand

- **Status:** akzeptiert
- **Datum:** 2026-06-15
- **Schnittstellen-relevant:** nein (keine Änderung am CAT062-Draht-Vertrag;
  betrifft Netz-/Betriebs-Annahmen am Empfangspfad und den Browser-Rand von
  Wayfinder)

## Kontext

Wayfinder ist das produktive ASD und hat **zwei Sicherheits-Ränder**, die bisher
nur in CLAUDE.md §7 als Prinzip benannt, aber nicht entschieden waren:

1. **Empfangspfad (Firefly → Wayfinder).** Wayfinder tritt der
   CAT062-Multicast-Gruppe bei (`pkg/receiver`, Default `239.255.0.62:8600`).
   UDP-Multicast hat **keine eingebaute Authentifizierung/Integrität/
   Verschlüsselung** — dieselbe Lage, die Firefly in **ADR 0017** für die
   Sende-Seite analysiert hat. Ein Angreifer im selben Segment könnte mitlesen
   oder gefälschte Datenblöcke einspeisen (Phantom-Tracks, verfälschte
   Positionen, oder seit ICD 2.2.0/ADR 0016 ein **gefälschtes TSE-Bit**, das
   Wayfinder einen echten Track sofort aus dem Bild entfernen lässt).
2. **Browser-Rand (Wayfinder → Lotsen-Browser).** Wayfinder verteilt die
   decodierte Luftlage an Browser. Heutiger Stand (`cmd/wayfinder/main.go`,
   `pkg/ws/handler.go`):
   - Reines **HTTP** auf `:8081` (kein TLS) für `/`, `/ws`,
     `/api/map-config`.
   - **Keine Authentifizierung** auf irgendeinem dieser Pfade.
   - WebSocket-Upgrade mit `CheckOrigin` → `true` (akzeptiert **jede** Herkunft;
     der Code trägt bereits den TODO „In production, restrict to known hosts").
   - `/ws` ist derzeit **read-only** (keine Client-Kommandos), d. h. der akute
     Browser-seitige Risikofall ist **Vertraulichkeit** (jeder, der die Seite
     erreicht, sieht das operative Lagebild) sowie **Cross-Site-WebSocket-
     Hijacking** (CSWSH) durch den fehlenden Origin-Check.

Das ASD-Bild ist **nicht öffentlich** (CLAUDE.md §7). Beide Ränder brauchen eine
dokumentierte, bewusst gewählte Vertrauensgrenze.

## Entscheidung

### 1. Empfangspfad: Vertrauensgrenze auf der Netzwerk-Schicht (Spiegel zu ADR 0017)

Die Vertrauensgrenze des CAT062-Empfangspfads liegt — exakt wie auf Fireflys
Sende-Seite (ADR 0017) — auf der **Netzwerk-Schicht**:

- Empfang erfolgt ausschließlich in einem **dedizierten, abgeschotteten
  Netzsegment** (eigenes VLAN/Segment), das nur Firefly-Sender und autorisierte
  ASD-Empfänger enthält.
- **Kein anwendungsseitiges Entschlüsseln/Signatur-Prüfen von CAT062** — der
  selbstbeschreibende, standardkonforme Draht-Vertrag (Fireflys ADR 0006) bleibt
  unangetastet; ein Eigenbau-Wrapper ist ausgeschlossen.
- Der **robuste Decoder** (CLAUDE.md §7: Längenprüfung, fehlerhafte Records
  verwerfen statt abstürzen) bleibt die anwendungsseitige Schutzschicht gegen
  fehlerhafte/feindliche Datagramme — er verhindert Abstürze, ersetzt aber
  **nicht** die Netz-Isolation gegen Injektion/Mitlesen.
- **Keine Code-Änderung** in diesem Schritt; die Entscheidung macht die
  Betriebs-Annahme explizit.

### 2. Browser-Rand: TLS + Auth primär am Reverse-Proxy, fail-closed-fähig in Wayfinder

- **Primärer Mechanismus: TLS-Terminierung und Authentifizierung am vorgelagerten
  Reverse-Proxy/Ingress** (z. B. Kubernetes-Ingress mit TLS + `oauth2-proxy`/
  OIDC oder mTLS). Das ist cloud-native Standard, hält das sicherheitsrelevante
  ASD frei von Krypto-/Auth-Eigenbau und überlässt die Identitäts-Anbindung der
  jeweiligen Betriebsumgebung.
- **Ergänzend in Wayfinder selbst** (Defense-in-Depth, **fail-closed**, additiv
  — Umsetzung in Häppchen 1.3):
  - **Strikter Origin-Check** auf `/ws` gegen CSWSH: konfigurierbare Allowlist
    (`WAYFINDER_ALLOWED_ORIGINS`). Default leer ⇒ Upgrade nur same-origin/ohne
    fremden Origin; pauschales `return true` entfällt.
  - **Optionale Token-Prüfung** (Shared-Secret/Bearer) als Middleware vor `/`,
    `/ws`, `/api/map-config` (12-Factor, `WAYFINDER_AUTH_TOKEN`). Default „aus"
    **mit deutlicher Warn-Log-Zeile**, damit das Standard-Deployment hinter dem
    Proxy läuft, Wayfinder aber auch standalone absicherbar ist.
  - **Optionales TLS** direkt in Wayfinder (`WAYFINDER_TLS_CERT`/
    `WAYFINDER_TLS_KEY`) für Deployments ohne vorgelagerten Proxy.
- **Health-/Readiness-Probes (`:8080`) bleiben bewusst unauthentifiziert** —
  separater Port, von Kubernetes/kubelet erreichbar, liefert **keine** Lagedaten
  (nur `{"status":...}`/Blockzähler).

## Begründung

- **Konsistenz mit Firefly (ADR 0017).** Beide Enden derselben Multicast-Leitung
  tragen dieselbe Vertrauensgrenze auf derselben Schicht — keine widersprüchliche
  halbe Krypto auf nur einer Seite.
- **Kein Krypto-/Auth-Eigenbau im ASD.** Authentifizierung am erprobten Proxy
  (OIDC/mTLS) ist auditierbarer und betriebsüblicher als handgeschriebene Auth in
  einem sicherheitsrelevanten Anzeige-System; Wayfinder bleibt schlank und
  analysierbar (Zertifizierungs-Argument, CLAUDE.md §7).
- **Fail-closed statt fail-open.** Der heutige `CheckOrigin → true` ist
  fail-open; der strikte Origin-Check schließt CSWSH unabhängig vom Proxy. Die
  optionalen In-App-Maßnahmen stellen sicher, dass eine *Fehlkonfiguration des
  Proxys* nicht automatisch ein offenes ASD bedeutet.
- **12-Factor/anbieter-neutral.** Alle Schalter über Env-Vars, Defaults
  dokumentiert; kein Zwang zu einem bestimmten Ingress/IdP.

### Verworfene Alternativen

- **Auth/TLS ausschließlich in Wayfinder** (kein Proxy-Pfad): mehr
  sicherheitsrelevanter Eigenbau, schlechter auditierbar, bindet die
  IdP-Anbindung in den ASD-Code. Verworfen als Primärweg — bleibt aber als
  optionaler Standalone-Pfad erhalten.
- **Gar keine In-App-Maßnahme, alles dem Proxy überlassen:** fail-open bei
  Proxy-Fehlkonfiguration, und CSWSH wäre ungeschützt. Verworfen.
- **App-Layer-Krypto/Signatur auf CAT062 am Empfang:** bricht den Draht-Vertrag
  (ADR 0006), spiegelbildlich zu ADR 0017 ausgeschlossen.

## Konsequenzen

- **Neue Anforderung NFR-SEC-001** im Register (`docs/requirements/`):
  Empfangspfad-Netz-Isolation (dokumentiert) + Browser-Rand-Absicherung (TLS/
  Auth/Origin-Check), mit Verweis auf diese ADR. Der Empfangspfad-Teil hat Status
  „dokumentiert" (Deployment-Sache); der Browser-Rand-Teil wird in Häppchen 1.3
  implementiert und testbar.
- **Häppchen 1.3** setzt den Browser-Rand-Teil um: strikter konfigurierbarer
  Origin-Check, optionale Token-Middleware (fail-closed, Default-aus mit
  Warn-Log), optionales TLS, jeweils mit Tests und Register-Eintrag.
- **Deployment-Doku** (README/Betriebshinweise) erhält bei nächster Gelegenheit
  den Hinweis auf das benötigte isolierte Segment **und** den empfohlenen
  Proxy-Pfad (TLS+Auth) vor `:8081`.
- **Schließt das transformierte ehem. Issue #7** (Auth auf `/ws`): die
  Sicherheitsfrage ist jetzt beidseitig entschieden — Netz-Isolation für den
  Multicast-Pfad (hier + ADR 0017) und Browser-Rand für die Verteilung an Lotsen.

## Ehrliche Grenze

- Diese ADR **garantiert nichts**, wenn die Netz-Isolation in einer konkreten
  Umgebung nicht korrekt umgesetzt ist (Empfangspfad) bzw. der Proxy fehlerhaft
  konfiguriert wird — sie macht Annahmen explizit und baut In-App-Fallbacks, ist
  aber kein Ersatz für Netzwerk-Audits/Pentests der Zielumgebung.
- **Autorisierung/Rollen** (welcher Lotse darf was) sind **nicht** Teil dieser
  Entscheidung — hier geht es um Authentifizierung und Transport-Schutz am Rand,
  nicht um Rechte-Modelle. Solange `/ws` read-only ist, ist das ausreichend; ein
  künftiger Schreib-/Kommando-Pfad bräuchte eine eigene Autorisierungs-ADR.
- **Last-/DoS-Schutz** gegen einen Flood gefälschter Datagramme im Segment ist —
  wie in ADR 0017 — durch reine Netz-Isolation nicht vollständig adressiert und
  bleibt ein Punkt der „Betriebs-Härtung"-Roadmap.
