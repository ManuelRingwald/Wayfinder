# End-to-End-Abnahme: Zero-Touch-Start bis erster Kunde (Flughafen EDLV / Weeze)

> **Zweck:** Ein **durchspielbarer Ablaufplan**, der die komplette Inbetriebnahme
> nachweist — vom **Zero-Touch-Multitenant-Start** über das **Einrichten des ersten
> Kunden** (Mandant, Zugang, Feed, Quelle, **AOI**) bis zu **sichtbaren Tracks**.
> Jeder Schritt hat eine **Aktion**, ein **erwartetes Ergebnis** und einen
> **Prüfschritt** (UI **und** API/CLI). Zum Abhaken gedacht.
>
> **Beispiel-Kunde dieses Testfalls:** ein Tower-Betrieb am **Flughafen Weeze
> (EDLV / NRN)** mit einer **AOI von 30 NM** um den Platz.

---

## 0. Testdaten (einmal festlegen, überall wiederverwenden)

| Größe | Wert | Hinweis |
|-------|------|---------|
| Mandant (Kunde) — Slug / Name | `weeze-tower` / `Weeze Tower` | DNS-Label-Form (klein, `a–z0–9-`) |
| Zugang (Lotse) — Subject / Passwort | `lotse-weeze` / `Lotse!2026` | Rolle `user` (Lotse); `admin` = Mandanten-Admin |
| Feed — Name | `edlv-adsb` | eindeutig |
| Feed — Multicast-Gruppe / Port | `239.255.0.62` / `8600` | = Fireflys Default |
| Quelle — Typ | `adsb_opensky` | Flächenquelle (braucht `bbox`) |
| **Flughafen EDLV — Referenzpunkt** | **lat `51.6024`, lon `6.1422`** | Aerodrome Reference Point (Weeze) |
| **AOI-Radius** | **30 NM** | clientseitig → Bounding-Box |
| **AOI-Bounding-Box** (aus 30 NM um EDLV) | `min_lat 51.1024`, `min_lon 5.3372`, `max_lat 52.1024`, `max_lon 6.9472` | siehe Phase 6, Rechenweg |
| Feed-Quell-`bbox` (Ingestion, ⊇ AOI) | `min_lat 51.0`, `min_lon 5.2`, `max_lat 52.2`, `max_lon 7.1` | etwas größer als die AOI |
| FL-Band | `fl_min 0`, `fl_max 200` | `× 100 ft` → Boden bis FL200; fail-open |

> **Maßgebliche Quellen:** Eingangs-Kontrakt `docs/source-input-contract.md`
> (Firefly-Repo, v1.3.0) für die Quell-Felder; `docs/INSTALLATION.md` für die
> Env-Variablen; CAT062-Vertrag `docs/ICD-CAT062.md` (Firefly).

---

## 1. Voraussetzungen

- **Docker** + **`docker compose` v2** auf dem Abnahme-Host.
- Ein Terminal mit **`curl`** und **`jq`** (für die API-Prüfschritte; UI-Weg geht auch ohne).
- **Browser** für die Admin-UI und das ASD.
- **Für sichtbare Tracks (Phase 8):** ein **Linux-Host** (Host-Networking +
  UDP-Multicast) und ein **Firefly-Image**. ⚠️ Auf **Docker Desktop (macOS/Windows)**
  funktioniert Host-Networking-Multicast i. d. R. **nicht** — die Phasen 1–7 (Setup +
  Prüfung der Konfiguration) laufen dort vollständig, nur die **Live-Tracks** in
  Phase 8 brauchen einen Linux-Host (siehe `DOCKER.md`).

> **Zwei Stacks — bewusst getrennt:**
> - **`docker-compose.onboarding.yml`** — der **Multitenant-Stack** (Postgres +
>   Wayfinder, Builtin-Login, Zero-Touch-Admin). **Das ist der Stack für Phasen 0–7.**
>   Bridge-Networking ⇒ ohne separaten Firefly bleibt die Karte leer.
> - **`docker-compose.orchestrated.yml`** — die **Orchestrator-Variante**, die je
>   Feed automatisch einen Firefly-Container startet (Host-Net, Multicast). Single-
>   Tenant (`AUTH_MODE=none`). Für die **Track-Demonstration** in Phase 8 (Option B).

---

## 2. Konventionen für die API-Prüfschritte

