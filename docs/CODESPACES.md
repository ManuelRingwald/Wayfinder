# Wayfinder im Browser testen — GitHub Codespaces (ohne lokale Installation)

Diese Anleitung richtet sich an den Fall „**ich habe nur einen Browser**" —
z. B. ein Arbeits-Laptop ohne Installationsrechte und ohne Terminal. Der
komplette Stack läuft in einem **GitHub Codespace** in der Cloud; der Laptop
braucht nur `github.com` im Browser.

**Was läuft dort?** Der **orchestrierte Stack** (`docker-compose.orchestrated.yml`,
ADR 0012) — dieselbe Kette wie im echten Betrieb: Postgres + Wayfinder +
Orchestrator-Steuerebene. Du legst in der Admin-UI Mandanten und Feeds an,
und der **Orchestrator spawnt automatisch einen Firefly-Tracker je
abonniertem Feed** (Auto-Spawn/-Teardown inklusive). Multicast funktioniert,
weil ein Codespace ein Linux-Host ist (docker-in-docker, ein gemeinsamer
Netz-Namespace) — die „nur Linux"-Einschränkung aus `DOCKER.md` betrifft
Docker Desktop, nicht Codespaces.

**Sicherheit:** Der weitergeleitete Port ist in Codespaces standardmäßig
**privat** — die URL verlangt deinen GitHub-Login, dahinter greift Wayfinders
builtin-Anmeldung (ADR 0003/0014). Das ASD-Bild ist damit doppelt gegated;
den Port **nicht** auf „Public" stellen. `start.sh` erzeugt beim ersten Start
eine Codespace-lokale `.env` mit festem Session- und Secret-Key (gitignored) —
Logins überleben Neustarts, und die Feed-Zugangsdaten-API ist aktiv.

---

## 1. Codespace starten (einmalig ~10–15 Minuten)

1. Im Browser: **github.com → ManuelRingwald/Wayfinder**.
2. Grüner Knopf **„Code" → Tab „Codespaces" → „Create codespace on main"**.
3. Beim ersten Mal fragt GitHub nach Zustimmung, dass der Codespace zusätzlich
   **lesend auf `ManuelRingwald/Firefly`** zugreifen darf (daraus wird das
   Tracker-Image gebaut, das der Orchestrator je Feed startet) — **bestätigen**.
4. Warten. Der erste Start baut das Firefly-Image (Rust-Release-Build, mehrere
   Minuten) und die Wayfinder-/Orchestrator-Images. Fortschritt: Menü „⋯" →
   „View creation log". Spätere Starts nutzen den Cache (<1 Minute).
5. Über den Reiter **„Ports"** (unteres Panel) bei **8081** „Open in Browser"
   klicken — oder `https://<codespace-name>-8081.app.github.dev` öffnen.

## 2. Einrichtung — wie im echten Betrieb (Admin-UI)

Der Auto-Seed legt nur den Plattform-Admin an (ADR 0011 Nachtrag); alles
Weitere machst du per UI — genau der Ablauf, den du auch produktiv testest:

1. **`…/admin`** öffnen → Login **`admin` / `admin`** → erzwungenen
   **Passwortwechsel** durchführen.
2. **Mandant anlegen** (z. B. `hamburg`).
3. **Feed anlegen** — Endpoint einfach **auto-allokieren** lassen (der
   Orchestrator übergibt Gruppe/Port an den gespawnten Firefly).
4. Im „Feeds"-Tab den **Quellen-Dialog** öffnen: Quelle `adsb_opensky` mit
   **BBox** (und optional Poll-Intervall) anlegen; die
   **OpenSky-Zugangsdaten** (Client-ID/-Secret) direkt im Dialog hinterlegen
   (sie werden AES-versiegelt gespeichert, nie zurückgegeben).
5. Auf der **Mandanten-Detailseite**: Feed **abonnieren**, Standard-Ansicht
   setzen (Zentrum/Radius zur BBox passend) und **„Ansicht speichern"**;
   Features nach Bedarf einschalten.
6. → Der **Orchestrator spawnt** binnen Sekunden einen Firefly für den Feed
   (Reconcile-Intervall 10 s); Tracks erscheinen auf der Karte (**`…/`**).
   Feed löschen/Abo entziehen räumt den Tracker automatisch wieder ab.

> **Hinweis:** Ein Feed **ohne** Quellen wird derzeit mit einer
> Platzhalter-Szene gespawnt (`WAYFINDER_FIREFLY_SCENE`) — diese Demo-Altlast
> ist zum Ausbau vorgesehen; für echte Tests immer Quellen konfigurieren.

## 3. Alltag

| Aktion | Wie |
|--------|-----|
| **Pausieren** | Nichts tun — der Codespace schläft nach Inaktivität ein (Standard 30 min). DB-Volume, Images und `.env` bleiben erhalten. |
| **Fortsetzen** | github.com → „Code" → Codespaces → Codespace öffnen; der Stack startet automatisch (`postStartCommand`). Gespawnte Firefly-Container stellt der Orchestrator per Reconcile selbst wieder her. |
| **Logs ansehen** | Codespace-Terminal (im Browser): `docker compose -f docker-compose.orchestrated.yml logs -f wayfinder` (oder `orchestrator`); gespawnte Tracker: `docker ps` / `docker logs <firefly-container>`. |
| **Alles zurücksetzen** | `docker compose -f docker-compose.orchestrated.yml down -v` (löscht auch die DB!), dann `bash .devcontainer/start.sh`. |
| **Codespace löschen** | github.com → Codespaces-Übersicht → „Delete". Kostenlos-Kontingent (Stand 2026): ~120 Core-Stunden/Monat ⇒ ~30 Betriebsstunden auf der 4-Core-Maschine — einschlafen lassen reicht im Alltag. |

## 4. Grenzen

- **OpenSky-Zugangsdaten nötig** für echte ADS-B-Tracks (kostenloses Konto,
  OAuth2-Client-Credentials — Firefly ADR 0024). Ohne Quellen kein sinnvolles
  Lagebild (die Platzhalter-Szene ist Altlast, s. o.).
- **Ressourcen:** 4-Core-Maschine empfohlen (ist als Minimum hinterlegt);
  viele parallele Feeds = mehrere Firefly-Container — für Lasttests weiterhin
  eine echte VM nutzen.
- **Egress:** OpenSky/DWD/NOAA/Karten-CDN brauchen ausgehendes HTTPS aus dem
  Codespace — dort normal gegeben.
- Die weitergeleitete URL wechselt mit dem Codespace-Namen; Lesezeichen nach
  einem Neuanlegen aktualisieren.
