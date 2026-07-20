package basemap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// #310 (K2): Reload switches the upstream style URL at runtime and forces a
// refetch on the next request, so an admin theme/URL change takes effect without
// a restart. The cached last-good style is kept if the new fetch fails.
func TestReloadSwitchesURLAndRefetches(t *testing.T) {
	var aHits, bHits atomic.Int64
	styleA := `{"version":8,"sources":{"s":{"type":"vector","tiles":["https://x/{z}/{x}/{y}"]}},` +
		`"layers":[{"id":"bg","type":"background","paint":{"background-color":"#ffffff"}}],` +
		`"glyphs":"https://up/fonts/{fontstack}/{range}.pbf"}`
	styleB := strings.Replace(styleA, "#ffffff", "#010203", 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/a.json", func(w http.ResponseWriter, _ *http.Request) { aHits.Add(1); _, _ = w.Write([]byte(styleA)) })
	mux.HandleFunc("/b.json", func(w http.ResponseWriter, _ *http.Request) { bHits.Add(1); _, _ = w.Write([]byte(styleB)) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := NewService(srv.Client(), Config{StyleURL: srv.URL + "/a.json"}, nil)
	ctx := context.Background()

	if _, err := svc.StyleJSON(ctx); err != nil {
		t.Fatalf("prime fetch: %v", err)
	}
	if aHits.Load() != 1 {
		t.Fatalf("A hits = %d, want 1", aHits.Load())
	}

	// Reload to B → the next request refetches from the new URL.
	svc.Reload(srv.URL+"/b.json", false)
	if _, err := svc.StyleJSON(ctx); err != nil {
		t.Fatalf("post-reload fetch: %v", err)
	}
	if bHits.Load() != 1 {
		t.Fatalf("B hits = %d after reload, want 1 (refetch did not switch URL)", bHits.Load())
	}
}
