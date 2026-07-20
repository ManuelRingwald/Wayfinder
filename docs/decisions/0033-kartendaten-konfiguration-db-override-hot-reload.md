# ADR 0033 — Kartendaten-Konfiguration: DB-Override über Env + Hot-Reload

- **Status:** **AKZEPTIERT** ✅ (2026-07-20). Fundament (K0) des Admin-Vorhabens
  „Kartendaten live konfigurierbar" (Epic #307): Wetter, Basiskarte, Radar-
  Abdeckung und Aeronautik sollen im Admin **ohne Neustart** einstellbar sein.
- **Datum:** 2026-07-20
- **Schnittstellen-relevant:** nein (kein CAT062-/Firefly-Bezug). Sicherheits-
  relevant: ja (server-seitiger Fetch admin-gesetzter URLs). Register: **FR-CFG-007**.
- **Bezug:** Epic #307, K0 (#308). Baut auf ADR 0005 (Multi-Tenant, Konfig als
  Daten), ADR 0018 (`platform_settings` + versiegelter UI-Key), `pkg/secret`,
  `pkg/store/settings.go`.

## Kontext

Drei der vier Karten-Datenquellen (Wetter/DWD, Basiskarte/BKG, Radar-Abdeckung)
werden heute **ausschließlich über Startup-Env** konfiguriert — Änderung erst
nach Neustart. Nur Aeronautik (OpenAIP) hat bereits eine DB-gestützte,
UI-editierbare Konfiguration (ADR 0018) und dient als Vorbild. Um die anderen
drei live-editierbar zu machen, braucht es ein **wiederverwendbares Fundament**
statt drei Einzellösungen.

## Entscheidungen

### 1. Precedence: DB-Override **über** Env-Default (12-Factor bleibt gültig)

Jede Einstellung hat einen **Startup-Env-Default** und einen optionalen
**DB-Override** in `platform_settings`. **Effektiv = Override, wenn vorhanden,
sonst Env-Default.** „Auf Default zurücksetzen" **löscht** die DB-Zeile.

Folge: Ein frisches Deployment **ohne** DB-Config verhält sich exakt wie bisher
(reine Env). Der Admin überschreibt nur, was er ändern will, und kann jederzeit
auf den Deployment-Default zurück. 12-Factor (Env-Konfiguration) bleibt der
Boden; die DB ist eine **Lauf­zeit-Überschreibung**, kein Ersatz.

### 2. Hot-Reload über eine Registry, **defensiv** (letzte gute Konfig)

Eine Konfig-Änderung publiziert an eine **Reload-Registry**; der besitzende
Dienst (weathertiles/basemap/coverage/…) registriert dort eine `ReloadFunc`, die
die effektive Konfig neu liest und **live anwendet**. Bei Fehler behält der Dienst
seine **letzte gute Konfig**, gibt den Fehler an die Admin-Antwort zurück und
**crasht nie** (CLAUDE §7: Operator-Eingabe darf kein laufendes Scope brechen).
Die Admin-PUT-Antwort meldet einen Reload-Fehler ehrlich als `reload_error`
(gespeichert, aber nicht angewandt) statt still zu scheitern.

### 3. Secrets bleiben versiegelt (nicht in dieser Plane)

Geheime Werte (OpenAIP-Key) laufen **nicht** über die Klartext-Plane, sondern
weiter über das versiegelte Muster (`pkg/secret` + `platform_settings`,
ADR 0018). `mapconfig` ist für **nicht-geheime** Werte (URLs, Themes, Flags,
JSON-Blobs); ein Klartext-Secret erreicht weder diese Plane noch die UI.

### 4. SSRF-Grenze: admin-gesetzte URLs werden vor dem Speichern validiert

Admin ist eine vertrauenswürdige Rolle, aber ein **server-seitiger Fetch** einer
operator-getippten URL ist eine **SSRF-Fläche**. `ValidateFetchURL` erzwingt
Defense-in-Depth **vor** dem Speichern: nur `http`/`https`; Host vorhanden;
Literale IPs in privaten/Loopback-/Link-Local-/Unspecified-/ULA-Bereichen
abgelehnt (u. a. Cloud-Metadaten `169.254.169.254`); interne Namen
(`localhost`, `*.local`, `*.internal`) abgelehnt; optionale harte Host-Allowlist.

**Dokumentierter Rest-Risiko:** Ein **öffentlicher Name, der auf eine private IP
auflöst** (DNS-Rebinding), wird hier **nicht** erkannt — das bräuchte eine
Prüfung zur Fetch-Zeit (Resolve + Re-Check) oder eine strikte Allowlist. Beim
Trusted-Admin-Bedrohungsmodell ist das eine bewusst akzeptierte Grenze; eine
Verschärfung ist Folge-Arbeit. Größen-/Timeout-Grenzen des Fetch liegen in den
fetchenden Diensten (z. B. `pkg/basemap`).

## Umsetzung (K0)

Neues Paket **`pkg/mapconfig`** (rein, unit-getestet):
- `Setting` — DB-Override ?? Env-Default (`Effective`/`Overridden`/`Set`/`Reset`;
  leerer Wert = Reset; Store-Fehler → Degradation auf Env-Default).
- `Registry` + `ReloadFunc` — defensives Hot-Reload-Dispatch je Domain.
- `ValidateFetchURL` — SSRF-Leitplanken (Schema/Host/IP-Bereiche/Allowlist).
- `Resource.Handler` — generischer `GET/PUT`-Admin-Endpunkt (lesen/validieren/
  speichern/reload; Reload-Fehler als `reload_error` mit 200).

**Noch nicht** Teil von K0: konkrete Subsystem-Verdrahtung + Admin-UI (K1–K5).

## Konsequenzen

- Ein einheitliches, getestetes Fundament; die Subsystem-Panels (K2–K5) sind
  dünne Verdrahtungen darauf.
- Determinismus des Track-Rechenpfads unberührt (Konfig-Ebene, Wanduhr-Admin-
  Aktion, nicht Datenzeit).
- **Ehrliche Grenze:** DNS-Rebinding-SSRF nicht abgedeckt (s. o.); Hot-Reload-
  Korrektheit je Dienst wird in K2–K5 einzeln nachgewiesen.
