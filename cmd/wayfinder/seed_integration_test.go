package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// TestIntegrationAutoSeed exercises the ONB-1/ONB-3 (ADR 0011) boot auto-seed
// against a real Postgres: the first run provisions the default (tenant-less)
// admin with the known default password and the forced-change flag — and
// deliberately NO tenant (ADR 0011 Nachtrag: the earlier convenience tenant
// "default" is gone); a second run is a no-op (an admin already exists); and
// once the admin has rotated its password and cleared the flag, a restart does
// not undo that. Skips without WAYFINDER_TEST_DB_URL.
func TestIntegrationAutoSeed(t *testing.T) {
	dsn := os.Getenv("WAYFINDER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("set WAYFINDER_TEST_DB_URL to run the auto-seed integration test")
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

	// First boot: the default admin is created, must_change_password is set, and the
	// known default password verifies.
	var out bytes.Buffer
	if err := autoSeedDefaultAdmin(ctx, pool, &out); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	u, err := users.GetBySubject(ctx, defaultAdminSubject)
	if err != nil {
		t.Fatalf("default admin not created: %v", err)
	}
	if u.Role != store.RoleAdmin || !u.MustChangePassword {
		t.Fatalf("seeded admin = {role:%s mustChange:%v}, want {admin true}", u.Role, u.MustChangePassword)
	}
	// The platform admin is tenant-less (ONB-3).
	if u.TenantID != 0 {
		t.Fatalf("seeded admin TenantID = %d, want 0 (tenant-less platform admin)", u.TenantID)
	}
	hash, err := creds.GetHash(ctx, u.ID)
	if err != nil {
		t.Fatalf("credential not set: %v", err)
	}
	if ok, _ := auth.VerifyPassword(hash, defaultAdminPassword); !ok {
		t.Fatal("default password does not verify")
	}
	// No tenant is seeded any more (ADR 0011 Nachtrag): a fresh instance starts
	// with zero tenants; the operator names their own via the UI (ONB-4).
	var tenantCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM tenants`).Scan(&tenantCount); err != nil {
		t.Fatalf("count tenants: %v", err)
	}
	if tenantCount != 0 {
		t.Fatalf("seed created %d tenant(s), want 0 (no convenience tenant)", tenantCount)
	}

	// Second boot: a no-op — no duplicate admin.
	out.Reset()
	if err := autoSeedDefaultAdmin(ctx, pool, &out); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if admins, err := users.ListAdmins(ctx); err != nil {
		t.Fatalf("list admins: %v", err)
	} else if len(admins) != 1 {
		t.Fatalf("admin count after re-seed = %d, want 1 (idempotent)", len(admins))
	}

	// Operator rotates the password and clears the flag; a later boot must not undo
	// it (the guard keys on "an active admin exists", not on the flag).
	if err := users.SetMustChangePassword(ctx, u.ID, false); err != nil {
		t.Fatalf("clear flag: %v", err)
	}
	newHash, _ := auth.HashPassword("rotated-secret")
	if err := creds.Set(ctx, u.ID, newHash); err != nil {
		t.Fatalf("rotate password: %v", err)
	}
	out.Reset()
	if err := autoSeedDefaultAdmin(ctx, pool, &out); err != nil {
		t.Fatalf("third seed: %v", err)
	}
	after, _ := users.GetByID(ctx, u.ID)
	if after.MustChangePassword {
		t.Fatal("re-seed must not re-arm must_change_password after the operator cleared it")
	}
	hash, _ = creds.GetHash(ctx, u.ID)
	if ok, _ := auth.VerifyPassword(hash, "rotated-secret"); !ok {
		t.Fatal("re-seed clobbered the operator's rotated password")
	}
}
