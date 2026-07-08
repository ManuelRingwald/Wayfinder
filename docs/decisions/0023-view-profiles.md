# ADR 0023 — View-Profile (persönliche Anzeige-Profile pro Nutzer)

- **Status:** akzeptiert (2026-07-08)
- **Kontext-Tags:** ASD-Sicht, Per-Nutzer-Präferenzen, Multi-Tenant, kein CAT062-Bezug

## Kontext

Ein Lotse arbeitet je nach Aufgabe mit unterschiedlichen Kartenbildern (z. B.
„Approach" mit Range Rings und wenig Airspace vs. „Überblick" mit allen Layern).
Bisher muss er die Anzeige-Toggles (`asd`-Store: Layer-Sichtbarkeit,
Airspace-Gruppen, Range-Ring-Config, History-Dauer, FL-Filter, Basiskarte) bei
jedem Login und Aufgabenwechsel neu setzen. Gewünscht ist, dass er **bis zu drei**
persönliche Ansichten **benennen, speichern, jederzeit abrufen** und eine als
**Default beim Login** setzen kann.

Es gibt bereits `view_configs` (Tabelle) mit Tenant-Default + **einem**
Pro-Nutzer-Override. Das ist aber **Karten-Rahmung** (Zentrum/Zoom/AOI/ICAO/QNH/
AoR), **admin-getrieben**, und auf genau eine Zeile pro Nutzer beschränkt
(partieller Unique-Index). Es passt semantisch nicht auf „mehrere benannte
persönliche Anzeige-Presets".

## Entscheidung

**Eigener, getrennter Per-Nutzer-Präferenz-Store** — nicht `view_configs`
umbauen.

1. **Neue Tabelle `user_view_profiles`** (Migration 00022): `id, user_id
   (FK ON DELETE CASCADE), name, settings JSONB, is_default, created_at,
   updated_at`.
2. **`settings` ist ein OPAQUE JSON-Objekt** der Frontend-Anzeige-Toggles. Das
   Backend **speichert und liefert es verbatim** und interpretiert es **nie** —
   ein neuer Toggle im Frontend braucht **keine** Migration. Das hält das Backend
   von der genauen UI-Toggle-Menge entkoppelt (vorwärtskompatibel).
3. **Umfang = nur Anzeige-Präferenzen** (Betreiber-Entscheidung 2026-07-08,
   „Option A"): Layer-Sichtbarkeit, Airspace-Gruppen, Range-Ring-Config
   (an/aus/Anzahl/Abstand), History-Dauer, FL-Filter, Basiskarte (dark/OSM).
   **Nicht** Karten-Zentrum/Zoom — die operative Sektor-Rahmung bleibt von
   `view_configs`/AOI getrieben. Profile (Anzeige) und View-Config (Rahmung) sind
   **orthogonal** und mischen nie.
4. **Grenzen als Invarianten im Store erzwungen** (nicht nur UI): **max. 3
   Profile pro Nutzer** (Transaktion + per-Nutzer Advisory-Lock gegen Races) und
   **höchstens ein Default** (partieller Unique-Index `WHERE is_default`).
5. **Streng per-Nutzer gescopt.** Alle Operationen laufen gegen `user_id` aus der
   Session (nie aus dem Request-Body); jede Query ist `WHERE id AND user_id`, ein
   fremdes Profil ergibt `ErrNotFound` — keine Cross-User-Leckage. Multi-Tenant
   ist implizit sicher (Nutzer gehört zu genau einem Tenant), Auth ist immer an.
6. **Apply-on-Login (Präzedenz):** Das Default-Profil wird **nach** dem
   Karten-Init angewandt und setzt nur die Anzeige-Toggles — **zusätzlich** zur
   bestehenden Tenant/User-Karten-Rahmung, ohne diese zu überschreiben. Kein
   Default-Profil → heutiges Verhalten (Store-Defaults).

## Konsequenzen

**Positiv**
- Klare Trennung der Zuständigkeiten (Rahmung vs. Anzeige); `view_configs`
  unangetastet.
- Vorwärtskompatibel: neue Anzeige-Toggles ohne Schema-Änderung.
- Sicherheit als Store-Invariante (Cap, Single-Default, Ownership), nicht nur UI.

**Negativ / Grenzen**
- Das Backend validiert `settings` nur strukturell (gültiges JSON-Objekt,
  Größenobergrenze) — **nicht** die einzelnen Schlüssel. Ein veralteter/unbekannter
  Toggle wird vom Frontend beim Anwenden ignoriert (tolerant).
- **Kein** Karten-Ausschnitt im Profil (bewusst, Option A). Falls später
  gewünscht, additive Erweiterung.

## Umsetzung (Häppchen)

1. **VP-1** — Store: Migration `00022_user_view_profiles.sql` + `ViewProfileRepo`
   (List/Create/Update/Delete/SetDefault/GetDefault), Cap/Single-Default/
   Isolation getestet (Integration). **(dieser Schritt)**
2. **VP-2** — User-gescopte REST-API (`/api/view-profiles`, hinter `tenantMW`).
3. **VP-3** — Frontend-Store + reine `captureSettings`/`applySettings`.
4. **VP-4** — UI-Umschalter + „Ansicht speichern"-Dialog.
5. **VP-5** — Apply-on-Login des Default-Profils.

**Stand 2026-07-08:** VP-1…VP-5 **umgesetzt** — das Feature ist komplett
(FR-PROFILE-001…005).

## Schnittstellen-Wirkung

**Keine** am CAT062-Draht-Vertrag. Reiner interner Per-Nutzer-Präferenz-Pfad.
