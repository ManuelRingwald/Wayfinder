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
	"time"

	"github.com/manuelringwald/wayfinder/pkg/feature"
	"github.com/manuelringwald/wayfinder/pkg/health"
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

func (f *fakeVS) GetTenantDefault(_ context.Context, _ int64) (store.ViewConfig, error) {
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
	list      []store.Feed
	byID      map[int64]store.Feed
	byName    map[string]store.Feed // for GetByName (ONB-5 duplicate pre-check)
	nextID    int64                 // id assigned by Create
	created   map[string]store.Feed // records Create calls by name (pre-init to record)
	deleted   map[int64]bool        // records Delete calls (pre-init to record)
	createErr error                 // when set, Create returns it (e.g. sensor-mix rejection)

	sourceCfg map[int64]store.SourceConfig // source config per feed (ORCH-1b)
	coverage  map[int64]*store.BBox        // derived coverage per feed (ORCH-1b)
}

func (f fakeFeeds) List(_ context.Context) ([]store.Feed, error) { return f.list, nil }

func (f fakeFeeds) GetByID(_ context.Context, id int64) (store.Feed, error) {
	if x, ok := f.byID[id]; ok {
		return x, nil
	}
	return store.Feed{}, store.ErrNotFound
}

func (f fakeFeeds) GetByName(_ context.Context, name string) (store.Feed, error) {
	if x, ok := f.byName[name]; ok {
		return x, nil
	}
	return store.Feed{}, store.ErrNotFound
}

func (f fakeFeeds) Create(_ context.Context, name, group string, port int, region *string, mix []string) (store.Feed, error) {
	if f.createErr != nil {
		return store.Feed{}, f.createErr
	}
	id := f.nextID
	if id == 0 {
		id = 1
	}
	feed := store.Feed{ID: id, Name: name, MulticastGroup: group, Port: port, Region: region, SensorMix: mix}
	if f.created != nil {
		f.created[name] = feed
	}
	return feed, nil
}

func (f fakeFeeds) Delete(_ context.Context, id int64) error {
	if f.deleted != nil {
		f.deleted[id] = true
	}
	return nil
}

func (f fakeFeeds) GetSourceConfig(_ context.Context, id int64) (store.SourceConfig, *store.BBox, error) {
	if _, ok := f.byID[id]; !ok {
		return nil, nil, store.ErrNotFound
	}
	return f.sourceCfg[id], f.coverage[id], nil
}

func (f fakeFeeds) SetSourceConfig(_ context.Context, id int64, sources store.SourceConfig, coverage *store.BBox) error {
	if _, ok := f.byID[id]; !ok {
		return store.ErrNotFound
	}
	if f.sourceCfg != nil {
		f.sourceCfg[id] = sources
	}
	if f.coverage != nil {
		f.coverage[id] = coverage
	}
	return nil
}

type fakeTenants struct {
	list        []store.Tenant
	byID        map[int64]store.Tenant
	bySlug      map[string]store.Tenant // for GetBySlug (ONB-4 duplicate pre-check)
	statusSet   map[int64]store.Status  // records SetStatus calls (AP6)
	created     map[string]store.Tenant // records Create calls by slug (ONB-4); pre-init to record
	deleted     map[int64]bool          // records Delete calls (ONB-4); pre-init to record
	openaipKey  map[int64]*string       // GetOpenAIPKey result per tenant (ONB-6)
	openaipSet  map[int64]*string       // records the value passed to SetOpenAIPKey; pre-init to record
	openaipCall map[int64]bool          // records that SetOpenAIPKey was called (distinguishes a nil clear); pre-init
	nextID      int64
}

func (f fakeTenants) List(_ context.Context) ([]store.Tenant, error) { return f.list, nil }

func (f fakeTenants) GetByID(_ context.Context, id int64) (store.Tenant, error) {
	if x, ok := f.byID[id]; ok {
		return x, nil
	}
	return store.Tenant{}, store.ErrNotFound
}

func (f fakeTenants) GetBySlug(_ context.Context, slug string) (store.Tenant, error) {
	if x, ok := f.bySlug[slug]; ok {
		return x, nil
	}
	return store.Tenant{}, store.ErrNotFound
}

