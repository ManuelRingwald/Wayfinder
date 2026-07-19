package feature

import "sort"

// Key is a typed feature identifier. Typed constants instead of bare strings
// keep gates refactor-safe and let the admin API reject typos before they reach
// the database (the DB column stays free-form TEXT, but writes are validated
// against this catalog).
type Key string

const (
	// STCA — Short-Term Conflict Alert display (ASD-006): the data block reacts
	// to a Firefly-provided conflict flag (I062/340). Entitlement-gated per
	// tenant; Wayfinder never computes STCA itself.
	STCA Key = "stca"
	// MultiFeed — permission to subscribe to more than one sensor feed (WF2-41).
	MultiFeed Key = "multi_feed"
	// PremiumLayers — premium ASD map overlays (e.g. extended aeronautical data).
	PremiumLayers Key = "premium_layers"
	// Airspaces — airspace overlay display (CTR, TMA, restricted, info; ASD-011).
	Airspaces Key = "airspaces"
	// RangeRings — range-ring overlay display (ASD-012).
	RangeRings Key = "range_rings"
	// HistoryDots — track history dots display (ASD-004a).
	HistoryDots Key = "history_dots"
	// VorNdb — VOR/NDB navaid overlay display (ASD-003).
	VorNdb Key = "vor_ndb"
	// Waypoints — waypoint overlay display (ASD-003).
	Waypoints Key = "waypoints"
	// WeatherRadar — DWD weather-radar map overlay display (WX-A, ADR 0016).
	WeatherRadar Key = "weather_radar"
	// QNH — QNH (altimeter setting) header infobox display (WX-B, ADR 0016).
	QNH Key = "qnh"
	// WeatherWarnings — DWD weather-warnings map overlay display (WX-C, ADR 0016).
	WeatherWarnings Key = "weather_warnings"
	// Airport — airport reference-point marker overlay (#192, offline OurAirports).
	Airport Key = "airport"
	// Runways — runway centreline overlay (#192, offline OurAirports).
	Runways Key = "runways"
	// Basemap — the official BKG base map as a switchable layer (#274). Without
	// the grant the scope runs purely synthetic (near-black + overlays); with it
	// the user may enable the map in the layer sidebar (default off). Display
	// option only — the map data itself is public, so there is no server-side
	// data edge to enforce.
	Basemap Key = "basemap"
)

// entry is a catalog record for one feature: a short human-readable label (the
// domain term shown as the heading in the admin entitlement panel) and a
// one-line, plain-language description that helps an administrator understand
// what granting the toggle actually does.
//
// Neither field carries internal document references (requirement IDs like
// "ASD-012", "WF2-41", ADR numbers): those are meaningless to an operator and
// only clutter the UI. Provenance for developers lives in the const doc comments
// above and in docs/requirements — not in this operator-facing copy. The
// TestNoInternalDocRefs guard keeps it that way.
type entry struct {
	Label       string
	Description string
	// Reserved marks a catalogued key that has NO consumer yet — enabling it
	// does nothing (#175). The key stays in the catalog (so it round-trips and
	// stays fail-closed), but the admin panel shows it disabled + "noch nicht
	// aktiv" instead of offering a toggle that silently does nothing (the #114
	// "a switch that visibly does nothing reads as a bug" principle). Turning a
	// reserved key into a real feature is a deliberate act: wire a consumer, then
	// drop this flag.
	Reserved bool
}

// catalog is the closed set of known feature keys with their operator-facing
// label + description. The admin API may only set keys in this set, and
// HasFeature treats any key outside it as fail-closed — so the catalog is the
// single source of truth for "which features exist".
//
// Copy is German to match the admin UI; the labels keep the international ATC
// terms (QNH, STCA, VOR/NDB, Range Rings) that operators already know.
var catalog = map[Key]entry{
	// STCA is reserved: it needs a Firefly conflict item (I062/340) on the wire
	// and an ASD data-block renderer, neither of which exists yet (cross-project).
	STCA:      {Label: "STCA", Description: "Kurzfrist-Konfliktwarnung: markiert im Datenblock, wenn zwei Tracks in Kürze zu nah kommen.", Reserved: true},
	MultiFeed: {Label: "Mehrere Feeds", Description: "Erlaubt dem Mandanten, mehrere Sensor-Feeds gleichzeitig zu abonnieren."},
	// PremiumLayers is reserved: a generic seed placeholder with no defined
	// overlays and no consumer — kept until "premium overlays" actually exist.
	PremiumLayers:   {Label: "Premium-Kartenlayer", Description: "Schaltet zusätzliche, erweiterte Kartenoverlays frei (Premium-Umfang).", Reserved: true},
	Airspaces:       {Label: "Lufträume", Description: "Blendet Luftraumstrukturen ein (CTR, TMA, Sperr- und Infogebiete)."},
	RangeRings:      {Label: "Range Rings", Description: "Konzentrische Entfernungsringe um das Kartenzentrum als Distanzraster."},
	HistoryDots:     {Label: "Positions-History", Description: "Zeigt vergangene Positionen eines Tracks als Punktespur."},
	VorNdb:          {Label: "VOR/NDB", Description: "Blendet VOR-/NDB-Funknavigationsanlagen (Navaids) ein."},
	Waypoints:       {Label: "Waypoints", Description: "Blendet Wegpunkte/Meldepunkte auf der Karte ein."},
	WeatherRadar:    {Label: "Wetterradar (DWD)", Description: "Niederschlagsradar des Deutschen Wetterdienstes als Kartenoverlay."},
	QNH:             {Label: "QNH", Description: "Höhenmesser-Einstellung (QNH) als Infobox in der Kopfzeile."},
	WeatherWarnings: {Label: "Wetterwarnungen (DWD)", Description: "Amtliche Wetterwarnungen des Deutschen Wetterdienstes als Kartenoverlay."},
	Airport:         {Label: "Flughäfen", Description: "Blendet Flughäfen als Referenzpunkt-Marker mit Namen ein."},
	Runways:         {Label: "Runways", Description: "Blendet die Start-/Landebahnen der Flughäfen als Mittellinien ein."},
	Basemap:         {Label: "Basiskarte (BKG)", Description: "Amtliche basemap.de-Hintergrundkarte als zuschaltbares Layer; ohne Freigabe läuft die Lagedarstellung rein synthetisch."},
}

// IsKnown reports whether key is part of the feature catalog.
func IsKnown(key Key) bool {
	_, ok := catalog[key]
	return ok
}

// All returns the known feature keys in a stable (sorted) order — e.g. for the
// admin API / whoami to present the full catalog with each tenant's state.
func All() []Key {
	keys := make([]Key, 0, len(catalog))
	for k := range catalog {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// Describe returns the plain-language description for a known key, or "" if the
// key is not in the catalog.
func Describe(key Key) string { return catalog[key].Description }

// Label returns the short human-readable term (the ATC/domain Fachbegriff) shown
// as the heading for a known key, or "" if the key is not in the catalog.
func Label(key Key) string { return catalog[key].Label }

// Reserved reports whether a known key is catalogued but not yet wired to a
// consumer (#175): enabling it is a no-op, so the admin panel shows it disabled
// with a "noch nicht aktiv" hint. Unknown keys are not reserved (they are simply
// unknown, fail-closed).
func Reserved(key Key) bool { return catalog[key].Reserved }
