# Wayfinder — Installationsanleitung

> **Zweck:** Schritt-für-Schritt-Anleitung zum Aufsetzen und Starten von
> Wayfinder, vom Build bis zum laufenden ASD-Fenster im Browser.

---

## Inhaltsverzeichnis

1. [Voraussetzungen](#1-voraussetzungen)
2. [Schnellstart mit Docker (empfohlen)](#2-schnellstart-mit-docker-empfohlen)
3. [Lokaler Build ohne Docker](#3-lokaler-build-ohne-docker)
4. [End-to-End mit Firefly](#4-end-to-end-mit-firefly)
5. [macOS / Windows Docker Desktop](#5-macos--windows-docker-desktop)
6. [Kubernetes / Cloud-Deployment](#6-kubernetes--cloud-deployment)
7. [Konfigurationsreferenz](#7-konfigurationsreferenz)
8. [Verifikation](#8-verifikation)

---

## 1. Voraussetzungen

### Für den Docker-Betrieb

| Komponente | Mindestversion | Hinweis |
|------------|----------------|---------|
| Docker | 24.x | |
| Docker Compose | v2 (`docker compose`) | Compose v1 (`docker-compose`) genügt |
| Betriebssystem | Linux (Kernel ≥ 5.x) | Für Multicast-Empfang ist `network_mode: host` nötig — auf Linux ohne Einschränkung verfügbar; macOS/Windows: siehe [Abschnitt 5](#5-macos--windows-docker-desktop) |

### Für den lokalen Build

| Komponente | Mindestversion | Hinweis |
|------------|----------------|---------|
| Go | 1.23 | `go version` zum Prüfen |
| Node.js | 18 LTS | Nur für Frontend-Build nötig |
| npm | 9+ | Kommt mit Node |

### Netzwerk

Wayfinder empfängt den CAT062/CAT065-Strom von Firefly als **UDP-Multicast**.
Damit der Empfang funktioniert, müssen beide Prozesse im selben Subnetz
erreichbar sein und das Multicast-Routing aktiv sein. Auf einem einzelnen
Linux-Host genügt `network_mode: host` in Docker; auf VM/Cloud-Instanzen muss
die Netzwerkkarte explizit für Multicast freigeschaltet sein.

---

## 2. Schnellstart mit Docker (empfohlen)

### 2.1 Repository klonen

```bash
git clone https://github.com/manuelringwald/wayfinder.git
cd wayfinder
```

### 2.2 Starten

```bash
docker compose up
```

Das Image wird beim ersten Start automatisch gebaut (`docker compose up --build`
für einen Neubau nach Code-Änderungen).

Wayfinder ist dann erreichbar unter:

| Adresse | Inhalt |
|---------|--------|
| `http://localhost:8081` | ASD-Karte (Browser) |
| `http://localhost:8080/health` | Liveness-Probe |
| `http://localhost:8080/ready` | Readiness-Probe |
| `http://localhost:8080/metrics` | Prometheus-Metriken |

> **Hinweis:** Ohne laufenden Firefly-Sender sieht die Karte eine leere
> Luftlage — das ist korrekt. Die Readiness-Probe zeigt `not ready`, bis
> mindestens ein CAT065-Heartbeat empfangen wurde.

### 2.3 Kartenzentrierung anpassen

Standardmäßig ist die Karte auf Frankfurt (50.0379 N / 8.5622 E, Zoom 8)
zentriert. Für einen anderen Ausschnitt kann entweder eine Datei
`wayfinder.yaml` im Projektverzeichnis angelegt werden:

```yaml
# wayfinder.yaml (aus wayfinder.yaml.example)
map:
  center_lat: 48.1374   # München
  center_lon: 11.5755
  zoom: 9
openaip:
  radius_km: 185        # 100 NM Radius
```

Alternativ über Umgebungsvariablen in `docker-compose.yml`:

```yaml
environment:
  WAYFINDER_MAP_CENTER_LAT: "48.1374"
  WAYFINDER_MAP_CENTER_LON: "11.5755"
  WAYFINDER_MAP_ZOOM: "9"
```

---

## 3. Lokaler Build ohne Docker

### 3.1 Backend bauen

```bash
go build -o wayfinder ./cmd/wayfinder
```

Für einen statischen Binary (empfohlen für Deployment):

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o wayfinder ./cmd/wayfinder
```

### 3.2 Frontend bauen

Das Frontend (`frontend/`) wird mit Vite gebaut und vom Go-Backend als
eingebettete statische Dateien ausgeliefert. Das Build-Artefakt liegt in
`internal/webui/dist/` und ist **bereits im Repository eingecheckt** — ein
lokaler Frontend-Build ist nur nach Änderungen am Frontend-Code nötig.

```bash
cd frontend
npm install
npm run build       # schreibt nach ../internal/webui/dist/
cd ..
```

Danach Go-Backend neu bauen (Schritt 3.1), damit die neuen dist-Assets
eingebettet werden.

### 3.3 Starten

```bash
./wayfinder
```

Oder mit angepasster Konfiguration:

```bash
WAYFINDER_MAP_CENTER_LAT=48.1374 \
WAYFINDER_MAP_CENTER_LON=11.5755 \
WAYFINDER_LOG_LEVEL=debug \
./wayfinder
```

### 3.4 Tests

```bash
go test ./...
go vet ./...
```

Frontend-Tests:

```bash
cd frontend && npm run test -- --run
```

---

## 4. End-to-End mit Firefly

Für ein vollständiges ASD-System müssen Firefly (Sender) und Wayfinder
(Empfänger) gleichzeitig laufen und über denselben Multicast-Stream verbunden
sein.

### 4.1 Voraussetzung

Beide Repositories ausgecheckt:

```
~/
├── wayfinder/
└── firefly/      # https://github.com/manuelringwald/firefly
```

### 4.2 Firefly starten (CAT062-Ausgabe aktivieren)

```bash
cd firefly
FIREFLY_CAT062_ENABLED=true \
FIREFLY_CAT062_GROUP=239.255.0.62 \
FIREFLY_CAT062_PORT=8600 \
docker compose up
```

### 4.3 Wayfinder starten

In einem zweiten Terminal:

```bash
cd wayfinder
docker compose up
```

Beide Container laufen mit `network_mode: host` und sind über den
Multicast-Socket `239.255.0.62:8600` verbunden. Tracks von Firefly erscheinen
innerhalb weniger Sekunden auf der Karte unter `http://localhost:8081`.

---

## 5. macOS / Windows Docker Desktop

Docker Desktop nutzt eine interne Linux-VM, die **keinen Host-Netzwerk-Stack**
teilt. `network_mode: host` ist daher wirkungslos. Stattdessen müssen Firefly
und Wayfinder in einem gemeinsamen **Bridge-Netzwerk** laufen, in dem die VM
selbst als Multicast-Router fungiert.

### 5.1 Gemeinsames docker-compose.yml erstellen

Lege ein neues Verzeichnis `adsb-stack/` mit folgendem `docker-compose.yml` an:

```yaml
version: "3.9"
networks:
  adsb:
    driver: bridge
    driver_opts:
      com.docker.network.bridge.enable_ip_masquerade: "true"

services:
  firefly:
    build: ../firefly
    networks: [adsb]
    environment:
      FIREFLY_CAT062_ENABLED: "true"
      FIREFLY_CAT062_GROUP: "239.255.0.62"
      FIREFLY_CAT062_PORT: "8600"

  wayfinder:
    build: ../wayfinder
    networks: [adsb]
    ports:
      - "8081:8081"
      - "8080:8080"
    environment:
      FIREFLY_CAT062_GROUP: "239.255.0.62"
      FIREFLY_CAT062_PORT: "8600"
```

### 5.2 Starten

```bash
cd adsb-stack
docker compose up
```

> **Hinweis:** Multicast-Routing in Docker Bridge-Netzwerken ist
> implementierungsabhängig — bei Problemen auf Linux mit Host-Networking
> ausweichen oder Firefly und Wayfinder als separate Prozesse (kein Docker)
> auf demselben Host starten.

---

## 6. Kubernetes / Cloud-Deployment

Wayfinder ist ein **12-Factor-Service** und eignet sich direkt für
Kubernetes-Deployment.

### 6.1 Image bauen und pushen

```bash
docker build -t your-registry/wayfinder:latest .
docker push your-registry/wayfinder:latest
```

### 6.2 Deployment-Hinweise

- **UDP-Multicast** ist in Cloud-Netzwerken (AWS VPC, GCP VPC) standardmäßig
  blockiert. Wayfinder muss im selben Subnetz wie Firefly laufen, und das
  Netzwerk muss Multicast-Traffic (Gruppe `239.255.0.62`, Port UDP/8600)
  zulassen. Alternativ: Firefly und Wayfinder als Sidecar-Container im selben
  Pod (localhost-Multicast).
- **Health- und Readiness-Probes** auf Port 8080:
  ```yaml
  livenessProbe:
    httpGet:
      path: /health
      port: 8080
    initialDelaySeconds: 5
    periodSeconds: 10
  readinessProbe:
    httpGet:
      path: /ready
      port: 8080
    initialDelaySeconds: 3
    periodSeconds: 5
    failureThreshold: 6
  ```
- **Konfiguration** ausschließlich über Umgebungsvariablen (keine Secrets in
  ConfigMaps — `WAYFINDER_AUTH_TOKEN` als Kubernetes-Secret einbinden).
- **Graceful Shutdown**: Wayfinder reagiert auf `SIGINT`/`SIGTERM` und
  schließt alle Verbindungen sauber. `terminationGracePeriodSeconds: 10`
  genügt.
- **Logs**: Strukturiertes JSON auf stderr — direkt von Fluentd/Loki/CloudWatch
  konsumierbar.

### 6.3 Minimalbeispiel: Kubernetes-Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wayfinder
spec:
  replicas: 1
  selector:
    matchLabels:
      app: wayfinder
  template:
    metadata:
      labels:
        app: wayfinder
    spec:
      containers:
        - name: wayfinder
          image: your-registry/wayfinder:latest
          ports:
            - containerPort: 8081
            - containerPort: 8080
          env:
            - name: FIREFLY_CAT062_GROUP
              value: "239.255.0.62"
            - name: FIREFLY_CAT062_PORT
              value: "8600"
            - name: WAYFINDER_LOG_LEVEL
              value: "info"
            - name: WAYFINDER_AUTH_TOKEN
              valueFrom:
                secretKeyRef:
                  name: wayfinder-secrets
                  key: auth-token
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
```

---

## 7. Konfigurationsreferenz

Konfiguration erfolgt über **Umgebungsvariablen** (höchste Priorität) und
optional über eine **YAML-Datei** (`wayfinder.yaml` im Arbeitsverzeichnis,
Pfad überschreibbar via `WAYFINDER_CONFIG_FILE`). Env-Vars gewinnen immer.

### 7.1 Netzwerk & Feed

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `FIREFLY_CAT062_GROUP` | `239.255.0.62` | UDP-Multicast-Gruppe für CAT062/CAT065-Eingang |
| `FIREFLY_CAT062_PORT` | `8600` | UDP-Port des Multicast-Stroms |
| `WAYFINDER_FEED_ID` | `0` | Katalog-Feed-ID dieses Einzel-Feeds (WF2-20); wird auf jeden Track gestempelt (`feed_id` im WS-Strom). `0` = Single-Tenant. Im Multi-Feed-Modus (WF2-20.2) liefert der DB-Katalog die Feed-IDs. |
| `WAYFINDER_PROBE_PORT` | `8080` | Port für `/health`, `/ready`, `/metrics` |
| `WAYFINDER_FEED_STALE_TIMEOUT` | `3` | Sekunden ohne CAT065-Heartbeat, ab denen der Feed als stale gilt |

### 7.2 Karte & Darstellung

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_MAP_CENTER_LAT` | `50.0379` | Latitude des Karten-Startzentrums (Frankfurt) |
| `WAYFINDER_MAP_CENTER_LON` | `8.5622` | Longitude des Karten-Startzentrums |
| `WAYFINDER_MAP_ZOOM` | `8` | Initialer Zoom-Level (1–22) |
| `WAYFINDER_MAP_THEME` | `dark` | Karten-Theme: `dark` (CARTO Dark, schlüsselfrei) oder `osm` (OpenStreetMap-Raster) |
| `WAYFINDER_MAP_STYLE_URL` | *(leer)* | Vollständige MapLibre-Style-URL — überschreibt `WAYFINDER_MAP_THEME` |

### 7.3 Aeronautische Daten (OpenAIP)

Alle Variablen dieser Gruppe sind optional. Ohne `WAYFINDER_OPENAIP_API_KEY`
ist das Feature deaktiviert (Warn-Log, keine Fehlermeldung an den Client).

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_OPENAIP_API_KEY` | *(leer)* | OpenAIP-API-Schlüssel; leer = Feature aus |
| `WAYFINDER_OPENAIP_RADIUS_KM` | `250` | Radius um das Kartenzentrum für Luftraum-/Navaid-Abfragen |
| `WAYFINDER_OPENAIP_REFRESH` | `24h` | Refresh-Intervall (Go-Duration-Format: `1h`, `30m`, `24h`) |
| `WAYFINDER_OPENAIP_BASE_URL` | *(intern)* | Override der OpenAIP-Basis-URL (für Tests/Proxies) |

### 7.5 Radarabdeckungs-Overlay (Paket 6)

Sensor-Positionen und -Reichweiten für das Coverage-Ring-Overlay. N = 1, 2, 3, …
(max. 20); die Reihe muss lückenlos beginnen — fehlende N=2 stoppt die Auswertung.

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_COVERAGE_SENSOR_N_LAT` | *(leer)* | Breitengrad des Radarstandorts (Dezimalgrad WGS84) |
| `WAYFINDER_COVERAGE_SENSOR_N_LON` | *(leer)* | Längengrad des Radarstandorts (Dezimalgrad WGS84) |
| `WAYFINDER_COVERAGE_SENSOR_N_MAX_RANGE_M` | *(leer)* | Maximale Reichweite in Metern (Pflicht; 0 = Sensor überspringen) |
| `WAYFINDER_COVERAGE_SENSOR_N_MIN_RANGE_M` | `0` | Innerer Blindbereich in Metern (0 = kein Blindbereich) |
| `WAYFINDER_COVERAGE_SENSOR_N_LABEL` | *(leer)* | Tooltip-Bezeichnung des Radars |
| `WAYFINDER_COVERAGE_RING_COLOR` | `#5B8DEF` | Farbe aller Radarringe (CSS-Hex-Farbe) |

**Beispiel (Frankfurt-Konfiguration):**
```
WAYFINDER_COVERAGE_SENSOR_1_LAT=50.0379
WAYFINDER_COVERAGE_SENSOR_1_LON=8.5622
WAYFINDER_COVERAGE_SENSOR_1_MAX_RANGE_M=120000
WAYFINDER_COVERAGE_SENSOR_1_LABEL=Frankfurt-Center

WAYFINDER_COVERAGE_SENSOR_2_LAT=50.0849
WAYFINDER_COVERAGE_SENSOR_2_LON=8.0638
WAYFINDER_COVERAGE_SENSOR_2_MAX_RANGE_M=100000
WAYFINDER_COVERAGE_SENSOR_2_LABEL=Frankfurt-West

WAYFINDER_COVERAGE_SENSOR_3_LAT=50.3558
WAYFINDER_COVERAGE_SENSOR_3_LON=9.0009
WAYFINDER_COVERAGE_SENSOR_3_MAX_RANGE_M=100000
WAYFINDER_COVERAGE_SENSOR_3_LABEL=Frankfurt-Nordost
```

Die Werte müssen mit den Sensor-Positionen in Fireflys Konfiguration übereinstimmen.
Ohne konfigurierte Sensoren bleibt das Feature deaktiviert (kein Fehler).

### 7.4 Sicherheit

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_ALLOWED_ORIGINS` | *(leer)* | Kommaseparierte Liste erlaubter Cross-Origin-Domains für `/ws`, z. B. `https://asd.example.com`. Leer = nur Same-Origin. |
| `WAYFINDER_AUTH_TOKEN` | *(leer)* | Bearer-Token für den Browser-Rand. Leer = kein Token-Check (Warn-Log). Prüfung via `Authorization: Bearer <token>` oder `?token=<token>`. |
| `WAYFINDER_TLS_CERT` | *(leer)* | Pfad zum TLS-Zertifikat (PEM). Nur aktiv, wenn beide Werte gesetzt sind. |
| `WAYFINDER_TLS_KEY` | *(leer)* | Pfad zum TLS-Schlüssel (PEM). |

### Multi-Mandanten (Wayfinder 2.0)

Multi-Tenancy ist **nur aktiv, wenn `WAYFINDER_DB_URL` gesetzt ist**. Ohne diese
Variable läuft Wayfinder als Single-Tenant-ASD (kein Datenbank-Zugriff, keine
Tenant-Middleware — wie bisher). Mit gesetzter DB werden die Schema-Migrationen
beim Start angewandt und `/ws` durch die Tenant-Middleware geschützt (fail-closed:
ohne gültigen, einem Mandanten zugeordneten Nutzer → `401`).

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_DB_URL` | *(leer)* | PostgreSQL-DSN (z. B. `postgres://user:pass@host:5432/wayfinder`). Leer = Single-Tenant, keine DB. |
| `WAYFINDER_AUTH_MODE` | `none` | `proxy` (OIDC-Token vom Reverse-Proxy validieren), `builtin` (eingebaute Nutzer + Session-Cookie) oder `none` (fixes Subject, nur mit Netz-Isolation). |
| `WAYFINDER_OIDC_ISSUER` | *(leer)* | `proxy`: OIDC-Issuer-URL (Discovery/JWKS). Pflicht im proxy-Modus. |
| `WAYFINDER_OIDC_AUDIENCE` | *(leer)* | `proxy`: erwartete Audience (Client-ID). Pflicht im proxy-Modus. |
| `WAYFINDER_SESSION_KEY` | *(leer)* | `builtin`: HMAC-Schlüssel zum Signieren der Session-Cookies. Pflicht im builtin-Modus. |
| `WAYFINDER_SESSION_COOKIE` | `wf_session` | `builtin`: Name der Session-Cookie. |
| `WAYFINDER_SESSION_TTL` | `12h` | `builtin`: Session-Lebensdauer (Go-Duration, z. B. `8h`). |
| `WAYFINDER_NONE_SUBJECT` | `default` | `none`: festes Subject, das jeder Anfrage zugeordnet wird. |

> ℹ️ **builtin-Login:** `POST /api/login` mit `{"subject":"…","password":"…"}` →
> setzt bei Erfolg eine HttpOnly-Session-Cookie (sonst `401`); `POST /api/logout`
> löscht sie. Passwörter werden als argon2id-Hash gespeichert; Nutzer und
> Passwörter legt der Admin-Bootstrap (WF2-13) an. Der **proxy-Modus** braucht
> keinen Login (der vorgelagerte OIDC-Proxy authentifiziert).

#### Admin-Bootstrap (ersten Mandanten/Nutzer anlegen)

Ein frisch aufgesetztes Multi-Mandanten-Deployment hat zunächst **keinen** Nutzer.
Der Subcommand `wayfinder bootstrap` legt den ersten Mandanten + Admin-Nutzer (und
im builtin-Modus dessen Passwort) an. Er liest `WAYFINDER_DB_URL`, wendet die
Migrationen an und ist **idempotent** (mehrfach ausführbar — vorhandener Mandant/
Nutzer wird wiederverwendet, das Passwort wird neu gesetzt):

```bash
# proxy-Modus: nur Mandant + Nutzer (das OIDC-subject mappt auf den Nutzer)
WAYFINDER_DB_URL=postgres://… wayfinder bootstrap \
    -tenant acme -tenant-name "ACME Air" -subject alice@example.com -role tenant_admin

# builtin-Modus: zusätzlich ein Passwort (über ENV, nicht als Flag — Flags sind
# in der Prozessliste sichtbar)
WAYFINDER_DB_URL=postgres://… WAYFINDER_BOOTSTRAP_PASSWORD='…' \
    wayfinder bootstrap -tenant acme -subject admin -role tenant_admin
```

| Flag / Variable | Default | Beschreibung |
|-----------------|---------|--------------|
| `-tenant` | *(Pflicht)* | Mandanten-Slug (eindeutig). |
| `-tenant-name` | = Slug | Anzeigename des Mandanten. |
| `-subject` | *(Pflicht)* | OIDC-Subject (proxy) bzw. Benutzername (builtin) des Admins. |
| `-email` | *(leer)* | Optionale E-Mail. |
| `-role` | `tenant_admin` | `operator` \| `tenant_admin` \| `super_admin`. |
| `-password` | *(leer)* | builtin-Passwort (besser über `WAYFINDER_BOOTSTRAP_PASSWORD`). |
| `WAYFINDER_BOOTSTRAP_PASSWORD` | *(leer)* | builtin-Passwort (bevorzugt; nicht in der Prozessliste sichtbar). |

> 🔒 **`/admin` (Browser-Route, WF2-32):** Bei aktiver Multi-Tenancy ist `/admin`
> die **Admin-Oberfläche** (Vue-SPA, History-Mode) — eine **eigenständige View**,
> die die ASD-Karte vollständig ersetzt (kein Overlay; die Karte wird unmounted).
> Der Server liefert für unbekannte Pfade die SPA-Shell aus (`webui.Handler`-
> Fallback), sodass Deep-Links wie `/admin` einen Reload überleben. Die Oberfläche
> konsumiert das Admin-API und ist durch dessen Rollen-Gate geschützt; die Rollen-
> Probe liegt auf `GET /api/admin/whoami` (`tenant_admin`/`super_admin`, sonst
> `403`). Der Provisioning-Bereich ist nur für `super_admin` sichtbar (kosmetisch —
> der Server erzwingt es unabhängig).

#### Admin-API (WF2-31)

Hinter demselben Rollen-Gate (`tenant_admin`/`super_admin`) liegt ein
**tenant-skopiertes REST-API** unter `/api/admin/*`. Die Mandanten-ID kommt
**immer aus der angemeldeten Identität** — ein Admin verwaltet nur die **eigene**
Mandanten-Konfiguration.

| Methode + Pfad | Wirkung |
|---|---|
| `GET /api/admin/whoami` | Eigene Identität/Rolle als JSON (Rollen-Probe der SPA). |
| `GET /api/admin/view` | Eigene effektive Sicht (Zentrum/Zoom/AOI/FL/Layer); `404` wenn keine gesetzt. |
| `PUT /api/admin/view` | Eigene Tenant-Default-Sicht setzen (server-validiert; `400` bei ungültig). |
| `GET /api/admin/subscriptions` | Eigene abonnierte Feeds. |
| `GET /api/admin/feeds` | Feed-Katalog (read-only). |

Beispiel:

```bash
curl -X PUT https://asd.example.com/api/admin/view \
  -H 'Content-Type: application/json' \
  -d '{"center_lat":50.1,"center_lon":8.7,"zoom":9,
       "aoi":{"min_lat":49,"min_lon":8,"max_lat":51,"max_lon":10},
       "fl_min":100,"fl_max":300}'
```

> Eine View-Änderung wirkt auf **neue** `/ws`-Verbindungen (der Scope wird beim
> Connect aufgelöst); bestehende Verbindungen werden erst mit Live-Apply (WF2-33)
> nachgezogen.

**Provisioning (nur `super_admin`, cross-tenant, WF2-31b):** Feed-Zugänge werden
über das API gegrantet/entzogen — die Ziel-Mandanten-ID steht im **Pfad** (nicht in
der Identity). Nur `super_admin` darf diese Routen (sonst `403`):

| Methode + Pfad | Wirkung |
|---|---|
| `GET /api/admin/tenants` | Alle Mandanten. |
| `GET /api/admin/tenants/{tenantID}/subscriptions` | Abos eines Mandanten. |
| `POST /api/admin/tenants/{tenantID}/subscriptions` | Feed granten (`{"feed_id":…}`), idempotent. |
| `DELETE /api/admin/tenants/{tenantID}/subscriptions/{feedID}` | Feed entziehen. |

```bash
# Feed 3 dem Mandanten 5 zuweisen (super_admin)
curl -X POST https://asd.example.com/api/admin/tenants/5/subscriptions \
  -H 'Content-Type: application/json' -d '{"feed_id":3}'
```

#### Feed-Katalog & Multi-Feed-Empfang (WF2-20)

Im Multi-Mandanten-Betrieb empfängt Wayfinder **mehrere Feeds** gleichzeitig: der
`feeds`-Katalog in der DB treibt **einen Receiver je Feed** (je eigene
Multicast-Gruppe/Port); jeder Track wird mit seiner Katalog-`feed_id` gestempelt
(Basis für die mandanten-skopierte Zustellung, WF2-21). Bis die Admin-API existiert
(WF2-31), wird der Katalog über das `feed`-Subcommand gepflegt:

```bash
# Feed in den Katalog aufnehmen
WAYFINDER_DB_URL=postgres://… wayfinder feed add \
    -name Frankfurt -group 239.255.0.62 -port 8600 -sensor-mix PSR,SSR,ADS-B

# Katalog anzeigen
WAYFINDER_DB_URL=postgres://… wayfinder feed list
```

| Flag | Default | Beschreibung |
|------|---------|--------------|
| `-name` | *(Pflicht)* | Anzeigename des Feeds. |
| `-group` | *(Pflicht)* | Multicast-Gruppe, z. B. `239.255.0.62`. |
| `-port` | `8600` | Multicast-Port. |
| `-region` | *(leer)* | Regions-Label (optional). |
| `-sensor-mix` | *(leer)* | Kommaseparierter Sensor-Mix, z. B. `PSR,SSR,ADS-B`. |

> ℹ️ **Fallback:** Ist der Katalog leer (oder läuft Wayfinder ohne
> `WAYFINDER_DB_URL`), wird **ein** Feed aus `FIREFLY_CAT062_GROUP`/`_PORT` +
> `WAYFINDER_FEED_ID` empfangen — das bisherige Single-Feed-Verhalten. Ein Feed,
> der nicht beitreten kann, wird übersprungen; kann **kein** Feed beitreten,
> beendet sich der Dienst. Der NATS-/Cloud-Bus-Pfad folgt später (WF2-53).

> 🔒 **Mandanten-Sicht (WF2-21):** Im Multi-Mandanten-Betrieb sieht ein `/ws`-Client
> **nur** Tracks aus den Feeds, die sein Mandant **abonniert** hat (`subscriptions`).
> Ein Mandant ohne Abo bekommt **keine** Tracks (fail-closed). Abos werden bis zur
> Admin-API (WF2-31) direkt in der DB gesetzt (`subscriptions`-Tabelle: `tenant_id`,
> `feed_id`). Single-Tenant (ohne `WAYFINDER_DB_URL`) sieht unverändert alles.
>
> Zusätzlich greift ein **Sicht-Filter** (WF2-21.2): ist in `view_configs` ein
> **Interessensgebiet (AOI, BBox)** und/oder ein **Flugflächen-Band**
> (`fl_min`/`fl_max`, in FL) gesetzt, verlassen Tracks außerhalb den Server gar
> nicht (harte Datensparsamkeits-Grenze — Bandbreite/Billing). **fail-open:** ein
> Track **ohne** gemessene Flugfläche wird trotzdem zugestellt (nie ein reales
> Flugzeug verschlucken). Der Lebenszyklus-Filter (confirmed/tentative/coasting)
> bleibt rein im Frontend (Declutter). Ohne AOI/FL-Eintrag wird der ganze
> abonnierte Feed zugestellt.
>
> 📝 **Audit-Log:** Jeder `/ws`-Connect erzeugt ein strukturiertes `slog`-Event
> (`component=audit`, `event=ws_connect`) mit Mandant, Nutzer und aufgelöstem Scope
> (Feeds + AOI/FL) — der Compliance-Nachweis „wer sah welchen Scope". Es geht in den
> normalen Log-Strom (`stderr`, JSON); für Auswertung/Aufbewahrung in eine externe
> Log-Senke leiten (ELK/Datadog o. Ä.). Keine DB-Audit-Tabelle.

### 7.5 Betrieb

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_LOG_LEVEL` | `info` | Log-Level: `debug`, `info`, `warn`, `error`. Ungültige Werte fallen auf `info` zurück. |
| `WAYFINDER_CONFIG_FILE` | `wayfinder.yaml` | Pfad zur optionalen YAML-Konfigurationsdatei. Fehlende Datei ist nicht fatal. |

### 7.6 YAML-Konfigurationsdatei

Felder aus `wayfinder.yaml` (oder dem per `WAYFINDER_CONFIG_FILE` angegebenen
Pfad) werden beim Start geladen. Env-Vars überschreiben sie immer.
Partielle Dateien sind zulässig — nicht angegebene Felder behalten ihre
Defaults.

```yaml
map:
  center_lat: 50.0379   # Latitude des Startzentrums
  center_lon: 8.5622    # Longitude des Startzentrums
  zoom: 8               # Initialer Zoom-Level
openaip:
  radius_km: 250        # Abfrageradius für aeronautische Daten
```

---

## 8. Verifikation

### 8.1 Liveness

```bash
curl -s http://localhost:8080/health
# → "ok"
```

### 8.2 Readiness

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/ready
# → "200" wenn Firefly-Feed aktiv (mindestens 1 CAT065-Heartbeat empfangen)
# → "503" wenn Feed noch nie gesehen oder gerade stale
```

### 8.3 Metriken

```bash
curl -s http://localhost:8080/metrics
```

Ein gesunder Feed zeigt `wayfinder_feed_stale 0` und steigende
`wayfinder_cat062_blocks_received_total`- bzw.
`wayfinder_cat065_heartbeats_received_total`-Zähler.

### 8.4 Browser

Browser auf `http://localhost:8081` öffnen. Die Karte erscheint sofort
(Radar Dark Theme). Tracks erscheinen, sobald Firefly Daten sendet —
erkennbar am Feed-Status-Banner oben links (grün: **FEED OK**).
