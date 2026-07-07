package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/impersonation"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
	"github.com/manuelringwald/wayfinder/pkg/ws"
)

type fakeFeedLister struct {
	feeds []int64
	err   error
}

func (f fakeFeedLister) ListFeedIDsByTenant(_ context.Context, _ int64) ([]int64, error) {
	return f.feeds, f.err
}

type fakeViewGetter struct {
	vc  store.ViewConfig
	err error
}

func (f fakeViewGetter) GetEffective(_ context.Context, _, _ int64) (store.ViewConfig, error) {
	return f.vc, f.err
}

// noView is a view getter that reports no configured view (the common case in
// the feed-scope tests, which assert on the feed allow-set only).
var noView = fakeViewGetter{err: store.ErrNotFound}

func withIdentity(tenantID int64) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	return r.WithContext(tenant.WithIdentity(r.Context(), tenant.Identity{TenantID: tenantID, Role: store.RoleUser}))
}

func TestNewScopeResolver(t *testing.T) {
	resolve := newScopeResolver(fakeFeedLister{feeds: []int64{1, 2}}, noView, nil, nil, discardLogger())

	scope, err := resolve(withIdentity(7))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !scope.AllowsFeed(1) || !scope.AllowsFeed(2) {
		t.Error("scope should allow the tenant's feeds 1 and 2")
	}
	if scope.AllowsFeed(3) {
		t.Error("scope must not allow a feed the tenant is not subscribed to")
	}
}

func TestNewScopeResolverFailsClosed(t *testing.T) {
	// No identity in context → fail-closed error (no stream is opened).
	resolve := newScopeResolver(fakeFeedLister{feeds: []int64{1}}, noView, nil, nil, discardLogger())
	if _, err := resolve(httptest.NewRequest(http.MethodGet, "/ws", nil)); err == nil {
		t.Error("expected error without a tenant identity")
	}

	// A subscription lookup error must not yield a scope.
	subErr := newScopeResolver(fakeFeedLister{err: errors.New("db down")}, noView, nil, nil, discardLogger())
	if _, err := subErr(withIdentity(7)); err == nil {
		t.Error("expected error when subscription lookup fails")
	}

	// A view lookup error (other than ErrNotFound) must not yield a scope.
	viewErr := newScopeResolver(fakeFeedLister{feeds: []int64{1}}, fakeViewGetter{err: errors.New("db down")}, nil, nil, discardLogger())
	if _, err := viewErr(withIdentity(7)); err == nil {
		t.Error("expected error when view lookup fails")
	}
}

