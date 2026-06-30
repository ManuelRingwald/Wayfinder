# End-to-End-Abnahme: Zero-Touch-Start bis zum ersten Kunden (EDLV)

> **Zweck.** Ein **durchspielbarer Ablaufplan**: vom Zero-Touch-Start der
> Multi-Tenant-Plattform bis zum fertig eingerichteten ersten Kunden — Mandant,
> Nutzer, Feed, **Sicht (Zentrum + AOI, lat/lon)** und Abo. Beispiel-Flughafen:
> **EDLV (Weeze)** mit **30 NM Radius**. Jeder Schritt hat **Aktion**,
> **erwartetes Ergebnis** und **Prüfschritt** zum Abhaken.
>
> **Betriebsmodus.** Multi-Tenant ist der einzige Modus (ADR 0014): Postgres ist
> Pflicht, die Anmeldung ist immer aktiv. Es gibt keinen DB-losen „Standalone".
>
> **Plattform (wichtig für macOS/Windows, z. B. Mac mini).** Teil A–D (Start +
> Kunden-Einrichtung) laufen **überall** gleich — der Onboarding-Stack nutzt ein
> Bridge-Netz. Nur die **Live-Tracks** (Teil E) sind plattform-abhängig:
> Host-Networking-Multicast funktioniert auf **Docker Desktop (macOS/Windows)
> nicht**. Dort den **Bridge-Weg E-2** nehmen, **nicht** den orchestrierten
> Host-Netz-Stack (E-1, nur Linux).

## Was hier nachgewiesen wird

| # | Behauptung | Wo |
|---|------------|-----|
| 1 | Ein einziger Compose-Befehl bringt Postgres + Wayfinder hoch (Zero-Touch). | Teil A |
| 2 | Der Default-Admin (`admin`/`admin`) wird auto-seeded und **erzwingt** den Passwortwechsel. | Teil B |
| 3 | Vor dem Passwortwechsel ist die Admin-Oberfläche gesperrt (fail-closed). | Teil B |
| 4 | Ein Kunde lässt sich vollständig per Admin-API einrichten: Mandant, Nutzer, Feed, Quelle, Sicht/AOI, Abo. | Teil C |
| 5 | Die Sicht trägt **Zentrum (lat/lon)**, **AOI-Bounding-Box** und **FL-Band** des EDLV-30-NM-Gebiets. | Teil C |
| 6 | Der Kunde sieht **seinen** gescopten Stream; `/ready` und Metriken sind plausibel. | Teil D |
| 7 | (optional) Mit dem orchestrierten Stack entstehen **Live-Tracks**. | Teil E |

## Voraussetzungen

- Docker-Daemon + `docker compose` v2.
- `curl` und (optional) `jq` für die Prüf-Befehle.
- Aus dem **Wayfinder-Repo-Wurzelverzeichnis** ausführen.
- Kein Vorab-Setup nötig — der Onboarding-Stack bringt PostgreSQL mit.

### EDLV-Geodaten (einmal berechnet)

EDLV (Weeze) liegt bei **51,40° N / 6,15° E**. Die Admin-Oberfläche rechnet einen
Radius in **nautischen Meilen (NM)** über `radiusNmToBbox` in eine Bounding-Box um
(60 NM/° Breite; Länge mit `cos(Breite)` skaliert). Für **30 NM**:

```
latΔ = 30 / 60                       = 0,5°
lonΔ = 30 / (60 · cos(51,40°))       ≈ 0,81°
```

| Feld | Wert |
|------|------|
| `center_lat` | `51.40` |
| `center_lon` | `6.15` |
| `aoi.min_lat` | `50.90` |
| `aoi.max_lat` | `51.90` |
| `aoi.min_lon` | `5.34` |
| `aoi.max_lon` | `6.96` |

---

## Teil A — Zero-Touch-Start