func (f fakeTenants) Create(_ context.Context, slug, name string) (store.Tenant, error) {
	id := f.nextID
	if id == 0 {
		id = 1
	}
	t := store.Tenant{ID: id, Slug: slug, Name: name, Status: store.StatusActive}
	if f.created != nil {
		f.created[slug] = t
	}
	return t, nil
}

func (f fakeTenants) SetStatus(_ context.Context, id int64, status store.Status) error {
	if f.statusSet != nil {
		f.statusSet[id] = status
	}
	return nil
}

func (f fakeTenants) Delete(_ context.Context, id int64) error {
	if f.deleted != nil {
		f.deleted[id] = true
	}
	return nil
}

func (f fakeTenants) GetOpenAIPKey(_ context.Context, id int64) (*string, error) {
	return f.openaipKey[id], nil
}

func (f fakeTenants) SetOpenAIPKey(_ context.Context, id int64, key *string) error {
	if f.openaipSet != nil {
		f.openaipSet[id] = key
	}
	if f.openaipCall != nil {
		f.openaipCall[id] = true
	}
	return nil
}

// fakeUserStore satisfies UserStore and records mutations (AP6 access mgmt +
// ONB-3 platform-admin mgmt).
type fakeUserStore struct {
	byID         map[int64]store.User
	bySubject    map[string]store.User
	listByTen    map[int64][]store.User
	admins       []store.User // ListAdmins result
	created      store.User
	createErr    error
	statusSet    map[int64]store.Status
	mustChgSet   map[int64]bool
	activeAdmins int
	deleted      map[int64]bool
	nextID       int64
}

func (f *fakeUserStore) ListByTenant(_ context.Context, tenantID int64) ([]store.User, error) {
	return f.listByTen[tenantID], nil
}

func (f *fakeUserStore) ListAdmins(_ context.Context) ([]store.User, error) {
	return f.admins, nil
}

