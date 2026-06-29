# ORCH-2a — Tracker-Instanz-Abstraktion (`pkg/instance`)

> Erstes Häppchen von **ORCH-2** (ADR 0012 §4). Legt die backend-agnostische
> Abstraktion an, mit der Wayfinder **pro Feed eine Firefly-Instanz** betreibt —
> ohne schon einen konkreten Runner (Docker/K8s) oder die getrennte
> Control-Plane zu bauen.
>
> **Lieferumfang ORCH-2a:** `Backend`-Interface, generische `Spec` +
> `SpecFromFeed`-Ableitung, `MemoryBackend` (Test-Double/Dev-Platzhalter) + Tests.
> **Noch nicht** Teil: Docker-Adapter (ORCH-2b), getrennter Control-Plane-Prozess
> + Secret-Auflösung (ORCH-2c), Reconciler (ORCH-3).

## Fachlicher Hintergrund

ORCH-1 gab dem Feed eine Quell-Konfiguration; gestartet wird daraus noch nichts.
Damit „Feed zuweisen ⇒ passende Firefly-Instanz startet" möglich wird, braucht es
eine **Abstraktion**, die aus einem Feed eine laufende Tracker-Instanz macht —
zuerst per Docker (ORCH-2b), später K8s (ORCH-6). ORCH-2a baut den
**Architektur-Kern** dieser Abstraktion, bewusst **ohne** Spawn-Abhängigkeit,
damit Control-Plane und Reconciler darauf unit-testbar aufsetzen können.

## Was umgesetzt wurde (`pkg/instance`)

### `Backend`-Interface
`Start(ctx, Spec)` / `Stop(ctx, feedID)` / `Status(ctx, feedID)` — **idempotent**
und nebenläufigkeitssicher, Instanz-Identität = `feed_id`. Injiziert (wie die
`feedmanager.Factory`), damit nichts Echtes gestartet werden muss, um den
Lebenszyklus zu testen. Vertrag explizit dokumentiert: gleicher Spec → No-op,
geänderter Spec → Re-Apply; Stop/Status auf unbekanntem Feed sind harmlos.

### `Spec` + `SpecFromFeed`
`Spec` ist die **generische, Firefly-agnostische** Launch-Spezifikation:
`FeedID`/`FeedName`, Multicast-Endpoint (`Group`/`Port`), grobe `Coverage`-BBox,
`Sources` (die Quell-Liste) und `SecretRefs` (**Referenzen**, keine Werte).
`SpecFromFeed` ist rein: es kopiert die strukturierte Konfig und sammelt die
**distinkten, sortierten** `cred_ref`-Handles — die deterministische Reihenfolge
macht zwei gleiche Quell-Listen zu gleichen Specs (der Reconciler vergleicht Specs,
um Drift zu erkennen, ORCH-3).

> **Bewusst aus der `Spec` herausgehalten:** die genauen Firefly-**Eingangs**-Env-
> Namen (`FIREFLY_SOURCES` o. Ä.). Dieser Eingangs-Kontrakt ist cross-project
> (ORCH-5, Firefly-Ball) und noch nicht ratifiziert. ORCH-2a bleibt deshalb
> strukturiert; die Übersetzung in konkrete Env passiert erst im Backend-Adapter
> (ORCH-2b), abgestimmt mit ORCH-5. Bis dahin kann der Adapter gegen Fireflys
> Szenen-Modus als Platzhalter entwickelt werden (ADR 0012, „Ehrliche Grenze").

### `MemoryBackend`
Ein In-Memory-`Backend`: merkt sich den Spec je Feed und meldet Status, **spawnt
aber nichts**. Dient als Test-Double für Control-Plane/Reconciler und als
Einzelhost-Dev-Platzhalter bis zum Docker-Adapter. Idempotentes `Start`
(gleicher Spec No-op, geänderter ersetzt), `Stop` (no-op auf unbekannt),
optionaler `startHook` zum Testen des Fehlerpfads (→ `StatusFailed`).

## Sicherheits-Betrachtung

- **Privilegien-Sprung isoliert (Vorbereitung):** Das `Backend` ist so geschnitten,
  dass es später in einer **getrennten, least-privilege** Control-Plane läuft
  (ORCH-2c) — nie im browser-zugewandten Prozess (ADR 0012 §6). ORCH-2a führt die
  Abstraktion ein; die Prozess-Trennung folgt in 2c.
- **Secret-Referenz statt Wert (NFR-SEC-004):** `Spec.SecretRefs` trägt nur die
  `cred_ref`-Handles, nie die Secret-Werte. Auflösung und Übergabe an den Backend
  beim Launch sind ORCH-2c.
- **Keine Schnittstellen-Wirkung** auf CAT062.

## Tests

`pkg/instance/instance_test.go`: `SpecFromFeed` (Ableitung; Secret-Refs distinkt +
sortiert; Dedup; keine Secrets/Coverage), `Spec.Validate`/`Endpoint`,
`MemoryBackend`-Lebenszyklus (Start/Status/Stop, Idempotenz, Re-Apply),
Reject-invalid-Spec, Start-Hook-Fehler → `StatusFailed` + Recovery, sowie ein
nebenläufiger Start/Stop/Status-Test unter `-race`.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-002** (Instanz-Abstraktion), **NFR-SEC-004**
(Secret-Referenz statt Wert).

## Nächste Häppchen

- **ORCH-2b** — Docker-`Backend`-Adapter: `Spec` → Container-Config/Env; Fake-
  Docker-Client unit-getestet, echter Daemon out-of-sandbox.
- **ORCH-2c** — getrennter Control-Plane-Prozess (Least-Privilege), Multicast-
  Allokation, Secret-Auflösung beim Launch.
