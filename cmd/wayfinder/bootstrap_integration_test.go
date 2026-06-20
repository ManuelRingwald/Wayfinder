package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// TestIntegrationBootstrap exercises the bootstrap provisioning against a real
// Postgres: first run creates tenant+user+credential, a second run is idempotent
// and (re)sets the password, and a subject already homed in another tenant is a
// conflict. Skips without WAYFINDER_TEST_DB_URL (run via scripts/pg-test.sh).
func TestIntegrationBootstrap(t *testing.T) {
	dsn := os.Getenv("WAYFINDER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("set WAYFINDER_TEST_DB_URL to run the bootstrap integration test")
	}

	ctx := context.Background()
	pool, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`TRUNCATE tenants, users, credentials RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	p := bootstrapParams{
		TenantSlug: "acme",
		TenantName: "ACME Air",
		Subject:    "admin",
		Role:       store.RoleTenantAdmin,
		Password:   "hunter2pw",
	}

	// First run: everything is created.
	var out bytes.Buffer
	if err := runBootstrap(ctx, pool, p, &out); err != nil {
		t.Fatalf("first run: %v", err)
	}

	users := store.NewUserRepo(pool)
	creds := store.NewCredentialRepo(pool)

	u, err := users.GetBySubject(ctx, "admin")
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if u.Role != store.RoleTenantAdmin {
		t.Fatalf("role = %q, want tenant_admin", u.Role)
	}
	hash, err := creds.GetHash(ctx, u.ID)
	if err != nil {
		t.Fatalf("credential not set: %v", err)
	}
	if ok, _ := auth.VerifyPassword(hash, "hunter2pw"); !ok {
		t.Fatal("stored password does not verify")
	}

	// Second run with a new password: idempotent (no duplicate tenant/user) and
	// the password is updated.
	p.Password = "newpassword9"
	out.Reset()
	if err := runBootstrap(ctx, pool, p, &out); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if tenants, err := store.NewTenantRepo(pool).List(ctx); err != nil {
		t.Fatalf("list tenants: %v", err)
	} else if len(tenants) != 1 {
		t.Fatalf("tenant count = %d, want 1 (idempotent)", len(tenants))
	}
	if all, err := users.ListByTenant(ctx, u.TenantID); err != nil {
		t.Fatalf("list users: %v", err)
	} else if len(all) != 1 {
		t.Fatalf("user count = %d, want 1 (idempotent)", len(all))
	}
	hash, _ = creds.GetHash(ctx, u.ID)
	if ok, _ := auth.VerifyPassword(hash, "newpassword9"); !ok {
		t.Fatal("password was not updated on re-run")
	}

	// A subject already homed in a different tenant is a conflict, not a re-home.
	conflict := bootstrapParams{TenantSlug: "other", Subject: "admin", Role: store.RoleOperator}
	out.Reset()
	if err := runBootstrap(ctx, pool, conflict, &out); err == nil {
		t.Fatal("expected conflict for subject under a different tenant, got nil")
	}
}
