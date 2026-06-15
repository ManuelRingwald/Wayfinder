package aeronautical

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
)

// Kind identifies an aeronautical data category. The values double as the
// frontend endpoint suffixes (/api/<kind>) and cache keys.
type Kind string

const (
	KindAirspace Kind = "airspace"
	KindNavaid   Kind = "navaid"
	KindWaypoint Kind = "waypoint"
)

// openaipPaths maps each kind to its OpenAIP REST path. Kept here so the wire
// detail lives in one place; the rest of the package is path-agnostic.
var openaipPaths = map[Kind]string{
	KindAirspace: "/airspaces",
	KindNavaid:   "/navaids",
	KindWaypoint: "/reporting-points",
}

// defaultBaseURL is OpenAIP's public core API. Overridable for testing and for
// self-hosted/mirror deployments.
const defaultBaseURL = "https://api.core.openaip.net/api"

// maxResponseBytes caps an OpenAIP response we are willing to read into memory,
// guarding against a hostile or runaway upstream (ADR 0004: defensive
// consumer). 32 MiB is generous for a regional airspace/navaid set.
const maxResponseBytes = 32 << 20

// BoundingBox is a geographic query window (degrees, WGS84).
type BoundingBox struct {
	MinLon, MinLat, MaxLon, MaxLat float64
}

// BoundingBoxFromCenter builds a bounding box around a center point with the
// given radius in kilometres. The longitude span widens with latitude so the
// box stays roughly square on the ground; latitude is clamped to valid ranges.
func BoundingBoxFromCenter(lat, lon, radiusKM float64) BoundingBox {
	const kmPerDegLat = 111.32
	dLat := radiusKM / kmPerDegLat
	cosLat := math.Cos(lat * math.Pi / 180)
	if cosLat < 0.01 {
		cosLat = 0.01 // avoid blow-up near the poles
	}
	dLon := radiusKM / (kmPerDegLat * cosLat)
	return BoundingBox{
		MinLon: clamp(lon-dLon, -180, 180),
		MinLat: clamp(lat-dLat, -90, 90),
		MaxLon: clamp(lon+dLon, -180, 180),
		MaxLat: clamp(lat+dLat, -90, 90),
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// query renders the bounding box as OpenAIP's "minLon,minLat,maxLon,maxLat".
func (b BoundingBox) query() string {
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }
	return f(b.MinLon) + "," + f(b.MinLat) + "," + f(b.MaxLon) + "," + f(b.MaxLat)
}

// Client fetches aeronautical data from OpenAIP and transforms it into GeoJSON.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient creates an OpenAIP client. baseURL may be empty to use the public
// API. The http.Client should carry a sensible timeout (set by the caller).
func NewClient(httpClient *http.Client, baseURL, apiKey string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{httpClient: httpClient, baseURL: baseURL, apiKey: apiKey}
}

// Fetch retrieves one kind of aeronautical data within bbox and returns it as a
// GeoJSON FeatureCollection. Individual items with missing/invalid geometry are
// skipped rather than failing the whole fetch (ADR 0004: defensive consumer).
func (c *Client) Fetch(ctx context.Context, kind Kind, bbox BoundingBox) (FeatureCollection, error) {
	path, ok := openaipPaths[kind]
	if !ok {
		return EmptyCollection(), fmt.Errorf("aeronautical: unknown kind %q", kind)
	}

	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return EmptyCollection(), fmt.Errorf("aeronautical: bad base URL: %w", err)
	}
	q := u.Query()
	q.Set("bbox", bbox.query())
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return EmptyCollection(), err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("x-openaip-api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return EmptyCollection(), fmt.Errorf("aeronautical: fetch %s: %w", kind, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return EmptyCollection(), fmt.Errorf("aeronautical: fetch %s: unexpected status %d", kind, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return EmptyCollection(), fmt.Errorf("aeronautical: read %s: %w", kind, err)
	}

	var parsed openaipResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return EmptyCollection(), fmt.Errorf("aeronautical: decode %s: %w", kind, err)
	}

	fc := EmptyCollection()
	for _, item := range parsed.Items {
		if !validGeometry(item.Geometry) {
			continue
		}
		fc.Features = append(fc.Features, newFeature(item.Geometry, item.properties(kind)))
	}
	return fc, nil
}

// openaipResponse is the tolerant view of an OpenAIP list response. Unknown
// fields are ignored; a missing "items" yields an empty slice.
type openaipResponse struct {
	Items []openaipItem `json:"items"`
}

// openaipItem captures the few fields we surface to the frontend. `type` is
// kept raw because OpenAIP encodes it as a numeric enum.
type openaipItem struct {
	Name       string          `json:"name"`
	Type       json.RawMessage `json:"type"`
	Identifier string          `json:"identifier"`
	Frequency  *openaipValue   `json:"frequency"`
	Geometry   json.RawMessage `json:"geometry"`
}

// openaipValue is OpenAIP's {value, unit} pair (e.g. a navaid frequency).
type openaipValue struct {
	Value string          `json:"value"`
	Unit  json.RawMessage `json:"unit"`
}

// properties builds the GeoJSON properties for an item, including a normalised
// `kind` and — for navaids — a best-effort `navaid_kind` (VOR/NDB/DME/…) so the
// frontend can pick an icon.
func (it openaipItem) properties(kind Kind) map[string]any {
	props := map[string]any{"kind": string(kind)}
	if it.Name != "" {
		props["name"] = it.Name
	}
	if it.Identifier != "" {
		props["ident"] = it.Identifier
	}
	if len(it.Type) > 0 {
		var n int
		if err := json.Unmarshal(it.Type, &n); err == nil {
			props["type"] = n
			if kind == KindNavaid {
				props["navaid_kind"] = navaidKind(n)
			}
		}
	}
	if it.Frequency != nil && it.Frequency.Value != "" {
		props["frequency"] = it.Frequency.Value
	}
	return props
}

// navaidKind maps OpenAIP's numeric navaid type enum to a short label. The
// mapping is best-effort per OpenAIP's documented enum; unknown values fall
// back to a generic "NAVAID" so the frontend still shows a beacon.
func navaidKind(n int) string {
	switch n {
	case 0:
		return "DME"
	case 1:
		return "TACAN"
	case 2:
		return "NDB"
	case 3:
		return "VOR"
	case 4:
		return "VOR-DME"
	case 5:
		return "VORTAC"
	case 6:
		return "DVOR"
	case 7:
		return "DVOR-DME"
	case 8:
		return "DVORTAC"
	default:
		return "NAVAID"
	}
}

// validGeometry reports whether raw is a usable GeoJSON geometry: a JSON object
// with a known type and non-empty coordinates.
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
	case "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon":
	default:
		return false
	}
	return len(g.Coordinates) > 0 && string(g.Coordinates) != "null" && string(g.Coordinates) != "[]"
}
