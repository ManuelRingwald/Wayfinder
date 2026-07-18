package basemap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// upstreamStyle builds a minimal but realistic basemap.de-shaped style served
// by the test upstream: relative sprite/tiles, its own glyphs endpoint, a BKG
// fontstack in use, no attribution.
const upstreamStyleTemplate = `{
	"version": 8,
	"name": "bm_web_col",
	"glyphs": "%HOST%/gdz_basemapde_vektor/fonts/{fontstack}/{range}.pbf",
	"sprite": "../sprites/bm_web_col",
	"sources": {
		"basemap": {
			"type": "vector",
			"tiles": ["/gdz_basemapde_vektor/tiles/v1/bm_web_de_3857/{z}/{x}/{y}.pbf"]
		}
	},
	"layers": [
		{"id": "background", "type": "background"},
		{"id": "place", "type": "symbol", "layout": {"text-font": ["BM Web Regular"]}}
	]
}`

// newUpstream serves the style at /styles/bm_web_col.json and glyph PBFs for
// the "BM Web Regular" fontstack. styleHits/glyphHits count upstream traffic so
// tests can assert caching behaviour.
func newUpstream(t *testing.T, styleHits, glyphHits *atomic.Int64) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/styles/bm_web_col.json", func(w http.ResponseWriter, r *http.Request) {
		styleHits.Add(1)
		body := strings.ReplaceAll(upstreamStyleTemplate, "%HOST%", srv.URL)
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc("/gdz_basemapde_vektor/fonts/", func(w http.ResponseWriter, r *http.Request) {
		glyphHits.Add(1)
		if strings.Contains(r.URL.Path, "Missing Font") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("PBFDATA:" + r.URL.Path))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestService(t *testing.T, srv *httptest.Server) *Service {
	t.Helper()
	return NewService(srv.Client(), Config{StyleURL: srv.URL + "/styles/bm_web_col.json"}, nil)
}

func getStyle(t *testing.T, svc *Service) (int, map[string]any, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	svc.StyleHandler()(rec, httptest.NewRequest(http.MethodGet, "/basemap/style.json", nil))
	if rec.Code != http.StatusOK {
		return rec.Code, nil, rec.Body.String()
	}
	var style map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &style); err != nil {
		t.Fatalf("served style is not JSON: %v", err)
	}
	return rec.Code, style, rec.Body.String()
}

func TestStyleRewritePointsGlyphsAtWayfinder(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := newTestService(t, srv)

	code, style, _ := getStyle(t, svc)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if g := style["glyphs"]; g != localGlyphsTemplate {
		t.Errorf("glyphs = %v, want %q", g, localGlyphsTemplate)
	}
}

func TestStyleRewriteAbsolutizesRelativeURLs(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := newTestService(t, srv)

	_, style, _ := getStyle(t, svc)
	// "../sprites/bm_web_col" relative to /styles/bm_web_col.json → /sprites/…
	if sprite := style["sprite"]; sprite != srv.URL+"/sprites/bm_web_col" {
		t.Errorf("sprite = %v, want %q", sprite, srv.URL+"/sprites/bm_web_col")
	}
	src := style["sources"].(map[string]any)["basemap"].(map[string]any)
	tiles := src["tiles"].([]any)
	wantTiles := srv.URL + "/gdz_basemapde_vektor/tiles/v1/bm_web_de_3857/{z}/{x}/{y}.pbf"
	if tiles[0] != wantTiles {
		t.Errorf("tiles[0] = %v, want %q", tiles[0], wantTiles)
	}
}

func TestStyleRewriteInjectsAttributionWhenMissing(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := newTestService(t, srv)

	_, style, _ := getStyle(t, svc)
	src := style["sources"].(map[string]any)["basemap"].(map[string]any)
	if src["attribution"] != defaultAttribution {
		t.Errorf("attribution = %v, want %q", src["attribution"], defaultAttribution)
	}
}

func TestStyleRewriteKeepsExistingAttribution(t *testing.T) {
	raw := []byte(`{"version":8,"sources":{"a":{"type":"vector","attribution":"© custom"}}}`)
	out, _, err := rewriteStyle(raw, "https://example.com/styles/s.json")
	if err != nil {
		t.Fatalf("rewriteStyle: %v", err)
	}
	var style map[string]any
	_ = json.Unmarshal(out, &style)
	src := style["sources"].(map[string]any)["a"].(map[string]any)
	if src["attribution"] != "© custom" {
		t.Errorf("attribution overwritten: %v", src["attribution"])
	}
}

func TestStyleCachedWithinTTLAndStaleOnFailure(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := newTestService(t, srv)

	now := time.Unix(1_700_000_000, 0)
	svc.now = func() time.Time { return now }

	if code, _, _ := getStyle(t, svc); code != http.StatusOK {
		t.Fatalf("first fetch failed")
	}
	if code, _, _ := getStyle(t, svc); code != http.StatusOK {
		t.Fatalf("second fetch failed")
	}
	if styleHits.Load() != 1 {
		t.Errorf("upstream hit %d times within TTL, want 1", styleHits.Load())
	}

	// TTL expiry + upstream outage → stale style still served.
	srv.Close()
	now = now.Add(defaultStyleTTL + time.Minute)
	code, style, _ := getStyle(t, svc)
	if code != http.StatusOK {
		t.Fatalf("stale fallback: expected 200, got %d", code)
	}
	if style["glyphs"] != localGlyphsTemplate {
		t.Errorf("stale style lost rewrite")
	}
	if svc.FetchFailureCount() == 0 {
		t.Errorf("expected a recorded fetch failure")
	}
}

