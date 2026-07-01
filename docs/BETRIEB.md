# Wayfinder — Betriebsführungshandbuch

> **Für wen ist dieses Handbuch?** Für alle, die ein **laufendes** Wayfinder-System
> betreuen — auch ohne tiefe IT-Vorkenntnisse. Es ist ein **Runbook**: für jede
> Betriebsaufgabe gibt es eine klare Schritt-für-Schritt-Anleitung mit Befehlen
> zum **Kopieren und Einfügen**.

> **Abgrenzung:**
> - **Aufsetzen/Installieren** → `docs/INSTALLATION.md` (Schritt für Schritt).
> - **Architektur/Referenz** (wie es intern funktioniert) → `docs/TECHNICAL.md`.
> - **Tagesbetrieb** (dieses Handbuch) → kontrollieren, pflegen, sichern, stören­
>   freie Hilfe.

> 🧭 **Annahme:** Die Beispiele gehen vom **Multi-Tenant-Aufbau** aus
> `INSTALLATION.md` Teil 4 aus (Verzeichnis `~/asd/start-plattform`, Compose-
> Dienste `db`, `firefly`, `wayfinder`). Multi-Tenant ist der einzige
> Betriebsmodus (ADR 0014).

---

## Inhaltsverzeichnis

1. [Das System auf einen Blick](#1-das-system-auf-einen-blick)
2. [Tägliche Kontrolle — die 60-Sekunden-Ampel](#2-tägliche-kontrolle--die-60-sekunden-ampel)
3. [Überwachung & Kennzahlen (Metriken) in Klartext](#3-überwachung--kennzahlen-metriken-in-klartext)
4. [Logbuch & Audit-Spur lesen](#4-logbuch--audit-spur-lesen)
5. [Routine-Aufgaben: Mandanten, Nutzer, Feeds, Sichten](#5-routine-aufgaben-mandanten-nutzer-feeds-sichten)
6. [Sicherheits-Betrieb](#6-sicherheits-betrieb)
7. [Datensicherung & Wiederherstellung](#7-datensicherung--wiederherstellung)
8. [Aktualisieren & Zurückrollen](#8-aktualisieren--zurückrollen)
9. [Starten, Stoppen, Skalieren](#9-starten-stoppen-skalieren)
10. [Störungsbehebung — Runbook](#10-störungsbehebung--runbook)
11. [Notfälle](#11-notfälle)
12. [Betriebs-Checklisten](#12-betriebs-checklisten)

---

## 1. Das System auf einen Blick

Wayfinder ist das **Lagebild** (ASD). Es bekommt Flugzeug-Daten von **Firefly**
(Datenquelle) über das Netz und zeigt sie im Browser. Im Multi-Tenant-Betrieb
kommt eine **PostgreSQL-Datenbank** für Mandanten, Nutzer, Feeds und Rechte dazu.

**Adressen, die im Betrieb zählen:**

| Adresse | Wofür | Wer schaut hin |
|---------|-------|----------------|
| `http://<host>:8081` | Das Lagebild (Browser) | Lotsen / Nutzer |
| `http://<host>:8081/admin` | Verwaltungsoberfläche | Admins |
| `http://<host>:8080/health` | „Lebt der Dienst?" | Betrieb / Monitoring |
| `http://<host>:8080/ready` | „Kommen Daten an?" | Betrieb / Load-Balancer |
| `http://<host>:8080/metrics` | Kennzahlen (Prometheus) | Monitoring |

> 💡 **Merksatz:** **8081 = sehen** (Browser/Admin), **8080 = Technik**
> (Gesundheit/Kennzahlen). Die 8080 ist bewusst **ohne Login** — sie darf nur im
> internen Netz erreichbar sein, nie öffentlich.

**Wichtig:** Wayfinder hält **keinen** dauerhaften eigenen Zustand auf der
Festplatte. Alles, was bewahrt werden muss, liegt in **PostgreSQL**. Das macht
den Dienst selbst jederzeit neustart- und ersetzbar — gesichert wird die
**Datenbank** (Abschnitt 7).

---

## 2. Tägliche Kontrolle — die 60-Sekunden-Ampel

Diese drei Prüfungen sagen, ob alles in Ordnung ist. Im Terminal auf dem Host:

### 🟢 Prüfung 1 — Lebt der Dienst? (Liveness)
```bash
curl -s http://localhost:8080/health
```
- **`ok`** → Dienst läuft. ✅
- **Keine Antwort / Fehler** → Dienst tot oder nicht erreichbar → [Abschnitt 11,
  „Backend reagiert nicht"](#11-notfälle).

### 🟢 Prüfung 2 — Kommen Daten an? (Readiness)
```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/ready
```
- **`200`** → Es kommt ein lebendiger Feed an. ✅
- **`503`** → Es kam noch **kein** Lebenszeichen (CAT065-Heartbeat) oder der Feed
  ist **gerade ausgefallen**. Kurz nach dem Start normal; bleibt es 503 →
  [Runbook „Karte bleibt leer / 503"](#10-störungsbehebung--runbook).

### 🟢 Prüfung 3 — Sieht der Nutzer etwas? (Browser)
**<http://localhost:8081>** öffnen. Erwartung: dunkle Karte, Flugzeuge, oben
rechts der **grüne Banner „FEED OK"**. Steht dort:
- **„SENSOR AUSFALL"** (gelb) → Heartbeat kommt noch an, aber mind. ein Radar ist abgefallen;
  Betrieb eingeschränkt (⏳ erst nach Firefly #32 / CAT063 aktivierbar).
- **„FEED STALE"** (rot) → Datenquelle stumm → Firefly bzw. Multicast prüfen (Runbook).

> ✅ **Alle drei grün?** Das System ist gesund. Diese Kontrolle gehört in die
> tägliche Routine (Abschnitt 12) und lässt sich leicht automatisieren
> (Monitoring fragt einfach `/health` und `/ready` ab).

---

## 3. Überwachung & Kennzahlen (Metriken) in Klartext

`http://localhost:8080/metrics` liefert Zahlen im Prometheus-Format. Man kann sie
direkt ansehen oder von einem Monitoring-System (Prometheus/Grafana) abholen
lassen. Die **wichtigsten** Kennzahlen und was sie bedeuten:

| Kennzahl | Bedeutung | Worauf achten |
|----------|-----------|---------------|
| `wayfinder_feed_stale` | `1` = Feed gerade stumm, `0` = frisch | **Alarm bei `1`** über längere Zeit |
| `wayfinder_cat062_blocks_received_total` | empfangene Track-Datenblöcke (zählt hoch) | **steigt** im Normalbetrieb; bleibt stehen = kein Empfang |
| `wayfinder_cat065_heartbeats_received_total` | empfangene Lebenszeichen | steigt ~1×/s; Stillstand = Quelle weg |
| `wayfinder_cat062_decode_errors_total` | fehlerhafte Datenblöcke | dauerhaft steigend = Daten-/Versionsproblem |
| `wayfinder_ws_clients_connected` | aktuell verbundene Browser | Plausibilität (Lotsenzahl) |
| `wayfinder_ws_clients_evicted_total` | wegen Überlast getrennte Clients | **springt hoch** = langsame Clients/Netzproblem |
| `wayfinder_tenant_ws_clients_connected{tenant="…"}` | verbundene Clients **pro Mandant** | Abrechnung/SLA je Kunde |
| `wayfinder_tenant_tracks_delivered_total{tenant="…"}` | zugestellte Tracks **pro Mandant** | Abrechnung/SLA je Kunde |
| `wayfinder_impersonation_sessions_total` | gestartete Read-Only-Einblicke (Support) | Plausibilität; bewusst **aus den Pro-Kunde-Zahlen ausgenommen** |
| `wayfinder_feature_check_failclosed_total{reason}` | „im Zweifel gesperrt"-Ereignisse | dauerhaft steigend = Konfig-/DB-Problem |

**Empfohlene Alarme (Minimal-Set):**
- `/health` antwortet nicht → **kritisch**.
- `/ready` ≠ 200 länger als ~1 Minute → **kritisch** (kein Feed).
- `wayfinder_feed_stale == 1` länger als ~30 s → **Warnung**.
- `wayfinder_ws_clients_evicted_total` steigt schnell → **Warnung** (Überlast).

Einzelne Zahl schnell ansehen, z. B.:
```bash
curl -s http://localhost:8080/metrics | grep wayfinder_feed_stale
# wayfinder_feed_stale 0
```

---

## 4. Logbuch & Audit-Spur lesen

Wayfinder schreibt **strukturierte Logs als JSON** auf die Standard-Fehlerausgabe
— im Docker-Betrieb sichtbar mit:
```bash
docker compose logs -f wayfinder     # live mitlaufen; Strg+C beendet nur die Anzeige
docker compose logs --since 1h wayfinder   # letzte Stunde
```

**Die Audit-Spur** (Compliance-Nachweis „wer sah welche Lage") trägt das Feld
`"component":"audit"`. Wichtige Ereignisse (`event`):

| `event` | Bedeutung |
|---------|-----------|
| `ws_connect` | Ein Nutzer hat das Lagebild geöffnet — mit Mandant, Nutzer und freigeschaltetem Umfang (Feeds, Gebiet, Flugflächen-Band) |
| `impersonation_start` | Ein `super_admin` hat einen **Read-Only-Einblick** in einen Mandanten gestartet (`impersonated_tenant_id`) |
| `impersonation_end` | …und wieder beendet |
| `impersonation_denied` | **Abgewiesener** Einblicks-Versuch (Nicht-`super_admin` mit Grant, oder unbekannter Ziel-Mandant) — **sicherheitsrelevant, prüfen!** |

Beispiel — alle Impersonation-Ereignisse der letzten Tage heraussuchen:
```bash
docker compose logs --since 72h wayfinder | grep impersonation
```

> 📌 **Aufbewahrung:** Die Logs leben im Container und sind flüchtig. Für
> Revision/Compliance die Logs in eine **externe Senke** leiten (z. B. Loki,
> ELK, CloudWatch, Datadog) — das ist 12-Factor-konform vorgesehen und in
> `docs/TECHNICAL.md` (§9 Logging) beschrieben. Es gibt **bewusst keine**
> Audit-Tabelle in der Datenbank.

---

## 5. Routine-Aufgaben: Mandanten, Nutzer, Feeds, Sichten

Alle Befehle laufen im Verzeichnis `~/asd/start-plattform`. Die
`docker compose run --rm wayfinder …`-Befehle starten die Datenbank bei Bedarf
automatisch mit.

### 5.1 Neuen Mandanten + Admin anlegen
```bash
WAYFINDER_BOOTSTRAP_PASSWORD='StartPasswort!' \
docker compose run --rm -e WAYFINDER_BOOTSTRAP_PASSWORD \
  wayfinder bootstrap \
    -tenant kunde-sued -tenant-name "Kunde Süd GmbH" \
    -subject ben -role tenant_admin
```
- `-role`: `operator` (nur sehen), `tenant_admin` (eigenen Mandanten verwalten),
  `super_admin` (alles verwalten). Der Befehl ist **idempotent** — erneut
  ausführen setzt nur das Passwort neu.

### 5.2 Weiteren Nutzer in einem bestehenden Mandanten anlegen
Gleicher Befehl, gleicher `-tenant`, anderer `-subject`/`-role`.

### 5.3 Feed in den Katalog aufnehmen / ansehen
```bash
docker compose run --rm wayfinder feed add \
  -name "München" -group 239.255.0.70 -port 8600 -sensor-mix PSR,SSR,ADS-B
docker compose run --rm wayfinder feed list
```
- `-sensor-mix` nur aus `PSR, SSR, MODE_S, ADS-B, MLAT, FLARM` (gängige
  Schreibweisen werden korrigiert; **Unbekanntes wird abgelehnt**).

### 5.4 Feed einem Mandanten zuweisen / entziehen (nur `super_admin`)
**Am einfachsten** über die Oberfläche **/admin** → Provisioning-Bereich.
Alternativ per Befehl (als angemeldeter `super_admin`; `{tenant}`/`feed_id` aus
`feed list` bzw. der Admin-Liste):
```bash
# zuweisen
curl -X POST http://localhost:8081/api/admin/tenants/2/subscriptions \
  -H 'Content-Type: application/json' -d '{"feed_id":1}'
# entziehen
curl -X DELETE http://localhost:8081/api/admin/tenants/2/subscriptions/1
```
- **Wirkt sofort, ohne Neuanmeldung** (Live-Apply, WF2-33): Beim Entzug
  verschwinden die Tracks des Feeds umgehend von der Karte des Kunden.
- **Regel:** Ein Mandant **ohne** `multi_feed`-Recht darf höchstens **einen** Feed
  halten — ein zweiter Zuweisungsversuch wird mit **409** abgelehnt (siehe 5.6).

### 5.5 Sicht eines Mandanten einstellen (Zentrum/Gebiet/Flugflächen)
Der jeweilige `tenant_admin` setzt seine Sicht (Kartenzentrum, Interessengebiet
**AOI**, Flugflächen-Band) in **/admin**. Das begrenzt **serverseitig**, was
überhaupt an dessen Browser geht (Datensparsamkeit/Bandbreite). Ohne Eintrag
sieht der Mandant den ganzen abonnierten Feed.

### 5.6 Funktions-Freischaltungen (Entitlements) setzen (nur `super_admin`)
Feature-Flags als Daten (z. B. `multi_feed`, `stca`, `premium_layers`):
```bash
curl -X PUT http://localhost:8081/api/admin/tenants/2/entitlements/multi_feed \
  -H 'Content-Type: application/json' -d '{"enabled":true}'
```
Erst danach darf Mandant 2 mehr als einen Feed halten.

### 5.7 Read-Only-Einblick beaufsichtigen
Der „View as Tenant"-Einblick (Bedienung: `INSTALLATION.md` Schritt 5.11) ist ein
**Support-Werkzeug für `super_admin`**. Betrieblich gilt: Jeder Einblick steht im
**Audit-Log** (`impersonation_start`/`_end`), läuft nach 30 min ab und ist
**read-only**. Abgewiesene Versuche (`impersonation_denied`) regelmäßig prüfen
(Abschnitt 6.5).

---

## 6. Sicherheits-Betrieb

### 6.1 Anmelde-Modus (Auth)
- **`builtin`** — eingebaute Nutzer + Passwörter (Standard, Multi-Tenant mit
  PostgreSQL; `INSTALLATION.md` Teil 4). Passwort setzen/zurücksetzen =
  `bootstrap` erneut ausführen (Schritt 4.4).
- **`proxy`** — Anmeldung über einen vorgelagerten OIDC-Proxy (Unternehmens-SSO);
  Wayfinder validiert nur das durchgereichte Token. Kein lokales Passwort.

Einen Betrieb **ohne** Anmeldung gibt es nicht mehr (ADR 0014): Auth ist immer
aktiv, `WAYFINDER_DB_URL` ist Pflicht.

### 6.2 Der Signing-Key — und die Not-Bremse „alle abmelden"
`WAYFINDER_SESSION_KEY` signiert die Login-Cookies **und** die
Impersonation-Grants. Praktische Folge:

> 🔑 **Schlüssel wechseln = sofort alle Sitzungen und alle Read-Only-Einblicke
> ungültig.** Bei Verdacht auf Missbrauch eines gestohlenen Cookies ist das die
> schnellste Notbremse: neuen Schlüssel in der Compose-/Deployment-Konfiguration setzen und
> `docker compose up -d` — alle müssen sich neu anmelden.

Neuen Schlüssel erzeugen:
```bash
openssl rand -hex 32
```
Den Schlüssel wie ein Passwort behandeln (nie ins Git, nur als Secret/ENV).

### 6.2a Einzelne Sitzung sperren + Session-Limit (AP7)
Seit AP7 kennt der Server jede aktive Sitzung (serverseitige Registry). Für den
Alltag heißt das — **ohne** den globalen Not-Bremsen-Schlüsselwechsel aus 6.2:

- **Zugang/Mandant pausieren wirkt sofort.** Pausierst du einen Zugang
  (Zugangs-Verwaltung → Pausieren) oder einen ganzen Mandanten, fliegen dessen
  **laufende** Sitzungen beim nächsten Request raus — nicht erst beim
  Cookie-Ablauf. Reaktivieren hebt die Sperre auf (bereits abgemeldete Konsolen
  müssen sich neu anmelden). Löschen wirkt genauso.
- **Abmelden ist echt.** Ein Logout löscht die Sitzung serverseitig, nicht nur im
  Browser.
- **Limit gleichzeitiger Sitzungen je Zugang** (opt-in, Default **aus**):
  `WAYFINDER_SESSION_LIMIT_DEFAULT=N` begrenzt parallele Logins pro Zugang;
  `WAYFINDER_SESSION_LIMIT_POLICY` steuert das Verhalten am Limit —
  `reject` (Default: der N+1-te Login wird mit „Sitzungslimit erreicht"/`429`
  abgewiesen) oder `evict_oldest` (die älteste Sitzung wird verdrängt). Ein
  einzelner Zugang kann per `users.session_limit` ein eigenes Limit tragen.
- **Rollout-Hinweis:** Nach dem Update auf AP7 laufen offene Browser noch kurz mit
  ihrem alten Cookie weiter und werden beim nächsten automatischen „Renew" (alle
  10 min) in die Registry überführt. Wer **sofort** eine saubere Basis will, nutzt
  die Not-Bremse aus 6.2 (Schlüsselwechsel) — dann meldet sich jeder einmalig neu
  an, direkt in eine registrierte Sitzung.
- **Kennzahlen:** `wayfinder_active_sessions` (aktuelle Sitzungen),
  `wayfinder_session_logins_rejected_total` (am Limit abgewiesen),
  `wayfinder_sessions_revoked_total` (durch Pause/Löschen beendet).

### 6.3 TLS & Herkunfts-Prüfung am Browser-Rand
- **TLS** im Produktivbetrieb **immer** — primär am vorgelagerten Reverse-Proxy/
  Ingress; alternativ direkt in Wayfinder (`WAYFINDER_TLS_CERT`/`_KEY`).
- **Origin-Allowlist** `WAYFINDER_ALLOWED_ORIGINS` auf die echte ASD-Domain
  setzen (Same-Origin ist immer erlaubt).

### 6.4 Netz-Isolation des Feeds
Der CAT062/065-Multicast hat **keine** eingebaute Authentifizierung. Die
Vertrauensgrenze liegt auf der **Netzwerk-Schicht**: Firefly-Sender und Wayfinder-
Empfänger gehören in ein **abgeschottetes Segment/VLAN**, in dem sonst niemand
sendet. Die `8080`-Technik-Schnittstelle gehört ebenfalls nur ins interne Netz.

### 6.5 Wiederkehrende Sicherheits-Kontrolle
```bash
# Abgewiesene Einblicks-Versuche (sollte normalerweise leer sein):
docker compose logs --since 168h wayfinder | grep impersonation_denied
# Wer hat in der letzten Woche welche Mandanten eingesehen?
docker compose logs --since 168h wayfinder | grep impersonation_start
```
Auffälligkeiten (häufige `_denied`, unerwartete `_start`) eskalieren.

---

## 7. Datensicherung & Wiederherstellung

**Was muss gesichert werden?** Nur die **PostgreSQL-Datenbank** (Mandanten,
Nutzer, Feeds, Abos, Sichten, Rechte, Passwort-Hashes). Wayfinder selbst hält
keinen sicherungswürdigen Zustand. Die `wayfinder.yaml` und die
Deployment-Konfiguration (`docker-compose.*.yml` mit `WAYFINDER_DB_URL`,
`WAYFINDER_AUTH_MODE`, `WAYFINDER_SESSION_KEY` u. a.) gehören **verschlüsselt** in
die Konfig-Sicherung/Versionskontrolle oder ein Secret-Management (K8s Secrets, Vault).

### 7.1 Sicherung anlegen (Backup)
```bash
docker compose exec -T db pg_dump -U wayfinder wayfinder > wayfinder-$(date +%F).sql
```
Das schreibt eine datierte Datei (z. B. `wayfinder-2026-06-24.sql`). **Regelmäßig**
ausführen (z. B. täglich per Cron) und die Dateien **außerhalb** des Hosts
aufbewahren.

### 7.2 Wiederherstellung (Restore)
```bash
# 1. Wayfinder anhalten, Datenbank weiterlaufen lassen
docker compose stop wayfinder
# 2. Sicherung einspielen
docker compose exec -T db psql -U wayfinder wayfinder < wayfinder-2026-06-24.sql
# 3. Wayfinder wieder starten
docker compose up -d wayfinder
```

### 7.3 Sicherung testen
Eine Sicherung, die nie zurückgespielt wurde, ist keine Sicherung. **Mindestens
quartalsweise** einen Restore auf einem Testsystem proben.

---

## 8. Aktualisieren & Zurückrollen

### 8.1 Neue Version einspielen
```bash
cd ~/asd/wayfinder
git pull                       # neue Programmstände holen
cd ~/asd/start-plattform
docker compose up -d --build   # neu bauen und starten
```
- **Datenbank-Migrationen** werden beim Start **automatisch** angewandt (je
  Migration eine Transaktion, idempotent). **Vor** einem Update mit Schema-
  Änderungen immer erst **sichern** (Abschnitt 7).
- Das Frontend-Bundle (`internal/webui/dist/`) ist eingecheckt — ein separater
  Frontend-Build ist nur bei eigenen Frontend-Änderungen nötig.

### 8.2 Zurückrollen (Rollback)
```bash
cd ~/asd/wayfinder
git checkout <vorheriger-stand>    # z. B. ein Tag oder Commit
cd ~/asd/start-plattform
docker compose up -d --build
```
> ⚠️ Wurde beim Update eine **Schema-Migration** angewandt, kann ein reiner Code-
> Rollback nicht reichen — dann zusätzlich die **Datenbank-Sicherung** von vor dem
> Update einspielen (7.2). Darum: vor jedem Update sichern.

---

## 9. Starten, Stoppen, Skalieren

### 9.1 Start / Stopp / Status
```bash
docker compose up -d           # starten (Hintergrund)
docker compose ps              # Status aller Dienste
docker compose stop            # anhalten (Daten bleiben)
docker compose down            # anhalten + Container entfernen (DB-Volume bleibt)
```
> ⚠️ **`docker compose down -v` löscht das Datenbank-Volume** — also **alle**
> Mandanten/Nutzer/Feeds. Nur mit aktueller Sicherung und mit Bedacht.

### 9.2 Sauberes Herunterfahren
Wayfinder reagiert auf das Stopp-Signal und schließt alle Verbindungen sauber;
`docker compose stop` genügt. Kein „hartes" Killen nötig.

### 9.3 Mehr Last / Hochverfügbarkeit
- Wayfinder ist **zustandslos** und kann **mehrfach** parallel laufen (mehrere
  Replicas hinter einem Load-Balancer), solange alle dieselbe Datenbank nutzen.
- **Aber:** Jede Replica muss den **Multicast-Feed empfangen** können — in
  Cloud-Netzen ist Multicast meist blockiert (Details + Sidecar-Muster:
  `docs/TECHNICAL.md`, `docs/INSTALLATION.md` Teil 8).
- Die **Datenbank** ist die zu schützende Komponente — hier auf Hochverfügbarkeit
  und Sicherung achten.

---

## 10. Störungsbehebung — Runbook

| Symptom | Wahrscheinliche Ursache | Maßnahme |
|---------|-------------------------|----------|
| **Karte bleibt leer, keine Flugzeuge** | Firefly läuft nicht / Multicast kommt nicht an | `docker compose ps` (laufen alle?); `docker compose logs firefly`; Gruppe/Port bei **beiden** Diensten gleich? Gemeinsames Netz (kein Aufteilen auf zwei Compose-Dateien)? |
| **`/ready` bleibt 503** | Kein Lebenszeichen vom Feed | Sekunden nach Start normal; sonst wie „Karte leer" |
| **Banner „FEED STALE"** | Quelle gerade stumm | Firefly/Sensorlage prüfen; `wayfinder_feed_stale` im Monitoring |
| **Nutzer sieht nichts (Multi-Tenant)** | Mandant hat **keinen Feed** zugewiesen (gewollt fail-closed) | Zuweisung nachholen (5.4) |
| **Login schlägt fehl (401)** | Passwort falsch **oder** `WAYFINDER_SESSION_KEY` fehlt/zu kurz | Key setzen (6.2), Container neu starten, ggf. `bootstrap` erneut |
| **„Als Mandant ansehen" fehlt** | Nutzer ist nicht `super_admin` **oder** kein Signing-Key gesetzt | Rolle prüfen; `WAYFINDER_SESSION_KEY` setzen |
| **Viele `clients_evicted`** | Langsame Clients / Netzengpass | Netz/Client-Last prüfen; ist erwartetes Schutzverhalten (kein Absturz) |
| **`feature_check_failclosed` steigt** | DB-Problem oder unbekannter Feature-Key | DB-Erreichbarkeit prüfen; Logs ansehen |
| **Port belegt (`address already in use`)** | Anderer Dienst nutzt 8080/8081 | Port-Abbildung in der Compose ändern (z. B. `9091:8081`) |

**Allgemeine Erstdiagnose immer:**
```bash
docker compose ps                       # läuft alles?
docker compose logs --since 15m wayfinder   # was sagen die Logs?
curl -s http://localhost:8080/health        # lebt der Dienst?
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/ready
```

---

## 11. Notfälle

### Backend reagiert nicht (`/health` stumm)
1. `docker compose ps` — Status von `wayfinder`.
2. `docker compose logs --since 15m wayfinder` — letzte Fehlermeldung.
3. Neustart: `docker compose restart wayfinder`.
4. Hilft das nicht: Logs sichern und mit der Versionshistorie abgleichen; ggf.
   auf den letzten funktionierenden Stand zurückrollen (Abschnitt 8.2).

### Datenbank weg / nicht erreichbar
1. `docker compose ps` / `docker compose logs db`.
2. `db` neu starten: `docker compose restart db`; danach `wayfinder` neu starten.
3. Datenbestand beschädigt → Wiederherstellung aus Sicherung (7.2).

### Sicherheitsvorfall (z. B. verdächtige Einblicke, geleaktes Cookie)
1. **Sofort** den Signing-Key wechseln → alle Sitzungen/Einblicke ungültig (6.2).
2. Audit-Log sichern und auswerten (`impersonation_*`, `ws_connect`).
3. Betroffene Nutzer-Passwörter zurücksetzen (5.1); ggf. Rollen prüfen.

### „Alles kaputt" — kontrolliert neu aufsetzen
Mit **aktueller Sicherung**: `docker compose down` (ohne `-v`!), Ursache beheben,
`docker compose up -d --build`. Erst als letztes Mittel und nur mit Backup das
Volume zurücksetzen (`down -v`) und die Sicherung einspielen.

---

## 12. Betriebs-Checklisten

**Täglich**
- [ ] 60-Sekunden-Ampel grün (Abschnitt 2): `/health` = ok, `/ready` = 200,
      Browser zeigt „FEED OK".
- [ ] Monitoring ohne offene Alarme (`feed_stale`, evicted).

**Wöchentlich**
- [ ] Audit-Log auf `impersonation_denied` und unerwartete `impersonation_start`
      durchsehen (6.5).
- [ ] Datenbank-Sicherung vorhanden und aktuell (Abschnitt 7).
- [ ] Plattenplatz/Log-Größe auf dem Host prüfen.

**Monatlich**
- [ ] Restore-Probe auf einem Testsystem (7.3).
- [ ] Verfügbare Updates sichten; Update mit vorheriger Sicherung einspielen (8).
- [ ] Nutzer-/Rollen-Inventur (wer hat `super_admin`?).

**Bei jeder Konfig-Änderung**
- [ ] Vorher Datenbank sichern.
- [ ] `docs/INSTALLATION.md` (Env-Variablen) und dieses Handbuch auf Aktualität
      prüfen.
- [ ] Nach der Änderung die 60-Sekunden-Ampel wiederholen.

---

> **Weiterführend:** `docs/INSTALLATION.md` (Aufsetzen), `docs/TECHNICAL.md`
> (Architektur, vollständige Metrik-/Env-Referenz, Sicherheitsmodell),
> `docs/glossary.md` (Fachbegriffe), `docs/decisions/` (Architektur-
> Entscheidungen, u. a. ADR 0008 zum Read-Only-Einblick).
