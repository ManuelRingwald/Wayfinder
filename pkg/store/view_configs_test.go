package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestViewJSONParams(t *testing.T) {
	// No AOI -> nil (SQL NULL); empty layers -> "{}".
	aoi, layers, err := viewJSONParams(ViewConfig{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if aoi != nil {
		t.Errorf("aoi = %v, want nil", aoi)
	}
	if layers != "{}" {
		t.Errorf("layers = %q, want {}", layers)
	}

	box := &BBox{MinLat: 49, MinLon: 8, MaxLat: 51, MaxLon: 9}
	aoi, layers, err = viewJSONParams(ViewConfig{AOI: box, Layers: map[string]bool{"airspace": true}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	s, ok := aoi.(string)
	if !ok || !strings.Contains(s, `"min_lat":49`) {
		t.Errorf("aoi json = %v", aoi)
	}
	if !strings.Contains(layers, `"airspace":true`) {
		t.Errorf("layers json = %q", layers)
	}
}

func TestIntegrationViewConfigRepo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)
	views := NewViewConfigRepo(pool)

	ten, _ := tenants.Create(ctx, "frankfurt", "FFM")

	if _, err := views.GetTenantDefault(ctx, ten.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no default yet: want ErrNotFound, got %v", err)
	}

	flMin := 100
	box := &BBox{MinLat: 49, MinLon: 8, MaxLat: 51, MaxLon: 9}
	def, err := views.UpsertTenantDefault(ctx, ten.ID, ViewConfig{
		CenterLat: 50.03, CenterLon: 8.57, Zoom: 8, AOI: box, FLMin: &flMin,
		Layers: map[string]bool{"airspace": true},
	})
	if err != nil {
		t.Fatalf("upsert default: %v", err)
	}
	if def.UserID != nil || def.AOI == nil || def.AOI.MaxLat != 51 || def.FLMin == nil || *def.FLMin != 100 {
		t.Fatalf("default round-trip: %+v", def)
	}
	if !def.Layers["airspace"] {
		t.Fatalf("layers did not round-trip: %+v", def.Layers)
	}

	// Upsert again updates in place: same row, new zoom, AOI/FL cleared.
	def2, err := views.UpsertTenantDefault(ctx, ten.ID, ViewConfig{CenterLat: 50, CenterLon: 8, Zoom: 9})
	if err != nil {
		t.Fatalf("upsert default 2: %v", err)
	}
	if def2.ID != def.ID {
		t.Fatalf("upsert created a new row: %d != %d", def2.ID, def.ID)
	}
	if def2.Zoom != 9 || def2.AOI != nil || def2.FLMin != nil || len(def2.Layers) != 0 {
		t.Fatalf("update did not replace fields: %+v", def2)
	}

	email := "l@ffm.example"
	u, _ := users.Create(ctx, ten.ID, "oidc|1", &email, RoleOperator)

	// Without an override, the effective view is the tenant default.
	eff, err := views.GetEffective(ctx, ten.ID, u.ID)
	if err != nil || eff.ID != def2.ID {
		t.Fatalf("effective (default) = %+v, %v", eff, err)
	}

	ov, err := views.UpsertUserOverride(ctx, ten.ID, u.ID, ViewConfig{CenterLat: 48, CenterLon: 11, Zoom: 10})
	if err != nil {
		t.Fatalf("upsert override: %v", err)
	}
	if ov.UserID == nil || *ov.UserID != u.ID {
		t.Fatalf("override user id: %+v", ov)
	}

	// Idempotent: a second override for the same user updates the same row.
	ov2, err := views.UpsertUserOverride(ctx, ten.ID, u.ID, ViewConfig{CenterLat: 47, CenterLon: 10, Zoom: 11})
	if err != nil || ov2.ID != ov.ID {
		t.Fatalf("override upsert created a new row: id %d -> %d, %v", ov.ID, ov2.ID, err)
	}

	// Now the effective view is the override.
	eff, err = views.GetEffective(ctx, ten.ID, u.ID)
	if err != nil || eff.ID != ov.ID || eff.Zoom != 11 {
		t.Fatalf("effective (override) = %+v, %v", eff, err)
	}
}
