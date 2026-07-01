# End-to-End-Abnahme auf dem Mac mini (mit Multipass-Linux-VM)

> **Ziel dieses Dokuments.** Von einem **frischen Mac mini** bis zur
> **nachgewiesen funktionierenden** Luftlage — Schritt für Schritt. Jeder Schritt
> nennt **exakt**, was du eingibst, **was genau herauskommen muss** und **woran du
> das prüfst**. Es gibt **keinen Interpretationsspielraum**: Wenn das erwartete
> Ergebnis nicht eintritt, ist der Schritt nicht bestanden — dann hilft
> [Teil 8 (Fehlerbehebung)](#teil-8--fehlerbehebung).

> **Warum eine Linux-VM?** Der vollständige Betrieb (Orchestrator startet je Feed
> automatisch eine Firefly-Instanz, CAT062 fließt per **UDP-Multicast** über
> `network_mode: host`) braucht einen **echten Linux-Kernel**. Docker Desktop auf
> dem Mac kann das nicht (nur die interne VM, nicht der Mac-Host). Lösung: eine
> schlanke **Ubuntu-VM mit Multipass** direkt auf dem Mac mini — darin läuft alles
> nativ wie auf einem Linux-Server. (Einen schnellen Teil-Check **ohne** VM
> beschreibt [Anhang A](#anhang-a--schnell-check-ohne-vm-nur-auf-dem-mac).)

---

## Was am Ende nachgewiesen ist

| # | Behauptung | Nachgewiesen in |
|---|------------|-----------------|
| 1 | Aus einem frischen Mac mini wird mit wenigen Befehlen ein **echter Linux-Docker-Host** (Multipass-VM). | Teil 1–2 |
| 2 | Der **orchestrierte Stack** (Datenbank + ASD-Server + Orchestrator) startet vollständig. | Teil 3 |
| 3 | **Automatischer Nachweis** der Kette: Feed → Auto-Spawn einer Firefly-Instanz → CAT062-Multicast → ASD empfängt Tracks → Abmelden räumt auf. | Teil 4 |
| 4 | Die **ganze Kundeneinrichtung** (Mandant, Nutzer, Feed, Sicht, Zuweisung) läuft in der **Browser-UI**; der angemeldete Mandant sieht **live Tracks**. | Teil 5 |
| 5 | **Hinter den Kulissen** belegen Container, Logs und Metriken jede Stufe der Kette. | Teil 6 |

**Inhalt:**
[Teil 0 Voraussetzungen](#teil-0--voraussetzungen) ·
[Teil 1 Linux-VM](#teil-1--linux-vm-mit-multipass-anlegen) ·
[Teil 2 Docker in der VM](#teil-2--docker-in-der-vm-einrichten) ·
[Teil 3 Repos + Stack](#teil-3--repos-holen-firefly-image-bauen-stack-starten) ·
[Teil 4 Automatischer Lauf](#teil-4--automatischer-abnahme-lauf-deterministisch) ·
[Teil 5 UI-Abnahme](#teil-5--abnahme-in-der-browser-ui) ·
[Teil 6 Hinter den Kulissen](#teil-6--hinter-den-kulissen-prüfen) ·
[Teil 7 Aufräumen](#teil-7--aufräumen) ·
[Teil 8 Fehlerbehebung](#teil-8--fehlerbehebung) ·
[Anhang A Schnell-Check ohne VM](#anhang-a--schnell-check-ohne-vm-nur-auf-dem-mac)

> **Konventionen.** `〈…〉` markiert einen Wert, den **du** aus einer vorherigen
> Ausgabe einsetzt (z. B. `〈VM-IP〉`). „**Auf dem Mac**" = Terminal auf macOS.
> „**In der VM**" = eine Shell **innerhalb** der Ubuntu-VM (nach
> `multipass shell asd`). Der Eingabe-Prompt in der VM sieht so aus:
> `ubuntu@asd:~$`.

---

## Teil 0 — Voraussetzungen

| # | Aktion (auf dem Mac) | Erwartetes Ergebnis | ✅ Prüfung |
|---|----------------------|---------------------|-----------|
| 0.1 | **Homebrew** installiert? Terminal öffnen, `brew --version` eingeben. | Eine Versionszeile erscheint, z. B. `Homebrew 4.x.x`. | Ausgabe beginnt mit `Homebrew`. Fehlt Homebrew: von <https://brew.sh> installieren, dann 0.1 wiederholen. |

> Mehr braucht der Mac **nicht** — Multipass bringt die komplette Linux-Umgebung
> mit. Multipass wählt automatisch die passende Architektur (Apple Silicon →
> ARM64, Intel → x86_64); beide Container-Images bauen dann **nativ**, ohne
> langsame Emulation.

---

## Teil 1 — Linux-VM mit Multipass anlegen

| # | Aktion (auf dem Mac) | Erwartetes Ergebnis | ✅ Prüfung |
|---|----------------------|---------------------|-----------|
| 1.1 | `brew install --cask multipass` | Installation endet ohne Fehler. | `multipass version` gibt eine Zeile `multipass 1.x.x` aus. |
| 1.2 | VM starten (dauert 1–2 min): <br>`multipass launch --name asd --cpus 4 --memory 8G --disk 40G` | Am Ende steht `Launched: asd`. | siehe 1.3. |
| 1.3 | `multipass list` | Eine Zeile für `asd`. | Spalte **State** = `Running`, und eine **IPv4**-Adresse ist eingetragen. |
| 1.4 | **VM-IP notieren:** `multipass info asd \| grep IPv4` | Zeile wie `IPv4: 192.168.64.7`. | Es erscheint **genau eine** IPv4-Adresse. **Diese Adresse ist ab jetzt `〈VM-IP〉`** (in Teil 5 im Browser gebraucht). |

> Die `〈VM-IP〉` liegt in einem privaten Netz (meist `192.168.64.x`) und ist vom
> Mac aus **direkt erreichbar** — genau das brauchen wir für den Browser-Zugriff
> auf das ASD.

---

## Teil 2 — Docker in der VM einrichten

Ab hier arbeiten wir **in der VM**.

| # | Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|--------|---------------------|-----------|
| 2.1 | **Auf dem Mac:** `multipass shell asd` | Der Prompt wechselt zu `ubuntu@asd:~$`. | Der Prompt beginnt mit `ubuntu@asd`. |
| 2.2 | **In der VM:** <br>`sudo apt-get update` <br>`sudo apt-get install -y docker.io docker-compose-v2 git` | Pakete werden installiert; endet ohne rot markierten Fehler. | Letzte Zeilen ohne `E:`-Fehler. |
| 2.3 | **In der VM:** `sudo usermod -aG docker $USER` | Keine Ausgabe (Erfolg ist stumm). | — |
| 2.4 | Gruppe aktivieren: `exit` (zurück auf den Mac), dann **auf dem Mac** erneut `multipass shell asd`. | Wieder `ubuntu@asd:~$`. | Der neue Shell-Login hat die `docker`-Gruppe. |
| 2.5 | **In der VM:** `docker run --rm hello-world` | Docker lädt ein Test-Image und führt es aus. | Die Ausgabe enthält **wörtlich** die Zeile `Hello from Docker!`. Erscheint stattdessen `permission denied`, wurde 2.4 übersprungen — nachholen. |
| 2.6 | **In der VM:** `docker compose version` und `git --version` | Je eine Versionszeile. | `Docker Compose version v2.x` **und** `git version 2.x`. |

---

## Teil 3 — Repos holen, Firefly-Image bauen, Stack starten

Weiter **in der VM**.

| # | Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|--------|---------------------|-----------|
| 3.1 | Projektordner + beide Repos als **Geschwister** klonen: <br>`mkdir -p ~/asd && cd ~/asd` <br>`git clone https://github.com/manuelringwald/firefly.git` <br>`git clone https://github.com/manuelringwald/wayfinder.git` | Zwei Klone laufen durch. | `ls ~/asd` gibt **genau** aus: `firefly  wayfinder`. |
| 3.2 | Firefly-Image bauen (erstes Mal einige Minuten, Rust-Compiler): <br>`cd ~/asd/firefly && docker build -t firefly:latest .` | Build endet mit einer Zeile wie `naming to docker.io/library/firefly:latest`. | `docker image inspect firefly:latest --format '{{.Id}}'` gibt eine `sha256:…`-ID aus (kein `No such image`). |
| 3.3 | Orchestrierten Stack bauen+starten (Go-Compiler beim ersten Mal): <br>`cd ~/asd/wayfinder` <br>`docker compose -f docker-compose.orchestrated.yml up --build -d` | Baut `wayfinder` + `orchestrator`, startet `db`, `wayfinder`, `orchestrator`; kehrt zur Eingabeaufforderung zurück. | Keine Fehlermeldung; siehe 3.4. |
| 3.4 | `docker compose -f docker-compose.orchestrated.yml ps` | Drei Dienste gelistet. | `db` zeigt `Up … (healthy)`; `wayfinder` und `orchestrator` zeigen `Up …`. |
| 3.5 | Server lebt? `curl -s localhost:8080/health` | — | Ausgabe ist **exakt** `ok`. |
| 3.6 | Feed noch nicht abonniert → noch keine Daten: <br>`curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/ready` | — | Ausgabe ist **`503`**. **Das ist korrekt** (noch kein Feed/Heartbeat). Nach Teil 4/5 wird daraus `200`. |

> **Wenn du nur den automatischen Nachweis willst,** kannst du Teil 3.3–3.6
> überspringen — das Skript in Teil 4 startet und stoppt den Stack selbst. Für die
> UI-Abnahme in Teil 5 brauchst du den Stack laufend (Teil 3.3).

---

## Teil 4 — Automatischer Abnahme-Lauf (deterministisch)

Dieser Lauf ist **vollständig offline** und **reproduzierbar**: Er seedet direkt
in der Datenbank einen Mandanten + Feed **ohne** Live-Quellen; der Orchestrator
spawnt daraufhin eine Firefly-Instanz, die die **Frankfurt-Demo-Szene** abspielt
und CAT062-Tracks sendet. Kein Internet, keine UI nötig.

| # | Aktion (in der VM, im Ordner `~/asd/wayfinder`) | Erwartetes Ergebnis |
|---|--------------------------------------------------|---------------------|
| 4.1 | `./scripts/e2e-orchestrated.sh --mode scene` | Das Skript fährt den Stack hoch, prüft die Kette Punkt für Punkt und räumt am Ende auf. |

**Erwartete Ausgabe — jede dieser Zeilen muss erscheinen (IDs können 1 oder höher sein):**

```
→ preflight
  ✓ Docker daemon up, Firefly image 'firefly:latest' present
  ✓ schema ready
  ✓ seeded feed id=1 (tenant id=1)
→ checkpoint 1 — orchestrator spawns the tracker container
  ✓ container running: wayfinder-firefly-feed-1
→ checkpoint 2 — container env matches the spec
  ✓ endpoint + FIREFLY_SCENE present (placeholder source)
→ checkpoint 5 — tracks reach the ASD (server /metrics)
  ✓ ASD received CAT062 tracks (wayfinder_cat062_tracks_received_total > 0)
→ checkpoint 8 — unsubscribe triggers orphan cleanup
  ✓ tracker torn down after unsubscribe

✅ E2E acceptance (scene) passed.
```

**Bestanden heißt eindeutig:**

1. Die **allerletzte** Zeile ist **exakt** `✅ E2E acceptance (scene) passed.`, **und**
2. bei **checkpoint 5** steht ein **`✓ ASD received CAT062 tracks …`** (nicht `⚠ WARN`).

Steht bei checkpoint 5 stattdessen `⚠ WARN: no CAT062 tracks observed …`, ist der
Spawn-/Aufräum-Teil bestanden, aber es kamen keine Tracks an → siehe
[Teil 8](#teil-8--fehlerbehebung). Bricht das Skript mit `✗ FAIL:` ab, nennt die
Zeile den fehlgeschlagenen Prüfpunkt.

> **Optionaler Live-Lauf.** `./scripts/e2e-orchestrated.sh --mode opensky-anon`
> nutzt statt der Szene eine **echte** anonyme ADS-B-Quelle (OpenSky). Er braucht
> **Internet** aus der VM; die Trefferzahl hängt vom realen Verkehr ab, deshalb ist
> checkpoint 5 hier bewusst nur ein **Hinweis** (`⚠ WARN` ist kein Fehler). Für den
> **deterministischen** Nachweis gilt `--mode scene`.

---

## Teil 5 — Abnahme in der Browser-UI

Jetzt der menschliche Durchlauf: **ein** Terminal-Befehl zum Start, alles Weitere
im Browser. Voraussetzung: Der Stack aus **Teil 3.3** läuft (falls du Teil 4
gefahren hast, hat es den Stack wieder abgebaut — dann Teil 3.3 erneut ausführen).

> Wir verwenden bewusst einen Feed **ohne** Live-Quellen. Dann spielt die
> gespawnte Firefly-Instanz die **Frankfurt-Szene** (acht Flugzeuge, ~40 min) ab —
> so ist garantiert Verkehr zu sehen, unabhängig von echtem Flugaufkommen.

### 5.1 Anmelden + Passwortwechsel

| # | UI-Aktion (Browser **auf dem Mac**) | Erwartetes Ergebnis | ✅ Prüfung |
|---|-------------------------------------|---------------------|-----------|
| 5.1.1 | `http://〈VM-IP〉:8081/admin` öffnen (die IP aus Schritt 1.4). | Login-Maske „Anmelden" (Benutzername/Passwort). | Maske erscheint. Lädt nichts → [Teil 8](#teil-8--fehlerbehebung). |
| 5.1.2 | Anmelden mit `admin` / `admin`. | Sofort die Maske **„Passwort ändern"** (erzwungen). | Kein Zugriff auf die Tabs vor dem Wechsel. |
| 5.1.3 | Neues Passwort (≥ 8 Zeichen) zweimal eingeben, bestätigen. | Admin-Dashboard mit den Tabs **Mandanten**, **Feeds**, **Plattform-Administratoren**. | Die drei Tabs sind sichtbar. |

### 5.2 Kunden anlegen

| # | UI-Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|-----------|---------------------|-----------|
| 5.2.1 | Tab **Mandanten** → **Neuer Mandant**: Slug `demo`, Name `Demo Frankfurt`. | Mandant erscheint in der Liste. | Eintrag „Demo Frankfurt" sichtbar. |
| 5.2.2 | Mandant **Demo Frankfurt** öffnen („Konfigurieren") → Karte **Nutzer** → **Neuer Nutzer**: Subject `lotse`, Passwort (≥ 8), Rolle Nutzer. | Nutzer erscheint in der Nutzerliste des Mandanten. | Eintrag „lotse" sichtbar. |
| 5.2.3 | Tab **Feeds** → **Neuer Feed**: Name `frankfurt-demo`, Sensor-Mix `PSR, SSR, ADS-B`, **Endpoint automatisch = AN**. **Keine** Quellen hinzufügen. | Feed erscheint mit **automatisch** vergebener Adresse. | Feed-Zeile zeigt eine `239.255.0.x:8600`-Adresse. |

> **Wichtig — Unterschied zum Nicht-Orchestrierten Weg:** Hier ist **„Endpoint
> automatisch = AN"** richtig. Der Orchestrator startet die Firefly-Instanz
> **genau auf dieser vergebenen Adresse**, und der ASD-Server hört dort zu. (Nur
> im VM-losen [Anhang A](#anhang-a--schnell-check-ohne-vm-nur-auf-dem-mac) muss man
> den Endpoint **fest** eintragen.)

### 5.3 Sicht setzen + Feed zuweisen

| # | UI-Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|-----------|---------------------|-----------|
| 5.3.1 | Im Mandanten **Demo Frankfurt** → **View-Config**: Zentrum `50.04` / `8.56`, Radius `100` (NM), Zoom `8`, FL `0`–`450`. Speichern. | Sicht gespeichert. | Werte stehen nach Reload unverändert da. |
| 5.3.2 | Im Mandanten → **Feeds** → Feed `frankfurt-demo` **zuweisen** (Grant). | Feed ist dem Mandanten zugewiesen. | Feed zeigt Status **„Granted"**. |

> Nach 5.3.2 spawnt der Orchestrator innerhalb weniger Sekunden die Firefly-
> Instanz (der Beleg dafür kommt in Teil 6).

### 5.4 Als Kunde anmelden und Tracks sehen

| # | UI-Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|-----------|---------------------|-----------|
| 5.4.1 | Oben rechts **Abmelden** (Admin). Am besten ein **privates Browserfenster** öffnen. | Zurück zur Login-Maske. | Login-Maske erscheint. |
| 5.4.2 | `http://〈VM-IP〉:8081/` öffnen, anmelden als `lotse` + Passwort. | Karte lädt, zentriert auf **Frankfurt** (50.04/8.56, Zoom 8); oben rechts der Konto-Chip `lotse`. | Kartenausschnitt = Raum Frankfurt. |
| 5.4.3 | Wenige Sekunden warten. | **Ca. acht** Flugzeug-Tracks erscheinen und **bewegen sich**; oben links ein **grüner Banner „FEED OK"**. | Bewegte Track-Symbole sichtbar **und** Banner grün. |

> **Sichtbar bleibend ~40 min:** Die Frankfurt-Szene läuft rund 40 Minuten und
> endet dann; danach kommen keine neuen Tracks mehr. Das ist **erwartetes**
> Verhalten, kein Fehler. Für einen neuen Lauf den Stack neu starten (Teil 7 → 3).

---

## Teil 6 — Hinter den Kulissen prüfen

Rein zur **Bestätigung**, dass die UI-Konfiguration real wirkt. Diese Befehle
laufen **in der VM** (`multipass shell asd`, dann `cd ~/asd/wayfinder`). `〈id〉` ist
die Feed-ID aus dem Container-Namen in 6.1.

| # | Prüf-Befehl | Erwartetes Ergebnis |
|---|-------------|---------------------|
| 6.1 | `docker ps --filter label=wayfinder.feed_id --format '{{.Names}}'` | Genau ein Name der Form **`wayfinder-firefly-feed-〈id〉`** (vom Orchestrator gespawnt). |
| 6.2 | `docker inspect wayfinder-firefly-feed-〈id〉 --format '{{json .Config.Env}}'` | Enthält `FIREFLY_CAT062_GROUP=239.255.0.x` und `FIREFLY_CAT062_PORT=8600` (**die Feed-Adresse aus 5.2.3**) sowie `FIREFLY_SCENE=frankfurt`. |
| 6.3 | `docker logs wayfinder-firefly-feed-〈id〉 2>&1 \| grep -i cat062` | Zeile **`CAT062 multicast feed enabled`** mit Ziel **`239.255.0.x:8600`**. |
| 6.4 | `curl -s localhost:8080/metrics \| grep cat062` | `wayfinder_cat062_blocks_received_total` **und** `wayfinder_cat062_tracks_received_total` sind **> 0**. |
| 6.5 | `curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/ready` | **`200`** (Feed aktiv — mindestens ein CAT065-Heartbeat empfangen). |
| 6.6 | In der Admin-UI: Tab **Feeds** → **Feed-Gesundheit** des zugewiesenen Feeds. | Der Feed-Chip ist **grün** (`ever_seen=true`, Heartbeat läuft). |

---

## Teil 7 — Aufräumen

| # | Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|--------|---------------------|-----------|
| 7.1 | **In der VM**, im Ordner `~/asd/wayfinder`: <br>`docker compose -f docker-compose.orchestrated.yml down -v --remove-orphans` <br>`docker ps -aq --filter 'label=wayfinder.managed=true' \| xargs -r docker rm -f` | Stack + Datenbank-Volume + gespawnte Firefly-Container entfernt. | `docker ps` listet keine `wayfinder-*`-Container mehr. |
| 7.2 | VM anhalten (Zustand bleibt): **auf dem Mac** `multipass stop asd`. | VM gestoppt. | `multipass list` zeigt `asd  Stopped`. |
| 7.3 | *(optional)* VM ganz löschen: `multipass delete asd --purge`. | VM vollständig entfernt. | `multipass list` enthält kein `asd` mehr. |

> Nur **anhalten** (7.2), wenn du später weitertesten willst — ein
> `multipass start asd` bringt sie in Sekunden zurück. **Löschen** (7.3) gibt den
> Speicher frei, dann beginnt ein neuer Test wieder bei Teil 1.2.

---

## Teil 8 — Fehlerbehebung

| Symptom | Ursache | Lösung |
|---------|---------|--------|
| **`http://〈VM-IP〉:8081` lädt nicht** im Mac-Browser | Falsche IP, oder `localhost` statt VM-IP verwendet, oder Server noch nicht oben. | 1) Server-Check **in der VM**: `curl -s localhost:8080/health` → muss `ok` sein. 2) IP neu holen: `multipass info asd \| grep IPv4`. 3) **Nicht** `localhost:8081` am Mac benutzen — die Ports liegen auf der VM. |
| **`docker run hello-world` → `permission denied`** | Schritt 2.4 (Gruppe aktivieren) übersprungen. | `exit`, dann auf dem Mac erneut `multipass shell asd`; 2.5 wiederholen. |
| **Skript (Teil 4): `✗ FAIL: Firefly image 'firefly:latest' not found`** | Teil 3.2 nicht gemacht. | `cd ~/asd/firefly && docker build -t firefly:latest .`, dann Teil 4 erneut. |
| **Skript: checkpoint 5 zeigt `⚠ WARN: no CAT062 tracks`** | Multicast überquert den Host nicht, oder die Szene ist still. | Läuft die VM als **echter** Linux-Host (ja bei Multipass)? `docker logs wayfinder-firefly-feed-〈id〉` prüfen: erscheint `CAT062 multicast feed enabled`? |
| **UI: Karte bleibt leer** | Feed nicht zugewiesen (5.3.2), Sicht-AOI zu klein (Tracks außerhalb), oder Szene nach ~40 min zu Ende. | Zuweisung prüfen (Status „Granted"); Radius in 5.3.1 auf `100` NM setzen; Stack neu starten (Teil 7 → 3.3). |
| **`docker compose … up` bricht mit Build-Fehler ab** | Zu wenig RAM/Disk oder Netzwerkabbruch beim ersten Abhängigkeits-Download. | VM größer neu anlegen: `multipass delete asd --purge` und `multipass launch … --memory 8G --disk 40G` erneut. |
| **`db` wird nicht `healthy`** | Datenbank braucht ein paar Sekunden. | 10 s warten, `docker compose -f docker-compose.orchestrated.yml ps` erneut; bleibt es `unhealthy`: `docker compose -f docker-compose.orchestrated.yml logs db`. |

---

## Anhang A — Schnell-Check ohne VM (nur auf dem Mac)

Wenn du **keinen** vollständigen orchestrierten Lauf brauchst, sondern nur schnell
die **UI + Live-Tracks** auf dem Mac sehen willst, gibt es einen VM-losen Weg über
ein **gemeinsames Bridge-Netz** (`docker-compose.bridge.yml`, Details in
`DOCKER.md`). Container↔Container-Multicast funktioniert dort auch unter Docker
Desktop.

**Abdeckung — dieser Weg zeigt weniger:**

| Prüf-Baustein | Bridge (Mac, ohne VM) | Voller Lauf (Multipass, Teil 1–6) |
|---|---|---|
| UI-Einrichtung (Login, Mandant, Nutzer, Feed, Sicht, Zuweisung) | ✅ | ✅ |
| Live-Tracks auf der Karte | ✅ | ✅ |
| Orchestrator-**Auto-Spawn je Feed** + Aufräumen (checkpoints 1/2/8) | ❌ | ✅ |
| Automatischer Skript-Nachweis `e2e-orchestrated.sh` | ❌ | ✅ |

**Ablauf (Kurzform):**

1. Firefly-Repo als **Geschwister** von `wayfinder/` klonen (wie Teil 3.1, aber auf
   dem Mac, z. B. unter `~/asd/`).
2. `cd ~/asd/wayfinder && docker compose -f docker-compose.bridge.yml up --build`.
3. Browser: `http://localhost:8081/admin` (Login `admin`/`admin`, Passwortwechsel).
4. **Entscheidender Unterschied:** Da es hier **keinen** Orchestrator gibt, ist
   Firefly ein **fester** Sender auf `239.255.0.62:8600`. Beim Feed-Anlegen deshalb
   **„Endpoint automatisch = AUS"** und Gruppe **`239.255.0.62`** / Port **`8600`**
   **von Hand** eintragen, dann dem Mandanten zuweisen.

   **Erwartetes Ergebnis:** Nach der Anmeldung als Mandant erscheinen die
   Frankfurt-Tracks; `curl -s localhost:8080/metrics | grep cat062` zeigt Werte
   **> 0**.
