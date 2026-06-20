package tenant

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

type fakeAuth struct {
	subject string
	err     error
}

func (f fakeAuth) Authenticate(*http.Request) (string, error) { return f.subject, f.err }

type fakeUsers struct {
	bySubject map[string]store.User
	err       error
}

func (f fakeUsers) GetBySubject(_ context.Context, subject string) (store.User, error) {
	if f.err != nil {
		return store.User{}, f.err
	}
	u, ok := f.bySubject[subject]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

func TestContextRoundTrip(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("empty context should carry no identity")
	}
	id := Identity{TenantID: 7, UserID: 3, Subject: "s", Role: store.RoleOperator}
	got, ok := FromContext(WithIdentity(context.Background(), id))
	if !ok || got != id {
		t.Fatalf("round-trip = %+v, %v", got, ok)
	}
}

func TestMiddlewareSuccess(t *testing.T) {
	users := fakeUsers{bySubject: map[string]store.User{
		"oidc|alice": {ID: 3, TenantID: 7, Subject: "oidc|alice", Role: store.RoleTenantAdmin},
	}}
	var seen Identity
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		seen, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	h := Middleware(fakeAuth{subject: "oidc|alice"}, users, nil)(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("called=%v status=%d", called, rec.Code)
	}
	if seen.TenantID != 7 || seen.UserID != 3 || seen.Role != store.RoleTenantAdmin || seen.Subject != "oidc|alice" {
		t.Fatalf("identity in context = %+v", seen)
	}
}

func TestMiddlewareFailClosed(t *testing.T) {
	users := fakeUsers{bySubject: map[string]store.User{
		"known": {ID: 1, TenantID: 1, Subject: "known", Role: store.RoleOperator},
	}}

	cases := map[string]struct {
		authn auth.Authenticator
		users UserLookup
	}{
		"auth fails":      {fakeAuth{err: auth.ErrUnauthenticated}, users},
		"unknown subject": {fakeAuth{subject: "stranger"}, users},
		"lookup db error": {fakeAuth{subject: "known"}, fakeUsers{err: errors.New("db down")}},
	}
	for name, tc := range cases {
		called := false
		next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
		h := Middleware(tc.authn, tc.users, nil)(next)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

		if called {
			t.Errorf("%s: next was called — must be fail-closed", name)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, rec.Code)
		}
	}
}