Alle `curl`-Beispiele nutzen eine Cookie-Jar (Builtin-Session). Einmal pro Sitzung:

```bash
API=http://localhost:8081      # Browser-zugewandter Server (Admin-API, /ws, ASD)
OBS=http://localhost:8080      # Health / Readiness / Metrics (separater Probe-Port)
COOKIES=$(mktemp)
```

UI-Weg und API-Weg sind **gleichwertig** — wähle einen. Die Prüfschritte gelten für beide.

---

## 3. Prüfpunkt-Übersicht (zum Abhaken)

| # | Phase | Behauptung | Schnellprüfung |
|---|-------|------------|----------------|
| 1 | Start | Stack kommt zero-touch hoch | `GET $OBS/health` → 200 |
| 2 | Login | Default-Admin + erzwungener Passwortwechsel | Login `admin`/`admin`, dann Passwortwechsel-Gate |
| 3 | Mandant | Kunde angelegt | `GET $API/api/admin/tenants` zeigt `weeze-tower` |
| 4 | Zugang | Lotse angelegt | `GET …/tenants/{id}/users` zeigt `lotse-weeze` |
| 5 | Feed | Feed + Quelle konfiguriert | `GET …/feeds/{id}/sources` zeigt `adsb_opensky` + bbox |
| 6 | Abo | Mandant abonniert Feed | `GET …/tenants/{id}/subscriptions` zeigt den Feed |
| 7 | AOI | Sicht (EDLV, 30 NM) gesetzt | `GET …/tenants/{id}/view` zeigt Center + AOI-Box + FL |
| 8 | Scoping | Lotse sieht nur seinen Mandanten | Login als `lotse-weeze` → nur abonnierter Feed |
| 9 | Tracks | Flugzeuge im AOI erscheinen | ASD-Karte + `wayfinder_cat062_tracks_received_total` > 0 |
| 10 | Teardown | Sauber „bei 0" | keine Container/Volumes übrig |

---

## Phase 0 — Zero-Touch-Multitenant-Start

**Aktion**
```bash
docker compose -p wayfinder -f docker-compose.onboarding.yml up -d --build
```
> Das `-p wayfinder` pinnt den **Projektnamen** (sonst hängt der Volume-Namespace am
> Ordnernamen — wichtig fürs saubere Teardown in Phase 10).

**Erwartetes Ergebnis**
- Postgres und Wayfinder starten; Wayfinder seedet **automatisch** einen Default-
  Mandanten und einen Default-Admin (`admin`/`admin`, ADR 0011). **Kein** manueller
  `bootstrap`-Schritt nötig.
- ASD/Admin auf Port **8081**, Health/Metrics auf Port **8080**.

**Prüfschritt**
```bash
docker compose -p wayfinder -f docker-compose.onboarding.yml ps   # db + wayfinder „running/healthy"
curl -fsS $OBS/health                                             # {"status":"ok"}
```
- ✅ **Erwartet:** beide Container `healthy`; `/health` → `200`.
- ℹ️ `GET $OBS/ready` darf hier noch **503** liefern (`feed_stale`/keine Daten) — das ist
  vor dem ersten Feed normal und kein Fehler.

---

## Phase 1 — Admin-Login & erzwungener Passwortwechsel

**Aktion (UI)** — `http://localhost:8081/admin` öffnen, mit `admin` / `admin` anmelden.
Die Oberfläche **verlangt sofort** ein neues Passwort, bevor irgendetwas anderes
erreichbar ist (`must_change_password`, serverseitig erzwungen).

**Aktion (API)**
```bash
# 1) Login → Session-Cookie (wf_session)
curl -fsS -c "$COOKIES" -X POST $API/api/login \
  -H 'Content-Type: application/json' \
  -d '{"subject":"admin","password":"admin"}'

# 2) Pflicht-Passwortwechsel (hebt das Gate für dieselbe Session auf)
curl -fsS -b "$COOKIES" -X PUT $API/api/admin/me/password \
  -H 'Content-Type: application/json' \
  -d '{"current_password":"admin","new_password":"Admin!Stark2026"}'
```

**Erwartetes Ergebnis**
- Vor dem Wechsel sind **nur** `whoami`, `me` und `me/password` erreichbar; jeder
  andere Admin-Aufruf → **403 `password_change_required`**.
- Nach dem Wechsel ist das Gate aufgehoben (`must_change_password=false`), die Session
  bleibt gültig.

