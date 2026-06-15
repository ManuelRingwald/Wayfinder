package aeronautical

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

// sampleNavaids is a representative OpenAIP-shaped response: one valid VOR, one
// valid NDB, and one item with broken geometry that must be skipped.
const sampleNavaids = `{
  "items": [
    {"name": "FRANKFURT", "identifier": "FFM", "type": 3,
     "frequency": {"value": "114.2", "unit": 2},
     "geometry": {"type": "Point", "coordinates": [8.6, 50.05]}},
    {"name": "BAD HOMBURG", "identifier": "HLM", "type": 2,
     "geometry": {"type": "Point", "coordinates": [8.5, 50.2]}},
    {"name": "BROKEN", "type": 3,
     "geometry": {"type": "Point", "coordinates": []}}
  ]
}`

func TestFetchTransformsNavaids(t *testing.T) {
	var gotKey, gotBBox, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-openaip-api-key")
		gotBBox = r.URL.Query().Get("bbox")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), srv.URL, "secret-key")
	bbox := BoundingBox{MinLon: 8, MinLat: 50, MaxLon: 9, MaxLat: 51}

	fc, err := c.Fetch(context.Background(), KindNavaid, bbox)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if gotKey != "secret-key" {
		t.Errorf("expected API key header forwarded, got %q", gotKey)
	}
	if gotBBox != "8.000000,50.000000,9.000000,51.000000" {
		t.Errorf("unexpected bbox query %q", gotBBox)
	}
	if gotPath != "/navaids" {
		t.Errorf("expected navaids path, got %q", gotPath)
	}

	// The broken-geometry item must be skipped.
	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 valid features, got %d", len(fc.Features))
	}

	props := fc.Features[0].Properties
	if props["name"] != "FRANKFURT" || props["ident"] != "FFM" {
		t.Errorf("unexpected props %v", props)
	}
	if props["navaid_kind"] != "VOR" {
		t.Errorf("expected navaid_kind VOR, got %v", props["navaid_kind"])
	}
	if props["frequency"] != "114.2" {
		t.Errorf("expected frequency 114.2, got %v", props["frequency"])
	}
	if fc.Features[1].Properties["navaid_kind"] != "NDB" {
		t.Errorf("expected second navaid NDB, got %v", fc.Features[1].Properties["navaid_kind"])
	}

	// Geometry is passed through as a valid GeoJSON Point.
	var g struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(fc.Features[0].Geometry, &g); err != nil || g.Type != "Point" {
		t.Errorf("expected passthrough Point geometry, got %s (%v)", fc.Features[0].Geometry, err)
	}
}

func TestFetchNon200ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), srv.URL, "bad")
	_, err := c.Fetch(context.Background(), KindAirspace, BoundingBox{})
	if err == nil {
		t.Fatal("expected an error on non-200 status")
	}
}

func TestFetchToleratesEmptyAndMissingItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"page":1}`)) // no "items" field
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), srv.URL, "")
	fc, err := c.Fetch(context.Background(), KindWaypoint, BoundingBox{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if fc.Type != "FeatureCollection" || len(fc.Features) != 0 {
		t.Errorf("expected empty FeatureCollection, got %+v", fc)
	}
}

func TestBoundingBoxFromCenterIsRoughlySquare(t *testing.T) {
	// At 50°N a 100 km radius should span ~1.8° lat and a wider lon span.
	bbox := BoundingBoxFromCenter(50, 8, 100)
	dLat := bbox.MaxLat - bbox.MinLat
	dLon := bbox.MaxLon - bbox.MinLon
	if math.Abs(dLat-1.796) > 0.05 {
		t.Errorf("unexpected lat span %.3f", dLat)
	}
	if dLon <= dLat {
		t.Errorf("expected lon span (%.3f) wider than lat span (%.3f) at 50N", dLon, dLat)
	}
}

func TestValidGeometryRejectsBadInput(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{`{"type":"Point","coordinates":[1,2]}`, true},
		{`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}`, true},
		{`{"type":"Point","coordinates":[]}`, false},
		{`{"type":"Banana","coordinates":[1,2]}`, false},
		{`{"coordinates":[1,2]}`, false},
		{`not json`, false},
		{``, false},
	}
	for _, tc := range cases {
		if got := validGeometry([]byte(tc.raw)); got != tc.want {
			t.Errorf("validGeometry(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
