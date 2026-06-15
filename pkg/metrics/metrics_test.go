package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandlerRendersPrometheusExpositionFormat verifies that Handler writes
// HELP/TYPE/value lines for each metric, in Prometheus text exposition
// format (REQ NFR-OBS-002).
func TestHandlerRendersPrometheusExpositionFormat(t *testing.T) {
	h := Handler(
		Counter("wayfinder_cat062_blocks_received_total", "Total blocks received.", 3),
		Gauge("wayfinder_tracks_current", "Current track count.", 2),
	)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	wantLines := []string{
		"# HELP wayfinder_cat062_blocks_received_total Total blocks received.",
		"# TYPE wayfinder_cat062_blocks_received_total counter",
		"wayfinder_cat062_blocks_received_total 3",
		"# HELP wayfinder_tracks_current Current track count.",
		"# TYPE wayfinder_tracks_current gauge",
		"wayfinder_tracks_current 2",
	}
	for _, want := range wantLines {
		if !strings.Contains(body, want) {
			t.Errorf("response body missing line %q\ngot:\n%s", want, body)
		}
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Content-Type: got %q, want text/plain prefix", contentType)
	}
}
