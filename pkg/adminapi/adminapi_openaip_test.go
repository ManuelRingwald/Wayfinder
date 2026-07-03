package adminapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// fakeAeroLife records the per-tenant OpenAIP apply/stop calls the handlers make
// (ONB-6), so the tests can prove live-apply is triggered on key change and
// teardown on tenant delete.
type fakeAeroLife struct {
	applied   []int64
	refreshed []int64
	stopped   []int64
}

func (l *fakeAeroLife) Apply(_ context.Context, tenantID int64) {
	l.applied = append(l.applied, tenantID)
}
func (l *fakeAeroLife) Refresh(_ context.Context, tenantID int64) {
	l.refreshed = append(l.refreshed, tenantID)
}
func (l *fakeAeroLife) Stop(tenantID int64) { l.stopped = append(l.stopped, tenantID) }

func handlerForOpenAIP(ft fakeTenants, aero TenantAeroLifecycle) *Handler {
	return New(&fakeVS{}, &fakeVS{}, fakeFeeds{}, ft, &fakeUserStore{}, &fakeCredStore{},
		&fakeEntitlements{}, nil, nil, aero, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

// fakeAeroCache is a stub AeroCacheStatusReader (AERO-1) returning fixed status.
type fakeAeroCache struct {
	fetchedAt *time.Time
	count     int
	ok        bool
}

func (f fakeAeroCache) AeroCacheStatus(_ context.Context, _ int64) (*time.Time, int, bool, error) {
	return f.fetchedAt, f.count, f.ok, nil
}

func TestGetTenantOpenAIPReportsCacheStatus(t *testing.T) {
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	at := time.Unix(1_700_000_000, 0).UTC()
	h := handlerForOpenAIP(ft, nil).WithAeroCache(fakeAeroCache{fetchedAt: &at, count: 42, ok: true})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/5/openaip", "", 1, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got struct {
		Configured   bool       `json:"configured"`
		FetchedAt    *time.Time `json:"fetched_at"`
		FeatureCount *int       `json:"feature_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.FetchedAt == nil || !got.FetchedAt.Equal(at) {
		t.Errorf("fetched_at = %v, want %v", got.FetchedAt, at)
	}
	if got.FeatureCount == nil || *got.FeatureCount != 42 {
		t.Errorf("feature_count = %v, want 42", got.FeatureCount)
	}
}

func TestGetTenantOpenAIPOmitsCacheStatusWhenEmpty(t *testing.T) {
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	// Reader wired but nothing cached (ok=false) → cache fields omitted.
	h := handlerForOpenAIP(ft, nil).WithAeroCache(fakeAeroCache{ok: false})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/5/openaip", "", 1, store.RoleAdmin))
	if body := rec.Body.String(); strings.Contains(body, "fetched_at") || strings.Contains(body, "feature_count") {
		t.Errorf("empty cache status should omit fields, got %s", body)
	}
}

func TestGetTenantOpenAIPReportsConfigured(t *testing.T) {
	key := "secret-key"
	ft := fakeTenants{
		byID:       map[int64]store.Tenant{5: {ID: 5}, 6: {ID: 6}},
		openaipKey: map[int64]*string{5: &key}, // 6 has none
	}
	h := handlerForOpenAIP(ft, nil)

	for _, tc := range []struct {
		tid  int64
		want bool
	}{{5, true}, {6, false}} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/"+strconv.FormatInt(tc.tid, 10)+"/openaip", "", 1, store.RoleAdmin))
		if rec.Code != http.StatusOK {
			t.Fatalf("tenant %d: status = %d, want 200", tc.tid, rec.Code)
		}
		var got struct {
			Configured bool `json:"configured"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got.Configured != tc.want {
			t.Errorf("tenant %d: configured = %v, want %v", tc.tid, got.Configured, tc.want)
		}
	}
}

func TestGetTenantOpenAIPNeverLeaksKey(t *testing.T) {
	key := "super-secret"
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}, openaipKey: map[int64]*string{5: &key}}
	rec := httptest.NewRecorder()
	handlerForOpenAIP(ft, nil).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/5/openaip", "", 1, store.RoleAdmin))
	if body := rec.Body.String(); strings.Contains(body, key) {
		t.Fatalf("response leaked the key: %s", body)
	}
}

