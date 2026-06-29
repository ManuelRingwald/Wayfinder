package adminapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/sensorclass"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// fakeLifecycle records the live join/leave calls the feed handlers make, and can
// fail Start to exercise the rollback path (ONB-5).
type fakeLifecycle struct {
	started  []int64
	stopped  []int64
	startErr error
}

func (l *fakeLifecycle) Start(id int64, _ string, _ string, _ int) error {
	if l.startErr != nil {
		return l.startErr
	}
	l.started = append(l.started, id)
	return nil
}

func (l *fakeLifecycle) Stop(id int64) bool {
	l.stopped = append(l.stopped, id)
	return true
}

// handlerForFeeds builds a handler wired with the given feed store and lifecycle
// for the ONB-5 feed-lifecycle tests.
func handlerForFeeds(ff fakeFeeds, life FeedLifecycle) *Handler {
	return New(&fakeVS{}, &fakeVS{}, ff, fakeTenants{}, &fakeUserStore{}, &fakeCredStore{},
		&fakeEntitlements{}, nil, life, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func TestCreateFeed(t *testing.T) {
	ff := fakeFeeds{byName: map[string]store.Feed{}, created: map[string]store.Feed{}, nextID: 5}
	life := &fakeLifecycle{}
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, life).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/feeds",
		`{"name":"north","multicast_group":"239.255.0.70","port":8600,"sensor_mix":["PSR","SSR"]}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var got feedDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "north" || got.MulticastGroup != "239.255.0.70" || got.Port != 8600 {
		t.Fatalf("created feed = %+v", got)
	}
	if _, ok := ff.created["north"]; !ok {
		t.Error("Create was not called with the name")
	}
	// The new feed must have been joined live.
	if len(life.started) != 1 || life.started[0] != got.ID {
		t.Errorf("live Start = %v, want [%d]", life.started, got.ID)
	}
}

func TestCreateFeedValidation(t *testing.T) {
	cases := map[string]struct {
		body string
		want int
	}{
		"missing name":      {`{"multicast_group":"239.255.0.70","port":8600}`, http.StatusBadRequest},
		"blank name":        {`{"name":"  ","multicast_group":"239.255.0.70","port":8600}`, http.StatusBadRequest},
		"missing group":     {`{"name":"x","port":8600}`, http.StatusBadRequest},
		"non-multicast IP":  {`{"name":"x","multicast_group":"10.0.0.1","port":8600}`, http.StatusBadRequest},
		"not an IP":         {`{"name":"x","multicast_group":"not-an-ip","port":8600}`, http.StatusBadRequest},
		"ipv6 group":        {`{"name":"x","multicast_group":"ff02::1","port":8600}`, http.StatusBadRequest},
		"port zero":         {`{"name":"x","multicast_group":"239.255.0.70","port":0}`, http.StatusBadRequest},
		"port out of range": {`{"name":"x","multicast_group":"239.255.0.70","port":70000}`, http.StatusBadRequest},
		"bad json":          {`not-json`, http.StatusBadRequest},
	}
	for name, tc := range cases {
		ff := fakeFeeds{byName: map[string]store.Feed{}, created: map[string]store.Feed{}}
		life := &fakeLifecycle{}
		rec := httptest.NewRecorder()
		handlerForFeeds(ff, life).ServeHTTP(rec,
			adminReq(http.MethodPost, "/api/admin/feeds", tc.body, 99, store.RoleAdmin))
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
		if len(ff.created) != 0 {
			t.Errorf("%s: an invalid feed reached the store", name)
		}
		if len(life.started) != 0 {
			t.Errorf("%s: an invalid feed must not be joined live", name)
		}
	}
}

func TestCreateFeedDuplicateName(t *testing.T) {
	ff := fakeFeeds{
		byName:  map[string]store.Feed{"north": {ID: 1, Name: "north"}},
		created: map[string]store.Feed{},
	}
	life := &fakeLifecycle{}
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, life).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/feeds",
		`{"name":"north","multicast_group":"239.255.0.70","port":8600}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if len(ff.created) != 0 {
		t.Error("duplicate name must not reach Create")
	}
	if len(life.started) != 0 {
		t.Error("duplicate name must not be joined live")
	}
}

// TestCreateFeedInvalidSensorMix maps the store's unknown-class rejection to 400.
func TestCreateFeedInvalidSensorMix(t *testing.T) {
	ff := fakeFeeds{
		byName:    map[string]store.Feed{},
		created:   map[string]store.Feed{},
		createErr: &sensorclass.UnknownClassError{Token: "bogus"},
	}
	life := &fakeLifecycle{}
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, life).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/feeds",
		`{"name":"north","multicast_group":"239.255.0.70","port":8600,"sensor_mix":["bogus"]}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(life.started) != 0 {
		t.Error("a feed that failed to create must not be joined live")
	}
}

// ORCH-4: omitting both group and port has the server auto-allocate a
// collision-free endpoint, which is returned and joined live.
func TestCreateFeedAutoAllocatesEndpoint(t *testing.T) {
	ff := fakeFeeds{byName: map[string]store.Feed{}, created: map[string]store.Feed{}, nextID: 5}
	life := &fakeLifecycle{}
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, life).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/feeds",
		`{"name":"auto","sensor_mix":["PSR"]}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var got feedDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.MulticastGroup == "" || got.Port == 0 {
		t.Fatalf("auto-allocated feed missing endpoint: %+v", got)
	}
	if len(life.started) != 1 {
		t.Errorf("auto-allocated feed should be joined live, got %v", life.started)
	}
}

