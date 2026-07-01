# Arbeitsstand (Handover-Notiz) — Wayfinder

> **Zweck:** Diese Datei beschreibt den **aktuellen IST-Stand** von Wayfinder.
> Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

> 🗺️ **Roadmap & Arbeitspakete:** siehe `docs/ROADMAP.md` in **diesem Repo**
> (zentrale Quelle für Wayfinder **und** Firefly). Cross-Project-Abhängigkeiten
> in `docs/cross-project/todo-for-wayfinder.md`.

---

## 🎯 Stand 2026-07-01

- **Zuletzt aktualisiert:** 2026-07-01
- **Großes Bild:** Das **Prio-1-Go-to-Market-Fundament ist fertig** — ONB
  (Zero-Touch-Onboarding) ✅ und **ORCH (Auto-Orchestrierung) ✅ Kern komplett**
  (1…5c). „Feed zuweisen ⇒ passende Firefly-Instanz startet automatisch" ist
  gebaut, getestet, sicherheits-reviewed und gehärtet. Alles auf `main`,
  alle Gates grün (Go: build/test/vet/gofmt/golangci-lint; Frontend: 180 vitest).

- **Diese Sitzung (2026-06-29/30):** ORCH-5b-1 (Cred-Auflösung in der
  Control-Plane, Variante A) · 5b-2 (UI-Zwei-Felder) · 5c (E2E-Abnahme-Harness:
  `docker-compose.orchestrated.yml` + `Dockerfile.orchestrator` +
  `scripts/e2e-orchestrated.sh` + `docs/E2E-ABNAHME.md`) · UI-Relabel
  Client-ID/Client-Secret (OpenSky OAuth2) · **Konsolidierung** (Sicherheits-Review
  ohne kritische Befunde, `broadcast.time_ms`-Fix, ROADMAP-Drift bereinigt) ·
  **Secret-Hardening** (AES-GCM-AAD-Bindung an `(feed_id, cred_ref)`).
  Cross-Repo: Firefly OpenSky **OAuth2 Client-Credentials** (ADR 0024).

- **Mac-mini-E2E über Bridge-Netz (Sitzung 2026-07-01):** Der orchestrierte
  E2E-Stack braucht Host-Net-Multicast und damit Linux; auf Docker Desktop
  (Mac mini/Windows) geht das nicht. Neu: eingecheckte **`docker-compose.bridge.yml`**
  (Firefly + Postgres + Wayfinder in **einem** Bridge-Netz; Container↔Container-
  Multicast funktioniert dort). Runbook `docs/E2E-ABNAHME.md` um **Anhang E**
  (E-1 Abgrenzung zum Auto-Spawn, E-2 Ablauf mit **festem** Feed-Endpoint statt
  Auto-Allokation) erweitert; `DOCKER.md` (Inline-YAML → Verweis auf die Datei,
  tote „Teil E-2"-Referenz aufgelöst, Stack-Tabelle), `INSTALLATION.md` (Schritt 4.A
  Kurzweg) und `TECHNICAL.md` (Einschränkungs-Tabelle) nachgezogen. Gates grün
  (gofmt/build/vet + 28 Test-Pakete; `docker compose config` valide). Kein
  Go-/ICD-Change — reine Betriebs-/Abnahme-Ergänzung.

