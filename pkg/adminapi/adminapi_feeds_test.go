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
		&fakeEntitlements{}, nil, life, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
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
		&fakeEntitlements{}, nil, nil /* no feed lifecycle */, nil /* no aero lifecycle */, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
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
