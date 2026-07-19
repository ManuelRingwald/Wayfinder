# ADR 0027 — Basiskarte als Entitlement-Layer: der synthetische Scope ist der Grundzustand

- **Status:** **AKZEPTIERT** ✅ (2026-07-19). Betreiber-Entscheidung zu #274 mit
  beiden Weichen: **W1=b** (ohne Admin-Freigabe keine Karte) und **W2=aus**
  (auch mit Freigabe startet die Karte aus; der Lotse schaltet sie bewusst zu).
  Betreiber-Begründung: „Der ASD kommt grundsätzlich ohne dieses Layer aus.
  Es ist nie der Default, sondern ein Nice-to-have."
- **Datum:** 2026-07-19
- **Schnittstellen-relevant:** nein (kein CAT062-/Firefly-Bezug; browser-seitige
  Anzeige-Semantik + ein neuer Feature-Katalog-Key).
- **Bezug:** ADR 0026 (BKG-Basiskarte; die Themes `bkg`/`bkg-dark` bleiben die
  Style-Grundlage), WF2-50 (Feature-Entitlements), ADR 0023 (View-Profile —
  persistieren den Nutzer-Toggle), Issue #274. Register: **FR-UI-036**.

## Kontext

Nach dem Ausbau der OSM/CARTO-Basiskarten (ADR 0026 Nachtrag) ist die
BKG-Karte die einzige Basiskarte. Der Betreiber-Wunsch #274 („BKG als
Layer-Option: Admin gibt frei, Nutzer schaltet zu, Default aus") stellt damit
die Grundsatzfrage: **Was ist der Scope ohne Karte?** Antwort des Betreibers:
der **rein synthetische Radar-Scope** — near-black Grund, nur Tracks,
Sektoren, AoR, Range-Rings. Das Lagebild (CAT062) ist die sicherheitsrelevante
Information; die Karte ist Orientierungs-Kontext.

## Entscheidung

1. **Feature-Key `basemap`** im geschlossenen Katalog (WF2-50, Default deny wie
   alle Keys): Der Admin gibt die Karten-*Option* je Mandant frei.
2. **Nutzer-Toggle „Basiskarte (BKG)"** in der Layer-Sidebar (nur mit
   Entitlement sichtbar), **Default aus** (`layerVisibility.basemap=false`);
   die Wahl persistiert über die bestehenden View-Profile (ADR 0023).
3. **Mechanik ohne Style-Wechsel:** Die Basiskarten-Layer werden beim
   Style-Laden als Menge geschnappschusst (alles, was vor den Wayfinder-
   Overlays im Style ist) und per Sichtbarkeit geschaltet — kein
   `map.setStyle`, keine Overlay-Neuregistrierung. Darunter liegt ein immer
   sichtbarer Near-Black-Grund (`synthetic-background`, #070b12, ADR 0015).
4. **Synthetischer Fallback-Style:** Ist der Basiskarten-Style beim Start
   nicht ladbar (BKG/Spiegel down, kein Cache), startet der Scope mit einem
   eingebauten Minimal-Style (Near-Black + **lokale Glyphs** — Track-Labels
   funktionieren weiter). Konsequenz aus W1=b: Wenn der ASD ohne Karte
   auskommt, darf ein Karten-Ausfall nie das Lagebild kosten.
5. **Ehrliche Grenzen:** (a) Das Entitlement ist eine **Anzeige-Option**, kein
   Daten-Rand — die Karte ist öffentlich, `/basemap/style.json` bleibt
   ungegatet (anders als z. B. Luftraum-Daten). Das Sidebar-Gate ist wie #106
   kosmetisch. (b) Mit hellem Theme `bkg` und ausgeschalteter Karte bleiben
   die hellen Vordergrund-Farben (Paletten sind beim Karten-Aufbau gebacken);
   Empfehlung ist ohnehin der Default `bkg-dark`. (c) **Migrations-Wirkung von
   W1=b:** Bestandsmandanten sehen nach dem Update den synthetischen Scope,
   bis der Admin `basemap` freigibt und der Nutzer zuschaltet — bewusste
   Betreiber-Entscheidung, in INSTALLATION vermerkt.

## Verworfene Alternativen

- **W1=a (Karte ohne Freigabe an, Freigabe schaltet nur den Wahlschalter):**
  sanfter für Bestandssysteme, aber vom Betreiber verworfen — die Karte selbst
  ist das freigebbare Gut.
- **Laufzeit-Style-Wechsel** (`map.setStyle` + Overlay-Neuaufbau): unnötig,
  seit nur noch eine Basiskarte existiert; Sichtbarkeits-Schaltung ist
  strukturell einfacher und risikofrei.
