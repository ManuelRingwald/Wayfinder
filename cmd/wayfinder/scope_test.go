package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
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
	return r.WithContext(tenant.WithIdentity(r.Context(), tenant.Identity{TenantID: tenantID, Role: store.RoleOperator}))
}

func TestNewScopeResolver(t *testing.T) {
	resolve := newScopeResolver(fakeFeedLister{feeds: []int64{1, 2}}, noView)

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
	resolve := newScopeResolver(fakeFeedLister{feeds: []int64{1}}, noView)
	if _, err := resolve(httptest.NewRequest(http.MethodGet, "/ws", nil)); err == nil {
		t.Error("expected error without a tenant identity")
	}

	// A subscription lookup error must not yield a scope.
	subErr := newScopeResolver(fakeFeedLister{err: errors.New("db down")}, noView)
	if _, err := subErr(withIdentity(7)); err == nil {
		t.Error("expected error when subscription lookup fails")
	}

	// A view lookup error (other than ErrNotFound) must not yield a scope.
	viewErr := newScopeResolver(fakeFeedLister{feeds: []int64{1}}, fakeViewGetter{err: errors.New("db down")})
	if _, err := viewErr(withIdentity(7)); err == nil {
		t.Error("expected error when view lookup fails")
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
