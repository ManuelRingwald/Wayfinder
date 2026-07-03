# ADR 0004 — Aeronautik-Datenquelle: Live-OpenAIP über Backend-Proxy + Cache

- **Status:** akzeptiert
- **Datum:** 2026-06-15
- **Schnittstellen-relevant:** nein (keine Änderung am CAT062-Draht-Vertrag mit
  Firefly; betrifft eine **neue, eigenständige** Datenquelle für die
  Kartendarstellung)
- **Nachtrag (ADR 0017):** Die „kein Schlüssel ⇒ Feature still aus"-Opt-in-Haltung
  (u.a. Offline-Begründung) wird durch **ADR 0017 (Connected-by-default)** abgelöst;
  OpenAIP wird auf ein **persistentes On-Demand-Modell** mit **global über die
  Admin-UI setzbarem Schlüssel** umgestellt (Folge-Häppchen). Die
  Schutz-Eigenschaften (Schlüssel server-seitig, best-effort, Größenlimits) bleiben.

## Kontext

ASD-003 (Roadmap Paket #13) fügt dem ASD die luftfahrt-relevanten
Karten-Layer hinzu: **Luftraumstrukturen** (Sektor-/FIR-Grenzen),
**Waypoints** und **VOR/NDB-Beacons**. Diese Daten gehören **nicht** zum
CAT062-Track-Strom von Firefly — sie sind statischer/semi-statischer
Luftfahrt-Kontext (AIRAC-Zyklus, alle 28 Tage).

Als Datenquelle wurde **Live-OpenAIP** (openaip.net) gewählt (gegenüber
gebündelten statischen GeoJSON oder einer reinen Datei-Konfiguration). OpenAIP
bietet eine REST-API für Airspaces, Navaids und weitere Luftfahrtdaten, die
einen **API-Key** erfordert.

Daraus ergeben sich Entwurfsfragen, die eine bewusste Weichenstellung verlangen:

1. **Wo läuft der Abruf** — im Browser (jeder Client direkt gegen OpenAIP) oder
   im **Go-Backend** (ein Egress-Punkt)?
2. **Verfügbarkeit** — das ASD ist sicherheitsrelevant; was passiert, wenn
   OpenAIP langsam/nicht erreichbar ist?
3. **Geheimnis-Handhabung** — der API-Key darf nicht an jeden Browser
   ausgeliefert werden.
4. **Vertrauen in externe Daten** — CLAUDE.md §7: „Niemals einem Datagramm
   vertrauen" gilt sinngemäß auch für eine externe HTTP-Quelle.

## Entscheidung

### 1. Abruf im Backend (Proxy + Cache), nicht im Browser

Wayfinder ruft OpenAIP **server-seitig** ab, cached die Ergebnisse und liefert
sie dem Frontend über **interne Endpoints** als GeoJSON aus
(`/api/airspace`, `/api/navaids`, `/api/waypoints`).

- **Ein Egress-Punkt** statt N Browser, die OpenAIP hämmern.
- Der **API-Key bleibt server-seitig** (`WAYFINDER_OPENAIP_API_KEY`, 12-Factor-
  Secret) und erreicht den Browser nie.
- Das Frontend spricht nur denselben Origin an (kein zusätzlicher CORS-/
  Mixed-Content-Rand).

### 2. Graceful Degradation — das ASD hängt nicht an OpenAIP

- Die Aeronautik-Layer sind **best-effort**. Der **Track-Pfad
  (CAT062 → WebSocket → Karte) ist davon vollständig unabhängig** und rendert
  immer.
- Ein OpenAIP-Ausfall darf **`/ready` nicht umkippen** und den **Start nicht
  blockieren**. Bei Fehler wird der **letzte gute Cache** weiter ausgeliefert;
  ist noch kein Cache vorhanden, liefert der Endpoint eine **leere
  FeatureCollection** (HTTP 200) statt eines Fehlers — die Karte zeigt dann
  einfach keine Aeronautik-Overlays.
- **Periodischer Refresh** (Default 24 h, AIRAC-Takt;
  `WAYFINDER_OPENAIP_REFRESH`) plus einmalig beim Start, **nicht-blockierend**.

### 3. Robuster, misstrauischer Konsument

- **Timeouts** auf alle OpenAIP-Requests; **Antwort-Größen begrenzen**
  (Schutz gegen Speicher-Exhaustion).
- **Validierung/Normalisierung** der OpenAIP-Antwort in saubere GeoJSON-
  FeatureCollections; fehlerhafte Einzel-Features werden verworfen statt den
  ganzen Abruf scheitern zu lassen.
- **Observability:** Erfolg/Fehler-Zähler und Cache-Alter als Metriken
  (`wayfinder_openaip_*`), Fehler als strukturierte Logs.

### 4. Kein Key konfiguriert ⇒ Feature still aus

Ohne `WAYFINDER_OPENAIP_API_KEY` bleibt die OpenAIP-Anbindung **deaktiviert**
(Warn-Log beim Start); die Endpoints liefern leere FeatureCollections, das ASD
läuft normal weiter. So bleibt die Demo ohne Pflicht-Secret lauffähig.

## Begründung

- **Sicherheit/Geheimnis:** Backend-Proxy hält den Key server-seitig und gibt
  einen einzigen, kontrollierbaren Egress — auditierbar, anbieter-neutral.
- **Verfügbarkeit/Sicherheit des ASD:** Eine externe Abhängigkeit darf die
  Kernfunktion (Tracks anzeigen) eines sicherheitsrelevanten Lagebilds nie
  gefährden. Best-effort + Last-Good-Cache + nicht-blockierender Start setzen
  das durch.
- **Misstrauen gegenüber externen Daten** ist die konsequente Fortschreibung
  des „robusten Decoder"-Prinzips (CLAUDE.md §7) auf eine HTTP-Quelle.
- **Cloud-native/12-Factor:** Key, Refresh-Intervall und Region über Env-Vars;
  Caching im Prozess, Zustand explizit.

### Verworfene Alternativen

- **Browser ruft OpenAIP direkt:** Key im Client exponiert, N-faches Abrufen,
  CORS-/Rate-Limit-Probleme, schwerer abzusichern. Verworfen.
- **Gebündelte statische GeoJSON (offline):** einfacher und ohne externe
  Abhängigkeit, aber veraltet ohne AIRAC-Pflege und entspricht nicht der
  gewählten „Live"-Anforderung. Bleibt als möglicher Offline-Fallback denkbar.
- **Pflicht-Datei/-URL ohne Default-Verhalten:** ließe die Demo ohne Daten
  leerlaufen und blockiert ggf. den Start. Verworfen zugunsten „still aus".
- **OpenAIP als Readiness-Voraussetzung:** würde das ASD an eine externe Quelle
  koppeln (fail-closed an der falschen Stelle). Verworfen.

## Konsequenzen

- **Neues Go-Paket** (`pkg/aeronautical` o. ä.): OpenAIP-Client, Transform nach
  GeoJSON, In-Memory-Cache mit Last-Good-Fallback, periodischer Refresh.
- **Neue interne Endpoints** `/api/airspace`, `/api/navaids`, `/api/waypoints`.
- **Neue Konfiguration:** `WAYFINDER_OPENAIP_API_KEY`,
  `WAYFINDER_OPENAIP_REFRESH` (Default 24 h), `WAYFINDER_OPENAIP_RADIUS_KM`
  (Region als bbox um den Map-Mittelpunkt, Default 250 km).
- **Neue Anforderungen** im Register: `FR-MAP-002` (Airspace),
  `FR-MAP-003` (Navaids/Waypoints), `NFR-OPS-004` (graceful degradation,
  nicht-blockierend), `NFR-SEC-002` (Key server-seitig, externe
  Vertrauens-/Größengrenze), `NFR-OBS-004` (OpenAIP-Metriken).
- **Roadmap:** ASD-003 überlappt damit das ehemals Firefly-seitig notierte
  Paket #9 („Live-OpenAIP-Integration"); die Wayfinder-seitige Integration wird
  hier verortet und #9 entsprechend referenziert.

## Ehrliche Grenze

- OpenAIP-Daten sind **Orientierungs-/Kontext-Information** für die Anzeige,
  **kein zertifizierter AIS/AIP-Datensatz**. Für einen realen operativen
  Einsatz wäre eine zertifizierte Luftfahrt-Datenquelle und ein
  Daten-Qualitäts-/Aktualitäts-Nachweis nötig — das ist nicht Teil dieses
  Code-Projekts (analog zur „ehrlichen Grenze" der Zertifizierungs-Fähigkeit,
  CLAUDE.md §7).
- Die Entscheidung sichert **Verfügbarkeit des ASD-Kerns** gegen OpenAIP-Ausfall,
  nicht die **Aktualität** der Aeronautik-Layer bei längerem Ausfall (dann
  altert der Cache sichtbar — Cache-Alter-Metrik macht das beobachtbar).