| # | Aktion | Erwartetes Ergebnis | Prüfschritt |
|---|--------|---------------------|-------------|
| A1 | `docker compose -f docker-compose.onboarding.yml up --build -d` | `db` und `wayfinder` starten; `db` wird `healthy`, dann startet `wayfinder`. | `docker compose -f docker-compose.onboarding.yml ps` → beide `Up`, `db` `(healthy)`. |
| A2 | Logs ansehen: `docker compose -f docker-compose.onboarding.yml logs wayfinder` | Migrationen laufen; Auto-Seed legt Default-Mandant + Admin an; Server lauscht auf `:8081`/`:8080`. | Im Log erscheinen `multi-tenancy enabled` und eine `auto-seed`-Zeile. |
| A3 | Liveness prüfen: `curl -s localhost:8080/health` | `ok`. | Ausgabe ist `ok`. |
| A4 | Readiness prüfen: `curl -s localhost:8080/ready` | Noch **nicht ready** (kein Feed, keine Clients) → HTTP `503`. | `curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/ready` → `503`. Das ist **erwartet** (frische Instanz). |

> Ein fehlendes `WAYFINDER_DB_URL` würde den Start mit klarer Meldung abbrechen
> (ADR 0014) — im Onboarding-Stack ist es gesetzt, daher startet alles.

---

## Teil B — Erstanmeldung & erzwungener Passwortwechsel

| # | Aktion | Erwartetes Ergebnis | Prüfschritt |
|---|--------|---------------------|-------------|
| B1 | Browser auf **http://localhost:8081/admin** | Login-Maske. | Seite lädt. |
| B2 | Anmelden mit `admin` / `admin` (UI oder curl, siehe unten). | Session-Cookie `wf_session` wird gesetzt; sofort Aufforderung zum Passwortwechsel. | curl: HTTP `204` + `Set-Cookie: wf_session=…`. |
| B3 | **Vor** dem Passwortwechsel eine beliebige Admin-Route aufrufen, z. B. `GET /api/admin/tenants`. | **Gesperrt** (fail-closed): HTTP `403` mit Marker `password_change_required`. | Antwort-Status `403`. |
| B4 | Passwort ändern: `PUT /api/admin/me/password` mit `{current_password,new_password}` (neu ≥ 8 Zeichen). | HTTP `204`; das `must_change_password`-Flag ist gelöscht, die Admin-Oberfläche ist freigeschaltet. | Danach liefert `GET /api/admin/tenants` `200`. |

```bash
# B2 — anmelden, Cookie speichern
curl -s -i -c cookies.txt -X POST localhost:8081/api/login \
  -H 'Content-Type: application/json' \
  -d '{"subject":"admin","password":"admin"}' | head -n1

# B3 — vor dem Wechsel ist alles gesperrt (erwartet: 403)
curl -s -o /dev/null -w "B3 -> %{http_code}\n" -b cookies.txt localhost:8081/api/admin/tenants

# B4 — Passwort setzen (neues Passwort frei wählen, >= 8 Zeichen)
curl -s -o /dev/null -w "B4 -> %{http_code}\n" -b cookies.txt -X PUT localhost:8081/api/admin/me/password \
  -H 'Content-Type: application/json' \
  -d '{"current_password":"admin","new_password":"WechselMich-123"}'

# Kontrolle: jetzt ist die Admin-API offen (erwartet: 200)
curl -s -o /dev/null -w "offen -> %{http_code}\n" -b cookies.txt localhost:8081/api/admin/tenants
```

---

## Teil C — Ersten Kunden einrichten: EDLV (Weeze)

Alle Aufrufe als angemeldeter Admin (Cookie aus Teil B). IDs aus den Antworten
übernehmen (`jq` hilft).