func (f *fakeUserStore) GetByID(_ context.Context, id int64) (store.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeUserStore) GetBySubject(_ context.Context, subject string) (store.User, error) {
	if u, ok := f.bySubject[subject]; ok {
		return u, nil
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeUserStore) Create(_ context.Context, tenantID int64, subject string, email *string) (store.User, error) {
	if f.createErr != nil {
		return store.User{}, f.createErr
	}
	id := f.nextID
	if id == 0 {
		id = 1
	}
	f.created = store.User{ID: id, TenantID: tenantID, Subject: subject, Email: email, Role: store.RoleUser, Status: store.StatusActive}
	return f.created, nil
}

func (f *fakeUserStore) CreateAdmin(_ context.Context, subject string, email *string) (store.User, error) {
	if f.createErr != nil {
		return store.User{}, f.createErr
	}
	id := f.nextID
	if id == 0 {
		id = 1
	}
	f.created = store.User{ID: id, TenantID: 0, Subject: subject, Email: email, Role: store.RoleAdmin, Status: store.StatusActive}
	return f.created, nil
}

func (f *fakeUserStore) SetStatus(_ context.Context, id int64, status store.Status) error {
	if f.statusSet == nil {
		f.statusSet = map[int64]store.Status{}
	}
	f.statusSet[id] = status
	return nil
}

func (f *fakeUserStore) SetMustChangePassword(_ context.Context, id int64, must bool) error {
	if f.mustChgSet == nil {
		f.mustChgSet = map[int64]bool{}
	}
	f.mustChgSet[id] = must
	return nil
}

func (f *fakeUserStore) CountActiveAdmins(_ context.Context) (int, error) {
	return f.activeAdmins, nil
}

func (f *fakeUserStore) Delete(_ context.Context, id int64) error {
	if f.deleted == nil {
		f.deleted = map[int64]bool{}
	}
	f.deleted[id] = true
	return nil
}

// fakeCredStore satisfies CredentialStore and records the last hash set. getHash
// (keyed by user id) backs the self-service password-change verification path.
type fakeCredStore struct {
	set     map[int64]string
	getHash map[int64]string
}

func (f *fakeCredStore) Set(_ context.Context, userID int64, passwordHash string) error {
	if f.set == nil {
		f.set = map[int64]string{}
	}
	f.set[userID] = passwordHash
	return nil
}

func (f *fakeCredStore) GetHash(_ context.Context, userID int64) (string, error) {
	if h, ok := f.getHash[userID]; ok {
		return h, nil
	}
	return "", store.ErrNotFound
}

// fakeEntitlements satisfies EntitlementService and records the last Set call
// (to prove tenant targeting and that cross-tenant gating blocks before it).
type fakeEntitlements struct {
	eff    map[feature.Key]bool
	effErr error
	setErr error
	setTid int64
	setKey feature.Key
	setVal bool
	has    map[feature.Key]bool
}

func (f *fakeEntitlements) Effective(_ context.Context, _ int64) (map[feature.Key]bool, error) {
	return f.eff, f.effErr
}

func (f *fakeEntitlements) Set(_ context.Context, tid int64, key feature.Key, enabled bool) error {
	f.setTid, f.setKey, f.setVal = tid, key, enabled
	return f.setErr
}

func (f *fakeEntitlements) HasFeature(_ context.Context, _ int64, key feature.Key) bool {
	return f.has[key]
}

func handlerWith(vs *fakeVS, ff fakeFeeds, ft fakeTenants) *Handler {
	return handlerWithEnt(vs, ff, ft, &fakeEntitlements{})
}

func handlerWithEnt(vs *fakeVS, ff fakeFeeds, ft fakeTenants, fe EntitlementService) *Handler {
	return New(vs, vs, ff, ft, &fakeUserStore{}, &fakeCredStore{}, fe, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

// handlerForUsers builds a handler wired with the given user/credential/tenant
// fakes for the AP6 access-management tests.
func handlerForUsers(us UserStore, cs CredentialStore, ft fakeTenants) *Handler {
	return New(&fakeVS{}, &fakeVS{}, fakeFeeds{}, ft, us, cs, &fakeEntitlements{}, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

// rescopeRecorder captures the tenant ids a handler asks to live-re-scope (WF2-33).
// Handlers invoke rescope synchronously in the request goroutine, so no locking.
type rescopeRecorder struct{ calls []int64 }

func (r *rescopeRecorder) fn(_ context.Context, tenantID int64) { r.calls = append(r.calls, tenantID) }

func handlerWithRescope(vs *fakeVS, ff fakeFeeds, ft fakeTenants, rescope RescopeFunc) *Handler {
	return New(vs, vs, ff, ft, &fakeUserStore{}, &fakeCredStore{}, &fakeEntitlements{}, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), rescope)
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

// --- whoami role probe (WF2-32) ---------------------------------------------

func TestWhoamiReportsIdentity(t *testing.T) {
	for _, role := range []store.Role{store.RoleAdmin, store.RoleAdmin} {
		rec := httptest.NewRecorder()
		handlerWith(&fakeVS{}, fakeFeeds{}, fakeTenants{}).
			ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/whoami", "", 7, role))
		if rec.Code != http.StatusOK {
			t.Fatalf("role %s: status = %d, want 200", role, rec.Code)
		}
		var got map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got["tenant_id"] != 7.0 || got["role"] != string(role) {
			t.Errorf("role %s: whoami body = %v", role, got)
		}
	}
}

func TestWhoamiIncludesEffectiveFeatures(t *testing.T) {
	fe := &fakeEntitlements{eff: map[feature.Key]bool{feature.STCA: true, feature.MultiFeed: false}}
	rec := httptest.NewRecorder()
	handlerWithEnt(&fakeVS{}, fakeFeeds{}, fakeTenants{}, fe).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/whoami", "", 7, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got struct {
		Features map[string]bool `json:"features"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Features["stca"] {
		t.Error("whoami features missing stca=true")
	}
	if got.Features["multi_feed"] {
		t.Error("whoami features multi_feed should be false (default-deny)")
	}
}

func TestWhoamiUnauthorizedWithoutIdentity(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerWith(&fakeVS{}, fakeFeeds{}, fakeTenants{}).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/whoami", "", 0, ""))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// --- live re-scope hook (WF2-33) --------------------------------------------

func TestPutViewTriggersRescope(t *testing.T) {
	rec := &rescopeRecorder{}
	h := handlerWithRescope(&fakeVS{}, fakeFeeds{}, fakeTenants{}, rec.fn)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, adminReq(http.MethodPut, "/api/admin/view",
		`{"center_lat":50,"center_lon":9,"zoom":8}`, 7, store.RoleAdmin))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(rec.calls) != 1 || rec.calls[0] != 7 {
		t.Errorf("rescope calls = %v, want [7] (tenant from Identity)", rec.calls)
	}
}

func TestPutViewInvalidDoesNotRescope(t *testing.T) {
	rec := &rescopeRecorder{}
	h := handlerWithRescope(&fakeVS{}, fakeFeeds{}, fakeTenants{}, rec.fn)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, adminReq(http.MethodPut, "/api/admin/view",
		`{"center_lat":50,"center_lon":9,"zoom":99}`, 7, store.RoleAdmin)) // zoom out of range
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if len(rec.calls) != 0 {
		t.Errorf("a rejected view must not re-scope, got %v", rec.calls)
	}
}

func TestGrantTriggersRescope(t *testing.T) {
	rec := &rescopeRecorder{}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	ff := fakeFeeds{byID: map[int64]store.Feed{3: {ID: 3}}}
	h := handlerWithRescope(&fakeVS{}, ff, ft, rec.fn)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, adminReq(http.MethodPost, "/api/admin/tenants/5/subscriptions",
		`{"feed_id":3}`, 1, store.RoleAdmin))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if len(rec.calls) != 1 || rec.calls[0] != 5 {
		t.Errorf("rescope calls = %v, want [5] (target tenant from path)", rec.calls)
	}
}

func TestRevokeTriggersRescope(t *testing.T) {
	rec := &rescopeRecorder{}
	h := handlerWithRescope(&fakeVS{}, fakeFeeds{}, fakeTenants{}, rec.fn)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, adminReq(http.MethodDelete, "/api/admin/tenants/5/subscriptions/3",
		"", 1, store.RoleAdmin))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if len(rec.calls) != 1 || rec.calls[0] != 5 {
		t.Errorf("rescope calls = %v, want [5]", rec.calls)
	}
}

// --- admin self-service (tenant from Identity) -----------------------

func TestGetView(t *testing.T) {
	flMin := 100
	vs := &fakeVS{vc: store.ViewConfig{CenterLat: 50, CenterLon: 9, Zoom: 8, FLMin: &flMin}}
	rec := httptest.NewRecorder()
	handlerWith(vs, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/view", "", 7, store.RoleAdmin))

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
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/view", "", 7, store.RoleAdmin))
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
	handlerWith(vs, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/view", body, 7, store.RoleAdmin))

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
			handlerWith(vs, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/view", body, 7, store.RoleAdmin))
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
	handlerWith(vs, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/subscriptions", "", 7, store.RoleAdmin))

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
	handlerWith(&fakeVS{}, ff, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/feeds", "", 7, store.RoleAdmin))
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
	handlerWith(&fakeVS{}, fakeFeeds{}, fakeTenants{}).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/view", `{}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/admin/view status = %d, want 405", rec.Code)
	}
}

// --- cross-tenant provisioning ----------------------------------

func provisioningFixture() (*fakeVS, fakeFeeds, fakeTenants) {
	return &fakeVS{},
		fakeFeeds{byID: map[int64]store.Feed{3: {ID: 3, Name: "Frankfurt"}}},
		fakeTenants{
			list: []store.Tenant{{ID: 5, Slug: "acme", Name: "ACME", Status: "active"}},
			byID: map[int64]store.Tenant{5: {ID: 5, Slug: "acme", Name: "ACME", Status: "active"}},
		}
}

func TestListTenantsAdmin(t *testing.T) {
	vs, ff, ft := provisioningFixture()
	rec := httptest.NewRecorder()
	handlerWith(vs, ff, ft).ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants", "", 99, store.RoleAdmin))
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
	handlerWith(vs, ff, ft).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/tenants/5/subscriptions", `{"feed_id":3}`, 99, store.RoleAdmin))
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
			handlerWith(vs, ff, ft).ServeHTTP(rec, adminReq(http.MethodPost, tc.path, tc.body, 99, store.RoleAdmin))
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
	handlerWith(vs, ff, ft).ServeHTTP(rec, adminReq(http.MethodDelete, "/api/admin/tenants/5/subscriptions/3", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if vs.revokeTenant != 5 || vs.revokeFeed != 3 {
		t.Errorf("revoked (tenant=%d, feed=%d), want (5, 3)", vs.revokeTenant, vs.revokeFeed)
	}
}

// TestCrossTenantRoutesForbidUser verifies that a user (non-admin) cannot reach
// the provisioning routes and no grant/revoke ever reaches the store.
func TestCrossTenantRoutesForbidUser(t *testing.T) {
	routes := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/admin/tenants", ""},
		{http.MethodGet, "/api/admin/tenants/5/subscriptions", ""},
		{http.MethodPost, "/api/admin/tenants/5/subscriptions", `{"feed_id":3}`},
		{http.MethodDelete, "/api/admin/tenants/5/subscriptions/3", ""},
		{http.MethodGet, "/api/admin/tenants/5/entitlements", ""},
		{http.MethodPut, "/api/admin/tenants/5/entitlements/stca", `{"enabled":true}`},
		{http.MethodGet, "/api/admin/overview", ""},
		{http.MethodGet, "/api/admin/feeds/health", ""},
		{http.MethodGet, "/api/admin/tenants/5/view", ""},
		{http.MethodPut, "/api/admin/tenants/5/view", `{"center_lat":50,"center_lon":9,"zoom":8}`},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			vs, ff, ft := provisioningFixture()
			fe := &fakeEntitlements{}
			rec := httptest.NewRecorder()
			handlerWithEnt(vs, ff, ft, fe).ServeHTTP(rec, adminReq(rt.method, rt.path, rt.body, 7, store.RoleUser))
			if rec.Code != http.StatusForbidden {
				t.Errorf("user on %s %s = %d, want 403", rt.method, rt.path, rec.Code)
			}
			if vs.grantTenant != 0 || vs.revokeTenant != 0 {
				t.Errorf("user must not reach grant/revoke (grant=%d revoke=%d)", vs.grantTenant, vs.revokeTenant)
			}
			if fe.setTid != 0 {
				t.Errorf("user must not reach entitlement Set (tid=%d)", fe.setTid)
			}
		})
	}
}

// --- feature entitlements (WF2-50) ------------------------------------------

func TestListTenantEntitlementsAdmin(t *testing.T) {
	fe := &fakeEntitlements{eff: map[feature.Key]bool{feature.STCA: true}}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rec := httptest.NewRecorder()
	handlerWithEnt(&fakeVS{}, fakeFeeds{}, ft, fe).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/5/entitlements", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []entitlementDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != len(feature.All()) {
		t.Fatalf("got %d entitlements, want full catalogue (%d)", len(got), len(feature.All()))
	}
	for _, e := range got { // stca enabled, the rest default-denied
		want := e.Key == string(feature.STCA)
		if e.Enabled != want {
			t.Errorf("entitlement %q enabled=%v, want %v", e.Key, e.Enabled, want)
		}
	}
}

func TestSetTenantEntitlementAdmin(t *testing.T) {
	fe := &fakeEntitlements{}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rec := httptest.NewRecorder()
	handlerWithEnt(&fakeVS{}, fakeFeeds{}, ft, fe).
		ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/tenants/5/entitlements/stca", `{"enabled":true}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if fe.setTid != 5 || fe.setKey != feature.STCA || !fe.setVal {
		t.Errorf("Set = (tid=%d key=%q val=%v), want (5, stca, true)", fe.setTid, fe.setKey, fe.setVal)
	}
}

func TestSetTenantEntitlementUnknownKeyIs400(t *testing.T) {
	fe := &fakeEntitlements{setErr: feature.ErrUnknownFeature}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rec := httptest.NewRecorder()
	handlerWithEnt(&fakeVS{}, fakeFeeds{}, ft, fe).
		ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/tenants/5/entitlements/bogus", `{"enabled":true}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSetTenantEntitlementUnknownTenantIs404(t *testing.T) {
	fe := &fakeEntitlements{}
	rec := httptest.NewRecorder()
	handlerWithEnt(&fakeVS{}, fakeFeeds{}, fakeTenants{}, fe).
		ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/tenants/5/entitlements/stca", `{"enabled":true}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if fe.setTid != 0 {
		t.Error("Set must not run for a nonexistent tenant")
	}
}

