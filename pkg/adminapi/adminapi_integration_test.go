package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/feature"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// TestIntegrationAdminAPI exercises the admin API against a real Postgres: the
// tenant_admin view round-trip, and the super_admin grant→list→revoke
// provisioning flow. Skips without WAYFINDER_TEST_DB_URL.
func TestIntegrationAdminAPI(t *testing.T) {
	dsn := os.Getenv("WAYFINDER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("set WAYFINDER_TEST_DB_URL to run the admin API integration test")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE tenants, feeds, subscriptions, view_configs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	ten, err := store.NewTenantRepo(pool).Create(ctx, "acme", "ACME")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	feed, err := store.NewFeedRepo(pool).Create(ctx, "Frankfurt", "239.255.0.62", 8600, nil, []string{"PSR", "SSR"})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	h := New(store.NewViewConfigRepo(pool), store.NewSubscriptionRepo(pool), store.NewFeedRepo(pool),
		store.NewTenantRepo(pool), feature.New(store.NewEntitlementRepo(pool), slog.New(slog.NewTextHandler(io.Discard, nil))),
		slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	req := func(method, path, body string, role store.Role) *httptest.ResponseRecorder {
		var r *http.Request
		if body != "" {
			r = httptest.NewRequest(method, path, strings.NewReader(body))
		} else {
			r = httptest.NewRequest(method, path, nil)
		}
		r = r.WithContext(tenant.WithIdentity(r.Context(),
			tenant.Identity{TenantID: ten.ID, UserID: 1, Role: role}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}

	// --- tenant_admin view round-trip ---
	if rec := req(http.MethodGet, "/api/admin/view", "", store.RoleTenantAdmin); rec.Code != http.StatusNotFound {
		t.Fatalf("GET view (none) = %d, want 404", rec.Code)
	}
	put := req(http.MethodPut, "/api/admin/view",
		`{"center_lat":50.1,"center_lon":8.7,"zoom":9,"aoi":{"min_lat":49,"min_lon":8,"max_lat":51,"max_lon":10},"fl_min":100,"fl_max":300}`,
		store.RoleTenantAdmin)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT view = %d, want 200; body=%s", put.Code, put.Body.String())
	}
	var vc map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/view", "", store.RoleTenantAdmin).Body.Bytes(), &vc)
	if vc["center_lat"] != 50.1 || vc["fl_min"] != 100.0 {
		t.Errorf("round-tripped view = %v", vc)
	}
	if aoi, ok := vc["aoi"].(map[string]any); !ok || aoi["max_lon"] != 10.0 {
		t.Errorf("round-tripped aoi = %v", vc["aoi"])
	}

	// --- super_admin provisioning: grant → list → revoke ---
	grantPath := fmt.Sprintf("/api/admin/tenants/%d/subscriptions", ten.ID)

	// A tenant_admin must NOT be able to grant (cross-tenant) → 403.
	if rec := req(http.MethodPost, grantPath, fmt.Sprintf(`{"feed_id":%d}`, feed.ID), store.RoleTenantAdmin); rec.Code != http.StatusForbidden {
		t.Fatalf("tenant_admin grant = %d, want 403", rec.Code)
	}

	if rec := req(http.MethodPost, grantPath, fmt.Sprintf(`{"feed_id":%d}`, feed.ID), store.RoleSuperAdmin); rec.Code != http.StatusNoContent {
		t.Fatalf("super_admin grant = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	// Now the tenant sees the feed in its own subscriptions.
	var subs []map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/subscriptions", "", store.RoleTenantAdmin).Body.Bytes(), &subs)
	if len(subs) != 1 || subs[0]["name"] != "Frankfurt" {
		t.Errorf("after grant, subscriptions = %v", subs)
	}

	// Revoke → the subscription is gone.
	if rec := req(http.MethodDelete, fmt.Sprintf("%s/%d", grantPath, feed.ID), "", store.RoleSuperAdmin); rec.Code != http.StatusNoContent {
		t.Fatalf("super_admin revoke = %d, want 204", rec.Code)
	}
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/subscriptions", "", store.RoleTenantAdmin).Body.Bytes(), &subs)
	if len(subs) != 0 {
		t.Errorf("after revoke, subscriptions = %v, want empty", subs)
	}

	// super_admin tenant list shows the tenant.
	var tenants []map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/tenants", "", store.RoleSuperAdmin).Body.Bytes(), &tenants)
	if len(tenants) != 1 || tenants[0]["slug"] != "acme" {
		t.Errorf("tenants = %v", tenants)
	}
}
