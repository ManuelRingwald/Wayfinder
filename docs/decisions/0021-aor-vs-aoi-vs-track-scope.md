# ADR 0021 — Geografie-Begriffsmodell: AoR vs. AoI/Track-Scope vs. Kartenrahmen

- **Status:** **AKZEPTIERT** ✅ (2026-07-06).
- **Datum:** 2026-07-06
- **Schnittstellen-relevant:** **nein** — der **CAT062/063/065-Draht-Vertrag mit
  Firefly bleibt unverändert**. Dieser ADR ist **rein konzeptionell/
  dokumentarisch**: er benennt und trennt bereits vorhandene geografische
  Konzepte; **kein Code, keine neuen Env-Variablen** in diesem Schritt.
- **Bezug:** **ADR 0004** (Aeronautik-Datenquelle Live-OpenAIP — Herkunft der
  Luftraum-Overlays und deren „ehrliche Grenze"), **ADR 0012** (Mandanten-
  Tracker-Orchestrierung — „grobe äußere Coverage-BBox in Firefly vs.
  autoritative, präzise AOI in Wayfinder"), **WF2-21.2** (server-seitiger
  AOI/FL-View-Filter), **ASD-012** (Range Rings um den Karten-Mittelpunkt),
  **ASD-005** (FL-Filter; benennt bereits den „Verantwortlichkeitsbereich"),
  **M1.4.a** (Map-Center/Zoom über Env). Firefly-seitig: **Fireflys ADR 0021**
  (System-Referenzpunkt = Projektions-Ursprung, **keine** Ansicht),
  **Fireflys ADR 0023/0030** (Quell-Eingangs-BBox). Fachliche Grundlage:
  EUROCONTROL/SKYbrary **Area of Responsibility (AoR)** bzw. **Area of Interest
  (AoI)**.
- **Anforderungs-Register:** **keine neue Anforderung in diesem Schritt** — der
  ADR schafft nur die begriffliche Grundlage. Die funktionale Anforderung
  („flughafengebundene AoR-Overlay-Ebene") entsteht **mit dem Folge-Häppchen**
  und wird dort registriert.

> ℹ️ **Auslöser:** Betreiber-Frage (2026-07-06). Wayfinder zeigt dem Nutzer heute
> einen **„Radius von X NM"** (bzw. eine daraus abgeleitete BBox). In der realen
> Flugsicherung hat ein Flughafen-ANSP aber **keinen Radius**, sondern
> **definierte Gebiete** (Verantwortungsbereiche), die die Lotsen im Blick
> behalten. Wunsch: die **Gebiete** flughafenbezogen anzeigen — und der „Radius"
> wird zu einer Konfiguration, **die nur die Tracks betrifft**. Dieser ADR hält
> das Begriffsmodell fest, bevor Code entsteht.

---

## Kontext

### Fachlicher Hintergrund (Grundwahrheit)

Ein Flughafen-ANSP ist für **definierte 3-D-Luftraum-Volumina** zuständig, nicht
für einen Radius:

- **Tower (TWR)** verantwortet die **CTR** (Control Zone, ab Boden) und ggf. die
  **ATZ**.
- **Approach (APP)** verantwortet die darüberliegende **TMA/CTA**.
- Diese Volumina sind im AIP als **Koordinaten-Polygone** (teils mit Kreisbögen/
  Radius-Segmenten) plus **Vertikalgrenzen** und **Luftraumklasse** publiziert.
  Ein Radius kann Teil der *Beschreibung* sein (z. B. „5 NM um den ARP"),
  ersetzt aber nicht das Polygon. Nur die **ATZ** ist vielerorts ein echter
  Zylinder.

Entscheidend ist die fachliche Trennung **„sehen ≠ besitzen"**:

- **Area of Responsibility (AoR):** das Volumen, das der Lotse **kontrolliert**
  (CTR/TMA).
- **Area of Interest (AoI):** das Volumen, das sein **System verfolgt** — es
  **schließt die AoR ein, reicht aber bewusst darüber hinaus** (Größenordnung
  100–300 NM über die AoR-Grenze), damit anfliegender Verkehr **früh** sichtbar
  ist. Darüber liegt noch die rohe Sensor-Reichweite.

### Ist-Zustand (am Code geerdet)

Das Wort „Radius" bzw. der Begriff „geografischer Ausschnitt" erledigt in
Wayfinder heute **mehrere, unterschiedliche Aufgaben** — sauber getrennt im
Code, aber **begrifflich vermischt**:

| Konzept | Rolle | Code-Heimat | Filtert Tracks? |
|---------|-------|-------------|-----------------|
| **AOI-BBox** (pro Mandant/View) | einziger geografischer **Track-Filter**, server-seitig; Datensparsamkeits-/Isolationsgrenze (WF2-21.2) | `pkg/store/view_configs.go` (`BBox`), `pkg/broadcast/broadcast.go` (`ViewFilter.AOI`) | **ja** |
| **Coverage-BBox** (Firefly) | grobe äußere Grenze, *welche* Flugzeuge überhaupt getrackt werden; von Wayfinder aus AOI + Marge abgeleitet (ADR 0012) | Orchestrierung → `FIREFLY_COVERAGE_BBOX` | ja (eingangsseitig) |
| **OpenAIP-Abfrage-Radius** | Fenster, in dem Luftraum-/Navaid-/Flughafen-Referenzdaten geholt werden (ADR 0004) | `WAYFINDER_OPENAIP_RADIUS_KM`, `pkg/aeronautical` | **nein** |
| **Map-Center/Zoom** | Kamera (Anfangs-Ausschnitt) | `WAYFINDER_MAP_CENTER_*`, `WAYFINDER_MAP_ZOOM`, View-Center/Zoom | **nein** |
| **Range Rings** (NM) | Deko: konzentrische Distanz-Kreise um den Karten-Mittelpunkt (ASD-012) | `frontend/src/map/rangerings.js` | **nein** |

Zwei Beobachtungen daraus:

1. **Die vom Betreiber gewünschte Entkopplung existiert bereits teilweise.** Der
   Track-Filter (AOI-BBox) ist von Kamera und Overlays getrennt. **Luftraum-
   Polygone (u. a. CTR/TMA) werden bereits aus OpenAIP gerendert** (ADR 0004,
   ASD-003) — bislang aber *generisch* („alle Lufträume im Abfragefenster"),
   nicht an eine Flughafen-/ANSP-Identität gebunden und **ohne** Unterscheidung
   „mein AoR" vs. „Kontext".
2. **Eine Inkonsistenz bleibt offen:** ADR 0012 **beschreibt** die AOI als
   „Kreis + Radius + FL, live verstellbar", der Code **implementiert** sie als
   `BBox`. Eine „Radius X NM → Track-Filter-BBox"-Umrechnung gibt es heute
   **nicht** (der einzige `center+radius→bbox`-Helfer füttert nur OpenAIP, nicht
   den Track-Filter).

### Spannungsfeld

Ohne eine festgelegte Begrifflichkeit droht die fachlich **falsche**
Gleichsetzung **„Track-Scope = Verantwortungsbereich"**. Das widerspricht dem
AoI-Prinzip (der Lotse sieht mehr, als er kontrolliert) und würde
Fehl-Konfiguration begünstigen (Verkehr würde erst an der AoR-Grenze sichtbar).
Bevor die flughafengebundene AoR-Darstellung gebaut wird, müssen die Ebenen
sauber benannt und getrennt sein.

---

## Entscheidung

Wayfinder führt **drei orthogonale geografische Ebenen** als verbindliche
Begriffe. Jede ist **unabhängig** konfigurierbar; keine impliziert eine andere.

### 1. Track-Scope (= Area of Interest, AoI) — *„was der Lotse sieht"*

Der **Daten-Ausschnitt**. Bewusst **größer** als der Verantwortungsbereich
(AoI-Prinzip). Realisiert durch die pro-Mandant **`view_configs.AOI`** (BBox) +
FL-Band (server-seitiger View-Filter, WF2-21.2) und, grob vorgelagert, Fireflys
**`FIREFLY_COVERAGE_BBOX`** (AOI + Marge, ADR 0012). **Dies ist der „Radius, der
nur die Tracks betrifft"** aus der Auslöser-Frage. Der Track-Scope ist **kein**
Verantwortungsbereich.

*Optionaler Ausbau (Folge-Häppchen):* Eingabe als „Zentrum + Radius NM", intern
in die AOI-BBox umgerechnet — schließt die ADR-0012-Wortlaut-Lücke.

### 2. Verantwortungsbereich (= Area of Responsibility, AoR) — *„was der Lotse kontrolliert"*

Die kontrollierten **Volumina** (CTR/TMA/CTA) als **hervorgehobene
Darstellungs-Ebene**. **Kein** Track-Filter. **Datenquelle: OpenAIP** (Anschluss
an ADR 0004), da aktuell die einzige Quelle und der Betrieb **noch nicht
kommerziell** ist. An eine **Flughafen-/ANSP-Identität** gebunden (Feature im
Folge-Häppchen). Autoritative Quellen (Betreiber-Polygon aus LoA/AIP, oder AIXM)
sind eine **spätere** Ausbaustufe.

### 3. Kartenrahmen & Deko — *„wie der Lotse draufschaut"*

Rein visuell: Map-Center/Zoom (M1.4.a), **Range Rings** (ASD-012) als
Distanz-Lineal, FL-Anzeigefilter (ASD-005). Betrifft weder Tracks noch AoR.

### Orthogonalitäts-Prinzip

Der „Radius" gehört **ausschließlich zu Ebene 1**. Ebene 2 ist Polygon-Geometrie,
Ebene 3 ist Darstellung. Kein Codepfad darf diese Ebenen wieder verschmelzen
(z. B. den AOI-Track-Filter aus einer AoR-Polygonfläche ableiten oder umgekehrt).

---

## Worked Example — Flughafen EDDH (Hamburg)

ARP Hamburg ≈ **53.6304 N, 9.9882 E**. „EDDH korrekt konfigurieren" heißt: die
drei Ebenen bewusst getrennt setzen (Werte **beispielhaft**, nicht autoritativ):

| Ebene | Für EDDH | Stellschraube | Status |
|-------|----------|---------------|--------|
| **1 Track-Scope/AoI** | großzügiges Sichtfeld um Hamburg, z. B. BBox `52.8–54.4 N, 8.6–11.4 E` (≈ 90 NM Kante), FL `0–195` | `view_configs.AOI` + FL-Band → abgeleitet `FIREFLY_COVERAGE_BBOX` | **heute vorhanden** |
| **2 AoR** | Hamburg **CTR** + Hamburg **TMA**, hervorgehoben | Auswahl der zugehörigen OpenAIP-Lufträume, an EDDH gebunden | **neu (Folge-Häppchen)** |
| **3 Kartenrahmen** | Center = ARP, Zoom ≈ 9; Range Rings 10 NM; FL-Filter-Default | `WAYFINDER_MAP_CENTER_*`/Zoom, `rangerings.js`, FL-Filter | **heute vorhanden** |

Merksatz: **Ebene 1 = Sichtfeld (großzügig), Ebene 2 = Zuständigkeitsgebiet
(hervorgehoben, kleiner), Ebene 3 = Darstellung.**

---

## Begründung

- **Fachliche Korrektheit:** Bildet die reale ATC-Trennung AoR/AoI ab
  („sehen ≠ besitzen", SKYbrary). Der Track-Scope *soll* größer sein als die AoR.
- **Am Bestand geerdet:** Die Architektur trennt AOI-Track-Filter und
  Darstellung bereits (ADR 0012, WF2-21.2); dieser ADR macht das explizit und
  benennbar, statt eine neue Struktur zu erfinden.
- **Weniger Fehl-Konfiguration:** Ein Betreiber, der EDDH einrichtet, weiß
  danach, welche Stellschraube welche Wirkung hat (Tracks vs. Gebiete vs.
  Kamera).
- **Vorbereitung des Features:** Schafft das Vokabular für die
  flughafengebundene, hervorgehobene AoR-Overlay-Ebene.

### Verworfene Alternativen

- **Radius als primäres Modell für alles** (Kamera + Tracks + AoR gemeinsam):
  fachlich falsch (AoR ist Polygon, nicht Radius) und vermischt die Ebenen.
  Verworfen.
- **Track-Scope = AoR gleichsetzen:** widerspricht dem AoI-Prinzip; anfliegender
  Verkehr würde erst an der AoR-Grenze erscheinen. Verworfen.
- **AoR autoritativ aus AIXM/EAD jetzt:** lizenz-/zugangsaufwändig, für den
  aktuellen, nicht-kommerziellen Stand nicht nötig. **Später** denkbar; jetzt
  OpenAIP.
- **AoR als Betreiber-Polygon (LoA/AIP) jetzt:** starke, autoritative Option —
  aber OpenAIP ist derzeit die einzige integrierte Quelle. Als spätere
  Ausbaustufe vorgemerkt.

---

## Konsequenzen

- **Dieser Schritt ist rein dokumentarisch:** dieser ADR + neue Glossar-Begriffe
  (AoR, AoI/Track-Scope, CTR, TMA, CTA, ATZ, „sehen ≠ besitzen"). **Kein Code,
  keine neuen Env-Variablen**, kein Eintrag im Anforderungs-Register.
  `INSTALLATION.md`/`TECHNICAL.md` bleiben unverändert (geprüft).
- **Folge-Häppchen (separat anzukündigen, noch nicht freigegeben):**
  flughafengebundene, hervorgehobene **AoR-Overlay-Ebene** auf OpenAIP-Basis.
  Dort zu entscheiden: die **Auswahl-Semantik** (welche OpenAIP-Lufträume gelten
  als AoR — z. B. explizite Namens-/ID-Liste pro Mandant, optional per räumlichem
  Test vorbefüllt) und optional die `center+radius→BBox`-Umrechnung für die AOI.
  Erst dort entsteht die funktionale Anforderung (Register + Milestone).

## Ehrliche Grenze

- OpenAIP-Daten sind **Orientierungs-/Kontext-Information**, **kein zertifizierter
  AIP-Datensatz** (Erbe ADR 0004). Die AoR-Darstellung ist
  **Situational-Awareness-Kontext**, keine rechtsverbindliche Luftraumgrenze.
- **Lizenz:** OpenAIP ist **CC BY-NC** (nicht kommerziell) — passend zum aktuellen
  Stand, aber ein bewusster Vorbehalt für eine spätere kommerzielle Nutzung.
- **AIRAC-Drift** (28 Tage) und die 2-D-Natur von MapLibre (Vertikalgrenzen sind
  Attribute, kein 3-D-Volumen) bleiben bestehende Grenzen der Darstellung.