func TestSetTenantOpenAIPSetsKeyAndRefreshes(t *testing.T) {
	ft := fakeTenants{
		byID:        map[int64]store.Tenant{5: {ID: 5}},
		openaipSet:  map[int64]*string{},
		openaipCall: map[int64]bool{},
	}
	aero := &fakeAeroLife{}
	rec := httptest.NewRecorder()
	handlerForOpenAIP(ft, aero).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/tenants/5/openaip",
		`{"api_key":"  my-key  "}`, 1, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	got := ft.openaipSet[5]
	if got == nil || *got != "my-key" {
		t.Fatalf("stored key = %v, want trimmed \"my-key\"", got)
	}
	// Setting a key is an explicit "fetch now" (AERO-1): it forces a refresh, not the
	// idempotent Apply.
	if len(aero.refreshed) != 1 || aero.refreshed[0] != 5 {
		t.Errorf("refresh calls = %v, want [5]", aero.refreshed)
	}
	if len(aero.applied) != 0 {
		t.Errorf("setting a key should force a refresh, not Apply; applied = %v", aero.applied)
	}
}

func TestSetTenantOpenAIPNullClearsKey(t *testing.T) {
	ft := fakeTenants{
		byID:        map[int64]store.Tenant{5: {ID: 5}},
		openaipSet:  map[int64]*string{},
		openaipCall: map[int64]bool{},
	}
	aero := &fakeAeroLife{}
	for _, body := range []string{`{"api_key":null}`, `{"api_key":"   "}`, `{}`} {
		ft.openaipCall[5] = false
		rec := httptest.NewRecorder()
		handlerForOpenAIP(ft, aero).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/tenants/5/openaip", body, 1, store.RoleAdmin))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("body %q: status = %d, want 204", body, rec.Code)
		}
		if !ft.openaipCall[5] {
			t.Errorf("body %q: SetOpenAIPKey was not called", body)
		}
		if ft.openaipSet[5] != nil {
			t.Errorf("body %q: expected a nil (clear), got %v", body, *ft.openaipSet[5])
		}
	}
}

func TestSetTenantOpenAIPUnknownTenantIs404(t *testing.T) {
	ft := fakeTenants{byID: map[int64]store.Tenant{}, openaipSet: map[int64]*string{}}
	aero := &fakeAeroLife{}
	rec := httptest.NewRecorder()
	handlerForOpenAIP(ft, aero).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/tenants/9/openaip",
		`{"api_key":"k"}`, 1, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if len(aero.applied) != 0 {
		t.Errorf("a 404 must not apply, got %v", aero.applied)
	}
}

func TestSetTenantOpenAIPInvalidBodyIs400(t *testing.T) {
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rec := httptest.NewRecorder()
	handlerForOpenAIP(ft, &fakeAeroLife{}).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/tenants/5/openaip",
		`not json`, 1, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestOpenAIPRoutesForbidNonAdmin(t *testing.T) {
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	h := handlerForOpenAIP(ft, &fakeAeroLife{})
	for _, tc := range []struct {
		method, body string
	}{
		{http.MethodGet, ""},
		{http.MethodPut, `{"api_key":"k"}`},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, adminReq(tc.method, "/api/admin/tenants/5/openaip", tc.body, 7, store.RoleUser))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s as user: status = %d, want 403", tc.method, rec.Code)
		}
	}
}

func TestDeleteTenantStopsAero(t *testing.T) {
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}, deleted: map[int64]bool{}}
	aero := &fakeAeroLife{}
	h := handlerForOpenAIP(ft, aero)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodDelete, "/api/admin/tenants/5", "", 1, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if len(aero.stopped) != 1 || aero.stopped[0] != 5 {
		t.Errorf("aero stop calls = %v, want [5]", aero.stopped)
	}
}
