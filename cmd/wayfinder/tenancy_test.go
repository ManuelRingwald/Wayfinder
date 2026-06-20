package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestLoadConfigTenancyEnvVars(t *testing.T) {
	t.Setenv("WAYFINDER_DB_URL", "postgres://x")
	t.Setenv("WAYFINDER_AUTH_MODE", "proxy")
	t.Setenv("WAYFINDER_SESSION_KEY", "sekret")
	t.Setenv("WAYFINDER_OIDC_ISSUER", "https://iss")
	t.Setenv("WAYFINDER_OIDC_AUDIENCE", "wf")

	cfg := loadConfig()
	if cfg.DBURL != "postgres://x" || cfg.AuthMode != auth.ModeProxy ||
		string(cfg.SessionKey) != "sekret" || cfg.OIDCIssuer != "https://iss" || cfg.OIDCAudience != "wf" {
		t.Fatalf("tenancy config not parsed: %+v", cfg)
	}

	// Unset/invalid auth mode falls back to none.
	t.Setenv("WAYFINDER_AUTH_MODE", "")
	if got := loadConfig(); got.AuthMode != auth.ModeNone {
		t.Fatalf("default auth mode = %q, want none", got.AuthMode)
	}
}

func TestSetupTenancyDisabledWithoutDB(t *testing.T) {
	mw, pool, err := setupTenancy(context.Background(), Config{}, discardLogger())
	if err != nil || mw != nil || pool != nil {
		t.Fatalf("no DB → want (nil,nil,nil); got mw=%v pool=%v err=%v", mw != nil, pool != nil, err)
	}
}

// TestSetupTenancyEnabled exercises the full wiring against a real Postgres:
// setupTenancy opens the DB, migrates, builds the (none-mode) authenticator and
// the tenant middleware; the middleware then denies until a matching user exists
// and resolves the tenant once it does. Skips without WAYFINDER_TEST_DB_URL.
func TestSetupTenancyEnabled(t *testing.T) {
	dsn := os.Getenv("WAYFINDER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("set WAYFINDER_TEST_DB_URL to run the tenancy wiring integration test")
	}

	cfg := Config{DBURL: dsn, AuthMode: auth.ModeNone, NoneSubject: "default"}
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

	// Fail-closed: with no "default" user, the request is denied.
	rec := httptest.NewRecorder()
	mw(ok).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no user → status %d, want 401", rec.Code)
	}

	// Seed a tenant + the "default" user; now the middleware resolves the tenant.
	ten, err := store.NewTenantRepo(pool).Create(ctx, "demo", "Demo")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if _, err := store.NewUserRepo(pool).Create(ctx, ten.ID, "default", nil, store.RoleOperator); err != nil {
		t.Fatalf("create user: %v", err)
	}

	var seenTenant int64
	resolved := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, found := tenant.FromContext(r.Context()); found {
			seenTenant = id.TenantID
		}
		w.WriteHeader(http.StatusOK)
	})
	rec = httptest.NewRecorder()
	mw(resolved).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK || seenTenant != ten.ID {
		t.Fatalf("with user → status %d, tenant %d (want 200, %d)", rec.Code, seenTenant, ten.ID)
	}
}
