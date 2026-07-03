package weatherwarnings

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestSeverityLevel(t *testing.T) {
	cases := map[string]int{
		"Minor": 1, "moderate": 2, "SEVERE": 3, "Extreme": 4,
		"": 2, "unknown": 2, "  severe  ": 3,
	}
	for in, want := range cases {
		if got := severityLevel(in); got != want {
			t.Errorf("severityLevel(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestNormalisePropsCaseInsensitive(t *testing.T) {
	props := normaliseProps(map[string]any{
		"SEVERITY": "Severe",
		"headline": "Amtliche WARNUNG vor STURMBÖEN",
		"EVENT":    "Wind",
		"expires":  "2026-07-02T18:00:00Z",
		"ignored":  123,
	})
	if props["wf_level"] != 3 {
		t.Errorf("wf_level = %v, want 3", props["wf_level"])
	}
	if props["headline"] != "Amtliche WARNUNG vor STURMBÖEN" {
		t.Errorf("headline = %v", props["headline"])
	}
	if props["event"] != "Wind" {
		t.Errorf("event = %v", props["event"])
	}
	if props["expires"] != "2026-07-02T18:00:00Z" {
		t.Errorf("expires = %v", props["expires"])
	}
}

func TestRequestURLEncodesWFS(t *testing.T) {
	c := NewClient(http.DefaultClient, "https://example.test/geoserver/dwd/ows", "dwd:Warnungen_Gemeinden_vereinigt")
	raw, err := c.requestURL()
	if err != nil {
		t.Fatalf("requestURL: %v", err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	for k, want := range map[string]string{
		"service": "WFS", "version": "2.0.0", "request": "GetFeature",
		"typeName":     "dwd:Warnungen_Gemeinden_vereinigt",
		"outputFormat": "application/json", "srsName": "EPSG:4326",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("param %s = %q, want %q", k, got, want)
		}
	}
}

const sampleWFS = `{
  "type": "FeatureCollection",
  "features": [
    {"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[8,50],[9,50],[9,51],[8,50]]]},
     "properties":{"SEVERITY":"Severe","HEADLINE":"Sturm","EVENT":"Wind"}},
    {"type":"Feature","geometry":null,"properties":{"SEVERITY":"Minor"}},
    {"type":"Feature","geometry":{"type":"Polygon","coordinates":[]},"properties":{"SEVERITY":"Extreme"}}
  ]
}`

func TestFetchParsesAndDropsBadGeometry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, sampleWFS)
	}))
	defer srv.Close()
	c := NewClient(srv.Client(), srv.URL, "dwd:Warnungen_Gemeinden_vereinigt")
	fc, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(fc.Features) != 1 {
		t.Fatalf("features = %d, want 1 (null + empty-coords dropped)", len(fc.Features))
	}
	if fc.Features[0].Properties["wf_level"] != 3 {
		t.Errorf("wf_level = %v, want 3", fc.Features[0].Properties["wf_level"])
	}
	// Serialisable back to GeoJSON.
	if _, err := json.Marshal(fc); err != nil {
		t.Errorf("marshal: %v", err)
	}
}

func TestFetchNon200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewClient(srv.Client(), srv.URL, "l")
	if _, err := c.Fetch(context.Background()); err == nil {
		t.Error("expected error on non-200")
	}
}

func newTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewService(NewClient(srv.Client(), srv.URL, "l"),
		Config{Enabled: true, Refresh: time.Minute},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestServiceServesEmptyBeforeFirstFetch(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	svc.Handler()(rec, httptest.NewRequest(http.MethodGet, "/api/weather/warnings.geojson", nil))
	var fc FeatureCollection
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fc.Type != "FeatureCollection" || len(fc.Features) != 0 {
		t.Errorf("empty serve = %+v, want empty FeatureCollection", fc)
	}
}

func TestRefreshCachesAndFailureKeepsLastGood(t *testing.T) {
	var fail bool
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, sampleWFS)
	})
	svc.refreshOnce(context.Background())
	if len(svc.Serve().Features) != 1 {
		t.Fatalf("after refresh: features = %d, want 1", len(svc.Serve().Features))
	}
	fail = true
	svc.refreshOnce(context.Background())
	if svc.FetchFailureCount() == 0 {
		t.Error("failure counter not incremented")
	}
	if len(svc.Serve().Features) != 1 {
		t.Error("last-good cache lost on failure")
	}
}

func TestDisabledServiceServesEmpty(t *testing.T) {
	svc := NewService(NewClient(http.DefaultClient, "u", "l"), Config{Enabled: false}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// Run returns immediately when disabled.
	svc.Run(context.Background())
	if len(svc.Serve().Features) != 0 {
		t.Error("disabled service should serve empty")
	}
}

func TestCacheAgeSeconds(t *testing.T) {
	svc := NewService(NewClient(http.DefaultClient, "u", "l"), Config{Enabled: true}, nil)
	if got := svc.CacheAgeSeconds(time.Now()); got != -1 {
		t.Errorf("CacheAgeSeconds before fetch = %d, want -1", got)
	}
	svc.lastSuccessUnix.Store(1000)
	if got := svc.CacheAgeSeconds(time.Unix(1005, 0)); got != 5 {
		t.Errorf("CacheAgeSeconds = %d, want 5", got)
	}
}
