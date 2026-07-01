package tenant

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// fakeSessions is a recording SessionStore for the registry-mode handler tests.
type createCall struct {
	userID             int64
	createdAt, expires time.Time
	limit              int
	policy             store.SessionLimitPolicy
	meta               store.SessionMeta
}

type extendCall struct {
	token   string
	ttl     time.Duration
	maxLife time.Duration
}

type fakeSessions struct {
	createErr  error
	extendErr  error
	extendExp  time.Time
	lastCreate *createCall
	lastExtend *extendCall
	deleted    []string
	seq        int
}

func (f *fakeSessions) CreateSession(_ context.Context, userID int64, createdAt, expires time.Time, limit int, policy store.SessionLimitPolicy, meta store.SessionMeta) (string, error) {
	f.lastCreate = &createCall{userID, createdAt, expires, limit, policy, meta}
	if f.createErr != nil {
		return "", f.createErr
	}
	f.seq++
	return fmt.Sprintf("tok-%d", f.seq), nil
}

func (f *fakeSessions) ExtendSession(_ context.Context, token string, ttl, maxLife time.Duration) (time.Time, error) {
	f.lastExtend = &extendCall{token, ttl, maxLife}
	if f.extendErr != nil {
		return time.Time{}, f.extendErr
	}
	if f.extendExp.IsZero() {
		return time.Now().Add(time.Hour), nil
	}
	return f.extendExp, nil
}

func (f *fakeSessions) DeleteSession(_ context.Context, token string) error {
	f.deleted = append(f.deleted, token)
	return nil
}

