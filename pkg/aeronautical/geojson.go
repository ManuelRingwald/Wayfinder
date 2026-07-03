// Package aeronautical fetches aeronautical context data (airspaces, navaids,
// waypoints) from OpenAIP, caches it, and serves it to the ASD frontend as
// GeoJSON. The track path (CAT062 → WebSocket → map) is fully independent of
// this package: per ADR 0004 the aeronautical layers are best-effort and an
// OpenAIP outage must never affect track display or readiness.
package aeronautical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ChangeSummary is the per-kind churn of one OpenAIP refresh (AERO-3): how many
// features were added/removed relative to the previous cache, plus the previous
// count. HasPrev is false on the very first fetch (nothing to diff against). The
// counts are content-based and exact — they need no assumption about a stable
// OpenAIP feature id (an in-place edit shows as one removed + one added).
type ChangeSummary struct {
	PrevFeatureCount int
	Added            int
	Removed          int
	HasPrev          bool
}

// featureHash is a stable content fingerprint of a feature (geometry + properties).
// json.Marshal sorts map keys, so the encoding — and thus the hash — is
// deterministic for equal features.
func featureHash(f Feature) string {
	b, err := json.Marshal(f)
	if err != nil {
		// A feature that cannot marshal is degenerate; fall back to a constant so it
		// simply collapses with other unmarshalable features rather than panicking.
		return "unmarshalable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// diffCollections computes the churn between the previous collection (may be nil on
// the first fetch) and the new one, keyed by content hash (multiset, so duplicate
// features are handled). added = new content not present before; removed = old
// content no longer present.
func diffCollections(prev *FeatureCollection, next FeatureCollection) ChangeSummary {
	if prev == nil {
		return ChangeSummary{HasPrev: false}
	}
	oldCounts := make(map[string]int, len(prev.Features))
	for _, f := range prev.Features {
		oldCounts[featureHash(f)]++
	}
	newCounts := make(map[string]int, len(next.Features))
	for _, f := range next.Features {
		newCounts[featureHash(f)]++
	}
	added, removed := 0, 0
	for h, n := range newCounts {
		if extra := n - oldCounts[h]; extra > 0 {
			added += extra
		}
	}
	for h, o := range oldCounts {
		if gone := o - newCounts[h]; gone > 0 {
			removed += gone
		}
	}
	return ChangeSummary{
		PrevFeatureCount: len(prev.Features),
		Added:            added,
		Removed:          removed,
		HasPrev:          true,
	}
}

// FeatureCollection is a minimal GeoJSON FeatureCollection used as the wire
// format towards the frontend. It is intentionally small: we only need to
// pass geometry plus a handful of display properties.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

// Feature is a single GeoJSON feature. Geometry is kept as raw JSON so a valid
// OpenAIP geometry can be passed through unchanged; Properties carries the
// display fields (name, kind, type, …).
type Feature struct {
	Type       string          `json:"type"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}

// EmptyCollection returns an empty, valid GeoJSON FeatureCollection. It is the
// graceful-degradation fallback (ADR 0004): when no data has been cached yet
// the endpoints serve this rather than an error, so the map simply shows no
// overlay.
func EmptyCollection() FeatureCollection {
	return FeatureCollection{Type: "FeatureCollection", Features: []Feature{}}
}

// newFeature builds a GeoJSON Feature from a validated geometry and properties.
func newFeature(geometry json.RawMessage, props map[string]any) Feature {
	return Feature{Type: "Feature", Geometry: geometry, Properties: props}
}