// ORCH-4: supplying only one of group/port is a client error (provide both or
// neither).
func TestCreateFeedPartialEndpointRejected(t *testing.T) {
	for _, body := range []string{
		`{"name":"x","multicast_group":"239.255.0.70"}`,
		`{"name":"x","port":8600}`,
	} {
		ff := fakeFeeds{byName: map[string]store.Feed{}, created: map[string]store.Feed{}}
		rec := httptest.NewRecorder()
		handlerForFeeds(ff, &fakeLifecycle{}).ServeHTTP(rec,
			adminReq(http.MethodPost, "/api/admin/feeds", body, 99, store.RoleAdmin))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, rec.Code)
		}
		if len(ff.created) != 0 {
			t.Errorf("body %s: must not reach the store", body)
		}
	}
}

// ORCH-4: a manual endpoint that collides surfaces the store's ErrEndpointTaken
// as 409.
func TestCreateFeedEndpointTakenIs409(t *testing.T) {
	ff := fakeFeeds{byName: map[string]store.Feed{}, created: map[string]store.Feed{}, createErr: store.ErrEndpointTaken}
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, &fakeLifecycle{}).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/feeds",
		`{"name":"dup","multicast_group":"239.255.0.62","port":8600}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

// ORCH-4: an exhausted pool on the auto path surfaces as 507.
func TestCreateFeedPoolExhaustedIs507(t *testing.T) {
	ff := fakeFeeds{byName: map[string]store.Feed{}, created: map[string]store.Feed{}, createErr: store.ErrPoolExhausted}
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, &fakeLifecycle{}).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/feeds",
		`{"name":"nofree"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, want 507", rec.Code)
	}
}

