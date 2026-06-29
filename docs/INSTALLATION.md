# Wayfinder — Installationsanleitung

> **Für wen ist diese Anleitung?** Für alle, die Wayfinder zum Laufen bringen
> wollen — **auch ohne IT-Hintergrund**. Sie führt Schritt für Schritt vom leeren
> Rechner bis zum fertigen Luftlagebild im Browser. Befehle einfach **kopieren und
> einfügen**. Wo etwas auf dem Mac anders ist, steht es ausdrücklich dabei.

> **Zwei Betriebsarten — wählen Sie eine:**
> - **Einzelplatz (Single-Tenant)** → [Teil 4](#teil-4--einzelplatz-single-tenant-schritt-für-schritt).
>   Ein Lagebild, **kein Login, keine Datenbank**. Der einfachste Fall — hier
>   anfangen.
> - **Mehrere Kunden (Multi-Tenant)** → [Teil 5](#teil-5--mehrere-kunden-multi-tenant-schritt-für-schritt).
>   Mehrere Mandanten mit **eigenem Login**, Datenbank und Admin-Oberfläche.

> 🛠️ **Läuft es schon und Sie wollen es im Alltag betreuen** (kontrollieren,
> Mandanten/Feeds pflegen, sichern, Störungen beheben)? → **Betriebsführungs­
> handbuch** `docs/BETRIEB.md`.

---

## Inhaltsverzeichnis

1. [Überblick — was läuft hier eigentlich?](#teil-1--überblick--was-läuft-hier-eigentlich)
2. [Werkzeuge installieren (macOS / Windows / Linux)](#teil-2--werkzeuge-installieren)
3. [Wayfinder & Firefly herunterladen](#teil-3--wayfinder--firefly-herunterladen)
4. [**Einzelplatz (Single-Tenant)** — Schritt für Schritt](#teil-4--einzelplatz-single-tenant-schritt-für-schritt)
5. [**Mehrere Kunden (Multi-Tenant)** — Schritt für Schritt](#teil-5--mehrere-kunden-multi-tenant-schritt-für-schritt)
   - [5.A — Master-Compose für Plattform + Firefly (Option B)](#schritt-5a--master-compose-für-plattform--firefly-option-b)
6. [Läuft es? — Verifikation](#teil-6--läuft-es--verifikation)
7. [Wenn etwas nicht geht — Fehlersuche](#teil-7--wenn-etwas-nicht-geht--fehlersuche)
8. [Konfigurationsreferenz (alle Schalter)](#teil-8--konfigurationsreferenz)
9. [Produktionsbetrieb (Kubernetes, TLS, Host-Netzwerk)](#teil-9--produktionsbetrieb)

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
> funktioniert hier **nicht** — das betrifft das Repo-eigene `docker-compose.yml`
> (Single-Tenant). **Teil 4** (Einzelplatz) und der **Multi-Tenant-Schnellstart
> `docker-compose.onboarding.yml`** (Teil 5) nutzen dagegen ein Bridge-Netz und
> laufen auf dem Mac **genauso wie auf Linux**. Sie müssen nichts extra tun,
> einfach den Schritten folgen. (Für echte Tracks mit Firefly auf dem Mac →
> DOCKER.md, „macOS/Windows"-Abschnitt.)

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

## Teil 4 — Einzelplatz (Single-Tenant): Schritt für Schritt

**Das richten wir hier ein:** Ein einzelnes Lagebild auf Ihrem Rechner.
**Kein Login, keine Datenbank.** Ideal zum Ausprobieren und für den
Einzelarbeitsplatz.

### Schritt 4.1 — Steuerungsordner anlegen

Wir legen einen kleinen Ordner an, der nur die beiden Konfigurationsdateien
enthält (den Programmcode haben wir schon in Teil 3 geladen):

```bash
mkdir -p ~/asd/start-einzelplatz
cd ~/asd/start-einzelplatz
```

### Schritt 4.2 — Die Start-Datei `docker-compose.yml` anlegen

Diese Datei beschreibt, **was** gestartet wird. Legen Sie sie an — am
einfachsten im Terminal mit einem Editor wie `nano`:

```bash
nano docker-compose.yml
```

Fügen Sie **genau diesen Inhalt** ein (kopieren, im Terminal mit `Cmd+V` bzw.
`Strg+V` einfügen). Speichern in `nano`: `Strg+O`, `Enter`, dann `Strg+X`.

```yaml
# ~/asd/start-einzelplatz/docker-compose.yml
# Einzelplatz-Aufbau: Firefly (Datenquelle) + Wayfinder (Karte) gemeinsam
# in einem Container-Netz. Funktioniert auf macOS, Windows und Linux gleich.
name: wayfinder-einzelplatz

networks:
  asd:
    driver: bridge

services:
  # Datenquelle: erzeugt Test-Flugzeuge und sendet sie als CAT062-Multicast.
  firefly:
    build: ../firefly
    networks: [asd]
    environment:
      FIREFLY_SCENE: "frankfurt"          # Test-Szenario rund um Frankfurt
      FIREFLY_CAT062_ENABLED: "true"      # CAT062-Ausgabe einschalten
      FIREFLY_CAT062_GROUP: "239.255.0.62"
      FIREFLY_CAT062_PORT: "8600"
    restart: unless-stopped

  # Die Karte (das ASD).
  wayfinder:
    build: ../wayfinder
    networks: [asd]
    ports:
      - "8081:8081"                       # Browser-Lagebild
      - "8080:8080"                       # Technik-Checks
    environment:
      FIREFLY_CAT062_GROUP: "239.255.0.62"  # muss zu Firefly oben passen
      FIREFLY_CAT062_PORT: "8600"
    volumes:
      - ./wayfinder.yaml:/app/wayfinder.yaml:ro   # Kartenausschnitt (nächster Schritt)
    restart: unless-stopped
```

### Schritt 4.3 — Den Kartenausschnitt festlegen (`wayfinder.yaml`)

Diese Datei legt fest, **wo** die Karte beim Start hinschaut. Anlegen:

```bash
nano wayfinder.yaml
```

Vollständiges Beispiel (Frankfurt — passt zum `frankfurt`-Szenario von Firefly):

```yaml
# ~/asd/start-einzelplatz/wayfinder.yaml
# Kartenausschnitt beim Start. Alle Felder sind optional —
# was Sie weglassen, behält seinen Standardwert.

map:
  center_lat: 50.0379    # Breitengrad des Kartenmittelpunkts (Frankfurt)
  center_lon: 8.5622     # Längengrad des Kartenmittelpunkts
  zoom: 8                # Zoomstufe: 8 = Region, 10 = Großraum, 12 = Platzrunde

openaip:
  radius_km: 185         # Umkreis für Lufträume/Navigationspunkte (185 km ≈ 100 NM)
```

> 📍 **Anderen Ort wählen?** Tragen Sie einfach andere Koordinaten ein. Beispiele:
> München `48.1374 / 11.5755`, Hamburg `53.5511 / 9.9937`, Wien `48.2082 / 16.3738`.
> Koordinaten finden Sie z. B., indem Sie in einer Kartensuche rechtsklicken.
>
> ⚠️ **Ehrliche Grenze:** Die Datei `wayfinder.yaml` kann **nur** den
> Kartenausschnitt (`map`) und den OpenAIP-Radius (`openaip`) einstellen. **Alle
> anderen** Einstellungen (Datenbank, Login, Sicherheit …) laufen über
> Umgebungsvariablen im `environment:`-Block der `docker-compose.yml` — siehe
> [Teil 8](#teil-8--konfigurationsreferenz).

### Schritt 4.4 — Starten

Jetzt steht alles bereit. Im selben Ordner (`~/asd/start-einzelplatz`):

```bash
docker compose up --build
```

Beim **ersten** Mal dauert das einige Minuten (Docker baut beide Programme).
Lassen Sie das Fenster offen — hier laufen die Log-Meldungen. Wenn Zeilen wie
`feed joined` / `listening on :8081` erscheinen, läuft es.

### Schritt 4.5 — Das Lagebild öffnen

Öffnen Sie im Browser: **<http://localhost:8081>**

Sie sehen die dunkle Radar-Karte. Nach wenigen Sekunden tauchen die
Test-Flugzeuge von Firefly auf, und oben links zeigt ein Banner **FEED OK**
(grün).

✅ **Fertig!** Zum **Beenden** klicken Sie ins Terminal-Fenster und drücken
`Strg+C`. Zum erneuten Start genügt künftig `docker compose up` (ohne `--build`).

---

## Teil 5 — Mehrere Kunden (Multi-Tenant): Schritt für Schritt

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
> (Inhalt → [Schritt 5.A](#schritt-5a--master-compose-für-plattform--firefly-option-b)),
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
> Wer eine dieser Optionen nutzt, **überspringt die Schritte 5.1–5.4 komplett**.
> Weiter ab [Schritt 5.5](#schritt-55--feeds-in-den-katalog-aufnehmen).

### Schritt 5.A — Master-Compose für Plattform + Firefly (Option B)

> ℹ️ **Nur nötig für Option B** (Plattform + Firefly-Feed). Für Option A (ohne
> Firefly) direkt zu [Schritt 5.5](#schritt-55--feeds-in-den-katalog-aufnehmen)
> springen.

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
      FIREFLY_SCENE: "frankfurt"
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
      WAYFINDER_SECRET_KEY: ${WAYFINDER_SECRET_KEY:-}
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
Sie fertig. **Weiter mit Schritt 5.5**, um den Firefly-Feed dem Admin zuzuordnen
und ihn einem Mandanten zuzuweisen.

---

### Schritt 5.1 — Steuerungsordner anlegen

```bash
mkdir -p ~/asd/start-plattform
cd ~/asd/start-plattform
```

### Schritt 5.2 — Die Start-Datei `docker-compose.yml` anlegen

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
      FIREFLY_SCENE: "frankfurt"
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

### Schritt 5.3 — Den Kartenausschnitt anlegen (`wayfinder.yaml`)

Genau wie im Einzelplatz-Fall:

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

### Schritt 5.4 — Den ersten Administrator anlegen (`bootstrap`) — *optional*

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
> `user`) legen Sie danach bequem über die Oberfläche an (Schritt 5.8b). Einen
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

### Schritt 5.5 — Feeds in den Katalog aufnehmen

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

### Schritt 5.6 — Alles starten

```bash
docker compose up -d --build
```

(`-d` = im Hintergrund. Logs ansehen mit `docker compose logs -f wayfinder`.)

### Schritt 5.7 — Als Administrator anmelden

Öffnen Sie **<http://localhost:8081/admin>** und melden Sie sich an:

- **Benutzername:** `admin`
- **Passwort:** `admin` (bei Zero-Touch-Schnellstart — Sie werden sofort zum Wechsel gezwungen) **oder** das in Schritt 5.4 selbst gewählte Passwort

Sie sehen die Admin-Oberfläche. Seit AP3 (ADR 0009) ist sie
**mandantenzentriert**: zuerst eine **Übersicht aller Mandanten** (mit Status,
aktiven Features, zugewiesenen Feeds und Anzahl der Zugänge); ein Klick auf
**„Konfigurieren"** öffnet die **Detailseite** des Mandanten, auf der Sie an
einem Ort die **Standard-Ansicht** (Zentrum + Radius, FL-Band), die **Features**,
die **Feeds** und die **Zugänge** dieses Kunden verwalten.

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

### Schritt 5.8 — Einen Kunden-Mandanten anlegen

> 🖥️ **Seit ONB-4 (ADR 0011) bequem über die Oberfläche:** Mandanten legen Sie
> im Admin-Bereich unter „Mandanten → Mandant anlegen" an (und löschen sie dort —
> ein Mandant mit noch vorhandenen Zugängen wird aus Sicherheitsgründen erst nach
> Entfernen der Konten gelöscht). Der folgende CLI-Weg bleibt als
> Skript-/Automatisierungs-Pfad erhalten.

Für jeden Kunden legen Sie einen eigenen Mandanten mit einem ersten `user`-Zugang
an — per Oberfläche oder mit `bootstrap` (weitere Zugänge danach bequem über die
Oberfläche, Schritt 5.8b):

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

### Schritt 5.8b — Weitere Zugänge verwalten (Oberfläche, AP6)

Im Admin-Bereich öffnen Sie in der **Mandanten-Übersicht** den gewünschten
Mandanten („Konfigurieren") und verwalten im Abschnitt **„Zugänge"** dessen
Login-Konten direkt im Browser — **ohne** `bootstrap`/SQL:

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

### Schritt 5.9 — Dem Kunden einen Feed zuweisen

Damit der neue Kunde Flugzeuge sieht, muss ihm ein Feed **zugewiesen** werden.
Das darf nur ein `admin`. Zwei Wege:

**Weg A — über die Admin-Oberfläche (empfohlen):** In `/admin` als `admin`
angemeldet, in der Mandanten-Übersicht **„Kunde Nord GmbH"** öffnen
(„Konfigurieren") und im Abschnitt **„Feeds"** den Feed **„Frankfurt"** zuweisen.

**Weg B — über die Befehlszeile (mit `curl`):** Zuerst Mandanten- und Feed-IDs
herausfinden, dann zuweisen. (`{tenant-id}` / `feed_id` aus `feed list` bzw. der
Admin-Liste.)

```bash
# Beispiel: Feed 1 dem Mandanten 2 zuweisen — als angemeldeter admin.
curl -X POST http://localhost:8081/api/admin/tenants/2/subscriptions \
  -H 'Content-Type: application/json' \
  -d '{"feed_id":1}'
```

### Schritt 5.10 — Als Kunde anmelden und prüfen

Melden Sie sich (am besten in einem **privaten Browserfenster**) unter
**<http://localhost:8081/admin>** als `anna` an. Auf dem Lagebild
(**<http://localhost:8081>**) sieht „Kunde Nord" nun **genau** die Flugzeuge des
zugewiesenen Feeds — und **keine** anderen.

✅ **Fertig!** Sie haben eine Multi-Tenant-Plattform aufgesetzt. Weitere Kunden:
Schritte 5.8 + 5.9 wiederholen.

### Schritt 5.11 — „View as Tenant": die Sicht eines Kunden einsehen (nur `admin`)

Für den Support gibt es einen **Read-Only-Einblick**: ein `admin` kann die
Lage **so sehen, wie ein bestimmter Kunde sie sieht** — ohne dessen Passwort, nur
lesend, vollständig protokolliert (ADR 0008).

So funktioniert es im Browser:

1. Als `admin` am Lagebild **<http://localhost:8081>** angemeldet,
   erscheint oben mittig die Schaltfläche **„Als Mandant ansehen"**.
2. Mandanten auswählen (z. B. „Kunde Nord GmbH") → die Karte wechselt sofort auf
   **dessen** Feeds und Sicht; ein **gelber Banner** zeigt
   „Sie betrachten **Kunde Nord GmbH** — nur Lesen".
3. Im Banner kann man per **„Mandant wechseln"** direkt zu einem anderen Kunden
   springen oder mit **„Beenden"** zur eigenen Sicht zurückkehren.

**Wichtig zu wissen:**

- **Nur lesend:** Es lässt sich nichts im Namen des Kunden ändern — Verwaltung
  läuft immer über die echte Identität.
- **Nur `admin`:** Nutzer mit Rolle `user` sehen die Funktion nicht; ein
  gefälschter Zugriffsversuch wird serverseitig **laut abgewiesen und ins
  Audit-Log geschrieben**.
- **Zeitlich befristet:** Der Einblick läuft nach `WAYFINDER_IMPERSONATION_TTL`
  (Standard 30 min) automatisch ab.
- **Voraussetzung:** Ein Signing-Key (`WAYFINDER_SESSION_KEY`) muss gesetzt sein —
  im `builtin`-Aufbau aus Teil 5 ist das bereits der Fall.

>> 📖 Die laufende Aufsicht über diese Einblicke (Audit-Spur „wer sah welchen
> Mandanten") ist im **Betriebsführungshandbuch** (`docs/BETRIEB.md`, Abschnitt
> Sicherheits-Betrieb) beschrieben.

---

## Teil 6 — Läuft es? — Verifikation

Diese Prüfungen funktionieren in **beiden** Betriebsarten. Im Terminal:

### 6.1 Läuft der Dienst überhaupt? (Liveness)
```bash
curl -s http://localhost:8080/health
# Erwartet:  ok
```

### 6.2 Kommen Daten an? (Readiness)
```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/ready
# 200 = Feed aktiv (mindestens ein Lebenszeichen empfangen)
# 503 = Feed noch nie gesehen oder gerade ausgefallen
```

### 6.3 Im Browser
Öffnen Sie **<http://localhost:8081>**. Die dunkle Karte erscheint sofort;
Flugzeuge erscheinen, sobald Firefly sendet — erkennbar am grünen Banner
**FEED OK** oben links.

### 6.4 Zahlen/Metriken (optional)
```bash
curl -s http://localhost:8080/metrics | grep wayfinder_feed_stale
# wayfinder_feed_stale 0   ← 0 bedeutet: Feed ist frisch/gesund
```

---

## Teil 7 — Wenn etwas nicht geht — Fehlersuche

| Symptom | Wahrscheinliche Ursache & Lösung |
|---------|----------------------------------|
| **Karte bleibt leer, keine Flugzeuge** | Läuft Firefly? `docker compose ps` zeigt alle Dienste als `running`. Logs prüfen: `docker compose logs firefly`. Stimmen `FIREFLY_CAT062_GROUP`/`_PORT` bei **beiden** Diensten überein? |
| **`/ready` liefert 503** | Es kam noch kein Lebenszeichen (CAT065-Heartbeat). Normal direkt nach dem Start — ein paar Sekunden warten. Bleibt es 503: Firefly sendet nicht / Multicast kommt nicht an (siehe nächste Zeile). |
| **Flugzeuge erscheinen nie, obwohl Firefly läuft** | Multicast wird im Container-Netz nicht durchgereicht. Stellen Sie sicher, dass Sie die **gemeinsame** Compose aus dieser Anleitung verwenden (Firefly **und** Wayfinder im selben `asd`-Netz). Trennen Sie sie nicht auf zwei Compose-Dateien. |
| **`docker compose up` bricht mit Build-Fehler ab** | Erstes Bauen braucht Internet (lädt Abhängigkeiten). Verbindung prüfen, dann `docker compose build --no-cache` erneut versuchen. |
| **`port is already allocated` (8081/8080 belegt)** | Ein anderer Dienst nutzt den Port. Anderen Port abbilden, z. B. `"9091:8081"`, dann `http://localhost:9091` öffnen. |
| **Mac: Docker-Befehle hängen / „Cannot connect to the Docker daemon"** | Docker Desktop ist nicht gestartet. Docker aus dem Launchpad öffnen, warten bis das Wal-Symbol ruhig steht. |
| **Multi-Tenant: Login schlägt fehl (401)** | Passwort falsch; **oder** der Zugang bzw. sein Mandant ist **pausiert** (Schritt 5.8b — fail-closed ist Absicht, im Reiter „Zugänge" reaktivieren); **oder** `WAYFINDER_SESSION_KEY` fehlt/ist zu kurz (Schlüssel setzen mit `openssl rand -hex 32`, Container neu starten, `bootstrap` ggf. erneut ausführen). |
| **Multi-Tenant: Kunde sieht keine Flugzeuge** | Dem Mandanten wurde **kein Feed zugewiesen** (Schritt 5.9) — fail-closed ist Absicht. Zuweisung als `admin` nachholen. |
| **Logs ansehen** | `docker compose logs -f wayfinder` (live mitlaufen, `Strg+C` beendet die Anzeige, **nicht** den Dienst). |
| **Alles sauber neu aufsetzen** | `docker compose down -v` löscht Container **und** die Datenbank-Daten (`-v`!). Danach bei Teil 4/5 neu beginnen. |

---

## Teil 8 — Konfigurationsreferenz

Konfiguriert wird über **Umgebungsvariablen** (im `environment:`-Block der
`docker-compose.yml`) und optional über die **YAML-Datei** `wayfinder.yaml` (nur
`map` + `openaip`). **Umgebungsvariablen gewinnen immer** (12-Factor).

### 8.1 Netzwerk & Feed

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `FIREFLY_CAT062_GROUP` | `239.255.0.62` | UDP-Multicast-Gruppe für den CAT062/CAT065-Eingang |
| `FIREFLY_CAT062_PORT` | `8600` | UDP-Port des Multicast-Stroms |
| `WAYFINDER_FEED_ID` | `0` | Katalog-Feed-ID dieses Einzel-Feeds (Single-Tenant). Im Multi-Feed-Betrieb liefert der DB-Katalog die IDs. |
| `WAYFINDER_PROBE_PORT` | `8080` | Port für `/health`, `/ready`, `/metrics` |
| `WAYFINDER_FEED_STALE_TIMEOUT` | `3` | Sekunden ohne Lebenszeichen, ab denen der Feed als „stale" gilt |

### 8.2 Karte & Darstellung

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_MAP_CENTER_LAT` | `50.0379` | Breitengrad des Kartenstartzentrums (Frankfurt) |
| `WAYFINDER_MAP_CENTER_LON` | `8.5622` | Längengrad des Kartenstartzentrums |
| `WAYFINDER_MAP_ZOOM` | `8` | Anfangs-Zoom (1–22) |
| `WAYFINDER_MAP_THEME` | `dark` | `dark` (CARTO Dark, schlüsselfrei) oder `osm` (OpenStreetMap-Raster) |
| `WAYFINDER_MAP_STYLE_URL` | *(leer)* | Vollständige MapLibre-Style-URL — überschreibt `WAYFINDER_MAP_THEME` |

> Dieselben drei `map`-Werte lassen sich auch in `wayfinder.yaml` setzen (siehe
> Schritt 4.3). Die Umgebungsvariable gewinnt, falls beides gesetzt ist.

### 8.3 Aeronautische Daten (OpenAIP, optional)

Ohne `WAYFINDER_OPENAIP_API_KEY` ist das Feature aus (Warn-Log, kein Fehler).

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_OPENAIP_API_KEY` | *(leer)* | **Globaler** OpenAIP-API-Schlüssel; leer = Feature global aus |
| `WAYFINDER_OPENAIP_RADIUS_KM` | `250` | Umkreis um das Zentrum für Luftraum-/Navaid-Abfragen (auch via `wayfinder.yaml` → `openaip.radius_km`) |
| `WAYFINDER_OPENAIP_REFRESH` | `24h` | Aktualisierungsintervall (`1h`, `30m`, `24h`) |
| `WAYFINDER_OPENAIP_BASE_URL` | *(intern)* | Override der OpenAIP-Basis-URL (Tests/Proxies) |

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

### 8.4 Sicherheit (Browser-Rand)

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_ALLOWED_ORIGINS` | *(leer)* | Kommaliste erlaubter Cross-Origin-Domains für `/ws`. Leer = nur Same-Origin |
| `WAYFINDER_AUTH_TOKEN` | *(leer)* | Bearer-Token für den Browser-Rand (Single-Tenant). Leer = kein Token-Check (Warn-Log). Prüfung via `Authorization: Bearer <token>` oder `?token=<token>` |
| `WAYFINDER_TLS_CERT` | *(leer)* | Pfad zum TLS-Zertifikat (PEM). Nur aktiv, wenn beide TLS-Werte gesetzt sind |
| `WAYFINDER_TLS_KEY` | *(leer)* | Pfad zum TLS-Schlüssel (PEM) |

### 8.5 Multi-Mandanten (nur Multi-Tenant)

Multi-Tenancy ist **nur aktiv, wenn `WAYFINDER_DB_URL` gesetzt ist**. Ohne diese
Variable läuft Wayfinder als Single-Tenant-ASD (keine DB, keine Login-Pflicht).
Mit gesetzter DB werden die Schema-Migrationen beim Start angewandt und `/ws` ist
durch die Mandanten-Middleware geschützt (fail-closed: ohne gültigen, einem
Mandanten zugeordneten Nutzer → `401`).

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_DB_URL` | *(leer)* | PostgreSQL-DSN, z. B. `postgres://user:pass@host:5432/wayfinder?sslmode=disable`. Leer = Single-Tenant |
| `WAYFINDER_AUTH_MODE` | `none` | `builtin` (eingebaute Nutzer + Session-Cookie), `proxy` (OIDC-Token vom Reverse-Proxy) oder `none` (festes Subject, nur mit Netz-Isolation) |
| `WAYFINDER_SESSION_KEY` | *(leer)* | `builtin`: HMAC-Schlüssel zum Signieren der Session-Cookies (**Pflicht** im builtin-Modus; ≥ 32 zufällige Zeichen) |
| `WAYFINDER_SESSION_COOKIE` | `wf_session` | `builtin`: Name der Session-Cookie |
| `WAYFINDER_SESSION_TTL` | `12h` | `builtin`: Session-Lebensdauer (`8h`, `12h` …) |
| `WAYFINDER_IMPERSONATION_TTL` | `30m` | Lebensdauer des Read-Only-Impersonation-Grants („View as Tenant", ADR 0008). Nur wirksam, wenn ein Signing-Key (`WAYFINDER_SESSION_KEY`) gesetzt ist; sonst ist Impersonation deaktiviert. |
| `WAYFINDER_NONE_SUBJECT` | `default` | `none`: festes Subject, das jeder Anfrage zugeordnet wird |
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
| `GET /api/admin/tenants/{id}/openaip` | OpenAIP-Schlüssel-Status (`{"configured":bool}`) | admin |
| `PUT /api/admin/tenants/{id}/openaip` | OpenAIP-Schlüssel setzen/löschen | admin |

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

### 8.6 Radarabdeckungs-Overlay (optional, Paket 6)

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

### 8.7 Betrieb

| Variable | Default | Beschreibung |
|----------|---------|--------------|
| `WAYFINDER_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` (ungültig → `info`) |
| `WAYFINDER_CONFIG_FILE` | `wayfinder.yaml` | Pfad zur optionalen YAML-Datei. Fehlende Datei ist nicht fatal |

### 8.8 Vollständige `wayfinder.yaml`

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

## Teil 9 — Produktionsbetrieb

Wayfinder ist ein **12-Factor-Service** und eignet sich direkt für Kubernetes.
Die folgenden Hinweise richten sich an Betriebs-/IT-Teams.

### 9.1 Image bauen und pushen

```bash
cd ~/asd/wayfinder
docker build -t your-registry/wayfinder:latest .
docker push your-registry/wayfinder:latest
```

### 9.2 Eigenständiger Build ohne Docker (optional)

```bash
# Backend (statisches Binary)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o wayfinder ./cmd/wayfinder

# Frontend (nur nach Änderungen am Frontend-Code nötig; dist/ ist eingecheckt)
cd frontend && npm install && npm run build && cd ..
```
Voraussetzungen dafür: Go 1.23+ und Node.js 18 LTS+.

### 9.3 Host-Netzwerk-Variante (nur Linux)

Im **Produktionsbetrieb auf Linux** mit einer **echten externen** CAT062-Quelle
ist `network_mode: host` der direkteste Weg (kein Bridge-Multicast nötig). Diese
Variante funktioniert **nicht** auf macOS/Windows (Docker-VM). Eine
minimale `docker-compose.yml`:

```yaml
name: wayfinder-host
services:
  wayfinder:
    image: your-registry/wayfinder:latest
    network_mode: host          # nur Linux!
    environment:
      FIREFLY_CAT062_GROUP: "239.255.0.62"
      FIREFLY_CAT062_PORT: "8600"
      WAYFINDER_MAP_CENTER_LAT: "50.0379"
      WAYFINDER_MAP_CENTER_LON: "8.5622"
      WAYFINDER_MAP_ZOOM: "8"
    restart: unless-stopped
```

### 9.4 Kubernetes-Hinweise

- **UDP-Multicast** ist in Cloud-Netzen (AWS/GCP VPC) i. d. R. blockiert. Wayfinder
  muss im selben Subnetz wie die Quelle laufen, oder Quelle + Wayfinder als
  Sidecars im selben Pod (localhost-Multicast).
- **Health/Readiness-Probes** auf Port 8080 (`/health`, `/ready`).
- **Secrets** (`WAYFINDER_AUTH_TOKEN`, `WAYFINDER_SESSION_KEY`, `WAYFINDER_DB_URL`)
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
            - name: WAYFINDER_AUTH_TOKEN
              valueFrom:
                secretKeyRef: { name: wayfinder-secrets, key: auth-token }
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

> **Geschafft.** Bei Problemen zuerst [Teil 7](#teil-7--wenn-etwas-nicht-geht--fehlersuche),
> dann die Logs (`docker compose logs -f wayfinder`). Die tiefergehende technische
> Dokumentation steht in `docs/TECHNICAL.md`.
