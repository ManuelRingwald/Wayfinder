# ORCH-2c (3a) — Verschlüsselter Secret-Speicher + Resolver

> Sicherheits-Kern des letzten ORCH-2c-Stücks (ADR 0012 §6, NFR-SEC-004). Gibt den
> Pro-Feed-Quell-Credentials eine **verschlüsselte** Heimat und einen Resolver,
> der sie in der getrennten Control-Plane entschlüsselt.
>
> **Entscheidung (Betreiber):** DB-Tabelle, **app-seitig verschlüsselt**
> (AES-256-GCM) — funktioniert über alle Phasen (Dev/Docker/später K8s), hält das
> Zero-Touch-Versprechen und bietet Defense-in-Depth gegen DB-at-rest-Leaks.
>
> **Lieferumfang 3a:** Krypto-Paket + Secret-Store + Resolver + Tests. **Noch
> nicht** Teil: write-only Admin-API + Frontend (3a-API), Container-Injection
> (braucht Firefly-Quell-Env-Contract, ORCH-5), Änderungs-Trigger (3b).

## Fachlicher Hintergrund

Die Quell-Konfig (ORCH-1) referenziert Credentials nur per Handle (`cred_ref`).
Der echte Wert (OpenSky-Client-Secret, FLARM-/Radar-Zugang) muss sicher abgelegt
werden: gesetzt vom Admin, aufgelöst vom Orchestrator beim Container-Start,
**nie** an den Browser, nie in ein DTO (analog OpenAIP-Key-Isolation ONB-6).

## Was umgesetzt wurde

### Krypto (`pkg/secret`)
`Cipher` über **AES-256-GCM**: `Seal` nutzt je Aufruf einen **zufälligen Nonce**
(zweimal dasselbe Klartext → verschiedene Blobs, keine deterministische
Leak-Fläche) und einen Auth-Tag; `Open` erkennt Manipulation/falschen
Schlüssel/Müll und liefert dann `ErrDecrypt` — **nie** Teil-Klartext (GCM
authentifiziert vor dem Entschlüsseln). `KeyFromBase64` parst den
deployment-verwalteten 32-Byte-Schlüssel (`WAYFINDER_SECRET_KEY`).

### Secret-Store (`pkg/store`, Migration 00011)
Tabelle `feed_secrets(feed_id, cred_ref, ciphertext, …)`, PK `(feed_id, cred_ref)`,
`ON DELETE CASCADE` auf `feeds`. `SecretRepo` ist **crypto-agnostisch** — speichert
nur den opaken Blob (`Set` Upsert / `Get` / `Delete` / `ListRefs`); der Schlüssel
berührt die Persistenz-Schicht nie. Der Blob ist `base64(nonce‖ciphertext‖tag)`.

### Resolver (`pkg/orchestrator`)
`SecretResolver.Resolve(feedID, credRef)` liest den Blob (`SecretReader`,
satisfied von `SecretRepo`) und entschlüsselt ihn mit dem `Cipher`. Lebt **nur in
der Control-Plane** (dem Prozess, der den Schlüssel hält und den Wert später in
den Container injiziert) — nie am Browser-Rand. Fehlender Ref → `ErrNotFound`;
manipuliert/falscher Schlüssel → `ErrDecrypt`.

## Sicherheits-Betrachtung

- **DB-at-rest geschützt:** Ein reiner DB-Leak liefert nur Ciphertext; ohne den
  deployment-verwalteten Schlüssel kein Klartext.
- **Schlüssel nie in der DB**, nie geloggt; getrennt vom Datenpfad.
- **Browser-Rand isoliert:** Entschlüsselung nur im Orchestrator; die (folgende)
  Admin-API gibt nie den Wert zurück, nur ob ein Ref gesetzt ist.
- **Authentifizierte Verschlüsselung:** Manipulation am Ciphertext wird erkannt
  (GCM-Tag) statt stillschweigend falsch entschlüsselt.
- **Ehrliche Grenze:** Die Verschlüsselung verteidigt die Data-at-rest-Grenze,
  **nicht** eine vollständige Prozess-Übernahme (wer den Prozess + Schlüssel hat,
  kann entschlüsseln — das ist erwartbar und dokumentiert).
- **Keine CAT062-Schnittstellen-Wirkung.**

## Tests

- `pkg/secret/secret_test.go`: Seal/Open-Round-Trip, Nicht-Determinismus, falscher
  Schlüssel/Manipulation/Garbage → `ErrDecrypt`, Schlüssel-Längen-Validierung,
  `KeyFromBase64`.
- `pkg/store/...::TestIntegrationSecretRepo` (real-PG): Set/Get/`ListRefs`
  (sortiert)/Upsert/Delete/`ErrNotFound`/Feed-Cascade.
- `pkg/orchestrator/secret_test.go`: Resolve, fehlender Ref → `ErrNotFound`,
  falscher Schlüssel → `ErrDecrypt`.

## Rückverfolgbarkeit

Anforderungs-Register: **NFR-SEC-004** (Quell-Credential-Isolation — um den
verschlüsselten Speicher + Resolver ergänzt).

## Nächste Stücke

- **3a-API** — write-only Admin-API (`PUT/DELETE /api/admin/feeds/{id}/secrets/{ref}`,
  `GET` nur `{configured}`) + Frontend; verdrahtet `WAYFINDER_SECRET_KEY` in den
  Server (Seal) und den Orchestrator (Open).
- **Container-Injection** — Resolver → Firefly-Container-Env; braucht den
  Firefly-Quell-Env-Contract (ORCH-5).
- **3b** — Postgres `LISTEN/NOTIFY`-Änderungs-Trigger.