- **ADR 0014 — Multi-Tenant als einziger Betriebsmodus (diese Sitzung):**
  Single-Tenant vollständig entfernt. **A** (ADR + Charta-Prinzip, PR #94 gemergt) ·
  **B** (Code: `none`-Modus/No-DB-Fallback/nil-Scope raus, DB **+** Auth Pflicht,
  unset `AUTH_MODE`→`builtin`, Legacy-`AUTH_TOKEN`-Gate weg) · **C** (ein
  Multi-Tenant-Deployment-Stack: `orchestrated.yml`→`builtin`, Single-Tenant-
  `docker-compose.yml` gelöscht, `DOCKER.md` aufgeräumt) · **D** (Doku:
  INSTALLATION/TECHNICAL/Anforderungen NFR-SEC-004/BETRIEB; `E2E-ABNAHME.md` als
  **EDLV-Zero-Touch-Runbook** neu). B–D in **PR #95**. Firefly-Doku quergeprüft —
  keine Änderung nötig (CAT062-Wire-Vertrag unverändert).

- **UI-getriebener E2E + Auth-UX-Lücken (diese Sitzung, PR #95):** UI-Audit über
  beide Repos. Admin-Konfig ist bereits vollständig per UI (Mandant/Nutzer/Feed/
  Quellen ADS-B+FLARM/Features/View/Abo). Geschlossene Lücken: **rollen-agnostischer
  `GET /api/whoami`**, **Mandanten-Login + Auth-Gate auf der Karte (`/`)**,
  **Logout** (Karte + Admin-Header), gemeinsamer `apiFetch`. `docs/E2E-ABNAHME.md`
  als **UI-only-Ablaufplan** neu (genau ein Terminal-Befehl zum Start, Rest per UI,
  Terminal nur zur Hinter-den-Kulissen-Prüfung: Firefly-Output Gruppe:Port +
  ADS-B/FLARM). Firefly-Audit: **ADS-B (`adsb_opensky`) und FLARM (`flarm_aprs`)
  beide produktionsreif** und live verdrahtet. Kundenseitige Landing-Login unter `/`:
  durch WF2-12.4 erfüllt + WF2-12.6 Minimal-Branding (siehe unten).

- **Sliding-Session + Login-Overlay (WF2-12.5, diese Sitzung, PR #95):** Der Lotse
  wird bei **aktiver** Nutzung nie ausgeloggt (ASD offen + lebende WS = aktiv, nicht
  Maus/Tastatur); eine verlassene Konsole läuft nach dem Idle-Fenster ab; ein Ablauf
  ist **sichtbar** (Login-Overlay „Sitzung abgelaufen") statt stillem Freeze. Server:
  `POST /api/session/renew`; Client: Renew alle 10 min + Tab-Fokus + WS-Reconnect;
  WS-Close → `/api/whoami`-Probe → ggf. Overlay. Standardwerte: `WAYFINDER_SESSION_TTL`
  = 12h (Sliding-Idle-Fenster), Renew-Takt 10 min. Doku: WF2-12.5, FR-UI-015, TECHNICAL.
  Gates grün (go+205 vitest+build). Manueller Browser-Durchlauf im echten Stack offen.

- **Landing-Branding + absolutes Sitzungs-Maximum (WF2-12.6, diese Sitzung, PR #95):**
  Drei offene Punkte abgearbeitet. **(1)** Landing-Login unter `/` trägt jetzt
  „Wayfinder — Anmelden" (Minimal-Branding, `10f1e04`; der Karten-Login-Gate selbst
  war durch WF2-12.4 bereits erfüllt — kein funktionaler Bedarf, kein separater
  `/login`-Pfad). **(2)** **Absolutes Sitzungs-Maximum** `WAYFINDER_SESSION_MAX_LIFETIME`
  (opt-in, **Default aus**): eine Sitzung lebt — egal wie aktiv — nie länger als diese
  Spanne ab Erst-Login. Signierter `iat`-Claim (rückwärtskompatibel, alte Cookies ohne
  `iat` sanft verankert), Login+Renew kappen die Expiry auf `iat+MAX`, Renew `401` am
  Maximum; Durchsetzung nur in Login/Renew (Impersonation-Grant unberührt).
  Doku: WF2-12.6, FR-UI-016, TECHNICAL/INSTALLATION. Gates grün. **(3)** E2E auf
  Linux-Docker-Host: Offline-Baseline hier grün; Live-Kern bleibt Host-Sache
  (Checkliste beim Testen ins Runbook). **Probelauf:** `WAYFINDER_SESSION_MAX_LIFETIME=30m`.

- **Nächste Schritte (für die frische Session — priorisiert):**
  1. **Realer E2E-Abnahme-Lauf.** Zwei Wege: (a) **Mac mini / Docker Desktop**
     über `docker-compose.bridge.yml` — voller UI-Durchlauf **+ Live-Tracks**,
     aber **ohne** Orchestrator-Auto-Spawn (Runbook Anhang E). (b) **Linux-Docker-
     Host** für den orchestrierten Kern: `scripts/e2e-orchestrated.sh`
     (Prüfpunkte 1/2/5/8) + authentifizierter Lauf mit echten OpenSky-
     `client_id`/`client_secret` (Prüfpunkte 3/4/6/7). Der Auto-Spawn-Nachweis
     (1/2/8) bleibt **Linux-Sache**.
  2. **Offene Wayfinder-Issues:** #57 (Admin-UI View-Config-Captions, S2) ·
     #64 (Session-Registry/-Limit, S4) · #68 (Impersonation auf `admin`-Rolle, S4).
  3. **Firefly-Cross-Project (Issue #35):** die übrigen Live-Adapter
     `flarm_aprs` + `radar_asterix` (je eigener ADR; Vokabular im Kontrakt
     reserviert, Wayfinder-Rendering steht schon).
  4. **Prio 2 — Epic CWP/EFS/IMS** (ADR 0013, modulare Controller Working
     Position) — großes Folge-Epic, erst nach Prio 1.
  5. **ORCH-6** (K8s-`InstanceBackend` + HA) — Skalierung; Secret-Management je
     Feed ist via ORCH-2c/5b bereits vorgezogen.

> 🧭 **Maßgeblich für „was als Nächstes":** `docs/ROADMAP.md` (Prioritäts-Rahmen)
> + dieses STATUS. Anforderungs-/Test-Rückverfolgung: `docs/requirements/README.md`
> (FR-ORCH-001…007, NFR-SEC-004).

---

## ✅ Abgeschlossene Stufen (Wayfinder 2.0)

| Stufe | Inhalt | Status |
|---|---|---|
| **Stufe 0** | ADRs 0001–0005 (Stack, Security, Observability) | ✅ |
| **Stufe 1** | CAT062-Decoder, Track-Modell, WS-Server, MapLibre-Karte | ✅ |
| **Stufe 2** | Mandanten-isolierter Datenstrom (WF2-20–WF2-23) | ✅ |
| **Stufe 3** | Admin-API + Admin-UI + Live-Apply (WF2-31–WF2-33) | ✅ |
| **Stufe 4** | Provenienz (WF2-40), Sensorklassen (WF2-41), Feature-Entitlements (WF2-50) | ✅ |
| **ASD-012** | Range-Rings + Scale-Bar + Nord-Orientierung | ✅ |
| **WF2-34** | Cross-Tenant Read-Only-Impersonation (ADR 0008) | ✅ |
| **ADR 0009** | Admin-Bereich-Neuschnitt: AP1–AP6 (Rollen, Features, Dashboard, Feed-Health, Impersonation, Zugänge) | ✅ (AP7 offen) |
| **WF-1–WF-4** | CAT063 Sensor-Status-Decoder + Broadcast + Frontend-Banner (ICD 2.5.0) | ✅ |

---

## 📦 Produktions-Phase (laufend)

### ✅ Epics fertig

| Epic | Inhalt | Status |
|---|---|---|
| **ONB (ADR 0011)** | Zero-Touch-Onboarding: ONB-0…ONB-6 (Auto-Seed, Pflichtwechsel, Admin-CRUD, Mandanten-CRUD, Feed-CRUD, OpenAIP pro Mandant) | ✅ ICD 2.5.0 |
| **ORCH-0 (ADR 0012)** | Architektur-Entscheidung: 1 Firefly-Instanz pro Mandant, Reconciler-Konzept | ✅ |
| **ORCH-1 (ADR 0012)** | Feed-Quell-Datenmodell: `source_config`/`coverage_bbox`, Admin-API, UI-Quell-Builder (1a/1b/1c) | ✅ |
| **ORCH-2a/3 (ADR 0012)** | `Backend`-Interface + `MemoryBackend` + Reconciler (Operator-Muster) | ✅ |
| **ORCH-2b (ADR 0012)** | Docker-Backend-Adapter (`ContainerClient`, Spec-Hash, Labels) | ✅ |
| **ORCH-2c 1–3a (ADR 0012)** | `StoreDesiredState`, `wayfinder-orchestrator`-Binary (Least-Privilege), AES-256-GCM Secret-Store + Resolver | ✅ |
| **ORCH-2c 3a-API (ADR 0012 §6)** | Write-only Secret-Admin-API + `SecretSealer` + `WAYFINDER_SECRET_KEY` + Frontend-Bedienung | ✅ |
| **ORCH-2c 3b (ADR 0012 §5)** | Änderungs-getriebener Reconcile: Migration 00012 (`LISTEN/NOTIFY`-Trigger) + `Listener` + Trigger-Channel/Coalescing | ✅ |
| **ORCH-4 (ADR 0012)** | Automatische Multicast-Endpoint-Allokation: Migration 00013 (`UNIQUE`) + `MulticastPool`/`CreateAutoAllocated` + optionaler Endpoint im Admin-API + Frontend | ✅ |
| **ADR 0013** | Modular CWP & Enterprise ATC Integration ratifiziert (Prio 2, Planung) | ✅ |

### 🚧 Offen

Siehe zentrale **`docs/ROADMAP.md`** für aktuelle Priorisierung (Prio 1 / Prio 2):

- **Prio 1 (jetzt):** ORCH-5 (Container-Injection + Firefly-Quell-Env, cross-project, Firefly #35) → ORCH-6 (ORCH-1, ORCH-2/3, ORCH-2c 3a+3a-API+3b, ORCH-4 ✅)
- **Prio 2 (nach Prio 1):** Modular CWP / EFS / IMS (ADR 0013, Epic CWP-0…IMS-3)
- **ADR 0009 AP7:** Session-Registry, DB-gestützt (S4, offen)

---

## 📋 Cross-Project-Abhängigkeiten (zu Firefly)

Siehe `docs/cross-project/todo-for-wayfinder.md`:

- **ORCH-5 (Live-Quell-Ingestion)** — Firefly-Input-Adapter `adsb_opensky` (Ports & Adapters)
- **Per-Track-Sensor-Provenienz** — erfordert CAT062-ICD-Änderung
- **SWIM-Integration** — Abhängigkeit von Wayfinder EFS/IMS (Prio 2)
- **Ende-zu-Ende-HA** — Wayfinder WF2-52/53 ↔ Firefly SDPS-002

---

## 🔧 Technologie-Stack (ratifiziert)

- **Backend:** Go (ADR 0001) — UDP/Multicast-Eingang, WebSocket-Ausgang
- **Frontend:** Vue 3 + MapLibre GL JS (ADR 0002/0009)
- **Datenbank:** PostgreSQL (Mandanten, Feeds, Entitlements, Sessions)
- **Eingang:** ASTERIX CAT062/CAT065/CAT063 über UDP-Multicast (Draht-Vertrag mit Firefly)
- **Deployment:** Docker + Kubernetes-ready (ADR 0005)

---

## 📚 Wichtige Dateien

- `docs/ROADMAP.md` — zentrale Roadmap für Wayfinder **und** Firefly
- `docs/decisions/` — ADRs (0001–0013)
- `docs/ICD-CAT062.md` → wird gepflegt im **Firefly-Repo** (maßgeblich)
- `CLAUDE.md` — Arbeitsregeln
