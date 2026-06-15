// Package aeronautical fetches aeronautical context data (airspaces, navaids,
// waypoints) from OpenAIP, caches it, and serves it to the ASD frontend as
// GeoJSON. The track path (CAT062 → WebSocket → map) is fully independent of
// this package: per ADR 0004 the aeronautical layers are best-effort and an
// OpenAIP outage must never affect track display or readiness.
package aeronautical

import "encoding/json"

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
