# ADR 0018 — OpenAIP: persistenter Cache, fetch-once statt periodischem Refresh

- **Status:** akzeptiert
- **Datum:** 2026-07-03
- **Schnittstellen-relevant:** nein (kein CAT062/065/063-Vertrag betroffen; rein
  Wayfinder-interne Speicher-/Abruf-Architektur der OpenAIP-Aeronautik-Schicht)

## Kontext

Die OpenAIP-Aeronautik-Schicht (Lufträume, Navaids, Wegpunkte — ADR 0004, ONB-6)
lief bisher als **periodischer Refresh**: je Mandant eine Goroutine mit einem
`time.Ticker` (`WAYFINDER_OPENAIP_REFRESH`, Default 24 h), Ergebnis **nur im RAM**
gecacht (`atomic.Pointer`). Das hat zwei Probleme gegen die Prämisse aus ADR 0017
(Informations-Plattform, statische Kontext-Daten, Betreiber-getriebene Aktualität):

1. **Datenverlust bei Redeploy.** Der In-Memory-Cache ist nach jedem Neustart leer
   und wird vollständig neu gezogen — ein Redeploy bedeutet einen Abruf-Sturm gegen
   OpenAIP und ein kurzes „leeres" Kartenbild, bis der erste Refresh durch ist.
2. **Dauer-Polling ohne Nutzen.** Aeronautik-Daten folgen dem **AIRAC-Zyklus**
   (28 Tage). Ein 24-h-Ticker fragt viel häufiger ab, als sich die Daten ändern —
   Last ohne fachlichen Gewinn, und die Aktualisierung ist an eine Wanduhr statt an
   den AIRAC-Stichtag gekoppelt.

Der Betreiber will Aeronautik-Daten **einmal holen und halten** und **bewusst**
aktualisieren (zum AIRAC-Update), nicht im Hintergrund dauer-pollen.

## Entscheidung

1. **Persistenter DB-Cache.** Die gefetchte GeoJSON wird pro Mandant (und für die
   globale Fallback-Ebene) in der Tabelle `aeronautical_cache` gehalten
   (`tenant_id` NULL = global; je `kind` eine Zeile: `geojson`, `feature_count`,
   `fetched_at`). Sie **überlebt Redeploys**.
2. **Hydrate-on-boot ohne Netz.** Beim Start lädt jede Ebene ihren Cache aus der DB
   in den In-Memory-Read-Cache — **kein** OpenAIP-Abruf nötig, das Kartenbild ist
   sofort warm.
3. **Fetch-once / on-demand statt Ticker.** Der periodische Refresh entfällt. Ein
   OpenAIP-Abruf passiert nur noch **ereignisgesteuert**:
   - **Erstbefüllung** beim Boot, wenn ein Schlüssel vorhanden ist, aber **noch
     keine** persistierten Daten existieren;
   - **AOI-Änderung** (die Abfrage-Box ändert sich) → ein Abruf;
   - **expliziter Refresh** (Schlüssel gesetzt/geändert; später die Admin-Refresh-
     Buttons, AERO-2) → ein erzwungener Abruf.
   Jeder erfolgreiche Abruf **persistiert** sofort und aktualisiert `fetched_at`.
4. **`WAYFINDER_OPENAIP_REFRESH` wird obsolet.** Der Wert wird noch geparst
   (kein harter Bruch bei gesetzter Env), aber **nicht mehr verwendet**; die Doku
   markiert ihn deprecated.
5. **Best-effort bleibt (ADR 0004).** Ein fehlgeschlagener Abruf behält den
   Last-Good-Cache (jetzt sogar über Redeploys hinweg), blockiert nie `/ready`,
   liefert nie einen Fehler an den Browser.

## Abgrenzung / Scope

Diese ADR deckt das **Backend-Fundament** (AERO-1): Persistenz + Hydrate +
fetch-once + `fetched_at`/`feature_count` in der Admin-Status-Route. **Nicht**
Teil davon (folgt in AERO-2): der **globale Schlüssel via Platform-Admin-UI** samt
Fetch-all, die **Refresh-Buttons** (global + pro Mandant) und die
Zeitstempel-**Anzeige** in der UI. Der **AIRAC-Kalender + Change-Impact** ist
AERO-3.

## Konsequenzen

- **Positiv:** kein Datenverlust bei Redeploy; kein nutzloses Dauer-Polling;
  Aktualität ist betreiber-/AIRAC-getrieben; das Kartenbild ist nach dem Start
  sofort da.
- **Negativ / ehrliche Grenze:** Ändert sich die Abfrage-Box **außerhalb** eines
  laufenden Servers (z. B. ein geänderter Default über eine Env), erkennt der Boot
  das nicht automatisch — die persistierten Daten passen dann evtl. nicht exakt zur
  neuen Box, bis ein expliziter Refresh (AERO-2) läuft. Für den Regelbetrieb
  (AOI-Änderung über die laufende Admin-UI) ist das abgedeckt.
- **Migration:** additive Tabelle `00017`; kein Rückbau bestehender Spalten. Der
  per-Mandant-Schlüssel (`tenants.openaip_api_key`, Migration `00009`) bleibt
  unverändert.

## Bezug

- **ADR 0004** — OpenAIP als Aeronautik-Quelle, best-effort.
- **ADR 0017** — Connected-by-default / Informations-Plattform (die Prämisse, aus
  der „statisch halten, bewusst aktualisieren" folgt).
- **ONB-6** — per-Mandant-Fetch mit eigenem Schlüssel + globalem Fallback.

## Nachtrag (AERO-2, 2026-07-03)

AERO-2 ist umgesetzt: der **globale** Schlüssel ist nun zur Laufzeit über die
Platform-Admin-UI setzbar, **verschlüsselt** in `platform_settings`
(`00018`, AES-256-GCM). **Verschlüsselungs-Entscheidung — Option A (strikt):** ohne
`WAYFINDER_SECRET_KEY` ist die UI-Set-Route deaktiviert (`503`); es wird **nie** ein
Klartext-Geheimnis gespeichert, konsistent mit den Feed-Credentials (ORCH-2c). Die
Env `WAYFINDER_OPENAIP_API_KEY` bleibt der schlüssellose Fallback; ein UI-Schlüssel
gewinnt und greift sofort. Zusätzlich **Fetch-all** (beim Setzen und per Button) und
ein **Per-Mandant-Refresh**. Nicht abgedeckt: die per-Mandant-Schlüssel bleiben
Klartext (mögliche Folgearbeit); der **AIRAC-Kalender + Change-Impact** ist AERO-3.
