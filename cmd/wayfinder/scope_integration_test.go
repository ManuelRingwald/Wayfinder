package main

import (
	"context"
	"os"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// TestIntegrationResolveViewFilter exercises resolveViewFilter against a real
// view_configs row: the JSONB AOI and the FL band round-trip into a broadcast
// ViewFilter (FL converted from flight levels to feet). Skips without
// WAYFINDER_TEST_DB_URL.
func TestIntegrationResolveViewFilter(t *testing.T) {
	dsn := os.Getenv("WAYFINDER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("set WAYFINDER_TEST_DB_URL to run the view-filter integration test")
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
	if _, err := pool.Exec(ctx, `TRUNCATE tenants, view_configs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	ten, err := store.NewTenantRepo(pool).Create(ctx, "demo", "Demo")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	views := store.NewViewConfigRepo(pool)
	flMin, flMax := 100, 300
	if _, err := views.UpsertTenantDefault(ctx, ten.ID, store.ViewConfig{
		CenterLat: 50, CenterLon: 9, Zoom: 8,
		AOI:   &store.BBox{MinLat: 49, MinLon: 8, MaxLat: 51, MaxLon: 10},
		FLMin: &flMin, FLMax: &flMax,
	}); err != nil {
		t.Fatalf("upsert tenant view: %v", err)
	}

	// A user with no override falls back to the tenant default; AOI + FL map through.
	vf, err := resolveViewFilter(ctx, views, ten.ID, 999)
	if err != nil {
		t.Fatalf("resolveViewFilter: %v", err)
	}
	if vf == nil || vf.AOI == nil || vf.AOI.MinLat != 49 || vf.AOI.MaxLon != 10 {
		t.Fatalf("AOI did not round-trip: %+v", vf)
	}
	if vf.FLMinFt == nil || *vf.FLMinFt != 10000 || vf.FLMaxFt == nil || *vf.FLMaxFt != 30000 {
		t.Fatalf("FL band not converted to feet: %+v", vf)
	}

	// A tenant with no view config at all → nil (no restriction).
	ten2, err := store.NewTenantRepo(pool).Create(ctx, "demo2", "Demo2")
	if err != nil {
		t.Fatalf("create tenant 2: %v", err)
	}
	if got, err := resolveViewFilter(ctx, views, ten2.ID, 999); err != nil || got != nil {
		t.Fatalf("tenant without view should yield (nil,nil); got (%+v,%v)", got, err)
	}
}
