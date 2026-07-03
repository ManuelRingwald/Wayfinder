// Package weathertiles is a best-effort DWD weather-radar tile proxy (WX-A,
// ADR 0016). MapLibre has no native WMS client, so the frontend requests
// standard XYZ tiles from Wayfinder (/api/weather/radar/{z}/{x}/{y}.png) and the
// backend translates each tile into a DWD GeoServer WMS GetMap call in Web
// Mercator (EPSG:3857), caches the PNG briefly, and serves it. Keeping the fetch
// server-side gives one auditable egress point, keeps the browser same-origin
// (no CORS), centralises the "© Deutscher Wetterdienst" attribution, and lets the
// overlay be gated per tenant — exactly the trust-boundary stance of ADR 0016.
//
// Like the OpenAIP overlays (ADR 0004) it is strictly best-effort: an upstream
// failure never surfaces as an error, never blocks readiness, and never touches
// the CAT062 track path. A failed or unconfigured fetch yields a transparent
// tile, so the map simply shows no radar rather than a broken overlay.
package weathertiles

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"strconv"
)

// webMercatorOriginShift is half the circumference of the Earth at the equator in
// EPSG:3857 metres (π·6378137). The projected world spans
// [-originShift, +originShift] on both axes.
const webMercatorOriginShift = 20037508.342789244

// maxZoom bounds the accepted tile zoom. Beyond this a client is almost certainly
// malformed; we refuse rather than compute absurd bounding boxes.
const maxZoom = 22

// tileBBox3857 returns the EPSG:3857 bounding box (minX, minY, maxX, maxY, metres)
// of the standard XYZ tile (z, x, y). XYZ uses a top-left origin, so y grows
// southward — hence maxY is derived from y and minY from y+1.
func tileBBox3857(z, x, y int) (minX, minY, maxX, maxY float64) {
	worldSize := 2 * webMercatorOriginShift
	tileSize := worldSize / math.Exp2(float64(z))
	minX = -webMercatorOriginShift + float64(x)*tileSize
	maxX = -webMercatorOriginShift + float64(x+1)*tileSize
	maxY = webMercatorOriginShift - float64(y)*tileSize
	minY = webMercatorOriginShift - float64(y+1)*tileSize
	return
}

// bboxParam renders a bounding box as the WMS "minx,miny,maxx,maxy" string.
// EPSG:3857 axis order is unambiguous (easting, northing), which is why WX-A uses
// Web Mercator rather than EPSG:4326 (whose WMS 1.3.0 lat,lon order is a classic
// blank-tile trap).
func bboxParam(minX, minY, maxX, maxY float64) string {
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }
	return f(minX) + "," + f(minY) + "," + f(maxX) + "," + f(maxY)
}

// validTile reports whether (z, x, y) is a valid XYZ tile coordinate: a sane zoom
// and x/y within the 2^z grid. Invalid coordinates are served a transparent tile
// rather than fetched, so a malformed request can never reach the upstream.
func validTile(z, x, y int) bool {
	if z < 0 || z > maxZoom {
		return false
	}
	n := 1 << uint(z) // 2^z; safe because z <= maxZoom
	return x >= 0 && x < n && y >= 0 && y < n
}

// transparentTilePNG is a 1×1 fully transparent PNG used as the graceful fallback
// when a tile cannot be fetched (upstream down, feature disabled, invalid coords).
// MapLibre scales it over the whole tile, so the map shows no radar there instead
// of a broken-image tile. Built once at init.
var transparentTilePNG = mustTransparentPNG()

func mustTransparentPNG() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1)) // zero value = fully transparent
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		// image/png never fails on a 1×1 NRGBA; fall back to an empty body, which
		// the handler still serves as image/png (a blank, harmless tile).
		return nil
	}
	return buf.Bytes()
}