func TestScopeResolverEmitsAudit(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	flMin, flMax := 100, 300
	views := fakeViewGetter{vc: store.ViewConfig{
		AOI:   &store.BBox{MinLat: 49, MinLon: 8, MaxLat: 51, MaxLon: 10},
		FLMin: &flMin, FLMax: &flMax,
	}}
	resolve := newScopeResolver(fakeFeedLister{feeds: []int64{1, 2}}, views, nil, nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req = req.WithContext(tenant.WithIdentity(req.Context(),
		tenant.Identity{TenantID: 7, UserID: 3, Subject: "alice", Role: store.RoleUser}))
	if _, err := resolve(req); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	var rec map[string]any
	if err := json.NewDecoder(&buf).Decode(&rec); err != nil {
		t.Fatalf("audit record is not valid JSON: %v", err)
	}
	if rec["component"] != "audit" || rec["event"] != "ws_connect" {
		t.Errorf("audit envelope = component:%v event:%v", rec["component"], rec["event"])
	}
	if rec["tenant_id"] != float64(7) || rec["user_id"] != float64(3) || rec["subject"] != "alice" {
		t.Errorf("audit identity = %+v", rec)
	}
	if feeds, ok := rec["feeds"].([]any); !ok || len(feeds) != 2 {
		t.Errorf("audit feeds = %v, want 2 entries", rec["feeds"])
	}
	if _, ok := rec["aoi"].(map[string]any); !ok {
		t.Errorf("audit aoi missing/!object: %v", rec["aoi"])
	}
	if rec["fl_min_ft"] != float64(10000) || rec["fl_max_ft"] != float64(30000) {
		t.Errorf("audit FL (feet) = min:%v max:%v", rec["fl_min_ft"], rec["fl_max_ft"])
	}
}

func TestResolveViewFilter(t *testing.T) {
	ctx := context.Background()
	aoi := &store.BBox{MinLat: 49, MinLon: 8, MaxLat: 51, MaxLon: 10}
	flMin, flMax := 100, 300

	// Full config → ViewFilter with FL converted from FL to feet (×100).
	vf, err := resolveViewFilter(ctx, fakeViewGetter{vc: store.ViewConfig{AOI: aoi, FLMin: &flMin, FLMax: &flMax}}, 1, 2)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if vf == nil || vf.AOI == nil || vf.AOI.MaxLat != 51 {
		t.Fatalf("AOI not mapped: %+v", vf)
	}
	if vf.FLMinFt == nil || *vf.FLMinFt != 10000 || vf.FLMaxFt == nil || *vf.FLMaxFt != 30000 {
		t.Fatalf("FL band not converted to feet: %+v", vf)
	}

	// No config → nil (no restriction within allowed feeds).
	if got, err := resolveViewFilter(ctx, noView, 1, 2); err != nil || got != nil {
		t.Fatalf("no config should yield (nil,nil); got (%+v,%v)", got, err)
	}

	// Config with neither AOI nor FL → nil (fast path).
	if got, err := resolveViewFilter(ctx, fakeViewGetter{vc: store.ViewConfig{CenterLat: 50}}, 1, 2); err != nil || got != nil {
		t.Fatalf("empty restriction should yield (nil,nil); got (%+v,%v)", got, err)
	}

	// Lookup error (not ErrNotFound) → propagated (fail-closed upstream).
	if _, err := resolveViewFilter(ctx, fakeViewGetter{err: errors.New("db down")}, 1, 2); err == nil {
		t.Error("expected lookup error to propagate")
	}
}

// --- Cross-tenant read-only impersonation (ADR 0008, WF2-34) ----------------

// feedsByTenant is a tenant-aware feed lister: it returns different feeds per
// tenant id, so the impersonation tests can prove the scope was resolved against
// the TARGET tenant and not the caller's own.
type feedsByTenant map[int64][]int64

func (m feedsByTenant) ListFeedIDsByTenant(_ context.Context, tenantID int64) ([]int64, error) {
	return m[tenantID], nil
}

// fakeTenantChecker is a DB-free impersonation.TenantChecker.
type fakeTenantChecker struct {
	existing map[int64]bool
	err      error
}

func (f fakeTenantChecker) Exists(_ context.Context, tenantID int64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.existing[tenantID], nil
}

// requestAs builds a /ws request whose context carries an Identity (the caller's
// real tenant/role) and, when grant != "", an impersonation grant cookie.
func requestAs(tenantID int64, role store.Role, grant string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	if grant != "" {
		r.AddCookie(&http.Cookie{Name: impersonation.CookieName, Value: grant})
	}
	return r.WithContext(tenant.WithIdentity(r.Context(),
		tenant.Identity{TenantID: tenantID, UserID: 1, Subject: "actor", Role: role}))
}

var impKey = []byte("scope-resolver-impersonation-key-32b!")

// impersonationResolver wires a resolver with impersonation enabled: tenants 7
// (feed 1) and 9 (feeds 2,3) both exist.
func impersonationResolver() ws.ScopeResolver {
	feeds := feedsByTenant{7: {1}, 9: {2, 3}}
	checker := fakeTenantChecker{existing: map[int64]bool{7: true, 9: true}}
	return newScopeResolver(feeds, noView, checker, impKey, discardLogger())
}

func TestScopeResolverImpersonationActive(t *testing.T) {
	resolve := impersonationResolver()
	grant := impersonation.MintGrant(9, time.Hour, impKey)

	scope, err := resolve(requestAs(7, store.RoleAdmin, grant))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !scope.AllowsFeed(2) || !scope.AllowsFeed(3) {
		t.Error("impersonator must see the TARGET tenant's feeds (2,3)")
	}
	if scope.AllowsFeed(1) {
		t.Error("impersonator must NOT see their OWN tenant's feed (1)")
	}
	// Detached from the target tenant → excluded from per-tenant metrics + live
	// re-scope (ADR 0008 §6/§4 snapshot-v1).
	if scope.TenantID != 0 {
		t.Errorf("impersonation scope must have TenantID=0, got %d", scope.TenantID)
	}
}

func TestScopeResolverImpersonationDeniedForNonAdmin(t *testing.T) {
	resolve := impersonationResolver()
	grant := impersonation.MintGrant(9, time.Hour, impKey)

	// A cryptographically valid grant presented by a non-admin must be a loud
	// failure (handshake reject), never silently honoured or ignored.
	if _, err := resolve(requestAs(7, store.RoleUser, grant)); err == nil {
		t.Errorf("role=user: a valid grant from a non-admin must be rejected")
	}
}

func TestScopeResolverImpersonationUnknownTenantRejected(t *testing.T) {
	resolve := impersonationResolver()
	grant := impersonation.MintGrant(404, time.Hour, impKey) // tenant 404 does not exist

	if _, err := resolve(requestAs(7, store.RoleAdmin, grant)); err == nil {
		t.Error("a grant naming a non-existent tenant must be rejected")
	}
}

func TestScopeResolverImpersonationExpiredFallsBack(t *testing.T) {
	resolve := impersonationResolver()
	expired := impersonation.MintGrant(9, -time.Minute, impKey)

	// An expired grant carries no authority → the default path, byte-identical
	// to no impersonation, with no error. For a USER that is their own tenant's
	// scope; a stale cookie left over from an earlier admin session must never
	// grant anything.
	scope, err := resolve(requestAs(7, store.RoleUser, expired))
	if err != nil {
		t.Fatalf("expired grant must fall back to the default path, got %v", err)
	}
	if !scope.AllowsFeed(1) || scope.AllowsFeed(2) {
		t.Error("expired grant → caller sees their OWN tenant (7 → feed 1)")
	}
	if scope.TenantID != 7 {
		t.Errorf("default path must keep the real tenant id, got %d", scope.TenantID)
	}

	// For an ADMIN the default path no longer exists (#208, ADR 0022): an
	// expired grant means no active impersonation, so the handshake is rejected
	// instead of serving the earlier "empty own picture".
	if _, err := resolve(requestAs(0, store.RoleAdmin, expired)); err == nil {
		t.Error("admin with an expired grant must be rejected, not fall back")
	}
}

// #208 (ADR 0022): a platform admin has no own ASD scope. Without an ACTIVE
// impersonation grant the /ws handshake is rejected fail-closed — the guest
// mode (eye icon in the tenant overview) is the only way an admin reads the
// air picture.
func TestScopeResolverAdminWithoutGrantRejected(t *testing.T) {
	resolve := impersonationResolver()

	// No grant cookie at all → rejected.
	if _, err := resolve(requestAs(0, store.RoleAdmin, "")); err == nil {
		t.Error("admin without an impersonation grant must be rejected")
	}

	// With an ACTIVE grant the admin reads the TARGET tenant (unchanged).
	grant := impersonation.MintGrant(9, time.Hour, impKey)
	scope, err := resolve(requestAs(0, store.RoleAdmin, grant))
	if err != nil {
		t.Fatalf("admin with active grant: %v", err)
	}
	if !scope.AllowsFeed(2) || !scope.AllowsFeed(3) {
		t.Error("admin with active grant must read the target tenant's feeds")
	}
}

func TestScopeResolverImpersonationDisabledWithoutKey(t *testing.T) {
	// No checker/key → impersonation disabled platform-wide: a valid-looking
	// grant is ignored. A USER keeps their own tenant scope; an ADMIN is
	// rejected (#208, ADR 0022) — with impersonation off there is no legitimate
	// ASD read for an admin at all.
	feeds := feedsByTenant{7: {1}, 9: {2, 3}}
	resolve := newScopeResolver(feeds, noView, nil, nil, discardLogger())
	grant := impersonation.MintGrant(9, time.Hour, impKey)

	scope, err := resolve(requestAs(7, store.RoleUser, grant))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !scope.AllowsFeed(1) || scope.AllowsFeed(2) {
		t.Error("with impersonation disabled the grant must be ignored (own tenant scope)")
	}

	if _, err := resolve(requestAs(0, store.RoleAdmin, grant)); err == nil {
		t.Error("with impersonation disabled an admin has no ASD path and must be rejected")
	}
}
