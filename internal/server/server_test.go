package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// /healthz answers 200 as long as the process is up. REQ: FR-OPS-001
func TestHealthzIsOK(t *testing.T) {
	mux := NewMux(AlwaysReady)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

// /readyz reflects the readiness check: 200 when ready, 503 when not.
// REQ: FR-OPS-002
func TestReadyzReflectsCheck(t *testing.T) {
	readiness := NewAtomicReadiness(false)
	mux := NewMux(readiness.Check)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d while not ready, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	readiness.Set(true)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d once ready, got %d", http.StatusOK, rec.Code)
	}
}
