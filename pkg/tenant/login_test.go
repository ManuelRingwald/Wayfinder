package tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

type fakeCreds struct{ byUser map[int64]string }

func (f fakeCreds) GetHash(_ context.Context, userID int64) (string, error) {
	h, ok := f.byUser[userID]
	if !ok {
		return "", store.ErrNotFound
	}
	return h, nil
}

type fakeTenants struct {
	byID map[int64]store.Tenant
	err  error
}

func (f fakeTenants) GetByID(_ context.Context, id int64) (store.Tenant, error) {
	if f.err != nil {
		return store.Tenant{}, f.err
	}
	t, ok := f.byID[id]
	if !ok {
		return store.Tenant{}, store.ErrNotFound
	}
	return t, nil
}

var loginKey = []byte("login-test-key")

func postLogin(t *testing.T, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body)))
	return rec
}

func loginFixture(t *testing.T) (UserLookup, CredentialLookup, TenantLookup) {
	t.Helper()
	hash, err := auth.HashPassword("s3cret")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	users := fakeUsers{bySubject: map[string]store.User{
		"bob": {ID: 5, TenantID: 1, Subject: "bob", Role: store.RoleUser, Status: store.StatusActive},
	}}
	creds := fakeCreds{byUser: map[int64]string{5: hash}}
	tenants := fakeTenants{byID: map[int64]store.Tenant{
		1: {ID: 1, Slug: "acme", Name: "ACME", Status: store.StatusActive},
	}}
	return users, creds, tenants
}

func TestLoginSuccessSetsValidCookie(t *testing.T) {
	users, creds, tenants := loginFixture(t)
	h := LoginHandler(users, creds, tenants, LoginConfig{SessionKey: loginKey})

	rec := postLogin(t, h, `{"subject":"bob","password":"s3cret"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "wf_session" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie set")
	}
	if !cookie.HttpOnly {
		t.Error("session cookie should be HttpOnly")
	}
	subject, err := auth.ParseSession(cookie.Value, loginKey)
	if err != nil || subject != "bob" {
		t.Fatalf("cookie session = %q, %v", subject, err)
	}
}

func TestLoginFailures(t *testing.T) {
	users, creds, tenants := loginFixture(t)
	noCred := fakeCreds{byUser: map[int64]string{}} // user exists, no credential
	h := func(c CredentialLookup) http.HandlerFunc {
		return LoginHandler(users, c, tenants, LoginConfig{SessionKey: loginKey})
	}

	cases := map[string]struct {
		creds CredentialLookup
		body  string
		want  int
	}{
		"wrong password": {creds, `{"subject":"bob","password":"nope"}`, http.StatusUnauthorized},
		"unknown user":   {creds, `{"subject":"ghost","password":"s3cret"}`, http.StatusUnauthorized},
		"no credential":  {noCred, `{"subject":"bob","password":"s3cret"}`, http.StatusUnauthorized},
		"empty subject":  {creds, `{"subject":"","password":"x"}`, http.StatusBadRequest},
		"bad json":       {creds, `not-json`, http.StatusBadRequest},
	}
	for name, tc := range cases {
		rec := postLogin(t, h(tc.creds), tc.body)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
		if rec.Code != http.StatusNoContent && len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s: a cookie was set on failure", name)
		}
	}
}

// TestLoginEnforcesStatus covers the AP6 pause cascade: a paused account, or an
// account under a paused tenant, is denied login even with correct credentials,
// with the same generic 401 (no paused/active enumeration). A tenant lookup
// error is treated fail-closed as suspended.
func TestLoginEnforcesStatus(t *testing.T) {
	hash, err := auth.HashPassword("s3cret")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	creds := fakeCreds{byUser: map[int64]string{5: hash}}
	activeTenant := fakeTenants{byID: map[int64]store.Tenant{
		1: {ID: 1, Status: store.StatusActive},
	}}
	pausedTenant := fakeTenants{byID: map[int64]store.Tenant{
		1: {ID: 1, Status: store.StatusPaused},
	}}
	usersWith := func(status store.Status) fakeUsers {
		return fakeUsers{bySubject: map[string]store.User{
			"bob": {ID: 5, TenantID: 1, Subject: "bob", Role: store.RoleUser, Status: status},
		}}
	}
	// A platform admin is tenant-less (TenantID 0, ONB-3): the tenant-pause cascade
	// is skipped entirely, so a failing tenant lookup must not lock the admin out.
	adminUser := fakeUsers{bySubject: map[string]store.User{
		"bob": {ID: 5, TenantID: 0, Subject: "bob", Role: store.RoleAdmin, Status: store.StatusActive},
	}}

	cases := map[string]struct {
		users   fakeUsers
		tenants TenantLookup
		want    int
	}{
		"active account, active tenant":   {usersWith(store.StatusActive), activeTenant, http.StatusNoContent},
		"paused account":                  {usersWith(store.StatusPaused), activeTenant, http.StatusUnauthorized},
		"paused tenant cascades":          {usersWith(store.StatusActive), pausedTenant, http.StatusUnauthorized},
		"tenant lookup error denies":      {usersWith(store.StatusActive), fakeTenants{err: store.ErrNotFound}, http.StatusUnauthorized},
		"nil tenants skips cascade":       {usersWith(store.StatusActive), nil, http.StatusNoContent},
		"tenantless admin not locked out": {adminUser, fakeTenants{err: store.ErrNotFound}, http.StatusNoContent},
		"paused admin still denied":       {fakeUsers{bySubject: map[string]store.User{"bob": {ID: 5, TenantID: 0, Subject: "bob", Role: store.RoleAdmin, Status: store.StatusPaused}}}, activeTenant, http.StatusUnauthorized},
	}
	for name, tc := range cases {
		h := LoginHandler(tc.users, creds, tc.tenants, LoginConfig{SessionKey: loginKey})
		rec := postLogin(t, h, `{"subject":"bob","password":"s3cret"}`)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
		gotCookie := len(rec.Result().Cookies()) != 0
		if (rec.Code == http.StatusNoContent) != gotCookie {
			t.Errorf("%s: cookie set = %v, want %v", name, gotCookie, rec.Code == http.StatusNoContent)
		}
	}
}

func TestLoginRejectsNonPost(t *testing.T) {
	users, creds, tenants := loginFixture(t)
	h := LoginHandler(users, creds, tenants, LoginConfig{SessionKey: loginKey})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/login", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", rec.Code)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	LogoutHandler(LoginConfig{SessionKey: loginKey})(rec, httptest.NewRequest(http.MethodPost, "/api/logout", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout did not clear the cookie: %+v", cookies)
	}
}
