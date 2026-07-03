# AERO-3 — AIRAC-Kalender + Change-Impact

> Drittes, optionales Häppchen der OpenAIP-Reihe (ADR 0018). Baut auf AERO-1
> (persistenter Cache) und AERO-2 (globaler Schlüssel via UI) auf.

## Fachlich

Zwei Betreiber-Hilfen für **proaktive Kundeninfo** rund um Aeronautik-Updates:

1. **AIRAC-Kalender** — zeigt den aktuellen 28-Tage-Zyklus und den **nächsten
   Stichtag**, damit der Admin den OpenAIP-Refresh bewusst um den AIRAC-Wechsel
   planen kann.
2. **Change-Impact** — nach jedem Abruf sieht der Admin je Ebene, **was sich
   geändert hat** („Luftraum 142 → 145, +5/−2"). So ist erkennbar, ob ein
   AIRAC-Update den eigenen Ausschnitt überhaupt betrifft.

## Technisch

- **`pkg/airac` (deterministisch, offline):** `Cycle(t) → {ident YYNN, effective,
  next_ident, next_effective, days_until_next}`, verankert an 2020-01-02 = „2001",
  28-Tage-Schritte; die YYNN-Sequenz setzt pro Kalenderjahr zurück. Kein externer
  Abruf. Endpunkt `GET /api/admin/airac`.
- **Change-Impact im `refreshAll`:** vor dem Überschreiben liegt der alte Stand noch
  im In-Memory-Cache; `diffCollections(old, new)` bildet je `kind` eine
  `ChangeSummary{PrevFeatureCount, Added, Removed, HasPrev}`. **Keying über
  Inhalts-Hash** (SHA-256 des kanonischen Feature-JSON) — robust, ohne Annahme über
  eine stabile OpenAIP-Feature-ID. **Persistenz:** Migration `00019` erweitert
  `aeronautical_cache` um `prev_feature_count`/`added`/`removed` (nullable =
  Erstbefüllung); `CacheStore.Save` trägt das Summary; `AeroCacheRepo.Changes` +
  `GET /api/admin/tenants/{id}/openaip/changes`.
- **Frontend:** AIRAC-Zeile in der „OpenAIP"-Plattform-Sektion; Change-Chips je
  Ebene auf der Mandanten-Detailseite.

## Ehrliche Grenze (wichtig)

Der **Count-Delta** (142 → 145) ist **exakt**. Die `added`/`removed`-Zahlen sind
**Churn** über den Inhalts-Hash — ein In-Place-Edit zählt als −1/+1. Eine
**namentliche** Zuordnung („genau diese Flugplätze") ist **bewusst nicht** enthalten:
sie bräuchte eine Live-Verifikation der OpenAIP-Feature-Identität, die in der
Entwurfs-Umgebung nicht möglich ist (Egress gesperrt). Blind gebaut würde sie
falsche Positive riskieren; der content-hash-basierte Churn ist dagegen immer
korrekt.

## Schnittstellen-Wirkung

**Keine** (kein CAT062/065/063). Reine Wayfinder-interne Admin-/Speicher-Ebene.

## Gates

- `go build/vet/gofmt` grün; `go test ./...` **+ `-race`** grün (`pkg/airac`
  Anker/Reset/28-Tage; `diffCollections` + `refreshAll`-Summary; real-PG
  `Changes`-Spalten; adminapi AIRAC + changes 200/404). golangci-lint 0 issues.
- vitest grün inkl. AERO-3-Store-Block; `dist` neu gebaut.

## Abschluss

Damit ist die OpenAIP-„Connected-by-default"-Reihe (AERO-1/2/3) **komplett** — und
mit ihr die gesamte connected-by-default-Umstellung (CBD-1/2/3 + AERO-1/2/3).
