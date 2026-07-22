package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// memTenantStore is an in-memory tenant_map_settings double keyed by (tenantID, key).
type memTenantStore struct{ m map[[2]any]string }

func newMemTenantStore() *memTenantStore { return &memTenantStore{m: map[[2]any]string{}} }

func (s *memTenantStore) Get(_ context.Context, tid int64, key string) (string, bool, error) {
	v, ok := s.m[[2]any{tid, key}]
	return v, ok, nil
}
func (s *memTenantStore) Set(_ context.Context, tid int64, key, value string) error {
	s.m[[2]any{tid, key}] = value
	return nil
}
func (s *memTenantStore) Delete(_ context.Context, tid int64, key string) error {
	delete(s.m, [2]any{tid, key})
	return nil
}

// T2 (ADR 0035): the tenant-effective base map resolves tenant ?? global ?? env
// and stays isolated between tenants (the "greift nur pro Mandant" guarantee).
func TestTenantBasemapEffectiveIsolation(t *testing.T) {
	ctx := context.Background()
	cfg := Config{MapTheme: mapThemeBKGDark, BKGStyleURL: "https://env.example/style.json"}
	md := newMapDataConfig(newMemSettings(), cfg, nil, nil, newMemTenantStore())

	// No overrides → env default for every tenant + the global (0) scope.
	for _, id := range []int64{0, 1, 2} {
		if got := md.effectiveThemeForTenant(ctx, id); got != mapThemeBKGDark {
			t.Fatalf("tenant %d theme = %q, want env default", id, got)
		}
	}

	// Tenant 1 overrides the theme to light → only tenant 1 changes.
	if err := md.themeTenant.Set(ctx, 1, mapThemeBKG); err != nil {
		t.Fatal(err)
	}
	if got := md.effectiveThemeForTenant(ctx, 1); got != mapThemeBKG {
		t.Fatalf("tenant 1 theme = %q, want override %q", got, mapThemeBKG)
	}
	if got := md.effectiveThemeForTenant(ctx, 2); got != mapThemeBKGDark {
		t.Fatalf("tenant 2 leaked = %q, want env default", got)
	}
	if got := md.effectiveThemeForTenant(ctx, 0); got != mapThemeBKGDark {
		t.Fatalf("global scope = %q, want env default", got)
	}

	// Tenant 1 overrides its style URL too.
	_ = md.styleURLTenant.Set(ctx, 1, "https://tenant1.example/style.json")
	if got := md.effectiveStyleURLForTenant(ctx, 1); got != "https://tenant1.example/style.json" {
		t.Fatalf("tenant 1 style = %q", got)
	}
	if got := md.effectiveStyleURLForTenant(ctx, 2); got != "https://env.example/style.json" {
		t.Fatalf("tenant 2 style leaked = %q", got)
	}
}

// The per-tenant admin handler stores + reports an override (GET/PUT), validates,
// and resets on an empty value.
func TestTenantBasemapAdminHandler(t *testing.T) {
	cfg := Config{MapTheme: mapThemeBKGDark}
	md := newMapDataConfig(newMemSettings(), cfg, nil, nil, newMemTenantStore())
	h := md.tenantBasemapHandler(md.themeTenant, validTheme)

	do := func(method, tid, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/admin/tenants/"+tid+"/mapdata/basemap/theme", strings.NewReader(body))
		req.SetPathValue("tenantID", tid)
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}

	// GET before override → global default, not overridden.
	rec := do(http.MethodGet, "7", "")
	var got tenantSettingResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Value != mapThemeBKGDark || got.Overridden {
		t.Fatalf("initial GET = %+v", got)
	}

	// PUT a valid override.
	rec = do(http.MethodPut, "7", `{"value":"bkg"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT code = %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Value != mapThemeBKG || !got.Overridden {
		t.Fatalf("after PUT = %+v", got)
	}

	// Invalid value → 400, not stored.
	if rec := do(http.MethodPut, "7", `{"value":"neon"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid PUT code = %d, want 400", rec.Code)
	}

	// Empty value → reset to global.
	rec = do(http.MethodPut, "7", `{"value":""}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Overridden {
		t.Fatalf("after reset still overridden: %+v", got)
	}

	// A bad tenant id → 400.
	if rec := do(http.MethodGet, "0", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("tenant 0 code = %d, want 400", rec.Code)
	}
}
