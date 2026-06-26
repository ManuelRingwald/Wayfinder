# ONB-2 — Zero-Touch-Onboarding: Selbstverwaltungs-UI „Mein Konto"

> Teil des Zero-Touch-Onboarding-Epics (ADR 0011). Dieses Paket ergänzt ein
> jederzeit zugängliches „Mein Konto"-Panel im Dashboard, das Passwortänderung
> und Konto-Löschung ohne Terminal ermöglicht. Die Backend-Endpunkte stammen aus
> ONB-1.

## Fachlicher Hintergrund

ONB-1 hat den erzwungenen Passwort-Wechsel beim ersten Login implementiert. Danach
fehlte eine Möglichkeit, das eigene Passwort im laufenden Betrieb zu ändern oder
das Konto zu löschen — beides erforderte bisher Administratorzugang zur Datenbank
oder einen Neustart mit neuem Seed. ONB-2 schließt diese Lücke: Ein
Administrator kann beides direkt aus dem Dashboard heraus erledigen.

## Was umgesetzt wurde

### 1. Store (`admin.js`) — `deleteOwnAccount`

Neue Action `deleteOwnAccount()`:
- Ruft `DELETE /api/admin/me` auf (bereits in ONB-1 implementiert).
- Bei Erfolg (204): Identity wird geleert, `accessStatus = 401` — der nächste
  Render zeigt die Login-Maske.
- Bei 409: Letzter-aktiver-Admin-Guard des Servers — Fehlermeldung wird im
  Store gesetzt, Identity bleibt erhalten.
- Bei sonstigen Fehlern: Fehler-Banner, keine Zustandsänderung.

### 2. Neue Komponente `MyAccountPanel.vue`

`v-dialog`-basiertes Panel mit zwei Abschnitten:

**Passwort ändern** (nutzt vorhandene `changeOwnPassword` Store-Action):
- Drei Felder: aktuelles Passwort, neues Passwort (min. 8 Zeichen), Bestätigung.
- Client-seitige Validierung: Mindestlänge, Übereinstimmung.
- Erfolgs-/Fehlermeldung direkt im Panel (kein Dashboard-Banner).
- 401-Antwort → „Das aktuelle Passwort ist falsch."

**Konto löschen** (nutzt neue `deleteOwnAccount` Store-Action):
- Erst „Konto löschen …"-Button; nach Klick erscheint ein Bestätigungs-Schritt.
- 409-Antwort → „Löschen nicht möglich: Sie sind der letzte aktive Administrator."
- Bei Erfolg: Store leert Identity → Dashboard springt automatisch zur Login-Maske.

### 3. Einstiegspunkt in `AdminView.vue`

Der bestehende Benutzername-Chip in der App-Bar ist jetzt anklickbar (`cursor: pointer`,
`mdi-account-cog`-Icon) und öffnet das `MyAccountPanel`. Das Panel wird direkt
unter der App-Bar gemountet (neben dem Login-/Dashboard-Block).

## Byte-/Verhaltens-Vertrag

- Passwortänderung: `PUT /api/admin/me/password {current_password, new_password}` →
  204 bei Erfolg, 401 bei falschem Current-PW, 400 bei < 8 Zeichen (Server).
- Konto-Löschung: `DELETE /api/admin/me` → 204 bei Erfolg, 409 wenn letzter
  aktiver Admin (Server-Guard, kein UI-Bypass möglich).

## Qualitäts-Gates

- `npm run test` (140 Frontend-Tests) grün.
- `npm run build` erfolgreich.
- Tests:
  - `admin.test.js` — `describe('admin store — self-management (ONB-2)')` mit
    3 Tests: Erfolgreiche Löschung, 409-Guard, 500-Fehler.

## Rückverfolgbarkeit

- **Anforderung:** FR-ADMIN-005 (`docs/requirements/README.md`).
- **Design:** ADR 0011 (`docs/decisions/0011-zero-touch-onboarding.md`).
- **Backend:** ONB-1 (`pkg/adminapi/adminapi_me.go` — `DELETE /api/admin/me`).
- **Vorgänger:** ONB-1 (`docs/milestones/ONB-1_Zero_Touch_Auto_Admin.md`).
- **Folgepakete:** ONB-3 (Admin-Verwaltung), ONB-4 (Mandanten-CRUD),
  ONB-5 (Feed-CRUD + Live-Join), ONB-6 (OpenAIP/Mandant).
