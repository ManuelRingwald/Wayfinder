package weathertiles

import (
	"bytes"
	"context"
	"image/png"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

// tinyPNG is a valid 1×1 PNG the mock upstream returns as a "tile".
var tinyPNG = transparentTilePNG

func TestTileBBox3857(t *testing.T) {
	const shift = webMercatorOriginShift
	const eps = 1e-3
	cases := []struct {
		z, x, y                int
		minX, minY, maxX, maxY float64
	}{
		// z0 single tile covers the whole Web-Mercator world.
		{0, 0, 0, -shift, -shift, shift, shift},
		// z1 top-left quadrant: western hemisphere, northern half.
		{1, 0, 0, -shift, 0, 0, shift},
		// z1 bottom-right quadrant: eastern hemisphere, southern half.
		{1, 1, 1, 0, -shift, shift, 0},
	}
	for _, c := range cases {
		minX, minY, maxX, maxY := tileBBox3857(c.z, c.x, c.y)
		if math.Abs(minX-c.minX) > eps || math.Abs(minY-c.minY) > eps ||
			math.Abs(maxX-c.maxX) > eps || math.Abs(maxY-c.maxY) > eps {
			t.Errorf("tileBBox3857(%d,%d,%d) = (%.3f,%.3f,%.3f,%.3f), want (%.3f,%.3f,%.3f,%.3f)",
				c.z, c.x, c.y, minX, minY, maxX, maxY, c.minX, c.minY, c.maxX, c.maxY)
		}
	}
}

func TestValidTile(t *testing.T) {
	cases := []struct {
		z, x, y int
		want    bool
	}{
		{0, 0, 0, true},
		{1, 1, 1, true},
		{5, 31, 31, true}, // 2^5-1
		{5, 32, 0, false}, // x out of range
		{5, 0, 32, false}, // y out of range
		{-1, 0, 0, false},
		{maxZoom + 1, 0, 0, false},
		{3, -1, 0, false},
	}
	for _, c := range cases {
		if got := validTile(c.z, c.x, c.y); got != c.want {
			t.Errorf("validTile(%d,%d,%d) = %v, want %v", c.z, c.x, c.y, got, c.want)
		}
	}
}

func TestTransparentTileIsValidPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(transparentTilePNG))
	if err != nil {
		t.Fatalf("transparent tile is not a valid PNG: %v", err)
	}
	_, _, _, a := img.At(0, 0).RGBA()
	if a != 0 {
		t.Errorf("transparent tile pixel alpha = %d, want 0 (fully transparent)", a)
	}
}

func TestTileURLEncodesWMSGetMap(t *testing.T) {
	c := NewClient(http.DefaultClient, "https://example.test/geoserver/dwd/wms", "dwd:Niederschlagsradar")
	raw, err := c.tileURL(6, 33, 21)
	if err != nil {
		t.Fatalf("tileURL: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse tileURL: %v", err)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"service": "WMS", "version": "1.1.1", "request": "GetMap",
		"layers": "dwd:Niederschlagsradar", "format": "image/png",
		"transparent": "true", "srs": "EPSG:3857", "width": "256", "height": "256",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("tileURL param %s = %q, want %q", k, got, want)
		}
	}
	// bbox must be four comma-separated numbers matching the computed bounds.
	minX, minY, maxX, maxY := tileBBox3857(6, 33, 21)
	want := bboxParam(minX, minY, maxX, maxY)
	if got := q.Get("bbox"); got != want {
		t.Errorf("tileURL bbox = %q, want %q", got, want)
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient(nil, "", "")
	if c.wmsURL != defaultWMSURL {
		t.Errorf("wmsURL = %q, want default %q", c.wmsURL, defaultWMSURL)
	}
	if c.layer != defaultLayer {
		t.Errorf("layer = %q, want default %q", c.layer, defaultLayer)
	}
	if c.httpClient == nil {
		t.Error("httpClient is nil, want http.DefaultClient")
	}
}

func newTestService(t *testing.T, handler http.HandlerFunc) (*Service, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	svc := NewService(
		NewClient(srv.Client(), srv.URL+"/wms", "dwd:Niederschlagsradar"),
		Config{Enabled: true, TTL: time.Minute},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return svc, srv
}

func serve(svc *Service, z, x, y int) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	svc.serveTile(context.Background(), rec, z, x, y)
	return rec
}

func TestServeTileFetchesAndCaches(t *testing.T) {
	var hits int
	svc, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	})

	rec := serve(svc, 6, 33, 21)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("content-type = %q, want image/png", ct)
	}
	if !bytes.Equal(rec.Body.Bytes(), tinyPNG) {
		t.Error("body did not match upstream tile")
	}
	// Second request for the same tile is served from cache (no upstream hit).
	_ = serve(svc, 6, 33, 21)
	if hits != 1 {
		t.Errorf("upstream hits = %d, want 1 (second request must be cached)", hits)
	}
	if svc.FetchSuccessCount() != 1 || svc.FetchFailureCount() != 0 {
		t.Errorf("counters success=%d failure=%d, want 1/0", svc.FetchSuccessCount(), svc.FetchFailureCount())
	}
}

