# ORCH-5b-2 — Quell-Credential-UI (Benutzername/Passwort, UX-2)

> Abschluss von ORCH-5b: Der Admin gibt eine Quell-Credential als **zwei**
> Felder ein — Benutzername und Passwort — die zu **einem** verschlüsselten
> `user:pass`-Secret kombiniert werden. Reine UI-/Frontend-Arbeit auf der
> bestehenden write-only Secret-API (ORCH-2c 3a-API); der Secret-Fluss in den
> Container ist ORCH-5b-1.

## Fachlicher Hintergrund

Authentifizierte Quellen (z. B. OpenSky mit Client-ID + Client-Secret) brauchen
zwei logische Teile. Der Secret-Speicher hält je `cred_ref` aber **einen** opaken
Wert, und Firefly löst die Credential auf, indem es am **ersten** Doppelpunkt
splittet (`user` = davor, `pass` = der Rest; ADR 0023). Damit der Betreiber nicht
selbst `user:pass` tippen muss (fehleranfällig), bietet das Admin-UI zwei Felder
und fügt sie korrekt zusammen.

## Was umgesetzt wurde

- **`frontend/src/admin/credential.js`** (neu, rein/testbar):
  - `validateCredential(user, pass)` → Fehlermeldung oder `''`. Beide Felder
    Pflicht; der Benutzername darf **keinen** Doppelpunkt enthalten (sonst würde
    der First-Colon-Split einen Teil als Passwort lesen). Das Passwort darf
    Doppelpunkte tragen.
  - `combineCredential(user, pass)` → `user:pass` (Benutzer getrimmt, Passwort
    verbatim) oder `null` bei ungültigem Paar.
- **`frontend/src/components/admin/AdminFeeds.vue`:**
  - Statt eines Passwort-Felds zwei Felder (`secretUser[i]`/`secretPass[i]`) je
    Quellzeile; Hinweis „kein Doppelpunkt im Benutzernamen".
  - `saveSecret` kombiniert via `combineCredential` und sendet den `user:pass`-Wert
    über die unveränderte `setFeedSecret`-Store-Action; ungültiges Paar → Speichern
    blockiert (Button disabled + Warn-Alert).
  - „Secret hinterlegt/Kein Secret"-Chip und „Entfernen" je `cred_ref` unverändert.

## Sicherheits-/Schnittstellen-Betrachtung

- **Write-only bleibt:** der Server meldet nur, **ob** ein Wert gesetzt ist
  (`configured`), nie den Wert; `GET` liefert keinen Klartext. Die Felder sind
  `autocomplete=off`/`new-password`.
- **Kein neuer Backend-/API-Pfad:** die Kombination passiert im Browser vor dem
  PUT; die `…/feeds/{id}/secrets`-API und der `SecretSealer` sind unverändert.
- **Kontrakt-Treue:** `user:pass` mit First-Colon-Semantik = exakt was
  ORCH-5b-1/Firefly beim Auflösen erwartet.

## Tests

- `frontend/src/admin/__tests__/credential.test.js`: validate (leer/Doppelpunkt im
  Benutzernamen/Doppelpunkt im Passwort erlaubt) + combine (Join, Trim,
  Doppelpunkt im Passwort erhalten, `null` bei ungültig).
- `npm test` (vitest) grün (180 Tests); `npm run build` grün (eingebettetes
  `internal/webui/dist` aktualisiert); `go build ./...` grün.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-006** (ORCH-5b-2 ✅) nachgezogen.

## Stand von ORCH-5

ORCH-5 (Quell-Eingangs-Übersetzung + Credential-Fluss) ist damit **komplett**:
Rendering (5a) → Control-Plane-Auflösung/-Injection (5b-1) → UI (5b-2). Offen bleibt
nur die End-to-End-Abnahme mit echtem authentifiziertem OpenSky sowie — separat,
eigene ADRs — Fireflys FLARM/APRS- und Radar-ASTERIX-Live-Adapter (#35).