// TestLoginOpensRegistrySession: a successful login with a registry store opens a
// session and hands out a session-id cookie (not a stateless one), passing the
// effective limit and default policy.
func TestLoginOpensRegistrySession(t *testing.T) {
	users, creds, tenants := loginFixture(t)
	fs := &fakeSessions{}
	cfg := LoginConfig{SessionKey: loginKey, Sessions: fs, SessionLimitDefault: 3}
	rec := postLogin(t, LoginHandler(users, creds, tenants, cfg), `{"subject":"bob","password":"s3cret"}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	cookie := sessionCookie(t, rec)
	tok, err := auth.ParseSessionID(cookie.Value, loginKey)
	if err != nil {
		t.Fatalf("cookie is not a session-id cookie: %v", err)
	}
	if tok != "tok-1" {
		t.Fatalf("cookie token = %q, want tok-1", tok)
	}
	if fs.lastCreate == nil || fs.lastCreate.userID != 5 {
		t.Fatalf("CreateSession userID = %+v, want 5", fs.lastCreate)
	}
	if fs.lastCreate.limit != 3 || fs.lastCreate.policy != store.SessionLimitReject {
		t.Fatalf("limit/policy = %d/%q, want 3/reject", fs.lastCreate.limit, fs.lastCreate.policy)
	}
}

// TestLoginPerAccessLimitOverride: a per-access session_limit overrides the
// deployment default.
func TestLoginPerAccessLimitOverride(t *testing.T) {
	hash, _ := auth.HashPassword("s3cret")
	limit := 1
	users := fakeUsers{bySubject: map[string]store.User{
		"bob": {ID: 5, TenantID: 1, Subject: "bob", Role: store.RoleUser, Status: store.StatusActive, SessionLimit: &limit},
	}}
	creds := fakeCreds{byUser: map[int64]string{5: hash}}
	tenants := fakeTenants{byID: map[int64]store.Tenant{1: {ID: 1, Status: store.StatusActive}}}
	fs := &fakeSessions{}
	cfg := LoginConfig{SessionKey: loginKey, Sessions: fs, SessionLimitDefault: 9}
	rec := postLogin(t, LoginHandler(users, creds, tenants, cfg), `{"subject":"bob","password":"s3cret"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if fs.lastCreate.limit != 1 {
		t.Fatalf("effective limit = %d, want per-access 1 (not default 9)", fs.lastCreate.limit)
	}
}

// TestLoginRejectedAtLimit: the reject policy surfaces ErrSessionLimit as 429
// with no cookie — distinct from the credential 401.
func TestLoginRejectedAtLimit(t *testing.T) {
	users, creds, tenants := loginFixture(t)
	fs := &fakeSessions{createErr: store.ErrSessionLimit}
	cfg := LoginConfig{SessionKey: loginKey, Sessions: fs, SessionLimitDefault: 1}
	rec := postLogin(t, LoginHandler(users, creds, tenants, cfg), `{"subject":"bob","password":"s3cret"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("no cookie should be set when the limit rejects the login")
	}
}

// TestLoginEvictPolicyPassedThrough: the configured evict_oldest policy reaches
// the store.
func TestLoginEvictPolicyPassedThrough(t *testing.T) {
	users, creds, tenants := loginFixture(t)
	fs := &fakeSessions{}
	cfg := LoginConfig{SessionKey: loginKey, Sessions: fs, SessionLimitDefault: 2, SessionLimitPolicy: store.SessionLimitEvictOldest}
	postLogin(t, LoginHandler(users, creds, tenants, cfg), `{"subject":"bob","password":"s3cret"}`)
	if fs.lastCreate == nil || fs.lastCreate.policy != store.SessionLimitEvictOldest {
		t.Fatalf("policy = %+v, want evict_oldest", fs.lastCreate)
	}
}

// TestRenewExtendsRegistrySession: a renew carrying a session-id cookie extends
// the registry row in place (same token) rather than minting a new one.
func TestRenewExtendsRegistrySession(t *testing.T) {
	fs := &fakeSessions{extendExp: time.Now().Add(30 * time.Minute)}
	cfg := LoginConfig{SessionKey: loginKey, TTL: 12 * time.Hour, MaxLifetime: 2 * time.Hour, Sessions: fs}
	cookieVal := auth.MintSessionID("live-tok", loginKey)

	req := httptest.NewRequest(http.MethodPost, "/api/session/renew", nil)
	req = req.WithContext(WithIdentity(req.Context(), Identity{TenantID: 7, UserID: 5, Subject: "bob"}))
	req.AddCookie(&http.Cookie{Name: "wf_session", Value: cookieVal})
	rec := httptest.NewRecorder()
	RenewHandler(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if fs.lastExtend == nil || fs.lastExtend.token != "live-tok" {
		t.Fatalf("ExtendSession token = %+v, want live-tok", fs.lastExtend)
	}
	if fs.lastExtend.ttl != 12*time.Hour || fs.lastExtend.maxLife != 2*time.Hour {
		t.Fatalf("extend ttl/max = %v/%v", fs.lastExtend.ttl, fs.lastExtend.maxLife)
	}
	if fs.lastCreate != nil {
		t.Error("renew of a registry session must not create a new one")
	}
	if got := sessionCookie(t, rec).Value; got != cookieVal {
		t.Errorf("renew changed the cookie value; want same token")
	}
}

// TestRenewRevokedSessionRejected: extending a revoked/expired registry session
// (ErrNotFound) forces a fresh login — 401, no cookie.
func TestRenewRevokedSessionRejected(t *testing.T) {
	fs := &fakeSessions{extendErr: store.ErrNotFound}
	cfg := LoginConfig{SessionKey: loginKey, Sessions: fs}
	req := httptest.NewRequest(http.MethodPost, "/api/session/renew", nil)
	req = req.WithContext(WithIdentity(req.Context(), Identity{UserID: 5, Subject: "bob"}))
	req.AddCookie(&http.Cookie{Name: "wf_session", Value: auth.MintSessionID("dead-tok", loginKey)})
	rec := httptest.NewRecorder()
	RenewHandler(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("no cookie should be re-issued for a revoked session")
	}
}

// TestRenewConvertsLegacyCookie: a renew carrying a legacy stateless cookie
// creates a registry session (sanfte Übernahme), anchoring the absolute-max clock
// at the original first-login time and issuing a session-id cookie.
func TestRenewConvertsLegacyCookie(t *testing.T) {
	fs := &fakeSessions{}
	cfg := LoginConfig{SessionKey: loginKey, TTL: 12 * time.Hour, MaxLifetime: 2 * time.Hour, Sessions: fs}
	iat := time.Now().Add(-30 * time.Minute)
	legacy := auth.MintSessionAt("bob", iat, time.Now().Add(time.Hour), loginKey)

	req := httptest.NewRequest(http.MethodPost, "/api/session/renew", nil)
	req = req.WithContext(WithIdentity(req.Context(), Identity{UserID: 5, Subject: "bob"}))
	req.AddCookie(&http.Cookie{Name: "wf_session", Value: legacy})
	rec := httptest.NewRecorder()
	RenewHandler(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if fs.lastCreate == nil {
		t.Fatal("legacy renew did not create a registry session")
	}
	if d := fs.lastCreate.createdAt.Sub(iat); d < -2*time.Second || d > 2*time.Second {
		t.Errorf("createdAt not anchored at legacy iat: delta %v", d)
	}
	// With no Users lookup and default 0, the effective limit is 0 (unlimited) — the
	// common opt-in-off case — so conversion is unconstrained here.
	if fs.lastCreate.limit != 0 {
		t.Errorf("conversion limit = %d, want 0 (no Users, default 0)", fs.lastCreate.limit)
	}
	if _, err := auth.ParseSessionID(sessionCookie(t, rec).Value, loginKey); err != nil {
		t.Errorf("converted cookie is not a session-id cookie: %v", err)
	}
}

// TestRenewConversionEnforcesLimit: the legacy-cookie conversion path enforces the
// per-access session limit (resolved via Users), so replaying a legacy cookie
// cannot mint unbounded sessions and bypass the limit during the rollout window.
func TestRenewConversionEnforcesLimit(t *testing.T) {
	limit := 2
	users := fakeUsers{bySubject: map[string]store.User{
		"bob": {ID: 5, TenantID: 7, Subject: "bob", Role: store.RoleUser, Status: store.StatusActive, SessionLimit: &limit},
	}}
	// The store reports the access is already at its limit.
	fs := &fakeSessions{createErr: store.ErrSessionLimit}
	cfg := LoginConfig{SessionKey: loginKey, TTL: 12 * time.Hour, Sessions: fs, Users: users, SessionLimitDefault: 9}
	legacy := auth.MintSessionAt("bob", time.Now().Add(-time.Minute), time.Now().Add(time.Hour), loginKey)

	req := httptest.NewRequest(http.MethodPost, "/api/session/renew", nil)
	req = req.WithContext(WithIdentity(req.Context(), Identity{TenantID: 7, UserID: 5, Subject: "bob"}))
	req.AddCookie(&http.Cookie{Name: "wf_session", Value: legacy})
	rec := httptest.NewRecorder()
	RenewHandler(cfg).ServeHTTP(rec, req)

	// The conversion must pass the per-access limit (2), not 0, and surface the
	// limit breach as 429 with no new cookie — the security fix for the bypass.
	if fs.lastCreate == nil || fs.lastCreate.limit != 2 {
		t.Fatalf("conversion limit = %+v, want per-access 2 (not 0/default 9)", fs.lastCreate)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (limit enforced on conversion)", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("no cookie should be set when conversion hits the limit")
	}
}

// TestLogoutDeletesRegistrySession: logout with a session-id cookie deletes the
// server-side session and clears the cookie.
func TestLogoutDeletesRegistrySession(t *testing.T) {
	fs := &fakeSessions{}
	cfg := LoginConfig{SessionKey: loginKey, Sessions: fs}
	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: "wf_session", Value: auth.MintSessionID("bye-tok", loginKey)})
	rec := httptest.NewRecorder()
	LogoutHandler(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if len(fs.deleted) != 1 || fs.deleted[0] != "bye-tok" {
		t.Fatalf("deleted = %v, want [bye-tok]", fs.deleted)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout did not clear the cookie: %+v", cookies)
	}
}