**Prüfschritt**
```bash
curl -fsS -b "$COOKIES" $API/api/admin/whoami | jq    # role: platform-admin/super_admin
# Gegenprobe: vor dem Wechsel hätte z. B. /api/admin/tenants 403 geliefert.
curl -fsS -b "$COOKIES" $API/api/admin/tenants | jq   # jetzt 200 (Liste)
```
- ✅ **Erwartet:** `whoami` zeigt die Admin-Identität; `tenants` liefert `200`.

---

## Phase 2 — Ersten Kunden (Mandant) anlegen

**Aktion (UI)** — Admin → **Mandanten** → „Mandant anlegen": Slug `weeze-tower`,
Name `Weeze Tower`.

**Aktion (API)**
```bash
curl -fsS -b "$COOKIES" -X POST $API/api/admin/tenants \
  -H 'Content-Type: application/json' \
  -d '{"slug":"weeze-tower","name":"Weeze Tower"}' | tee /tmp/tenant.json | jq
TENANT_ID=$(jq -r '.id' /tmp/tenant.json)
echo "TENANT_ID=$TENANT_ID"
```

**Erwartetes Ergebnis** — HTTP `201`, JSON mit `id`, `slug`, `name`, `status:"active"`.

**Prüfschritt**
```bash
curl -fsS -b "$COOKIES" $API/api/admin/tenants | jq '.[] | select(.slug=="weeze-tower")'
```
- ✅ **Erwartet:** der Mandant erscheint mit `status:"active"`.
- 🗄️ **DB-Gegenprobe:** `SELECT id,slug,name,status FROM tenants WHERE slug='weeze-tower';`

---

## Phase 3 — Zugang (Lotse) für den Kunden anlegen

**Aktion (UI)** — Mandant `weeze-tower` öffnen → **Zugänge** → „Zugang anlegen":
Subject `lotse-weeze`, Passwort `Lotse!2026`, Rolle **`user`** (Lotse). (`admin` =
Mandanten-Admin mit Konfig-Rechten innerhalb des Mandanten.)

**Aktion (API)**
```bash
curl -fsS -b "$COOKIES" -X POST $API/api/admin/tenants/$TENANT_ID/users \
  -H 'Content-Type: application/json' \
  -d '{"subject":"lotse-weeze","email":"lotse@weeze.example","password":"Lotse!2026","role":"user"}' | jq
```

**Erwartetes Ergebnis** — HTTP `201`, JSON mit `subject:"lotse-weeze"`, `role:"user"`,
`status:"active"`.

**Prüfschritt**
```bash
curl -fsS -b "$COOKIES" $API/api/admin/tenants/$TENANT_ID/users | jq '.[].subject'
```
- ✅ **Erwartet:** `lotse-weeze` ist gelistet.
- 🗄️ **DB:** `SELECT subject,role,status FROM users WHERE tenant_id=$TENANT_ID;`

---

## Phase 4 — Feed anlegen und Quelle konfigurieren

Ein **Feed** ist ein Upstream-Track-Strom; seine **Quelle** beschreibt Firefly-
agnostisch, *woraus* getrackt wird. Für EDLV nehmen wir eine **`adsb_opensky`**-
Flächenquelle mit einer `bbox`, die den Platz großzügig abdeckt.

**Aktion (UI)** — Admin → **Feeds** → „Feed anlegen": Name `edlv-adsb`, Multicast-
Gruppe `239.255.0.62`, Port `8600`. Dann Feed öffnen → **Quellen** → Quelle vom Typ
`adsb_opensky` mit der bbox hinzufügen.

**Aktion (API)**
```bash
# 4a) Feed anlegen
curl -fsS -b "$COOKIES" -X POST $API/api/admin/feeds \
  -H 'Content-Type: application/json' \
  -d '{"name":"edlv-adsb","multicast_group":"239.255.0.62","port":8600,"region":"edlv","sensor_mix":["ADS-B"]}' \
  | tee /tmp/feed.json | jq
FEED_ID=$(jq -r '.id' /tmp/feed.json); echo "FEED_ID=$FEED_ID"

# 4b) Quelle setzen (adsb_opensky, bbox um EDLV ⊇ AOI)
curl -fsS -b "$COOKIES" -X PUT $API/api/admin/feeds/$FEED_ID/sources \
  -H 'Content-Type: application/json' \
  -d '{
    "sources":[
      {"type":"adsb_opensky",
       "bbox":{"min_lat":51.0,"min_lon":5.2,"max_lat":52.2,"max_lon":7.1}}
    ]
  }' | jq
```

