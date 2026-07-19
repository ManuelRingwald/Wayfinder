package basemapsearch

import "math"

// BBox is a WGS84 bounding box (degrees). Defined locally so the package does
// not depend on the store layer; main adapts the tenant view config.
type BBox struct {
	MinLat, MinLon, MaxLat, MaxLon float64
}

// tileRange is an inclusive x/y range of slippy-map tiles at one zoom level.
type tileRange struct {
	zoom               int
	minX, maxX         int
	minY, maxY         int
	clamped            bool // true when the AOI exceeded the tile cap and was shrunk
	requestedTileCount int  // pre-clamp count, for the operator log
}

func (r tileRange) count() int { return (r.maxX - r.minX + 1) * (r.maxY - r.minY + 1) }

// tileXY converts a WGS84 coordinate to slippy-map tile indices (OSM/XYZ
// scheme, the scheme basemap.de serves).
func tileXY(lat, lon float64, zoom int) (x, y int) {
	n := float64(int(1) << zoom)
	x = int(math.Floor((lon + 180) / 360 * n))
	latRad := lat * math.Pi / 180
	y = int(math.Floor((1 - math.Log(math.Tan(latRad)+1/math.Cos(latRad))/math.Pi) / 2 * n))
	max := int(n) - 1
	return clampInt(x, 0, max), clampInt(y, 0, max)
}

// tilesForBBox computes the tile range covering bbox at zoom, clamped
// symmetrically around the bbox centre so the total never exceeds maxTiles
// (the operator's hard cap: an oversized AOI still gets a working index for
// its central area instead of a refusal — W2 of the #277 design decision).
func tilesForBBox(b BBox, zoom, maxTiles int) tileRange {
	minX, maxY := tileXY(b.MinLat, b.MinLon, zoom) // south-west corner: min x, MAX y (y grows southward)
	maxX, minY := tileXY(b.MaxLat, b.MaxLon, zoom)
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	r := tileRange{zoom: zoom, minX: minX, maxX: maxX, minY: minY, maxY: maxY}
	r.requestedTileCount = r.count()
	for r.count() > maxTiles {
		// Shrink the longer axis by one tile on each side per iteration —
		// deterministic, centre-preserving, and it terminates (count strictly
		// decreases while > 1×1).
		if r.maxX-r.minX >= r.maxY-r.minY && r.maxX > r.minX {
			r.minX++
			if r.maxX > r.minX {
				r.maxX--
			}
		} else if r.maxY > r.minY {
			r.minY++
			if r.maxY > r.minY {
				r.maxY--
			}
		} else {
			break // 1×1 cannot shrink further
		}
		r.clamped = true
	}
	return r
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
