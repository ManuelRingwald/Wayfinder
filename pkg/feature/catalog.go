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
}

// catalog is the closed set of known feature keys with their operator-facing
// label + description. The admin API may only set keys in this set, and
// HasFeature treats any key outside it as fail-closed — so the catalog is the
// single source of truth for "which features exist".
//
// Copy is German to match the admin UI; the labels keep the international ATC
// terms (QNH, STCA, VOR/NDB, Range Rings) that operators already know.
var catalog = map[Key]entry{
	STCA:            {Label: "STCA", Description: "Kurzfrist-Konfliktwarnung: markiert im Datenblock, wenn zwei Tracks in Kürze zu nah kommen."},
	MultiFeed:       {Label: "Mehrere Feeds", Description: "Erlaubt dem Mandanten, mehrere Sensor-Feeds gleichzeitig zu abonnieren."},
	PremiumLayers:   {Label: "Premium-Kartenlayer", Description: "Schaltet zusätzliche, erweiterte Kartenoverlays frei (Premium-Umfang)."},
	Airspaces:       {Label: "Lufträume", Description: "Blendet Luftraumstrukturen ein (CTR, TMA, Sperr- und Infogebiete)."},
	RangeRings:      {Label: "Range Rings", Description: "Konzentrische Entfernungsringe um das Kartenzentrum als Distanzraster."},
	HistoryDots:     {Label: "Positions-History", Description: "Zeigt vergangene Positionen eines Tracks als Punktespur."},
	VorNdb:          {Label: "VOR/NDB", Description: "Blendet VOR-/NDB-Funknavigationsanlagen (Navaids) ein."},
	Waypoints:       {Label: "Waypoints", Description: "Blendet Wegpunkte/Meldepunkte auf der Karte ein."},
	WeatherRadar:    {Label: "Wetterradar (DWD)", Description: "Niederschlagsradar des Deutschen Wetterdienstes als Kartenoverlay."},
	QNH:             {Label: "QNH", Description: "Höhenmesser-Einstellung (QNH) als Infobox in der Kopfzeile."},
	WeatherWarnings: {Label: "Wetterwarnungen (DWD)", Description: "Amtliche Wetterwarnungen des Deutschen Wetterdienstes als Kartenoverlay."},
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
