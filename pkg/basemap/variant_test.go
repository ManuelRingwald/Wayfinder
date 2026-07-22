package basemap

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// T2 (ADR 0035): StyleJSONFor serves a per-(styleURL, dark) variant so tenants can
// have their own base map. This test proves: (a) the dark variant differs from the
// light one (the transform runs), (b) each variant is cached (no refetch within
// TTL), and (c) requesting the service default routes through the default cache.
func TestStyleJSONForVariants(t *testing.T) {
	var aHits, bHits atomic.Int64
	// A minimal style with a coloured background so the dark transform changes bytes.
	styleA := `{"version":8,"sources":{"s":{"type":"vector","tiles":["https://x/{z}/{x}/{y}"]}},` +
		`"layers":[{"id":"bg","type":"background","paint":{"background-color":"#ffffff"}}],` +
		`"glyphs":"https://up/fonts/{fontstack}/{range}.pbf"}`
	styleB := strings.Replace(styleA, "#ffffff", "#abcdef", 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/a.json", func(w http.ResponseWriter, _ *http.Request) { aHits.Add(1); _, _ = w.Write([]byte(styleA)) })
	mux.HandleFunc("/b.json", func(w http.ResponseWriter, _ *http.Request) { bHits.Add(1); _, _ = w.Write([]byte(styleB)) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Service default = /a.json, light.
	svc := NewService(srv.Client(), Config{StyleURL: srv.URL + "/a.json", Dark: false}, nil)
	ctx := context.Background()

	// Light and dark of the SAME url must differ (dark transform applied).
	light, err := svc.StyleJSONFor(ctx, srv.URL+"/a.json", false)
	if err != nil {
		t.Fatalf("light: %v", err)
	}
	dark, err := svc.StyleJSONFor(ctx, srv.URL+"/a.json", true)
	if err != nil {
		t.Fatalf("dark: %v", err)
	}
	if bytes.Equal(light, dark) {
		t.Fatal("dark variant should differ from light (transform not applied)")
	}

	// The light request matched the default → default cache (aHits was 1 from it).
	// The dark request is a non-default variant → one more fetch of /a.json.
	if got := aHits.Load(); got != 2 {
		t.Fatalf("A hits = %d, want 2 (default light + dark variant)", got)
	}

	// Cached: a second dark request within TTL does not refetch.
	if _, err := svc.StyleJSONFor(ctx, srv.URL+"/a.json", true); err != nil {
		t.Fatal(err)
	}
	if got := aHits.Load(); got != 2 {
		t.Fatalf("A hits = %d after cache hit, want still 2", got)
	}

	// A different URL variant is fetched from that URL.
	if _, err := svc.StyleJSONFor(ctx, srv.URL+"/b.json", true); err != nil {
		t.Fatal(err)
	}
	if got := bHits.Load(); got != 1 {
		t.Fatalf("B hits = %d, want 1", got)
	}

	// An empty style URL resolves to the service default (no extra fetch — cached).
	if _, err := svc.StyleJSONFor(ctx, "", false); err != nil {
		t.Fatal(err)
	}
	if got := aHits.Load(); got != 2 {
		t.Fatalf("A hits = %d after default request, want still 2", got)
	}
}

// A non-default variant serves its stale last-good when a refetch fails, so a bad
// tenant style URL never blanks that tenant's scope.
func TestStyleJSONForVariantStaleOnFailure(t *testing.T) {
	var fail atomic.Bool
	style := `{"version":8,"sources":{},"layers":[{"id":"bg","type":"background","paint":{"background-color":"#123456"}}]}`
	mux := http.NewServeMux()
	mux.HandleFunc("/v.json", func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(style))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Default is a different URL so /v.json is a non-default variant.
	svc := NewService(srv.Client(), Config{StyleURL: "https://other.example/style.json"}, nil)
	svc.ttl = 0 // every request re-checks the TTL → forces the refetch path
	ctx := context.Background()

	first, err := svc.StyleJSONFor(ctx, srv.URL+"/v.json", false)
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	// Now upstream fails; the variant must serve its stale last-good.
	fail.Store(true)
	got, err := svc.StyleJSONFor(ctx, srv.URL+"/v.json", false)
	if err != nil {
		t.Fatalf("stale serve should not error: %v", err)
	}
	if !bytes.Equal(first, got) {
		t.Fatal("expected the stale last-good variant bytes")
	}
}