**Erwartetes Ergebnis**
- `POST /feeds` → `201` mit `id`, `multicast_group`, `port`, `source_count:0`.
- `PUT …/sources` → `200`; die Quelle ist mit `type:"adsb_opensky"` + `bbox` gespeichert.
- ℹ️ Eine `adsb_opensky`-Quelle **darf kein** `sac`/`sic` haben (nur `radar_asterix`);
  fehlende/inkonsistente Felder → **Startfehler/Validierungsfehler** (kein stilles Ignorieren).

> **Anonym vs. authentifiziert:** Ohne `cred_ref` zieht Firefly OpenSky **anonym**
> (gedrosselt). Für höheres Limit eine `cred_ref` setzen und das Secret separat über
> `PUT /api/admin/feeds/{id}/secrets/{ref}` hinterlegen (`client_id:client_secret`,
> OAuth2). Das Secret steht **nie** im Quell-JSON.

**Prüfschritt**
```bash
curl -fsS -b "$COOKIES" $API/api/admin/feeds/$FEED_ID/sources | jq
```
- ✅ **Erwartet:** ein Quell-Eintrag `adsb_opensky` mit der bbox.
- 🗄️ **DB:** `SELECT id,name,multicast_group,port,source_config FROM feeds WHERE id=$FEED_ID;`

---

## Phase 5 — Mandant auf den Feed abonnieren

**Aktion (UI)** — Mandant `weeze-tower` → **Feeds** → „Feed zuweisen" → `edlv-adsb`.

**Aktion (API)**
```bash
curl -fsS -b "$COOKIES" -X POST $API/api/admin/tenants/$TENANT_ID/subscriptions \
  -H 'Content-Type: application/json' \
  -d "{\"feed_id\":$FEED_ID}" | jq
```

**Erwartetes Ergebnis** — HTTP `201`; die Zuordnung Mandant↔Feed besteht (idempotent —
doppeltes Abo ist ein No-Op).

**Prüfschritt**
```bash
curl -fsS -b "$COOKIES" $API/api/admin/tenants/$TENANT_ID/subscriptions | jq
```
- ✅ **Erwartet:** der Feed `edlv-adsb` ist im Abo des Mandanten.
- 🗄️ **DB:** `SELECT tenant_id,feed_id FROM subscriptions WHERE tenant_id=$TENANT_ID;`

---

## Phase 6 — Sicht / AOI des Kunden setzen (EDLV, 30 NM)

Die **AOI** ist die **harte serverseitige Daten-Minimierungsgrenze**: Tracks außerhalb
werden **vor** dem Client verworfen (keine reine Anzeigepräferenz). In der UI gibst du
**Zentrum + Radius (NM)** ein; der Client rechnet daraus die Bounding-Box (`geo.js:
radiusNmToBbox`, 60 NM je Breitengrad, Länge ÷ cos(lat)).

**Rechenweg (Kontrolle):** 30 NM um EDLV (51.6024 / 6.1422):
- `latΔ = 30/60 = 0.5°` → `min_lat 51.1024`, `max_lat 52.1024`
- `lonΔ = 30/(60·cos 51.6024°) = 30/(60·0.6211) = 0.8050°` → `min_lon 5.3372`, `max_lon 6.9472`

**Aktion (UI)** — Mandant `weeze-tower` → **Standard-Ansicht**:
Zentrum Breite `51.6024`, Länge `6.1422`, **Radius (NM) `30`**, Zoom `10`,
FL min `0`, FL max `200` → „Ansicht speichern".

**Aktion (API)** — die UI sendet bereits die fertige Box; per API direkt:
```bash
curl -fsS -b "$COOKIES" -X PUT $API/api/admin/tenants/$TENANT_ID/view \
  -H 'Content-Type: application/json' \
  -d '{
    "center_lat":51.6024,"center_lon":6.1422,"zoom":10,
    "aoi":{"min_lat":51.1024,"min_lon":5.3372,"max_lat":52.1024,"max_lon":6.9472},
    "fl_min":0,"fl_max":200
  }' | jq
```

