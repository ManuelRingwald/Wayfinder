// Package coverage provides sensor coverage ring configuration and GeoJSON
// generation for Wayfinder's radar coverage overlay.
//
// The operator supplies sensor positions and ranges via environment variables
// (WAYFINDER_COVERAGE_SENSOR_N_*), independently of Firefly's own sensor
// configuration. Both systems must be configured with matching values, just
// as they both need aligned multicast group/port settings. This keeps the
// CAT062 wire contract as the sole coupling between Firefly and Wayfinder.
package coverage

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// SensorConfig holds the display parameters for one radar sensor's coverage ring.
type SensorConfig struct {
	// Lat / Lon: geodetic position of the radar site, decimal degrees (WGS84).
	Lat float64
	Lon float64
	// MinRangeM: inner detection boundary, metres. Zero means no inner dead zone.
	MinRangeM float64
	// MaxRangeM: outer detection boundary, metres. Must be > 0 to be displayed.
	MaxRangeM float64
	// Label: human-readable name shown in hover tooltips. Optional.
	Label string
}

// ParseEnv reads sensor coverage configuration from a lookup function.
//
// Variables follow the pattern WAYFINDER_COVERAGE_SENSOR_N_* where N starts
// at 1.  Parsing stops at the first N for which LAT is absent.  Up to 20
// sensors are supported.  A sensor is silently skipped when MaxRangeM ≤ 0.
//
// Example:
//
//	WAYFINDER_COVERAGE_SENSOR_1_LAT=50.0379
//	WAYFINDER_COVERAGE_SENSOR_1_LON=8.5622
//	WAYFINDER_COVERAGE_SENSOR_1_MIN_RANGE_M=0
//	WAYFINDER_COVERAGE_SENSOR_1_MAX_RANGE_M=120000
//	WAYFINDER_COVERAGE_SENSOR_1_LABEL=Frankfurt
func ParseEnv(getenv func(string) string) []SensorConfig {
	const maxSensors = 20
	var sensors []SensorConfig
	for n := 1; n <= maxSensors; n++ {
		prefix := fmt.Sprintf("WAYFINDER_COVERAGE_SENSOR_%d_", n)
		latStr := getenv(prefix + "LAT")
		if latStr == "" {
			break // stop at first missing entry
		}
		lat, err := strconv.ParseFloat(strings.TrimSpace(latStr), 64)
		if err != nil {
			continue
		}
		lonStr := getenv(prefix + "LON")
		lon, err := strconv.ParseFloat(strings.TrimSpace(lonStr), 64)
		if err != nil {
			continue
		}
		maxR, _ := strconv.ParseFloat(strings.TrimSpace(getenv(prefix+"MAX_RANGE_M")), 64)
		if maxR <= 0 {
			continue // nothing to draw without a valid outer range
		}
		minR, _ := strconv.ParseFloat(strings.TrimSpace(getenv(prefix+"MIN_RANGE_M")), 64)
		if minR < 0 {
			minR = 0
		}
		label := strings.TrimSpace(getenv(prefix + "LABEL"))
		sensors = append(sensors, SensorConfig{
			Lat:       lat,
			Lon:       lon,
			MinRangeM: minR,
			MaxRangeM: maxR,
			Label:     label,
		})
	}
	return sensors
}

// RingsGeoJSON builds a GeoJSON FeatureCollection representing the coverage
// rings for all configured sensors.
//
// Each sensor contributes:
//   - One LineString feature at MaxRangeM (outer boundary, always present).
//   - One LineString feature at MinRangeM (inner boundary, only when > 0).
//   - One Point feature at the sensor position (for hover label / click).
//
// All ring features carry properties: "sensor_label", "range_m", "type"
// ("outer" | "inner" | "center"), "color".
// The color is passed in from WAYFINDER_COVERAGE_RING_COLOR so the frontend
// can set it uniformly for all sensors without per-feature style logic.
func RingsGeoJSON(sensors []SensorConfig, color string) ([]byte, error) {
	type geometry struct {
		Type        string      `json:"type"`
		Coordinates interface{} `json:"coordinates"`
	}
	type properties map[string]interface{}
	type feature struct {
		Type       string     `json:"type"`
		Geometry   geometry   `json:"geometry"`
		Properties properties `json:"properties"`
	}
	type featureCollection struct {
		Type     string    `json:"type"`
		Features []feature `json:"features"`
	}

	fc := featureCollection{Type: "FeatureCollection"}

	for _, s := range sensors {
		label := s.Label

		// Outer ring.
		outer := circleLineString(s.Lat, s.Lon, s.MaxRangeM, 128)
		fc.Features = append(fc.Features, feature{
			Type:     "Feature",
			Geometry: geometry{Type: "LineString", Coordinates: outer},
			Properties: properties{
				"sensor_label": label,
				"range_m":      s.MaxRangeM,
				"type":         "outer",
				"color":        color,
			},
		})

		// Inner ring (only when there is a dead zone).
		if s.MinRangeM > 0 {
			inner := circleLineString(s.Lat, s.Lon, s.MinRangeM, 128)
			fc.Features = append(fc.Features, feature{
				Type:     "Feature",
				Geometry: geometry{Type: "LineString", Coordinates: inner},
				Properties: properties{
					"sensor_label": label,
					"range_m":      s.MinRangeM,
					"type":         "inner",
					"color":        color,
				},
			})
		}

		// Sensor centre point (for tooltip / label).
		fc.Features = append(fc.Features, feature{
			Type: "Feature",
			Geometry: geometry{
				Type:        "Point",
				Coordinates: [2]float64{s.Lon, s.Lat},
			},
			Properties: properties{
				"sensor_label": label,
				"type":         "center",
				"color":        color,
			},
		})
	}

	return json.Marshal(fc)
}

// circleLineString approximates a circle of radius r metres centred at
// (latDeg, lonDeg) as a closed GeoJSON LineString with n points.
//
// The approximation uses a flat-earth conversion: for coverage rings (r up to
// ~250 km) the positional error stays well below 1 % of the radius, which is
// indistinguishable on a controller's scope.
func circleLineString(latDeg, lonDeg, radiusM float64, n int) [][2]float64 {
	const metersPerDegLat = 111_320.0
	latRad := latDeg * math.Pi / 180.0
	metersPerDegLon := metersPerDegLat * math.Cos(latRad)
	if metersPerDegLon < 1 {
		metersPerDegLon = 1 // guard against poles
	}

	pts := make([][2]float64, n+1)
	for i := 0; i < n; i++ {
		θ := 2 * math.Pi * float64(i) / float64(n)
		dLat := (radiusM / metersPerDegLat) * math.Cos(θ)
		dLon := (radiusM / metersPerDegLon) * math.Sin(θ)
		pts[i] = [2]float64{lonDeg + dLon, latDeg + dLat}
	}
	pts[n] = pts[0] // close the ring
	return pts
}