| # | Aktion | Erwartetes Ergebnis | Prüfschritt |
|---|--------|---------------------|-------------|
| C1 | **Mandant anlegen:** `POST /api/admin/tenants` `{"slug":"edlv","name":"EDLV Weeze"}` | HTTP `201` mit `{id,slug,name,status:"active"}`. | `slug` eindeutig; merke `TENANT_ID`. |
| C2 | **Kunden-Nutzer anlegen:** `POST /api/admin/tenants/{TENANT_ID}/users` `{"subject":"edlv-lotse","password":"…","role":"user"}` | HTTP `201` mit `{id,subject,role:"user",status:"active"}`. | `subject` global eindeutig; Passwort ≥ 8 Zeichen. |
| C3 | **Feed anlegen** (Endpoint automatisch zuweisen): `POST /api/admin/feeds` `{"name":"edlv-weeze","region":"Europe","sensor_mix":["SSR","ADS-B"]}` | HTTP `201`; Server vergibt `multicast_group`/`port` aus dem Pool. | Merke `FEED_ID`; Antwort trägt eine zugewiesene Gruppe + Port. |
| C4 | **Quelle setzen** (ADS-B um EDLV): `PUT /api/admin/feeds/{FEED_ID}/sources` mit `adsb_opensky` + EDLV-BBox. | HTTP `200`; `coverage_bbox` wird zurückgemeldet (aus der Quell-BBox + Marge abgeleitet). | Antwort enthält die gesetzte Quelle und eine `coverage_bbox`. |
| C5 | **Sicht setzen** (Zentrum EDLV + 30-NM-AOI + FL-Band): `PUT /api/admin/tenants/{TENANT_ID}/view`. | HTTP `200`; Zentrum `51.40/6.15`, AOI `50.90…51.90 / 5.34…6.96`, `fl_min 0`, `fl_max 450`. | Antwort spiegelt die gesetzten Werte. |
| C6 | **Feed abonnieren:** `POST /api/admin/tenants/{TENANT_ID}/subscriptions` `{"feed_id":FEED_ID}` | HTTP `204`; der Mandant ist auf den EDLV-Feed gescopt. | Erneuter Aufruf ist idempotent (`204`). |

```bash
# C1 — Mandant
TENANT_ID=$(curl -s -b cookies.txt -X POST localhost:8081/api/admin/tenants \
  -H 'Content-Type: application/json' \
  -d '{"slug":"edlv","name":"EDLV Weeze"}' | jq -r .id)
echo "TENANT_ID=$TENANT_ID"

# C2 — Kunden-Nutzer
curl -s -b cookies.txt -X POST localhost:8081/api/admin/tenants/$TENANT_ID/users \
  -H 'Content-Type: application/json' \
  -d '{"subject":"edlv-lotse","email":"lotse@edlv.example","password":"Weeze-30nm!","role":"user"}' | jq .

# C3 — Feed (Endpoint auto-allokiert)
FEED_ID=$(curl -s -b cookies.txt -X POST localhost:8081/api/admin/feeds \
  -H 'Content-Type: application/json' \
  -d '{"name":"edlv-weeze","region":"Europe","sensor_mix":["SSR","ADS-B"]}' | jq -r .id)
echo "FEED_ID=$FEED_ID"

# C4 — Quelle: ADS-B im EDLV-30-NM-Gebiet
curl -s -b cookies.txt -X PUT localhost:8081/api/admin/feeds/$FEED_ID/sources \
  -H 'Content-Type: application/json' \
  -d '{"sources":[{"type":"adsb_opensky","bbox":{"min_lat":50.90,"max_lat":51.90,"min_lon":5.34,"max_lon":6.96}}]}' | jq .

# C5 — Sicht: Zentrum EDLV + 30-NM-AOI + FL000..FL450
curl -s -b cookies.txt -X PUT localhost:8081/api/admin/tenants/$TENANT_ID/view \
  -H 'Content-Type: application/json' \
  -d '{"center_lat":51.40,"center_lon":6.15,"zoom":9,
       "aoi":{"min_lat":50.90,"max_lat":51.90,"min_lon":5.34,"max_lon":6.96},
       "fl_min":0,"fl_max":450}' | jq .

# C6 — Feed abonnieren
curl -s -o /dev/null -w "C6 -> %{http_code}\n" -b cookies.txt \
  -X POST localhost:8081/api/admin/tenants/$TENANT_ID/subscriptions \
  -H 'Content-Type: application/json' \
  -d "{\"feed_id\":$FEED_ID}"
```

