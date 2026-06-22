// Package sensorclass is the controlled vocabulary for a feed's sensor mix
// (ADR 0005 §6.4: the sensor mix is *feed metadata*, "Feed A = ADS-B-only",
// "Feed B = PSR+SSR+ADS-B"). A closed catalogue instead of free-form strings
// keeps the metadata trustworthy and auditable, and lets the feed-creation path
// reject typos before they reach the database (WF2-41).
//
// Entitlements bind to *feeds*, not to per-track sensor types (ADR 0005 §6.4);
// this package only describes/normalises what a feed contains.
package sensorclass

import (
	"fmt"
	"sort"
	"strings"
)

// Class is a canonical surveillance sensor class.
type Class string

const (
	PSR   Class = "PSR"    // primary surveillance radar
	SSR   Class = "SSR"    // secondary surveillance radar (Mode A/C)
	ModeS Class = "MODE_S" // Mode S selective interrogation
	ADSB  Class = "ADS-B"  // 1090 MHz Extended Squitter
	MLAT  Class = "MLAT"   // (wide-area) multilateration
	FLARM Class = "FLARM"  // cooperative collision avoidance (GA/gliders)
)

// catalog maps each canonical class to a human-readable description.
var catalog = map[Class]string{
	PSR:   "Primary surveillance radar (skin paint, no identity)",
	SSR:   "Secondary surveillance radar (Mode A/C)",
	ModeS: "Mode S (selective interrogation, ICAO address)",
	ADSB:  "ADS-B (1090 MHz Extended Squitter)",
	MLAT:  "Multilateration / wide-area multilateration",
	FLARM: "FLARM (cooperative collision avoidance, GA/gliders)",
}

// aliases maps a *normalised* token (uppercase, A–Z0–9 only — see normalize) to
// its canonical class. It covers the common legacy spellings seen in feed
// configs so e.g. "ads-b", "ADS_B", "ADSB", "1090ES" all collapse to ADS-B.
// Canonical values round-trip: ADS-B → "ADSB" → ADS-B.
var aliases = map[string]Class{
	"PSR": PSR, "PRIMARY": PSR, "PRIMARYRADAR": PSR,
	"SSR": SSR, "SECONDARY": SSR, "SECONDARYRADAR": SSR,
	"MODEAC": SSR, "MODEA": SSR, "MODEC": SSR,
	"MODES": ModeS, "SMODE": ModeS, "MODESELECTIVE": ModeS,
	"ADSB": ADSB, "ADSB1090": ADSB, "1090ES": ADSB, "1090": ADSB,
	"MLAT": MLAT, "MULTILATERATION": MLAT, "WAM": MLAT,
	"FLARM": FLARM,
}

// UnknownClassError reports a sensor-mix token that maps to no known class.
type UnknownClassError struct{ Token string }

func (e *UnknownClassError) Error() string {
	return fmt.Sprintf("sensorclass: unknown sensor class %q", e.Token)
}

// normalize reduces a raw token to its alias-map key: trim, uppercase, and keep
// only A–Z0–9 (so "ADS-B", "ads_b", "ADS B" all become "ADSB").
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(s)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Parse maps a raw token (any common spelling) to its canonical Class.
func Parse(s string) (Class, bool) {
	c, ok := aliases[normalize(s)]
	return c, ok
}

// IsKnown reports whether c is a canonical class in the catalogue.
func IsKnown(c Class) bool {
	_, ok := catalog[c]
	return ok
}

// Describe returns the human-readable description for a canonical class, or "".
func Describe(c Class) string { return catalog[c] }

// All returns the canonical classes in a stable (sorted) order.
func All() []Class {
	out := make([]Class, 0, len(catalog))
	for c := range catalog {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Canonicalize parses every token in raw, deduplicates (preserving first-seen
// order) and returns the canonical class strings. It returns an
// *UnknownClassError on the first token it cannot map, so an invalid mix never
// reaches the database. Empty/whitespace tokens are skipped; a nil/empty input
// yields a non-nil empty slice.
func Canonicalize(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[Class]bool, len(raw))
	for _, tok := range raw {
		if strings.TrimSpace(tok) == "" {
			continue
		}
		c, ok := Parse(tok)
		if !ok {
			return nil, &UnknownClassError{Token: tok}
		}
		if !seen[c] {
			seen[c] = true
			out = append(out, string(c))
		}
	}
	return out, nil
}
