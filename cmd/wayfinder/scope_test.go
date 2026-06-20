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

func withIdentity(tenantID int64) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	return r.WithContext(tenant.WithIdentity(r.Context(), tenant.Identity{TenantID: tenantID, Role: store.RoleOperator}))
}

func TestNewScopeResolver(t *testing.T) {
	resolve := newScopeResolver(fakeFeedLister{feeds: []int64{1, 2}})

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
	resolve := newScopeResolver(fakeFeedLister{feeds: []int64{1}})
	if _, err := resolve(httptest.NewRequest(http.MethodGet, "/ws", nil)); err == nil {
		t.Error("expected error without a tenant identity")
	}

	// A subscription lookup error must not yield a scope.
	resolveErr := newScopeResolver(fakeFeedLister{err: errors.New("db down")})
	if _, err := resolveErr(withIdentity(7)); err == nil {
		t.Error("expected error when subscription lookup fails")
	}
}