func TestStyleUnavailableWithoutCacheIs502(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	svc := NewService(srv.Client(), Config{StyleURL: srv.URL + "/style.json"}, nil)
	code, _, _ := getStyle(t, svc)
	if code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", code)
	}
}

func TestStyleRejectsInvalidUpstreamJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	}))
	defer srv.Close()
	svc := NewService(srv.Client(), Config{StyleURL: srv.URL + "/style.json"}, nil)
	code, _, _ := getStyle(t, svc)
	if code != http.StatusBadGateway {
		t.Errorf("expected 502 on non-JSON upstream, got %d", code)
	}
}

// glyphEnv wires a Service with a local fallback handler the way main.go does.
func glyphEnv(t *testing.T, svc *Service) (http.Handler, *atomic.Int64) {
	t.Helper()
	var localHits atomic.Int64
	local := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localHits.Add(1)
		_, _ = w.Write([]byte("LOCAL"))
	})
	isLocal := func(fontstack string) bool { return fontstack == "Roboto Mono Medium" }
	return svc.GlyphsHandler(local, isLocal), &localHits
}

func TestGlyphsLocalFontstackServedLocally(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := newTestService(t, srv)
	h, localHits := glyphEnv(t, svc)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/glyphs/Roboto%20Mono%20Medium/0-255.pbf", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "LOCAL" {
		t.Errorf("local fontstack not served locally: %d %q", rec.Code, rec.Body.String())
	}
	if localHits.Load() != 1 || glyphHits.Load() != 0 {
		t.Errorf("local=%d upstream=%d, want 1/0", localHits.Load(), glyphHits.Load())
	}
}

func TestGlyphsUnknownFontstackProxiedAndCached(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := newTestService(t, srv)
	h, localHits := glyphEnv(t, svc)

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/glyphs/BM%20Web%20Regular/0-255.pbf", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("proxy attempt %d: HTTP %d", i, rec.Code)
		}
		if !strings.HasPrefix(rec.Body.String(), "PBFDATA:") {
			t.Fatalf("unexpected body %q", rec.Body.String())
		}
		// The escaped fontstack must reach the upstream path (percent-decoded by
		// the upstream HTTP server back to spaces).
		if !strings.Contains(rec.Body.String(), "/fonts/BM Web Regular/0-255.pbf") {
			t.Errorf("upstream URL wrong: %q", rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/x-protobuf" {
			t.Errorf("Content-Type = %q", ct)
		}
	}
	if glyphHits.Load() != 1 {
		t.Errorf("upstream glyph hit %d times, want 1 (cache)", glyphHits.Load())
	}
	if localHits.Load() != 0 {
		t.Errorf("local handler hit for proxied fontstack")
	}
}

func TestGlyphsUpstreamErrorIs502(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := newTestService(t, srv)
	h, _ := glyphEnv(t, svc)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/glyphs/Missing%20Font/0-255.pbf", nil))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for upstream 404, got %d", rec.Code)
	}
}

func TestGlyphsInvalidPathsRejected(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := newTestService(t, srv)
	h, _ := glyphEnv(t, svc)

	for _, p := range []string{
		"/glyphs/",
		"/glyphs/OnlyStack",
		"/glyphs/Stack/0-255.pbf/extra",
		"/glyphs/../secret/0-255.pbf",
		"/glyphs/Stack/notarange.pbf",
		"/glyphs/Stack/0-255.png",
		"/glyphs/Sta\x01ck/0-255.pbf",
	} {
		rec := httptest.NewRecorder()
		// Build the request directly: the handler sees the DECODED URL.Path, and
		// some of these raw paths (control chars) are unrepresentable as a valid
		// request target for httptest.NewRequest.
		h.ServeHTTP(rec, &http.Request{Method: http.MethodGet, URL: &url.URL{Path: p}})
		if rec.Code != http.StatusNotFound {
			t.Errorf("%q: expected 404, got %d", p, rec.Code)
		}
	}
	if glyphHits.Load() != 0 {
		t.Errorf("invalid paths must never reach the upstream (hit %d times)", glyphHits.Load())
	}
}

func TestGlyphsWithoutUpstreamFallThroughToLocal(t *testing.T) {
	// Upstream style has no glyphs key → no proxy template → local fallback.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":8,"sources":{}}`))
	}))
	defer srv.Close()
	svc := NewService(srv.Client(), Config{StyleURL: srv.URL + "/style.json"}, nil)
	h, localHits := glyphEnv(t, svc)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/glyphs/BM%20Web%20Regular/0-255.pbf", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "LOCAL" {
		t.Errorf("expected local fallback, got %d %q", rec.Code, rec.Body.String())
	}
	if localHits.Load() != 1 {
		t.Errorf("local handler not used")
	}
}

func TestGlyphCacheBounded(t *testing.T) {
	var styleHits, glyphHits atomic.Int64
	srv := newUpstream(t, &styleHits, &glyphHits)
	svc := newTestService(t, srv)
	h, _ := glyphEnv(t, svc)

	for i := 0; i < maxGlyphCacheEntries+10; i++ {
		rec := httptest.NewRecorder()
		p := "/glyphs/BM%20Web%20Regular/" + strconv.Itoa(i*256) + "-" + strconv.Itoa(i*256+255) + ".pbf"
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("glyph %d: HTTP %d", i, rec.Code)
		}
	}
	svc.glyphMu.Lock()
	n := len(svc.glyphCache)
	svc.glyphMu.Unlock()
	if n > maxGlyphCacheEntries {
		t.Errorf("glyph cache grew to %d entries (cap %d)", n, maxGlyphCacheEntries)
	}
}
