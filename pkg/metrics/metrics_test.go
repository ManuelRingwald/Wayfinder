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

// TestHandlerRendersLabels verifies labelled series (WF2-23.2): HELP/TYPE are
// emitted once per name, each series carries its `{tenant="…"}` label, and label
// values are escaped per the Prometheus text format.
func TestHandlerRendersLabels(t *testing.T) {
	h := Handler(
		Gauge("wayfinder_tenant_ws_clients_connected", "Connected clients per tenant.", 2).With(Label{"tenant", "7"}),
		Gauge("wayfinder_tenant_ws_clients_connected", "Connected clients per tenant.", 5).With(Label{"tenant", "42"}),
		Counter("escaped", "Escaping check.", 1).With(Label{"v", `a"b\c`}),
	)
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`wayfinder_tenant_ws_clients_connected{tenant="7"} 2`,
		`wayfinder_tenant_ws_clients_connected{tenant="42"} 5`,
		`escaped{v="a\"b\\c"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\ngot:\n%s", want, body)
		}
	}

	// HELP/TYPE for the shared name must appear exactly once.
	if n := strings.Count(body, "# TYPE wayfinder_tenant_ws_clients_connected gauge"); n != 1 {
		t.Errorf("TYPE line count for shared name = %d, want 1", n)
	}
}
