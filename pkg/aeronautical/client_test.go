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

// sampleAirspaces exercises the AoR enrichment (ASD-014): one fully-attributed
// airspace (stable id, ICAO class, floor/ceiling triples) and one bare airspace
// whose absent fields must be omitted, not emitted as zero values.
const sampleAirspaces = `{
  "items": [
    {"_id": "62a1f0c0abcdef0123456789", "name": "HAMBURG CTR", "type": 4, "icaoClass": 3,
     "lowerLimit": {"value": 0, "unit": 1, "referenceDatum": 0},
     "upperLimit": {"value": 1500, "unit": 1, "referenceDatum": 1},
     "geometry": {"type": "Polygon", "coordinates": [[[9.9,53.5],[10.1,53.5],[10.1,53.7],[9.9,53.5]]]}},
    {"name": "BARE TMA", "type": 7,
     "geometry": {"type": "Polygon", "coordinates": [[[9,53],[10,53],[10,54],[9,53]]]}}
  ]
}`

func TestFetchEnrichesAirspaceProperties(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleAirspaces))
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), srv.URL, "")
	fc, err := c.Fetch(context.Background(), KindAirspace, BoundingBox{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(fc.Features))
	}

	// First airspace carries the full enrichment.
	p := fc.Features[0].Properties
	if p["kind"] != "airspace" || p["name"] != "HAMBURG CTR" {
		t.Fatalf("unexpected base props %v", p)
	}
	if p["id"] != "62a1f0c0abcdef0123456789" {
		t.Errorf("expected stable id, got %v", p["id"])
	}
	if p["icao_class"] != 3 {
		t.Errorf("expected icao_class 3, got %v", p["icao_class"])
	}
	lower, ok := p["lower"].(map[string]any)
	if !ok {
		t.Fatalf("expected lower band object, got %T", p["lower"])
	}
	if lower["value"] != float64(0) || lower["unit"] != 1 || lower["referenceDatum"] != 0 {
		t.Errorf("unexpected lower band %v", lower)
	}
	upper, ok := p["upper"].(map[string]any)
	if !ok {
		t.Fatalf("expected upper band object, got %T", p["upper"])
	}
	if upper["value"] != float64(1500) || upper["unit"] != 1 || upper["referenceDatum"] != 1 {
		t.Errorf("unexpected upper band %v", upper)
	}

	// Second airspace: absent fields must be omitted, not emitted as zero/empty.
	q := fc.Features[1].Properties
	for _, k := range []string{"id", "icao_class", "lower", "upper"} {
		if _, present := q[k]; present {
			t.Errorf("expected %q omitted when absent, got %v", k, q[k])
		}
	}
}

func TestEnrichmentFieldsAreAirspaceOnly(t *testing.T) {
	// Even if OpenAIP returns _id/icaoClass/limits on a non-airspace object, the
	// AoR enrichment must not leak onto navaid/waypoint output (backward compat).
	const body = `{"items":[{"name":"X","type":3,"_id":"abc","icaoClass":2,
	  "lowerLimit":{"value":0,"unit":1,"referenceDatum":0},
	  "geometry":{"type":"Point","coordinates":[8,50]}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), srv.URL, "")
	fc, err := c.Fetch(context.Background(), KindNavaid, BoundingBox{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}
	p := fc.Features[0].Properties
	for _, k := range []string{"id", "icao_class", "lower", "upper"} {
		if _, present := p[k]; present {
			t.Errorf("navaid must not carry airspace field %q, got %v", k, p[k])
		}
	}
	if p["navaid_kind"] != "VOR" {
		t.Errorf("expected navaid still transformed, got %v", p["navaid_kind"])
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
