package adminapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// TestIntegrationAdminAPI exercises the admin API against a real Postgres: PUT
// view then GET view round-trips, and the subscriptions/feeds endpoints reflect
// the catalogue. Skips without WAYFINDER_TEST_DB_URL.
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
	if err := store.NewSubscriptionRepo(pool).Subscribe(ctx, ten.ID, feed.ID); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	h := New(store.NewViewConfigRepo(pool), store.NewSubscriptionRepo(pool), store.NewFeedRepo(pool),
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := func(method, path, body string) *httptest.ResponseRecorder {
		var r *http.Request
		if body != "" {
			r = httptest.NewRequest(method, path, strings.NewReader(body))
		} else {
			r = httptest.NewRequest(method, path, nil)
		}
		r = r.WithContext(tenant.WithIdentity(r.Context(),
			tenant.Identity{TenantID: ten.ID, UserID: 1, Role: store.RoleTenantAdmin}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}

	// No view yet → 404.
	if rec := req(http.MethodGet, "/api/admin/view", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("GET view (none) = %d, want 404", rec.Code)
	}

	// PUT then GET round-trips the stored view (AOI + FL band).
	put := req(http.MethodPut, "/api/admin/view",
		`{"center_lat":50.1,"center_lon":8.7,"zoom":9,"aoi":{"min_lat":49,"min_lon":8,"max_lat":51,"max_lon":10},"fl_min":100,"fl_max":300}`)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT view = %d, want 200; body=%s", put.Code, put.Body.String())
	}
	get := req(http.MethodGet, "/api/admin/view", "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET view = %d, want 200", get.Code)
	}
	var vc map[string]any
	_ = json.Unmarshal(get.Body.Bytes(), &vc)
	if vc["center_lat"] != 50.1 || vc["fl_min"] != 100.0 {
		t.Errorf("round-tripped view = %v", vc)
	}
	if aoi, ok := vc["aoi"].(map[string]any); !ok || aoi["max_lon"] != 10.0 {
		t.Errorf("round-tripped aoi = %v", vc["aoi"])
	}

	// Subscriptions reflect the granted feed; feeds list the catalogue.
	var subs []map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/subscriptions", "").Body.Bytes(), &subs)
	if len(subs) != 1 || subs[0]["name"] != "Frankfurt" {
		t.Errorf("subscriptions = %v", subs)
	}
	var feeds []map[string]any
	_ = json.Unmarshal(req(http.MethodGet, "/api/admin/feeds", "").Body.Bytes(), &feeds)
	if len(feeds) != 1 || feeds[0]["name"] != "Frankfurt" {
		t.Errorf("feeds = %v", feeds)
	}
}
