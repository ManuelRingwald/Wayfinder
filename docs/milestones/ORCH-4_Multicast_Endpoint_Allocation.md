# ORCH-4 — Automatische Multicast-Endpoint-Allokation

> Nimmt dem Betreiber die Zuteilung der Multicast-Adressen ab: legt ein Admin
> einen Feed **ohne** Endpoint an, vergibt der Server kollisionsfrei den nächsten
> freien — **eine Gruppe pro Feed**. Manueller Override bleibt.

## Fachlicher Hintergrund

Jede orchestrierte Firefly-Instanz sendet auf **ihrem eigenen** Multicast-Endpoint
(Gruppe + Port). Bisher trug der Admin beides **manuell** je Feed ein. Im
Mehr-Feed-Betrieb ist das Handarbeit *und* fehleranfällig: zwei Feeds auf
derselben `(Gruppe, Port)` **übersprechen sich auf der Leitung** — ein Mandant
sähe fremde Datagramme schon auf Netzebene, **bevor** das Scoped-Fan-out
(NFR-SEC-003) überhaupt greift. ORCH-4 garantiert kollisionsfreie Endpoints und
schiebt die Isolation auf die **Netzebene** runter (eine Gruppe je Feed → IGMP/
Switches prunen pro Feed; ein Empfänger sieht nur Pakete der beigetretenen
Gruppen). Defense-in-depth.

## Was umgesetzt wurde

### Migration `00013` — Race-Backstop
`UNIQUE(multicast_group, port)` (`feeds_endpoint_unique`). Macht einen doppelten
Endpoint **unmöglich** und ist die Wahrheit, gegen die der Allocator nebenläufig
arbeitet. (Setzt voraus: keine bestehenden Duplikate — bei Ein-Feed-Setups gegeben.)

### Allocator (`pkg/store/feed_alloc.go`)
`MulticastPool{Base24, OctetMin, OctetMax, Port}` beschreibt den Adressraum: eine
Gruppe pro Feed, letztes Oktett variiert in `[OctetMin, OctetMax]`, fester Port.
`DefaultMulticastPool` = `239.255.0.1 .. .254 : 8600` (~254 Feeds; `.0`/`.255`
ausgespart). `CreateAutoAllocated`:

1. liest die belegten Oktette im `/24` am Port (`lowestFreeOctet`),
2. nimmt das **niedrigste freie** und fügt ein,
3. gewinnt ein paralleler Create dasselbe (`feeds_endpoint_unique` →
   `ErrEndpointTaken`), wird **neu gelesen und retried** (bounded durch die
   Pool-Größe),
4. voller Pool → `ErrPoolExhausted`.

Endpoints **außerhalb** des Pools (Legacy/manuell auf anderer Gruppe) verkleinern
den Pool nicht. `FeedRepo.Create` mappt die Endpoint-Unique-Verletzung (PG `23505`
auf genau diesem Constraint) auf `ErrEndpointTaken`, sodass der manuelle Pfad einen
sauberen 409 liefert. `WithMulticastPool` validiert den Pool und fällt bei
Unsinn auf den Default zurück.

### Admin-API (`pkg/adminapi`)
`POST /api/admin/feeds`: `multicast_group`/`port` **optional**.
- beide weggelassen ⇒ `CreateAutoAllocated` (Auto-Vergabe, Endpoint zurückgegeben,
  live beigetreten);
- beide gesetzt ⇒ manueller Override (validiert; Kollision → **409**);
- nur eines → **400** („beide oder keines");
- Pool erschöpft → **507**.

### Config (`cmd/wayfinder`, 12-Factor)
`WAYFINDER_FEED_GROUP_BASE` (Default `239.255.0`), `WAYFINDER_FEED_PORT` (`8600`),
`WAYFINDER_FEED_OCTET_MIN`/`_MAX` (`1`/`254`) → `feedPool()` → `WithMulticastPool`
auf dem an die Admin-API gereichten `FeedRepo`. Auf /16 weitbar; ungültige Kombi →
Default-Pool.

### Frontend (`AdminFeeds.vue`, `stores/admin.js`)
Schalter „Multicast-Endpoint automatisch zuweisen" (Default **an**) im
Anlegen-Dialog; bei „an" werden Gruppe/Port nicht gezeigt und weggelassen, der
zugewiesene Endpoint steht danach in der Liste. Manuelle Eingabe als
Aufklapp-Option. Die Store-Action mappt 409 (Name **oder** Endpoint, distinkte
Texte) und 507 (Pool erschöpft) auf freundliche Meldungen. Gating kosmetisch.

## Sicherheits-/Robustheits-Betrachtung

- **Kollisionsfreiheit garantiert** durch den DB-Constraint, nicht durch ein
  vorab gelesenes „frei" — der Allocator ist gegen Races korrekt (retry).
- **Netz-Isolation als defense-in-depth:** eine Gruppe je Feed → Pro-Feed-IGMP-
  Pruning, zusätzlich zum Scoped-Fan-out.
- **Keine CAT062-Schnittstellen-Wirkung;** Migration additiv (nur ein Constraint).

## Tests

- `pkg/store/feed_alloc_test.go`: `TestMulticastPoolValidate`/`…Group` (Unit);
  `TestIntegrationFeedAutoAllocate` (**real-PG**): Sequenz-Vergabe, Skip eines
  manuell belegten Endpoints, manueller Dup → `ErrEndpointTaken`, Pool-Erschöpfung,
  Endpoint außerhalb des Pools unberührt.
- `pkg/adminapi/adminapi_feeds_test.go`: Auto-Vergabe (kein Endpoint gesendet →
  201 mit Endpoint), nur eines gesetzt → 400, Kollision → 409, Pool erschöpft → 507.
- `frontend/.../admin.test.js`: Auto-Allokation (kein Endpoint im Payload),
  409-Endpoint-Text, 507-Pool-Text.
- Gegen ein echtes PostgreSQL verifiziert (Migration 00013 + Allocator); alle
  Gates grün, 171 Vitest-Tests grün, Production-Build erfolgreich.

## Rückverfolgbarkeit

Anforderungs-Register: **FR-ORCH-005** (Automatische Multicast-Endpoint-Allokation).

## Nächste Stücke

- **Container-Injection (ORCH-5, cross-project):** der orchestrierte Firefly-
  Container bekommt seine Live-Quellen + aufgelösten Credentials — geblockt auf
  den Quell-Eingangs-Kontrakt von Firefly
  ([Firefly #35](https://github.com/manuelringwald/firefly/issues/35)).
