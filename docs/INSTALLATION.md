# Wayfinder — Installationsanleitung

> **Für wen ist diese Anleitung?** Für alle, die Wayfinder zum Laufen bringen
> wollen — **auch ohne IT-Hintergrund**. Sie führt Schritt für Schritt vom leeren
> Rechner bis zum fertigen Luftlagebild im Browser. Befehle einfach **kopieren und
> einfügen**. Wo etwas auf dem Mac anders ist, steht es ausdrücklich dabei.

> **Es gibt nur eine Betriebsart: die Multi-Tenant-Plattform** (Login, Datenbank,
> Admin-Oberfläche — ADR 0014). Die ganze Einrichtung steht in
> [Teil 4](#teil-4--einrichtung-multi-tenant-schritt-für-schritt):
> - **Schnellster Einstieg auf einem Rechner:** der Master-Compose-Schnellstart
>   ([Schritt 4.A](#schritt-4a--master-compose-für-plattform--firefly-option-b)) —
>   Plattform + Firefly-Testfeed in einem Bridge-Netz, läuft auf macOS, Windows
>   und Linux gleich.
> - **Voller Mehrkunden-Aufbau:** der Rest von Teil 4 (Mandanten, Zugänge, Feeds
>   zuweisen).

> 🛠️ **Läuft es schon und Sie wollen es im Alltag betreuen** (kontrollieren,
> Mandanten/Feeds pflegen, sichern, Störungen beheben)? → **Betriebsführungs­
> handbuch** `docs/BETRIEB.md`.

---

## Inhaltsverzeichnis

1. [Überblick — was läuft hier eigentlich?](#teil-1--überblick--was-läuft-hier-eigentlich)
2. [Werkzeuge installieren (macOS / Windows / Linux)](#teil-2--werkzeuge-installieren)
3. [Wayfinder & Firefly herunterladen](#teil-3--wayfinder--firefly-herunterladen)
4. [**Einrichtung (Multi-Tenant)** — Schritt für Schritt](#teil-4--einrichtung-multi-tenant-schritt-für-schritt)
   - [4.A — Master-Compose für Plattform + Firefly (Option B)](#schritt-4a--master-compose-für-plattform--firefly-option-b)
5. [Läuft es? — Verifikation](#teil-5--läuft-es--verifikation)
6. [Wenn etwas nicht geht — Fehlersuche](#teil-6--wenn-etwas-nicht-geht--fehlersuche)
7. [Konfigurationsreferenz (alle Schalter)](#teil-7--konfigurationsreferenz)
8. [Produktionsbetrieb (Kubernetes, TLS, Host-Netzwerk)](#teil-8--produktionsbetrieb)

---

## Teil 1 — Überblick: was läuft hier eigentlich?

Wayfinder ist das **ASD** (Air Situation Display) — die Karte mit den Flugzeugen.
Es zeigt aber **nicht selbst** Flugzeuge an, sondern **empfängt** sie von einem
zweiten Programm namens **Firefly** (dem Radar-Tracker). Die beiden reden über
einen Netzwerk-„Draht" (UDP-Multicast) miteinander.

```
┌─────────────┐   Flugzeug-Daten (UDP-Multicast)   ┌─────────────┐   Browser
│   Firefly   │  ───────────────────────────────►  │  Wayfinder  │  ──────►  🗺️
│ (rechnet    │        Gruppe 239.255.0.62          │  (zeigt an) │        localhost:8081
│  Tracks)    │             Port 8600               │             │
└─────────────┘                                     └─────────────┘
```

**Wichtig zu verstehen:** Ohne Firefly (oder eine andere Datenquelle) bleibt die
Karte **leer** — das ist kein Fehler. Deshalb richten wir in dieser Anleitung
**beide** zusammen ein, damit Sie sofort Flugzeuge sehen.

**Zwei Adressen (Ports), die Wayfinder öffnet:**

| Adresse | Wofür | Wer benutzt das? |
|---------|-------|------------------|
| `http://localhost:8081` | **Das Lagebild** (die Karte) | **Sie**, im Browser |
| `http://localhost:8080` | Technik-Checks (`/health`, `/ready`, `/metrics`) | Monitoring/Betrieb |

> 💡 **Merksatz:** **81 = sehen** (Browser), **80 = Technik**. Sie öffnen im
> Alltag nur die **8081**.

> 🌐 **Gar nichts installieren können?** (z. B. Arbeits-Laptop ohne
> Admin-Rechte und ohne Terminal): Der komplette Stack läuft alternativ in
> einem **GitHub Codespace** in der Cloud — der Rechner braucht dann nur einen
> Browser. Anleitung: **`docs/CODESPACES.md`**. Die restliche Anleitung hier
> gilt für die Installation auf einem eigenen Rechner/einer VM.

---

## Teil 2 — Werkzeuge installieren

Sie brauchen genau **zwei** Werkzeuge: **Docker** (führt Wayfinder & Firefly in
Containern aus, ohne dass Sie etwas kompilieren müssen) und **Git** (lädt den
Programmcode herunter).

### 🍎 macOS

1. **Docker Desktop installieren**
   - Laden Sie es von <https://www.docker.com/products/docker-desktop/> (Button
     „Download for Mac" — achten Sie auf **Apple Silicon** (M1/M2/M3/M4) bzw.
     **Intel**, je nach Mac).
   - Öffnen Sie die heruntergeladene `.dmg`, ziehen Sie **Docker** in den
     Ordner **Programme**, starten Sie **Docker** aus dem Launchpad.
   - Warten Sie, bis das Docker-Wal-Symbol oben in der Menüleiste **ruhig**
     steht (nicht mehr animiert). Erst dann ist Docker bereit.

2. **Git installieren** — öffnen Sie die App **Terminal**
   (Programme → Dienstprogramme → Terminal) und tippen Sie:
   ```bash
   git --version
   ```
   Fehlt Git, bietet macOS automatisch an, die „Command Line Tools" zu
   installieren — auf **Installieren** klicken und warten.

3. **Prüfen, dass alles da ist** — im Terminal:
   ```bash
   docker --version
   docker compose version
   git --version
   ```
   Jede Zeile sollte eine Versionsnummer ausgeben (keine Fehlermeldung).

> ⚠️ **Mac-Besonderheit (wichtig):** Docker läuft auf dem Mac in einer kleinen
> internen Linux-Maschine. Der direkte „Host-Netzwerk"-Modus (`network_mode: host`)
> funktioniert hier **nicht**. Der Multi-Tenant-Aufbau aus **Teil 4** (Master-
> Compose-Schnellstart bzw. `docker-compose.onboarding.yml`) nutzt dagegen ein
> Bridge-Netz und läuft auf dem Mac **genauso wie auf Linux**. Sie müssen nichts
> extra tun, einfach den Schritten folgen. (Für echte Tracks mit Firefly auf dem
> Mac → DOCKER.md, „macOS/Windows"-Abschnitt.)

### 🪟 Windows

1. **Docker Desktop** von <https://www.docker.com/products/docker-desktop/>
   installieren (Button „Download for Windows"). Docker Desktop richtet **WSL 2**
   automatisch ein — den Anweisungen des Installers folgen, danach **neu starten**.
2. **Git für Windows** von <https://git-scm.com/download/win> installieren
   (alle Vorgaben übernehmen).
3. Danach **Git Bash** öffnen (Startmenü → „Git Bash") und alle folgenden Befehle
   dort eingeben. Prüfen mit `docker --version`, `docker compose version`,
   `git --version`.

### 🐧 Linux (Ubuntu/Debian)

```bash
# Docker-Engine + Compose-Plugin + Git
sudo apt-get update
sudo apt-get install -y docker.io docker-compose-v2 git
# Den eigenen Benutzer zur docker-Gruppe hinzufügen (danach einmal aus- und
# wieder einloggen), damit "docker" ohne sudo läuft:
sudo usermod -aG docker "$USER"
```
Prüfen: `docker --version`, `docker compose version`, `git --version`.

---

## Teil 3 — Wayfinder & Firefly herunterladen

Wir legen **einen** Projektordner an, in den **beide** Programme nebeneinander
kommen. Das ist später wichtig, damit die gemeinsame Konfigurationsdatei beide
findet.

Im **Terminal** (Mac/Linux) bzw. **Git Bash** (Windows):

```bash
# 1. Einen Projektordner im Home-Verzeichnis anlegen und hineinwechseln
mkdir -p ~/asd
cd ~/asd

# 2. Beide Programme herunterladen
git clone https://github.com/manuelringwald/wayfinder.git
git clone https://github.com/manuelringwald/firefly.git
```

Danach sieht Ihr Ordner so aus:

```
~/asd/
├── wayfinder/     ← dieses Programm (die Karte)
└── firefly/       ← die Datenquelle (rechnet die Flugzeuge)
```

Prüfen Sie das mit:
```bash
ls ~/asd
# Erwartete Ausgabe:  firefly   wayfinder
```

> ℹ️ Sie brauchen Firefly **nur**, um Test-Flugzeuge zu erzeugen. Haben Sie
> bereits eine **echte** CAT062-Datenquelle im Netz, können Sie den
> `firefly`-Teil in den folgenden Compose-Dateien weglassen und stattdessen
> Gruppe/Port Ihrer echten Quelle eintragen.

---

## Teil 4 — Einrichtung (Multi-Tenant): Schritt für Schritt

**Das richten wir hier ein:** Eine Plattform, auf der sich **mehrere Mandanten
(Kunden)** mit **eigenem Login** anmelden — jeder sieht **nur** die Flugzeuge der
Feeds, die ihm zugewiesen wurden. Dazu kommen drei neue Bausteine hinzu:

| Baustein | Wozu |
|----------|------|
| **PostgreSQL-Datenbank** | Speichert Mandanten, Nutzer, Feeds, Berechtigungen |
| **Login (`builtin`)** | Benutzername + Passwort, Session über ein Cookie |
| **Admin-Oberfläche** (`/admin`) | Mandanten verwalten, Feeds zuweisen |

> **Rollen, die es gibt (ADR 0009):** `user` (sieht nur das Lagebild des eigenen
> Mandanten) und `admin` (Plattform-Betreiber: verwaltet **alle** Mandanten,
> Zugänge und Feeds). Ein Admin wird beim ersten Start **automatisch angelegt**
> (siehe Kasten unten) — Sie müssen ihn nicht mehr von Hand erzeugen.

> ⚡ **Schnellster Weg (Zero-Touch-Onboarding, ADR 0011) — zwei Optionen:**
>
> **Option A — nur Plattform, ohne Firefly** (Funktioniert auf macOS, Windows, Linux):
> Zum Testen von Login, Admin-UI und Passwortwechsel — kein Feed, Karte bleibt leer.
> ```bash
> cd ~/asd/wayfinder
> docker compose -f docker-compose.onboarding.yml up --build
> ```
>
> **Option B — Plattform + Firefly-Feed (empfohlen — mit echten Tracks):**
> Einmalig eine `docker-compose.yml` im **Überordner** (`~/asd/`) anlegen
> (Inhalt → [Schritt 4.A](#schritt-4a--master-compose-für-plattform--firefly-option-b)),
> dann:
> ```bash
> cd ~/asd
> docker compose up --build
> ```
> Alle drei Services (Firefly + Datenbank + Wayfinder) starten gemeinsam in
> einem Bridge-Netz. Funktioniert auf **macOS, Windows und Linux** gleich.
>
> **In beiden Fällen:** Beim ersten Start legt Wayfinder automatisch einen
> **Standard-Admin** an — Benutzername **`admin`**, Passwort **`admin`**. Öffnen
> Sie **`http://localhost:8081/admin`** und melden Sie sich an. Sie werden **sofort
> zum Passwortwechsel gezwungen** — bevor irgendeine andere Aktion möglich ist.
> Kein `bootstrap`, kein Terminal-Schritt nötig.
>
> Wer eine dieser Optionen nutzt, **überspringt die Schritte 4.1–4.4 komplett**.
> Weiter ab [Schritt 4.5](#schritt-45--feeds-in-den-katalog-aufnehmen).

### Schritt 4.A — Master-Compose für Plattform + Firefly (Option B)

> ℹ️ **Nur nötig für Option B** (Plattform + Firefly-Feed). Für Option A (ohne
> Firefly) direkt zu [Schritt 4.5](#schritt-45--feeds-in-den-katalog-aufnehmen)
> springen.

> ⚡ **Schneller (ohne selbst Compose schreiben):** Der bequemste Weg ohne
> eigene Linux-Umgebung ist heute ein **GitHub Codespace** mit dem
> orchestrierten Stack (Auto-Spawn je Feed, alles per Admin-UI) —
> `docs/CODESPACES.md`. Die früher eingecheckte `docker-compose.bridge.yml`
> (fester Sender mit Demo-Szene) ist entfallen (Firefly ADR 0030). Wer lokal
> auf Docker Desktop bleiben will, folgt dem Rest dieses Schritts.
> (E2E-Abnahme auf dem Mac mini: **voller** Lauf mit einer Linux-VM in
> `docs/E2E-ABNAHME.md`, Teil 1–6.)

Legen Sie eine Datei `docker-compose.yml` im **gemeinsamen Überordner** beider
Repos an. Die Struktur sieht dann so aus:

```
~/asd/
├── firefly/            ← Firefly-Repo (geklont, Teil 3)
├── wayfinder/          ← Wayfinder-Repo (geklont, Teil 3)
└── docker-compose.yml  ← diese Datei (jetzt anlegen)
```

```bash
nano ~/asd/docker-compose.yml
```

Vollständiger Inhalt:

```yaml
# ~/asd/docker-compose.yml
# Zero-Touch Multi-Tenant + Firefly — alle Plattformen (macOS, Windows, Linux).
# Startet: Firefly (Testdaten), PostgreSQL (Datenbank) und Wayfinder (ASD).
# Alle drei Services laufen im selben Bridge-Netz; Multicast funktioniert
# innerhalb des Netzes problemlos.
name: asd-plattform

networks:
  asd:
    driver: bridge

volumes:
  wayfinder-db:

services:
  # 1) Test-Datenquelle: sendet Testflugzeuge als CAT062-Multicast.
  firefly:
    build: ./firefly
    networks: [asd]
    environment:
      # Quellen sind Opt-in (Firefly ADR 0030): ohne Quelle sendet Firefly nur
      # den CAT065-Heartbeat (leerer Himmel). Für echte Tracks OHNE Konto der
      # Community-Aggregator (adsb.lol/adsb.fi — Firefly ADR 0031; funktioniert
      # auch dort, wo OpenSky Datacenter-IPs sperrt, z. B. Codespaces):
      FIREFLY_ADSBAGG_ENABLED: "true"
      # …oder alternativ OpenSky (kostenloses Konto, OAuth2 — Firefly ADR 0024):
      # FIREFLY_OPENSKY_ENABLED: "true"
      # FIREFLY_OPENSKY_CREDENTIALS: "client_id:client_secret"
      FIREFLY_CAT062_ENABLED: "true"
      FIREFLY_CAT062_GROUP: "239.255.0.62"
      FIREFLY_CAT062_PORT: "8600"
    restart: unless-stopped

  # 2) Datenbank für den Multi-Tenant-Betrieb.
  db:
    image: postgres:16-alpine
    networks: [asd]
    environment:
      POSTGRES_USER: "wayfinder"
      POSTGRES_PASSWORD: "wayfinder"
      POSTGRES_DB: "wayfinder"
    volumes:
      - wayfinder-db:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U wayfinder"]
      interval: 5s
      timeout: 3s
      retries: 10
    restart: unless-stopped

  # 3) Wayfinder — ASD-Frontend + Multi-Tenant-Plattform.
  wayfinder:
    build: ./wayfinder
    networks: [asd]
    depends_on:
      db:
        condition: service_healthy
    ports:
      - "8081:8081"     # Browser-Lagebild + Admin-UI
      - "8080:8080"     # Technik-Checks (/health, /ready, /metrics)
    environment:
      FIREFLY_CAT062_GROUP: "239.255.0.62"
      FIREFLY_CAT062_PORT: "8600"
      WAYFINDER_DB_URL: "postgres://wayfinder:wayfinder@db:5432/wayfinder?sslmode=disable"
      WAYFINDER_AUTH_MODE: "builtin"
      # Session-Schlüssel: leer = flüchtiger Zufalls-Schlüssel (Warn-Log).
      # Für Produktion: export WAYFINDER_SESSION_KEY=$(openssl rand -hex 32)
      WAYFINDER_SESSION_KEY: ${WAYFINDER_SESSION_KEY:-}
      # Optional (ORCH-2c, ADR 0012 §6): AES-256-Schlüssel, der Pro-Feed-Quell-
      # Credentials verschlüsselt. Leer = die Secret-Verwaltung im Admin
      # (Feed → Quellen → „Secret hinterlegen") bleibt deaktiviert (503).
      # Erzeugen: export WAYFINDER_SECRET_KEY=$(openssl rand -base64 32)
      # Wichtig (ORCH-5b): DERSELBE Schlüssel muss auch am Orchestrator-Prozess
      # (cmd/wayfinder-orchestrator) gesetzt sein, damit dieser die Credentials
      # beim Tracker-Start entschlüsseln und injizieren kann; fehlt er dort,
      # laufen credentialled Quellen anonym (WARN, kein Abbruch).
      WAYFINDER_SECRET_KEY: ${WAYFINDER_SECRET_KEY:-}
      # Optional (ORCH-4): Pool für die automatische Multicast-Endpoint-Vergabe
      # beim Feed-Anlegen (eine Gruppe je Feed). Defaults reichen i. d. R.:
      # WAYFINDER_FEED_GROUP_BASE=239.255.0  WAYFINDER_FEED_PORT=8600
      # WAYFINDER_FEED_OCTET_MIN=1  WAYFINDER_FEED_OCTET_MAX=254  (~254 Feeds)
      WAYFINDER_MAP_CENTER_LAT: "50.0379"   # Frankfurt
      WAYFINDER_MAP_CENTER_LON: "8.5622"
      WAYFINDER_MAP_ZOOM: "8"
    restart: unless-stopped
```

Starten:

```bash
cd ~/asd
docker compose up --build
```

Beim ersten Start dauert der Build einige Minuten (Go-Compiler + Rust-Compiler für
Firefly). Danach erscheinen Zeilen wie `feed joined` und `listening on :8081`.
Öffnen Sie **<http://localhost:8081/admin>** — nach Login und Passwortwechsel sind
Sie fertig. **Weiter mit Schritt 4.5**, um den Firefly-Feed dem Admin zuzuordnen
und ihn einem Mandanten zuzuweisen.

> **Auto-Orchestrierung (ORCH, ADR 0012).** Das obige `docker-compose.yml` fährt
> den ASD-Server gegen einen **externen** Firefly-Feed. Für den Modus „Feed
> zuweisen ⇒ Tracker startet automatisch" gibt es ein eigenes Compose-Profil
> `docker-compose.orchestrated.yml` (Postgres + Server + Least-Privilege-
> Orchestrator, der pro Feed eine Firefly-Instanz spawnt). Ein Linux-Docker-Host
> ist nötig (Host-Networking-Multicast). Die End-to-End-Abnahme inkl. Skript
> (`scripts/e2e-orchestrated.sh`) und Prüfpunkten steht in **`docs/E2E-ABNAHME.md`**.

---

### Schritt 4.1 — Steuerungsordner anlegen

```bash
mkdir -p ~/asd/start-plattform
cd ~/asd/start-plattform
```

### Schritt 4.2 — Die Start-Datei `docker-compose.yml` anlegen

```bash
nano docker-compose.yml
```

Vollständiger Inhalt (Datenbank **+** Firefly **+** Wayfinder):

```yaml
# ~/asd/start-plattform/docker-compose.yml
# Multi-Tenant-Aufbau: PostgreSQL + Firefly (Datenquelle) + Wayfinder.
# Funktioniert auf macOS, Windows und Linux gleich.
name: wayfinder-plattform

networks:
  asd:
    driver: bridge

volumes:
  db-daten:        # damit die Datenbank einen Neustart übersteht

services:
  # 1) Die Datenbank.
  db:
    image: postgres:16-alpine
    networks: [asd]
    environment:
      POSTGRES_USER: "wayfinder"
      POSTGRES_PASSWORD: "wayfinder"     # für lokal ok; in Produktion ändern!
      POSTGRES_DB: "wayfinder"
    volumes:
      - db-daten:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U wayfinder"]
      interval: 5s
      timeout: 3s
      retries: 10

  # 2) Die Datenquelle (Test-Flugzeuge).
  firefly:
    build: ../firefly
    networks: [asd]
    environment:
      # Quellen sind Opt-in (Firefly ADR 0030): ohne Quelle sendet Firefly nur
      # den CAT065-Heartbeat (leerer Himmel). Für echte Tracks OHNE Konto der
      # Community-Aggregator (adsb.lol/adsb.fi — Firefly ADR 0031; funktioniert
      # auch dort, wo OpenSky Datacenter-IPs sperrt, z. B. Codespaces):
      FIREFLY_ADSBAGG_ENABLED: "true"
      # …oder alternativ OpenSky (kostenloses Konto, OAuth2 — Firefly ADR 0024):
      # FIREFLY_OPENSKY_ENABLED: "true"
      # FIREFLY_OPENSKY_CREDENTIALS: "client_id:client_secret"
      FIREFLY_CAT062_ENABLED: "true"
      FIREFLY_CAT062_GROUP: "239.255.0.62"
      FIREFLY_CAT062_PORT: "8600"
    restart: unless-stopped

  # 3) Die Karte + Plattform.
  wayfinder:
    build: ../wayfinder
    networks: [asd]
    depends_on:
      db:
        condition: service_healthy        # erst starten, wenn die DB bereit ist
    ports:
      - "8081:8081"
      - "8080:8080"
    environment:
      FIREFLY_CAT062_GROUP: "239.255.0.62"
      FIREFLY_CAT062_PORT: "8600"
      # --- Das schaltet den Multi-Tenant-Betrieb ein: ---
      WAYFINDER_DB_URL: "postgres://wayfinder:wayfinder@db:5432/wayfinder?sslmode=disable"
      WAYFINDER_AUTH_MODE: "builtin"      # Login mit Benutzername + Passwort
      # Geheimer Schlüssel zum Signieren der Login-Cookies — UNBEDINGT ändern,
      # mind. 32 zufällige Zeichen (z. B. Ausgabe von:  openssl rand -hex 32):
      WAYFINDER_SESSION_KEY: "BITTE-AENDERN-mind-32-zufaellige-zeichen-einsetzen"
    volumes:
      - ./wayfinder.yaml:/app/wayfinder.yaml:ro
    restart: unless-stopped
```

> 🔑 **Den Session-Schlüssel jetzt erzeugen:** Tippen Sie `openssl rand -hex 32`
> ins Terminal, kopieren Sie die ausgegebene Zeichenkette und ersetzen Sie damit
> den Platzhalter bei `WAYFINDER_SESSION_KEY`. **Für Produktion dringend
> empfohlen.** Lassen Sie ihn weg, erzeugt Wayfinder beim Start einen flüchtigen
> Zufalls-Schlüssel und warnt — dann gehen Sessions bei jedem Neustart verloren
> und sind nicht multi-Replica-fähig (ADR 0011).

### Schritt 4.3 — Den Kartenausschnitt anlegen (`wayfinder.yaml`)

Diese Datei legt fest, **wo** die Karte beim Start hinschaut:

```bash
nano wayfinder.yaml
```

```yaml
# ~/asd/start-plattform/wayfinder.yaml
map:
  center_lat: 50.0379
  center_lon: 8.5622
  zoom: 8
openaip:
  radius_km: 185
```

### Schritt 4.4 — Den ersten Administrator anlegen (`bootstrap`) — *optional*

> ✅ **In den meisten Fällen übersprungen:** Im `builtin`-Modus legt Wayfinder
> beim ersten Start automatisch einen Standard-Admin `admin`/`admin` an (ONB-1,
> ADR 0011) und erzwingt den Passwortwechsel beim ersten Login. Sie brauchen
> `bootstrap` nur, wenn Sie den ersten Admin **mit eigenem Namen/Passwort** statt
> des Standard-Kontos anlegen wollen.

Eine frische Datenbank hat (ohne Auto-Seed) **noch keinen** Nutzer. Der eingebaute
Befehl `bootstrap` legt den **ersten Plattform-Admin** an. Er startet die
Datenbank automatisch mit und richtet das Schema ein.

> 🔑 **Admins sind mandanten-los (ONB-3, ADR 0011):** Ein Plattform-Admin gehört
> **keinem** Mandanten an — `-role admin` braucht daher **kein** `-tenant` (ein
> mitgegebenes `-tenant` wird ignoriert). Mandanten und Lotsen-Zugänge (Rolle
> `user`) legen Sie danach bequem über die Oberfläche an (Schritt 4.8b). Einen
> Mandanten-Nutzer per CLI anzulegen verlangt dagegen `-role user -tenant <slug>`.

Geben Sie das **als einen Block** ein (das Passwort wird über eine Variable
übergeben, damit es **nicht** in der Befehlsliste sichtbar ist):

```bash
WAYFINDER_BOOTSTRAP_PASSWORD='MeinAdminPasswort123' \
docker compose run --rm \
  -e WAYFINDER_BOOTSTRAP_PASSWORD \
  wayfinder bootstrap \
    -subject admin \
    -role admin
```

Erwartete Ausgabe (sinngemäß):
```
created admin "admin" (id=1)
set builtin password for "admin"
```

> 🔁 Der Befehl ist **idempotent** — Sie können ihn gefahrlos erneut ausführen
> (z. B. um das Passwort neu zu setzen). Bestehende Konten werden wiederverwendet.

### Schritt 4.5 — Feeds in den Katalog aufnehmen

Im Multi-Tenant-Betrieb werden die Datenquellen **in der Datenbank** verwaltet
(nicht über die `docker-compose.yml`). Nehmen wir den Firefly-Feed auf:

```bash
docker compose run --rm wayfinder feed add \
  -name "Frankfurt" \
  -group 239.255.0.62 \
  -port 8600 \
  -sensor-mix PSR,SSR,ADS-B
```

Erwartete Ausgabe:
```
created feed "Frankfurt" (id=1) 239.255.0.62:8600
```

Den Katalog ansehen:
```bash
docker compose run --rm wayfinder feed list
```

> Der `-sensor-mix` beschreibt, welche Sensorarten der Feed liefert. Erlaubt sind
> `PSR, SSR, MODE_S, ADS-B, MLAT, FLARM`. Gängige Schreibweisen werden automatisch
> korrigiert (`ads-b` → `ADS-B`); **unbekannte** Klassen werden abgelehnt.

> **Seit ONB-5 (ADR 0011) geht das auch ohne Terminal:** Feeds lassen sich im
> Admin-Bereich unter **„Feeds"** anlegen und löschen. Der Server tritt der
> Multicast-Gruppe eines neuen Feeds **sofort** bei bzw. verlässt die Gruppe eines
> gelöschten Feeds **sofort** — **ohne Neustart**. Die CLI (`feed add`/`feed
> list`) bleibt für Skripting/CI erhalten; beide Wege schreiben denselben Katalog.

> **Orchestrierter (Auto-Spawn-)Modus — Feed-Quellen:** Dort trägt ein Feed
> zusätzlich eine **Quellen-Konfiguration**, die den gespawnten Firefly-Tracker
> speist. Quell-Typen: `adsb_opensky` (Konto/OAuth2), **`adsb_aggregator`**
> (auth-frei via adsb.lol/adsb.fi — Provider `adsb_lol`/`adsb_fi`, Firefly ADR 0031;
> nutzbar auch dort, wo OpenSky Datacenter-IPs sperrt, z. B. Codespaces),
> `flarm_aprs`, `radar_asterix` (CAT048), sowie die lokalen ASTERIX-über-UDP-
> Push-Quellen **`adsb_asterix`** (eigene ADS-B-Bodenstation, CAT021/UDP,
> Firefly-Kontrakt v1.6.0) und **`mlat_asterix`** (WAM/MLAT, CAT020/019/UDP,
> v1.7.0) — beide auth-frei (Vertrauensgrenze = Netz-Isolation), konfiguriert mit
> Listen-Endpoint + optional SAC/SIC + Sensor-ID (keine `bbox`, kein Standort,
> keine Credentials). Anlegen über die Admin-UI (Feed-Zeile → **„Quellen"**)
> oder `PUT /api/admin/feeds/{id}/sources`. Schritt-für-Schritt in
> `docs/E2E-ABNAHME.md`, API-Kontrakt in `docs/TECHNICAL.md`.

### Schritt 4.6 — Alles starten

```bash
docker compose up -d --build
```

(`-d` = im Hintergrund. Logs ansehen mit `docker compose logs -f wayfinder`.)

### Schritt 4.7 — Als Administrator anmelden

Öffnen Sie **<http://localhost:8081/admin>** und melden Sie sich an:

- **Benutzername:** `admin`
- **Passwort:** `admin` (bei Zero-Touch-Schnellstart — Sie werden sofort zum Wechsel gezwungen) **oder** das in Schritt 4.4 selbst gewählte Passwort

> **Hinweis (ADR 0022):** Der Passwortwechsel wird auf **jedem** Anmeldeweg
> erzwungen — auch eine Anmeldung über die Lagebild-Seite (`/`) führt direkt in
> die Verwaltung zur Passwort-Maske; bis zur Änderung weist der Server alle
> Daten-Pfade ab (`403 password_change_required`). Außerdem hat ein Admin
> **kein eigenes Lagebild**: der Aufruf von `/` als Admin leitet nach `/admin`
> um; die Lage sehen Sie als Admin ausschließlich über den **Gastmodus**
> (Schritt 4.11).

Sie sehen die Admin-Oberfläche. Seit AP3 (ADR 0009) ist sie
**mandantenzentriert**: eine **Übersicht aller Mandanten** (mit Status und
aktiven Features). Die betrieblichen Funktionen haben in der Übersicht **je eine
eigene Spalte mit einem Konfigurations-Icon (⚙)**, das einen fokussierten Dialog
öffnet: **Feeds**, **OpenAIP** und **Nutzer** (Zugänge). Ein Klick auf
**„Konfigurieren"** öffnet die **Detailseite** des Mandanten, die auf die
**Standard-Ansicht** (Zentrum + Radius, FL-Band) und die **Features** reduziert
ist; dort werden Änderungen erst mit dem **einen globalen „Speichern"** aktiv
(„Abbrechen" verwirft sie), und danach kehren Sie in die Übersicht zurück. Der
Read-Only-Einblick in einen Kunden startet über das **Augen-Icon** in der Spalte
**„Gastmodus"** (Schritt 4.11).

> **Feed-Gesundheit (AP4, ADR 0009):** Die einem Mandanten zugewiesenen Feeds
> tragen in Übersicht und Detailseite einen farbigen **Ampel-Chip**:
> **grün** = Heartbeat kommt an — auch bei leerem Himmel, kein Verkehr ist kein
> Fehler; **gelb** = Sensor-Teilausfall (mindestens ein Radar abgefallen, aber
> noch mindestens eines aktiv; CAT063, ADR 0010);
> **rot** = kein Heartbeat mehr („toter Feed", z. B. Firefly-Sender aus oder
> Netz-Pfad gestört). So unterscheiden Sie auf einen Blick einen *ruhigen Himmel*
> von einem *ausgefallenen Feed* oder einem *degradierten Multi-Radar-System*.
> Die Werte stammen aus derselben Quelle wie die `/metrics`-Felder
> `wayfinder_feed_stale` / `wayfinder_cat065_heartbeats_received_total`.

> **Rollen:** Es gibt genau zwei Rollen — **`admin`** (Plattform-Betreiber:
> verwaltet **alle** Mandanten, Feeds und Zugänge; hat Zugang zum kompletten
> Admin-Bereich) und **`user`** (Endnutzer/Lotse eines Mandanten: meldet sich an
> und sieht nur das ihm zugewiesene Lagebild). Ein Kunde bekommt also
> `user`-Zugänge; den Admin-Bereich (`/admin`) erreicht nur `admin`.

### Schritt 4.8 — Einen Kunden-Mandanten anlegen

> 🖥️ **Seit ONB-4 (ADR 0011) bequem über die Oberfläche:** Mandanten legen Sie
> im Admin-Bereich unter „Mandanten → Mandant anlegen" an (und löschen sie dort —
> ein Mandant mit noch vorhandenen Zugängen wird aus Sicherheitsgründen erst nach
> Entfernen der Konten gelöscht). Der folgende CLI-Weg bleibt als
> Skript-/Automatisierungs-Pfad erhalten.

Für jeden Kunden legen Sie einen eigenen Mandanten mit einem ersten `user`-Zugang
an — per Oberfläche oder mit `bootstrap` (weitere Zugänge danach bequem über die
Oberfläche, Schritt 4.8b):

```bash
WAYFINDER_BOOTSTRAP_PASSWORD='KundePasswort456' \
docker compose run --rm \
  -e WAYFINDER_BOOTSTRAP_PASSWORD \
  wayfinder bootstrap \
    -tenant kunde-nord \
    -tenant-name "Kunde Nord GmbH" \
    -subject anna \
    -role user
```

### Schritt 4.8b — Weitere Zugänge verwalten (Oberfläche, AP6)

Im Admin-Bereich öffnen Sie in der **Mandanten-Übersicht** über das
Konfigurations-Icon (⚙) in der Spalte **„Nutzer"** den Zugänge-Dialog des
gewünschten Mandanten und verwalten dessen Login-Konten direkt im Browser —
**ohne** `bootstrap`/SQL:

- **Zugang anlegen** (Benutzername, optional E-Mail und Passwort; Rolle ist
  immer `user`; ein Passwort muss mindestens 8 Zeichen haben). Ohne Passwort
  entsteht ein Konto für den Proxy-/OIDC-Betrieb (kein lokales Passwort).
- **Pausieren / Reaktivieren** eines Zugangs: Ein pausierter Zugang **kann sich
  nicht mehr anmelden** (fail-closed), seine Konfiguration bleibt aber erhalten.
- **Passwort setzen/zurücksetzen** und **Löschen** eines Zugangs.
- **Mandant pausieren:** sperrt **alle** Zugänge dieses Kunden für die Anmeldung
  auf einmal (z. B. bei Zahlungsstopp), ohne Daten zu verlieren.

> **Hinweis:** Pausieren/Löschen wirkt auf **neue** Anmeldungen sofort. Eine
> bereits laufende Sitzung läuft noch bis zum Ablauf des Session-Cookies weiter —
> das sofortige Beenden laufender Sitzungen kommt mit der Session-Registry (AP7).

### Schritt 4.9 — Dem Kunden einen Feed zuweisen

Damit der neue Kunde Flugzeuge sieht, muss ihm ein Feed **zugewiesen** werden.
Das darf nur ein `admin`. Zwei Wege:

**Weg A — über die Admin-Oberfläche (empfohlen):** In `/admin` als `admin`
angemeldet, in der Mandanten-Übersicht in der Zeile **„Kunde Nord GmbH"** über das
Konfigurations-Icon (⚙) in der Spalte **„Feeds"** den Feed **„Frankfurt"** zuweisen.

**Weg B — über die Befehlszeile (mit `curl`):** Zuerst Mandanten- und Feed-IDs
herausfinden, dann zuweisen. (`{tenant-id}` / `feed_id` aus `feed list` bzw. der
Admin-Liste.)

```bash
# Beispiel: Feed 1 dem Mandanten 2 zuweisen — als angemeldeter admin.
curl -X POST http://localhost:8081/api/admin/tenants/2/subscriptions \
  -H 'Content-Type: application/json' \
  -d '{"feed_id":1}'
```

### Schritt 4.10 — Als Kunde anmelden und prüfen

Melden Sie sich (am besten in einem **privaten Browserfenster**) unter
**<http://localhost:8081/admin>** als `anna` an. Auf dem Lagebild
(**<http://localhost:8081>**) sieht „Kunde Nord" nun **genau** die Flugzeuge des
zugewiesenen Feeds — und **keine** anderen.

✅ **Fertig!** Sie haben eine Multi-Tenant-Plattform aufgesetzt. Weitere Kunden:
Schritte 4.8 + 4.9 wiederholen.

### Schritt 4.11 — „View as Tenant": die Sicht eines Kunden einsehen (nur `admin`)

Für den Support gibt es einen **Read-Only-Einblick**: ein `admin` kann die
Lage **so sehen, wie ein bestimmter Kunde sie sieht** — ohne dessen Passwort, nur
lesend, vollständig protokolliert (ADR 0008).

So funktioniert es im Browser:

1. Als `admin` im Admin-Bereich **<http://localhost:8081/admin>** in der
   **Mandanten-Übersicht** in der Spalte **„Gastmodus"** auf das **Augen-Icon**
   der gewünschten Zeile (z. B. „Kunde Nord GmbH") klicken.
2. Die Karte öffnet sofort **dessen** Feeds und Sicht; ein **gelber Banner** zeigt
   „Sie betrachten **Kunde Nord GmbH** — nur Lesen".
3. Im Banner kann man per **„Mandant wechseln"** direkt zu einem anderen Kunden
   springen oder mit **„Beenden"** in die Verwaltung (`/admin`) zurückkehren —
   ein eigenes Admin-Lagebild gibt es nicht (ADR 0022); ohne aktiven Gastmodus
   weist der Server den Lagebild-Zugang eines Admins ab.

**Wichtig zu wissen:**

- **Nur lesend:** Es lässt sich nichts im Namen des Kunden ändern — Verwaltung
  läuft immer über die echte Identität.
- **Nur `admin`:** Nutzer mit Rolle `user` sehen die Funktion nicht; ein
  gefälschter Zugriffsversuch wird serverseitig **laut abgewiesen und ins
  Audit-Log geschrieben**.
- **Zeitlich befristet:** Der Einblick läuft nach `WAYFINDER_IMPERSONATION_TTL`
  (Standard 30 min) automatisch ab.
- **Voraussetzung:** Ein Signing-Key (`WAYFINDER_SESSION_KEY`) muss gesetzt sein —
  im `builtin`-Aufbau aus Teil 4 ist das bereits der Fall.

>> 📖 Die laufende Aufsicht über diese Einblicke (Audit-Spur „wer sah welchen
> Mandanten") ist im **Betriebsführungshandbuch** (`docs/BETRIEB.md`, Abschnitt
> Sicherheits-Betrieb) beschrieben.

---

## Teil 5 — Läuft es? — Verifikation

Diese Prüfungen funktionieren im laufenden Aufbau. Im Terminal:

### 5.1 Läuft der Dienst überhaupt? (Liveness)
```bash
curl -s http://localhost:8080/health
# Erwartet:  ok
```

### 5.2 Kommen Daten an? (Readiness)
```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/ready
# 200 = Feed aktiv (mindestens ein Lebenszeichen empfangen)
# 503 = Feed noch nie gesehen oder gerade ausgefallen
```

### 5.3 Im Browser
Öffnen Sie **<http://localhost:8081>**. Die dunkle Karte erscheint sofort;
Flugzeuge erscheinen, sobald Firefly sendet — erkennbar am grünen Banner
**FEED OK** oben links.

### 5.4 Zahlen/Metriken (optional)
```bash
curl -s http://localhost:8080/metrics | grep wayfinder_feed_stale
# wayfinder_feed_stale 0   ← 0 bedeutet: Feed ist frisch/gesund
```

---

## Teil 6 — Wenn etwas nicht geht — Fehlersuche

| Symptom | Wahrscheinliche Ursache & Lösung |
|---------|----------------------------------|
| **Karte bleibt leer, keine Flugzeuge** | Läuft Firefly? `docker compose ps` zeigt alle Dienste als `running`. Logs prüfen: `docker compose logs firefly`. Stimmen `FIREFLY_CAT062_GROUP`/`_PORT` bei **beiden** Diensten überein? |
| **`/ready` liefert 503** | Es kam noch kein Lebenszeichen (CAT065-Heartbeat). Normal direkt nach dem Start — ein paar Sekunden warten. Bleibt es 503: Firefly sendet nicht / Multicast kommt nicht an (siehe nächste Zeile). |
| **Flugzeuge erscheinen nie, obwohl Firefly läuft** | Multicast wird im Container-Netz nicht durchgereicht. Stellen Sie sicher, dass Sie die **gemeinsame** Compose aus dieser Anleitung verwenden (Firefly **und** Wayfinder im selben `asd`-Netz). Trennen Sie sie nicht auf zwei Compose-Dateien. |
| **`docker compose up` bricht mit Build-Fehler ab** | Erstes Bauen braucht Internet (lädt Abhängigkeiten). Verbindung prüfen, dann `docker compose build --no-cache` erneut versuchen. |
| **`port is already allocated` (8081/8080 belegt)** | Ein anderer Dienst nutzt den Port. Anderen Port abbilden, z. B. `"9091:8081"`, dann `http://localhost:9091` öffnen. |
| **Mac: Docker-Befehle hängen / „Cannot connect to the Docker daemon"** | Docker Desktop ist nicht gestartet. Docker aus dem Launchpad öffnen, warten bis das Wal-Symbol ruhig steht. |
| **Multi-Tenant: Login schlägt fehl (401)** | Passwort falsch; **oder** der Zugang bzw. sein Mandant ist **pausiert** (Schritt 4.8b — fail-closed ist Absicht, im Reiter „Zugänge" reaktivieren); **oder** `WAYFINDER_SESSION_KEY` fehlt/ist zu kurz (Schlüssel setzen mit `openssl rand -hex 32`, Container neu starten, `bootstrap` ggf. erneut ausführen). |
| **Multi-Tenant: Kunde sieht keine Flugzeuge** | Dem Mandanten wurde **kein Feed zugewiesen** (Schritt 4.9) — fail-closed ist Absicht. Zuweisung als `admin` nachholen. |
| **Logs ansehen** | `docker compose logs -f wayfinder` (live mitlaufen, `Strg+C` beendet die Anzeige, **nicht** den Dienst). |
| **Alles sauber neu aufsetzen** | `docker compose down -v` löscht Container **und** die Datenbank-Daten (`-v`!). Danach bei Teil 4 neu beginnen. |

---

## Teil 7 — Konfigurationsreferenz

Konfiguriert wird über **Umgebungsvariablen** (im `environment:`-Block der
`docker-compose.yml`) und optional über die **YAML-Datei** `wayfinder.yaml` (nur
`map` + `openaip`). **Umgebungsvariablen gewinnen immer** (12-Factor).

### 7.1 Netzwerk & Feed

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `FIREFLY_CAT062_GROUP` | `239.255.0.62` | UDP-Multicast-Gruppe für den CAT062/CAT065-Eingang |
| `FIREFLY_CAT062_PORT` | `8600` | UDP-Port des Multicast-Stroms |
| `WAYFINDER_FEED_ID` | `0` | Legacy-Einzel-Feed-ID; im Multi-Feed-Betrieb liefert der DB-Katalog die IDs. |
| `WAYFINDER_PROBE_PORT` | `8080` | Port für `/health`, `/ready`, `/metrics` |
| `WAYFINDER_FEED_STALE_TIMEOUT` | `3` | Sekunden ohne Lebenszeichen, ab denen der Feed als „stale" gilt |
| `WAYFINDER_FEED_GROUP_BASE` | `239.255.0` | /24-Basis für die **automatische** Multicast-Endpoint-Vergabe beim Feed-Anlegen (eine Gruppe pro Feed, orchestrierter Modus). |
| `WAYFINDER_FEED_PORT` | `8600` | Port für auto-vergebene Feed-Endpunkte. |
| `WAYFINDER_FEED_OCTET_MIN` | `1` | Kleinstes Host-Oktett des Auto-Vergabe-Pools. |
| `WAYFINDER_FEED_OCTET_MAX` | `254` | Größtes Host-Oktett des Pools (~254 Feeds; auf /16 erweiterbar). |

### 7.2 Karte & Darstellung

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_MAP_CENTER_LAT` | `50.0379` | Breitengrad des Kartenstartzentrums (Frankfurt) |
| `WAYFINDER_MAP_CENTER_LON` | `8.5622` | Längengrad des Kartenstartzentrums |
| `WAYFINDER_MAP_ZOOM` | `8` | Anfangs-Zoom (1–22) |
| `WAYFINDER_MAP_THEME` | `dark` | `dark` (CARTO Dark, schlüsselfrei), `bkg` (**amtliche basemap.de-Vektorkarte** des BKG, hell, ADR 0026), `bkg-dark` (**dunkler Radar-Scope** aus denselben amtlichen Daten, H2) oder `osm` (OpenStreetMap-Raster, deprecated) |
| `WAYFINDER_BKG_STYLE_URL` | basemap.world-„Farbe"-Style | Upstream-Style für `bkg`/`bkg-dark` (Default: `bm_web_wld_col.json` — innerhalb Deutschlands amtlich, außerhalb BKG-kuratierter Weltkontext); Alternativen: Nur-Deutschland `bm_web_col.json`, Grau `bm_web_gry.json` oder self-hosted Mirror |
| `WAYFINDER_MAP_STYLE_URL` | *(leer)* | Vollständige MapLibre-Style-URL — überschreibt `WAYFINDER_MAP_THEME`. Für basemap.de **nicht** hier eintragen, sondern `WAYFINDER_MAP_THEME=bkg` nutzen (sonst fehlen die Track-Label-Schriften, ADR 0026) |

> Dieselben drei `map`-Werte lassen sich auch in `wayfinder.yaml` setzen (siehe
> Schritt 4.3). Die Umgebungsvariable gewinnt, falls beides gesetzt ist.

### 7.3 Aeronautische Daten (OpenAIP, optional)

Ohne `WAYFINDER_OPENAIP_API_KEY` ist das Feature aus (Warn-Log, kein Fehler).

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_OPENAIP_API_KEY` | *(leer)* | **Globaler** OpenAIP-API-Schlüssel; leer = Feature global aus |
| `WAYFINDER_OPENAIP_RADIUS_KM` | `250` | Umkreis um das Zentrum für Luftraum-/Navaid-Abfragen (auch via `wayfinder.yaml` → `openaip.radius_km`) |
| `WAYFINDER_OPENAIP_REFRESH` | *(ignoriert)* | **Deprecated seit AERO-1 (ADR 0018)** — OpenAIP wird **einmalig/on-demand** geholt und **persistiert**, nicht mehr periodisch. Ein gesetzter Wert wird ignoriert (Warn-Log), bricht aber den Start nicht |
| `WAYFINDER_OPENAIP_BASE_URL` | *(intern)* | Override der OpenAIP-Basis-URL (Tests/Proxies) |

> **Persistenter Cache & Fetch-once (AERO-1, ADR 0018).** Die geholten
> Aeronautik-Daten (Luftraum/Navaids/Wegpunkte) werden in der DB **persistiert**
> (Tabelle `aeronautical_cache`) und überleben einen **Redeploy** — beim Start wird
> der Cache **ohne Netz** aus der DB geladen. Ein OpenAIP-Abruf passiert nur noch
> **ereignisgesteuert**: Erstbefüllung (Schlüssel vorhanden, aber nichts
> persistiert), AOI-Änderung oder ein **expliziter Refresh** (Schlüssel gesetzt/
> geändert). Aeronautik-Daten folgen dem **AIRAC-Zyklus** (28 Tage) — ein
> Dauer-Polling gibt es bewusst nicht mehr. Der Admin-Status-Endpunkt zeigt je
> Mandant, **wann** zuletzt geholt wurde und **wie viele** Objekte gecacht sind.

> **OpenAIP pro Mandant (ONB-6, ADR 0011).** Im Multi-Mandanten-Betrieb kann
> jeder Mandant einen **eigenen** OpenAIP-Schlüssel bekommen — im Admin-Dashboard
> auf der Mandanten-**Detailseite** unter „OpenAIP-Konfiguration" (Schlüssel
> setzen/löschen; der gesetzte Schlüssel wird aus Sicherheitsgründen **nie wieder
> angezeigt**, nur sein Status). Jeder Mandant ruft die Luftraumdaten dann mit
> seinem eigenen Konto und gegen **seine eigene Sicht-AOI** ab (Zentrum + Radius
> bzw. die gesetzte AOI-Box). Mandanten **ohne** eigenen Schlüssel nutzen den
> globalen `WAYFINDER_OPENAIP_API_KEY` als Rückfall. Ändert ein Admin den
> Schlüssel oder die Sicht eines Mandanten, greift das **sofort** (kein Neustart).
> Die Endpunkte `/api/airspace`, `/api/navaids`, `/api/waypoints` liefern damit im
> Multi-Mandanten-Betrieb je Anmeldung die Daten des **eigenen** Mandanten.

> **Globaler Schlüssel via UI + Refresh-Buttons (AERO-2, ADR 0018).** Der
> **globale** Rückfall-Schlüssel lässt sich zur Laufzeit über die Admin-UI setzen
> (Kopf-Navigation **„OpenAIP"**), statt nur über die Env `WAYFINDER_OPENAIP_API_KEY`.
> Er wird **verschlüsselt** gespeichert (AES-256-GCM) und braucht dafür einen
> gesetzten **`WAYFINDER_SECRET_KEY`** — ohne ihn ist die UI-Route bewusst
> deaktiviert (`503`, kein Klartext-Geheimnis in der DB); die Env bleibt dann der
> einzige Weg. Ein UI-gesetzter Schlüssel **gewinnt** über die Env und greift
> **sofort**; das Setzen löst automatisch einen **Abruf für alle Mandanten** aus.
> Zusätzlich gibt es Refresh-Buttons **global** („Alle Mandanten aktualisieren")
> und **pro Mandant** („Jetzt aktualisieren", z. B. zum AIRAC-Stichtag), plus die
> Anzeige „zuletzt geholt / N Objekte" je Mandant. *(Die per-Mandant-Schlüssel
> liegen weiterhin unverschlüsselt in der DB — deren Versiegelung ist ein möglicher
> Folge-Schritt.)*

> **AIRAC-Kalender + Change-Impact (AERO-3, ADR 0018).** Die „OpenAIP"-Sektion zeigt
> den **aktuellen AIRAC-Zyklus** und den **nächsten Stichtag** (deterministisch aus
> dem 28-Tage-Raster berechnet — **keine** externe Quelle), damit der Betreiber den
> Refresh rund um den AIRAC-Wechsel planen kann. Nach jedem Abruf zeigt die
> Mandanten-Detailseite je Ebene den **Change-Impact** („Luftraum 142 → 145,
> +5/−2"). **Ehrliche Grenze:** der Count-Delta (142 → 145) ist exakt; die
> `+hinzu/−entfernt`-Zahlen sind **Churn** (ein In-Place-Edit zählt als −1/+1) und
> werden über einen Inhalts-Hash bestimmt — eine **namentliche** Zuordnung („genau
> diese Flugplätze") ist bewusst **nicht** enthalten.

### 7.4 Wetter-Overlays & QNH (DWD/NOAA, optional)

Best-effort Wetter-Kontext aus offenen Quellen (ADR 0016). Der Track-Pfad
(CAT062) ist davon unabhängig — ein Ausfall lässt nur das Overlay/QNH fehlen.

**Wetter-Overlay (DWD-Radar, WX-A).** **Connected-by-default (ADR 0017): standardmäßig AN** —
die WMS-URL zeigt per Default auf den öffentlichen DWD-GeoServer. Der Mandant
braucht nur noch das Feature-Entitlement `weather_radar`, damit der Schalter
erscheint und **sofort** funktioniert. Abschalten mit
`WAYFINDER_DWD_RADAR_ENABLED=false` (best-effort — eine unerreichbare Quelle liefert
transparente Kacheln, nie einen Fehler).

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_DWD_RADAR_ENABLED` | `true` | Radar-Overlay an/aus. `false` = Opt-out (keine Abfrage an DWD) |
| `WAYFINDER_DWD_WMS_URL` | `https://maps.dwd.de/geoserver/dwd/wms` | DWD-GeoServer-WMS-Basis-URL (Override, z. B. eigener Mirror) |
| `WAYFINDER_DWD_RADAR_LAYER` | `dwd:Niederschlagsradar` | WMS-Layer-Name des Radar-/Niederschlagskomposits |
| `WAYFINDER_DWD_RADAR_STYLE` | (leer) | Optionaler WMS-`STYLES`-Parameter (#189); leer = Default-Style. Für ein echo-only-Rendering ohne Messbereichs-Grau/Stationsringe (Style-Name gegen den DWD-GeoServer verifizieren) |
| `WAYFINDER_DWD_REFRESH` | `5m` | Cache-Lebensdauer je Radar-Kachel (DWD-Radar aktualisiert ~5 min) |

**QNH-Infobox (NOAA-METAR, WX-B / CBD-3).** Zeigt das aktuelle QNH (Höhenmesser-
Einstellung, hPa) des Flugplatzes des Mandanten in der Kopfzeile.
**Connected-by-default (ADR 0017): die NOAA-Quelle ist standardmäßig AN**,
abschaltbar mit `WAYFINDER_QNH_ENABLED=false`. Welcher Flugplatz angezeigt wird,
setzt der Admin **pro Mandant** in der Admin-UI (Feld **„QNH-Flugplatz (ICAO)"**,
`qnh_icao`, echter 4-stelliger ICAO wie `EDDH`) — der Poller fragt die Vereinigung
aller Mandanten-Flugplätze ab. Zusätzlich braucht der Mandant das Entitlement
`qnh`. Ohne gesetzten Flugplatz zeigt die Kopfzeile nichts (Quelle an, aber nichts
abzufragen). **Wichtig:** QNH kommt **nur aus echtem METAR** (NOAA-Feld `altim`),
**nicht** aus DWD-Druckdaten (PMSL/MOSMIX sind eine andere Größe).

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_QNH_ENABLED` | `true` | QNH-Quelle (NOAA-METAR) an/aus. `false` = Opt-out (keine Abfrage an `aviationweather.gov`) |
| `WAYFINDER_METAR_STATIONS` | *(leer)* | **Deprecated globaler Fallback** — Kommaliste von ICAO-Flugplätzen (Prioritätsreihenfolge) für Mandanten **ohne** eigenen `qnh_icao`. Der per-Mandant-Flugplatz aus der Admin-UI ist der vorgesehene Weg |
| `WAYFINDER_METAR_URL` | *(NOAA AWC)* | METAR-Daten-API (Default `https://aviationweather.gov/api/data/metar`) |
| `WAYFINDER_METAR_USER_AGENT` | `Wayfinder-ASD/1.0` | Distinktiver User-Agent (leere/Default-UAs werden von der AWC gefiltert → 403) |
| `WAYFINDER_QNH_REFRESH` | `15m` | METAR-Poll-Intervall (METAR ~30 min; unter dem AWC-Limit von ~100 req/min) |

**Wetterwarnungen-Overlay (DWD-WFS, WX-C).** Amtliche DWD-Warnpolygone (Gewitter,
Sturm, Schnee/Eis …), nach Warnstufe eingefärbt. **Connected-by-default (ADR 0017):
standardmäßig AN**; Mandant braucht nur das Entitlement `weather_warnings`.
Abschalten mit `WAYFINDER_DWD_WARN_ENABLED=false`.

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_DWD_WARN_ENABLED` | `true` | Warnungen-Overlay an/aus. `false` = Opt-out |
| `WAYFINDER_DWD_WARN_URL` | `https://maps.dwd.de/geoserver/dwd/ows` | DWD-GeoServer-WFS/OWS-Basis-URL (Override) |
| `WAYFINDER_DWD_WARN_LAYER` | `dwd:Warnungen_Gemeinden_vereinigt` | WFS-Layer (aufgelöste Gemeinde-Warnungen; leichtgewichtig) |
| `WAYFINDER_DWD_WARN_REFRESH` | `5m` | Poll-Intervall des Warn-Feeds |

> **Ausgehender Netzzugang (Vertrauensgrenze, ADR 0016).** Wayfinder holt Radar,
> Warnungen und QNH **server-seitig** und liefert sie same-origin an den Browser
> (`/api/weather/radar/{z}/{x}/{y}.png`, `/api/weather/warnings.geojson`,
> `/api/weather/qnh`). Das **Deployment-Netz muss daher ausgehend `maps.dwd.de`
> (Radar + Warnungen) bzw. `aviationweather.gov` (QNH), jeweils HTTPS/443,
> erreichen dürfen.** Alle Abrufe sind best-effort und
> misstrauisch (Timeout, Größenlimit, kein Absturz auf Fehldaten) und blockieren
> nie `/ready`.
>
> **Lizenz/Attribution.** DWD-Daten sind frei unter GeoNutzV/CC BY 4.0
> („© Deutscher Wetterdienst", im Karten-Overlay gesetzt); NOAA/NWS-METAR ist
> US-Government Public Domain.

### 7.5 Sicherheit (Browser-Rand)

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_ALLOWED_ORIGINS` | *(leer)* | Kommaliste erlaubter Cross-Origin-Domains für `/ws`. Leer = nur Same-Origin |
| `WAYFINDER_TLS_CERT` | *(leer)* | Pfad zum TLS-Zertifikat (PEM). Nur aktiv, wenn beide TLS-Werte gesetzt sind |
| `WAYFINDER_TLS_KEY` | *(leer)* | Pfad zum TLS-Schlüssel (PEM) |
| `WAYFINDER_SECRET_KEY` | *(leer)* | Base64-kodierter 32-Byte-AES-256-GCM-Schlüssel für **ruhende Geheimnisse**: verschlüsselt Pro-Feed-Quell-Credentials (`PUT …/feeds/{id}/sources`) und den UI-gesetzten globalen OpenAIP-Schlüssel. Leer/ungültig → die betroffenen Secret-Routen sind deaktiviert (`503`), nie Klartext at rest. Im **orchestrierten** Modus muss **derselbe** Wert auch am Orchestrator-Prozess gesetzt sein (`cmd/wayfinder-orchestrator`), sonst laufen credentialled Quellen anonym. Erzeugen: `openssl rand -base64 32`. |
| `WAYFINDER_FIREFLY_COMMAND_TOKEN` | *(leer)* | Deployment-weites Bearer-Token für die **manuelle Flugplan-Korrelation** (`POST/DELETE /api/correlation`, ADR 0024, #245 Teil B). Gesetzt → der Endpoint ist aktiv und schickt Kommandos an die feed-eigene Firefly-Instanz; leer → Endpoint deaktiviert (`503`). Im **orchestrierten** Modus muss **derselbe** Wert auch am Orchestrator-Prozess (`cmd/wayfinder-orchestrator`) gesetzt sein: dieser injiziert ihn als `FIREFLY_WS_TOKEN` in jede gespawnte Firefly-Instanz, damit deren Kommando-API genau dieses Bearer verlangt (ohne die Injektion würde Firefly die Kommandos mit `401` ablehnen). Nur relevant, wenn Lotsen manuell korrelieren können sollen. Erzeugen: `openssl rand -hex 32`. |

> Der eigentliche Login am Browser-Rand läuft über die Mandanten-Authentifizierung
> (`WAYFINDER_AUTH_MODE`, siehe §7.6); `/ws` ist immer durch die
> Mandanten-Middleware geschützt (fail-closed).

### 7.6 Multi-Mandanten

`WAYFINDER_DB_URL` ist **Pflicht** (ADR 0014): Ohne diese Variable **startet der
Server nicht**. Mit gesetzter DB werden die Schema-Migrationen beim Start
angewandt und `/ws` ist durch die Mandanten-Middleware geschützt (fail-closed:
ohne gültigen, einem Mandanten zugeordneten Nutzer → `401`).

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_DB_URL` | *(Pflicht)* | PostgreSQL-DSN, z. B. `postgres://user:pass@host:5432/wayfinder?sslmode=disable`. **Pflichtfeld**; ohne DB bricht der Start ab. |
| `WAYFINDER_AUTH_MODE` | `builtin` | `builtin` (eingebaute Nutzer + Session-Cookie) oder `proxy` (OIDC-Token vom Reverse-Proxy) |
| `WAYFINDER_SESSION_KEY` | *(leer)* | `builtin`: HMAC-Schlüssel zum Signieren der Session-Cookies (**Pflicht** im builtin-Modus; ≥ 32 zufällige Zeichen) |
| `WAYFINDER_SESSION_COOKIE` | `wf_session` | `builtin`: Name der Session-Cookie |
| `WAYFINDER_SESSION_TTL` | `12h` | `builtin`: Session-Lebensdauer = Sliding-Idle-Fenster (`8h`, `12h` …) |
| `WAYFINDER_SESSION_MAX_LIFETIME` | *(leer = aus)* | `builtin`: **absolutes** Sitzungs-Maximum ab Erst-Login, unabhängig von Aktivität (`0`/leer = aus, Default). Gesetzt (z. B. `12h`) erzwingt es nach dieser Spanne einen Neu-Login, auch bei aktiver Konsole. **Probelauf:** `30m` setzen, um das Zwangs-Logout schnell zu sehen. |
| `WAYFINDER_SESSION_LIMIT_DEFAULT` | `0` (unbegrenzt) | `builtin` (AP7): Default-Limit **gleichzeitiger** Sessions je Zugang, wenn der Zugang kein eigenes Limit trägt. `0`/leer = unbegrenzt (opt-in, Default). Positiver Wert (z. B. `3`) begrenzt parallele Logins pro Zugang. |
| `WAYFINDER_SESSION_LIMIT_POLICY` | `reject` | `builtin` (AP7): Verhalten am Limit — `reject` (Default: N+1-ter Login → `429`, alte Sessions bleiben) oder `evict_oldest` (älteste Session verdrängen, neuen Login zulassen). |
| `WAYFINDER_IMPERSONATION_TTL` | `30m` | Lebensdauer des Read-Only-Impersonation-Grants („View as Tenant", ADR 0008). Nur wirksam, wenn ein Signing-Key (`WAYFINDER_SESSION_KEY`) gesetzt ist; sonst ist Impersonation deaktiviert. |
| `WAYFINDER_OIDC_ISSUER` | *(leer)* | `proxy`: OIDC-Issuer-URL (Pflicht im proxy-Modus) |
| `WAYFINDER_OIDC_AUDIENCE` | *(leer)* | `proxy`: erwartete Audience/Client-ID (Pflicht im proxy-Modus) |

#### Befehl `bootstrap` — ersten Mandanten/Nutzer anlegen

| Flag / Variable | Default | Beschreibung |
|-----------------|---------|--------------|
| `-tenant` | *(leer)* | Mandanten-Slug (z. B. `kunde-nord`). **Pflicht** für `-role user`; **nicht benötigt** für `-role admin` (Admins sind mandantenlos) |
| `-tenant-name` | = Slug | Anzeigename des Mandanten (nur wenn `-tenant` gesetzt) |
| `-subject` | *(Pflicht)* | Benutzername (builtin) bzw. OIDC-Subject (proxy) |
| `-email` | *(leer)* | optionale E-Mail |
| `-role` | `admin` | `user` \| `admin` |
| `-password` | *(leer)* | builtin-Passwort (besser über `WAYFINDER_BOOTSTRAP_PASSWORD`) |
| `WAYFINDER_BOOTSTRAP_PASSWORD` | *(leer)* | builtin-Passwort (bevorzugt — **nicht** in der Prozessliste sichtbar) |

#### Befehl `feed add` / `feed list` — Feed-Katalog pflegen

| Flag | Default | Beschreibung |
|------|---------|--------------|
| `-name` | *(Pflicht)* | Anzeigename des Feeds |
| `-group` | *(Pflicht)* | Multicast-Gruppe, z. B. `239.255.0.62` |
| `-port` | `8600` | Multicast-Port |
| `-region` | *(leer)* | Regions-Label (optional) |
| `-sensor-mix` | *(leer)* | Kommaliste aus `PSR,SSR,MODE_S,ADS-B,MLAT,FLARM`; gängige Schreibweisen werden normalisiert, Unbekanntes abgelehnt |

#### Admin-Oberfläche & Admin-API

- **`/admin`** (Browser): mandantenzentrierte Verwaltungsoberfläche (ersetzt die
  Karte). Geschützt durch das Rollen-Gate; die Rollen-Probe liegt auf
  `GET /api/admin/whoami` (Rolle `admin`, sonst `403`). Seit AP3 (ADR 0009):
  Mandanten-Übersicht → Detailseite je Mandant (Ansicht, Features, Feeds, Zugänge).
- **`/api/admin/*`** (REST): die Selbstbedienungs-Routen (`/api/admin/view` etc.)
  leiten die Mandanten-ID **immer aus der angemeldeten Identität** ab; die
  cross-tenant Admin-Routen (`/api/admin/tenants/{id}/…`, `/api/admin/overview`)
  nehmen die Ziel-Mandanten-ID aus dem Pfad und sind `admin`-gegated
  (`requireAdmin → 403`).

| Methode + Pfad | Wirkung | Rolle |
|---|---|---|
| `GET /api/admin/whoami` | Eigene Identität/Rolle (Rollen-Probe) | admin |
| `GET /api/admin/view` · `PUT /api/admin/view` | Eigene Sicht (Zentrum/Zoom/AOI/FL) lesen/setzen | admin |
| `GET /api/admin/subscriptions` | Eigene abonnierte Feeds | admin |
| `GET /api/admin/feeds` | Feed-Katalog (read-only) | admin |
| `GET /api/admin/overview` | Alle Mandanten (Übersicht) | admin |
| `GET /api/admin/tenants` | Alle Mandanten (Liste) | admin |
| `POST /api/admin/tenants` | Mandant anlegen | admin |
| `DELETE /api/admin/tenants/{id}` | Mandant löschen (nur wenn keine Zugänge) | admin |
| `POST /api/admin/tenants/{id}/subscriptions` | Feed zuweisen (`{"feed_id":…}`), idempotent | admin |
| `DELETE /api/admin/tenants/{id}/subscriptions/{feedID}` | Feed entziehen | admin |
| `GET /api/admin/tenants/{id}/openaip` | OpenAIP-Status: Schlüssel gesetzt? + Cache-Frische (`{"configured":bool, "fetched_at"?, "feature_count"?}`, AERO-1) | admin |
| `PUT /api/admin/tenants/{id}/openaip` | OpenAIP-Schlüssel setzen/löschen (erzwingt Fetch) | admin |
| `POST /api/admin/tenants/{id}/openaip/refresh` | OpenAIP für **einen** Mandanten neu holen (AERO-2) → 202 | admin |
| `GET /api/admin/openaip` | Globaler-Schlüssel-Status (`{"configured":bool, "encryption_available":bool}`, AERO-2) | admin |
| `PUT /api/admin/openaip` | Globalen Schlüssel setzen/löschen (versiegelt; `503` ohne `WAYFINDER_SECRET_KEY`; löst Fetch-all aus) | admin |
| `POST /api/admin/openaip/refresh` | OpenAIP für **alle** Mandanten neu holen (AERO-2) → 202 | admin |
| `GET /api/admin/airac` | Aktueller AIRAC-Zyklus + nächster Stichtag (`{ident, effective, next_ident, next_effective, days_until_next}`, AERO-3; deterministisch, keine externe Quelle) | admin |
| `GET /api/admin/tenants/{id}/openaip/changes` | Change-Impact des letzten Abrufs je Ebene (`[{kind, feature_count, prev_feature_count?, added?, removed?, fetched_at}]`, AERO-3) | admin |

> 🔒 **Mandanten-Isolation:** Ein `/ws`-Client sieht **nur** Tracks aus den Feeds,
> die sein Mandant **abonniert** hat. Kein Abo → keine Tracks (fail-closed).
> Zusätzlich greift ein optionaler **Sicht-Filter** (Interessensgebiet/AOI +
> Flugflächen-Band aus `view_configs`): Tracks außerhalb verlassen den Server gar
> nicht. **fail-open:** ein Track ohne gemessene Flugfläche wird trotzdem
> zugestellt (nie ein reales Flugzeug verschlucken).
>
> 📝 **Audit-Log:** Jeder `/ws`-Connect erzeugt ein strukturiertes Log-Event
> (`component=audit`, `event=ws_connect`) mit Mandant, Nutzer und aufgelöstem
> Scope — der Compliance-Nachweis „wer sah welchen Scope". Es geht in den normalen
> Log-Strom (JSON auf `stderr`); zur Aufbewahrung in eine externe Log-Senke leiten.

### 7.7 Radarabdeckungs-Overlay (optional, Paket 6)

Sensor-Positionen/-Reichweiten für die Coverage-Ringe. `N` = 1, 2, 3, … (max. 20),
lückenlos beginnend.

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_COVERAGE_SENSOR_N_LAT` | *(leer)* | Breitengrad des Radarstandorts |
| `WAYFINDER_COVERAGE_SENSOR_N_LON` | *(leer)* | Längengrad des Radarstandorts |
| `WAYFINDER_COVERAGE_SENSOR_N_MAX_RANGE_M` | *(leer)* | Max. Reichweite in Metern (Pflicht; 0 = überspringen) |
| `WAYFINDER_COVERAGE_SENSOR_N_MIN_RANGE_M` | `0` | Innerer Blindbereich in Metern |
| `WAYFINDER_COVERAGE_SENSOR_N_LABEL` | *(leer)* | Tooltip-Bezeichnung |
| `WAYFINDER_COVERAGE_RING_COLOR` | `#5B8DEF` | Farbe aller Ringe (CSS-Hex) |

### 7.8 Betrieb

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` (ungültig → `info`) |
| `WAYFINDER_CONFIG_FILE` | `wayfinder.yaml` | Pfad zur optionalen YAML-Datei. Fehlende Datei ist nicht fatal |

### 7.9 Vollständige `wayfinder.yaml`

```yaml
# wayfinder.yaml — die einzigen unterstützten Felder.
# Alle anderen Einstellungen laufen über Umgebungsvariablen (siehe oben).
map:
  center_lat: 50.0379    # Breitengrad des Startzentrums
  center_lon: 8.5622     # Längengrad des Startzentrums
  zoom: 8                # Anfangs-Zoom (1–22)
openaip:
  radius_km: 250         # Umkreis für aeronautische Daten (km)
```

---

## Teil 8 — Produktionsbetrieb

Wayfinder ist ein **12-Factor-Service** und eignet sich direkt für Kubernetes.
Die folgenden Hinweise richten sich an Betriebs-/IT-Teams.

### 8.0 Netzwerk-Anforderungen: ausgehende Verbindungen (Egress)

> **Wayfinder ist für den vernetzten Betrieb ausgelegt (ADR 0017).** Es ist ein
> System zur **Informationsbereitstellung/Lagedarstellung**, nicht zur Steuerung
> von Flugbewegungen — die externen Kontext-Quellen (Karte, Wetter, Aeronautik)
> sind **standardmäßig aktiv**.

Das Deployment-Netz muss **ausgehend** (HTTPS/443) folgende Ziele erreichen können:

| Ziel | Wofür | Abschaltbar per |
|------|-------|-----------------|
| Karten-Tile-CDN (`tile.openstreetmap.org` bzw. `basemaps.cartocdn.com`) | Basiskarte (Themes `osm`/`dark`) | eigener `WAYFINDER_MAP_STYLE_URL` (self-hosted) |
| `sgx.geodatenzentrum.de` (BKG) | Amtliche Basiskarte (Theme `bkg`, ADR 0026): Style/Glyphs holt der **Server**, Kacheln/Sprite der **Browser** | `WAYFINDER_BKG_STYLE_URL` auf einen self-hosted Mirror zeigen lassen |
| `maps.dwd.de` | DWD-Radar + Wetterwarnungen | `WAYFINDER_DWD_RADAR_ENABLED=false` / `_WARN_ENABLED=false` |
| `aviationweather.gov` | QNH (NOAA-METAR) | `WAYFINDER_QNH_ENABLED=false` |
| `api.core.openaip.net` | Luftraum/Navaids/Wegpunkte (OpenAIP) | kein globaler Schlüssel gesetzt = keine Abfrage |

> **Rollout-Stand:** **DWD-Radar + Warnungen** (ADR 0017, abschaltbar per
> `WAYFINDER_DWD_RADAR_ENABLED` / `_WARN_ENABLED=false`) und **QNH** (NOAA,
> abschaltbar per `WAYFINDER_QNH_ENABLED=false`) sind **default-an**. Die
> QNH-Anzeige braucht zusätzlich einen **pro-Mandant** gesetzten Flugplatz-ICAO
> (Admin-UI, `qnh_icao`) und das Entitlement `qnh` — die Quelle selbst ist an, aber
> ohne Flugplatz gibt es nichts zu zeigen (siehe §7.4). **OpenAIP** folgt in einem
> späteren Häppchen und ist bis dahin noch opt-in (globaler Schlüssel — siehe §7.3).
> Die Egress-Ziele oben sind Betriebsvoraussetzung, sobald die jeweilige Quelle aktiv ist.

> **Abgrenzung zur Feed-Isolation:** Der **CAT062/065/063-Multicast-Eingang** bleibt
> davon unberührt in einem **abgeschotteten Segment/VLAN** (NFR-SEC-001, ADR 0003).
> Die obigen Egress-Ziele betreffen **ausschließlich ausgehende Kontext-Quellen**,
> nicht den Feed-Draht. Fällt eine Kontext-Quelle aus oder ist gesperrt, bleibt der
> ASD-Kern (CAT062 → Karte) voll funktionsfähig (best-effort, blockiert nie
> `/ready`).
>
> **Besonders isolierter Betrieb:** Wer die Kontext-Quellen bewusst nicht
> nach außen sprechen lassen will, schaltet sie einzeln über die
> `WAYFINDER_..._ENABLED=false`-Schalter ab (bzw. hinterlegt keinen OpenAIP-
> Schlüssel und einen self-hosted Kartenstil).

### 8.1 Image bauen und pushen

```bash
cd ~/asd/wayfinder
docker build -t your-registry/wayfinder:latest .
docker push your-registry/wayfinder:latest
```

### 8.2 Eigenständiger Build ohne Docker (optional)

```bash
# Backend (statisches Binary)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o wayfinder ./cmd/wayfinder

# Frontend (nur nach Änderungen am Frontend-Code nötig; dist/ ist eingecheckt)
cd frontend && npm install && npm run build && cd ..
```
Voraussetzungen dafür: Go 1.23+ und Node.js 18 LTS+.

### 8.3 Host-Netzwerk-Variante (nur Linux)

Im **Produktionsbetrieb auf Linux** mit einer **echten externen** CAT062-Quelle
ist `network_mode: host` der direkteste Weg (kein Bridge-Multicast nötig). Diese
Variante funktioniert **nicht** auf macOS/Windows (Docker-VM). Da Wayfinder
multi-tenant läuft, gehört eine PostgreSQL-Datenbank dazu (`WAYFINDER_DB_URL` ist
Pflicht). Eine minimale `docker-compose.yml`:

```yaml
name: wayfinder-host

volumes:
  wayfinder-db:

services:
  # Datenbank für den Multi-Tenant-Betrieb (Pflicht).
  db:
    image: postgres:16-alpine
    network_mode: host          # nur Linux!
    environment:
      POSTGRES_USER: "wayfinder"
      POSTGRES_PASSWORD: "wayfinder"   # in Produktion ändern!
      POSTGRES_DB: "wayfinder"
    volumes:
      - wayfinder-db:/var/lib/postgresql/data
    restart: unless-stopped

  wayfinder:
    image: your-registry/wayfinder:latest
    network_mode: host          # nur Linux!
    depends_on: [db]
    environment:
      FIREFLY_CAT062_GROUP: "239.255.0.62"
      FIREFLY_CAT062_PORT: "8600"
      WAYFINDER_DB_URL: "postgres://wayfinder:wayfinder@localhost:5432/wayfinder?sslmode=disable"
      WAYFINDER_AUTH_MODE: "builtin"
      # Produktion: WAYFINDER_SESSION_KEY=$(openssl rand -hex 32) setzen.
      WAYFINDER_SESSION_KEY: ${WAYFINDER_SESSION_KEY:-}
      WAYFINDER_MAP_CENTER_LAT: "50.0379"
      WAYFINDER_MAP_CENTER_LON: "8.5622"
      WAYFINDER_MAP_ZOOM: "8"
    restart: unless-stopped
```

> ℹ️ Für den vollen orchestrierten Aufbau (Postgres + Server + Orchestrator, der
> pro Feed eine Firefly-Instanz startet) liegt ein fertiges Host-Net-Profil bereit:
> `docker-compose.orchestrated.yml` (Linux). Details in `docs/E2E-ABNAHME.md`.

### 8.4 Kubernetes-Hinweise

- **UDP-Multicast** ist in Cloud-Netzen (AWS/GCP VPC) i. d. R. blockiert. Wayfinder
  muss im selben Subnetz wie die Quelle laufen, oder Quelle + Wayfinder als
  Sidecars im selben Pod (localhost-Multicast).
- **Health/Readiness-Probes** auf Port 8080 (`/health`, `/ready`).
- **Secrets** (`WAYFINDER_SESSION_KEY`, `WAYFINDER_DB_URL`)
  als Kubernetes-Secret einbinden — **nicht** in ConfigMaps.
- **Graceful Shutdown:** reagiert auf `SIGINT`/`SIGTERM`;
  `terminationGracePeriodSeconds: 10` genügt.
- **Logs:** strukturiertes JSON auf `stderr` (Fluentd/Loki/CloudWatch).

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wayfinder
spec:
  replicas: 1
  selector:
    matchLabels: { app: wayfinder }
  template:
    metadata:
      labels: { app: wayfinder }
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
            - name: WAYFINDER_AUTH_MODE
              value: "builtin"
            - name: WAYFINDER_DB_URL
              valueFrom:
                secretKeyRef: { name: wayfinder-secrets, key: db-url }
            - name: WAYFINDER_SESSION_KEY
              valueFrom:
                secretKeyRef: { name: wayfinder-secrets, key: session-key }
          livenessProbe:
            httpGet: { path: /health, port: 8080 }
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet: { path: /ready, port: 8080 }
            initialDelaySeconds: 3
            periodSeconds: 5
            failureThreshold: 6
```

---

> **Geschafft.** Bei Problemen zuerst [Teil 6](#teil-6--wenn-etwas-nicht-geht--fehlersuche),
> dann die Logs (`docker compose logs -f wayfinder`). Die tiefergehende technische
> Dokumentation steht in `docs/TECHNICAL.md`.
