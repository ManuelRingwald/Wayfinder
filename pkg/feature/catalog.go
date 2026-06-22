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
)

// catalog is the closed set of known feature keys with human-readable
// descriptions. The admin API may only set keys in this set, and HasFeature
// treats any key outside it as fail-closed — so the catalog is the single
// source of truth for "which features exist".
var catalog = map[Key]string{
	STCA:          "Short-Term Conflict Alert display (ASD-006)",
	MultiFeed:     "Subscribe to multiple sensor feeds (WF2-41)",
	PremiumLayers: "Premium ASD map overlays",
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
