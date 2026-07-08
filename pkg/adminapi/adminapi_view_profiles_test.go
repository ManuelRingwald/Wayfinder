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

// --- pure validation --------------------------------------------------------

func TestValidateViewProfile(t *testing.T) {
	// Empty / whitespace name rejected.
	if _, _, err := validateViewProfile("   ", nil); err == nil {
		t.Errorf("empty name accepted")
	}
	// Over-long name rejected.
	if _, _, err := validateViewProfile(strings.Repeat("x", maxViewProfileNameLen+1), nil); err == nil {
		t.Errorf("over-long name accepted")
	}
	// Name is trimmed; nil settings normalise to {}.
	name, s, err := validateViewProfile("  Approach  ", nil)
	if err != nil || name != "Approach" || string(s) != "{}" {
		t.Errorf("normalise = (%q,%q,%v), want (Approach,{},nil)", name, s, err)
	}
	// Non-object settings (array / scalar) rejected.
	for _, bad := range []string{`[1,2,3]`, `42`, `"x"`, `{`} {
		if _, _, err := validateViewProfile("n", json.RawMessage(bad)); err == nil {
			t.Errorf("settings %q accepted, want rejected", bad)
		}
	}
	// Oversize settings rejected.
	big := json.RawMessage(`{"k":"` + strings.Repeat("a", maxViewProfileSettingsBytes) + `"}`)
	if _, _, err := validateViewProfile("n", big); err == nil {
		t.Errorf("oversize settings accepted")
	}
	// A valid object passes through verbatim.
	if _, s, err := validateViewProfile("n", json.RawMessage(`{"rangeRings":true}`)); err != nil || string(s) != `{"rangeRings":true}` {
		t.Errorf("valid object = (%q,%v)", s, err)
	}
}

func TestToViewProfileDTO_NilSettings(t *testing.T) {
	dto := toViewProfileDTO(store.ViewProfile{ID: 3, Name: "x"})
	if string(dto.Settings) != "{}" {
		t.Errorf("nil settings -> %q, want {}", dto.Settings)
	}
}

// --- handler wiring ---------------------------------------------------------

// fakeViewProfiles records the scoped arguments so tests can assert the handler
// always uses the SESSION user id (never the request) and returns canned rows.
type fakeViewProfiles struct {
	list        []store.ViewProfile
	createErr   error
	updateErr   error
	deleteErr   error
	setDefErr   error
	gotUserID   int64
	gotID       int64
	gotName     string
	gotSettings json.RawMessage
	gotDefault  bool
}

func (f *fakeViewProfiles) ListByUser(_ context.Context, userID int64) ([]store.ViewProfile, error) {
	f.gotUserID = userID
	return f.list, nil
}
func (f *fakeViewProfiles) Create(_ context.Context, userID int64, name string, settings json.RawMessage, makeDefault bool) (store.ViewProfile, error) {
	f.gotUserID, f.gotName, f.gotSettings, f.gotDefault = userID, name, settings, makeDefault
	if f.createErr != nil {
		return store.ViewProfile{}, f.createErr
	}
	return store.ViewProfile{ID: 1, UserID: userID, Name: name, Settings: settings, IsDefault: makeDefault}, nil
}
func (f *fakeViewProfiles) Update(_ context.Context, userID, id int64, name string, settings json.RawMessage) (store.ViewProfile, error) {
	f.gotUserID, f.gotID, f.gotName, f.gotSettings = userID, id, name, settings
	if f.updateErr != nil {
		return store.ViewProfile{}, f.updateErr
	}
	return store.ViewProfile{ID: id, UserID: userID, Name: name, Settings: settings}, nil
}
func (f *fakeViewProfiles) Delete(_ context.Context, userID, id int64) error {
	f.gotUserID, f.gotID = userID, id
	return f.deleteErr
}
func (f *fakeViewProfiles) SetDefault(_ context.Context, userID, id int64) (store.ViewProfile, error) {
	f.gotUserID, f.gotID = userID, id
	if f.setDefErr != nil {
		return store.ViewProfile{}, f.setDefErr
	}
	return store.ViewProfile{ID: id, UserID: userID, IsDefault: true}, nil
}

const vpTestUserID = 42

