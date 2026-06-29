# Arbeitsstand (Handover-Notiz) — Wayfinder

> **Zweck:** Diese Datei beschreibt den **aktuellen IST-Stand** von Wayfinder.
> Sie wird am Ende jeder Arbeitssitzung aktualisiert und committet.
> Claude liest sie zu Sitzungsbeginn (siehe `CLAUDE.md`).

> 🗺️ **Roadmap & Arbeitspakete:** siehe `docs/ROADMAP.md` in **diesem Repo**
> (zentrale Quelle für Wayfinder **und** Firefly). Cross-Project-Abhängigkeiten
> in `docs/cross-project/todo-for-wayfinder.md`.

---

## 🎯 Stand 2026-06-29

- **Zuletzt aktualisiert:** 2026-06-29
- **Letzte Arbeit:** **ORCH-2c (3b)** — änderungs-getriebener Reconcile (auf Branch
  `claude/roadmap-action-points-z50u6w`). Migration `00012` (Statement-Trigger auf
  `feeds`/`subscriptions` → `pg_notify('wayfinder_reconcile')`), `orchestrator.Listener`
  (dedizierte `LISTEN`-Verbindung, Resync-on-(Re-)Connect, Reconnect-Backoff),
  `Reconciler.Run` um nicht-blockierenden Size-1-Trigger-Channel (Coalescing),
  Intervall bleibt Sicherheitsnetz. Real-PG-verifiziert (Migration + Listener).
  Davor in main gemergt: **ORCH-1** (#77), **ORCH-2a/2b/2c-1-3a + ORCH-3** (#78),
  **ORCH-2c 3a-API** (#79). Cross-Project: Firefly **#35** (ORCH-5 Quell-Eingangs-Kontrakt) offen.
- **Nächster Schritt:** **Container-Injection** (`cred_ref` → Firefly-Quell-Env) ist
  durch Firefly #35 geblockt; unabhängig buildbar: **ORCH-4** (Multicast-Allokation).
  Ankündigung & „Go" je Schritt.

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
| **ADR 0013** | Modular CWP & Enterprise ATC Integration ratifiziert (Prio 2, Planung) | ✅ |

### 🚧 Offen

Siehe zentrale **`docs/ROADMAP.md`** für aktuelle Priorisierung (Prio 1 / Prio 2):

- **Prio 1 (jetzt):** ORCH-4 (Multicast-Allokation) → ORCH-5 (Container-Injection + Firefly-Quell-Env, cross-project, Firefly #35) → ORCH-6 (ORCH-1, ORCH-2/3, ORCH-2c 3a+3a-API+3b ✅)
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
