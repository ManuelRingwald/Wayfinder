# End-to-End-Abnahme: ein Befehl starten, alles Weitere per UI

> **Philosophie.** **Genau ein** Terminal-Befehl ist nötig — der **Start** der
> Plattform. Alles danach passiert **ausschließlich in der Browser-Oberfläche**:
> Passwortwechsel, Mandant/Nutzer/Features, Feed + Quellen, Feed zuweisen,
> Abmelden als Admin, Anmelden als Mandant, Karte sehen. Das Terminal wird **nur
> noch zum Nachschauen hinter den Kulissen** verwendet (Logs/Metriken: sendet
> Firefly auf der erwarteten Gruppe:Port die richtigen Datentypen ADS-B / FLARM?).
>
> **Betriebsmodus.** Multi-Tenant ist der einzige Modus (ADR 0014): Postgres ist
> Pflicht, die Anmeldung ist immer aktiv.

## Was hier nachgewiesen wird

| # | Behauptung | Teil |
|---|------------|------|
| 1 | Die ganze Kette startet mit **einem** Befehl (Zero-Touch). | 0 |
| 2 | Erstanmeldung + erzwungener Passwortwechsel laufen **in der UI**. | 1 |
| 3 | Ein Kunde wird **komplett über die UI** eingerichtet: Mandant, Features, Nutzer, Feed, Quellen (ADS-B/FLARM), Sicht (Zentrum + 30-NM-Radius), Zuweisung. | 2 |
| 4 | **Abmelden als Admin** und **Anmelden als Mandant** geht in der UI; der Kunde sieht **seine** Karte. | 3 |
| 5 | Hinter den Kulissen: Firefly wurde je Feed gestartet, sendet auf der erwarteten **Gruppe:Port** und trägt **echte ADS-B/FLARM-Daten**. | 4 |

> ⚠️ **Plattform.** Die **Live-Daten-Kette** (Auto-Spawn + Multicast, Teil 0/4)
> braucht **Host-Networking** und damit einen **Linux-Docker-Host**. Auf **Docker
> Desktop (macOS/Windows, z. B. Mac mini)** funktioniert Host-Net-Multicast nicht
> — dort laufen Teil 1–3 (die ganze UI-Einrichtung) identisch, aber für die
> Live-Verifikation aus Teil 4 einen Linux-Host (oder eine Linux-VM) nutzen.
> Hintergrund + Bridge-Workaround: `DOCKER.md`.

## Voraussetzungen

- Linux-Docker-Host mit `docker compose` v2 (für Teil 4 Live-Multicast).
- **Firefly-Image lokal**: im Firefly-Repo `docker build -t firefly:latest .`
  (oder `WAYFINDER_FIREFLY_IMAGE` auf ein veröffentlichtes Tag setzen).
- Netz-Egress, wenn echte ADS-B/FLARM-Daten geholt werden sollen (OpenSky,
  Open Glider Network).

### EDLV-Geodaten (für die Sicht in Teil 2)

EDLV (Weeze) liegt bei **51,40° N / 6,15° E**. Die View-Config-Maske nimmt einen
**Radius in nautischen Meilen (NM)** entgegen und rechnet die AOI selbst aus —
also einfach **Radius = 30** eintragen.

| Feld in der Maske | Wert |
|-------------------|------|
| Zentrum Breite (`center_lat`) | `51.40` |
| Zentrum Länge (`center_lon`) | `6.15` |
| Radius | `30` (NM) |
| Zoom | `9` |
| FL min / FL max | `0` / `450` |

---

## Teil 0 — Der **einzige** Terminal-Befehl: starten

| # | Aktion | Erwartetes Ergebnis | Prüfung |
|---|--------|---------------------|---------|
| 0.1 | Im Wayfinder-Repo: `docker compose -f docker-compose.orchestrated.yml up --build` | Postgres + Wayfinder (`builtin`) + Orchestrator starten; Auto-Seed legt Default-Admin an. | `docker compose -f docker-compose.orchestrated.yml ps` → alle `Up`, `db (healthy)`. |

**Ab hier kein Terminal mehr für Aktionen — nur noch Browser.**

---

## Teil 1 — Erstanmeldung + Passwortwechsel (UI)

