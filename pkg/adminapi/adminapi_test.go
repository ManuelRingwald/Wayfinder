package adminapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// fakeVS satisfies ViewStore + SubscriptionStore and records the tenant/feed it
// was called with (to prove tenant-scoping and grant/revoke targeting).
type fakeVS struct {
	vc           store.ViewConfig
	getErr       error
	upsertTenant int64
	upserted     store.ViewConfig
	subsFeeds    []store.Feed
	subsTenant   int64
	grantTenant  int64
	grantFeed    int64
	revokeTenant int64
	revokeFeed   int64
}

func (f *fakeVS) GetEffective(_ context.Context, _, _ int64) (store.ViewConfig, error) {
	return f.vc, f.getErr
}

func (f *fakeVS) UpsertTenantDefault(_ context.Context, tenantID int64, vc store.ViewConfig) (store.ViewConfig, error) {
	f.upsertTenant = tenantID
	f.upserted = vc
	vc.TenantID = tenantID
	return vc, nil
}

func (f *fakeVS) ListFeedsByTenant(_ context.Context, tenantID int64) ([]store.Feed, error) {
	f.subsTenant = tenantID
	return f.subsFeeds, nil
}

func (f *fakeVS) Subscribe(_ context.Context, tid, fid int64) error {
	f.grantTenant, f.grantFeed = tid, fid
	return nil
}

func (f *fakeVS) Unsubscribe(_ context.Context, tid, fid int64) error {
	f.revokeTenant, f.revokeFeed = tid, fid
	return nil
}

type fakeFeeds struct {
	list []store.Feed
	byID map[int64]store.Feed
}

func (f fakeFeeds) List(_ context.Context) ([]store.Feed, error) { return f.list, nil }

func (f fakeFeeds) GetByID(_ context.Context, id int64) (store.Feed, error) {
	if x, ok := f.byID[id]; ok {
		return x, nil
	}
	return store.Feed{}, store.ErrNotFound
}

type fakeTenants struct {
	list []store.Tenant
	byID map[int64]store.Tenant
}

func (f fakeTenants) List(_ context.Context) ([]store.Tenant, error) { return f.list, nil }

func (f fakeTenants) GetByID(_ context.Context, id int64) (store.Tenant, error) {
	if x, ok := f.byID[id]; ok {
		return x, nil
	}
	return store.Tenant{}, store.ErrNotFound
}

func handlerWith(vs *fakeVS, ff fakeFeeds, ft fakeTenants) *Handler {
	return New(vs, vs, ff, ft, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func adminReq(method, path, body string, tenantID int64, role store.Role) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if tenantID != 0 {
		r = r.WithContext(tenant.WithIdentity(r.Context(),
			tenant.Identity{TenantID: tenantID, UserID: 1, Role: role}))
	}
	return r
}

// --- tenant_admin self-service (tenant from Identity) -----------------------

func TestGetView(t *testing.T) {
	flMin := 100
	vs := &fakeVS{vc: store.ViewConfig{CenterLat: 50, CenterLon: 9, Zoom: 8, FLMin: &flMin}}
	rec := httptest.NewRecorder()
	handlerWith(vs, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/view", "", 7, store.RoleTenantAdmin))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["center_lat"] != 50.0 || got["zoom"] != 8.0 || got["fl_min"] != 100.0 {
		t.Errorf("view body = %v", got)
	}
}

func TestGetViewNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerWith(&fakeVS{getErr: store.ErrNotFound}, fakeFeeds{}, fakeTenants{}).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/view", "", 7, store.RoleTenantAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestAdminUnauthorizedWithoutIdentity(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerWith(&fakeVS{}, fakeFeeds{}, fakeTenants{}).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/view", "", 0, ""))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (no identity)", rec.Code)
	}
}

// TestPutViewIsTenantScoped is the isolation crux: the upsert targets the tenant
// from the Identity, never one supplied by the client.
func TestPutViewIsTenantScoped(t *testing.T) {
	vs := &fakeVS{}
	rec := httptest.NewRecorder()
	body := `{"center_lat":50,"center_lon":9,"zoom":8,"fl_min":100,"fl_max":300}`
	handlerWith(vs, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/view", body, 7, store.RoleTenantAdmin))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if vs.upsertTenant != 7 {
		t.Errorf("upsert tenant = %d, want 7 (from Identity, not body)", vs.upsertTenant)
	}
	if vs.upserted.CenterLat != 50 || vs.upserted.FLMin == nil || *vs.upserted.FLMin != 100 {
		t.Errorf("upserted view = %+v", vs.upserted)
	}
}

func TestPutViewValidation(t *testing.T) {
	cases := map[string]string{
		"bad lat":      `{"center_lat":91,"center_lon":9,"zoom":8}`,
		"bad lon":      `{"center_lat":50,"center_lon":181,"zoom":8}`,
		"bad zoom":     `{"center_lat":50,"center_lon":9,"zoom":25}`,
		"inverted aoi": `{"center_lat":50,"center_lon":9,"zoom":8,"aoi":{"min_lat":51,"min_lon":8,"max_lat":49,"max_lon":10}}`,
		"aoi range":    `{"center_lat":50,"center_lon":9,"zoom":8,"aoi":{"min_lat":-91,"min_lon":8,"max_lat":51,"max_lon":10}}`,
		"fl inverted":  `{"center_lat":50,"center_lon":9,"zoom":8,"fl_min":300,"fl_max":100}`,
		"bad json":     `not-json`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			vs := &fakeVS{}
			rec := httptest.NewRecorder()
			handlerWith(vs, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/view", body, 7, store.RoleTenantAdmin))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if vs.upsertTenant != 0 {
				t.Errorf("invalid view must not reach the store (tenant=%d)", vs.upsertTenant)
			}
		})
	}
}