// --- WF2-41: multi_feed grant gating (the hard invariant) -------------------

func grantSubReq(feedID string) *http.Request {
	return adminReq(http.MethodPost, "/api/admin/tenants/5/subscriptions", `{"feed_id":`+feedID+`}`, 99, store.RoleAdmin)
}

func TestGrantFirstFeedAllowedWithoutMultiFeed(t *testing.T) {
	vs := &fakeVS{} // no existing subscriptions
	ff := fakeFeeds{byID: map[int64]store.Feed{3: {ID: 3}}}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rec := httptest.NewRecorder()
	handlerWithEnt(vs, ff, ft, &fakeEntitlements{}).ServeHTTP(rec, grantSubReq("3"))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("first feed status = %d, want 204", rec.Code)
	}
	if vs.grantFeed != 3 {
		t.Errorf("Subscribe not called for feed 3 (got %d)", vs.grantFeed)
	}
}

func TestGrantSecondFeedDeniedWithoutMultiFeed(t *testing.T) {
	vs := &fakeVS{subsFeeds: []store.Feed{{ID: 3}}} // already holds feed 3
	ff := fakeFeeds{byID: map[int64]store.Feed{7: {ID: 7}}}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rec := httptest.NewRecorder()
	handlerWithEnt(vs, ff, ft, &fakeEntitlements{}).ServeHTTP(rec, grantSubReq("7"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("second feed status = %d, want 409", rec.Code)
	}
	if vs.grantFeed != 0 {
		t.Error("Subscribe must not run when the entitlement gate denies (invalid state never reaches DB)")
	}
}

func TestGrantSecondFeedAllowedWithMultiFeed(t *testing.T) {
	vs := &fakeVS{subsFeeds: []store.Feed{{ID: 3}}}
	ff := fakeFeeds{byID: map[int64]store.Feed{7: {ID: 7}}}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	fe := &fakeEntitlements{has: map[feature.Key]bool{feature.MultiFeed: true}}
	rec := httptest.NewRecorder()
	handlerWithEnt(vs, ff, ft, fe).ServeHTTP(rec, grantSubReq("7"))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("second feed (entitled) status = %d, want 204", rec.Code)
	}
	if vs.grantFeed != 7 {
		t.Errorf("Subscribe not called for feed 7 (got %d)", vs.grantFeed)
	}
}

