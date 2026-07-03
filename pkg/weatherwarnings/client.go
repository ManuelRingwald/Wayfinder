package weatherwarnings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// defaultWFSURL is the DWD GeoServer "dwd" workspace OWS/WFS endpoint. Public, no
// key. Overridable via WAYFINDER_DWD_WARN_URL.
const defaultWFSURL = "https://maps.dwd.de/geoserver/dwd/ows"

// defaultLayer is the dissolved municipality warnings layer — adjacent cells with
// the same warning merged into one polygon, i.e. far fewer features than the raw
// per-municipality layer. Overridable via WAYFINDER_DWD_WARN_LAYER.
const defaultLayer = "dwd:Warnungen_Gemeinden_vereinigt"

// maxResponseBytes caps the WFS response read into memory (defensive consumer,
// ADR 0016). Nationwide dissolved warnings are well under this; 16 MiB is a
// generous ceiling against a hostile/runaway upstream.
const maxResponseBytes = 16 << 20

// Client fetches DWD warning polygons from a GeoServer WFS as GeoJSON.
type Client struct {
	httpClient *http.Client
	wfsURL     string
	layer      string
}

// NewClient builds a WFS client. Nil httpClient → http.DefaultClient (production
// injects a timed one). Empty wfsURL/layer fall back to the DWD defaults.
func NewClient(httpClient *http.Client, wfsURL, layer string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if strings.TrimSpace(wfsURL) == "" {
		wfsURL = defaultWFSURL
	}
	if strings.TrimSpace(layer) == "" {
		layer = defaultLayer
	}
	return &Client{httpClient: httpClient, wfsURL: wfsURL, layer: layer}
}

// requestURL builds the WFS 2.0 GetFeature URL requesting GeoJSON in WGS84.
// GeoJSON output is always lon,lat (CRS:84) per spec, so there is no axis-order
// trap — MapLibre gets correct coordinates directly.
func (c *Client) requestURL() (string, error) {
	u, err := url.Parse(c.wfsURL)
	if err != nil {
		return "", fmt.Errorf("weatherwarnings: bad WFS URL: %w", err)
	}
	q := u.Query()
	q.Set("service", "WFS")
	q.Set("version", "2.0.0")
	q.Set("request", "GetFeature")
	q.Set("typeName", c.layer)
	q.Set("outputFormat", "application/json")
	q.Set("srsName", "EPSG:4326")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Fetch retrieves the current warnings as a normalised FeatureCollection.
// Features with missing/invalid geometry are dropped rather than failing the
// whole fetch (ADR 0016: defensive consumer).
func (c *Client) Fetch(ctx context.Context) (FeatureCollection, error) {
	target, err := c.requestURL()
	if err != nil {
		return EmptyCollection(), err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return EmptyCollection(), err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return EmptyCollection(), fmt.Errorf("weatherwarnings: fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return EmptyCollection(), fmt.Errorf("weatherwarnings: fetch: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return EmptyCollection(), fmt.Errorf("weatherwarnings: read: %w", err)
	}

	var parsed rawCollection
	if err := json.Unmarshal(body, &parsed); err != nil {
		return EmptyCollection(), fmt.Errorf("weatherwarnings: decode: %w", err)
	}

	fc := EmptyCollection()
	for _, f := range parsed.Features {
		if !validGeometry(f.Geometry) {
			continue
		}
		fc.Features = append(fc.Features, newFeature(f.Geometry, normaliseProps(f.Properties)))
	}
	return fc, nil
}

// rawCollection / rawFeature are the tolerant view of the WFS GeoJSON. Unknown
// fields are ignored; missing arrays decode to empty.
type rawCollection struct {
	Features []rawFeature `json:"features"`
}

type rawFeature struct {
	Geometry   json.RawMessage `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}

// normaliseProps reduces the DWD CAP-derived attributes to a small, stable shape
// the frontend can style/popup without knowing DWD's schema: a numeric
// wf_level (1..4) driving the colour, plus headline/event/expires for the popup.
// It reads keys case-insensitively (GeoServer casing varies) and never panics on
// a missing/odd value.
func normaliseProps(in map[string]any) map[string]any {
	get := func(keys ...string) string {
		for k, v := range in {
			for _, want := range keys {
				if strings.EqualFold(k, want) {
					if s, ok := v.(string); ok {
						return s
					}
				}
			}
		}
		return ""
	}
	out := map[string]any{
		"wf_level": severityLevel(get("SEVERITY")),
	}
	if h := get("HEADLINE"); h != "" {
		out["headline"] = h
	}
	if e := get("EVENT"); e != "" {
		out["event"] = e
	}
	if x := get("EXPIRES"); x != "" {
		out["expires"] = x
	}
	return out
}

// severityLevel maps the CAP SEVERITY string to a 1..4 level for the colour ramp.
// Unknown/empty maps to 2 (moderate) so a warning is always visibly rendered.
func severityLevel(sev string) int {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "minor":
		return 1
	case "moderate":
		return 2
	case "severe":
		return 3
	case "extreme":
		return 4
	default:
		return 2
	}
}
