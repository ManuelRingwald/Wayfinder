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

var loginKey = []byte("login-test-key")

func postLogin(t *testing.T, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body)))
	return rec
}

func loginFixture(t *testing.T) (UserLookup, CredentialLookup) {
	t.Helper()
	hash, err := auth.HashPassword("s3cret")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	users := fakeUsers{bySubject: map[string]store.User{
		"bob": {ID: 5, TenantID: 1, Subject: "bob", Role: store.RoleUser},
	}}
	creds := fakeCreds{byUser: map[int64]string{5: hash}}
	return users, creds
}

func TestLoginSuccessSetsValidCookie(t *testing.T) {
	users, creds := loginFixture(t)
	h := LoginHandler(users, creds, LoginConfig{SessionKey: loginKey})

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
	users, creds := loginFixture(t)
	noCred := fakeCreds{byUser: map[int64]string{}} // user exists, no credential
	h := func(c CredentialLookup) http.HandlerFunc {
		return LoginHandler(users, c, LoginConfig{SessionKey: loginKey})
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

func TestLoginRejectsNonPost(t *testing.T) {
	users, creds := loginFixture(t)
	h := LoginHandler(users, creds, LoginConfig{SessionKey: loginKey})
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