> **Validierungs-Regeln, die der Server durchsetzt** (für eigene Experimente):
> Feed `multicast_group` und `port` müssen **beide oder keiner** gesetzt sein
> (keiner → Auto-Vergabe). Flächenquellen (`adsb_opensky`/`flarm_aprs`) brauchen
> eine `bbox`; echte Radar-Quellen (`radar_asterix`) brauchen `sac`/`sic`. Ohne
> das `multi_feed`-Entitlement darf ein Mandant **höchstens einen** Feed
> abonnieren (zweiter → `409`).
>
> **Echte Live-Daten** (kein Demo): **`adsb_opensky`** (ADS-B über das
> OpenSky-Netz, anonym oder mit OAuth2-Client-Credentials) **und**
> **`flarm_aprs`** (FLARM/Segelflug über das Open Glider Network, OGN/APRS) sind
> in Firefly **produktiv** nutzbar — beide liefern echten Verkehr rund um EDLV;
> mehrere Quellen pro Feed sind erlaubt. (Ingestion läuft auf der Firefly-Seite,
> vom Orchestrator je Feed gestartet.)

---

## Teil D — Verifikation aus Kundensicht

Aus Kundensicht ist das ein **Browser-Vorgang**: URL öffnen, anmelden, Karte
sehen. Die `curl`-Befehle hier sind nur das **automatisierbare Äquivalent**
desselben `/api/login`-Endpunkts (für den Skript-Durchlauf der Abnahme).

| # | Aktion | Erwartetes Ergebnis | Prüfschritt |
|---|--------|---------------------|-------------|
| D1 | **Im Browser anmelden:** **http://localhost:8081/admin** öffnen → Login-Maske „Anmelden" (Benutzername/Passwort) → `edlv-lotse` + Passwort. | Anmeldung erfolgreich, Session-Cookie `wf_session` gesetzt. | Ein reiner Kunden-Nutzer (Rolle `user`) sieht danach „Kein Zugriff auf die Administration" — **erwartet**; der Login (Cookie) hat trotzdem geklappt. |
| D1-alt | **Headless** statt Browser (Skript): `POST /api/login` `{"subject":"edlv-lotse","password":"Weeze-30nm!"}`. | HTTP `204` + Cookie `wf_session`. | Genau der Aufruf, den die Login-Maske intern macht. |
| D2 | **Karte öffnen:** **http://localhost:8081/** (mit bestehender Sitzung). | Karte zentriert auf EDLV (51,40/6,15), Zoom 9; der Kunde sieht **nur** seinen gescopten Strom. | Sichtbarer Ausschnitt = Weeze. |
| D3 | `GET /api/map-config` (als Kunde). | JSON mit `center_lat 51.40`, `center_lon 6.15`. | Werte stimmen mit C5 überein. |
| D4 | Feed-Gesundheit (Admin): `GET /api/admin/feeds/health`. | Eintrag für `FEED_ID`; `color:"red"`/`ever_seen:false`, solange kein CAT065-Heartbeat ankommt. | Ohne Live-Sender ist „rot/never seen" **erwartet** (siehe Teil E). |
| D5 | Readiness: `curl -s localhost:8080/ready`. | Bleibt `503`, solange weder Tracks noch Clients da sind. | Erst mit Live-Feed **oder** verbundenem `/ws`-Client wird `200` möglich. |

> **Offener UX-Punkt (Stand heute).** Die Login-Maske liegt in der
> **`/admin`**-Ansicht; die Karte unter **`/`** hat noch **keinen** eigenen
> Login-Bildschirm — sie setzt eine bestehende Sitzung voraus. Ein
> kundenseitiger Login direkt unter `/` (Lotse öffnet `localhost:8081`, bekommt
> sofort eine Login-Maske, dann die Karte) ist ein sinnvoller nächster Schritt
> (eigenes Wayfinder-Issue).

