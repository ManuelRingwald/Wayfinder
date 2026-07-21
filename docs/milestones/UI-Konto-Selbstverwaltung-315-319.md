# UI-/Konto-Bestandsaufnahme #315–#319

> **Kurz:** Fünf vom Betreiber gemeldete Bedien-Themen der ASD-Oberfläche. Vier am
> Layer-/Filter-Menü, eines zur Konto-Selbstverwaltung. Die architektonische
> Weichenstellung (rollen-agnostische Konto-API) ist in **ADR 0033** festgehalten;
> Rückverfolgbarkeit über **FR-UI-050** und **FR-ADMIN-011** im Anforderungs-Register.

## Fachlich — welches Problem löst es für den Lotsen?

1. **#315 — Gruppen-Auswahl im Menü:** Ein Klick auf eine Layer-Gruppe (z. B.
   „Aeronautik") **de-selektierte** vorher alle Unterpunkte, statt sie zu
   selektieren — genau verkehrt herum. Jetzt schaltet ein Klick auf eine leere
   oder teilweise aktive Gruppe **alle** ihre Layer **an**; nur eine bereits
   vollständig aktive Gruppe wird geleert („fill-then-clear").
2. **#316 — Beschriftungen passen:** Das Menü-Panel war zu schmal, die
   Karten-Presets „Minimal/Standard/Detailliert" wurden abgeschnitten. Das Panel
   ist jetzt breiter (248 → 288 px), die Preset-Buttons teilen sich die Breite
   flexibel — die Beschriftungen passen.
3. **#317 — kein Scrollen in der 2. Ebene:** Vorher waren alle Gruppen offen und
   die Liste scrollte. Jetzt ist **immer nur eine** Gruppe aufgeklappt (Akkordeon):
   Öffnet der Lotse „Wetter", klappen „Aeronautik/Karte/Radar & Reichweite"
   automatisch zu.
4. **#318 — sieht man, was aktiv ist:** Die Menü-Icons „Layer" und „Filter" in der
   Seitenleiste **leuchten jetzt blau**, sobald mindestens ein Layer bzw. ein
   Filter aktiv ist — analog zum orangenen Leuchten „scharfer" Mess-Werkzeuge
   (RBL/DIST/QDM). So ist auf einen Blick erkennbar, dass etwas gefiltert/angezeigt
   wird, ohne das Panel zu öffnen.
5. **#319 — Konto selbst verwalten:** Ein Nutzer kann **unter „Konto"** eine neue
   **E-Mail-Adresse und ein neues Passwort** setzen — jetzt **auch ein reiner
   Lotse** im ASD, nicht nur ein Admin. Die geänderte E-Mail wird im **Admin-Panel**
   (Zugangstabelle) sichtbar. Für das Passwort gilt derselbe Mindeststandard
   (≥ 8 Zeichen) wie bei der Anlage über das Admin-Panel.

## Technisch — wie umgesetzt?

### Layer-Menü (#315–#318, Frontend)
- `frontend/src/map/layerGroups.js` — `nextMaster` liefert `state !== 'on'`.
- `frontend/src/components/LayerGroup.vue` — von intern-gesteuert auf **kontrolliert**
  (`:expanded` + `@toggle`) umgestellt.
- `frontend/src/components/LayerFilterContent.vue` — `openGroup`-Akkordeon (eine
  offene Gruppe), Preset-Button-CSS.
- `frontend/src/components/NavigationRail.vue` — Panelbreite 288 px; `hasActiveLayers`/
  `hasActiveFilter` + `nav-rail__btn--engaged`-Glow (Cyan).
- `frontend/src/design/tokens/spacing.css` — Panel-Breiten-Token nachgezogen.

### Konto-Selbstverwaltung (#319, Backend + Frontend) — siehe ADR 0033
- **Backend:** `pkg/store/users.go` (`SetEmail`); `pkg/adminapi/adminapi_me.go`
  (`putMeEmail` + E-Mail-Validierung); `pkg/adminapi/adminapi.go`
  (`AccountHandler()`, `email` in `whoamiDTO`); `cmd/wayfinder/main.go` (Mount
  `/api/account/` hinter `tenantMW`+`pwGate`, **nicht** dem Admin-Gate).
- **Frontend:** `stores/session.js` + `stores/admin.js`
  (`changeOwnPassword`/`changeOwnEmail` → `/api/account/*`);
  `components/AccountSelfServiceDialog.vue` (ASD-Sidebar „Konto");
  `components/admin/MyAccountPanel.vue` (E-Mail-Feld).

## Sicherheit (Kernpunkt #319)

Die neuen Routen sind **self-scoped**: die User-ID kommt aus der Session-Identity
(`tenant.FromContext`), **nie** aus dem Request — ein Nutzer kann ausschließlich
sein **eigenes** Konto ändern, unabhängig von der Rolle (Muster wie
`/api/view-profiles`, ADR 0023). Der Pflicht-Passwortwechsel des Seed-Admins läuft
weiter über das Admin-gegatete, allowlisted `/api/admin/me/password` — kein
Deadlock, keine Rollen-Eskalation. Kein CAT062-/Wire-Vertrag betroffen.

## Verifikation
- **Frontend:** 736 Tests grün (58 Dateien), inkl. neuer
  `layerGroups`/`railTools`/`accountSelfService`-Tests.
- **Backend:** volle `go test ./...` grün (u. a. `TestAccountPasswordChangeRoleAgnostic`,
  `TestPutMeEmail*`, `TestWhoamiIncludesOwnEmail`); `go vet`, `gofmt`,
  `golangci-lint` (0 issues) sauber.
- Frontend-Bundle (`internal/webui/dist`) neu gebaut/eingebettet.
