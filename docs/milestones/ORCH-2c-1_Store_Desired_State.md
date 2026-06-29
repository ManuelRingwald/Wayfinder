# ORCH-2c (1/3) — Store-gestützter `DesiredState`-Adapter

> Erstes Stück von **ORCH-2c** (Control-Plane, ADR 0012). Schließt den
> Reconciler-Kern (ORCH-3) an **echte Katalog-Daten** an: das Soll der
> Orchestrierung wird aus Feeds + Abos + Quell-Konfig abgeleitet.
>
> **Lieferumfang:** Store-Abfrage `ListSubscribedFeeds` + `orchestrator.StoreDesiredState`-
> Adapter + Tests. **Noch nicht** Teil: der getrennte Control-Plane-Prozess, die
> Secret-Auflösung beim Launch und der Änderungs-Trigger (ORCH-2c, 2–3/3).

## Fachlicher Hintergrund

Der Reconciler (ORCH-3) ist gegen ein injiziertes `DesiredState`-Interface
gebaut und bisher nur gegen Fakes getestet. Damit „Feed zuweisen ⇒ Instanz
startet" real wird, muss das **Soll** aus dem Katalog kommen: genau die Feeds mit
mindestens einem Abo sollen laufen (ADR 0012 §5), jeweils konfiguriert aus ihrer
Quell-Liste (ORCH-1).

## Was umgesetzt wurde

### `SubscriptionRepo.ListSubscribedFeeds` (`pkg/store`)
Liefert die **distinkten** Feed-Zeilen mit ≥ 1 Abo (`JOIN subscriptions`,
`DISTINCT`, nach id sortiert). Ein Feed ohne Abonnenten fällt heraus — sein
Tracker soll abgebaut werden. `DISTINCT` kollabiert den Fall mehrerer Abonnenten
pro Feed.

### `orchestrator.StoreDesiredState` (`pkg/orchestrator`)
Implementiert `reconciler.DesiredState` aus dem Katalog: listet die abonnierten
Feeds, liest je Feed die Quell-Konfig (`FeedRepo.GetSourceConfig`) und baut über
`instance.SpecFromFeed` einen Spec. Über zwei schmale Interfaces
(`SubscribedFeedLister`, `SourceConfigReader`) an die Repos gekoppelt → mit Fakes
unit-testbar. **Spawnt nichts, löst keine Secrets auf** — übersetzt nur Katalog-
Zeilen in Launch-Specs (der Spec trägt nur Secret-*Referenzen*, ADR 0012 §6).

## Robustheits-/Sicherheits-Betrachtung

- **Kein Teil-Soll bei Lese-Fehler:** Scheitert das Lesen *einer* Quell-Konfig,
  bricht `DesiredSpecs` den ganzen Lauf ab statt eine Teilmenge zu liefern — sonst
  würde der Reconciler Instanzen abbauen, die er nur nicht *lesen* konnte. Der
  Reconciler behandelt einen `DesiredState`-Fehler als „diesen Zyklus nichts tun"
  und versucht es beim nächsten Tick erneut.
- **Control-Plane-Trennung (Vorbereitung):** Dieser Adapter ist reine
  Lese-Ableitung; er gehört in die getrennte, least-privilege Control-Plane
  (ORCH-2c, 2/3) — nie in den browser-zugewandten Prozess.
- **Keine Schnittstellen-Wirkung** auf CAT062.

## Tests

- `pkg/orchestrator/desired_test.go`: Spec-Aufbau aus abonnierten Feeds (Endpoint,
  Coverage, Secret-Referenz nur als Handle), leeres Soll ohne Abos, Abbruch bei
  Listen-Fehler, Abbruch bei Quell-Konfig-Fehler.
- `pkg/store/...::TestIntegrationSubscriptionRepoIsolation` (real-PG): `ListSubscribedFeeds`
  ist DISTINCT (zweiter Abonnent dupliziert nicht) und ein Feed fällt heraus,
  sobald sein letzter Abonnent geht.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-003** (Reconciler — Implementierung/Tests um den
Store-Adapter ergänzt).

## Nächste Stücke (ORCH-2c)

- **2/3 — getrennter Control-Plane-Prozess:** eigener Einstieg/Subcommand, der
  Store-`DesiredState` + Reconciler + Backend verdrahtet, Least-Privilege, von
  Browser-/WS-Prozess getrennt.
- **3/3 — Secret-Auflösung + Änderungs-Trigger:** `cred_ref` → Wert beim Launch
  (Secret-Speicher), Reconcile-Anstoß bei Feed-/Abo-/Quell-Änderung statt nur
  periodisch.