func handlerForProfiles(p ViewProfileStore) http.Handler {
	h := New(&fakeVS{}, &fakeVS{}, fakeFeeds{}, fakeTenants{}, &fakeUserStore{}, &fakeCredStore{}, &fakeEntitlements{},
		nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	if p != nil {
		h = h.WithViewProfiles(p)
	}
	return h.ViewProfilesHandler()
}

// vpReq builds a view-profile request carrying a normal (non-admin) user Identity.
func vpReq(method, path, body string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	return r.WithContext(tenant.WithIdentity(r.Context(),
		tenant.Identity{TenantID: 7, UserID: vpTestUserID, Subject: "alice", Role: store.RoleUser}))
}

func TestViewProfilesList(t *testing.T) {
	f := &fakeViewProfiles{list: []store.ViewProfile{
		{ID: 1, UserID: vpTestUserID, Name: "Approach", Settings: json.RawMessage(`{"rangeRings":true}`), IsDefault: true},
	}}
	rec := httptest.NewRecorder()
	handlerForProfiles(f).ServeHTTP(rec, vpReq(http.MethodGet, "/api/view-profiles", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if f.gotUserID != vpTestUserID {
		t.Errorf("scoped to user %d, want %d", f.gotUserID, vpTestUserID)
	}
	var out []viewProfileDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out) != 1 || out[0].Name != "Approach" {
		t.Errorf("body = %s (err %v)", rec.Body.String(), err)
	}
}

func TestViewProfilesListEmptyIsArray(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerForProfiles(&fakeViewProfiles{}).ServeHTTP(rec, vpReq(http.MethodGet, "/api/view-profiles", ""))
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("empty list body = %q, want []", rec.Body.String())
	}
}

func TestViewProfilesCreate(t *testing.T) {
	f := &fakeViewProfiles{}
	rec := httptest.NewRecorder()
	handlerForProfiles(f).ServeHTTP(rec, vpReq(http.MethodPost, "/api/view-profiles",
		`{"name":"  Tower  ","settings":{"historyDots":false},"make_default":true}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	if f.gotUserID != vpTestUserID {
		t.Errorf("create scoped to user %d, want %d", f.gotUserID, vpTestUserID)
	}
	if f.gotName != "Tower" {
		t.Errorf("name = %q, want trimmed Tower", f.gotName)
	}
	if !f.gotDefault {
		t.Errorf("make_default not honoured")
	}
}

func TestViewProfilesCreateValidation(t *testing.T) {
	cases := []struct {
		body string
		want int
	}{
		{`{"name":"","settings":{}}`, http.StatusUnprocessableEntity},
		{`{"name":"ok","settings":[1,2]}`, http.StatusUnprocessableEntity},
		{`not json`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		handlerForProfiles(&fakeViewProfiles{}).ServeHTTP(rec, vpReq(http.MethodPost, "/api/view-profiles", tc.body))
		if rec.Code != tc.want {
			t.Errorf("body %q -> %d, want %d", tc.body, rec.Code, tc.want)
		}
	}
}

func TestViewProfilesCreateLimit(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerForProfiles(&fakeViewProfiles{createErr: store.ErrProfileLimit}).
		ServeHTTP(rec, vpReq(http.MethodPost, "/api/view-profiles", `{"name":"x","settings":{}}`))
	if rec.Code != http.StatusConflict {
		t.Errorf("limit -> %d, want 409", rec.Code)
	}
}

func TestViewProfilesUpdateAndDefaultUsePathAndSession(t *testing.T) {
	f := &fakeViewProfiles{}
	rec := httptest.NewRecorder()
	handlerForProfiles(f).ServeHTTP(rec, vpReq(http.MethodPut, "/api/view-profiles/5", `{"name":"n","settings":{}}`))
	if rec.Code != http.StatusOK || f.gotID != 5 || f.gotUserID != vpTestUserID {
		t.Errorf("update: code %d, id %d, user %d", rec.Code, f.gotID, f.gotUserID)
	}
	rec = httptest.NewRecorder()
	handlerForProfiles(f).ServeHTTP(rec, vpReq(http.MethodPost, "/api/view-profiles/9/default", ""))
	if rec.Code != http.StatusOK || f.gotID != 9 {
		t.Errorf("setdefault: code %d, id %d", rec.Code, f.gotID)
	}
}

func TestViewProfilesNotFound(t *testing.T) {
	// A foreign id surfaces from the store as ErrNotFound and must map to 404.
	rec := httptest.NewRecorder()
	handlerForProfiles(&fakeViewProfiles{updateErr: store.ErrNotFound}).
		ServeHTTP(rec, vpReq(http.MethodPut, "/api/view-profiles/5", `{"name":"n","settings":{}}`))
	if rec.Code != http.StatusNotFound {
		t.Errorf("update foreign -> %d, want 404", rec.Code)
	}
	rec = httptest.NewRecorder()
	handlerForProfiles(&fakeViewProfiles{deleteErr: store.ErrNotFound}).
		ServeHTTP(rec, vpReq(http.MethodDelete, "/api/view-profiles/5", ""))
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete foreign -> %d, want 404", rec.Code)
	}
}

func TestViewProfilesDelete(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerForProfiles(&fakeViewProfiles{}).ServeHTTP(rec, vpReq(http.MethodDelete, "/api/view-profiles/5", ""))
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete -> %d, want 204", rec.Code)
	}
}

func TestViewProfilesUnauthorized(t *testing.T) {
	// No Identity in context → 401 (the tenant middleware normally sets it).
	req := httptest.NewRequest(http.MethodGet, "/api/view-profiles", nil)
	rec := httptest.NewRecorder()
	handlerForProfiles(&fakeViewProfiles{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no identity -> %d, want 401", rec.Code)
	}
}

func TestViewProfilesDisabledWhenNil(t *testing.T) {
	// Store not wired (nil) → 404, feature unavailable.
	rec := httptest.NewRecorder()
	handlerForProfiles(nil).ServeHTTP(rec, vpReq(http.MethodGet, "/api/view-profiles", ""))
	if rec.Code != http.StatusNotFound {
		t.Errorf("nil store -> %d, want 404", rec.Code)
	}
}