**Erwartetes Ergebnis** — HTTP `200`; die Sicht ist gespeichert. Verbundene
WS-Clients dieses Mandanten werden **live neu gefiltert** (kein Reconnect nötig, WF2-33).

**Prüfschritt**
```bash
curl -fsS -b "$COOKIES" $API/api/admin/tenants/$TENANT_ID/view | jq
```
- ✅ **Erwartet:** Center = EDLV, AOI-Box wie berechnet, `fl_min/fl_max` = 0/200.
- 🗄️ **DB:** `SELECT center_lat,center_lon,zoom,aoi,fl_min,fl_max FROM view_configs WHERE tenant_id=$TENANT_ID AND user_id IS NULL;`
- 💡 **Einheit prüfen:** FL `× 100 ft` → `fl_max 200` = FL200 = 20 000 ft. **Fail-open:**
  Tracks **ohne** gemeldete Flugfläche werden trotzdem zugestellt.

---

## Phase 7 — Kunden-Login & Mandanten-Scoping prüfen

**Aktion** — In einem **separaten** Browser/Inkognito (oder eigener Cookie-Jar) als
`lotse-weeze` / `Lotse!2026` anmelden und das ASD `http://localhost:8081/` öffnen.

```bash
LOTSE=$(mktemp)
curl -fsS -c "$LOTSE" -X POST $API/api/login \
  -H 'Content-Type: application/json' \
  -d '{"subject":"lotse-weeze","password":"Lotse!2026"}'
curl -fsS -b "$LOTSE" $API/api/admin/whoami | jq    # tenant = weeze-tower, role = user
```

**Erwartetes Ergebnis**
- Der Lotse sieht **ausschließlich** Daten seines Mandanten (nur der abonnierte Feed,
  nur Tracks innerhalb seiner AOI + FL-Band).
- Die Karte startet auf dem in Phase 6 gesetzten Zentrum (EDLV).

**Prüfschritt**
- ✅ **Erwartet:** `whoami` zeigt `tenant=weeze-tower`, `role=user`. Ein Lotse hat
  **keine** Admin-Rechte (Konfig-Routen → 403) — die Mandanten-Grenze zieht der Server
  autoritativ, nicht die UI.

---

## Phase 8 — Tracks für EDLV sichtbar machen

> **Wichtig — der Multicast-Hinweis:** Der Onboarding-Stack nutzt **Bridge-
> Networking**; UDP-Multicast (CAT062 von Firefly) **durchquert Bridge-Netze nicht**,
> die Karte bleibt **leer**. Das ist erwartet. Für echte Tracks muss Firefly seine
> CAT062 dorthin senden, wo Wayfinder lauscht. Zwei Wege:

### Option A — Firefly live neben Wayfinder (Host-Net, Linux)

Firefly als eigene Instanz mit einer ADS-B-Quelle um EDLV starten (Host-Networking,
damit das Multicast Wayfinder erreicht):
```bash
# im Firefly-Repo, Linux-Host:
docker run --rm --network host \
  -e FIREFLY_MODE=live \
  -e FIREFLY_CAT062_ENABLED=true \
  -e FIREFLY_CAT062_GROUP=239.255.0.62 -e FIREFLY_CAT062_PORT=8600 \
  -e 'FIREFLY_SOURCES=[{"type":"adsb_opensky","bbox":{"min_lat":51.0,"min_lon":5.2,"max_lat":52.2,"max_lon":7.1}}]' \
  firefly:latest
```
Wayfinder muss dafür ebenfalls am selben Netz lauschen — siehe das **Master-Compose**
in `DOCKER.md` (Firefly + Wayfinder, Host-Net). Ohne Live-Quelle geht auch eine
Demo-Szene (`FIREFLY_SCENE=…`), die zeigt allerdings keine EDLV-Flugzeuge.

### Option B — Orchestrator spawnt Firefly automatisch (orchestrated-Stack)

Genau die Auto-Orchestrierungs-Kette: **Feed abonnieren → Orchestrator startet einen
Firefly-Container → CAT062 → ASD**. Spawnt aus dem Feed-`source_config` automatisch:
```bash
docker compose -p wayfinder -f docker-compose.orchestrated.yml up -d --build
# (Single-Tenant, AUTH_MODE=none; Firefly-Image lokal: docker build -t firefly:latest . im Firefly-Repo)
```
Der Orchestrator legt je abonniertem Feed einen Container `wayfinder-firefly-feed-<id>`
an (Label `wayfinder.managed=true`).

