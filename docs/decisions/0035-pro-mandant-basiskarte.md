# ADR 0035 — Pro-Mandant-Basiskarte (Theme + Style) auf der Kartendaten-Config-Plane

- **Status:** akzeptiert (2026-07-22)
- **Kontext-Issues:** Epic #307 (Kartendaten live konfigurierbar), Betreiber-Wunsch
  „Kartendaten pro Mandant"; baut auf ADR 0033 (K0-`mapconfig`) und ADR 0026
  (BKG-Basiskarte / Style-Proxy) auf.

## Kontext

Mit Epic #307 wurde die Kartendaten-Konfiguration (Basiskarte, Wetter, Radar-
Abdeckung, Aeronautik) im Admin **live** einstellbar — aber **global**
(`platform_settings`, ADR 0033). Der Betreiber möchte, dass jeder **Mandant**
seine **eigene Basiskarte** bekommt (Look/Theme, ggf. eigener BKG-Style), passend
zum bereits mandanten-spezifischen Zuschnitt (AOI, Freigaben).

**Hybrid-Scope (bewusst):** Nur die **Basiskarte** (Theme + Style-URL) wird
pro-Mandant. Wetter-Upstream-URLs und die **physischen** Radar-Sensoren sind
Infrastruktur und bleiben **global**; *ob* ein Mandant ein Overlay sieht, ist
weiterhin das bestehende **Entitlement**. So wird keine Infrastruktur pro Mandant
dupliziert.

**Architektur-Feinheit, die den Umfang bestimmt:** Die *tatsächliche Optik* der
Basiskarte (hell/dunkel-Transform + welcher Style) entsteht **einmal global** im
Style-Proxy (`/basemap/style.json` → ein `basemap.Service`, ein Fetch, ein
Rewrite/Dark-Transform, ein Cache, ADR 0026). Das `theme` aus `/api/map-config`
steuert im Frontend nur die **Vordergrund-Palette**. Ein reines Pro-Mandant-`theme`
würde also nicht reichen (helle Labels über dunkler Karte). Für eine **echte**
Pro-Mandant-Basiskarte muss der **Style-Proxy mandanten-fähig** werden.

## Entscheidung

1. **Getrennte Pro-Mandant-Speicherung.** Neue Tabelle **`tenant_map_settings`**
   `(tenant_id, key, value)` — **getrennt** von der globalen `platform_settings`,
   sodass globale Werte (inkl. des **versiegelten** OpenAIP-Keys) unberührt
   bleiben. `ON DELETE CASCADE`. Repo `store.TenantMapSettingsRepo`.

2. **Drei-Stufen-Auflösung.** `mapconfig.TenantSetting` löst
   **Mandant-Override ?? globaler Override (`platform_settings`) ?? Env-Default**.
   `tenantID 0` (mandantenloser Plattform-Admin, ONB-3) / nil-Store → global-only;
   ein Store-Fehler degradiert auf den globalen Wert (nie den Read brechen); ein
   leerer Wert = Reset. **Isolation ist garantiert und getestet** (Mandant A ändert
   nie Mandant B).

3. **Mandanten-fähiger Style-Proxy.** `basemap.Service` bekommt einen **gekeyten
   Varianten-Cache** je `(styleURL, dark)`. Die **Default-Variante** (die global
   konfigurierte) behält ihre starke **Last-Good-Garantie** (unveränderter Pfad);
   nur echte Abweicher landen im Varianten-Cache (jede Variante mit eigenem
   Last-Good, Bounded auf 32 Varianten). Neue Methode `StyleJSONFor(ctx, url, dark)`.

4. **Mandanten-effektive Endpunkte.** `/basemap/style.json` und `/api/map-config`
   laufen hinter der **Tenant-Middleware** (`tenantMW ∘ pwGate ∘ impReadMW`, wie
   die Wetter-/Aeronautik-Endpunkte). Sie lesen den effektiven Mandanten (inkl.
   Impersonations-Ziel) und liefern dessen Theme + Style-Bytes.
   `/basemap/style.json` ist **`Cache-Control: private`** (gleiche URL, andere
   Bytes je Mandant). Der Frontend holt den Style same-origin per `fetch` →
   Session-Cookie wird mitgesendet; bei 401/Ausfall greift der bestehende
   synthetische Fallback-Scope.

5. **Admin-Schreibpfad.** `GET/PUT /api/admin/tenants/{tenantID}/mapdata/basemap/
   {theme,style-url}` (hinter `RequireRole(admin)`): Theme gegen `bkg`/`bkg-dark`,
   Style-URL via `ValidateFetchURL` (SSRF) validiert; leerer Wert = Reset auf
   global.

## Konsequenzen

- **Positiv:** Jeder Mandant kann eine **echt eigene** Basiskarte (hell/dunkel +
  Style) bekommen; ohne Override verhält sich alles exakt wie zuvor (12-Factor
  bleibt gültig). Der Track-Rechenpfad und dessen Determinismus sind unberührt
  (reine Konfig-/Darstellungs-Ebene).
- **Sicherheit:** Style-/URL-Overrides sind admin-gesetzt und SSRF-geprüft; der
  Style-Proxy hinter Auth verhindert unauthentifiziertes Egress; `private`-Cache
  verhindert Cross-Tenant-Cache-Vermischung. Secrets bleiben versiegelt.
- **Ehrliche Grenzen:** Der **Glyph-Proxy-Fallback** (`/glyphs`) bleibt global —
  alle unterstützten BKG-Styles teilen denselben Glyph-Host, die selbst-gehosteten
  Glyphs decken die Labels; das rechtfertigt den einen gemeinsamen Fallback. Der
  Varianten-Cache ist auf 32 `(url, dark)`-Kombinationen begrenzt (darüber
  ungecacht, geloggt) — real ist die Menge winzig. Kein WebGL-Harness →
  Backend unit-getestet, optische Abnahme durch den Betreiber.

## Umsetzung (Häppchen)

- **T1** (#326): Fundament — `tenant_map_settings` + Repo + `mapconfig.TenantSetting`
  + Isolations-Tests.
- **T2** (dieses ADR): mandanten-fähiger Style-Proxy (`StyleJSONFor`,
  Varianten-Cache) + mandanten-effektive `/basemap/style.json` + `/api/map-config`
  + Admin-Schreibpfad + Tests.
- **T3** ✅: Mandanten-Detail (`AdminTenantDetail.vue`) als **Tabs** (Sicht |
  Freigaben | Kartendaten) + Pro-Mandant-Basiskarte-Editor im Kartendaten-Reiter
  (Theme + Style-URL, „überschrieben/Standard"-Chip, „Auf Standard"-Reset), ruft
  die T2-Admin-Endpunkte. Der bestehende globale Speichern-Knopf (Sicht +
  Freigaben) bleibt; der Kartendaten-Reiter speichert eigenständig.