func TestGrantReGrantSameFeedIsIdempotent(t *testing.T) {
	// Re-granting a feed the tenant already holds must stay 204 even without
	// multi_feed — the feed count does not increase.
	vs := &fakeVS{subsFeeds: []store.Feed{{ID: 3}}}
	ff := fakeFeeds{byID: map[int64]store.Feed{3: {ID: 3}}}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rec := httptest.NewRecorder()
	handlerWithEnt(vs, ff, ft, &fakeEntitlements{}).ServeHTTP(rec, grantSubReq("3"))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("re-grant status = %d, want 204 (idempotent)", rec.Code)
	}
	if vs.grantFeed != 3 {
		t.Errorf("Subscribe not called on idempotent re-grant (got %d)", vs.grantFeed)
	}
}

func TestGetSensorClasses(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerWith(&fakeVS{}, fakeFeeds{}, fakeTenants{}).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/sensor-classes", "", 7, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 6 {
		t.Fatalf("got %d sensor classes, want full catalogue of 6", len(got))
	}
	for _, e := range got {
		if e["class"] == "" || e["description"] == "" {
			t.Errorf("incomplete catalogue entry: %v", e)
		}
	}
}

// --- AP3: tenant-centric admin dashboard ------------------------------------

// handlerForOverview wires the fakes the AP3 overview needs (custom user store
// for the account count). fakeVS serves both views and subscriptions.
func handlerForOverview(vs *fakeVS, ff fakeFeeds, ft fakeTenants, us UserStore, fe EntitlementService) *Handler {
	return New(vs, vs, ff, ft, us, &fakeCredStore{}, fe, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func TestGetOverviewAggregates(t *testing.T) {
	vs := &fakeVS{subsFeeds: []store.Feed{{ID: 3, Name: "Frankfurt"}}}
	ff := fakeFeeds{}
	ft := fakeTenants{list: []store.Tenant{{ID: 5, Slug: "acme", Name: "ACME", Status: "active"}}}
	us := &fakeUserStore{listByTen: map[int64][]store.User{5: {{ID: 1}, {ID: 2}}}}
	fe := &fakeEntitlements{eff: map[feature.Key]bool{feature.STCA: true, feature.MultiFeed: false, feature.Airspaces: true}}

	rec := httptest.NewRecorder()
	handlerForOverview(vs, ff, ft, us, fe).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/overview", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []tenantOverviewDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d overview rows, want 1", len(got))
	}
	row := got[0]
	if row.ID != 5 || row.Slug != "acme" || row.Status != "active" {
		t.Errorf("row identity = %+v, want id=5 slug=acme status=active", row)
	}
	if row.UserCount != 2 {
		t.Errorf("user_count = %d, want 2", row.UserCount)
	}
	if len(row.Feeds) != 1 || row.Feeds[0].ID != 3 {
		t.Errorf("feeds = %+v, want [feed 3]", row.Feeds)
	}
	// Only enabled keys, in stable catalogue order (airspaces < stca).
	if len(row.Features) != 2 || row.Features[0] != "airspaces" || row.Features[1] != "stca" {
		t.Errorf("features = %v, want [airspaces stca]", row.Features)
	}
}

