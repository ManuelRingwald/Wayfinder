package weather

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQnhFromRaw(t *testing.T) {
	cases := []struct {
		raw    string
		want   float64
		wantOK bool
	}{
		{"EDDF 021950Z 24008KT ... Q1013 NOSIG", 1013, true},
		{"KMCI 021953Z ... A2992 RMK", 29.92 * inHgToHpa, true}, // inHg → hPa
		{"EDDL ... Q0998", 998, true},
		{"EDDF ... no pressure group", 0, false},
		{"EDDF ... Q0500", 0, false}, // implausible, rejected
	}
	for _, c := range cases {
		got, ok := qnhFromRaw(c.raw)
		if ok != c.wantOK {
			t.Errorf("qnhFromRaw(%q) ok = %v, want %v", c.raw, ok, c.wantOK)
			continue
		}
		if ok && (got < c.want-0.1 || got > c.want+0.1) {
			t.Errorf("qnhFromRaw(%q) = %.2f, want %.2f", c.raw, got, c.want)
		}
	}
}

func TestQnhFromPrefersAltimThenRaw(t *testing.T) {
	altim := 1011.9
	// altim present and sane → used verbatim.
	if got, ok := qnhFrom(metarJSON{Altim: &altim, RawOb: "... Q1013"}); !ok || got != 1011.9 {
		t.Errorf("qnhFrom with altim = %.2f (%v), want 1011.9", got, ok)
	}
	// altim absent → fall back to raw Q group.
	if got, ok := qnhFrom(metarJSON{RawOb: "EDDF ... Q1020 NOSIG"}); !ok || got != 1020 {
		t.Errorf("qnhFrom raw fallback = %.2f (%v), want 1020", got, ok)
	}
	// altim implausible → fall back to raw.
	bad := 5.0
	if got, ok := qnhFrom(metarJSON{Altim: &bad, RawOb: "EDDF ... Q1005"}); !ok || got != 1005 {
		t.Errorf("qnhFrom implausible-altim fallback = %.2f (%v), want 1005", got, ok)
	}
	// nothing usable → not ok.
	if _, ok := qnhFrom(metarJSON{RawOb: "EDDF calm"}); ok {
		t.Error("qnhFrom with no pressure should return ok=false")
	}
}

func metarJSONBody(t *testing.T, recs []map[string]any) string {
	t.Helper()
	b, err := json.Marshal(recs)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFetchMETARParsesAndSkipsBad(t *testing.T) {
	var gotUA, gotIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotIDs = r.URL.Query().Get("ids")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, metarJSONBody(t, []map[string]any{
			{"icaoId": "EDDF", "altim": 1013.0, "obsTime": 1700000000, "rawOb": "EDDF ... Q1013"},
			{"icaoId": "EDDL", "rawOb": "EDDL ... Q0998"}, // no altim, raw fallback
			{"icaoId": "EDXX", "rawOb": "EDXX ... calm"},  // no QNH → skipped
			{"icaoId": "", "altim": 1010.0},               // no id → skipped
		}))
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), srv.URL, "TestAgent/1.0")
	reports, err := c.FetchMETAR(context.Background(), []string{"EDDF", "EDDL", "EDXX"})
	if err != nil {
		t.Fatalf("FetchMETAR: %v", err)
	}
	if gotUA != "TestAgent/1.0" {
		t.Errorf("User-Agent = %q, want TestAgent/1.0", gotUA)
	}
	if gotIDs != "EDDF,EDDL,EDXX" {
		t.Errorf("ids param = %q, want EDDF,EDDL,EDXX", gotIDs)
	}
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want 2 (bad records skipped)", len(reports))
	}
	if reports[0].ICAO != "EDDF" || reports[0].QNHHpa != 1013 || reports[0].ObsTimeUnix != 1700000000 {
		t.Errorf("report[0] = %+v, want EDDF/1013/1700000000", reports[0])
	}
	if reports[1].ICAO != "EDDL" || reports[1].QNHHpa != 998 {
		t.Errorf("report[1] = %+v, want EDDL/998", reports[1])
	}
}

func TestFetchMETARNon200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()
	c := NewClient(srv.Client(), srv.URL, "UA")
	if _, err := c.FetchMETAR(context.Background(), []string{"EDDF"}); err == nil {
		t.Error("expected error on non-200, got nil")
	}
}

func TestFetchMETARGarbageIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not json at all")
	}))
	defer srv.Close()
	c := NewClient(srv.Client(), srv.URL, "UA")
	if _, err := c.FetchMETAR(context.Background(), []string{"EDDF"}); err == nil {
		t.Error("expected decode error on garbage, got nil")
	}
}

func newQNHService(t *testing.T, stations []string, handler http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewService(
		NewClient(srv.Client(), srv.URL, "UA"),
		Config{Enabled: true, Stations: stations, Refresh: time.Minute, StaleAfter: 2 * time.Hour},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func TestSnapshotOrderPrimaryAndStale(t *testing.T) {
	svc := newQNHService(t, []string{"EDDF", "EDDL"}, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, metarJSONBody(t, []map[string]any{
			{"icaoId": "EDDF", "altim": 1013.4, "obsTime": 1_000_000},
			{"icaoId": "EDDL", "altim": 1008.6, "obsTime": 1_100_000},
		}))
	})
	// Fixed clock: EDDF obs is > 2h old (stale), EDDL is recent.
	svc.now = func() time.Time { return time.Unix(1_100_000+60, 0) }
	svc.refreshOnce(context.Background())

	snap := svc.Snapshot()
	if len(snap.Stations) != 2 {
		t.Fatalf("stations = %d, want 2", len(snap.Stations))
	}
	// Priority order preserved: EDDF first (primary).
	if snap.Primary == nil || snap.Primary.ICAO != "EDDF" {
		t.Fatalf("primary = %+v, want EDDF first", snap.Primary)
	}
	// Rounded to whole hPa.
	if snap.Stations[0].QNHHpa != 1013 || snap.Stations[1].QNHHpa != 1009 {
		t.Errorf("rounded QNH = %d/%d, want 1013/1009", snap.Stations[0].QNHHpa, snap.Stations[1].QNHHpa)
	}
	if !snap.Stations[0].Stale {
		t.Error("EDDF should be stale (obs > 2h old)")
	}
	if snap.Stations[1].Stale {
		t.Error("EDDL should be fresh")
	}
}

func TestSnapshotOmitsUnreadStations(t *testing.T) {
	svc := newQNHService(t, []string{"EDDF", "EDDL"}, func(w http.ResponseWriter, r *http.Request) {
		// only EDDF comes back
		_, _ = io.WriteString(w, metarJSONBody(t, []map[string]any{
			{"icaoId": "EDDF", "altim": 1013.0, "obsTime": 1_100_000},
		}))
	})
	svc.now = func() time.Time { return time.Unix(1_100_000, 0) }
	svc.refreshOnce(context.Background())
	snap := svc.Snapshot()
	if len(snap.Stations) != 1 || snap.Stations[0].ICAO != "EDDF" {
		t.Errorf("snapshot = %+v, want only EDDF", snap.Stations)
	}
}

func TestRefreshFailureKeepsLastGood(t *testing.T) {
	var fail bool
	svc := newQNHService(t, []string{"EDDF"}, func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, metarJSONBody(t, []map[string]any{
			{"icaoId": "EDDF", "altim": 1013.0, "obsTime": 1_100_000},
		}))
	})
	svc.now = func() time.Time { return time.Unix(1_100_000, 0) }
	svc.refreshOnce(context.Background())
	fail = true
	svc.refreshOnce(context.Background()) // failure: last-good must survive
	if svc.FetchFailureCount() == 0 {
		t.Error("failure counter not incremented")
	}
	if snap := svc.Snapshot(); len(snap.Stations) != 1 || snap.Stations[0].QNHHpa != 1013 {
		t.Errorf("last-good QNH lost on failure: %+v", snap.Stations)
	}
}

func TestDisabledServiceServesEmpty(t *testing.T) {
	svc := NewService(NewClient(http.DefaultClient, "u", "ua"),
		Config{Enabled: true, Stations: nil}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if svc.enabled {
		t.Error("service with no stations must be disabled")
	}
	rec := httptest.NewRecorder()
	svc.Handler()(rec, httptest.NewRequest(http.MethodGet, "/api/weather/qnh", nil))
	var resp qnhResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Stations) != 0 || resp.Primary != nil {
		t.Errorf("disabled service should serve empty, got %+v", resp)
	}
}

func TestNormaliseStations(t *testing.T) {
	got := normaliseStations([]string{" eddf ", "EDDL", "eddf", "", "eddl"})
	if len(got) != 2 || got[0] != "EDDF" || got[1] != "EDDL" {
		t.Errorf("normaliseStations = %v, want [EDDF EDDL]", got)
	}
}
