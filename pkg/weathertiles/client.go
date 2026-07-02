package weathertiles

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// defaultWMSURL is the DWD GeoServer "dwd" workspace WMS endpoint. It is public
// and needs no API key. Overridable via WAYFINDER_DWD_WMS_URL (e.g. a caching
// mirror or an air-gapped GeoServer). Empty config disables the feature entirely,
// so this default only applies once a URL is explicitly configured — the const
// documents the expected shape.
const defaultWMSURL = "https://maps.dwd.de/geoserver/dwd/wms"

// defaultLayer is the DWD radar reflectivity composite ("Niederschlagsradar"),
// the standard rain-radar overlay. Overridable via WAYFINDER_DWD_RADAR_LAYER.
const defaultLayer = "dwd:Niederschlagsradar"

// maxTileBytes caps a single WMS tile response read into memory (defensive
// consumer, ADR 0016 / CLAUDE.md §7). A 256×256 PNG is a few tens of KiB; 4 MiB
// is a very generous ceiling that still guards against a hostile/runaway upstream.
const maxTileBytes = 4 << 20

// Client fetches one radar tile from a DWD GeoServer WMS as a Web-Mercator
// GetMap. It holds no state beyond its configuration; the caller injects the
// *http.Client (and thus the timeout).
type Client struct {
	httpClient *http.Client
	wmsURL     string
	layer      string
}

// NewClient builds a WMS tile client. A nil httpClient falls back to
// http.DefaultClient (no timeout) — production always injects a timed client.
// Empty wmsURL/layer fall back to the documented DWD defaults.
func NewClient(httpClient *http.Client, wmsURL, layer string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if strings.TrimSpace(wmsURL) == "" {
		wmsURL = defaultWMSURL
	}
	if strings.TrimSpace(layer) == "" {
		layer = defaultLayer
	}
	return &Client{httpClient: httpClient, wmsURL: wmsURL, layer: layer}
}

// tileURL builds the WMS 1.1.1 GetMap URL for tile (z, x, y) in EPSG:3857.
// WMS 1.1.1 + srs=EPSG:3857 is used deliberately: Web Mercator has an
// unambiguous axis order, avoiding the WMS 1.3.0 EPSG:4326 lat,lon trap, and it
// matches MapLibre's native projection so no client-side reprojection is needed.
func (c *Client) tileURL(z, x, y int) (string, error) {
	u, err := url.Parse(c.wmsURL)
	if err != nil {
		return "", fmt.Errorf("weathertiles: bad WMS URL: %w", err)
	}
	minX, minY, maxX, maxY := tileBBox3857(z, x, y)
	q := u.Query()
	q.Set("service", "WMS")
	q.Set("version", "1.1.1")
	q.Set("request", "GetMap")
	q.Set("layers", c.layer)
	q.Set("styles", "")
	q.Set("format", "image/png")
	q.Set("transparent", "true")
	q.Set("srs", "EPSG:3857")
	q.Set("width", "256")
	q.Set("height", "256")
	q.Set("bbox", bboxParam(minX, minY, maxX, maxY))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// FetchTile retrieves one radar tile as PNG bytes. It is defensive: it caps the
// response size, requires a 200 with an image content type, and returns an error
// on any anomaly so the Service can fall back to a transparent tile.
func (c *Client) FetchTile(ctx context.Context, z, x, y int) ([]byte, error) {
	target, err := c.tileURL(z, x, y)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/png")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weathertiles: fetch tile %d/%d/%d: %w", z, x, y, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weathertiles: fetch tile %d/%d/%d: unexpected status %d", z, x, y, resp.StatusCode)
	}
	// A GeoServer WMS error is returned as an XML ServiceException with a non-image
	// content type; reject it so we never cache/serve an XML "tile".
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("weathertiles: fetch tile %d/%d/%d: non-image content type %q", z, x, y, ct)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTileBytes))
	if err != nil {
		return nil, fmt.Errorf("weathertiles: read tile %d/%d/%d: %w", z, x, y, err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("weathertiles: empty tile %d/%d/%d", z, x, y)
	}
	return body, nil
}
