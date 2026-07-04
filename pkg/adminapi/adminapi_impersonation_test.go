package adminapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/feature"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// With a read-tenant marker on the context (set by the impersonation middleware,
// ADR 0008 Nachtrag) whoami must resolve every tenant-scoped field — features,
// sensor classes, effective view — against the TARGET tenant and disclose the
// state via impersonated_tenant_id, while the identity fields stay the caller's.
func TestWhoamiImpersonationResolvesTargetTenant(t *testing.T) {
	vs := &fakeVS{}
	fe := &fakeEntitlements{eff: map[feature.Key]bool{feature.Airspaces: true}}
	h := handlerWithEnt(vs, fakeFeeds{}, fakeTenants{}, fe)

	req := adminReq(http.MethodGet, "/api/admin/whoami", "", 7, store.RoleAdmin)
	req = req.WithContext(tenant.WithReadTenant(req.Context(), 9))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if vs.effTenant != 9 {
		t.Errorf("effective view resolved for tenant %d, want target 9", vs.effTenant)
	}
	if vs.subsTenant != 9 {
		t.Errorf("sensor classes resolved for tenant %d, want target 9", vs.subsTenant)
	}
	if fe.effTenant != 9 {
		t.Errorf("features resolved for tenant %d, want target 9", fe.effTenant)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["impersonated_tenant_id"] != 9.0 {
		t.Errorf("impersonated_tenant_id = %v, want 9", got["impersonated_tenant_id"])
	}
	if got["tenant_id"] != 7.0 {
		t.Errorf("tenant_id = %v, want the caller's own 7 (identity untouched)", got["tenant_id"])
	}
}

// Without the marker whoami stays byte-identical: everything resolves against
// the caller's own tenant and the disclosure field is absent.
func TestWhoamiWithoutImpersonationUnchanged(t *testing.T) {
	vs := &fakeVS{}
	fe := &fakeEntitlements{eff: map[feature.Key]bool{}}
	h := handlerWithEnt(vs, fakeFeeds{}, fakeTenants{}, fe)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/whoami", "", 7, store.RoleAdmin))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if vs.effTenant != 7 || fe.effTenant != 7 {
		t.Errorf("resolved tenants = (view %d, features %d), want own 7", vs.effTenant, fe.effTenant)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if _, present := got["impersonated_tenant_id"]; present {
		t.Error("impersonated_tenant_id must be omitted on the normal path")
	}
}
