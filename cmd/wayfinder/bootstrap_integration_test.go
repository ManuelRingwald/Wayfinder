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
// Postgres, in both worlds (ONB-3): a platform admin is created tenant-less and
// without spawning a tenant; a tenant user is homed under a get-or-created
// tenant. Re-runs are idempotent and (re)set the password, and crossing the
// admin/user boundary for an existing subject is a conflict. Skips without
// WAYFINDER_TEST_DB_URL (run via scripts/pg-test.sh).
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

	users := store.NewUserRepo(pool)
	creds := store.NewCredentialRepo(pool)
	tenants := store.NewTenantRepo(pool)

	// --- Admin world: tenant-less, creates no tenant ---
	p := bootstrapParams{Subject: "admin", Role: store.RoleAdmin, Password: "hunter2pw"}
	var out bytes.Buffer
	if err := runBootstrap(ctx, pool, p, &out); err != nil {
		t.Fatalf("admin first run: %v", err)
	}
	a, err := users.GetBySubject(ctx, "admin")
	if err != nil {
		t.Fatalf("admin not created: %v", err)
	}
	if a.Role != store.RoleAdmin || a.TenantID != 0 {
		t.Fatalf("admin = {role:%s tenant:%d}, want {admin 0} (tenant-less)", a.Role, a.TenantID)
	}
	if hash, err := creds.GetHash(ctx, a.ID); err != nil {
		t.Fatalf("admin credential not set: %v", err)
	} else if ok, _ := auth.VerifyPassword(hash, "hunter2pw"); !ok {
		t.Fatal("admin password does not verify")
	}
	// An admin bootstrap must not have created any tenant.
	if ts, err := tenants.List(ctx); err != nil {
		t.Fatalf("list tenants: %v", err)
	} else if len(ts) != 0 {
		t.Fatalf("tenant count after admin bootstrap = %d, want 0", len(ts))
	}

	// Second admin run with a new password: idempotent, password updated.
	p.Password = "newpassword9"
	out.Reset()
	if err := runBootstrap(ctx, pool, p, &out); err != nil {
		t.Fatalf("admin second run: %v", err)
	}
	if admins, err := users.ListAdmins(ctx); err != nil {
		t.Fatalf("list admins: %v", err)
	} else if len(admins) != 1 {
		t.Fatalf("admin count = %d, want 1 (idempotent)", len(admins))
	}
	hash, _ := creds.GetHash(ctx, a.ID)
	if ok, _ := auth.VerifyPassword(hash, "newpassword9"); !ok {
		t.Fatal("admin password was not updated on re-run")
	}

	// --- User world: homed under a get-or-created tenant ---
	up := bootstrapParams{TenantSlug: "acme", TenantName: "ACME Air", Subject: "pilot", Role: store.RoleUser, Password: "pilotpw12"}
	out.Reset()
	if err := runBootstrap(ctx, pool, up, &out); err != nil {
		t.Fatalf("user first run: %v", err)
	}
	pu, err := users.GetBySubject(ctx, "pilot")
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	ten, err := tenants.GetBySlug(ctx, "acme")
	if err != nil {
		t.Fatalf("tenant not created: %v", err)
	}
	if pu.Role != store.RoleUser || pu.TenantID != ten.ID {
		t.Fatalf("user = {role:%s tenant:%d}, want {user %d}", pu.Role, pu.TenantID, ten.ID)
	}
	// Idempotent re-run: still one tenant, one user under it.
	out.Reset()
	if err := runBootstrap(ctx, pool, up, &out); err != nil {
		t.Fatalf("user second run: %v", err)
	}
	if ts, _ := tenants.List(ctx); len(ts) != 1 {
		t.Fatalf("tenant count = %d, want 1 (idempotent)", len(ts))
	}
	if all, _ := users.ListByTenant(ctx, ten.ID); len(all) != 1 {
		t.Fatalf("user count under tenant = %d, want 1 (idempotent)", len(all))
	}

	// --- Crossing the admin/user boundary for an existing subject is a conflict ---
	out.Reset()
	if err := runBootstrap(ctx, pool, bootstrapParams{TenantSlug: "other", Subject: "admin", Role: store.RoleUser}, &out); err == nil {
		t.Fatal("expected conflict: existing admin re-bootstrapped as a tenant user")
	}
	out.Reset()
	if err := runBootstrap(ctx, pool, bootstrapParams{Subject: "pilot", Role: store.RoleAdmin}, &out); err == nil {
		t.Fatal("expected conflict: existing tenant user re-bootstrapped as an admin")
	}
}