**Erwartetes Ergebnis** — Flugzeuge **innerhalb** der 30-NM-AOI um EDLV erscheinen auf
der Karte; Tracks **außerhalb** werden serverseitig verworfen (AOI-Grenze).

**Prüfschritt**
```bash
# Spawn (nur Option B):
docker ps --filter "label=wayfinder.managed=true" --format '{{.Names}}'   # wayfinder-firefly-feed-<id>

# Datenfluss (beide Optionen), auf dem Observability-Port:
curl -fsS $OBS/metrics | grep -E 'wayfinder_cat062_(blocks|tracks)_received_total|wayfinder_feed_stale|wayfinder_tracks_current'
curl -fsS $OBS/ready | jq      # jetzt {"status":"ready", blocks>0, feed_stale:false}

# Feed-Ampel im Admin:
curl -fsS -b "$COOKIES" $API/api/admin/feeds/health | jq
```
- ✅ **Erwartet:** `wayfinder_cat062_blocks_received_total` und
  `…_tracks_received_total` **> 0**, `wayfinder_feed_stale 0`; `/ready` → `200`;
  Feed-Ampel **grün**; im ASD bewegen sich Tracks im EDLV-Umkreis.
- ❌ **Leer trotz allem?** Siehe „Fehlerbilder" unten (Multicast/Host-Net, Image, Quelle).

---

## Phase 9 — Negativ-/Robustheits-Checks (optional, empfohlen)

| Check | Aktion | Erwartetes Ergebnis |
|-------|--------|---------------------|
| AOI greift hart | AOI klein setzen (z. B. Radius 2 NM) | weit entfernte Flugzeuge verschwinden serverseitig |
| FL-Band fail-open | `fl_min 300` setzen | Tracks **ohne** Mode-C bleiben sichtbar; mit FL < 300 verschwinden |
| Mandanten-Isolation | zweiten Mandanten **ohne** Abo anlegen | dessen Lotse sieht **keine** EDLV-Tracks |
| Feed-Staleness | Firefly stoppen | nach `WAYFINDER_FEED_STALE_TIMEOUT` (Default 3 s): Feed-Banner, `wayfinder_feed_stale 1`, `/ready` → 503 |
| Abo entfernen (Option B) | letztes Abo des Feeds löschen | Orchestrator stoppt/entfernt `wayfinder-firefly-feed-<id>` |

---

## Phase 10 — Teardown „bei 0" (sauberer Neustart)

`docker compose down` allein genügt **nicht**, wenn der Orchestrator lief: die von ihm
gestarteten Firefly-Container gehören **nicht** zum Compose-Projekt.

```bash
# 1) Compose-Stack runter + Volumes (Postgres-Daten!) + Compose-Orphans
docker compose -p wayfinder -f docker-compose.onboarding.yml   down -v --remove-orphans
docker compose -p wayfinder -f docker-compose.orchestrated.yml down -v --remove-orphans  # falls Option B benutzt

# 2) Vom Orchestrator gespawnte Firefly-Container (Label!) entfernen
docker ps -aq --filter "label=wayfinder.managed=true" | xargs -r docker rm -f
```

**Reihenfolge wichtig:** erst `down` (stoppt den Orchestrator), **dann** die Firefly-
Container löschen — sonst spawnt der Reconcile-Loop sie sofort nach.

**Prüfschritt (wirklich bei 0)**
```bash
docker compose -p wayfinder -f docker-compose.onboarding.yml ps   # leer
docker ps -a --filter "label=wayfinder.managed=true"               # leer
docker volume ls | grep -i wayfinder                               # leer
```
- ⚠️ Bleiben Volumes übrig (z. B. `<projekt>_wayfinder-db`), stammen sie aus einem **anderen
  Projektnamen** (Präfix vor `_`). Gezielt entfernen: `docker volume rm <name>`. Genau deshalb
  oben überall **`-p wayfinder`** pinnen.

---

## Anhang A — Metrik-Referenz (`GET $OBS/metrics`, Port 8080)