func TestGetSubscriptionsIsTenantScoped(t *testing.T) {
	region := "Hessen"
	vs := &fakeVS{subsFeeds: []store.Feed{{ID: 1, Name: "Frankfurt", Region: &region, SensorMix: []string{"PSR"}}}}
	rec := httptest.NewRecorder()
	handlerWith(vs, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/subscriptions", "", 7, store.RoleTenantAdmin))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if vs.subsTenant != 7 {
		t.Errorf("subscriptions tenant = %d, want 7 (from Identity)", vs.subsTenant)
	}
	var got []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0]["name"] != "Frankfurt" {
		t.Errorf("subscriptions body = %v", got)
	}
}

func TestGetFeeds(t *testing.T) {
	ff := fakeFeeds{list: []store.Feed{{ID: 1, Name: "Frankfurt"}, {ID: 2, Name: "Stuttgart"}}}
	rec := httptest.NewRecorder()
	handlerWith(&fakeVS{}, ff, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/feeds", "", 7, store.RoleTenantAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Errorf("feeds body = %v, want 2 entries", got)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerWith(&fakeVS{}, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/view", `{}`, 7, store.RoleTenantAdmin))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/admin/view status = %d, want 405", rec.Code)
	}
}

// --- super_admin cross-tenant provisioning ----------------------------------

func provisioningFixture() (*fakeVS, fakeFeeds, fakeTenants) {
	return &fakeVS{},
		fakeFeeds{byID: map[int64]store.Feed{3: {ID: 3, Name: "Frankfurt"}}},
		fakeTenants{
			list: []store.Tenant{{ID: 5, Slug: "acme", Name: "ACME", Status: "active"}},
			byID: map[int64]store.Tenant{5: {ID: 5, Slug: "acme", Name: "ACME", Status: "active"}},
		}
}

func TestListTenantsSuperAdmin(t *testing.T) {
	vs, ff, ft := provisioningFixture()
	rec := httptest.NewRecorder()
	handlerWith(vs, ff, ft).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants", "", 99, store.RoleSuperAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0]["slug"] != "acme" {
		t.Errorf("tenants body = %v", got)
	}
}

func TestGrantSubscription(t *testing.T) {
	vs, ff, ft := provisioningFixture()
	rec := httptest.NewRecorder()
	handlerWith(vs, ff, ft).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/tenants/5/subscriptions", `{"feed_id":3}`, 99, store.RoleSuperAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if vs.grantTenant != 5 || vs.grantFeed != 3 {
		t.Errorf("granted (tenant=%d, feed=%d), want (5, 3) — target from path", vs.grantTenant, vs.grantFeed)
	}
}

func TestGrantValidation(t *testing.T) {
	cases := map[string]struct {
		path string
		body string
		want int
	}{
		"unknown tenant": {"/api/admin/tenants/999/subscriptions", `{"feed_id":3}`, http.StatusNotFound},
		"unknown feed":   {"/api/admin/tenants/5/subscriptions", `{"feed_id":999}`, http.StatusNotFound},
		"missing feed":   {"/api/admin/tenants/5/subscriptions", `{}`, http.StatusBadRequest},
		"bad json":       {"/api/admin/tenants/5/subscriptions", `nope`, http.StatusBadRequest},
		"bad tenant id":  {"/api/admin/tenants/abc/subscriptions", `{"feed_id":3}`, http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			vs, ff, ft := provisioningFixture()
			rec := httptest.NewRecorder()
			handlerWith(vs, ff, ft).ServeHTTP(rec, adminReq(http.MethodPost, tc.path, tc.body, 99, store.RoleSuperAdmin))
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if vs.grantTenant != 0 {
				t.Errorf("invalid grant must not reach the store (tenant=%d)", vs.grantTenant)
			}
		})
	}
}

func TestRevokeSubscription(t *testing.T) {
	vs, ff, ft := provisioningFixture()
	rec := httptest.NewRecorder()
	handlerWith(vs, ff, ft).ServeHTTP(rec, adminReq(http.MethodDelete, "/api/admin/tenants/5/subscriptions/3", "", 99, store.RoleSuperAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if vs.revokeTenant != 5 || vs.revokeFeed != 3 {
		t.Errorf("revoked (tenant=%d, feed=%d), want (5, 3)", vs.revokeTenant, vs.revokeFeed)
	}
}

// TestCrossTenantRoutesForbidTenantAdmin is the cross-tenant security negative
// test: a tenant_admin must NOT be able to use the provisioning routes (403), and
// no grant/revoke may reach the store.
func TestCrossTenantRoutesForbidTenantAdmin(t *testing.T) {
	routes := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/admin/tenants", ""},
		{http.MethodGet, "/api/admin/tenants/5/subscriptions", ""},
		{http.MethodPost, "/api/admin/tenants/5/subscriptions", `{"feed_id":3}`},
		{http.MethodDelete, "/api/admin/tenants/5/subscriptions/3", ""},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			vs, ff, ft := provisioningFixture()
			rec := httptest.NewRecorder()
			handlerWith(vs, ff, ft).ServeHTTP(rec, adminReq(rt.method, rt.path, rt.body, 7, store.RoleTenantAdmin))
			if rec.Code != http.StatusForbidden {
				t.Errorf("tenant_admin on %s %s = %d, want 403", rt.method, rt.path, rec.Code)
			}
			if vs.grantTenant != 0 || vs.revokeTenant != 0 {
				t.Errorf("tenant_admin must not reach grant/revoke (grant=%d revoke=%d)", vs.grantTenant, vs.revokeTenant)
			}
		})
	}
}
