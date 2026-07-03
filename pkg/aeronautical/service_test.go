package aeronautical

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func decodeFC(t *testing.T, rec *httptest.ResponseRecorder) FeatureCollection {
	t.Helper()
	var fc FeatureCollection
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatalf("decode FeatureCollection: %v", err)
	}
	return fc
}

func TestServiceServesEmptyBeforeFirstRefresh(t *testing.T) {
	s := NewService(NewClient(nil, "http://unused", ""), Config{}, nil)
	mux := http.NewServeMux()
	s.Register(mux)

	for _, path := range []string{"/api/airspace", "/api/navaids", "/api/waypoints"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, rec.Code)
		}
		fc := decodeFC(t, rec)
		if fc.Type != "FeatureCollection" || len(fc.Features) != 0 {
			t.Errorf("%s: expected empty collection, got %+v", path, fc)
		}
	}
}

func TestServiceRefreshAllPopulatesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()

	s := NewService(NewClient(srv.Client(), srv.URL, "k"),
		Config{Enabled: true, BBox: BoundingBox{}}, nil)
	s.refreshAll(context.Background())

	mux := http.NewServeMux()
	s.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/navaids", nil))
	fc := decodeFC(t, rec)
	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 cached navaids, got %d", len(fc.Features))
	}
	if s.FetchSuccessCount() == 0 {
		t.Error("expected fetch success count > 0")
	}
}

func TestServiceKeepsLastGoodOnFailure(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(sampleNavaids))
	}))
	defer srv.Close()

	s := NewService(NewClient(srv.Client(), srv.URL, "k"),
		Config{Enabled: true}, nil)

	// First refresh succeeds and populates the cache.
	s.refreshAll(context.Background())
	// Second refresh fails for every kind; the last-good cache must survive.
	fail.Store(true)
	s.refreshAll(context.Background())

	mux := http.NewServeMux()
	s.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/navaids", nil))
	fc := decodeFC(t, rec)
	if len(fc.Features) != 2 {
		t.Errorf("expected last-good cache (2 features) to survive failure, got %d", len(fc.Features))
	}
	if s.FetchFailureCount() == 0 {
		t.Error("expected fetch failure count > 0")
	}
}

func TestServiceDisabledBootstrapReturnsImmediately(t *testing.T) {
	s := NewService(NewClient(nil, "http://unused", ""), Config{Enabled: false}, nil)
	done := make(chan struct{})
	go func() {
		s.BootstrapOnce(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disabled BootstrapOnce should return immediately")
	}
	if s.FetchSuccessCount() != 0 || s.FetchFailureCount() != 0 {
		t.Error("disabled service should not fetch")
	}
}

func TestCacheAgeSeconds(t *testing.T) {
	s := NewService(NewClient(nil, "http://unused", ""), Config{}, nil)
	if got := s.CacheAgeSeconds(time.Now()); got != -1 {
		t.Errorf("expected -1 before any success, got %d", got)
	}
	s.lastSuccessUnix.Store(time.Now().Add(-30 * time.Second).Unix())
	if got := s.CacheAgeSeconds(time.Now()); got < 29 || got > 31 {
		t.Errorf("expected ~30s cache age, got %d", got)
	}
}
