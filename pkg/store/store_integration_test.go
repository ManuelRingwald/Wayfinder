package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool returns a migrated, empty store backed by the Postgres at
// WAYFINDER_TEST_DB_URL. When the variable is unset (e.g. no database in the
// sandbox) the test skips; CI and the local "temp Postgres" runner set it. This
// also exercises Migrate against a real database, validating the 10.1 schema.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("WAYFINDER_TEST_DB_URL")
	if dsn == "" {
		t.Skip("set WAYFINDER_TEST_DB_URL to run store integration tests")
	}
	ctx := context.Background()
	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`TRUNCATE tenants, users, feeds, subscriptions, view_configs, entitlements RESTART IDENTITY CASCADE`,
	); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func TestIntegrationTenantRepo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewTenantRepo(pool)

	created, err := repo.Create(ctx, "frankfurt", "Leitstelle Frankfurt")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 || created.Slug != "frankfurt" || created.Status != "active" || created.CreatedAt.IsZero() {
		t.Fatalf("unexpected created tenant: %+v", created)
	}

	bySlug, err := repo.GetBySlug(ctx, "frankfurt")
	if err != nil || bySlug.ID != created.ID {
		t.Fatalf("GetBySlug = %+v, %v", bySlug, err)
	}
	byID, err := repo.GetByID(ctx, created.ID)
	if err != nil || byID.Slug != "frankfurt" {
		t.Fatalf("GetByID = %+v, %v", byID, err)
	}

	if _, err := repo.GetByID(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID(missing) err = %v, want ErrNotFound", err)
	}

	// The UNIQUE constraint on slug must reject a duplicate.
	if _, err := repo.Create(ctx, "frankfurt", "dup"); err == nil {
		t.Fatal("expected duplicate slug to be rejected")
	}

	list, err := repo.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List len = %d, %v", len(list), err)
	}
}

func TestIntegrationUserRepo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)

	ten, err := tenants.Create(ctx, "frankfurt", "FFM")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	email := "lotse@ffm.example"
	u, err := users.Create(ctx, ten.ID, "oidc|abc", &email, RoleUser)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.TenantID != ten.ID || u.Role != RoleUser || u.Email == nil || *u.Email != email {
		t.Fatalf("unexpected user: %+v", u)
	}

	// Subject lookup is the path WF2-11/12 use to resolve identity -> tenant.
	bySub, err := users.GetBySubject(ctx, "oidc|abc")
	if err != nil || bySub.ID != u.ID || bySub.TenantID != ten.ID {
		t.Fatalf("GetBySubject = %+v, %v", bySub, err)
	}

	// A nullable email round-trips as nil.
	u2, err := users.Create(ctx, ten.ID, "oidc|noemail", nil, RoleAdmin)
	if err != nil || u2.Email != nil {
		t.Fatalf("create user without email = %+v, %v", u2, err)
	}

	// Invalid role is rejected before hitting the database.
	if _, err := users.Create(ctx, ten.ID, "oidc|x", nil, Role("root")); err == nil {
		t.Fatal("expected invalid role to be rejected")
	}

	// Unknown subject -> ErrNotFound.
	if _, err := users.GetBySubject(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetBySubject(missing) err = %v, want ErrNotFound", err)
	}

	list, err := users.ListByTenant(ctx, ten.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListByTenant len = %d, %v", len(list), err)
	}
}
