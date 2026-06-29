# ORCH-2c (3a-API) — Write-only Secret-Admin-API + Frontend

> Schließt die Schreib-Seite des Pro-Feed-Secret-Mechanismus (ADR 0012 §6,
> NFR-SEC-004). ORCH-2c (3a) hatte den **verschlüsselten Speicher** + Resolver
> gebaut; hier kommt der Weg, **einen Wert hineinzulegen** — streng write-only,
> samt Frontend und Schlüssel-Verdrahtung in den Server.
>
> **Lieferumfang 3a-API:** Sealer (Schreib-Gegenstück zum Resolver) + write-only
> Admin-API + `WAYFINDER_SECRET_KEY`-Verdrahtung in den Server + Frontend-Bedienung
> + Tests. **Noch nicht** Teil: Container-Injection des aufgelösten Werts (braucht
> den Firefly-Quell-Env-Contract, ORCH-5) und der Änderungs-Trigger (3b).

## Fachlicher Hintergrund

Die Quell-Konfig (ORCH-1) referenziert Credentials nur per Handle (`cred_ref`).
Der echte Wert (OpenSky-Client-Secret, FLARM-/Radar-Zugang) muss gesetzt,
rotiert und gelöscht werden können — **vom Admin**, aufgelöst **vom Orchestrator**
beim Container-Start, **nie** zum Browser. Vor diesem Schritt gab es keinen Weg,
einen Wert abzulegen; jetzt schließt sich die Lücke.

## Was umgesetzt wurde

### Sealer — Schreib-Gegenstück zum Resolver (`pkg/orchestrator`)
`SecretSealer` spiegelt `SecretResolver`: `SetSecret(feedID, ref, plaintext)`
**versiegelt** den Wert mit dem Deployment-Schlüssel (`secret.Cipher.Seal`,
AES-256-GCM, zufälliger Nonce + Auth-Tag) und legt nur den opaken Blob ab;
`DeleteSecret`/`ListSecretRefs` reichen an den Store durch. Er lebt im
**browser-zugewandten Server** (der einzige, der einen vom Admin gelieferten Wert
annimmt) — der Klartext wird **sofort** versiegelt, nie im Klartext persistiert,
nie geloggt, nie zurückgegeben. Gelesen/entschlüsselt wird ausschließlich in der
getrennten Control-Plane (`SecretResolver`). `SecretWriter` ist das schmale
Store-Interface (`Set`/`Delete`/`ListRefs`), erfüllt von `store.SecretRepo`.

### Write-only Admin-API (`pkg/adminapi`)
Drei Routen hinter `requireAdmin`, Muster wie die OpenAIP-Key-Isolation (ONB-6):

| Route | Verb | Verhalten |
|-------|------|-----------|
| `…/feeds/{id}/secrets` | GET | `{secrets:[{ref, configured:true}]}` — nur **welche** Refs gesetzt sind, **nie** ein Wert |
| `…/feeds/{id}/secrets/{ref…}` | PUT | `{value}` → 204; Wert wird **vor** der Speicherung versiegelt, nie zurückgegeben. Leer → 400 (Löschen via DELETE), >4096 → 400 |
| `…/feeds/{id}/secrets/{ref…}` | DELETE | Wert entfernen → 204; nicht gesetzt → 404 |

`adminapi` bleibt **crypto-agnostisch**: es hängt nur an einem schmalen
`SecretService`-Interface (`SetSecret`/`DeleteSecret`/`ListSecretRefs`), das der
`SecretSealer` erfüllt — der Schlüssel berührt die API-Schicht nie. Das
`{ref…}`-Trailing-Wildcard erlaubt `cred_ref`s mit Slash (z. B. `secret/opensky`).
Ist **kein** `SecretService` verdrahtet (kein Schlüssel), antworten alle drei
Routen **503** — die Fähigkeit ist schlicht aus, nie still unverschlüsselt.

