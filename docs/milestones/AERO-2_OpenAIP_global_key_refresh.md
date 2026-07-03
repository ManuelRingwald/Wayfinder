# AERO-2 — Globaler OpenAIP-Schlüssel via UI + Refresh-Buttons

> Zweites Häppchen der OpenAIP-„Connected-by-default"-Umstellung (ADR 0018), baut
> auf AERO-1 (persistenter Cache + fetch-once) auf. Als **ein** PR gebaut (2a+2b).

## Fachlich

Der globale OpenAIP-Rückfall-Schlüssel lebte bisher nur in der Env
(`WAYFINDER_OPENAIP_API_KEY`, Deploy-Zeit). Der Betreiber will ihn **im Betrieb**
über die UI setzen (kein mitgelieferter Schlüssel), **verschlüsselt** abgelegt, und
das Setzen soll **einmalig für alle Mandanten** einen Abruf auslösen. Dazu
**Refresh-Buttons** global und pro Mandant (zum AIRAC-Update) sowie die
Sichtbar­machung der in AERO-1 gelieferten **Zeitstempel/Objekt-Anzeige**.

## Technisch

- **Settings-Store (Migration `00018`):** generische `platform_settings`-Tabelle
  (`key`/`value`/`updated_at`); `SettingsRepo` (Get/Set/Delete, crypto-frei). Der
  globale Schlüssel liegt unter `openaip_global_key`, **versiegelt** mit `pkg/secret`
  (AES-256-GCM, AAD `openaip:global`).
- **Adapter (`cmd/wayfinder`):** `globalOpenAIP` — `Seal`/`Open`, env-Fallback,
  `Available`/`Configured`/`SetKey`/`effectiveKey`. Der Cipher
  (`WAYFINDER_SECRET_KEY`) wird **einmal** gebaut und mit der Feed-Credential-
  Versiegelung (ORCH-2c) geteilt. `tenantAeroLifecycle.globalKey` ist jetzt ein
  **dynamischer Reader** (DB sealed → env), also greift ein UI-Schlüssel **sofort**;
  neue `RefreshAll` iteriert die Mandanten.
- **Routen (platform-admin):** `GET/PUT /api/admin/openaip` (Status / setzen-löschen,
  `503` ohne Cipher, PUT triggert Fetch-all), `POST /api/admin/openaip/refresh`
  (Fetch-all), `POST /api/admin/tenants/{id}/openaip/refresh` (ein Mandant).
  `GlobalOpenAIPStore`-Interface + `WithGlobalOpenAIP`; `TenantAeroLifecycle.RefreshAll`.
- **Frontend:** neue Kopf-Navigations-Sektion **„OpenAIP"** (`AdminGlobalOpenAIP.vue`):
  globalen Schlüssel setzen/löschen (Feld deaktiviert + Hinweis, wenn
  `encryption_available=false`), „Alle Mandanten aktualisieren"-Button. In
  `AdminTenantDetail.vue`: „Jetzt aktualisieren"-Button + „zuletzt geholt / N Objekte".
  Store-Actions in `admin.js`.

## Sicherheits-Entscheidung (Option A)

Der versiegelte globale Schlüssel braucht `WAYFINDER_SECRET_KEY`. Ohne ihn liefert
die UI-Route **`503`** — **kein Klartext-Geheimnis** in der DB (konsistent mit den
Feed-Credentials, ORCH-2c). Ehrliche Grenze: die **Per-Mandant**-Schlüssel
(`tenants.openaip_api_key`) liegen weiterhin unverschlüsselt — deren Versiegelung
wäre ein separater Folge-Schritt.

## Schnittstellen-Wirkung

**Keine** (kein CAT062/065/063). Reine Wayfinder-interne Admin-/Speicher-Ebene.

## Gates

- `go build/vet/gofmt` grün; `go test ./...` **+ `-race`** grün (adminapi Routen,
  `globalOpenAIP`-Adapter Seal-Round-Trip/kein-Cipher/Decrypt-Fallback, real-PG
  `TestIntegrationSettingsRepo`). golangci-lint 0 issues.
- vitest grün inkl. AERO-2-Store-Block (323 Tests); `dist` neu gebaut (Frontend-Change).

## Ehrliche Grenze

Die OpenAIP-API ist in der Entwurfs-Umgebung nicht live erreichbar (Egress-Policy)
— best-effort deckt das ab; der Live-Smoke-Test (UI-Schlüssel setzen → Fetch-all →
Objekte sichtbar) ist ein Deploy-Schritt. **Folgt (optional):** AERO-3 —
AIRAC-Kalender + Self-Diff-Change-Impact.
