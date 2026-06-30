# Secret-AAD-Identitäts-Bindung (Hardening)

> Defense-in-depth-Härtung des verschlüsselten Pro-Feed-Secret-Speichers
> (NFR-SEC-004), Befund aus dem Sicherheits-Review nach dem ORCH-Sprint.

## Fachlicher Hintergrund

Quell-Credentials liegen AES-256-GCM-verschlüsselt in `feed_secrets`. Die
Verschlüsselung schützt die **Vertraulichkeit at rest** (ein DB-Leak liefert keinen
Klartext). Sie band den Blob bisher aber **nicht** an seine Identität: ein Angreifer
mit **DB-Schreibzugriff** hätte einen Secret-Blob von Feed A auf Feed B kopieren
können — er entschlüsselt unter demselben Schlüssel weiter, und die Control-Plane
hätte Feed B mit den Credentials von Feed A gestartet.

## Was umgesetzt wurde

- **`pkg/secret/secret.go`:** `Seal(plaintext, aad)` / `Open(blob, aad)` reichen
  **Additional Authenticated Data** an AES-GCM durch — authentifiziert (vom Tag
  gedeckt), aber **nicht** verschlüsselt und **nicht** gespeichert. Ein abweichendes
  (oder fehlendes) AAD scheitert beim Open exakt wie ein falscher Schlüssel.
- **`pkg/orchestrator/secret.go`:** `credAAD(feedID, credRef)` = `feedID` (dezimal)
  + NUL + `credRef` — eindeutig, da die Dezimalzahl kein NUL enthält. `SecretSealer`
  versiegelt mit dieser AAD, `SecretResolver` öffnet mit derselben. Ein
  verschobener/replayter Blob unter fremder `(feed_id, cred_ref)`-Identität
  scheitert **fail-closed**.

## Sicherheits-Betrachtung

- **Bedrohung:** Angreifer mit DB-Schreibzugriff (jenseits des reinen Leaks).
- **Wirkung:** Relocate/Replay eines Blobs auf einen anderen Feed wird
  kryptografisch verhindert; der Klartext bleibt an seine Identität gebunden.
- **Grenze:** Dies ist Defense-in-depth — DB-Schreibzugriff bleibt ein schwerer
  Vorfall; die Bindung verkleinert nur die Missbrauchs-Fläche.

## Migrations-Hinweis

Die AAD ist Teil dessen, was der Tag authentifiziert: **vorher ohne AAD versiegelte
Blobs werden danach nicht mehr entschlüsselt.** Da noch **keine Produktiv-Secrets**
existieren, ist keine Migration nötig. Käme das später in Betrieb, wäre ein
Re-Seal-Lauf erforderlich (oder ein Format-Versionsbyte).

## Tests

- `pkg/secret/secret_test.go::TestAADMustMatch` — Open nur unter byte-gleichem AAD;
  nil/abweichend → `ErrDecrypt`.
- `pkg/orchestrator/secret_test.go::TestSecretAADBindsToFeedIdentity` — ein für
  Feed 1 versiegelter Blob, unter Feed 2 gelesen → `ErrDecrypt`; unter Feed 1 →
  Round-Trip.
- `go test ./...`, `go vet`, `gofmt`, `golangci-lint` grün.

## Rückverfolgbarkeit

Anforderungs-Register: **NFR-SEC-004** (AAD-Identitäts-Bindung) nachgezogen.
