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

// fakeStore satisfies ViewStore, SubscriptionStore and FeedStore and records the
// tenant id it was called with (to prove tenant-scoping comes from the Identity).
type fakeStore struct {
	vc           store.ViewConfig
	getErr       error
	upsertTenant int64
	upserted     store.ViewConfig
	subsFeeds    []store.Feed
	subsTenant   int64
	feeds        []store.Feed
}

func (f *fakeStore) GetEffective(_ context.Context, _, _ int64) (store.ViewConfig, error) {
	return f.vc, f.getErr
}

func (f *fakeStore) UpsertTenantDefault(_ context.Context, tenantID int64, vc store.ViewConfig) (store.ViewConfig, error) {
	f.upsertTenant = tenantID
	f.upserted = vc
	vc.TenantID = tenantID
	return vc, nil
}

func (f *fakeStore) ListFeedsByTenant(_ context.Context, tenantID int64) ([]store.Feed, error) {
	f.subsTenant = tenantID
	return f.subsFeeds, nil
}

func (f *fakeStore) List(_ context.Context) ([]store.Feed, error) { return f.feeds, nil }

func testHandler(f *fakeStore) *Handler {
	return New(f, f, f, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func adminReq(method, path, body string, tenantID int64) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if tenantID != 0 {
		r = r.WithContext(tenant.WithIdentity(r.Context(),
			tenant.Identity{TenantID: tenantID, UserID: 1, Role: store.RoleTenantAdmin}))
	}
	return r
}

func TestGetView(t *testing.T) {
	flMin := 100
	f := &fakeStore{vc: store.ViewConfig{CenterLat: 50, CenterLon: 9, Zoom: 8, FLMin: &flMin}}
	rec := httptest.NewRecorder()
	testHandler(f).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/view", "", 7))

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
	testHandler(&fakeStore{getErr: store.ErrNotFound}).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/view", "", 7))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestAdminUnauthorizedWithoutIdentity(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(&fakeStore{}).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/view", "", 0))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (no identity)", rec.Code)
	}
}

// TestPutViewIsTenantScoped is the isolation crux: the upsert targets the tenant
// from the Identity, never one supplied by the client.
func TestPutViewIsTenantScoped(t *testing.T) {
	f := &fakeStore{}
	rec := httptest.NewRecorder()
	body := `{"center_lat":50,"center_lon":9,"zoom":8,"fl_min":100,"fl_max":300}`
	testHandler(f).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/view", body, 7))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if f.upsertTenant != 7 {
		t.Errorf("upsert tenant = %d, want 7 (from Identity, not body)", f.upsertTenant)
	}
	if f.upserted.CenterLat != 50 || f.upserted.FLMin == nil || *f.upserted.FLMin != 100 {
		t.Errorf("upserted view = %+v", f.upserted)
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
			f := &fakeStore{}
			rec := httptest.NewRecorder()
			testHandler(f).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/view", body, 7))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if f.upsertTenant != 0 {
				t.Errorf("invalid view must not reach the store (tenant=%d)", f.upsertTenant)
			}
		})
	}
}

func TestGetSubscriptionsIsTenantScoped(t *testing.T) {
	region := "Hessen"
	f := &fakeStore{subsFeeds: []store.Feed{{ID: 1, Name: "Frankfurt", Region: &region, SensorMix: []string{"PSR"}}}}
	rec := httptest.NewRecorder()
	testHandler(f).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/subscriptions", "", 7))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if f.subsTenant != 7 {
		t.Errorf("subscriptions tenant = %d, want 7 (from Identity)", f.subsTenant)
	}
	var got []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0]["name"] != "Frankfurt" {
		t.Errorf("subscriptions body = %v", got)
	}
}

func TestGetFeeds(t *testing.T) {
	f := &fakeStore{feeds: []store.Feed{{ID: 1, Name: "Frankfurt"}, {ID: 2, Name: "Stuttgart"}}}
	rec := httptest.NewRecorder()
	testHandler(f).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/feeds", "", 7))
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
	testHandler(&fakeStore{}).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/view", `{}`, 7))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/admin/view status = %d, want 405", rec.Code)
	}
}