// TestCreateFeedRollsBackOnJoinFailure verifies the catalogue row is deleted when
// the live multicast join fails, so a feed is never left catalogued-but-silent.
func TestCreateFeedRollsBackOnJoinFailure(t *testing.T) {
	ff := fakeFeeds{
		byName:  map[string]store.Feed{},
		created: map[string]store.Feed{},
		deleted: map[int64]bool{},
		nextID:  7,
	}
	life := &fakeLifecycle{startErr: errors.New("address already in use")}
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, life).ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/feeds",
		`{"name":"north","multicast_group":"239.255.0.70","port":8600}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (join failed)", rec.Code)
	}
	if !ff.deleted[7] {
		t.Error("a feed whose live join failed must be rolled back (deleted)")
	}
}

func TestDeleteFeedSucceeds(t *testing.T) {
	ff := fakeFeeds{
		byID:    map[int64]store.Feed{3: {ID: 3, Name: "north"}},
		deleted: map[int64]bool{},
	}
	life := &fakeLifecycle{}
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, life).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/feeds/3", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if !ff.deleted[3] {
		t.Error("feed should have been deleted")
	}
	// The receiver must have been stopped (multicast leave) before the row went.
	if len(life.stopped) != 1 || life.stopped[0] != 3 {
		t.Errorf("live Stop = %v, want [3]", life.stopped)
	}
}

func TestDeleteFeedUnknownIs404(t *testing.T) {
	ff := fakeFeeds{byID: map[int64]store.Feed{}, deleted: map[int64]bool{}}
	life := &fakeLifecycle{}
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, life).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/feeds/9", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if len(life.stopped) != 0 {
		t.Error("an unknown feed must not trigger a live Stop")
	}
}

// TestFeedLifecycleRoutesForbidNonAdmin verifies both routes are gated to admins
// (requireAdmin → 403 for a non-admin identity).
func TestFeedLifecycleRoutesForbidNonAdmin(t *testing.T) {
	ff := fakeFeeds{byID: map[int64]store.Feed{3: {ID: 3}}, byName: map[string]store.Feed{}}
	life := &fakeLifecycle{}
	h := handlerForFeeds(ff, life)
	reqs := []*http.Request{
		adminReq(http.MethodPost, "/api/admin/feeds", `{"name":"x","multicast_group":"239.255.0.70","port":8600}`, 99, store.RoleUser),
		adminReq(http.MethodDelete, "/api/admin/feeds/3", "", 99, store.RoleUser),
	}
	for _, req := range reqs {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", req.Method, req.URL.Path, rec.Code)
		}
	}
	if len(life.started) != 0 || len(life.stopped) != 0 {
		t.Error("a forbidden request must not touch the live receiver set")
	}
}

// TestCreateFeedWithoutLifecycle verifies a nil lifecycle (single-tenant / no
// live-apply) still catalogues the feed and returns 201 — it simply does not
// join live (the receiver set follows on restart). Context guard kept for parity.
func TestCreateFeedWithoutLifecycle(t *testing.T) {
	ff := fakeFeeds{byName: map[string]store.Feed{}, created: map[string]store.Feed{}, nextID: 1}
	h := New(&fakeVS{}, &fakeVS{}, ff, fakeTenants{}, &fakeUserStore{}, &fakeCredStore{},
		&fakeEntitlements{}, nil, nil /* no feed lifecycle */, nil /* no aero lifecycle */, nil /* no secret service */, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/feeds",
		`{"name":"north","multicast_group":"239.255.0.70","port":8600}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := ff.created["north"]; !ok {
		t.Error("feed should still be catalogued without a lifecycle")
	}
}

// --- ORCH-1b: feed source configuration -------------------------------------

// feedSourcesFixture builds a feed store with one feed (id 3) ready for source
// config round-trips.
func feedSourcesFixture() fakeFeeds {
	return fakeFeeds{
		byID:      map[int64]store.Feed{3: {ID: 3, Name: "speyer"}},
		sourceCfg: map[int64]store.SourceConfig{},
		coverage:  map[int64]*store.BBox{},
	}
}

