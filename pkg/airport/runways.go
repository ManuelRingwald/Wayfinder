package airport

import (
	_ "embed"
	"sort"
	"strconv"
	"strings"
)

//go:embed runways.tsv
var rawRunways string

// Runway is one runway centreline: the two thresholds (low/high end) in WGS84,
// with the airport's ICAO and the runway ident (e.g. "05/23"). JSON tags match
// the /api/runways.geojson feature properties the frontend consumes (#192).
type Runway struct {
	ICAO  string  `json:"icao"`
	Ident string  `json:"ident"`
	LELat float64 `json:"-"`
	LELon float64 `json:"-"`
	HELat float64 `json:"-"`
	HELon float64 `json:"-"`
}

// RunwayIndex is a searchable, in-memory runway directory.
type RunwayIndex struct {
	runways []Runway
}

// defaultRunwayIndex is parsed once from the embedded data at package init.
// Parsing is tolerant: a malformed line is skipped, never fatal — the overlay is
// best-effort display context, not a safety-critical path.
var defaultRunwayIndex = parseRunways(rawRunways)

// RunwaysInBBox runs against the embedded directory. See (*RunwayIndex).InBBox.
func RunwaysInBBox(minLat, minLon, maxLat, maxLon float64, limit int) []Runway {
	return defaultRunwayIndex.InBBox(minLat, minLon, maxLat, maxLon, limit)
}

// RunwayCount reports how many runways are loaded (for startup logging / sanity).
func RunwayCount() int { return len(defaultRunwayIndex.runways) }

// parseRunways builds a RunwayIndex from ICAO<TAB>IDENT<TAB>LE_LAT<TAB>LE_LON<TAB>
// HE_LAT<TAB>HE_LON lines.
func parseRunways(data string) *RunwayIndex {
	ix := &RunwayIndex{}
	for _, line := range strings.Split(data, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			continue
		}
		leLat, e1 := strconv.ParseFloat(f[2], 64)
		leLon, e2 := strconv.ParseFloat(f[3], 64)
		heLat, e3 := strconv.ParseFloat(f[4], 64)
		heLon, e4 := strconv.ParseFloat(f[5], 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			continue
		}
		ix.runways = append(ix.runways, Runway{
			ICAO: f[0], Ident: f[1],
			LELat: leLat, LELon: leLon, HELat: heLat, HELon: heLon,
		})
	}
	return ix
}

// InBBox returns the runways whose centre (midpoint of the two thresholds) falls
// inside the WGS84 bounding box, ordered by ICAO then ident, capped at limit
// (#192, the runway overlay). A non-positive limit means unbounded. The midpoint
// test keeps a runway wholly in-sector; callers pass the tenant AOI so only
// in-sector runways are returned.
func (ix *RunwayIndex) InBBox(minLat, minLon, maxLat, maxLon float64, limit int) []Runway {
	var out []Runway
	for _, rw := range ix.runways {
		midLat := (rw.LELat + rw.HELat) / 2
		midLon := (rw.LELon + rw.HELon) / 2
		if midLat < minLat || midLat > maxLat || midLon < minLon || midLon > maxLon {
			continue
		}
		out = append(out, rw)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ICAO != out[j].ICAO {
			return out[i].ICAO < out[j].ICAO
		}
		return out[i].Ident < out[j].Ident
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
