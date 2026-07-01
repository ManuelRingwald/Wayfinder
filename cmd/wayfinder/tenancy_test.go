package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestLoadConfigTenancyEnvVars(t *testing.T) {
	t.Setenv("WAYFINDER_DB_URL", "postgres://x")
	t.Setenv("WAYFINDER_AUTH_MODE", "proxy")
	t.Setenv("WAYFINDER_SESSION_KEY", "sekret")
	t.Setenv("WAYFINDER_SESSION_TTL", "8h")
	t.Setenv("WAYFINDER_SESSION_MAX_LIFETIME", "30m")
	t.Setenv("WAYFINDER_OIDC_ISSUER", "https://iss")
	t.Setenv("WAYFINDER_OIDC_AUDIENCE", "wf")

	cfg := loadConfig()
	if cfg.DBURL != "postgres://x" || cfg.AuthMode != auth.ModeProxy ||
		string(cfg.SessionKey) != "sekret" || cfg.OIDCIssuer != "https://iss" || cfg.OIDCAudience != "wf" {
		t.Fatalf("tenancy config not parsed: %+v", cfg)
	}
	if cfg.SessionTTL != 8*time.Hour || cfg.SessionMaxLife != 30*time.Minute {
		t.Fatalf("session durations not parsed: ttl=%v max=%v", cfg.SessionTTL, cfg.SessionMaxLife)
	}

	// The absolute maximum is opt-in: unset/invalid leaves it disabled (0).
	t.Setenv("WAYFINDER_SESSION_MAX_LIFETIME", "not-a-duration")
	if got := loadConfig(); got.SessionMaxLife != 0 {
		t.Fatalf("invalid max lifetime = %v, want 0 (disabled)", got.SessionMaxLife)
	}

	// Unset/invalid auth mode falls back to builtin (ADR 0014: zero-touch default).
	t.Setenv("WAYFINDER_AUTH_MODE", "")
	if got := loadConfig(); got.AuthMode != auth.ModeBuiltin {
		t.Fatalf("default auth mode = %q, want builtin", got.AuthMode)
	}
}

func TestSetupTenancyRequiresDB(t *testing.T) {
	// Multi-tenant is the only mode (ADR 0014): a missing WAYFINDER_DB_URL fails
	// the start instead of degrading to an unauthenticated, unscoped ASD.
	mw, pool, err := setupTenancy(context.Background(), Config{}, discardLogger())
	if err == nil {
		t.Fatal("no DB → want an error; got nil")
	}
	if mw != nil || pool != nil {
		t.Fatalf("no DB → want (nil,nil,err); got mw=%v pool=%v", mw != nil, pool != nil)
	}
}

// TestSetupTenancyEnabled exercises the full wiring against a real Postgres:
// setupTenancy opens the DB, migrates, builds the builtin authenticator and the
// tenant middleware; the middleware then denies until a matching user exists and
// a valid session cookie is presented, and resolves the tenant once both hold.
// Skips without WAYFINDER_TEST_DB_URL.
func TestSetupTenancyEnabled(t *testing.T) {
	dsn := os.Getenv("WAYFINDER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("set WAYFINDER_TEST_DB_URL to run the tenancy wiring integration test")
	}

	key := []byte("test-session-key-test-session-key")
	cfg := Config{DBURL: dsn, AuthMode: auth.ModeBuiltin, SessionKey: key}
	mw, pool, err := setupTenancy(context.Background(), cfg, discardLogger())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if mw == nil || pool == nil {
		t.Fatal("expected middleware and pool")
	}
	defer pool.Close()

	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`TRUNCATE tenants, users, feeds, subscriptions, view_configs, entitlements RESTART IDENTITY CASCADE`,
	); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	// Fail-closed: with no session cookie, the request is denied.
	rec := httptest.NewRecorder()
	mw(ok).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no session → status %d, want 401", rec.Code)
	}

	// Seed a tenant + the "default" user; a request carrying a valid session
	// cookie for that subject now resolves the tenant.
	ten, err := store.NewTenantRepo(pool).Create(ctx, "demo", "Demo")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if _, err := store.NewUserRepo(pool).Create(ctx, ten.ID, "default", nil); err != nil {
		t.Fatalf("create user: %v", err)
	}

	var seenTenant int64
	resolved := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, found := tenant.FromContext(r.Context()); found {
			seenTenant = id.TenantID
		}
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "wf_session", Value: auth.MintSession("default", time.Hour, key)})
	rec = httptest.NewRecorder()
	mw(resolved).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || seenTenant != ten.ID {
		t.Fatalf("with user → status %d, tenant %d (want 200, %d)", rec.Code, seenTenant, ten.ID)
	}
}
