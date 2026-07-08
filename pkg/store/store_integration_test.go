package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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
		`TRUNCATE tenants, users, sessions, feeds, subscriptions, view_configs, user_view_profiles, entitlements, aeronautical_cache, platform_settings RESTART IDENTITY CASCADE`,
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

func TestIntegrationAeroCacheRepo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	repo := NewAeroCacheRepo(pool)

	ten, err := tenants.Create(ctx, "aero", "Aero Tenant")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	tid := ten.ID

	// Nothing cached yet: Load miss, Status not ok.
	if _, ok, err := repo.Load(ctx, &tid, "airspace"); err != nil || ok {
		t.Fatalf("initial load: ok=%v err=%v, want miss", ok, err)
	}
	if _, ok, err := repo.Status(ctx, &tid); err != nil || ok {
		t.Fatalf("initial status: ok=%v err=%v, want not ok", ok, err)
	}

	at := time.Unix(1_700_000_000, 0).UTC()
	// First fetch: change columns are nil (no prior to diff).
	if err := repo.Save(ctx, &tid, "airspace", `{"type":"FeatureCollection","features":[]}`, 3, nil, nil, nil, at); err != nil {
		t.Fatalf("save airspace: %v", err)
	}
	if err := repo.Save(ctx, &tid, "navaid", `{"type":"FeatureCollection","features":[]}`, 5, nil, nil, nil, at.Add(time.Hour)); err != nil {
		t.Fatalf("save navaid: %v", err)
	}

	// Round-trip one kind.
	got, ok, err := repo.Load(ctx, &tid, "airspace")
	if err != nil || !ok {
		t.Fatalf("load airspace: ok=%v err=%v", ok, err)
	}
	if got.FeatureCount != 3 || !got.FetchedAt.Equal(at) {
		t.Errorf("loaded = %+v, want count 3 @ %v", got, at)
	}

	// Upsert overwrites in place (still one row).
	// Second fetch (AERO-3 change-impact): prev 3 → 9, +6/−0.
	p3, a6, r0 := 3, 6, 0
	if err := repo.Save(ctx, &tid, "airspace", `{"type":"FeatureCollection","features":[]}`, 9, &p3, &a6, &r0, at.Add(2*time.Hour)); err != nil {
		t.Fatalf("upsert airspace: %v", err)
	}
	if got, _, _ := repo.Load(ctx, &tid, "airspace"); got.FeatureCount != 9 {
		t.Errorf("after upsert count = %d, want 9", got.FeatureCount)
	}
	// Changes reports the per-kind change-impact; airspace carries the diff, navaid
	// (first fetch) has nil change columns.
	changes, err := repo.Changes(ctx, &tid)
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	byKind := map[string]AeroCacheChange{}
	for _, c := range changes {
		byKind[c.Kind] = c
	}
	if as := byKind["airspace"]; as.PrevFeatureCount == nil || *as.PrevFeatureCount != 3 || as.Added == nil || *as.Added != 6 || as.Removed == nil || *as.Removed != 0 {
		t.Errorf("airspace change = %+v, want prev 3 / +6 / -0", as)
	}
	if nv := byKind["navaid"]; nv.PrevFeatureCount != nil || nv.Added != nil {
		t.Errorf("navaid (first fetch) should have nil change columns, got %+v", nv)
	}

	// Status: latest fetched_at (navaid, +1h) and summed features (9 + 5).
	st, ok, err := repo.Status(ctx, &tid)
	if err != nil || !ok {
		t.Fatalf("status: ok=%v err=%v", ok, err)
	}
	if st.FeatureCount != 14 {
		t.Errorf("status feature count = %d, want 14 (9+5)", st.FeatureCount)
	}
	if !st.FetchedAt.Equal(at.Add(time.Hour)) {
		t.Errorf("status fetched_at = %v, want the latest (%v)", st.FetchedAt, at.Add(time.Hour))
	}

	// The global (NULL tenant) cache is a distinct row from the tenant's.
	if err := repo.Save(ctx, nil, "airspace", `{"type":"FeatureCollection","features":[]}`, 100, nil, nil, nil, at); err != nil {
		t.Fatalf("save global: %v", err)
	}
	if g, ok, _ := repo.Load(ctx, nil, "airspace"); !ok || g.FeatureCount != 100 {
		t.Errorf("global load = %+v, want count 100", g)
	}
	if tn, _, _ := repo.Load(ctx, &tid, "airspace"); tn.FeatureCount != 9 {
		t.Errorf("tenant row must be unaffected by the global save, got %d", tn.FeatureCount)
	}

	// Deleting the tenant cascades its cache rows away (FK ON DELETE CASCADE).
	if err := tenants.Delete(ctx, tid); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	if _, ok, _ := repo.Status(ctx, &tid); ok {
		t.Error("tenant cache rows should be gone after the tenant is deleted")
	}
	if _, ok, _ := repo.Load(ctx, nil, "airspace"); !ok {
		t.Error("the global cache row must survive a tenant delete")
	}
}

func TestIntegrationSettingsRepo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewSettingsRepo(pool)

	// Missing key → not ok.
	if _, ok, err := repo.Get(ctx, "openaip_global_key"); err != nil || ok {
		t.Fatalf("initial get: ok=%v err=%v, want miss", ok, err)
	}

	if err := repo.Set(ctx, "openaip_global_key", "sealed-blob-1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if v, ok, err := repo.Get(ctx, "openaip_global_key"); err != nil || !ok || v != "sealed-blob-1" {
		t.Fatalf("get after set = (%q, %v, %v), want sealed-blob-1", v, ok, err)
	}

	// Upsert overwrites in place.
	if err := repo.Set(ctx, "openaip_global_key", "sealed-blob-2"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if v, _, _ := repo.Get(ctx, "openaip_global_key"); v != "sealed-blob-2" {
		t.Errorf("after upsert = %q, want sealed-blob-2", v)
	}

	// Delete is idempotent (deleting a missing key is not an error).
	if err := repo.Delete(ctx, "openaip_global_key"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, _ := repo.Get(ctx, "openaip_global_key"); ok {
		t.Error("key should be gone after delete")
	}
	if err := repo.Delete(ctx, "openaip_global_key"); err != nil {
		t.Errorf("deleting a missing key should be a no-op, got %v", err)
	}
}
