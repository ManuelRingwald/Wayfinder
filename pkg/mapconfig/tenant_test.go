package mapconfig

import (
	"context"
	"errors"
	"testing"
)

// fakeTenantStore is an in-memory tenant_map_settings double keyed by
// (tenantID, key). errOn forces an error on the named op to exercise fallback.
type fakeTenantStore struct {
	m     map[[2]any]string
	errOn string
}

func newFakeTenantStore() *fakeTenantStore { return &fakeTenantStore{m: map[[2]any]string{}} }

func (f *fakeTenantStore) Get(_ context.Context, tenantID int64, key string) (string, bool, error) {
	if f.errOn == "get" {
		return "", false, errors.New("boom")
	}
	v, ok := f.m[[2]any{tenantID, key}]
	return v, ok, nil
}
func (f *fakeTenantStore) Set(_ context.Context, tenantID int64, key, value string) error {
	if f.errOn == "set" {
		return errors.New("boom")
	}
	f.m[[2]any{tenantID, key}] = value
	return nil
}
func (f *fakeTenantStore) Delete(_ context.Context, tenantID int64, key string) error {
	delete(f.m, [2]any{tenantID, key})
	return nil
}

// TestTenantSettingPrecedence proves the three-tier resolution:
// tenant-override ?? global override ?? env default.
func TestTenantSettingPrecedence(t *testing.T) {
	ctx := context.Background()
	glob := newFakeStore()
	ten := newFakeTenantStore()
	setting := NewSetting(glob, "mapdata.basemap.theme", "bkg-dark") // env default
	ts := NewTenantSetting(ten, setting)

	// No overrides → env default, for any tenant and the global (0) scope.
	for _, id := range []int64{0, 1, 2} {
		if v, _ := ts.Effective(ctx, id); v != "bkg-dark" {
			t.Fatalf("tenant %d default = %q, want env default", id, v)
		}
	}

	// Global override → applies to every tenant that has no own override.
	_ = setting.Set(ctx, "bkg")
	if v, _ := ts.Effective(ctx, 1); v != "bkg" {
		t.Fatalf("tenant 1 = %q, want global override bkg", v)
	}
	if v, _ := ts.Effective(ctx, 0); v != "bkg" {
		t.Fatalf("global scope = %q, want global override bkg", v)
	}

	// Tenant 1 override → wins for tenant 1 only.
	if err := ts.Set(ctx, 1, "bkg-dark"); err != nil {
		t.Fatal(err)
	}
	if v, _ := ts.Effective(ctx, 1); v != "bkg-dark" {
		t.Fatalf("tenant 1 override = %q, want bkg-dark", v)
	}
}

// TestTenantSettingIsolation is the key guard: one tenant's override never leaks
// to another tenant (the "prüfe, ob die Konfig nur pro Mandant greift" gate).
func TestTenantSettingIsolation(t *testing.T) {
	ctx := context.Background()
	glob := newFakeStore()
	ten := newFakeTenantStore()
	ts := NewTenantSetting(ten, NewSetting(glob, "mapdata.basemap.style_url", "https://env.example/style.json"))

	if err := ts.Set(ctx, 1, "https://tenant-a.example/style.json"); err != nil {
		t.Fatal(err)
	}
	// Tenant A sees its own value.
	if v, _ := ts.Effective(ctx, 1); v != "https://tenant-a.example/style.json" {
		t.Fatalf("tenant A = %q", v)
	}
	// Tenant B is completely unaffected — still the env/global default.
	if v, _ := ts.Effective(ctx, 2); v != "https://env.example/style.json" {
		t.Fatalf("tenant B leaked = %q, want env default", v)
	}
	// The Overridden flag is per-tenant too.
	if ov, _ := ts.Overridden(ctx, 1); !ov {
		t.Fatal("tenant A should be overridden")
	}
	if ov, _ := ts.Overridden(ctx, 2); ov {
		t.Fatal("tenant B must NOT be overridden")
	}
}

// TestTenantSettingResetAndNoScope covers reset-to-fallback and the tenantID 0 /
// nil-store guards.
func TestTenantSettingResetAndNoScope(t *testing.T) {
	ctx := context.Background()
	glob := newFakeStore()
	ten := newFakeTenantStore()
	ts := NewTenantSetting(ten, NewSetting(glob, "k", "envval"))

	_ = ts.Set(ctx, 5, "tenantval")
	if v, _ := ts.Effective(ctx, 5); v != "tenantval" {
		t.Fatalf("got %q", v)
	}
	// Reset → back to global/env.
	if err := ts.Reset(ctx, 5); err != nil {
		t.Fatal(err)
	}
	if v, _ := ts.Effective(ctx, 5); v != "envval" {
		t.Fatalf("after reset = %q, want envval", v)
	}
	// An empty Set is also a reset.
	_ = ts.Set(ctx, 5, "x")
	_ = ts.Set(ctx, 5, "")
	if ov, _ := ts.Overridden(ctx, 5); ov {
		t.Fatal("empty Set should clear the override")
	}

	// tenantID 0 (platform admin) → global-only; writes are rejected.
	if err := ts.Set(ctx, 0, "nope"); err == nil {
		t.Fatal("Set with tenantID 0 must error")
	}
	if v, _ := ts.Effective(ctx, 0); v != "envval" {
		t.Fatalf("tenantID 0 = %q, want env", v)
	}

	// Nil store → global-only, no panic.
	gOnly := NewTenantSetting(nil, NewSetting(glob, "k", "envval"))
	if v, _ := gOnly.Effective(ctx, 9); v != "envval" {
		t.Fatalf("nil-store = %q", v)
	}
	if err := gOnly.Set(ctx, 9, "x"); err == nil {
		t.Fatal("nil-store Set must error")
	}
}

// A tenant store error degrades to the global value (never fails the read).
func TestTenantSettingDegradesOnStoreError(t *testing.T) {
	ctx := context.Background()
	glob := newFakeStore()
	ten := newFakeTenantStore()
	ten.errOn = "get"
	ts := NewTenantSetting(ten, NewSetting(glob, "k", "envval"))
	if v, _ := ts.Effective(ctx, 1); v != "envval" {
		t.Fatalf("store error should degrade to global, got %q", v)
	}
}