func TestGetTenantViewReadsDefault(t *testing.T) {
	vs := &fakeVS{vc: store.ViewConfig{CenterLat: 50, CenterLon: 9, Zoom: 8}}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rec := httptest.NewRecorder()
	handlerWith(vs, fakeFeeds{}, ft).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/5/view", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got viewDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.CenterLat != 50 || got.CenterLon != 9 || got.Zoom != 8 {
		t.Errorf("view = %+v, want center 50/9 zoom 8", got)
	}
}

func TestGetTenantViewUnknownTenantIs404(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerWith(&fakeVS{}, fakeFeeds{}, fakeTenants{}).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/5/view", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestPutTenantViewUpsertsAndRescopes(t *testing.T) {
	vs := &fakeVS{}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rr := &rescopeRecorder{}
	h := New(vs, vs, fakeFeeds{}, ft, &fakeUserStore{}, &fakeCredStore{}, &fakeEntitlements{}, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), rr.fn)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/tenants/5/view", `{"center_lat":48,"center_lon":11,"zoom":7}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if vs.upsertTenant != 5 {
		t.Errorf("UpsertTenantDefault tenant = %d, want 5", vs.upsertTenant)
	}
	if vs.upserted.CenterLat != 48 || vs.upserted.Zoom != 7 {
		t.Errorf("upserted view = %+v, want center_lat 48 zoom 7", vs.upserted)
	}
	if len(rr.calls) != 1 || rr.calls[0] != 5 {
		t.Errorf("rescope calls = %v, want [5]", rr.calls)
	}
}