| # | UI-Aktion | Erwartetes Ergebnis | Prüfung |
|---|-----------|---------------------|---------|
| 1.1 | Browser: **http://localhost:8081/admin** | Login-Maske „Anmelden" (Benutzername/Passwort). | Maske erscheint. |
| 1.2 | Anmelden mit `admin` / `admin`. | Sofort die Maske **„Passwort ändern"** (erzwungen). | Kein Zugriff auf die Tabs, bevor das Passwort gesetzt ist. |
| 1.3 | Neues Passwort (≥ 8 Zeichen) zweimal eingeben, bestätigen. | Admin-Dashboard mit Tabs **Mandanten / Feeds / Plattform-Administratoren**. | Tabs sind jetzt sichtbar/bedienbar. |

---

## Teil 2 — Ersten Kunden (EDLV) einrichten (UI)

Alles im Admin-Dashboard, keine Terminal-Befehle.

| # | UI-Aktion | Erwartetes Ergebnis | Prüfung |
|---|-----------|---------------------|---------|
| 2.1 | Tab **Mandanten** → **Neuer Mandant**: Slug `edlv`, Name `EDLV Weeze`. | Mandant erscheint in der Liste. | Eintrag „EDLV Weeze" sichtbar. |
| 2.2 | Mandant **EDLV Weeze** öffnen → Karte **Features**: das gewünschte Feature aktivieren (z. B. **`multi_feed`**, wenn der Kunde mehrere Feeds bekommen soll). | Toggle bleibt an (serverseitig gespeichert). | Feature-Toggle steht auf „an". |
| 2.3 | Im Mandanten → Karte **Nutzer** → **Neuer Nutzer**: Subject `edlv-lotse`, Passwort (≥ 8), Rolle Nutzer. | Nutzer erscheint in der Mandanten-Nutzerliste. | Eintrag „edlv-lotse" sichtbar. |
| 2.4 | Tab **Feeds** → **Neuer Feed**: Name `edlv-weeze`, Sensor-Mix `SSR, ADS-B`, **Endpoint automatisch** (Schalter an). | Feed erscheint mit **automatisch** zugewiesener Gruppe/Port. | Feed-Zeile zeigt eine `239.255.0.x:8600`-Adresse. |
| 2.5 | Beim Feed **Quellen** öffnen → **echte Live-Quellen** hinzufügen: <br>• Typ `adsb_opensky`, BBox = EDLV-Gebiet (min/max Lat 50.90/51.90, Lon 5.34/6.96). <br>• Typ `flarm_aprs`, gleiche BBox. <br>Speichern. | Beide Quellen gespeichert; eine `coverage_bbox` wird angezeigt. | Quellen-Liste zeigt `adsb_opensky` **und** `flarm_aprs`. |
| 2.6 | Im Mandanten → **View-Config**: Zentrum `51.40 / 6.15`, **Radius `30`**, Zoom `9`, FL `0`–`450`. Speichern. | Sicht gespeichert (AOI aus Radius berechnet). | Werte stehen nach Reload unverändert da. |
| 2.7 | Im Mandanten → **Feeds/Provisioning**: Feed `edlv-weeze` **zuweisen** (Grant). | Feed ist dem Mandanten zugewiesen. | „Granted"-Status beim Feed. |

> **Credentials (optional).** Anonyme ADS-B/FLARM-Quellen brauchen **keine**
> Zugangsdaten (OpenSky/OGN anonym, rate-limitiert). Für höhere OpenSky-Limits
> (OAuth2-Client-Credentials) den Secret-Dialog nutzen — der ist nur sichtbar,
> wenn der Server mit `WAYFINDER_SECRET_KEY` läuft.

---

## Teil 3 — Abmelden als Admin, anmelden als Mandant (UI)

| # | UI-Aktion | Erwartetes Ergebnis | Prüfung |
|---|-----------|---------------------|---------|
| 3.1 | Im Admin-Header **Abmelden**. | Zurück zur Login-Maske (Sitzung beendet). | Login-Maske erscheint wieder. |
| 3.2 | Browser: **http://localhost:8081/** (die Lage-Karte). | Da keine Sitzung besteht: **Login-Maske** statt leerer Karte. | Login-Maske auf `/`. |
| 3.3 | Anmelden als `edlv-lotse` + Passwort. | Karte lädt, zentriert auf **EDLV** (51,40/6,15, Zoom 9); oben rechts der Konto-Chip `edlv-lotse`. | Kartenausschnitt = Weeze. |
| 3.4 | Warten, bis Tracks erscheinen (Live-Daten aus Teil 2.5). | ADS-B-/FLARM-Tracks im EDLV-Gebiet; der Kunde sieht **nur** seinen gescopten Strom. | Tracks liegen innerhalb der 30-NM-AOI. |