### Schlüssel-Verdrahtung (`cmd/wayfinder`)
`WAYFINDER_SECRET_KEY` (base64-32-Byte) → `secret.Cipher` →
`orchestrator.NewSecretSealer(store.NewSecretRepo(pool), cipher)` → in
`adminapi.New` injiziert. Fehlt der Schlüssel: Routen deaktiviert (503), Server
bootet normal. Ist er **gesetzt aber ungültig**, wird das **laut geloggt**
(Warn) statt still herabgestuft. Das Open-Plumbing im
`wayfinder-orchestrator` folgt erst mit der Container-Injection (ORCH-5) — jetzt
wäre es totes Wiring.

### Frontend (`AdminFeeds.vue`, `stores/admin.js`)
Im Quellen-Dialog je Quelle mit `cred_ref`: ein Status-Chip („Secret hinterlegt"
/ „Kein Secret"), ein Passwort-Feld zum Setzen/Ersetzen und ein Entfernen-Knopf.
Beim Öffnen lädt der Dialog die konfigurierten Refs (`loadFeedSecrets`); ein
**503** schaltet die Bedienelemente unsichtbar (kein Schlüssel server-seitig). Die
Store-Actions `setFeedSecret`/`deleteFeedSecret` kodieren den Ref mit `encodeURI`
(Slashes bleiben erhalten). Gating ist kosmetisch — der Server erzwingt jede Grenze.

## Sicherheits-Betrachtung

- **Write-only durchgehalten:** Kein Pfad gibt einen Wert zurück; der `GET` meldet
  nur `configured`. (Test: Antwort enthält den Wert nie.)
- **At-rest verschlüsselt vor Persistenz:** Der Sealer versiegelt **vor** dem
  Store-Schreiben; ein DB-Leak liefert nur Ciphertext.
- **Schlüssel nur zum Seal im Server**, nur zum Open in der Control-Plane; nie in
  der DB, nie geloggt.
- **Fail-safe ohne Schlüssel:** 503 statt unverschlüsselt zu speichern.
- **Browser-Rand isoliert:** Entschlüsselung passiert nie im browser-zugewandten
  Server; der hält den Schlüssel nur zum Versiegeln.
- **Keine CAT062-Schnittstellen-Wirkung** — rein Wayfinder-intern.

## Tests

- `pkg/orchestrator/secret_test.go`: `…SealerStoresCiphertextAndRoundTrips`
  (Store erhält Ciphertext, nicht Klartext; Resolver mit gleichem Schlüssel
  gewinnt den Klartext zurück; `ListSecretRefs`/`Delete`/`ErrNotFound`).
- `pkg/adminapi/adminapi_secrets_test.go`: configured-Liste (sortiert), **kein
  Wert-Leak**, PUT-Round-Trip **mit Slash-Ref**, Wert-Pflicht (leer → 400),
  zu-lang → 400, DELETE + 404, unbekannter Feed → 404, **503 ohne Schlüssel**,
  **403 für Non-Admin**.
- `frontend/.../admin.test.js`: `loadFeedSecrets`/`setFeedSecret`/`deleteFeedSecret`
  (GET, 503-Pfad, PUT mit Slash-Ref + `{value}`, DELETE + Notice).
- Gates: `go test ./...`, `go vet`, `gofmt` grün; 168 Vitest-Tests grün;
  Production-Build erfolgreich.

## Rückverfolgbarkeit

Anforderungs-Register: **NFR-SEC-004** (um Sealer + write-only Admin-API +
Schlüssel-Verdrahtung ergänzt).

## Nächste Stücke

- **Container-Injection** — `SecretResolver` → Firefly-Container-Env; braucht den
  Firefly-Quell-Env-Contract (ORCH-5).
- **3b** — Postgres `LISTEN/NOTIFY`-Änderungs-Trigger (sofortiger Reconcile statt
  Intervall-Wartezeit).