> **Warum bleibt die Karte leer?** Der Onboarding-Stack nutzt **Bridge-Netz** —
> UDP-Multicast (CAT062) traversiert es nicht. Der Feed ist katalogisiert und der
> Mandant ist korrekt gescopt; es fehlt nur ein Sender. Live-Tracks liefert Teil E.

---

## Teil E — (optional) Live-Tracks mit Firefly

Für echte Tracks braucht es einen CAT062-Sender auf der Multicast-Gruppe des
**abonnierten** Feeds. Der Weg hängt von der Plattform ab.

### E-1 — Linux: orchestrierter Stack (Auto-Spawn, empfohlen)

`docker compose -f docker-compose.orchestrated.yml up --build` — Postgres +
Wayfinder (`builtin`) + **Orchestrator** in Host-Netz. Der Orchestrator startet je
abonniertem Feed automatisch eine Firefly-Instanz **auf der Feed-Gruppe**;
Multicast trifft direkt beim Server ein. Den Kunden wie in Teil C einrichten
(Feed-Endpoint ruhig **auto-allokieren** — der Orchestrator trifft die Gruppe
selbst). `scripts/e2e-orchestrated.sh` automatisiert das (Modi `--mode scene`
offline / `--mode opensky-anon` Live-ADS-B).

| # | Aktion | Erwartetes Ergebnis | Prüfschritt |
|---|--------|---------------------|-------------|
| E1.1 | Orchestrierten Stack starten (Linux). | Postgres + Server + Orchestrator laufen; nur der Orchestrator mountet den Docker-Socket. | `docker ps` zeigt alle drei; `docker inspect` bestätigt den Socket nur am Orchestrator. |
| E1.2 | Feed abonnieren (Teil C bzw. Skript-Seed). | Orchestrator spawnt `wayfinder-firefly-feed-<id>`. | `docker ps` zeigt den Tracker mit Label `wayfinder.feed_id`. |
| E1.3 | Tracks prüfen. | `wayfinder_cat062_tracks_received_total > 0`; der abonnierte Kunde sieht Tracks. | `curl -s localhost:8080/metrics \| grep cat062_tracks_received_total`. |
| E1.4 | Abo entfernen. | Tracker-Container gestoppt/entfernt (Orphan-Cleanup). | `docker ps` zeigt den Tracker nicht mehr. |

> ⚠️ **Docker-Socket = root-äquivalent (ADR 0012 §6).** Nur der Orchestrator
> bekommt ihn; der browser-zugewandte Server nie. Den Orchestrator-Host als
> hochwertige Vertrauensgrenze behandeln (Netz-Isolation, restriktiver Zugang).

### E-2 — macOS / Windows (Docker Desktop, z. B. Mac mini): Bridge-Master-Compose