func TestPutTenantViewRejectsInvalid(t *testing.T) {
	vs := &fakeVS{}
	ft := fakeTenants{byID: map[int64]store.Tenant{5: {ID: 5}}}
	rec := httptest.NewRecorder()
	handlerWith(vs, fakeFeeds{}, ft).
		ServeHTTP(rec, adminReq(http.MethodPut, "/api/admin/tenants/5/view", `{"center_lat":50,"center_lon":9,"zoom":99}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if vs.upsertTenant != 0 {
		t.Error("invalid view must not reach the store")
	}
}

// --- AP4: feed health endpoint -----------------------------------------------

// fakeFeedHealth is an in-memory FeedHealthSource for testing.
type fakeFeedHealth struct {
	snaps map[int64]health.FeedSnapshot
}

func (f *fakeFeedHealth) Snapshot(feedID int64, _ time.Time) health.FeedSnapshot {
	if f.snaps == nil {
		return health.FeedSnapshot{}
	}
	return f.snaps[feedID]
}

func handlerForHealth(ff fakeFeeds, fh FeedHealthSource) *Handler {
	ft := fakeTenants{}
	return New(&fakeVS{}, &fakeVS{}, ff, ft, &fakeUserStore{}, &fakeCredStore{}, &fakeEntitlements{}, fh, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func TestGetFeedsHealthNilSourceReturnsEmpty(t *testing.T) {
	h := handlerForHealth(fakeFeeds{}, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/feeds/health", "", 1, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []feedHealthDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d items, want 0 (nil source)", len(got))
	}
}

func TestGetFeedsHealthReturnsColorAndFields(t *testing.T) {
	ff := fakeFeeds{list: []store.Feed{{ID: 1, Name: "Berlin"}, {ID: 2, Name: "Vienna"}, {ID: 3, Name: "Paris"}}}
	fh := &fakeFeedHealth{snaps: map[int64]health.FeedSnapshot{
		1: {EverSeen: true, Stale: false, LastHeartbeatAgoS: 0.5, TrackCountRecent: 3},
		// Feed 2: empty sky (no tracks) — green, not yellow (leerer Himmel is not a warning).
		2: {EverSeen: true, Stale: false, LastHeartbeatAgoS: 1.0, TrackCountRecent: 0},
		// Feed 3: sensor fusion degraded (2 of 3 radars active) — yellow.
		3: {EverSeen: true, Stale: false, LastHeartbeatAgoS: 0.8, TrackCountRecent: 4, SensorsActive: 2, SensorsTotal: 3},
	}}
	h := handlerForHealth(ff, fh)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/feeds/health", "", 1, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []feedHealthDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}
	// Feed 1: has tracks → green.
	if got[0].FeedID != 1 || got[0].Color != "green" {
		t.Errorf("feed 1: got id=%d color=%q, want id=1 color=green", got[0].FeedID, got[0].Color)
	}
	// Feed 2: empty sky (heartbeat healthy, 0 tracks) → green (not yellow).
	if got[1].FeedID != 2 || got[1].Color != "green" {
		t.Errorf("feed 2: got id=%d color=%q, want id=2 color=green (empty sky)", got[1].FeedID, got[1].Color)
	}
	// Feed 3: degraded sensor fusion (2/3 radars) → yellow.
	if got[2].FeedID != 3 || got[2].Color != "yellow" {
		t.Errorf("feed 3: got id=%d color=%q, want id=3 color=yellow (degraded)", got[2].FeedID, got[2].Color)
	}
	if got[2].SensorsActive != 2 || got[2].SensorsTotal != 3 {
		t.Errorf("feed 3 sensors: got active=%d total=%d, want active=2 total=3", got[2].SensorsActive, got[2].SensorsTotal)
	}
}

func TestGetFeedsHealthStaleFeedIsRed(t *testing.T) {
	ff := fakeFeeds{list: []store.Feed{{ID: 7, Name: "Stale"}}}
	fh := &fakeFeedHealth{snaps: map[int64]health.FeedSnapshot{
		7: {EverSeen: true, Stale: true, LastHeartbeatAgoS: 10.0, TrackCountRecent: 0},
	}}
	h := handlerForHealth(ff, fh)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/feeds/health", "", 1, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []feedHealthDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].Color != "red" {
		t.Errorf("stale feed: got %+v, want color=red", got)
	}
}

func TestGetFeedsHealthForbidUser(t *testing.T) {
	h := handlerForHealth(fakeFeeds{}, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/feeds/health", "", 1, store.RoleUser))
	if rec.Code != http.StatusForbidden {
		t.Errorf("user on feeds/health = %d, want 403", rec.Code)
	}
}
