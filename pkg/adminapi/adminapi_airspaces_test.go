package adminapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

type fakeAirspaceLister struct{ opts []AirspaceOption }

func (f fakeAirspaceLister) ListAirspaces(_ int64) []AirspaceOption { return f.opts }

func TestGetTenantAirspacesReturnsOptions(t *testing.T) {
	typ, cls := 4, 3
	lister := fakeAirspaceLister{opts: []AirspaceOption{
		{ID: "62a1", Name: "HAMBURG CTR", Type: &typ, ICAOClass: &cls},
		{ID: "62b2", Name: "HAMBURG TMA"},
	}}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	h := handlerWith(&fakeVS{}, fakeFeeds{}, ft).WithAirspaceLister(lister)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/5/airspaces", "", 7, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []AirspaceOption
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 options, got %d", len(got))
	}
	if got[0].ID != "62a1" || got[0].Name != "HAMBURG CTR" || got[0].Type == nil || *got[0].Type != 4 {
		t.Errorf("unexpected first option %+v", got[0])
	}
	// type/icao_class are omitted when the airspace lacked them (omitempty).
	if got[1].Type != nil || got[1].ICAOClass != nil {
		t.Errorf("expected omitted type/class on the second option, got %+v", got[1])
	}
}

func TestGetTenantAirspaces404WhenNoLister(t *testing.T) {
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	h := handlerWith(&fakeVS{}, fakeFeeds{}, ft) // no WithAirspaceLister
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/5/airspaces", "", 7, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (no lister wired)", rec.Code)
	}
}

func TestGetTenantAirspacesUnknownTenantIs404(t *testing.T) {
	h := handlerWith(&fakeVS{}, fakeFeeds{}, fakeTenants{byID: map[int64]store.Tenant{}}).
		WithAirspaceLister(fakeAirspaceLister{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/9/airspaces", "", 7, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (unknown tenant)", rec.Code)
	}
}

func TestGetTenantAirspacesForbidsNonAdmin(t *testing.T) {
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	h := handlerWith(&fakeVS{}, fakeFeeds{}, ft).WithAirspaceLister(fakeAirspaceLister{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/5/airspaces", "", 7, store.RoleUser))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (requireAdmin)", rec.Code)
	}
}
