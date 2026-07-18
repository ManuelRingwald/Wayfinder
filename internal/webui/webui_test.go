package webui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// get drives the embedded handler and returns status, content-type and body.
func get(t *testing.T, h http.Handler, target string) (int, string, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, target, nil))
	res := rr.Result()
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header.Get("Content-Type"), string(body)
}

// indexMarker is a stable fragment of the SPA shell (the mount point Vue uses).
const indexMarker = `id="app"`

func TestServesIndexAtRoot(t *testing.T) {
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	status, ct, body := get(t, h, "/")
	if status != http.StatusOK {
		t.Fatalf("root status = %d, want 200", status)
	}
	if !strings.Contains(ct, "text/html") {
		t.Errorf("root content-type = %q, want text/html", ct)
	}
	if !strings.Contains(body, indexMarker) {
		t.Errorf("root body missing %q", indexMarker)
	}
}

// TestSPAFallbackServesShell is the core WF2-32 guarantee: client-side deep links
// that have no corresponding embedded file must return the SPA shell (history
// mode), not a 404 — otherwise a hard reload of /admin breaks.
func TestSPAFallbackServesShell(t *testing.T) {
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	for _, target := range []string{
		"/admin",
		"/admin/",
		"/admin/tenants/5/subscriptions",
		"/some/unknown/deep/link",
		"/assets", // bare directory: falls through to the shell, no listing
	} {
		status, ct, body := get(t, h, target)
		if status != http.StatusOK {
			t.Errorf("%s status = %d, want 200", target, status)
		}
		if !strings.Contains(ct, "text/html") {
			t.Errorf("%s content-type = %q, want text/html", target, ct)
		}
		if !strings.Contains(body, indexMarker) {
			t.Errorf("%s did not return the SPA shell", target)
		}
	}
}

// TestServesRealAsset checks a genuine embedded asset is served as itself, not
// shadowed by the fallback. favicon.svg ships in dist/ from the Vite build.
func TestServesRealAsset(t *testing.T) {
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	status, ct, body := get(t, h, "/favicon.svg")
	if status != http.StatusOK {
		t.Fatalf("favicon status = %d, want 200", status)
	}
	if strings.Contains(ct, "text/html") || strings.Contains(body, indexMarker) {
		t.Errorf("favicon.svg was shadowed by the SPA shell (ct=%q)", ct)
	}
}

// TestUnknownAssetFallsBack documents the deliberate "all not-found → index.html"
// rule: even a missing path under /assets returns the shell rather than a 404.
func TestUnknownAssetFallsBack(t *testing.T) {
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	status, _, body := get(t, h, "/assets/does-not-exist-12345.js")
	if status != http.StatusOK || !strings.Contains(body, indexMarker) {
		t.Errorf("missing asset status=%d shell=%v, want 200+shell", status, strings.Contains(body, indexMarker))
	}
}

// TestGlyphsHandlerServesPBF is the G4 guarantee: the self-hosted MapLibre glyph
// endpoint returns the embedded Roboto Mono PBF for a generated range (so the
// scope renders its data blocks in the monospace face with no font CDN). The
// fontstack segment carries spaces and arrives percent-encoded from the map
// style's {fontstack} expansion.
func TestGlyphsHandlerServesPBF(t *testing.T) {
	h, err := GlyphsHandler()
	if err != nil {
		t.Fatalf("GlyphsHandler: %v", err)
	}
	status, ct, body := get(t, h, "/glyphs/Roboto%20Mono%20Medium/0-255.pbf")
	if status != http.StatusOK {
		t.Fatalf("glyph range status = %d, want 200", status)
	}
	if ct != "application/x-protobuf" {
		t.Errorf("glyph content-type = %q, want application/x-protobuf", ct)
	}
	if len(body) == 0 {
		t.Errorf("glyph body is empty")
	}
}

// TestGlyphsHandlerNotFound covers the degradation + safety paths: an
// ungenerated range 404s (MapLibre then renders those code points blank), a
// non-.pbf request 404s, and a traversal attempt cannot escape the embed FS.
func TestGlyphsHandlerNotFound(t *testing.T) {
	h, err := GlyphsHandler()
	if err != nil {
		t.Fatalf("GlyphsHandler: %v", err)
	}
	for _, target := range []string{
		"/glyphs/Roboto%20Mono%20Medium/2048-2303.pbf", // range not generated
		"/glyphs/Unknown%20Font/0-255.pbf",             // fontstack we do not host
		"/glyphs/Roboto%20Mono%20Medium/0-255.txt",     // not a .pbf
		"/glyphs/../dist/index.html",                   // traversal → cleaned, no .pbf
	} {
		if status, _, _ := get(t, h, target); status != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", target, status)
		}
	}
}

// TestGlyphFontstacks: the basemap glyph proxy (ADR 0026) routes requests
// local-vs-upstream based on this list, so it must name exactly the embedded
// fontstack directories.
func TestGlyphFontstacks(t *testing.T) {
	stacks, err := GlyphFontstacks()
	if err != nil {
		t.Fatalf("GlyphFontstacks: %v", err)
	}
	if len(stacks) != 1 || stacks[0] != "Roboto Mono Medium" {
		t.Errorf("stacks = %v, want [\"Roboto Mono Medium\"]", stacks)
	}
}
