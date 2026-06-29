# Wayfinder — Technische Referenz

> **Zweck:** Technische **Referenz** für Systemadministratoren, Integrations­
> partner und Entwickler. Beschreibt Architektur, Schnittstellen, Metriken,
> Konfigurationsparameter und Betriebsverhalten von Wayfinder.

> 📚 **Verwandte Dokumente:** Aufsetzen → `docs/INSTALLATION.md`; **Tagesbetrieb**
> als aufgabenorientiertes Runbook (Kontrollen, Pflege, Sicherung, Störungs­
> behebung) → `docs/BETRIEB.md` (Betriebsführungshandbuch).

---

## Inhaltsverzeichnis

1. [Systemübersicht](#1-systemübersicht)
2. [Datenfluss](#2-datenfluss)
3. [Ports und Endpunkte](#3-ports-und-endpunkte)
4. [Health- und Readiness-Probes](#4-health--und-readiness-probes)
5. [Prometheus-Metriken](#5-prometheus-metriken)
6. [Umgebungsvariablen](#6-umgebungsvariablen)
7. [Feed-Staleness-Erkennung](#7-feed-staleness-erkennung)
8. [Sicherheitsmodell](#8-sicherheitsmodell)
9. [Logging](#9-logging)
10. [Betriebsverhalten](#10-betriebsverhalten)
11. [Bekannte Einschränkungen](#11-bekannte-einschränkungen)

---

## 1. Systemübersicht

Wayfinder ist das **Air Situation Display (ASD)** — die Lagedarstellung für
den Lotsen. Es empfängt den von Firefly berechneten Systemtrack-Strom,
dekodiert ihn und stellt ihn als live-mitlaufendes, interaktives Luftlagebild
im Browser dar.

### Komponenten

```
UDP Multicast (CAT062 + CAT065)
        │
        ▼
┌────────────────────────────────────────┐
│  pkg/receiver — Multicast-Empfänger   │
│  CAT-Dispatch (0x3E → Track,           │
│                0x41 → Status)          │
└──────────────┬─────────────────────────┘
               │
       ┌───────┴────────┐
       │                │
       ▼                ▼
pkg/cat062          pkg/cat065
(Track-Decoder)     (Heartbeat-Decoder)
       │                │
       ▼                ▼
pkg/broadcast       pkg/health
(WebSocket-Hub)     (Feed-Liveness)
       │
       ▼
Browser (WebSocket JSON)
  internal/webui/static/
    app.js + Vue 3 + MapLibre GL JS
```

### Technologie-Stack

| Schicht | Technologie |
|---------|-------------|
| Backend | Go 1.23, stdlib (`net`, `net/http`, `log/slog`) |
| WebSocket | `github.com/gorilla/websocket` v1.5.3 |
| YAML-Config | `gopkg.in/yaml.v3` |
| Frontend | Vue 3, Vuetify 3 (MD3), Pinia, Vite |
| Kartenbibliothek | MapLibre GL JS |
| Frontend-Tests | Vitest |

---

## 2. Datenfluss

### 2.1 Eingang: CAT062/CAT065/CAT063 über UDP-Multicast

```
Firefly
  └─► UDP Multicast 239.255.0.62:8600
          └─► pkg/receiver.Receiver.Run()
                  ├─► CAT-Oktet 0x3E → pkg/cat062.DecodeBlock()
                  │       └─► []DecodedTrack  →  pkg/broadcast.Broadcaster.Broadcast()
                  ├─► CAT-Oktet 0x41 → pkg/cat065.DecodeStatusBlock()
                  │       └─► pkg/health.Registry.RecordHeartbeat()
                  └─► CAT-Oktet 0x3F → pkg/cat063.DecodeSensorBlock()
                          └─► pkg/health.Registry.RecordSensors()
```

**Dispatch-Logik:** Der Receiver liest ein komplettes UDP-Datagramm (max.
65 535 Byte) und prüft das erste Byte als ASTERIX-Kategorie:

- `0x3E` (62 dezimal) → CAT062-Decoder → Track-Update
- `0x41` (65 dezimal) → CAT065-Decoder → Heartbeat
- `0x3F` (63 dezimal) → CAT063-Decoder → Sensor-Status-Update
- anderes → Decode-Fehler, Zähler `wayfinder_cat062_decode_errors_total`
  erhöht, Datagramm verworfen

**Robustheit:** Fehlerhafte Datagramme (zu kurz, ungültige Länge, FSPEC
überschreitet Puffer) werden verworfen, ohne den Prozess zu beenden.
Es gibt keinen Panic auf Netzwerkeingaben.

### 2.2 Ausgang: Track-Updates an den Browser

```
pkg/broadcast.Broadcaster
    │
    ├─► WebSocket /ws  (Port 8081)
    │       └─► JSON TrackMessage  {feed_id, track_num, lat, lon, vx, vy,
    │                                flight_level_ft, callsign, mode_3a,
    │                                icao_addr, adsb_age_s,
    │                                coasting, ended, ...}
    └─► (Eviction bei vollem Send-Channel, Warn-Log)
```

Jeder verbundene Browser-Client erhält dieselben Track-Updates. Der
Broadcaster hält keine Track-History — jedes Update ist ein vollständiges
Snapshot-Frame (alle aktuell bekannten Tracks).

**ADS-B-Anteil (`adsb_age_s`, ICD 2.4.0, AP9.9):** Das Feld `adsb_age_s`
ist nur vorhanden (`omitempty`), wenn Firefly den Track zuletzt mit einem
ADS-B-Selbstbericht aktualisiert hat. Der Wert gibt das Alter dieses Updates
in Sekunden an (Auflösung 1/4 s, aus I062/290 ES-Age). Fehlt das Feld, ist
der Track ein reiner Radar-Track.

Das Frontend leitet daraus — zusammen mit `icao_addr`/`mode_3a`/`callsign` — die
**track-abgeleitete Herkunft** ab und kodiert sie als **Symbol-Form** (WF2-40).
Die **Farbe** des Symbols bleibt dabei der Track-Zustand
(confirmed/coasting/tentative/filtered):

| Symbol | Herkunft | Bedingung |
|--------|----------|-----------|
| ◆ Karo (gefüllt)    | ADS-B (kooperativ) | `adsb_age_s` vorhanden **und** ≤ 30 s (frisch) |
| ▢ Quadrat (gefüllt) | SSR / Mode S       | kein frisches ADS-B, aber `icao_addr`/`mode_3a`/Callsign |
| ○ Ring (offen)      | Primär (PSR)       | keines der obigen — reine Skin-Paint ohne ID |

Die 30-Sekunden-Frische-Schwelle (`ADSB_FRESH_THRESHOLD_S`) und die Klassifikation
liegen in `frontend/src/map/provenance.js` (`trackProvenance`, `isAdsbFresh`); die
Symbole werden in `frontend/src/map/layers.js` (`addTrackIcons`) zur Laufzeit
gezeichnet. Das **Track-Detail-Panel** zeigt die Herkunft im Klartext, die
**Sidebar** eine Form-Legende. **Ehrliche Grenze:** track-abgeleitet, keine
zertifizierte Per-Plot-Provenienz — CAT062 trägt keine explizite Sensor-Quelle
pro Plot (offen als WF2-42).

> **Hinweis (Regression behoben):** Bis WF2-40 war ein ADS-B-`◆`-Badge nur im
> **Data-Block-Label** vorgesehen (frühere `internal/webui/static/app.js`); es
> ging beim Vue-Port verloren und ist nun als Symbol-Form ◆ wiederhergestellt
> (Register: **FR-ASD-007** löst **FR-ASD-006** ab). Die alte `static/app.js` ist
> toter Referenz-Code.

### 2.3 Ausgang: Feed-Status an den Browser

Der Feed-Status (`feed_status`-Nachricht) wird separat gesendet, wenn sich
die Liveness des Feeds ändert (ok → stale, stale → ok, erster Heartbeat).
Er löscht **nicht** das Lagebild im Browser.

### 2.4 Aeronautische Daten (best-effort)

```
pkg/aeronautical.Service
    │
    ├─► Periodischer Fetch von OpenAIP-REST-API (default 24h)
    │       └─► Last-Good-Cache bei Fehler
    └─► HTTP-Endpunkte /api/airspace, /api/navaids, /api/waypoints
            └─► GeoJSON-FeatureCollections an den Browser
```

Diese Daten sind entkoppelt vom Track-Pfad: ein OpenAIP-Ausfall beeinflusst
weder die Track-Darstellung noch den Readiness-Status.

---

## 3. Ports und Endpunkte

### Port 8080 — Betriebsebene

Unauthentifiziert, für Monitoring-Systeme und Load-Balancer.

| Pfad | Methode | Bedeutung |
|------|---------|-----------|
| `/health` | GET | Liveness-Probe: `200 ok` wenn Prozess läuft |
| `/ready` | GET | Readiness-Probe: `200 ready` oder `503 not ready: …` (siehe [Abschnitt 4](#4-health--und-readiness-probes)) |
| `/metrics` | GET | Prometheus-Exposition (Textformat 0.0.4) |

Port überschreibbar via `WAYFINDER_PROBE_PORT`.

### Port 8081 — Browser-Rand

Durch `authMiddleware` geschützt (wenn `WAYFINDER_AUTH_TOKEN` gesetzt).

| Pfad | Methode | Bedeutung |
|------|---------|-----------|
| `/` | GET | ASD-Frontend (eingebettete Vue-3-SPA, Route `/`) |
| `/admin` | GET | Admin-Oberfläche (Vue-SPA-Route, History-Mode; nur sinnvoll bei Multi-Tenancy) — WF2-32 |
| `/ws` | GET → Upgrade | WebSocket — Track- und Feed-Status-Updates |
| `/api/map-config` | GET | Kartentheme und Startkonfiguration als JSON |
| `/api/airspace` | GET | Luftraumstrukturen (GeoJSON, best-effort). **ONB-6 (ADR 0011):** im Multi-Mandanten-Betrieb hinter der Tenant-Middleware und liefert den **Cache des Request-Mandanten** (eigener Schlüssel/AOI, Fallback auf den globalen Cache); Single-Tenant unverändert global/unauthentifiziert |
| `/api/navaids` | GET | VOR/NDB-Beacons (GeoJSON, best-effort). **ONB-6:** mandanten-aufgelöst wie `/api/airspace` |
| `/api/waypoints` | GET | Wegpunkte (GeoJSON, best-effort). **ONB-6:** mandanten-aufgelöst wie `/api/airspace` |
| `/api/admin/whoami` | GET | Rollen-Probe + **effektive Feature-Flags** (`features`) als JSON; enthält seit ONB-1 `must_change_password`; rollen-gegated (WF2-32/50) |
| `/api/admin/me` | GET | **ONB-1 (ADR 0011):** eigenes Konto (`{user_id, tenant_id, subject, role, must_change_password}`); **rollen-unabhängig** (kein `requireAdmin`) |
| `/api/admin/me/password` | PUT | **ONB-1:** eigenes Passwort ändern (`{current_password, new_password}`, neu min. 8); aktuelles Passwort falsch → 401; setzt `must_change_password=false`; **auch im Pflichtwechsel-Zustand erreichbar** |
| `/api/admin/me` | DELETE | **ONB-1:** eigenes Konto löschen; **„letzter aktiver Admin"-Guard** (letzter Admin → 409, keine Selbst-Aussperrung) |
| `/api/admin/overview` | GET | **AP3:** Mandanten-Dashboard als Aggregat — je Mandant `{id, slug, name, status, features[], feeds[], user_count}` in einem Call; **admin** |
| `/api/admin/feeds/health` | GET | **AP4:** Gesundheitszustand aller Feeds — je Feed `{feed_id, color, stale, ever_seen, last_heartbeat_ago_s, track_count_recent, sensors_active, sensors_total}` aus der In-Memory-Health-Registry; `color` ist **grün** (Heartbeat frisch, unabhängig vom Verkehr — leerer Himmel ist kein Fehler) / **gelb** (Sensor-Teilausfall: `sensors_active < sensors_total > 0`; CAT063, ADR 0010) / **rot** (kein Heartbeat = toter Feed oder nie gesehen); **admin** |
| `/api/admin/feeds` | POST | **ONB-5 (ADR 0011):** neuen Feed anlegen (`{name, multicast_group, port, region?, sensor_mix?}`) → 201; `multicast_group` muss IPv4-Multicast sein (224.0.0.0–239.255.255.255), `port` 1..65535, `sensor_mix` gegen das Sensorklassen-Vokabular validiert (unbekannt → 400); doppelter Name → 409; **der Live-Receiver tritt der Multicast-Gruppe sofort bei** (kein Neustart); scheitert der Beitritt, wird die Katalogzeile zurückgerollt; **admin** |
| `/api/admin/feeds/{id}` | DELETE | **ONB-5:** Feed löschen → 204; **der Live-Receiver verlässt die Multicast-Gruppe sofort**; kaskadiert (ON DELETE CASCADE) auf die Abos, die ihn referenzierten (Guard C: kein Blockieren bei bestehenden Abos — Grants kaskadieren); unbekannter Feed → 404; **admin** |
| `/api/admin/feeds/{id}/sources` | GET | **ORCH-1b (ADR 0012):** Quell-Konfiguration des Feeds — `{sources:[{type, bbox?, sac?, sic?, cred_ref?}], coverage_bbox}`; `sources` serialisiert als `[]` (nie `null`); unbekannter Feed → 404; **admin** |
| `/api/admin/feeds/{id}/sources` | PUT | **ORCH-1b:** Quell-Konfiguration setzen → 200 (kanonisch zurückgelesen). Server-validiert: geschlossenes Quell-Vokabular (`adsb_opensky`/`flarm_aprs` erfordern `bbox`, keine `sac`/`sic`; `radar_asterix` erfordert `sac`/`sic` 0..255), WGS84-`bbox`, `cred_ref` als Verweis (non-blank, ≤200) — Verstoß → **400 mit Quell-Index**, **kein** Teil-Write. Fehlt `coverage_bbox`, leitet der Server die **grobe äußere** BBox aus den Quell-BBoxen + Default-Marge (50 km) ab; eine explizit gesetzte `coverage_bbox` (WGS84-validiert) gewinnt (Operator-Override). Unbekannter Feed → 404; **admin** |
| `/api/admin/tenants/{id}/view` | GET/PUT | **AP3:** Standard-Sicht **eines beliebigen** Mandanten lesen/schreiben (cross-tenant Editor; gleiche `validateView` wie `/api/admin/view`); **admin** |
| `/api/admin/tenants/{id}/entitlements[/{key}]` | GET/PUT | Feature-Entitlements pro Mandant; **admin** (WF2-50) |
| `/api/admin/tenants/{id}/openaip` | GET | **ONB-6 (ADR 0011):** meldet `{configured: bool}` — ob der Mandant einen **eigenen** OpenAIP-Schlüssel hat. **Nie der Schlüssel selbst.** **admin** |
| `/api/admin/tenants/{id}/openaip` | PUT | **ONB-6:** Schlüssel setzen/löschen (`{api_key: string\|null}`) → 204; leer/Whitespace/`null` = löschen (Rückfall auf den globalen Schlüssel); zu lang → 400; unbekannter Mandant → 404; **Live-Apply** (re)startet den Per-Mandant-Refresh sofort; **admin** |
| `/api/admin/admins` | GET/POST | **ONB-3 (ADR 0011):** Plattform-Admins (global, **kein Mandant**) auflisten / anlegen; **admin**. POST `{subject, email?, password?}` → 201; doppelter Subject → 409; Passwort min. 8 (optional, für Proxy-Modus) |
| `/api/admin/admins/{id}` | PATCH/DELETE | **ONB-3:** Admin pausieren/reaktivieren (`{status}`) bzw. löschen; **admin**; **„letzter aktiver Admin"-Guard** (Pausieren/Löschen des letzten aktiven Admins → 409); ID eines Mandanten-Nutzers → 404 (nicht auf dieser Fläche erreichbar) |
| `/api/admin/admins/{id}/password` | PUT | **ONB-3:** Admin-Passwort setzen/zurücksetzen (`{password}`, min. 8); **admin** |
| `/api/admin/tenants/{id}/users` | GET/POST | Zugänge eines Mandanten (Rolle `user`) auflisten / anlegen (AP6); **admin**. POST `{subject, email?, password?}` → 201; Rolle **immer `user`** (ein mitgeschicktes `role:"admin"` → 400, Admins laufen über `/api/admin/admins`, ONB-3); Passwort min. 8 Zeichen; doppelter Subject → 409 |
| `/api/admin/tenants/{id}/users/{uid}` | PATCH/DELETE | Zugang pausieren/reaktivieren (`{status:"active"\|"paused"}`) bzw. löschen (AP6); **admin**. User-ID aus fremdem Mandanten → 404 |
| `/api/admin/tenants/{id}/users/{uid}/password` | PUT | Passwort setzen/zurücksetzen (`{password}`, min. 8) (AP6); **admin** |
| `/api/admin/tenants` | POST | **ONB-4 (ADR 0011):** neuen Mandanten anlegen (`{slug, name?}`) → 201; Slug DNS-label-artig (Kleinbuchstaben/Ziffern/Bindestrich, kein führender/abschließender Bindestrich, ≤ 63), `name` Default = `slug`; doppelter Slug → 409; **admin** |
| `/api/admin/tenants/{id}` | DELETE | **ONB-4:** Mandanten löschen → 204; kaskadiert (ON DELETE CASCADE) auf Zugänge (+ Credentials), Abos, Entitlements, View-Konfig; **Guard B**: solange der Mandant noch **Zugänge** hat → **409** (erst Konten entfernen); **admin** |
| `/api/admin/tenants/{id}` | PATCH | Mandant pausieren/reaktivieren (`{status}`); kaskadiert via Login-Enforcement auf alle Zugänge (AP6); **admin** |
| `/api/admin/sensor-classes` | GET | Sensorklassen-Katalog (read-only Referenz, WF2-41) |
| `/api/admin/impersonation` | GET/POST/DELETE | Cross-Tenant Read-Only-Impersonation (ADR 0008): **GET** liefert den aktuellen Status (`{active, tenant_id}`) für den Banner (Reload-fest, da der Cookie HttpOnly ist); **POST** `{"tenant_id":…}` mintet den signierten Grant-Cookie (`super_admin` only, Ziel-Mandant muss existieren → sonst 404); **DELETE** beendet sie (Cookie löschen). Nur aktiv, wenn ein Signing-Key (`WAYFINDER_SESSION_KEY`) konfiguriert ist. |
| `/api/admin/*` | div. | Tenant-skopiertes Admin-API (WF2-31/31b); rollen-gegated |

> **Pflicht-Passwortwechsel-Gate (ONB-1, ADR 0011):** Trägt das eingeloggte Konto
> `must_change_password=true` (so wird der beim ersten Boot auto-seedete
> Standard-Admin angelegt), weist das Admin-API **jede** Route außer `GET
> /api/admin/whoami`, `GET /api/admin/me` und `PUT /api/admin/me/password`
> **fail-closed mit 403 `password_change_required`** ab. Das Flag wird vom
> `tenant.Identity` getragen (kein zusätzlicher DB-Lookup); das SPA erkennt den
> Marker und zeigt die Pflichtwechsel-Maske. Erst der erfolgreiche Passwortwechsel
> setzt das Flag zurück und gibt das übrige Admin-Surface frei.

> **Feed-Sensorklassen & Abo-Entitlement (WF2-41):** Ein Feed trägt eine
> **Sensorklassen-Zusammensetzung** als Metadatum (`sensor_mix`) aus dem
> kontrollierten Vokabular `PSR`/`SSR`/`MODE_S`/`ADS-B`/`MLAT`/`FLARM`; gängige
> Legacy-Schreibweisen werden beim Anlegen kanonisiert, unbekannte Klassen
> **abgewiesen** (`feed add` → Fehler). **Abos binden an Feeds:** ein Mandant
> **ohne** `multi_feed`-Entitlement hält **höchstens einen** Feed — ein zweiter
> distinkter Grant wird mit **409 Conflict** abgewiesen, *bevor* er die DB
> erreicht (harte Invariante; super_admin muss erst `multi_feed` setzen).

> **SPA-History-Fallback (WF2-32):** `webui.Handler` liefert für jeden nicht als
> Datei auflösbaren Pfad die `index.html`-Shell aus (Client-Router übernimmt) —
> so überleben Deep-Links wie `/admin` einen Reload. Das API-Surface (`/api/…`,
> `/ws`, Probes) ist über speziellere Mux-Pattern registriert und wird vom Fallback
> nie beschattet.

---

## 4. Health- und Readiness-Probes

### `/health` — Liveness

Gibt immer `200 ok` zurück, sobald der HTTP-Server auf Port 8080 läuft. Wenn
dieser Endpunkt nicht antwortet, ist der Prozess tot und muss neu gestartet
werden.

### `/ready` — Readiness

Signalisiert, ob Wayfinder einen Datenstrom empfängt und bereit ist,
Lotsen-Anfragen zu bedienen.

| Zustand | HTTP | Body |
|---------|------|------|
| Noch kein Heartbeat empfangen | 503 | `not ready: waiting for first heartbeat` |
| Feed stale (Timeout überschritten) | 503 | `not ready: feed stale` |
| Feed ok (Heartbeat frisch) | 200 | `ready` |

**Semantik:** Readiness schaltet auf 503, sobald der Feed jemals aktiv war
und danach für länger als `WAYFINDER_FEED_STALE_TIMEOUT` schweigt. Auf diese
Weise schließt ein Kubernetes-Ingress den Pod aus der Rotation, wenn Firefly
nicht sendet — der Lotse sieht keinen veralteten Lagestand.

Wenn Firefly nie gestartet wurde (und damit nie ein CAT065-Heartbeat
eintraf), gilt `/ready` als "nie bereit" und gibt dauerhaft 503 zurück.

---

## 5. Prometheus-Metriken

Alle Metriken werden auf `http://localhost:8080/metrics` im
Prometheus-Textformat 0.0.4 exponiert. Die Implementierung verwendet keine
externe Prometheus-Bibliothek — der Exporter ist handgerollt in
`pkg/metrics/metrics.go`.

### 5.1 CAT062-Track-Eingang

| Metrik | Typ | Beschreibung |
|--------|-----|--------------|
| `wayfinder_cat062_blocks_received_total` | Counter | Anzahl empfangener CAT062-Datenblöcke (UDP-Datagramme, Kategorie `0x3E`) |
| `wayfinder_cat062_tracks_received_total` | Counter | Anzahl dekodierter Track-Records über alle Blöcke |
| `wayfinder_cat062_decode_errors_total` | Counter | Anzahl verworfener Datagramme (Längen-/Format-Fehler, unbekannte Kategorie) |
| `wayfinder_tracks_current` | Gauge | Anzahl aktuell bekannter Tracks aus dem zuletzt empfangenen Block |

### 5.2 WebSocket-Clients

| Metrik | Typ | Beschreibung |
|--------|-----|--------------|
| `wayfinder_ws_clients_connected` | Gauge | Anzahl aktuell verbundener Browser-Clients (global, ungelabelt) |
| `wayfinder_ws_clients_evicted_total` | Counter | Anzahl Clients, die wegen vollem Send-Channel entfernt wurden (langsame oder hängende Verbindungen) |
| `wayfinder_tenant_ws_clients_connected{tenant="…"}` | Gauge | **Pro Mandant** verbundene Clients (WF2-23.2). Label-Wert = stabile `tenant_id`. Nur im Multi-Mandanten-Betrieb. |
| `wayfinder_tenant_tracks_delivered_total{tenant="…"}` | Counter | **Pro Mandant** zugestellte Track-Nachrichten (WF2-23.2), fürs Billing/SLA-Monitoring. |
| `wayfinder_impersonation_sessions_total` | Counter | Gestartete `super_admin`-Read-Only-Impersonation-`/ws`-Sessions (ADR 0008). **Bewusst aus den Pro-Tenant-Serien ausgeschlossen** (die Session läuft mit `scope.TenantID=0`), damit Support-Einblicke Verbrauch/SLA des Ziel-Mandanten nicht verfälschen. |

> **Kardinalitäts-Regel (WF2-23):** Metrik-Labels sind auf das **kontrollierte
> `tenant`-Label** (stabile `tenant_id`) beschränkt. Hochkardinale Identität
> (`user_id`, `subject`, `session`) gehört **ausschließlich** ins Audit-Log
> (§6 Audit), nie in Metriken.

### 5.3 CAT065-Feed-Health

| Metrik | Typ | Beschreibung |
|--------|-----|--------------|
| `wayfinder_cat065_heartbeats_received_total` | Counter | Anzahl empfangener CAT065-SDPS-Status-Meldungen |
| `wayfinder_feed_stale` | Gauge | `1` wenn Feed aktuell als stale gilt, `0` wenn ok oder noch nie gesehen |

### 5.4 Aeronautische Daten (OpenAIP)

| Metrik | Typ | Beschreibung |
|--------|-----|--------------|
| `wayfinder_openaip_fetch_success_total` | Counter | Anzahl erfolgreicher OpenAIP-Datenabrufe. **ONB-6:** im Multi-Mandanten-Betrieb **Summe** über den globalen + alle Per-Mandant-Caches (monoton über Mandanten-Churn) |
| `wayfinder_openaip_fetch_failures_total` | Counter | Anzahl fehlgeschlagener OpenAIP-Datenabrufe. **ONB-6:** wie oben summiert |
| `wayfinder_openaip_cache_age_seconds` | Gauge | Alter des letzten erfolgreichen Cache-Befüllens in Sekunden; `-1` wenn noch kein erfolgreicher Fetch. **ONB-6:** bezieht sich auf den **globalen Fallback-Cache** |

### 5.5 Feature-Entitlements (Multi-Mandant, WF2-50)

| Metrik | Typ | Bedeutung |
|--------|-----|-----------|
| `wayfinder_feature_check_failclosed_total{reason="db_error"}` | Counter | Feature-Checks, die **fail-closed** verweigert wurden, weil der Store einen Fehler lieferte. `> 0` ⇒ DB-/Persistenz-Problem am Entitlement-Pfad (alarmwürdig). |
| `wayfinder_feature_check_failclosed_total{reason="unknown_key"}` | Counter | Feature-Checks gegen einen **nicht im Katalog** geführten Key (verweigert). `> 0` ⇒ Code-/Konfig-Drift (Tippfehler oder entferntes Feature). |

Nur im Multi-Mandanten-Betrieb (Feature-Gating existiert nur dort). Default-Deny:
Ein fehlendes Flag ist kein Fehler und erzeugt **keinen** Zähler-Anstieg.

#### Feature-Katalog (`pkg/feature`, AP2)

Der Katalog ist **geschlossen** — nur hier geführte Keys sind gültig. Unbekannte
Keys werden fail-closed verweigert und über den `unknown_key`-Zähler sichtbar.
`whoami` liefert automatisch alle Katalog-Keys mit ihrem effektiven Wert.

| Key | Beschreibung | Default |
|-----|--------------|---------|
| `stca` | Short-Term Conflict Alert (ASD-006) | deny |
| `multi_feed` | Mehrere Sensor-Feeds abonnieren (WF2-41) | deny |
| `premium_layers` | Premium-ASD-Kartenoverlay | deny |
| `airspaces` | Luftraum-Overlays (CTR, TMA, restricted, info) — ASD-011 | deny |
| `range_rings` | Range-Ring-Overlay — ASD-012 | deny |
| `history_dots` | Track-History-Punkte — ASD-004a | deny |
| `vor_ndb` | VOR/NDB-Navaid-Overlay — ASD-003 | deny |
| `waypoints` | Wegpunkt-Overlay — ASD-003 | deny |

**UI-Gate-Formel (rein kosmetisch, keine Serverenforcement auf Aero-Daten):**
`!isAuthorized || hasFeature(key)` — Nicht-Admin-Nutzer (403 auf `whoami`,
`isAuthorized = false`) sehen alle Layer-Steuerelemente; Admin-Nutzer sehen nur,
was ihr Mandant freigeschaltet hat.

### 5.6 Beispiel-Ausgabe

```
# HELP wayfinder_cat062_blocks_received_total Total CAT062 data blocks received
# TYPE wayfinder_cat062_blocks_received_total counter
wayfinder_cat062_blocks_received_total 1482

# HELP wayfinder_cat062_decode_errors_total Total CAT062 decode errors
# TYPE wayfinder_cat062_decode_errors_total counter
wayfinder_cat062_decode_errors_total 0

# HELP wayfinder_feed_stale 1 if the CAT065 feed is currently stale
# TYPE wayfinder_feed_stale gauge
wayfinder_feed_stale 0

# HELP wayfinder_openaip_cache_age_seconds Seconds since last successful OpenAIP cache fill
# TYPE wayfinder_openaip_cache_age_seconds gauge
wayfinder_openaip_cache_age_seconds 3821.4
```

---

## 6. Umgebungsvariablen

Vollständige Referenz aller Konfigurationsparameter. Reihenfolge der
Auflösung (höchste Priorität zuerst):

1. Umgebungsvariablen
2. YAML-Konfigurationsdatei (`wayfinder.yaml`)
3. Eingebaute Defaults

### 6.1 Netzwerk & Feed

| Variable | Default | Typ | Beschreibung |
|----------|---------|-----|--------------|
| `FIREFLY_CAT062_GROUP` | `239.255.0.62` | string | UDP-Multicast-Gruppe |
| `FIREFLY_CAT062_PORT` | `8600` | int | UDP-Port |
| `WAYFINDER_FEED_ID` | `0` | int64 | Katalog-Feed-ID des Einzel-Feeds (WF2-20), auf jeden Track gestempelt; `0` = Single-Tenant. Wird im Multi-Feed-Modus (WF2-20.2) durch den DB-Katalog abgelöst. |
| `WAYFINDER_PROBE_PORT` | `8080` | int | Port für Probe/Metrics-Endpunkte |
| `WAYFINDER_FEED_STALE_TIMEOUT` | `3` | int (s) | Sekunden ohne CAT065-Heartbeat bis Staleness |

### 6.2 Karte

| Variable | Default | Typ | Beschreibung |
|----------|---------|-----|--------------|
| `WAYFINDER_MAP_CENTER_LAT` | `50.0379` | float64 | Latitude des Startzoom-Zentrums |
| `WAYFINDER_MAP_CENTER_LON` | `8.5622` | float64 | Longitude des Startzoom-Zentrums |
| `WAYFINDER_MAP_ZOOM` | `8` | float64 | Initialer Zoom-Level (MapLibre, 1–22) |
| `WAYFINDER_MAP_THEME` | `dark` | enum | `dark` (CARTO Dark, keine API-Key) oder `osm` (OpenStreetMap) |
| `WAYFINDER_MAP_STYLE_URL` | *(leer)* | URL | Überschreibt Theme komplett — beliebige MapLibre-Style-URL |

### 6.3 OpenAIP

| Variable | Default | Typ | Beschreibung |
|----------|---------|-----|--------------|
| `WAYFINDER_OPENAIP_API_KEY` | *(leer)* | string | **Globaler** API-Key; leer = Feature global aus. **ONB-6 (ADR 0011):** dient im Multi-Mandanten-Betrieb als **Fallback** für Mandanten ohne eigenen Schlüssel; pro Mandant wird der Schlüssel in der DB (`tenants.openaip_api_key`) über `PUT /api/admin/tenants/{id}/openaip` gesetzt |
| `WAYFINDER_OPENAIP_RADIUS_KM` | `250` | int | Abfrageradius um das Zentrum in km. **ONB-6:** je Mandant um das **View-Zentrum** (oder dessen AOI-Box, falls gesetzt); ohne View die globale Karten-Box |
| `WAYFINDER_OPENAIP_REFRESH` | `24h` | duration | Refresh-Intervall (Go-Duration, z. B. `1h`, `30m`); gilt für den globalen **und** jeden Per-Mandant-Refresh |
| `WAYFINDER_OPENAIP_BASE_URL` | *(intern)* | URL | Override der OpenAIP-API-Basis-URL (geteilt von globalem und Per-Mandant-Client) |

### 6.5 Radarabdeckungs-Overlay (Paket 6)

| Variable | Default | Typ | Beschreibung |
|----------|---------|-----|--------------|
| `WAYFINDER_COVERAGE_SENSOR_N_LAT` | *(leer)* | float | Breitengrad Sensorstandort (WGS84) |
| `WAYFINDER_COVERAGE_SENSOR_N_LON` | *(leer)* | float | Längengrad Sensorstandort (WGS84) |
| `WAYFINDER_COVERAGE_SENSOR_N_MAX_RANGE_M` | *(leer)* | float | Außenradius in Metern |
| `WAYFINDER_COVERAGE_SENSOR_N_MIN_RANGE_M` | `0` | float | Innenradius in Metern (0 = kein Blindbereich) |
| `WAYFINDER_COVERAGE_SENSOR_N_LABEL` | *(leer)* | string | Sensor-Bezeichnung für Tooltip |
| `WAYFINDER_COVERAGE_RING_COLOR` | `#5B8DEF` | CSS-Hex | Farbe aller Ringe (einheitlich) |

**Endpoint:** `GET /api/coverage/rings` → `application/geo+json` FeatureCollection.
Wird einmal beim Frontend-Load abgerufen. Leere Collection wenn kein Sensor konfiguriert.

### 6.6 Range-Ring-Overlay & Karten-Controls (ASD-012)

Rein **client-seitige** Anzeigehilfen (keine Env-Variablen, keine Backend-Wirkung):

- **Range-Rings:** konzentrische Kreise **konstanter Boden-Distanz** um den
  Karten-Mittelpunkt (`/api/map-config`) als Distanz-Raster — abgegrenzt vom
  Sensor-Coverage-Overlay (§6.5). **Live operator-konfigurierbar** über die
  Sidebar: Abstand `5/10/15 NM` + Anzahl (Default 10 NM / 5), default
  ausgeblendet. Geodätisch erzeugt (gleiche Distanz in jede Richtung, **nicht**
  grad-gestaucht).
- **Scale-Bar:** MapLibre `ScaleControl` in **nautischen Meilen** (unten-links).
- **Nord-Kompass:** MapLibre `NavigationControl` (oben-links) zeigt das aktuelle
  Bearing und setzt per Klick auf Nord; freie Kartendrehung bleibt aktiv.

### 6.4 Sicherheit

| Variable | Default | Typ | Beschreibung |
|----------|---------|-----|--------------|
| `WAYFINDER_ALLOWED_ORIGINS` | *(leer)* | string | Kommaseparierte Cross-Origin-Allowlist. Leer = nur Same-Origin. |
| `WAYFINDER_AUTH_TOKEN` | *(leer)* | string | Bearer-Token. Leer = kein Check (Warn-Log). |
| `WAYFINDER_TLS_CERT` | *(leer)* | Pfad | TLS-Zertifikat (PEM). Nur aktiv wenn beide TLS-Variablen gesetzt. |
| `WAYFINDER_TLS_KEY` | *(leer)* | Pfad | TLS-Schlüssel (PEM). |

### 6.6 Multi-Mandanten (Wayfinder 2.0)

Aktiv nur bei gesetztem `WAYFINDER_DB_URL` (sonst Single-Tenant, keine DB/
Middleware). Mit DB: Schema-Migrationen beim Start, `/ws` durch die
Tenant-Middleware geschützt (fail-closed → `401` ohne gültigen Mandanten-Nutzer).
Identitäts-Modell siehe ADR 0006 §5.

| Variable | Default | Typ | Beschreibung |
|----------|---------|-----|--------------|
| `WAYFINDER_DB_URL` | *(leer)* | DSN | PostgreSQL-Verbindung. Leer = Single-Tenant (keine DB). |
| `WAYFINDER_AUTH_MODE` | `none` | enum | `proxy` / `builtin` / `none`. Ungültig → `none`. |
| `WAYFINDER_OIDC_ISSUER` | *(leer)* | URL | proxy: OIDC-Issuer (Discovery/JWKS), Pflicht. |
| `WAYFINDER_OIDC_AUDIENCE` | *(leer)* | string | proxy: erwartete Audience, Pflicht. |
| `WAYFINDER_SESSION_KEY` | *(leer)* | string | builtin: HMAC-Schlüssel für Session-Cookies. Leer in builtin → Wayfinder erzeugt einen **flüchtigen** Zufalls-Schlüssel und warnt (Sessions überleben keinen Neustart, nicht multi-Replica-fähig; ONB-1, ADR 0011). Für Produktion festen Schlüssel setzen (`openssl rand -hex 32`). |
| `WAYFINDER_SESSION_COOKIE` | `wf_session` | string | builtin: Cookie-Name. |
| `WAYFINDER_SESSION_TTL` | `12h` | duration | builtin: Session-Lebensdauer. |
| `WAYFINDER_NONE_SUBJECT` | `default` | string | none: festes Subject je Anfrage. |
| `WAYFINDER_BOOTSTRAP_PASSWORD` | *(leer)* | string | Nur vom `bootstrap`-Subcommand gelesen: builtin-Passwort des ersten Admins. |

**builtin-Login-Endpoints:** `POST /api/login` (`{"subject","password"}` →
HttpOnly-Cookie via `auth.MintSession`, sonst `401` mit Timing-Angleich gegen
User-Enumeration), `POST /api/logout` (Cookie löschen). Nur im builtin-Modus
registriert.

**Admin-Bootstrap (WF2-13):** Subcommand `wayfinder bootstrap` (`cmd/wayfinder/
bootstrap.go`) legt **idempotent** ersten Mandanten + Admin-Nutzer (+ builtin-
Passwort via `WAYFINDER_BOOTSTRAP_PASSWORD`) an; liest `WAYFINDER_DB_URL`,
migriert, verweigert das Re-Homing eines Subjects in einen anderen Mandanten.

**Boot-Auto-Seed (ONB-1, ADR 0011):** In `builtin`-Modus mit gesetzter
`WAYFINDER_DB_URL` provisioniert Wayfinder beim Start **automatisch** einen
Standard-Mandanten (`default`, als bequemes Zuhause für die ersten Lotsen-Zugänge)
+ Standard-Admin (Subject `admin`, Passwort `admin`) — aber **nur, wenn noch kein
aktiver Admin existiert** (`UserRepo.CountActiveAdmins == 0`). Der seedete Admin
trägt `must_change_password=true`; das bekannte Default-Passwort ist also nur bis
zum erzwungenen Wechsel beim ersten Login gültig. Der Seed
(`cmd/wayfinder/seed.go`) ist idempotent und wiederverwendet `runBootstrap`; ein
Neustart oder ein bereits rotiertes Passwort wird nie überschrieben. So ist eine
frische Instanz ohne Terminal-Schritt benutzbar (`docker-compose.onboarding.yml`).

**Strikte Admin/Nutzer-Trennung (ONB-3, ADR 0011):** Plattform-Admins und
Mandanten-Nutzer (Lotsen) sind sauber getrennt. Ein **Admin ist global** und
gehört **keinem Mandanten** an; ein **Nutzer gehört genau einem Mandanten**.
Migration `00007_admin_tenant_nullable.sql` macht `users.tenant_id` nullable,
löst bestehende Admins von ihrem (bedeutungslosen) Mandanten und erzwingt die
Invariante per **CHECK-Constraint** (`admin` ⇒ `tenant_id IS NULL`, `user` ⇒
`tenant_id IS NOT NULL`). In Go bildet `TenantID == 0` „kein Mandant" ab
(`scanUser` liest NULL → 0). **Folgen:** der `store.UserRepo` hat getrennte
Konstruktoren `Create` (Nutzer, mit Mandant) und `CreateAdmin` (Admin, ohne);
`runBootstrap` verzweigt nach Rolle (Admin braucht **kein** `-tenant`); der
Login-Pfad überspringt die Mandanten-Pause-Kaskade für tenantlose Admins (sonst
Selbst-Aussperrung); ein Admin hat auf der ASD-Karte ohne „Als Mandant ansehen"
(WF2-34) kein Feed-Scope (TenantID 0 → leeres Bild — gewollt). Admin-Verwaltung
läuft über die **dedizierten** Routen `/api/admin/admins` (siehe Endpunkt-Tabelle);
die per-Mandant-Route `/api/admin/tenants/{id}/users` verwaltet ausschließlich
Nutzer. Der **„letzter aktiver Admin"-Guard** (`wouldOrphanAdmins` →
`CountActiveAdmins`) schützt Pausieren/Löschen von Admins (409) — dieselbe
Invariante wie beim Boot-Seed und `DELETE /api/admin/me`.
**`/admin`-Gate:** `tenant.RequireRole(tenant_admin, super_admin)` hinter der
Tenant-Middleware (fail-closed `403` ohne passende Rolle/Identität); liefert eine
minimale whoami-JSON-Antwort, Admin-UI folgt WF2-32.

**Admin-API (WF2-31, `pkg/adminapi`):** tenant-skopiertes REST unter `/api/admin/*`
hinter `tenantMW`+`RequireRole(tenant_admin, super_admin)`. Die `tenant_id` kommt
**aus der Identity**, nie aus Pfad/Body (Isolation per Konstruktion). `GET/PUT
/api/admin/view` (Tenant-Default-Sicht, **server-validiert** in `validateView`:
Lat/Lon/Zoom-Bereiche, AOI wohlgeformt, `fl_min ≤ fl_max`), `GET
/api/admin/subscriptions` (eigene Feeds), `GET /api/admin/feeds` (Katalog,
read-only). DTOs verbergen Infra-Felder (multicast_group/port).

**super_admin-Provisioning (WF2-31b, cross-tenant):** `GET /api/admin/tenants`,
`GET/POST /api/admin/tenants/{tenantID}/subscriptions`, `DELETE
/api/admin/tenants/{tenantID}/subscriptions/{feedID}` — Ziel-`tenant_id` aus dem
**Pfad**. Doppel-Gate: äußerer `RequireRole(tenant_admin, super_admin)` +
in-handler `requireSuper` (`Identity.Role == super_admin`, sonst `403`) — die
einzige cross-tenant-schreibende Rolle (Billing-/Entitlement-Grenze). Validierung:
Ziel-Tenant/Feed müssen existieren (`404`), Body/Pfad-IDs wohlgeformt (`400`);
Grant/Revoke idempotent (`204`). Der Config-Cache (WF2-30) folgt später.

**Zugangs-Verwaltung (AP6, ADR 0009):** Der `admin` provisioniert und sperrt
**Zugänge** (Login-Konten, Rolle `user`) pro Mandant und pausiert ganze Mandanten
— alles cross-tenant hinter `requireAdmin` (`pkg/adminapi/adminapi_users.go`):
`GET/POST /api/admin/tenants/{id}/users`, `PATCH/DELETE …/users/{uid}`,
`PUT …/users/{uid}/password`, `PATCH /api/admin/tenants/{id}` (Mandant-Status).
Neue Konten sind **immer** Rolle `user` (Plattform-Admins kommen über
`bootstrap`); Passwort min. 8 Zeichen; doppelter Subject → `409`; eine User-ID
aus einem fremden Mandanten → `404` (die Ressourcen-Hierarchie bleibt ehrlich).
**Login-Enforcement (`pkg/tenant/login.go`, fail-closed):** ein **pausierter
Zugang** (`users.status='paused'`) — oder ein Zugang unter einem **pausierten
Mandanten** (`tenants.status='paused'`) — wird beim Login mit demselben
generischen `401` abgewiesen wie ein falsches Passwort (keine paused/active-
Enumeration, Timing-uniform; ein Tenant-Lookup-Fehler gilt als suspendiert). Die
**Sofort-Wirkung auf bereits laufende Sessions** ist bewusst **AP7**
(Session-Registry) — AP6 sperrt nur **neue** Anmeldungen. Schema:
`00005_user_status.sql` (`users.status`, CHECK `active|paused`, Default `active`,
nicht-breaking); Mandanten-Pause nutzt das vorhandene `tenants.status`.

**Multi-Feed-Empfang (WF2-20.2):** der `feeds`-Katalog (DB) treibt **N Receiver**
(einer je Feed, je `feed_id` aus 20.1). `cmd/wayfinder/feeds.go`: `resolveFeeds`
(Katalog → `feedConfig` je Zeile; leer/kein-DB → Fallback auf den ENV-Einzelfeed)
+ `buildReceivers`. In `main()` läuft `setupTenancy` **vor** dem Receiver-Start;
ein Feed, der nicht beitreten kann, wird übersprungen (kein Feed → fatal);
`wayfinder_cat062_decode_errors_total` summiert über alle Receiver. Feed-Health
bleibt **global** (per-Feed später, WF2-23). **Feed-CLI** (`cmd/wayfinder/
feedcmd.go`): `wayfinder feed add -name -group [-port] [-region] [-sensor-mix]`
und `wayfinder feed list` pflegen den Katalog, bis die Admin-API existiert
(WF2-31). NATS-/Stream-Feed-Source folgt WF2-53.

**Feed-Quell-Datenmodell (ORCH-1a, ADR 0012):** Vorbereitung der Auto-
Orchestrierung. Migration `00010_feed_source_config.sql` ergänzt `feeds` um
`source_config` (JSONB-Array, Default `'[]'`) — die generische, Firefly-agnostische
Liste der Live-Quellen, aus denen die dem Feed gewidmete Firefly-Instanz später
ihre Tracks rechnen soll (`adsb_opensky`/`flarm_aprs` mit WGS84-`bbox`,
`radar_asterix` mit `sac`/`sic`; optional `cred_ref` als **Verweis** auf ein
Pro-Feed-Secret, nie Klartext) — und `coverage_bbox` (JSONB, nullable), die daraus
abgeleitete **grobe äußere** Geo-Grenze (Union der Quell-BBoxen + Marge), getrennt
von der präzisen inneren Mandanten-AOI (WF2-21.2). `pkg/store/feed_sources.go`:
`SourceConfig.Validate` (geschlossenes Vokabular, Per-Art-Regeln am Schreib-Rand),
`CoverageBBox(marginKm)` (reine Ableitung, lat/lon-geklemmt) und dedizierte
Accessoren `FeedRepo.GetSourceConfig`/`SetSourceConfig` (nicht in der schlanken
`Feed`-Zeile, analog OpenAIP-Key-Isolation). **Rein Wayfinder-intern**, keine
CAT062-Schnittstellen-Wirkung. Bedienbar über `GET/PUT /api/admin/feeds/{id}/sources`
(ORCH-1b) und den Quell-Builder im „Feeds"-Tab (ORCH-1c, `AdminFeeds.vue`). Der
Reconciler (ORCH-3) übersetzt `coverage_bbox` später nach `FIREFLY_COVERAGE_BBOX`.

**Auto-Orchestrierung — Control-Plane (ORCH-2/3, ADR 0012):** Wayfinder fährt pro
abonniertem Feed automatisch eine Firefly-Instanz hoch. Bausteine:
`pkg/instance` (`Backend`-Abstraktion `Start`/`Stop`/`Status`/`RunningFeeds`,
idempotent, Identität = `feed_id`; generische `Spec` mit Multicast-Endpoint,
Coverage, Quellen und Secret-**Referenzen**; `MemoryBackend` als Platzhalter bis
zum Docker-Adapter), `pkg/reconciler` (Operator-Muster: Soll = alle Feeds mit ≥ 1
Abo, Ist = `Backend.RunningFeeds`; idempotent, crash-fest, Orphan-Cleanup,
Per-Feed-Fehler isoliert) und `pkg/orchestrator` (`StoreDesiredState`: Soll aus
`SubscriptionRepo.ListSubscribedFeeds` × `FeedRepo.GetSourceConfig`).
**🔒 Getrennter Prozess (ADR 0012 §6):** Die Control-Plane läuft als **eigenes
Binary `cmd/wayfinder-orchestrator`**, NICHT im browser-zugewandten Server — nur
dieser Prozess erhält später das Privileg, Tracker-Instanzen zu starten
(Container-Runtime), während der Browser-Rand bloß den Soll-Zustand in die DB
schreibt. Beide kommunizieren ausschließlich über den Katalog. Das Binary
**migriert NICHT** (der Hauptserver besitzt das Schema; ein einziger Migrator
vermeidet Races). Env: `WAYFINDER_DB_URL` (Pflicht), `WAYFINDER_ORCHESTRATOR_INTERVAL`
(Reconcile-Periode, Default `15s`), `WAYFINDER_LOG_LEVEL`. Modi: `--once` (ein
Reconcile-Lauf, dann Exit — für CI/Dev/k8s-Job; Exit 2 = Config/Flag-Fehler,
Exit 1 = Laufzeitfehler) und Default (Reconcile-Schleife mit Graceful-Shutdown
auf SIGINT/SIGTERM). **Stand:** Prozess-Skelett gegen Store-Soll + Reconciler
verdrahtet, Backend noch `MemoryBackend`; der echte Docker-Adapter (ORCH-2b) und
die Secret-Auflösung + der Änderungs-Trigger (ORCH-2c 3/3) folgen. Bis dahin ist
der Orchestrator **nicht** Teil des Standard-Deployments (`INSTALLATION.md`), weil
er noch nichts real startet.

**Scoped Fan-out (WF2-21.1, 🔒 NFR-SEC-003):** der Broadcaster stellt einem
`/ws`-Client einen Track **nur** zu, wenn dessen Mandant den Feed abonniert hat.
`broadcast.Scope` (Menge erlaubter `feed_id`; nil = unscoped/Single-Tenant, leer =
nichts/fail-closed) hängt am `Client`; `broadcastTracks` prüft
`scope.AllowsFeed(feed_id)` pro Batch/Client (Feed-Health über `messageChan` bleibt
**global**). `ws.ScopeResolver` löst den Scope am Handshake **vor** dem Upgrade auf
(Fehler → `403`, kein Stream); `cmd/wayfinder.newScopeResolver` liest die
Tenant-Identity (Middleware) + `subscriptions.ListFeedIDsByTenant`.

**Sicht-Filter (WF2-21.2, harte AOI/FL-Grenze):** Über die erlaubten Feeds legt
`broadcast.ViewFilter` (AOI-BBox + FL-Band in Fuß) eine **server-seitige
Datensparsamkeits-Grenze** — Tracks außerhalb verlassen den Server nicht
(Bandbreite/Billing/kein F12-Leck). `broadcastTracks` filtert pro Client die
einzelnen Tracks (`Scope.filterView`); **fail-open**: ein Track ohne gemessene FL
wird zugestellt, nie verworfen (NFR-SEC-003 Safety: nie ein reales Flugzeug
verschlucken). `resolveViewFilter` mappt `view_configs.GetEffective` (User-Override
→ Tenant-Default) → `ViewFilter` (FL von Flugfläche in Fuß, ×100); kein/leeres
Config → keine Beschränkung. **Lebenszyklus** (confirmed/tentative/coasting) bleibt
bewusst **client-seitig** (Declutter, reversibel); echte Klassifizierung wird ein
späteres server-seitiges Premium-Feature (nach Track-Anreicherung, WF2-40).

**Live-Apply (WF2-33):** Ändert ein Admin View oder Feed-Grants, werden die
**aktiven** `/ws`-Streams des Mandanten **live re-skopiert** — ohne Reconnect. Der
Broadcaster ist ein **Single-Goroutine-Actor**; der Scope-Tausch ist ein Kommando
durch denselben `Run`-Loop (`rescopeChan`/`ApplyScopes`), in dem auch
`broadcastTracks` läuft → **kein Lock am heißen Pfad, keine Race**, Run-Loop nie
blockiert. Ablauf: `cmd/wayfinder.rescopeTenant` schnappt die betroffenen Clients
(`ClientsForTenant`, liest nur immutable Identität), löst **off-Run** pro distinct
User neu auf (`resolveScope`, identisch zum Connect) und reicht den Tausch an `Run`
(`ApplyScopes`); Disconnects zwischen Snapshot und Apply werden übersprungen. Bei
**verkleinerter AOI** sendet der Server außenliegende Tracks schlicht nicht mehr
(kein Lösch-Signal) — das Frontend coastet sie über den Client-Timeout aus.
Ausgelöst von `pkg/adminapi` (`putView`/`grant`/`revoke`) über einen injizierten
`RescopeFunc`; bei Validierungsfehler (`400`) erfolgt **kein** Re-Scope.

**Isolations-Gate (WF2-22, NFR-SEC-003):** `pkg/broadcast/isolation_test.go`
sichert das Fan-out-Prädikat als Property-/Fuzz-Gate ab (Differential-Test gegen
ein unabhängiges Oracle + Ende-zu-Ende-Property + `FuzzScopeFilter`). Die Property-
und Fuzz-Seed-Tests laufen im normalen `go test`; erweitertes Fuzzing on-demand:
`go test ./pkg/broadcast/ -run '^$' -fuzz FuzzScopeFilter -fuzztime 30s`.

**Audit-Log (WF2-23.1, NFR-SEC-003):** Bei jedem `/ws`-Connect schreibt der
Scope-Resolver ein **strukturiertes `slog`-Event** (`component=audit`,
`event=ws_connect`) mit `tenant_id`, `user_id`, `subject`, `role`, `feeds`,
`aoi`, `fl_min_ft`/`fl_max_ft`, `remote` — der Nachweis „welcher Tenant sah welchen
Scope". 12-Factor: keine DB-Audit-Tabelle, Querying via externe Log-Senke
(ELK/Datadog). **Kardinalitäts-Regel:** hochkardinale Identität (`user_id`,
`subject`, `remote`) **nur** im Audit-Log, **nie** als Metrik-Label (Tenant-Label
für Metriken folgt WF2-23.2).

### 6.5 Betrieb

| Variable | Default | Typ | Beschreibung |
|----------|---------|-----|--------------|
| `WAYFINDER_LOG_LEVEL` | `info` | enum | `debug` / `info` / `warn` / `error`. Ungültiger Wert → `info`. |
| `WAYFINDER_CONFIG_FILE` | `wayfinder.yaml` | Pfad | Optionale YAML-Konfigurationsdatei. Fehlende Datei ist nicht fatal. |

---

## 7. Feed-Staleness-Erkennung

Wayfinder unterscheidet vier Feed-Zustände, die im Browser als farbiges
Banner angezeigt werden:

| Zustand | Banner | Beschreibung |
|---------|--------|--------------|
| Unbekannt | grau ⬜ | Noch kein CAT065-Heartbeat seit Start |
| OK | grün ✅ | Heartbeat frisch — auch bei leerem Himmel; kein Verkehr ist kein Fehler |
| Degraded | gelb ⚠️ | Heartbeat frisch, aber Sensor-Teilausfall (`sensors_active < sensors_total`): mindestens ein Radar abgefallen, aber noch mindestens eines aktiv (CAT063, ADR 0010) |
| Stale | rot 🔴 | Letzter Heartbeat liegt länger als `WAYFINDER_FEED_STALE_TIMEOUT` Sekunden zurück, oder Firefly hat aufgehört zu senden |

**Implementierung:** `pkg/health.FeedHealth` verfolgt den Zeitpunkt des
zuletzt empfangenen CAT065-Heartbeats. Eine Monitor-Goroutine in `main.go`
prüft den Zustand zyklisch (auch ohne eintreffenden Verkehr) und broadcastet
eine `feed_status`-WebSocket-Nachricht, wenn sich der Zustand ändert.

**Wichtig:** Ein Zustandswechsel leert **nicht** das Lagebild. Bereits
sichtbare Tracks bleiben auf der Karte, bis ein TSE-Record eintrifft oder
ein neuer Block ohne den jeweiligen Track kommt.

---

## 8. Sicherheitsmodell

### 8.1 Empfangspfad (CAT062/UDP)

Die Vertrauensgrenze liegt auf der **Netzwerkschicht** (ADR 0003):

- Der CAT062/CAT065-Strom wird als vertrauenswürdig behandelt, wenn er aus
  dem dedizierten Radar-Netzwerk-Segment stammt.
- Wayfinder implementiert **kein** kryptografisches Verfahren auf dem
  Datagrammpfad.
- Schutzmaßnahme gegen fehlerhafte Datagramme: robuster Decoder mit
  strikter Längenprüfung — kein Panic auf Eingabe-Daten, fehlerhafter
  Record wird verworfen, restliche Records im Block werden weiterverarbeitet.

### 8.2 Browser-Rand (Port 8081)

Der Browser-Rand kann mehrschichtig abgesichert werden:

**Primär (empfohlen):** TLS + Authentifizierung am vorgelagerten
Reverse-Proxy oder Kubernetes-Ingress (z. B. OIDC, mTLS) — kein
Krypto-Eigenbau im ASD.

**Ergänzend (fail-closed in Wayfinder selbst):**

| Mechanismus | Konfiguration | Verhalten bei Fehler |
|-------------|---------------|----------------------|
| Origin-Check | `WAYFINDER_ALLOWED_ORIGINS` | Cross-Origin-Request abgelehnt wenn nicht in Allowlist |
| Token-Auth | `WAYFINDER_AUTH_TOKEN` | `401 Unauthorized` + `WWW-Authenticate: Bearer` |
| TLS | `WAYFINDER_TLS_CERT` + `WAYFINDER_TLS_KEY` | HTTPS/WSS statt HTTP/WS |

**Token-Übergabe:** Da Browser-WebSocket-Clients keine Custom-Header beim
Handshake senden können, akzeptiert Wayfinder den Token entweder als
`Authorization: Bearer <token>`-Header (für REST-Clients) oder als
`?token=<token>`-Query-Parameter (für Browser-WebSocket).

**Health/Metrics (Port 8080)** sind bewusst unauthentifiziert — sie sollen
für Monitoring-Systeme ohne Token erreichbar sein.

---

## 9. Logging

Wayfinder schreibt strukturierte Logs im **JSON-Format auf stderr** via
`log/slog`. Log-Level ist zur Laufzeit über `WAYFINDER_LOG_LEVEL`
konfigurierbar.

### Wichtige Log-Ereignisse

| Level | Ereignis | Bedeutung |
|-------|----------|-----------|
| `INFO` | `receiver started` | UDP-Multicast-Socket geöffnet |
| `INFO` | `feed status changed` | Übergang ok↔stale oder erster Heartbeat |
| `WARN` | `auth token not set, relying on network isolation` | `WAYFINDER_AUTH_TOKEN` ist leer |
| `WARN` | `client evicted: send channel full` | Browser-Client hängt oder ist zu langsam |
| `WARN` | `openaip fetch failed` | OpenAIP-API nicht erreichbar (Last-Good-Cache aktiv) |
| `ERROR` | `multicast join failed` | Socket konnte Multicast-Gruppe nicht beitreten |
| `DEBUG` | *(alle Datagramm-Details)* | Nur mit `WAYFINDER_LOG_LEVEL=debug` |

### Beispiel-Logeintrag (JSON)

```json
{
  "time": "2026-06-18T14:23:01.456Z",
  "level": "INFO",
  "msg": "feed status changed",
  "previous": "unknown",
  "current": "ok",
  "heartbeats_total": 1
}
```

---

## 10. Betriebsverhalten

### 10.1 Startup-Sequenz

1. Konfiguration laden (Env-Vars → YAML → Defaults)
2. Multicast-Socket öffnen (`FIREFLY_CAT062_GROUP:FIREFLY_CAT062_PORT`)
3. HTTP-Server auf Port 8080 starten (`/health`, `/ready`, `/metrics`)
4. HTTP/WebSocket-Server auf Port 8081 starten (ASD-Frontend, `/ws`)
5. Aeronautical-Service starten (erster Fetch; nicht-blockierend)
6. Empfangs-Goroutine starten (wartet auf Datagramme)
7. Feed-Monitor-Goroutine starten (prüft Staleness ohne Datenstrom)

> **Hinweis:** Nach dem Start antwortet `/health` sofort mit `200`, aber
> `/ready` bleibt `503`, bis der erste CAT065-Heartbeat empfangen wurde.

### 10.2 Graceful Shutdown

Auf `SIGINT` oder `SIGTERM`:

1. Alle WebSocket-Clients werden geschlossen
2. HTTP-Server auf Port 8081 fährt herunter
3. HTTP-Server auf Port 8080 fährt herunter
4. Multicast-Socket wird geschlossen
5. Prozess beendet sich mit Exit-Code 0

Kubernetes `terminationGracePeriodSeconds: 10` ist ausreichend.

### 10.3 Update-Rate und Latenz

Die Track-Update-Rate ist **nicht durch Wayfinder begrenzt** — jedes
empfangene CAT062-Datagramm wird unmittelbar an alle WebSocket-Clients
weitergeleitet. Die End-to-End-Latenz (Firefly-Encoder → Browser-Darstellung)
liegt typischerweise unter 100 ms im LAN.

Fireflys Ausgabetakt ist konfigurierbar (typisch 4–12 s pro Sensor-Scan).
Wayfinder stellt keine eigene Clock und puffert keine Tracks über
Datagramm-Grenzen hinaus.

### 10.4 WebSocket-Backpressure

Wenn ein Browser-Client Nachrichten nicht schnell genug verarbeitet und sein
Send-Channel (intern: 256 Slots) voll ist, wird er **evicted** (entfernt).
Das schützt den Broadcaster vor Head-of-Line-Blocking durch einen langsamen
Client. Das Ereignis wird als Warn-Log und über
`wayfinder_ws_clients_evicted_total` sichtbar.

### 10.5 Zeitstempel und UTC-Mitternacht

ASTERIX CAT062 I062/070 (Time of Day) ist ein 24-Bit-Zähler in 1/128-s-Ticks
seit UTC-Mitternacht. Er **springt bei Mitternacht auf 0 zurück**. Wayfinder
leitet daraus keinen monoton steigenden Zeitstempel über Mitternacht hinaus ab
— ein Sprung von einem großen Wert auf einen kleinen Wert ist kein Fehler,
sondern ein normaler Tageswechsel.

---

## 11. Bekannte Einschränkungen

| Thema | Einschränkung | Workaround / Geplant |
|-------|---------------|----------------------|
| **Multicast auf macOS/Windows** | `network_mode: host` nicht verfügbar in Docker Desktop | Bridge-Netzwerk (siehe `docs/INSTALLATION.md`, Abschnitt 5) |
| **Konfigurierbarer System-Referenzpunkt (I062/100)** | I062/100-Koordinaten beziehen sich auf Fireflys Demo-Ursprung (Frankfurt); für andere Gebiete nur I062/105 (WGS84) nutzbar | Geplant in Fireflys Roadmap |
| **Multicast-Authentifizierung** | UDP-Multicast hat keine eingebaute Absender-Authentifizierung; Schutz liegt auf Netzwerkebene (ADR 0003) | Netz-Isolation; optionaler App-Layer in Planung |
| **Single-Node** | Wayfinder ist nicht für horizontale Skalierung (mehrere Instanzen hinter Load-Balancer) ausgelegt — jede Instanz hält ihren eigenen WebSocket-State | Für ASD typischerweise nicht nötig |
| **Keine Track-History im Backend** | Vergangene Positionen werden nur im Browser-State gehalten; nach einem Browser-Reload ist die Track-History leer | By Design: Browser-State reicht für das ASD-Use-Case |