func TestGetFeedSourcesDefaultsEmpty(t *testing.T) {
	ff := feedSourcesFixture()
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, nil).ServeHTTP(rec,
		adminReq(http.MethodGet, "/api/admin/feeds/3/sources", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got feedSourcesDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Sources == nil {
		t.Error("sources should serialise as [] not null")
	}
	if len(got.Sources) != 0 || got.CoverageBBox != nil {
		t.Fatalf("default = %+v, want empty/nil", got)
	}
}

func TestGetFeedSourcesUnknownIs404(t *testing.T) {
	ff := feedSourcesFixture()
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, nil).ServeHTTP(rec,
		adminReq(http.MethodGet, "/api/admin/feeds/9/sources", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestPutFeedSourcesRoundTripAndDerivesCoverage(t *testing.T) {
	ff := feedSourcesFixture()
	body := `{"sources":[
		{"type":"adsb_opensky","bbox":{"min_lat":48,"min_lon":7,"max_lat":50,"max_lon":9},"cred_ref":"secret/speyer"},
		{"type":"radar_asterix","sac":1,"sic":4}
	]}`
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, nil).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/feeds/3/sources", body, 99, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got feedSourcesDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Sources) != 2 || got.Sources[0].Type != store.SourceADSBOpenSky {
		t.Fatalf("sources = %+v", got.Sources)
	}
	// Coverage was not supplied, so the server derives it from the source bbox
	// (+ default margin): it must contain the source bbox and be wider.
	c := got.CoverageBBox
	if c == nil {
		t.Fatal("coverage should have been derived")
	}
	if c.MinLat >= 48 || c.MaxLat <= 50 || c.MinLon >= 7 || c.MaxLon <= 9 {
		t.Errorf("derived coverage %+v does not pad the source bbox", *c)
	}
	// The store recorded the config and coverage.
	if len(ff.sourceCfg[3]) != 2 || ff.coverage[3] == nil {
		t.Errorf("store state = %+v / %+v", ff.sourceCfg[3], ff.coverage[3])
	}
}

func TestPutFeedSourcesExplicitCoverageOverrides(t *testing.T) {
	ff := feedSourcesFixture()
	body := `{"sources":[{"type":"adsb_opensky","bbox":{"min_lat":48,"min_lon":7,"max_lat":50,"max_lon":9}}],
		"coverage_bbox":{"min_lat":40,"min_lon":0,"max_lat":55,"max_lon":15}}`
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, nil).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/feeds/3/sources", body, 99, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got feedSourcesDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	want := store.BBox{MinLat: 40, MinLon: 0, MaxLat: 55, MaxLon: 15}
	if got.CoverageBBox == nil || *got.CoverageBBox != want {
		t.Fatalf("coverage = %+v, want explicit %+v", got.CoverageBBox, want)
	}
}

func TestPutFeedSourcesInvalidIsRejected(t *testing.T) {
	cases := map[string]string{
		"adsb without bbox":  `{"sources":[{"type":"adsb_opensky"}]}`,
		"unknown type":       `{"sources":[{"type":"satellite"}]}`,
		"radar without sic":  `{"sources":[{"type":"radar_asterix","sac":1}]}`,
		"bad coverage range": `{"sources":[],"coverage_bbox":{"min_lat":-100,"min_lon":0,"max_lat":10,"max_lon":10}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			ff := feedSourcesFixture()
			rec := httptest.NewRecorder()
			handlerForFeeds(ff, nil).ServeHTTP(rec,
				adminReq(http.MethodPut, "/api/admin/feeds/3/sources", body, 99, store.RoleAdmin))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			// A rejected request must not write to the store.
			if len(ff.sourceCfg[3]) != 0 {
				t.Errorf("rejected config must not be stored: %+v", ff.sourceCfg[3])
			}
		})
	}
}

func TestPutFeedSourcesUnknownIs404(t *testing.T) {
	ff := feedSourcesFixture()
	rec := httptest.NewRecorder()
	handlerForFeeds(ff, nil).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/feeds/9/sources",
			`{"sources":[{"type":"radar_asterix","sac":1,"sic":4}]}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

// TestFeedSourcesRoutesForbidNonAdmin verifies both source routes are admin-gated.
func TestFeedSourcesRoutesForbidNonAdmin(t *testing.T) {
	ff := feedSourcesFixture()
	h := handlerForFeeds(ff, nil)
	reqs := []*http.Request{
		adminReq(http.MethodGet, "/api/admin/feeds/3/sources", "", 99, store.RoleUser),
		adminReq(http.MethodPut, "/api/admin/feeds/3/sources",
			`{"sources":[{"type":"radar_asterix","sac":1,"sic":4}]}`, 99, store.RoleUser),
	}
	for _, req := range reqs {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", req.Method, req.URL.Path, rec.Code)
		}
	}
	if len(ff.sourceCfg[3]) != 0 {
		t.Error("a forbidden request must not write source config")
	}
}
