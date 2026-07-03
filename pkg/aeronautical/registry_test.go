package aeronautical

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newTestRegistry builds a Registry whose client factory points every per-tenant
// service at the given OpenAIP test server, and a global fallback service.
func newTestRegistry(t *testing.T, serverURL string, global *Service) *Registry {
	t.Helper()
	factory := func(apiKey string) *Client {
		return NewClient(http.DefaultClient, serverURL, apiKey)
	}
	return NewRegistry(context.Background(), global, factory, nil, nil)
}

func TestRegistryServesPerTenantCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()

	reg := newTestRegistry(t, srv.URL, nil)
	defer reg.StopAll()

	reg.Start(7, "tenant-key", BoundingBox{}, false)
	// The per-tenant service fetches once on Start; wait for the cache to warm.
	waitFor(t, func() bool { return len(reg.Serve(7, KindNavaid).Features) == 2 })

	if got := len(reg.Serve(7, KindNavaid).Features); got != 2 {
		t.Fatalf("tenant 7 navaids = %d, want 2", got)
	}
}

func TestRegistryEmptyKeyFallsBackToGlobal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()

	// Global fallback service warmed from the same server.
	global := NewService(NewClient(srv.Client(), srv.URL, "global-key"), Config{Enabled: true}, nil)
	global.refreshAll(context.Background())

	reg := newTestRegistry(t, srv.URL, global)
	defer reg.StopAll()

	// No per-tenant key: Start(tenant, "", …) must not run a service, and Serve
	// must transparently fall back to the global cache.
	reg.Start(7, "", BoundingBox{}, false)
	if reg.IsRunning(7) {
		t.Fatal("an empty key must not start a per-tenant service")
	}
	if got := len(reg.Serve(7, KindNavaid).Features); got != 2 {
		t.Fatalf("tenant 7 (no key) should fall back to global cache (2), got %d", got)
	}
}

func TestRegistryStartIdempotentOnUnchangedInputs(t *testing.T) {
	var builds atomic.Int64
	factory := func(apiKey string) *Client {
		builds.Add(1)
		return NewClient(http.DefaultClient, "http://unused", apiKey)
	}
	reg := NewRegistry(context.Background(), nil, factory, nil, nil)
	defer reg.StopAll()

	bbox := BoundingBox{MinLon: 1, MinLat: 2, MaxLon: 3, MaxLat: 4}
	reg.Start(7, "k", bbox, false)
	reg.Start(7, "k", bbox, false) // identical → no-op
	reg.Start(7, "k", bbox, false)
	if builds.Load() != 1 {
		t.Fatalf("expected exactly one client build for unchanged inputs, got %d", builds.Load())
	}

	// A changed key restarts (new build).
	reg.Start(7, "k2", bbox, false)
	if builds.Load() != 2 {
		t.Fatalf("a changed key must restart the service (build), got %d builds", builds.Load())
	}
}

func TestRegistryStopFallsBackToGlobal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()

	reg := newTestRegistry(t, srv.URL, nil)
	defer reg.StopAll()

	reg.Start(7, "k", BoundingBox{}, false)
	waitFor(t, func() bool { return reg.IsRunning(7) })
	if !reg.Stop(7) {
		t.Fatal("Stop should report true for a running tenant")
	}
	if reg.IsRunning(7) {
		t.Fatal("tenant service should be gone after Stop")
	}
	if reg.Stop(7) {
		t.Fatal("Stop on an unknown tenant should report false")
	}
	// With no global and no per-tenant service, Serve degrades to empty.
	if got := len(reg.Serve(7, KindNavaid).Features); got != 0 {
		t.Fatalf("after Stop with no global, Serve should be empty, got %d", got)
	}
}

func TestRegistryAggregatesFetchCounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()

	global := NewService(NewClient(srv.Client(), srv.URL, "g"), Config{Enabled: true}, nil)
	global.refreshAll(context.Background()) // 3 kinds succeed

	reg := newTestRegistry(t, srv.URL, global)
	defer reg.StopAll()
	reg.Start(7, "k", BoundingBox{}, false)
	waitFor(t, func() bool { return reg.FetchSuccessCount() >= 6 })

	if reg.FetchSuccessCount() < 6 {
		t.Fatalf("expected aggregated success >= 6 (global 3 + tenant 3), got %d", reg.FetchSuccessCount())
	}
}

func TestRegistryHandlerResolvesTenant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()

	reg := newTestRegistry(t, srv.URL, nil)
	defer reg.StopAll()
	reg.Start(7, "k", BoundingBox{}, false)
	waitFor(t, func() bool { return len(reg.Serve(7, KindNavaid).Features) == 2 })

	mux := http.NewServeMux()
	// The middleware would normally set the Identity; here it is a passthrough and
	// the resolver returns a fixed tenant id, isolating the Register/Serve wiring.
	passthrough := func(next http.Handler) http.Handler { return next }
	reg.Register(mux, passthrough, func(r *http.Request) (int64, bool) { return 7, true }, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/navaids", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := len(decodeFC(t, rec).Features); got != 2 {
		t.Fatalf("tenant-aware handler navaids = %d, want 2", got)
	}
}

// A tenant whose feature entitlement for a kind is off receives an empty
// collection for that kind — the overlay is gated on the SERVER, not just the
// (cosmetic) frontend toggle. The allow path is covered by the tests above (nil
// gate); here we assert the deny path drops cached data.
func TestRegistryHandlerFeatureGateDeniesServesEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()

	reg := newTestRegistry(t, srv.URL, nil)
	defer reg.StopAll()
	reg.Start(7, "k", BoundingBox{}, false)
	waitFor(t, func() bool { return len(reg.Serve(7, KindNavaid).Features) == 2 })

	// Gate denies navaids for tenant 7 (feature off), allows every other kind.
	gate := func(_ context.Context, tid int64, kind Kind) bool {
		return tid != 7 || kind != KindNavaid
	}
	mux := http.NewServeMux()
	passthrough := func(next http.Handler) http.Handler { return next }
	reg.Register(mux, passthrough, func(r *http.Request) (int64, bool) { return 7, true }, gate)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/navaids", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (graceful)", rec.Code)
	}
	if got := len(decodeFC(t, rec).Features); got != 0 {
		t.Fatalf("gated-off kind must serve empty despite cached data, got %d features", got)
	}
}

func TestRegistryHandlerNoIdentityServesEmpty(t *testing.T) {
	reg := newTestRegistry(t, "http://unused", nil)
	defer reg.StopAll()

	mux := http.NewServeMux()
	passthrough := func(next http.Handler) http.Handler { return next }
	reg.Register(mux, passthrough, func(r *http.Request) (int64, bool) { return 0, false }, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/airspace", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (graceful)", rec.Code)
	}
	if got := len(decodeFC(t, rec).Features); got != 0 {
		t.Fatalf("no-identity request should serve empty, got %d", got)
	}
}

// waitFor polls cond up to a short deadline; fails the test if it never holds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}
