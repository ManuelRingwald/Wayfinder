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
)

// catalog is the closed set of known feature keys with human-readable
// descriptions. The admin API may only set keys in this set, and HasFeature
// treats any key outside it as fail-closed — so the catalog is the single
// source of truth for "which features exist".
var catalog = map[Key]string{
	STCA:          "Short-Term Conflict Alert display (ASD-006)",
	MultiFeed:     "Subscribe to multiple sensor feeds (WF2-41)",
	PremiumLayers: "Premium ASD map overlays",
	Airspaces:     "Airspace overlays (CTR, TMA, restricted, info) display (ASD-011)",
	RangeRings:    "Range-ring overlay display (ASD-012)",
	HistoryDots:   "Track history dots display (ASD-004a)",
	VorNdb:        "VOR/NDB navaid overlay display (ASD-003)",
	Waypoints:     "Waypoint overlay display (ASD-003)",
	WeatherRadar:  "DWD weather-radar map overlay (WX-A, ADR 0016)",
	QNH:           "QNH altimeter-setting header infobox (WX-B, ADR 0016)",
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

// Describe returns the human-readable description for a known key, or "" if the
// key is not in the catalog.
func Describe(key Key) string { return catalog[key] }