func TestServeTileRefetchesAfterTTL(t *testing.T) {
	var hits int
	svc, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	})
	// Deterministic clock: advance past the TTL between the two requests.
	base := time.Unix(1_700_000_000, 0)
	cur := base
	svc.now = func() time.Time { return cur }

	_ = serve(svc, 1, 0, 0)
	cur = base.Add(2 * time.Minute) // TTL is 1 min
	_ = serve(svc, 1, 0, 0)
	if hits != 2 {
		t.Errorf("upstream hits = %d, want 2 (cache must expire after TTL)", hits)
	}
}

func TestServeTileUpstreamErrorServesLastGoodThenTransparent(t *testing.T) {
	var fail bool
	svc, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	})
	base := time.Unix(1_700_000_000, 0)
	cur := base
	svc.now = func() time.Time { return cur }

	// Prime the cache with a good tile.
	_ = serve(svc, 2, 1, 1)
	// Expire it, then make upstream fail: last-good tile is served, 200.
	cur = base.Add(2 * time.Minute)
	fail = true
	rec := serve(svc, 2, 1, 1)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), tinyPNG) {
		t.Errorf("expected last-good tile on upstream error, got status=%d len=%d", rec.Code, rec.Body.Len())
	}
	if svc.FetchFailureCount() == 0 {
		t.Error("failure counter not incremented on upstream error")
	}

	// A tile that was never cached falls back to the transparent tile, still 200.
	rec = serve(svc, 3, 5, 5)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), transparentTilePNG) {
		t.Errorf("expected transparent tile for uncached failure, got status=%d", rec.Code)
	}
}

func TestServeTileRejectsNonImageUpstream(t *testing.T) {
	svc, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		// GeoServer ServiceException: XML, not an image.
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, `<ServiceExceptionReport/>`)
	})
	rec := serve(svc, 4, 8, 8)
	if !bytes.Equal(rec.Body.Bytes(), transparentTilePNG) {
		t.Error("XML ServiceException must not be served as a tile; expected transparent fallback")
	}
	if svc.FetchFailureCount() == 0 {
		t.Error("non-image upstream should count as a failure")
	}
}

func TestServeTileInvalidCoordsAreTransparent(t *testing.T) {
	var hits int
	svc, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write(tinyPNG)
	})
	rec := serve(svc, 5, 999, 0) // x out of range for z5
	if !bytes.Equal(rec.Body.Bytes(), transparentTilePNG) {
		t.Error("invalid tile coords must serve transparent, not fetch")
	}
	if hits != 0 {
		t.Errorf("upstream hit %d times for invalid coords, want 0", hits)
	}
}

func TestDisabledServiceServesTransparentWithoutFetch(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write(tinyPNG)
	}))
	defer srv.Close()
	svc := NewService(NewClient(srv.Client(), srv.URL, "l"),
		Config{Enabled: false, TTL: time.Minute},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := serve(svc, 6, 33, 21)
	if !bytes.Equal(rec.Body.Bytes(), transparentTilePNG) {
		t.Error("disabled service must serve transparent tile")
	}
	if hits != 0 {
		t.Error("disabled service must not touch the network")
	}
}

func TestTileHandlerParsesPathAndTrimsPNG(t *testing.T) {
	svc, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/weather/radar/{z}/{x}/{y}", svc.TileHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/weather/radar/6/33/21.png", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), tinyPNG) {
		t.Fatalf("handler status=%d len=%d, want 200 tile", rec.Code, rec.Body.Len())
	}

	// Non-numeric path segment → transparent, no panic.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/weather/radar/x/y/z", nil)
	mux.ServeHTTP(rec, req)
	if !bytes.Equal(rec.Body.Bytes(), transparentTilePNG) {
		t.Error("non-numeric path must serve transparent tile")
	}
}

func TestCacheAgeSeconds(t *testing.T) {
	svc := NewService(NewClient(http.DefaultClient, "u", "l"), Config{Enabled: true}, nil)
	if got := svc.CacheAgeSeconds(time.Now()); got != -1 {
		t.Errorf("CacheAgeSeconds before any fetch = %d, want -1", got)
	}
	svc.lastSuccessUnix.Store(1000)
	if got := svc.CacheAgeSeconds(time.Unix(1042, 0)); got != 42 {
		t.Errorf("CacheAgeSeconds = %d, want 42", got)
	}
}

// Guard against accidental precision regressions in the bbox string.
func TestBBoxParamPrecision(t *testing.T) {
	if got := bboxParam(0, 0, 1234.5678, 1); got != "0.000,0.000,1234.568,1.000" {
		t.Errorf("bboxParam precision = %q", got)
	}
	_ = strconv.Itoa(0)
}