> **Hinweis.** Der Konto-Chip oben rechts bietet **Abmelden** (und für Admins eine
> Verknüpfung zur Administration). So ist der ganze Auth-Zyklus UI-bedienbar.

---

## Teil 4 — Hinter den Kulissen prüfen (Terminal — **nur** Verifikation)

Erst hier wieder das Terminal — ausschließlich, um zu **bestätigen**, dass die
in der UI angelegte Konfiguration real wirkt.

| # | Prüf-Befehl | Erwartetes Ergebnis |
|---|-------------|---------------------|
| 4.1 | `docker ps --filter label=wayfinder.feed_id` | Container **`wayfinder-firefly-feed-<id>`** läuft (vom Orchestrator gespawnt). |
| 4.2 | `docker inspect wayfinder-firefly-feed-<id> --format '{{json .Config.Env}}'` | `FIREFLY_CAT062_GROUP`/`FIREFLY_CAT062_PORT` = die **Feed-Adresse aus 2.4**; `FIREFLY_MODE=live`; `FIREFLY_SOURCES` enthält `adsb_opensky` **und** `flarm_aprs`. |
| 4.3 | `docker logs wayfinder-firefly-feed-<id>` | Zeilen: **`live mode: starting tracker`** (mit `opensky_sources=1`, `flarm_sources=1`), **`CAT062 multicast feed enabled`** + Ziel **`<group>:<port>`**, **`OpenSky ADS-B poller … started`** (+ BBox), **`FLARM/OGN APRS-IS listener started`**. → bestätigt Adresse:Port **und** die Datentypen. |
| 4.4 | `curl -s localhost:8080/metrics \| grep cat062` (Wayfinder) | `wayfinder_cat062_blocks_received_total` und `…_tracks_received_total` **> 0** → Wayfinder empfängt den Strom. |
| 4.5 | Admin-UI → Feeds → **Feed-Gesundheit** (oder `GET /api/admin/feeds/health`) | Feld für den Feed wird **grün**, `ever_seen=true` (CAT065-Heartbeat läuft). |

> Optional, falls Firefly `/metrics` exponiert: `firefly_sources_opensky` und
> `firefly_sources_flarm` = `1` (Quelltypen verdrahtet), `firefly_cat062_scans_sent_total`
> wächst (Multicast wird gesendet).

---

## Anhang E — Bridge-Abnahme auf Docker Desktop (Mac mini / Windows)

Der Haupt-Ablauf oben (Teil 0/4) fährt den **orchestrierten** Stack mit
`network_mode: host`. Auf **Docker Desktop** (Mac mini, Windows) bindet
Host-Networking nur an die interne Linux-VM, nicht an den Rechner — der
Live-Multicast-Pfad kommt dort nicht an. Auf dem Mac mini nimmt man deshalb den
**Bridge-Weg**: Firefly, Postgres und Wayfinder in **einem gemeinsamen
Bridge-Netz** (`docker-compose.bridge.yml`). Multicast zwischen Containern auf
derselben Bridge funktioniert auch unter Docker Desktop — kaputt ist dort nur
Host↔Container.

### Teil E-1 — Was der Bridge-Weg abdeckt (und was nicht)

| Prüf-Baustein | Bridge (Mac mini) | Orchestriert (Linux) |
|---|---|---|
| UI-Einrichtung (Login, Mandant, Nutzer, Feature, Feed, Quellen, View, Zuweisung) — Teil 1–3 | ✅ identisch | ✅ |
| **Live-Tracks auf der Karte** (Multicast Container→Container) | ✅ ja | ✅ |
| Orchestrator-**Auto-Spawn je Feed** + Teardown (Prüfpunkte 1/2/8) | ❌ nein | ✅ |
| `scripts/e2e-orchestrated.sh` | ❌ (braucht Host-Net + Docker-Socket) | ✅ |

Auf dem Bridge-Weg gibt es **keinen Orchestrator**: Firefly ist ein **fester**
externer Sender auf `239.255.0.62:8600`. Der Kern-Unterschied zur UI aus Teil 2
ist deshalb **ein einziger Schritt** (siehe E-2, Schritt E.4): der Feed wird mit
**genau diesem festen Endpoint** angelegt — **nicht** „Endpoint automatisch". Eine
auto-allokierte `239.255.0.x`-Adresse würde ins Leere zeigen (Firefly sendet dort
nicht) und die Karte bliebe leer.

