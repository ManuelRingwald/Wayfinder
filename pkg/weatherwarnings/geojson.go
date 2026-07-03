// Package weatherwarnings is a best-effort DWD weather-warnings overlay (WX-C,
// ADR 0016). The DWD GeoServer publishes official severe-weather warning polygons
// as WFS GeoJSON (WGS84 lon/lat — exactly what MapLibre wants). Wayfinder fetches
// it server-side, normalises each feature's properties to a small, stable shape
// (a numeric severity level + headline/event for the popup), caches the last good
// result, and serves it as GeoJSON. Same trust-boundary stance as WX-A/B: one
// auditable egress, best-effort, never touches the CAT062 track path.
package weatherwarnings

import "encoding/json"

// FeatureCollection is the GeoJSON collection served to the frontend.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

// Feature is one warning polygon with normalised properties.
type Feature struct {
	Type       string          `json:"type"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}

// EmptyCollection is the graceful-degradation value: a valid, empty
// FeatureCollection (never nil), so the endpoint always serves valid GeoJSON.
func EmptyCollection() FeatureCollection {
	return FeatureCollection{Type: "FeatureCollection", Features: []Feature{}}
}

// newFeature builds a normalised warning feature.
func newFeature(geometry json.RawMessage, props map[string]any) Feature {
	return Feature{Type: "Feature", Geometry: geometry, Properties: props}
}

// validGeometry reports whether raw is a usable GeoJSON geometry (a JSON object
// with a known type and non-empty coordinates). Mirrors the aeronautical decoder
// so a malformed feature is dropped rather than crashing the overlay.
func validGeometry(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		return false
	}
	switch g.Type {
	case "Polygon", "MultiPolygon", "Point", "MultiPoint", "LineString", "MultiLineString":
	default:
		return false
	}
	return len(g.Coordinates) > 0 && string(g.Coordinates) != "null" && string(g.Coordinates) != "[]"
}
