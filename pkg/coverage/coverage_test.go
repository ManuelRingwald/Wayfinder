package coverage_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/coverage"
)

func TestParseEnv_empty(t *testing.T) {
	sensors := coverage.ParseEnv(func(string) string { return "" })
	if len(sensors) != 0 {
		t.Fatalf("expected 0 sensors, got %d", len(sensors))
	}
}

func TestParseEnv_single(t *testing.T) {
	env := map[string]string{
		"WAYFINDER_COVERAGE_SENSOR_1_LAT":         "50.0379",
		"WAYFINDER_COVERAGE_SENSOR_1_LON":         "8.5622",
		"WAYFINDER_COVERAGE_SENSOR_1_MAX_RANGE_M": "120000",
		"WAYFINDER_COVERAGE_SENSOR_1_LABEL":       "Frankfurt",
	}
	sensors := coverage.ParseEnv(func(k string) string { return env[k] })
	if len(sensors) != 1 {
		t.Fatalf("expected 1 sensor, got %d", len(sensors))
	}
	s := sensors[0]
	if s.Lat != 50.0379 {
		t.Errorf("lat: got %v want 50.0379", s.Lat)
	}
	if s.Lon != 8.5622 {
		t.Errorf("lon: got %v want 8.5622", s.Lon)
	}
	if s.MaxRangeM != 120_000 {
		t.Errorf("max_range_m: got %v want 120000", s.MaxRangeM)
	}
	if s.Label != "Frankfurt" {
		t.Errorf("label: got %q want Frankfurt", s.Label)
	}
	if s.MinRangeM != 0 {
		t.Errorf("min_range_m: got %v want 0", s.MinRangeM)
	}
}

func TestParseEnv_multiple_stops_at_gap(t *testing.T) {
	env := map[string]string{
		"WAYFINDER_COVERAGE_SENSOR_1_LAT":         "50.0379",
		"WAYFINDER_COVERAGE_SENSOR_1_LON":         "8.5622",
		"WAYFINDER_COVERAGE_SENSOR_1_MAX_RANGE_M": "120000",
		// Sensor 2 is absent → parsing must stop here (no sensor 3 should sneak in).
		"WAYFINDER_COVERAGE_SENSOR_3_LAT":         "48.0",
		"WAYFINDER_COVERAGE_SENSOR_3_LON":         "11.0",
		"WAYFINDER_COVERAGE_SENSOR_3_MAX_RANGE_M": "200000",
	}
	sensors := coverage.ParseEnv(func(k string) string { return env[k] })
	if len(sensors) != 1 {
		t.Fatalf("expected 1 sensor (gap at N=2 stops iteration), got %d", len(sensors))
	}
}

func TestParseEnv_skips_zero_max_range(t *testing.T) {
	env := map[string]string{
		"WAYFINDER_COVERAGE_SENSOR_1_LAT":         "50.0",
		"WAYFINDER_COVERAGE_SENSOR_1_LON":         "8.0",
		"WAYFINDER_COVERAGE_SENSOR_1_MAX_RANGE_M": "0",
	}
	sensors := coverage.ParseEnv(func(k string) string { return env[k] })
	if len(sensors) != 0 {
		t.Fatalf("expected sensor with max_range=0 to be skipped, got %d", len(sensors))
	}
}

func TestRingsGeoJSON_outer_only(t *testing.T) {
	sensors := []coverage.SensorConfig{
		{Lat: 0, Lon: 0, MaxRangeM: 100_000, Label: "Test"},
	}
	data, err := coverage.RingsGeoJSON(sensors, "#5B8DEF")
	if err != nil {
		t.Fatalf("RingsGeoJSON error: %v", err)
	}

	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Type     string `json:"type"`
			Geometry struct {
				Type string `json:"type"`
			} `json:"geometry"`
			Properties map[string]interface{} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}
	if fc.Type != "FeatureCollection" {
		t.Errorf("type: got %q", fc.Type)
	}
	// One outer ring + one center point = 2 features (no inner ring when MinRangeM=0).
	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 features (outer + center), got %d", len(fc.Features))
	}

	outerF := fc.Features[0]
	if outerF.Geometry.Type != "LineString" {
		t.Errorf("outer geometry type: got %q want LineString", outerF.Geometry.Type)
	}
	if outerF.Properties["type"] != "outer" {
		t.Errorf("outer type property: got %v", outerF.Properties["type"])
	}
	if outerF.Properties["color"] != "#5B8DEF" {
		t.Errorf("outer color: got %v", outerF.Properties["color"])
	}

	centerF := fc.Features[1]
	if centerF.Geometry.Type != "Point" {
		t.Errorf("center geometry type: got %q want Point", centerF.Geometry.Type)
	}
}

func TestRingsGeoJSON_with_inner_ring(t *testing.T) {
	sensors := []coverage.SensorConfig{
		{Lat: 50.0, Lon: 8.0, MinRangeM: 5_000, MaxRangeM: 120_000, Label: "FRA"},
	}
	data, err := coverage.RingsGeoJSON(sensors, "#fff")
	if err != nil {
		t.Fatalf("RingsGeoJSON error: %v", err)
	}
	var fc struct {
		Features []struct {
			Properties map[string]interface{} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}
	// outer + inner + center = 3
	if len(fc.Features) != 3 {
		t.Fatalf("expected 3 features (outer + inner + center), got %d", len(fc.Features))
	}
	if fc.Features[1].Properties["type"] != "inner" {
		t.Errorf("second feature type property: got %v", fc.Features[1].Properties["type"])
	}
}

// TestCircleApproximationRadius verifies that circle points are within 1 % of
// the requested radius from the centre.
func TestCircleApproximationRadius(t *testing.T) {
	sensors := []coverage.SensorConfig{
		{Lat: 50.0379, Lon: 8.5622, MaxRangeM: 120_000},
	}
	data, _ := coverage.RingsGeoJSON(sensors, "#fff")

	var fc struct {
		Features []struct {
			Geometry struct {
				Type        string      `json:"type"`
				Coordinates interface{} `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// First feature is the outer LineString.
	raw, _ := json.Marshal(fc.Features[0].Geometry.Coordinates)
	var coords [][2]float64
	if err := json.Unmarshal(raw, &coords); err != nil {
		t.Fatalf("coords parse: %v", err)
	}

	const (
		latDeg    = 50.0379
		lonDeg    = 8.5622
		wantM     = 120_000.0
		tolerance = 0.01 // 1 % relative error
	)
	const metersPerDegLat = 111_320.0
	latRad := latDeg * math.Pi / 180
	metersPerDegLon := metersPerDegLat * math.Cos(latRad)

	for _, c := range coords {
		dLon := (c[0] - lonDeg) * metersPerDegLon
		dLat := (c[1] - latDeg) * metersPerDegLat
		gotM := math.Sqrt(dLon*dLon + dLat*dLat)
		if math.Abs(gotM-wantM)/wantM > tolerance {
			t.Errorf("point [%.6f, %.6f]: radius %.1f m, want ~%.1f m (tol %.0f%%)",
				c[0], c[1], gotM, wantM, tolerance*100)
		}
	}
}
