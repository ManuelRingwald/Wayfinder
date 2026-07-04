package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/impersonation"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// readMWRequest builds a GET request carrying an authenticated identity and,
// when grant is non-empty, the wf_impersonation cookie — mirroring what the
// browser sends to the map's read endpoints.
func readMWRequest(role store.Role, grant string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/whoami", nil)
	r = r.WithContext(tenant.WithIdentity(r.Context(),
		tenant.Identity{TenantID: 1, UserID: 1, Subject: "root", Role: role}))
	if grant != "" {
		r.AddCookie(&http.Cookie{Name: impersonation.CookieName, Value: grant})
	}
	return r
}

// serveReadMW runs one request through impersonationReadMW and captures the
// read tenant the wrapped handler observed (fallback semantics included).
func serveReadMW(t *testing.T, checker impersonation.TenantChecker, r *http.Request) (*httptest.ResponseRecorder, int64, bool) {
	t.Helper()
	var seenTenant int64
	var handlerRan bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerRan = true
		id, _ := tenant.FromContext(r.Context())
		seenTenant = tenant.ReadTenant(r.Context(), id.TenantID)
		w.WriteHeader(http.StatusOK)
	})
	mw := impersonationReadMW(checker, endpointKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, r)
	return rec, seenTenant, handlerRan
}

// Without a grant cookie the read path must stay byte-identical: the handler
// runs against the caller's own tenant.
func TestImpersonationReadMWNoCookiePassesThrough(t *testing.T) {
	checker := fakeTenantChecker{existing: map[int64]bool{5: true}}
	rec, seen, ran := serveReadMW(t, checker, readMWRequest(store.RoleAdmin, ""))
	if rec.Code != http.StatusOK || !ran {
		t.Fatalf("status = %d, ran = %v; want 200 + handler run", rec.Code, ran)
	}
	if seen != 1 {
		t.Fatalf("read tenant = %d, want caller's own 1", seen)
	}
}

// A valid admin grant stamps the target tenant onto the context — the wrapped
// read handler serves the target's data (ADR 0008 Nachtrag).
func TestImpersonationReadMWValidGrantStampsTarget(t *testing.T) {
	checker := fakeTenantChecker{existing: map[int64]bool{5: true}}
	grant := impersonation.MintGrant(5, time.Minute, endpointKey)
	rec, seen, ran := serveReadMW(t, checker, readMWRequest(store.RoleAdmin, grant))
	if rec.Code != http.StatusOK || !ran {
		t.Fatalf("status = %d, ran = %v; want 200 + handler run", rec.Code, ran)
	}
	if seen != 5 {
		t.Fatalf("read tenant = %d, want impersonated 5", seen)
	}
}

// A stale/garbled cookie carries no authority and must NOT break the request:
// same fall-back-to-own semantics as the /ws path.
func TestImpersonationReadMWStaleGrantIgnored(t *testing.T) {
	checker := fakeTenantChecker{existing: map[int64]bool{5: true}}
	rec, seen, ran := serveReadMW(t, checker, readMWRequest(store.RoleAdmin, "garbled.grant"))
	if rec.Code != http.StatusOK || !ran {
		t.Fatalf("status = %d, ran = %v; want 200 + handler run", rec.Code, ran)
	}
	if seen != 1 {
		t.Fatalf("read tenant = %d, want caller's own 1 (stale grant ignored)", seen)
	}
}

// A VALID grant presented by a non-admin is a misuse signal: reject loudly,
// never run the handler (ADR 0008 §3, decision 4).
func TestImpersonationReadMWNonAdminValidGrantRejected(t *testing.T) {
	checker := fakeTenantChecker{existing: map[int64]bool{5: true}}
	grant := impersonation.MintGrant(5, time.Minute, endpointKey)
	rec, _, ran := serveReadMW(t, checker, readMWRequest(store.RoleUser, grant))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if ran {
		t.Fatal("handler must not run on a denied grant")
	}
}

// An admin grant naming a deleted tenant is rejected (cannot impersonate a
// non-existent tenant).
func TestImpersonationReadMWUnknownTenantRejected(t *testing.T) {
	checker := fakeTenantChecker{existing: map[int64]bool{5: true}} // 99 absent
	grant := impersonation.MintGrant(99, time.Minute, endpointKey)
	rec, _, ran := serveReadMW(t, checker, readMWRequest(store.RoleAdmin, grant))
	if rec.Code != http.StatusForbidden || ran {
		t.Fatalf("status = %d, ran = %v; want 403 + handler not run", rec.Code, ran)
	}
}

// A tenant-lookup failure (database down) fails closed as a server error, not
// as a silent fall-back that could mask a live grant.
func TestImpersonationReadMWCheckerErrorFailsClosed(t *testing.T) {
	checker := fakeTenantChecker{err: context.DeadlineExceeded}
	grant := impersonation.MintGrant(5, time.Minute, endpointKey)
	rec, _, ran := serveReadMW(t, checker, readMWRequest(store.RoleAdmin, grant))
	if rec.Code != http.StatusInternalServerError || ran {
		t.Fatalf("status = %d, ran = %v; want 500 + handler not run", rec.Code, ran)
	}
}

// readTenantOf is the aeronautical Registry's TenantResolver: it serves the
// impersonation target when stamped, the caller's own tenant otherwise, and
// reports no tenant without an identity.
func TestReadTenantOf(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/navaids", nil)
	if _, ok := readTenantOf(r); ok {
		t.Fatal("no identity → ok must be false")
	}

	r = r.WithContext(tenant.WithIdentity(r.Context(), tenant.Identity{TenantID: 1, UserID: 1, Role: store.RoleAdmin}))
	if tid, ok := readTenantOf(r); !ok || tid != 1 {
		t.Fatalf("own tenant = (%d, %v), want (1, true)", tid, ok)
	}

	r = r.WithContext(tenant.WithReadTenant(r.Context(), 5))
	if tid, ok := readTenantOf(r); !ok || tid != 5 {
		t.Fatalf("impersonated tenant = (%d, %v), want (5, true)", tid, ok)
	}
}
