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
// admin view round-trip, and the admin grant→list→revoke provisioning flow.
// Skips without WAYFINDER_TEST_DB_URL.
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
		store.NewTenantRepo(pool), store.NewUserRepo(pool), store.NewCredentialRepo(pool),
		feature.New(store.NewEntitlementRepo(pool), slog.New(slog.NewTextHandler(io.Discard, nil))),
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

	// --- admin view round-trip ---
	if rec := req(http.MethodGet, "/api/admin/view", "", store.RoleAdmin); rec.Code != http.StatusNotFound {
		t.Fatalf("GET view (none) = %d, want 404", rec.Code)
	}
	put := req(http.MethodPut, "/api/admin/view",
		`{"center_lat":50.1,"center_lon":8.7,"zoom":9,"aoi":{"min_lat":49,"min_lon":8,"max_lat":51,"max_lon":10},"fl_min":100,"fl_max":300}`,
		store.RoleAdmin)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT view = %d, want 200; body=%s", put.Code, put.Body.String())
	}
	var vc map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/view", "", store.RoleAdmin).Body.Bytes(), &vc)
	if vc["center_lat"] != 50.1 || vc["fl_min"] != 100.0 {
		t.Errorf("round-tripped view = %v", vc)
	}
	if aoi, ok := vc["aoi"].(map[string]any); !ok || aoi["max_lon"] != 10.0 {
		t.Errorf("round-tripped aoi = %v", vc["aoi"])
	}

	// --- admin provisioning: grant → list → revoke ---
	grantPath := fmt.Sprintf("/api/admin/tenants/%d/subscriptions", ten.ID)

	// A non-admin (user) must NOT be able to grant (cross-tenant) → 403.
	if rec := req(http.MethodPost, grantPath, fmt.Sprintf(`{"feed_id":%d}`, feed.ID), store.RoleUser); rec.Code != http.StatusForbidden {
		t.Fatalf("user grant = %d, want 403", rec.Code)
	}

	if rec := req(http.MethodPost, grantPath, fmt.Sprintf(`{"feed_id":%d}`, feed.ID), store.RoleAdmin); rec.Code != http.StatusNoContent {
		t.Fatalf("admin grant = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	// Now the tenant sees the feed in its own subscriptions.
	var subs []map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/subscriptions", "", store.RoleAdmin).Body.Bytes(), &subs)
	if len(subs) != 1 || subs[0]["name"] != "Frankfurt" {
		t.Errorf("after grant, subscriptions = %v", subs)
	}

	// Revoke → the subscription is gone.
	if rec := req(http.MethodDelete, fmt.Sprintf("%s/%d", grantPath, feed.ID), "", store.RoleAdmin); rec.Code != http.StatusNoContent {
		t.Fatalf("admin revoke = %d, want 204", rec.Code)
	}
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/subscriptions", "", store.RoleAdmin).Body.Bytes(), &subs)
	if len(subs) != 0 {
		t.Errorf("after revoke, subscriptions = %v, want empty", subs)
	}

	// admin tenant list shows the tenant.
	var tenants []map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/tenants", "", store.RoleAdmin).Body.Bytes(), &tenants)
	if len(tenants) != 1 || tenants[0]["slug"] != "acme" {
		t.Errorf("tenants = %v", tenants)
	}

	// --- admin feature entitlements: list → set → re-list, + whoami (WF2-50) ---
	entPath := fmt.Sprintf("/api/admin/tenants/%d/entitlements", ten.ID)

	// A non-admin (user) must NOT read another tenant's entitlement provisioning view.
	if rec := req(http.MethodGet, entPath, "", store.RoleUser); rec.Code != http.StatusForbidden {
		t.Fatalf("user entitlements GET = %d, want 403", rec.Code)
	}

	// Initially the full catalogue is returned, all default-denied.
	var ents []map[string]any
	_ = json.Unmarshal(req(http.MethodGet, entPath, "", store.RoleAdmin).Body.Bytes(), &ents)
	if len(ents) != len(feature.All()) {
		t.Fatalf("entitlements = %v, want full catalogue of %d", ents, len(feature.All()))
	}
	for _, e := range ents {
		if e["enabled"] != false {
			t.Errorf("entitlement %v enabled before set, want false", e)
		}
	}

	// An unknown feature key is rejected end-to-end by the service catalogue guard.
	if rec := req(http.MethodPut, entPath+"/bogus", `{"enabled":true}`, store.RoleAdmin); rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT unknown entitlement = %d, want 400", rec.Code)
	}

	// Enable stca → it round-trips through the real EntitlementRepo as enabled.
	if rec := req(http.MethodPut, entPath+"/"+string(feature.STCA), `{"enabled":true}`, store.RoleAdmin); rec.Code != http.StatusNoContent {
		t.Fatalf("PUT entitlement = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(req(http.MethodGet, entPath, "", store.RoleAdmin).Body.Bytes(), &ents)
	enabled := map[string]bool{}
	for _, e := range ents {
		enabled[e["key"].(string)] = e["enabled"].(bool)
	}
	if !enabled[string(feature.STCA)] {
		t.Errorf("after set, %s not enabled: %v", feature.STCA, ents)
	}

	// whoami carries the same effective flag for the tenant's SPA gating.
	var who map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/whoami", "", store.RoleAdmin).Body.Bytes(), &who)
	feats, _ := who["features"].(map[string]any)
	if feats == nil || feats[string(feature.STCA)] != true {
		t.Errorf("whoami features = %v, want stca=true", who["features"])
	}

	// --- WF2-41: multi_feed grant gating (real-PG, end-to-end) ---
	// The tenant currently holds no feeds (granted+revoked above). Create a
	// second feed so a 2-feed subscription can be attempted.
	feed2, err := store.NewFeedRepo(pool).Create(ctx, "Munich", "239.255.0.63", 8601, nil, []string{"ADS-B"})
	if err != nil {
		t.Fatalf("create feed2: %v", err)
	}
	grantFeed := func(feedID int64) int {
		return req(http.MethodPost, grantPath, fmt.Sprintf(`{"feed_id":%d}`, feedID), store.RoleAdmin).Code
	}
	if code := grantFeed(feed.ID); code != http.StatusNoContent {
		t.Fatalf("grant 1st feed = %d, want 204", code)
	}
	if code := grantFeed(feed2.ID); code != http.StatusConflict {
		t.Fatalf("grant 2nd feed without multi_feed = %d, want 409", code)
	}
	entPut := fmt.Sprintf("/api/admin/tenants/%d/entitlements/%s", ten.ID, feature.MultiFeed)
	if code := req(http.MethodPut, entPut, `{"enabled":true}`, store.RoleAdmin).Code; code != http.StatusNoContent {
		t.Fatalf("enable multi_feed = %d, want 204", code)
	}
	if code := grantFeed(feed2.ID); code != http.StatusNoContent {
		t.Fatalf("grant 2nd feed with multi_feed = %d, want 204", code)
	}
	var subs2 []map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/subscriptions", "", store.RoleAdmin).Body.Bytes(), &subs2)
	if len(subs2) != 2 {
		t.Errorf("after entitled grant, subscriptions = %d, want 2", len(subs2))
	}
}
