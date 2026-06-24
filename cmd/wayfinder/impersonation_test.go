package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/impersonation"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

var endpointKey = []byte("impersonation-endpoint-test-key-32by")

func superRequest(method, body string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "/api/admin/impersonation", strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, "/api/admin/impersonation", nil)
	}
	return r.WithContext(tenant.WithIdentity(r.Context(),
		tenant.Identity{TenantID: 1, UserID: 1, Subject: "root", Role: store.RoleSuperAdmin}))
}

func grantCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == impersonation.CookieName {
			return c
		}
	}
	return nil
}

func TestStartImpersonationSetsGrantCookie(t *testing.T) {
	checker := fakeTenantChecker{existing: map[int64]bool{5: true}}
	cfg := impersonationCookieConfig{key: endpointKey, ttl: 30 * time.Minute}
	h := startImpersonationHandler(checker, cfg, discardLogger())

	rec := httptest.NewRecorder()
	h(rec, superRequest(http.MethodPost, `{"tenant_id":5}`))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	c := grantCookie(rec)
	if c == nil {
		t.Fatal("no impersonation cookie set")
	}
	if !c.HttpOnly {
		t.Error("impersonation cookie must be HttpOnly")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Error("impersonation cookie must be SameSite=Strict")
	}
	// The minted grant must resolve to tenant 5 for a super_admin.
	d, err := impersonation.Resolve(context.Background(), c.Value,
		tenant.Identity{Role: store.RoleSuperAdmin}, endpointKey, checker)
	if err != nil || !d.Active || d.TargetTenantID != 5 {
		t.Fatalf("minted grant did not resolve to tenant 5: active=%v target=%d err=%v", d.Active, d.TargetTenantID, err)
	}
}

func TestStartImpersonationUnknownTenant(t *testing.T) {
	checker := fakeTenantChecker{existing: map[int64]bool{5: true}} // 99 absent
	cfg := impersonationCookieConfig{key: endpointKey, ttl: time.Minute}
	h := startImpersonationHandler(checker, cfg, discardLogger())

	rec := httptest.NewRecorder()
	h(rec, superRequest(http.MethodPost, `{"tenant_id":99}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown tenant → status %d, want 404", rec.Code)
	}
	if grantCookie(rec) != nil {
		t.Error("no cookie must be set for an unknown target tenant")
	}
}

func TestStartImpersonationBadBody(t *testing.T) {
	checker := fakeTenantChecker{existing: map[int64]bool{5: true}}
	cfg := impersonationCookieConfig{key: endpointKey, ttl: time.Minute}
	h := startImpersonationHandler(checker, cfg, discardLogger())

	for _, body := range []string{`{"tenant_id":0}`, `{"tenant_id":-3}`, `not json`, `{}`} {
		rec := httptest.NewRecorder()
		h(rec, superRequest(http.MethodPost, body))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q → status %d, want 400", body, rec.Code)
		}
		if grantCookie(rec) != nil {
			t.Errorf("body %q must not set a cookie", body)
		}
	}
}

func TestStartImpersonationTenantLookupErrorFailsClosed(t *testing.T) {
	checker := fakeTenantChecker{err: context.DeadlineExceeded} // DB down
	cfg := impersonationCookieConfig{key: endpointKey, ttl: time.Minute}
	h := startImpersonationHandler(checker, cfg, discardLogger())

	rec := httptest.NewRecorder()
	h(rec, superRequest(http.MethodPost, `{"tenant_id":5}`))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("tenant lookup error → status %d, want 500", rec.Code)
	}
	if grantCookie(rec) != nil {
		t.Error("no cookie must be set when the tenant lookup fails")
	}
}

func TestStopImpersonationClearsCookie(t *testing.T) {
	h := stopImpersonationHandler(impersonationCookieConfig{}, discardLogger())

	rec := httptest.NewRecorder()
	h(rec, superRequest(http.MethodDelete, ""))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	c := grantCookie(rec)
	if c == nil || c.MaxAge >= 0 {
		t.Errorf("stop must clear the impersonation cookie (MaxAge<0), got %+v", c)
	}
}
