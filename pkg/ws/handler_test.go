package ws

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/broadcast"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCheckOriginAllowsRequestsWithoutOriginHeader(t *testing.T) {
	h := New(broadcast.New(testLogger()), testLogger(), nil)

	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Host = "wayfinder.example:8081"

	if !h.checkOrigin(r) {
		t.Error("expected request without Origin header to be allowed")
	}
}

func TestCheckOriginAllowsSameOrigin(t *testing.T) {
	h := New(broadcast.New(testLogger()), testLogger(), nil)

	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Host = "wayfinder.example:8081"
	r.Header.Set("Origin", "http://wayfinder.example:8081")

	if !h.checkOrigin(r) {
		t.Error("expected same-origin request to be allowed")
	}
}

func TestCheckOriginRejectsCrossOriginByDefault(t *testing.T) {
	h := New(broadcast.New(testLogger()), testLogger(), nil)

	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Host = "wayfinder.example:8081"
	r.Header.Set("Origin", "https://evil.example")

	if h.checkOrigin(r) {
		t.Error("expected cross-origin request to be rejected when no allowlist is configured")
	}
}

func TestCheckOriginAllowsAllowlistedOrigin(t *testing.T) {
	h := New(broadcast.New(testLogger()), testLogger(), []string{"https://allowed.example"})

	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Host = "wayfinder.example:8081"
	r.Header.Set("Origin", "https://allowed.example")

	if !h.checkOrigin(r) {
		t.Error("expected allowlisted cross-origin request to be allowed")
	}
}

func TestCheckOriginRejectsOriginNotInAllowlist(t *testing.T) {
	h := New(broadcast.New(testLogger()), testLogger(), []string{"https://allowed.example"})

	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Host = "wayfinder.example:8081"
	r.Header.Set("Origin", "https://other.example")

	if h.checkOrigin(r) {
		t.Error("expected non-allowlisted cross-origin request to be rejected")
	}
}

func TestCheckOriginRejectsInvalidOriginHeader(t *testing.T) {
	h := New(broadcast.New(testLogger()), testLogger(), nil)

	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Host = "wayfinder.example:8081"
	r.Header.Set("Origin", "://not a url")

	if h.checkOrigin(r) {
		t.Error("expected request with an unparseable Origin header to be rejected")
	}
}
