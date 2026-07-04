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
> nativ wie auf einem Linux-Server. (Der Weg **ohne** eigene VM ist heute ein
> GitHub Codespace — siehe [Anhang A](#anhang-a--schnell-check-ohne-vm-entfallen--codespaces)
> bzw. `docs/CODESPACES.md`.)

---

## Was am Ende nachgewiesen ist

| # | Behauptung | Nachgewiesen in |
|---|------------|-----------------|
| 1 | Aus einem frischen Mac mini wird mit wenigen Befehlen ein **echter Linux-Docker-Host** (Multipass-VM). | Teil 1–2 |
| 2 | Der **orchestrierte Stack** (Datenbank + ASD-Server + Orchestrator) startet vollständig. | Teil 3 |
| 3 | *(optional)* Ein **offline Smoke-Test** belegt die Kette Feed → Auto-Spawn → CAT065-Multicast → ASD → Aufräumen mit einem bewusst quellenlosen Feed (leerer Himmel + Heartbeat, Firefly ADR 0030). | Teil 4 |
| 4 | Die **ganze Kundeneinrichtung** (Mandant, Zugang, Feed, Sicht, Zuweisung) läuft in der **Browser-UI**, mit **echten** ADS-B-/FLARM-Daten (Raum Frankfurt); der angemeldete Mandant sieht **live Tracks**. | Teil 5 |
| 5 | **Hinter den Kulissen** belegen Container, Logs und Metriken jede Stufe der Kette. | Teil 6 |

**Inhalt:**
[Teil 0 Voraussetzungen](#teil-0--voraussetzungen) ·
[Teil 1 Linux-VM](#teil-1--linux-vm-mit-multipass-anlegen) ·
[Teil 2 Docker in der VM](#teil-2--docker-in-der-vm-einrichten) ·
[Teil 3 Repos + Stack](#teil-3--repos-holen-firefly-image-bauen-stack-starten) ·
[Teil 4 Optionaler Offline-Smoke-Test](#teil-4--optionaler-offline-smoke-test) ·
[Teil 5 UI-Abnahme mit echten Daten](#teil-5--ui-abnahme-mit-echten-daten) ·
[Teil 6 Hinter den Kulissen](#teil-6--hinter-den-kulissen-prüfen) ·
[Teil 7 Aufräumen](#teil-7--aufräumen) ·
[Teil 8 Fehlerbehebung](#teil-8--fehlerbehebung) ·
[Anhang A ohne VM → Codespaces](#anhang-a--schnell-check-ohne-vm-entfallen--codespaces)

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

> **Wenn du nur den optionalen Offline-Smoke-Test willst,** kannst du Teil 3.3–3.6
> überspringen — das Skript in Teil 4 startet und stoppt den Stack selbst. Für die
> UI-Abnahme in Teil 5 (der Hauptweg) brauchst du den Stack laufend (Teil 3.3).

---

## Teil 4 — Optionaler Offline-Smoke-Test

> **Dieser Teil ist optional.** Er ersetzt **nicht** die eigentliche Abnahme —
> die läuft mit **echten** Daten in [Teil 5](#teil-5--ui-abnahme-mit-echten-daten).
> Teil 4 ist ein kurzer, **offline** laufender Smoke-Test, der nur die technische
> Kette (Feed → Auto-Spawn → CAT062-Multicast → ASD → Aufräumen) belegt, **ohne**
> echten Verkehr und ohne Internet. Nützlich als schneller Vorab-Check, bevor du
> mit Teil 5 die eigentliche Abnahme mit echten Daten machst — aber überspringbar,
> wenn du direkt zu Teil 5 willst.

Dieser Lauf ist **vollständig offline** und **reproduzierbar**: Er seedet direkt
in der Datenbank einen Mandanten + Feed **ohne** Live-Quellen; der Orchestrator
spawnt daraufhin eine Firefly-Instanz mit **bewusst leerem Himmel** — sie sendet
keine Tracks, aber den **CAT065-Heartbeat** (Firefly ADR 0030). Genau der
beweist die Kette Spawn → Multicast → ASD-Empfang. Kein Internet, keine UI nötig.

| # | Aktion (in der VM, im Ordner `~/asd/wayfinder`) | Erwartetes Ergebnis |
|---|--------------------------------------------------|---------------------|
| 4.1 | `./scripts/e2e-orchestrated.sh --mode empty` | Das Skript fährt den Stack hoch, prüft die Kette Punkt für Punkt und räumt am Ende auf. |

**Erwartete Ausgabe — jede dieser Zeilen muss erscheinen (IDs können 1 oder höher sein):**

```
→ preflight
  ✓ Docker daemon up, Firefly image 'firefly:latest' present
  ✓ schema ready
  ✓ seeded feed id=1 (tenant id=1)
→ checkpoint 1 — orchestrator spawns the tracker container
  ✓ container running: wayfinder-firefly-feed-1
→ checkpoint 2 — container env matches the spec
  ✓ endpoint + FIREFLY_SOURCES=[] present (empty sky contract)
→ checkpoint 5 — heartbeat reaches the ASD (server /metrics)
  ✓ ASD received the feed signal (wayfinder_cat065_heartbeats_received_total > 0)
→ checkpoint 8 — unsubscribe triggers orphan cleanup
  ✓ tracker torn down after unsubscribe

✅ E2E acceptance (empty) passed.
```

**Bestanden heißt eindeutig:**

1. Die **allerletzte** Zeile ist **exakt** `✅ E2E acceptance (empty) passed.`, **und**
2. bei **checkpoint 5** steht das **`✓ ASD received the feed signal …`**.

Ein quellenloser Feed sendet **absichtlich keine Tracks** (leerer Himmel) —
Tracks prüft die eigentliche Abnahme mit echten Quellen in Teil 5. Bricht das
Skript mit `✗ FAIL:` ab, nennt die Zeile den fehlgeschlagenen Prüfpunkt.

> **Das war's für Teil 4.** Für die eigentliche Abnahme mit echten Daten geht es
> jetzt in [Teil 5](#teil-5--ui-abnahme-mit-echten-daten) weiter — dort wird der
> Stack (Teil 3.3) noch einmal frisch gebraucht, falls Teil 4 ihn wieder abgebaut
> hat.

---

## Teil 5 — UI-Abnahme mit echten Daten

Das ist der **Hauptweg** dieser Abnahme: die komplette Kundeneinrichtung
(Mandant, Zugang, Feed, Quellen, Sicht, Zuweisung) in der Browser-UI, mit
**echten** ADS-B- und FLARM-Daten im Raum Frankfurt — **keine** Simulation.
Voraussetzung: Der Stack aus **Teil 3.3** läuft (falls du Teil 4
gefahren hast, hat es den Stack wieder abgebaut — dann Teil 3.3 erneut
ausführen).

> **Nicht-deterministisch, mit Absicht.** Anders als der Heartbeat-Smoke-Test
> aus Teil 4 hängt das Track-Bild jetzt vom **echten** Flugverkehr ab. Es gibt **keine**
> Garantie auf eine feste Anzahl Tracks zu einem festen Zeitpunkt — das ist
> erwartet, kein Fehler. ADS-B ist um Frankfurt dicht beflogen und sollte
> zuverlässig Tracks liefern; **FLARM** (Segelflug/GA) ist wetter- und
> tageszeitabhängig und kann **spärlich oder zeitweise leer** sein — auch das ist
> kein Fehler, siehe die einzelnen Prüfschritte unten.

Wir prüfen **drei Feeds nacheinander**, jeweils als Lotse in der UI:

1. Ein Feed **nur ADS-B** ([5.4](#54-feed-1--nur-ads-b-prüfen)),
2. ein Feed **nur FLARM** ([5.5](#55-feed-2--nur-flarm-prüfen)),
3. ein Feed **ADS-B + FLARM kombiniert** ([5.6](#56-feed-3--ads-b--flarm-kombiniert-prüfen)).

Damit dafür **kein** `multi_feed`-Entitlement nötig ist (ein Mandant ohne dieses
Recht darf höchstens einen Feed gleichzeitig zugewiesen bekommen — siehe
`docs/BETRIEB.md`), weisen wir **immer nur einen Feed auf einmal** zu: prüfen,
dann **entziehen**, dann den nächsten zuweisen. So bleiben die drei Prüfungen
sauber voneinander isoliert.

### 5.1 Anmelden + Passwortwechsel

| # | UI-Aktion (Browser **auf dem Mac**) | Erwartetes Ergebnis | ✅ Prüfung |
|---|-------------------------------------|---------------------|-----------|
| 5.1.1 | `http://〈VM-IP〉:8081/admin` öffnen (die IP aus Schritt 1.4). | Login-Maske „Anmelden" (Benutzername/Passwort). | Maske erscheint. Lädt nichts → [Teil 8](#teil-8--fehlerbehebung). |
| 5.1.2 | Anmelden mit `admin` / `admin`. | Sofort die Maske **„Passwort ändern"** (erzwungen). | Kein Zugriff auf die Tabs vor dem Wechsel. |
| 5.1.3 | Neues Passwort (≥ 8 Zeichen) zweimal eingeben, bestätigen. | Admin-Dashboard mit den Tabs **Mandanten**, **Feeds**, **Plattform-Administratoren**. | Die drei Tabs sind sichtbar. |

### 5.2 Mandant + Zugang anlegen

| # | UI-Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|-----------|---------------------|-----------|
| 5.2.1 | Tab **Mandanten** → **Mandant anlegen**: Slug `demo`, Name `Demo Frankfurt`. | Mandant erscheint in der Liste. | Eintrag „Demo Frankfurt" sichtbar. |
| 5.2.2 | Mandant **Demo Frankfurt** öffnen („Konfigurieren") → Abschnitt **„Zugänge"** → **„Zugang anlegen"**: Benutzername `lotse`, E-Mail (optional) leer lassen, Passwort (optional, min. 8 Zeichen) setzen, dann **„Anlegen"**. | Zugang erscheint in der Zugänge-Tabelle des Mandanten. | Eintrag „lotse" sichtbar. |

> **Keine Rollenauswahl nötig.** Mandanten-Zugänge sind immer die Rolle `user` —
> es gibt im Dialog kein Rollenfeld.

### 5.3 Sicht setzen + drei Feeds anlegen

Wir legen jetzt **drei Feeds** im globalen Feed-Katalog an (Tab **Feeds**) — sie
werden erst in 5.4–5.6 einzeln dem Mandanten zugewiesen. Die Quell-Abdeckung
wird — konsistent mit der Standard-Ansicht — als **Zentrum + Radius** eingegeben
(die interne Query-BBox leitet der Server daraus ab, Issue #109). Am einfachsten
per Dropdown **„Aus Mandant übernehmen"** → **Demo Frankfurt**: dann werden
Zentrum `50.04 / 8.56` und Radius `100` NM automatisch aus der Standard-Ansicht
übernommen (Issue #113).

| # | UI-Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|-----------|---------------------|-----------|
| 5.3.1 | Im Mandanten **Demo Frankfurt** → **Standard-Ansicht**: Zentrum `50.04` / `8.56`, Radius `100` (NM), Zoom `8`, FL `0`–`450`. Speichern. | Sicht gespeichert. | Werte stehen nach Reload unverändert da. |
| 5.3.2 | Tab **Feeds** → **„Feed anlegen"**: Feld „Name" = `frankfurt-adsb`; Schalter **„Multicast-Endpoint automatisch zuweisen"** = **AN** lassen; Feld **„Sensor-Mix (optional)"** **leer** lassen; **„Anlegen"**. | Feed erscheint mit **automatisch** vergebener Adresse. | Feed-Zeile zeigt eine `239.255.0.x:8600`-Adresse. |
| 5.3.3 | In der Feed-Zeile von `frankfurt-adsb`: Button **„Quellen"** → **„Quelle hinzufügen"** → **Quell-Typ** = **„ADS-B (OpenSky)"**; Dropdown **„Aus Mandant übernehmen"** = **Demo Frankfurt** (füllt Zentrum `50.04`/`8.56` + Radius `100`); **„Speichern"**. | Quelle ist dem Feed zugeordnet. | Dialog „Quellen — frankfurt-adsb" zeigt eine Zeile mit Typ ADS-B (OpenSky) und Zentrum/Radius. |
| 5.3.4 | Wie 5.3.2, aber Name `frankfurt-flarm`. | Zweiter Feed erscheint mit eigener automatischer Adresse. | Feed-Zeile `frankfurt-flarm` mit eigener `239.255.0.x:8600`-Adresse. |
| 5.3.5 | Wie 5.3.3, aber am Feed `frankfurt-flarm`, Quell-Typ **„FLARM (OGN/APRS)"**, gleiches Zentrum + Radius (wieder per **„Aus Mandant übernehmen"** = Demo Frankfurt). | Quelle ist dem Feed zugeordnet. | Dialog „Quellen — frankfurt-flarm" zeigt eine Zeile mit Typ FLARM (OGN/APRS) und Zentrum/Radius. |
| 5.3.6 | Wie 5.3.2, aber Name `frankfurt-kombiniert`. | Dritter Feed erscheint mit eigener automatischer Adresse. | Feed-Zeile `frankfurt-kombiniert` mit eigener `239.255.0.x:8600`-Adresse. |
| 5.3.7 | Am Feed `frankfurt-kombiniert`: Button **„Quellen"** → **zwei** Quellen nacheinander mit **„Quelle hinzufügen"** anlegen — einmal Quell-Typ **„ADS-B (OpenSky)"**, einmal **„FLARM (OGN/APRS)"**, bei beiden **„Aus Mandant übernehmen"** = Demo Frankfurt; einmal **„Speichern"** für beide. | Beide Quellen sind dem Feed zugeordnet. | Dialog „Quellen — frankfurt-kombiniert" zeigt **zwei** Zeilen (ADS-B und FLARM). |

> **„Sensor-Mix (optional)" bewusst leer lassen.** Der Sensor-Mix ist eine
> reine Anzeige-Eigenschaft und wird automatisch aus den Quellen abgeleitet
> (Issue #102) — von Hand pflegen ist überflüssig und kann veralten. Nach dem
> **Speichern der Quellen** erscheint der abgeleitete Sensor-Mix **sofort** in
> der Feed-Zeile (Issue #112), ohne Tab-Wechsel.
>
> **Warum „Multicast-Endpoint automatisch zuweisen" = AN richtig ist:** Der
> Orchestrator startet die Firefly-Instanz **genau auf der vergebenen Adresse**,
> und der ASD-Server hört dort zu.

> **Neue Sidebar-Gliederung (Issues #115/#116).** In der Lotsen-Sicht ist die
> Sidebar links **standardmäßig eingeklappt** — nur die schmale Icon-Leiste
> (Sidecar) ist sichtbar, die Karte bekommt die volle Fläche. Über die Icons
> öffnet man je eine Sektion: **Layer** (Layer-Schalter + Legende „Spurherkunft"),
> **Filter** (FL-Filter; der zulässige FL-Bereich der Standard-Ansicht steht grau
> als Hinweis darunter) und ganz unten **Konto** (Abmelden). Der Layer
> **„Radarabdeckung"** ist bei ADS-B/FLARM-Feeds **deaktiviert** (kein Radar →
> keine Abdeckungsdaten, Issue #114) — das ist erwartungsgemäß.

### 5.4 Feed 1 — nur ADS-B prüfen

| # | UI-Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|-----------|---------------------|-----------|
| 5.4.1 | Im Mandanten **Demo Frankfurt** → Abschnitt **„Feed-Zuweisungen"** → bei `frankfurt-adsb` **„Zuweisen"**. | Feed ist dem Mandanten zugewiesen. | Status-Chip zeigt **„zugewiesen"**. |
| 5.4.2 | Oben rechts **Abmelden** (Admin). Am besten ein **privates Browserfenster** öffnen. | Zurück zur Login-Maske. | Login-Maske erscheint. |
| 5.4.3 | `http://〈VM-IP〉:8081/` öffnen, anmelden als `lotse` + Passwort. | Karte lädt, zentriert auf **Frankfurt** (50.04/8.56, Zoom 8); oben rechts der **Feed-Status-Badge** (der frühere `lotse`-Konto-Chip ist entfallen — Konto liegt in der Sidebar unter **Konto**). | Kartenausschnitt = Raum Frankfurt. |
| 5.4.4 | Etwas warten (echter Verkehr, keine feste Zeit). | Tracks erscheinen, sobald realer ADS-B-Verkehr im Gebiet ist; oben rechts der **grüne Feed-Chip „FEED OK"** (zeigt jetzt korrekt die Feed-Gesundheit statt dauerhaft „FEED ?", Issue #117). ADS-B um Frankfurt ist dicht beflogen → sollte zuverlässig kommen. | Mindestens ein bewegtes Track-Symbol sichtbar **und** Feed-Chip grün („FEED OK"). |
| 5.4.5 | Zurück im Admin-Fenster: im Mandanten → **„Feed-Zuweisungen"** → bei `frankfurt-adsb` **„Entziehen"**. | Feed ist dem Mandanten nicht mehr zugewiesen. | Status-Chip zeigt wieder **„—"** statt „zugewiesen". |

### 5.5 Feed 2 — nur FLARM prüfen

| # | UI-Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|-----------|---------------------|-----------|
| 5.5.1 | Im Mandanten → **„Feed-Zuweisungen"** → bei `frankfurt-flarm` **„Zuweisen"**. | Feed ist dem Mandanten zugewiesen. | Status-Chip zeigt **„zugewiesen"**. |
| 5.5.2 | Als `lotse` (privates Fenster von 5.4.2, ggf. Seite neu laden). | Karte lädt wie zuvor. | Kartenausschnitt = Raum Frankfurt. |
| 5.5.3 | Etwas warten. | Tracks erscheinen, sobald reale FLARM-Sender (Segelflug/GA) im Gebiet aktiv sind. **FLARM ist wetter- und tageszeitabhängig** — je nach Lage kann die Karte **spärlich besetzt oder zeitweise leer** bleiben. Das ist **kein Fehler**, solange der Feed-Banner grün bleibt (Feed lebt, es fliegt nur gerade niemand). | Banner **grün** (Feed-Heartbeat da); Tracks **wenn** Segelflugverkehr aktiv ist — leere Karte bei grünem Banner ist an sich schon ein bestandener Schritt. |
| 5.5.4 | Zurück im Admin-Fenster: im Mandanten → **„Feed-Zuweisungen"** → bei `frankfurt-flarm` **„Entziehen"**. | Feed ist dem Mandanten nicht mehr zugewiesen. | Status-Chip zeigt wieder **„—"**. |

### 5.6 Feed 3 — ADS-B + FLARM kombiniert prüfen

| # | UI-Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|-----------|---------------------|-----------|
| 5.6.1 | Im Mandanten → **„Feed-Zuweisungen"** → bei `frankfurt-kombiniert` **„Zuweisen"**. | Feed ist dem Mandanten zugewiesen. | Status-Chip zeigt **„zugewiesen"**. |
| 5.6.2 | Als `lotse` (Seite neu laden). | Karte lädt wie zuvor. | Kartenausschnitt = Raum Frankfurt. |
| 5.6.3 | Etwas warten. | Tracks aus **beiden** Quellen können erscheinen — ADS-B üblicherweise zuverlässig, FLARM wetter-/tageszeitabhängig (siehe 5.5.3). | Banner grün; mindestens die ADS-B-Tracks aus 5.4 sollten wieder erscheinen. |
| 5.6.4 | In der **Sidebar** links die Sektion **„Layer"** öffnen und unten die Legende **„Spurherkunft"** ansehen. | Die Legende ist **dynamisch** (Issue #107): sie zeigt **nur** die Herkünfte, die die abonnierten Feeds liefern. Beim kombinierten Feed also **ADS-B** (Symbol **`A`**) **und FLARM** (Symbol **`F`**) — die beiden sind jetzt eindeutig unterscheidbar (Issues #118/#119); beim reinen ADS-B-Feed nur `A`, beim reinen FLARM-Feed nur `F`. | Legende zeigt genau die zum Feed passenden Einträge; FLARM-Tracks tragen auf der Karte ein **`F`**, ADS-B ein **`A`** (kein Reload nötig). |
| 5.6.5 | Zurück im Admin-Fenster: im Mandanten → **„Feed-Zuweisungen"** → bei `frankfurt-kombiniert` **„Entziehen"**. | Feed ist dem Mandanten nicht mehr zugewiesen. | Status-Chip zeigt wieder **„—"**. |

### 5.7 OpenAIP-Layer aktivieren und prüfen

Ohne einen OpenAIP-API-Schlüssel bleiben die Layer **Lufträume**, **VOR / NDB**
und **Waypoints** **leer**, auch wenn ihre Schalter in der Kartenleiste auf „an"
stehen — das ist erwartungsgemäß und **kein Fehler**. Dieser Schritt setzt den
Schlüssel und prüft, dass die Layer danach Daten zeigen.

| # | Aktion | Erwartetes Ergebnis | ✅ Prüfung |
|---|--------|---------------------|-----------|
| 5.7.1 | Ohne Schlüssel: als `lotse` in der Kartenleiste die Layer **Lufträume**, **VOR / NDB**, **Waypoints** einschalten. | Schalter stehen auf „an", aber es erscheinen **keine** Symbole/Flächen. | Karte bleibt in diesen Layern leer — das ist der erwartete Ausgangszustand ohne Schlüssel. |
| 5.7.2 | Schlüssel setzen — **Option A, global:** in der VM in `~/asd/wayfinder/docker-compose.orchestrated.yml` beim Service `wayfinder` unter `environment:` die Zeile `WAYFINDER_OPENAIP_API_KEY: "〈dein-schlüssel〉"` ergänzen, dann `docker compose -f docker-compose.orchestrated.yml up -d wayfinder`. **Option B, pro Mandant:** als Admin im Mandanten **Demo Frankfurt** im Abschnitt **„OpenAIP-Konfiguration"** den Schlüssel eintragen und **„Schlüssel speichern"**. | Bei Option A: Container `wayfinder` neu gestartet mit gesetztem Schlüssel. Bei Option B: Chip zeigt „Eigener Schlüssel: gesetzt". | Option A: `docker compose -f docker-compose.orchestrated.yml ps` zeigt `wayfinder` wieder `Up`. Option B: Chip „gesetzt" sichtbar, kein Neustart nötig. |
| 5.7.3 | Als `lotse` die Karte neu laden (bzw. bei Option B reicht Warten — die Änderung greift ohne Neustart). | Layer **Lufträume**, **VOR / NDB** und **Waypoints** zeigen jetzt Daten im Frankfurt-Raum. | Sichtbare Luftraum-Flächen und/oder Navaid-/Waypoint-Symbole im Kartenausschnitt. |

> **Zwei Wege, eine Wirkung.** Der pro-Mandant-Schlüssel überschreibt den
> globalen nur für diesen Mandanten; ohne eigenen Schlüssel greift der globale
> als Rückfall. Für die Abnahme reicht **einer** der beiden Wege.

---

## Teil 6 — Hinter den Kulissen prüfen

Rein zur **Bestätigung**, dass die UI-Konfiguration real wirkt. Diese Befehle
laufen **in der VM** (`multipass shell asd`, dann `cd ~/asd/wayfinder`). `〈id〉` ist
die Feed-ID aus dem Container-Namen in 6.1; die Beispiele gehen von einem
aktuell zugewiesenen Feed aus einem der Prüfschritte 5.4–5.6 aus.

| # | Prüf-Befehl | Erwartetes Ergebnis |
|---|-------------|---------------------|
| 6.1 | `docker ps --filter label=wayfinder.feed_id --format '{{.Names}}'` | Genau ein Name der Form **`wayfinder-firefly-feed-〈id〉`** (vom Orchestrator gespawnt). |
| 6.2 | `docker inspect wayfinder-firefly-feed-〈id〉 --format '{{json .Config.Env}}'` | Enthält `FIREFLY_CAT062_GROUP=239.255.0.x` und `FIREFLY_CAT062_PORT=8600` (**die Feed-Adresse aus 5.3**) sowie `FIREFLY_SOURCES` mit der konfigurierten Quelle (ADS-B und/oder FLARM). |
| 6.3 | `docker logs wayfinder-firefly-feed-〈id〉 2>&1 \| grep -i cat062` | Zeile **`CAT062 multicast feed enabled`** mit Ziel **`239.255.0.x:8600`**. |
| 6.4 | `curl -s localhost:8080/metrics \| grep cat062` | `wayfinder_cat062_blocks_received_total` **und** `wayfinder_cat062_tracks_received_total` sind **> 0**, sobald realer Verkehr eingetroffen ist (siehe 5.4/5.5/5.6 — bei ADS-B typischerweise schnell, bei FLARM ggf. verzögert). |
| 6.5 | `curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/ready` | **`200`** (Feed aktiv — mindestens ein CAT065-Heartbeat empfangen; das gilt bereits ohne Tracks). |
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
| **Skript (Teil 4, optional): `✗ FAIL: Firefly image 'firefly:latest' not found`** | Teil 3.2 nicht gemacht. | `cd ~/asd/firefly && docker build -t firefly:latest .`, dann Teil 4 erneut. |
| **Skript (Teil 4, optional): checkpoint 5 schlägt fehl (kein Heartbeat)** | Multicast überquert den Host nicht. | Läuft die VM als **echter** Linux-Host (ja bei Multipass)? `docker logs wayfinder-firefly-feed-〈id〉` prüfen: erscheint `CAT062 multicast feed enabled`? |
| **UI (Teil 5): Karte bleibt leer** | Feed nicht zugewiesen (5.4.1/5.5.1/5.6.1), Sicht-AOI zu klein (Tracks außerhalb), oder gerade kein echter Verkehr im Gebiet (bei FLARM normal, siehe 5.5.3). | Zuweisung prüfen (Status „zugewiesen"); Radius in 5.3.1 auf `100` NM setzen; bei ADS-B länger warten (dichter Verkehr, sollte kommen); bei FLARM ist eine leere Karte **kein Fehler**, solange der Feed-Banner grün ist. |
| **UI (Teil 5.7): Lufträume/VOR-NDB/Waypoints bleiben leer** | Kein OpenAIP-Schlüssel gesetzt (weder global noch pro Mandant). | Schritt 5.7.2 ausführen (`WAYFINDER_OPENAIP_API_KEY` global oder „OpenAIP-Konfiguration" pro Mandant); danach Karte neu laden. |
| **`docker compose … up` bricht mit Build-Fehler ab** | Zu wenig RAM/Disk oder Netzwerkabbruch beim ersten Abhängigkeits-Download. | VM größer neu anlegen: `multipass delete asd --purge` und `multipass launch … --memory 8G --disk 40G` erneut. |
| **`db` wird nicht `healthy`** | Datenbank braucht ein paar Sekunden. | 10 s warten, `docker compose -f docker-compose.orchestrated.yml ps` erneut; bleibt es `unhealthy`: `docker compose -f docker-compose.orchestrated.yml logs db`. |
| **`apt`-Fehler „Release file is not valid yet"** | VM-Uhr nachgelaufen. | In der VM `sudo timedatectl set-ntp true` bzw. auf dem Mac `multipass restart asd`. |

---

## Anhang A — Schnell-Check ohne VM (entfallen → Codespaces)

Der frühere VM-lose Bridge-Weg (`docker-compose.bridge.yml` mit festem
Firefly-Sender und Demo-Szene) ist mit dem Ausbau des Szenen-Modus entfallen
(Firefly ADR 0030). Der Weg ohne eigene Linux-VM ist heute **GitHub
Codespaces**: derselbe orchestrierte Stack wie in diesem Runbook, komplett im
Browser, inklusive Auto-Spawn je Feed — Anleitung in `docs/CODESPACES.md`.