| Metrik | Typ | Bedeutung |
|--------|-----|-----------|
| `wayfinder_cat062_blocks_received_total` | counter | empfangene CAT062-Datenblöcke |
| `wayfinder_cat062_tracks_received_total` | counter | dekodierte Tracks |
| `wayfinder_cat062_decode_errors_total` | counter | verworfene fehlerhafte Records |
| `wayfinder_tracks_current` | gauge | Tracks im zuletzt empfangenen Block |
| `wayfinder_cat065_heartbeats_received_total` | counter | SDPS-Heartbeats (Feed lebt) |
| `wayfinder_feed_stale` | gauge | `1` = kein Heartbeat seit `WAYFINDER_FEED_STALE_TIMEOUT` |
| `wayfinder_ws_clients_connected` | gauge | verbundene ASD-Clients (global) |
| `wayfinder_tenant_ws_clients_connected{tenant="…"}` | gauge | Clients je Mandant |
| `wayfinder_tenant_tracks_delivered_total{tenant="…"}` | counter | je Mandant zugestellte Tracks (nach AOI/FL-Filter) |

## Anhang B — DB-Direktprüfung

```bash
# In den DB-Container (Onboarding-Stack, Service „db"):
docker compose -p wayfinder -f docker-compose.onboarding.yml exec db \
  psql -U wayfinder -d wayfinder -c \
  "SELECT t.slug, f.name AS feed, vc.center_lat, vc.center_lon, vc.aoi, vc.fl_min, vc.fl_max
     FROM tenants t
     LEFT JOIN subscriptions s ON s.tenant_id=t.id
     LEFT JOIN feeds f         ON f.id=s.feed_id
     LEFT JOIN view_configs vc ON vc.tenant_id=t.id AND vc.user_id IS NULL
    WHERE t.slug='weeze-tower';"
```

## Anhang C — Fehlerbilder

| Symptom | Ursache | Abhilfe |
|---------|---------|---------|
| Admin-API liefert `403 password_change_required` | Pflicht-Passwortwechsel offen | Phase 1 Schritt 2 ausführen |
| Karte bleibt leer, `…_received_total = 0` | Multicast erreicht Wayfinder nicht (Bridge-Net / macOS-Desktop) | Phase 8: Host-Net/Linux, Master-Compose, oder orchestrated-Stack |
| `wayfinder-firefly-feed-<id>` wird nicht gespawnt | kein Abo / kein Firefly-Image / Orchestrator nicht aktiv | Abo prüfen (Phase 5); `docker build -t firefly:latest .` im Firefly-Repo |
| Flugzeuge da, aber nicht bei EDLV sichtbar | AOI/Feed-bbox passt nicht | Phase 4/6: bbox um EDLV prüfen (Feed-bbox ⊇ AOI) |
| `/ready` bleibt 503 | `feed_stale` (kein Heartbeat) | Firefly läuft? `FIREFLY_CAT065_ENABLED` (Default an)? Gruppe/Port = 239.255.0.62/8600? |
| Volumes überleben `down -v` | anderer Compose-Projektname | `-p wayfinder` pinnen; Rest-Volumes `docker volume rm` |

---

## Sicherheits-Hinweis (Docker-Socket, nur orchestrated-Stack)

Der Orchestrator mountet `/var/run/docker.sock` — **root-äquivalente** Host-Kontrolle.
Deshalb ist er ein **getrennter Least-Privilege-Prozess**; der browser-zugewandte
Server bekommt den Socket **nie** (ADR 0012 §6). Im Produktivbetrieb ist der
Orchestrator-Host eine **hochwertige Vertrauensgrenze** (Netz-Isolation, restriktiver
Zugang, keine Co-Location mit dem Browser-Rand).

## Bekannte Grenzen

- **Docker Desktop (macOS/Windows):** Host-Networking-Multicast funktioniert i. d. R.
  nicht — Phasen 1–7 laufen vollständig, **Live-Tracks (Phase 8) brauchen einen Linux-Host**.
- **Kein kombinierter Stack** liefert heute *Multitenant + Orchestrator-Auto-Spawn +
  Live-Tracks* in einem Compose: Phasen 0–7 auf `onboarding`, Track-Auto-Spawn auf
  `orchestrated` (Single-Tenant). Im Produktiv-Deployment laufen beide gegen **dieselbe
  DB** (Orchestrator im selben Auth-Modus) — das ist die reale Topologie, nicht die
  Einzelhost-Harness.
- **Diese Repo-CI/Sandbox** ohne Docker-Daemon kann den Lauf nicht ausführen; verifiziert
  sind dort nur `docker compose config`, Builds und Skript-Syntax. Der echte Lauf gehört
  auf einen Docker-Host.