> **Voraussetzung Layout.** Beide Repos als Geschwister-Ordner, z. B. unter
> `~/asd/`:
>
> ```
> ~/asd/
> ├── firefly/     ← Firefly-Repo (der ../firefly-Build-Kontext)
> └── wayfinder/   ← dieses Repo (von hier starten)
> ```

### Teil E-2 — Ablauf

| # | Aktion | Erwartetes Ergebnis | Prüfung |
|---|--------|---------------------|---------|
| E.1 | Firefly-Repo daneben klonen (falls noch nicht): `git clone …/firefly.git` als Geschwister von `wayfinder/`. | `~/asd/firefly` und `~/asd/wayfinder` liegen nebeneinander. | `ls ~/asd` → `firefly wayfinder`. |
| E.2 | Im Wayfinder-Repo: `docker compose -f docker-compose.bridge.yml up --build` | Firefly + Postgres + Wayfinder starten in einem Bridge-Netz; Auto-Seed legt den Default-Admin an. | `docker compose -f docker-compose.bridge.yml ps` → alle `Up`, `db (healthy)`. |
| E.3 | **Teil 1–3 wie oben** durchlaufen (Login `admin`/`admin` + Passwortwechsel, Mandant/Nutzer/Features/View, Abmelden/Anmelden). | Identisch zum Haupt-Ablauf — die UI unterscheidet sich nicht. | Siehe Teil 1–3. |
| E.4 | Tab **Feeds** → **Neuer Feed**: **Endpoint automatisch AUS**, Gruppe **`239.255.0.62`**, Port **`8600`** von Hand eintragen (= Fireflys fester Sende-Endpoint). | Feed erscheint mit genau `239.255.0.62:8600`. | Feed-Zeile zeigt `239.255.0.62:8600` (nicht `239.255.0.x`). |
| E.5 | Feed dem Mandanten **zuweisen** (Grant), dann als Mandant anmelden. | Karte lädt; nach wenigen Sekunden erscheinen die Frankfurt-Tracks. | Tracks sichtbar; oben links **FEED OK** (grün). |
| E.6 | Hinter den Kulissen (Terminal, nur Verifikation): `curl -s localhost:8080/metrics \| grep cat062` | `wayfinder_cat062_blocks_received_total` und `…_tracks_received_total` **> 0**. | Werte wachsen. |

> **Bleibt die Karte leer?** Fast immer ist es E.4 — der Feed wurde
> **auto-allokiert** statt auf `239.255.0.62:8600` gesetzt. Prüfen mit
> `docker compose -f docker-compose.bridge.yml logs firefly` (sendet Firefly auf
> `239.255.0.62:8600`?) und der Feed-Adresse in der Admin-UI.

## Aufräumen

```bash
# Orchestrierter Stack (Linux):
docker compose -f docker-compose.orchestrated.yml down -v --remove-orphans
docker ps -aq --filter 'label=wayfinder.managed=true' | xargs -r docker rm -f

# Bridge-Stack (Mac mini / Windows):
docker compose -f docker-compose.bridge.yml down -v --remove-orphans
```

## Bekannte Grenzen

- **Docker Desktop (macOS/Windows, Mac mini):** Host-Net-Multicast funktioniert
  dort nicht, der **orchestrierte** Stack (Auto-Spawn, Prüfpunkte 1/2/8) gehört
  daher auf einen Linux-Host. **Live-Tracks** sind auf dem Mac mini trotzdem
  möglich — über den **Bridge-Weg** (`docker-compose.bridge.yml`): Firefly +
  Wayfinder im selben Bridge-Netz, fester Feed-Endpoint statt Auto-Spawn. Der
  vollständige Ablauf steht oben in **Anhang E**; Hintergrund in `DOCKER.md`.
- **Sitzungsablauf:** läuft die Mandanten-Sitzung ab (Default 12 h), zeigt die
  Karte den Stand bis zum Reload; ein erneutes Öffnen von `/` führt zur
  Login-Maske. (Inline-Re-Login bei WS-Ablauf ist ein Folge-Schritt.)
- **Diese Repo-CI/Sandbox:** ohne Docker-Daemon nur `docker compose config` /
  Binär-/Frontend-Build verifizierbar; der echte Lauf gehört auf einen Docker-Host.