Auf Docker Desktop bindet `network_mode: host` nur an die interne Linux-VM — der
orchestrierte Stack (E-1) und ein separat laufendes Firefly sehen sich **nicht**,
Multicast „nach außen" scheitert. **Lösung:** Firefly, Postgres und Wayfinder in
**einem** benutzerdefinierten Bridge-Netz; **innerhalb** desselben Docker-Netzes
funktioniert Multicast zwischen den Containern. Das fertige Master-Compose steht
in **`DOCKER.md`** (Abschnitt „macOS/Windows") — es um den `db`-Service erweitert
und multi-tenant (`WAYFINDER_DB_URL` + `WAYFINDER_AUTH_MODE: builtin`).

> **Produktiv ≠ dieser Test.** Im Produktivbetrieb (Linux, E-1) vergibt Wayfinder
> die Feed-Multicast-Gruppe **automatisch** und der Orchestrator startet Firefly
> genau auf dieser Gruppe — man legt nur den Feed an, sonst nichts. Der manuelle
> explizite Endpoint unten ist **allein** ein Docker-Desktop-Behelf (dort läuft
> der Host-Netz-Orchestrator nicht): eine Grenze der Mac-Test-Umgebung, **keine**
> Produkteigenschaft.

**Zwei Unterschiede zu Teil C/E-1** (es gibt hier **keinen** Orchestrator —
Firefly ist ein **fester externer Sender**):

1. **Feed mit explizitem Endpoint anlegen** (statt auto-allokieren), passend zu
   Fireflys Sender (Default `239.255.0.62:8600`):
   ```bash
   curl -s -b cookies.txt -X POST localhost:8081/api/admin/feeds \
     -H 'Content-Type: application/json' \
     -d '{"name":"edlv-weeze","multicast_group":"239.255.0.62","port":8600,
          "region":"Europe","sensor_mix":["SSR","ADS-B"]}'
   ```
2. **Firefly im selben Compose** mit `FIREFLY_CAT062_ENABLED=true` (sendet auf
   `FIREFLY_CAT062_GROUP`, Default `239.255.0.62:8600`).

| # | Aktion | Erwartetes Ergebnis | Prüfschritt |
|---|--------|---------------------|-------------|
| E2.1 | Master-Compose starten: `db` + `firefly-server` + `wayfinder` in `radar-net`. | Alle drei laufen; Wayfinder erreicht Postgres über DNS-Name `db`. | `docker compose ps` → alle `Up`. |
| E2.2 | Teil B (Passwortwechsel) + Teil C, **Feed mit explizitem `239.255.0.62:8600`**, Kunde abonniert. | Feed auf Fireflys Gruppe; Mandant gescopt. | Feed-Antwort trägt `239.255.0.62:8600`. |
| E2.3 | Tracks prüfen. | `wayfinder_cat062_tracks_received_total > 0`. | `curl -s localhost:8080/metrics \| grep cat062_tracks_received_total`. |

> ⚠️ **Demo-Szene ≠ EDLV.** Fireflys eingebaute Szene ist **Frankfurt** (kein
> Weeze). Mit `FIREFLY_SCENE=frankfurt` liegen die Tracks ~150 km südöstlich von
> EDLV — **außerhalb** der 30-NM-AOI aus Teil C; der View-Filter blendet sie aus
> (Tracks fließen, Karte bleibt leer). Zwei Wege zu sichtbaren Tracks:
> **(a) Schnelltest:** die Sicht (C5) testweise auf Frankfurt (`50.0379/8.5622`)
> setzen **oder** die `aoi` weglassen — dann erscheinen die Demo-Tracks.
> **(b) Echter EDLV-Verkehr:** Firefly mit `FIREFLY_MODE=live` + `FIREFLY_SOURCES`
> (eine `adsb_opensky`-Quelle mit der EDLV-BBox `50.90…51.90 / 5.34…6.96`) statt
> der Szene fahren — liefert echten ADS-B-Verkehr rund um Weeze (Netz-Egress nötig).

---

## Aufräumen

```bash
# Onboarding-Stack inkl. Datenbank-Volume entfernen (Start bei 0):
docker compose -f docker-compose.onboarding.yml down -v

# Orchestrierter Stack (falls in Teil E benutzt):
docker compose -f docker-compose.orchestrated.yml down -v --remove-orphans
docker ps -aq --filter 'label=wayfinder.managed=true' | xargs -r docker rm -f
```

## Bekannte Grenzen

- **Onboarding-Stack = Bridge-Netz:** keine Multicast-Tracks ohne externen Sender
  (Teil A–D prüfen die **Einrichtung**, nicht die Live-Daten). Live-Tracks: Teil E.
- **Docker Desktop (macOS/Windows):** Host-Networking-Multicast funktioniert dort
  i. d. R. nicht — der orchestrierte Stack braucht einen Linux-Host. Für Live-Tracks
  auf Docker Desktop das Bridge-Master-Compose aus `DOCKER.md` nutzen.
- **Diese Repo-CI/Sandbox:** ohne laufenden Docker-Daemon ist nur
  `docker compose config` / der Binär-Build verifizierbar; der echte Lauf gehört
  auf einen Docker-Host.
