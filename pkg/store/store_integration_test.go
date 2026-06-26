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

// TestIntegrationUserStatusLifecycle exercises the AP6 access lifecycle against
// a real database: the status column defaults to active, can be paused and
// reactivated, a tenant can be paused, and a user can be deleted (cascading its
// credential). It also validates 00005's new column + CHECK constraint apply.
func TestIntegrationUserStatusLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)
	creds := NewCredentialRepo(pool)

	ten, err := tenants.Create(ctx, "acme", "ACME")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	u, err := users.Create(ctx, ten.ID, "alice", nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	// New rows default to active (non-breaking migration).
	if u.Status != StatusActive {
		t.Fatalf("new user status = %q, want active", u.Status)
	}

	if err := users.SetStatus(ctx, u.ID, StatusPaused); err != nil {
		t.Fatalf("pause user: %v", err)
	}
	if got, _ := users.GetByID(ctx, u.ID); got.Status != StatusPaused {
		t.Fatalf("after pause status = %q, want paused", got.Status)
	}
	if err := users.SetStatus(ctx, u.ID, StatusActive); err != nil {
		t.Fatalf("reactivate user: %v", err)
	}
	if err := users.SetStatus(ctx, 999999, StatusPaused); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetStatus(missing) = %v, want ErrNotFound", err)
	}

	// Tenant pause is a separate lever.
	if err := tenants.SetStatus(ctx, ten.ID, StatusPaused); err != nil {
		t.Fatalf("pause tenant: %v", err)
	}
	if got, _ := tenants.GetByID(ctx, ten.ID); got.Status != StatusPaused {
		t.Fatalf("after pause tenant status = %q, want paused", got.Status)
	}

	// Delete cascades to the credential and is idempotent on a missing row.
	if err := creds.Set(ctx, u.ID, "$argon2id$hash"); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	if err := users.Delete(ctx, u.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := users.GetByID(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID after delete = %v, want ErrNotFound", err)
	}
	if _, err := creds.GetHash(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("credential survived user delete = %v, want ErrNotFound", err)
	}
	if err := users.Delete(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(missing) = %v, want ErrNotFound", err)
	}
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

// TestIntegrationTenantOpenAIPKey verifies ONB-6 (ADR 0011): the per-tenant
// OpenAIP key round-trips through Get/SetOpenAIPKey, defaults to nil (global-key
// fallback), and can be cleared back to nil. A missing tenant yields ErrNotFound.
func TestIntegrationTenantOpenAIPKey(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewTenantRepo(pool)

	ten, err := repo.Create(ctx, "frankfurt", "FFM")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A freshly created tenant has no key (global fallback applies).
	if k, err := repo.GetOpenAIPKey(ctx, ten.ID); err != nil || k != nil {
		t.Fatalf("initial key = %v, %v; want nil, nil", k, err)
	}

	key := "openaip-key-123"
	if err := repo.SetOpenAIPKey(ctx, ten.ID, &key); err != nil {
		t.Fatalf("set key: %v", err)
	}
	got, err := repo.GetOpenAIPKey(ctx, ten.ID)
	if err != nil || got == nil || *got != key {
		t.Fatalf("after set, key = %v, %v; want %q", got, err, key)
	}

	// Clearing restores the global fallback (nil).
	if err := repo.SetOpenAIPKey(ctx, ten.ID, nil); err != nil {
		t.Fatalf("clear key: %v", err)
	}
	if k, err := repo.GetOpenAIPKey(ctx, ten.ID); err != nil || k != nil {
		t.Fatalf("after clear, key = %v, %v; want nil, nil", k, err)
	}

	// Missing tenant.
	if _, err := repo.GetOpenAIPKey(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetOpenAIPKey(missing) err = %v, want ErrNotFound", err)
	}
	if err := repo.SetOpenAIPKey(ctx, 999999, &key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetOpenAIPKey(missing) err = %v, want ErrNotFound", err)
	}
}

// TestIntegrationTenantDeleteCascades verifies ONB-4 (ADR 0011): deleting a
// tenant removes every row that references it — its users (and their
// credentials), feed subscriptions and entitlements — via ON DELETE CASCADE in a
// single atomic DELETE. A second delete yields ErrNotFound. Feeds are a global
// catalogue and must survive.
func TestIntegrationTenantDeleteCascades(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)
	creds := NewCredentialRepo(pool)
	feeds := NewFeedRepo(pool)
	subs := NewSubscriptionRepo(pool)
	ents := NewEntitlementRepo(pool)

	ten, err := tenants.Create(ctx, "acme", "ACME")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	u, err := users.Create(ctx, ten.ID, "pilot", nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := creds.Set(ctx, u.ID, "$argon2id$hash"); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	feed, err := feeds.Create(ctx, "FFM", "239.255.0.62", 8600, nil, []string{"PSR"})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	if err := subs.Subscribe(ctx, ten.ID, feed.ID); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := ents.Set(ctx, ten.ID, "stca", true); err != nil {
		t.Fatalf("set entitlement: %v", err)
	}

	// Delete the tenant: the cascade must take its dependents with it.
	if err := tenants.Delete(ctx, ten.ID); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	if _, err := tenants.GetByID(ctx, ten.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tenant survived delete = %v, want ErrNotFound", err)
	}
	if _, err := users.GetByID(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("user survived tenant delete = %v, want ErrNotFound", err)
	}
	if _, err := creds.GetHash(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("credential survived tenant delete = %v, want ErrNotFound", err)
	}
	if subscribed, err := subs.IsSubscribed(ctx, ten.ID, feed.ID); err != nil || subscribed {
		t.Fatalf("subscription survived tenant delete (subscribed=%v, err=%v)", subscribed, err)
	}
	// The feed is a global catalogue entry and must NOT be deleted with the tenant.
	if _, err := feeds.GetByID(ctx, feed.ID); err != nil {
		t.Fatalf("feed must survive tenant delete: %v", err)
	}
	// A second delete is a clean ErrNotFound.
	if err := tenants.Delete(ctx, ten.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(missing) = %v, want ErrNotFound", err)
	}
}

// TestIntegrationFeedDeleteCascades verifies ONB-5 (ADR 0011): deleting a feed
// removes its rows from the catalogue and cascades to the subscriptions that
// referenced it (ON DELETE CASCADE on subscriptions.feed_id), while the
// subscribing tenant itself survives. A second delete yields ErrNotFound, and the
// unique name freed by the delete can be reused.
func TestIntegrationFeedDeleteCascades(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	feeds := NewFeedRepo(pool)
	subs := NewSubscriptionRepo(pool)

	ten, err := tenants.Create(ctx, "acme", "ACME")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	feed, err := feeds.Create(ctx, "north", "239.255.0.70", 8600, nil, []string{"PSR"})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	if err := subs.Subscribe(ctx, ten.ID, feed.ID); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// GetByName resolves the feed; an unknown name is ErrNotFound.
	if got, err := feeds.GetByName(ctx, "north"); err != nil || got.ID != feed.ID {
		t.Fatalf("GetByName(north) = (%+v, %v), want feed %d", got, err, feed.ID)
	}
	if _, err := feeds.GetByName(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByName(nope) = %v, want ErrNotFound", err)
	}

	// Delete the feed: the subscription cascades away, the tenant survives.
	if err := feeds.Delete(ctx, feed.ID); err != nil {
		t.Fatalf("delete feed: %v", err)
	}
	if _, err := feeds.GetByID(ctx, feed.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("feed survived delete = %v, want ErrNotFound", err)
	}
	if subscribed, err := subs.IsSubscribed(ctx, ten.ID, feed.ID); err != nil || subscribed {
		t.Fatalf("subscription survived feed delete (subscribed=%v, err=%v)", subscribed, err)
	}
	if _, err := tenants.GetByID(ctx, ten.ID); err != nil {
		t.Fatalf("tenant must survive feed delete: %v", err)
	}
	// A second delete is a clean ErrNotFound.
	if err := feeds.Delete(ctx, feed.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(missing) = %v, want ErrNotFound", err)
	}
	// The freed unique name can be reused (migration 00008 constraint released).
	if _, err := feeds.Create(ctx, "north", "239.255.0.71", 8601, nil, nil); err != nil {
		t.Fatalf("reuse freed feed name: %v", err)
	}
}

// TestIntegrationFeedNameUnique verifies the migration 00008 UNIQUE(name)
// constraint: a second feed with the same name is rejected by the database.
func TestIntegrationFeedNameUnique(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	feeds := NewFeedRepo(pool)
	if _, err := feeds.Create(ctx, "dup", "239.255.0.70", 8600, nil, nil); err != nil {
		t.Fatalf("create first feed: %v", err)
	}
	if _, err := feeds.Create(ctx, "dup", "239.255.0.71", 8601, nil, nil); err == nil {
		t.Fatal("second feed with duplicate name should be rejected by the unique constraint")
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
	u, err := users.Create(ctx, ten.ID, "oidc|abc", &email)
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
	u2, err := users.Create(ctx, ten.ID, "oidc|noemail", nil)
	if err != nil || u2.Email != nil {
		t.Fatalf("create user without email = %+v, %v", u2, err)
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

// TestIntegrationAdminTenantSeparation verifies the strict admin/user separation
// (ONB-3, ADR 0011) against a real database: a platform admin is created with no
// tenant (TenantID 0), ListAdmins returns only admins (not tenant users), and the
// role/tenant CHECK constraint rejects both half-states (admin WITH a tenant, user
// WITHOUT one).
func TestIntegrationAdminTenantSeparation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)

	ten, err := tenants.Create(ctx, "acme", "ACME")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	// A platform admin has no tenant.
	admin, err := users.CreateAdmin(ctx, "root", nil)
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	if admin.Role != RoleAdmin || admin.TenantID != 0 {
		t.Fatalf("admin = %+v, want role admin and TenantID 0 (no tenant)", admin)
	}
	// Round-trips as tenant-less through GetBySubject too (the middleware path).
	if got, _ := users.GetBySubject(ctx, "root"); got.TenantID != 0 || got.Role != RoleAdmin {
		t.Fatalf("GetBySubject(admin) = %+v, want tenant-less admin", got)
	}

	// A tenant user lives under a tenant; ListAdmins must not include it.
	if _, err := users.Create(ctx, ten.ID, "pilot", nil); err != nil {
		t.Fatalf("create user: %v", err)
	}
	admins, err := users.ListAdmins(ctx)
	if err != nil {
		t.Fatalf("ListAdmins: %v", err)
	}
	if len(admins) != 1 || admins[0].Subject != "root" {
		t.Fatalf("ListAdmins = %+v, want exactly [root]", admins)
	}

	// The CHECK constraint rejects an admin WITH a tenant.
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (tenant_id, subject, role) VALUES ($1, 'bad-admin', 'admin')`, ten.ID); err == nil {
		t.Fatal("expected the role/tenant CHECK to reject an admin with a tenant")
	}
	// ...and a user WITHOUT one.
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (tenant_id, subject, role) VALUES (NULL, 'bad-user', 'user')`); err == nil {
		t.Fatal("expected the role/tenant CHECK to reject a tenant-less user")
	}
}
