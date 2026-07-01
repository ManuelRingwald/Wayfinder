package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestIntegrationUserSessionLimitColumn verifies the per-access session_limit
// column round-trips through SetSessionLimit + scanUser (AP7, backs the admin-UI
// session-limit editor): default NULL, set to a value, and cleared back to NULL.
func TestIntegrationUserSessionLimitColumn(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)

	ten, _ := tenants.Create(ctx, "acme", "ACME")
	u, err := users.Create(ctx, ten.ID, "alice", nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	// New rows default to NULL (fall back to the deployment default).
	if u.SessionLimit != nil {
		t.Fatalf("new user session_limit = %v, want nil", u.SessionLimit)
	}

	three := 3
	if err := users.SetSessionLimit(ctx, u.ID, &three); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if got, _ := users.GetByID(ctx, u.ID); got.SessionLimit == nil || *got.SessionLimit != 3 {
		t.Fatalf("after set, session_limit = %v, want 3", got.SessionLimit)
	}

	// Clear back to NULL (fall back to default).
	if err := users.SetSessionLimit(ctx, u.ID, nil); err != nil {
		t.Fatalf("clear limit: %v", err)
	}
	if got, _ := users.GetByID(ctx, u.ID); got.SessionLimit != nil {
		t.Fatalf("after clear, session_limit = %v, want nil", *got.SessionLimit)
	}

	// A negative value is rejected before the query (fail-closed).
	neg := -1
	if err := users.SetSessionLimit(ctx, u.ID, &neg); err == nil {
		t.Fatal("SetSessionLimit(-1) = nil, want error")
	}
	// A missing user yields ErrNotFound.
	if err := users.SetSessionLimit(ctx, 999999, &three); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetSessionLimit(missing) = %v, want ErrNotFound", err)
	}
}

// TestIntegrationSessionRegistry exercises AP7 against a real database: create +
// resolve (with last_seen touch), the sliding/absolute expiry, real logout,
// and the janitor sweep. The limit and revocation paths have their own tests.
func TestIntegrationSessionRegistry(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)
	sessions := NewSessionRepo(pool)

	ten, err := tenants.Create(ctx, "acme", "ACME")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	u, err := users.Create(ctx, ten.ID, "alice", nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now()
	token, err := sessions.CreateSession(ctx, u.ID, now, now.Add(time.Hour), 0, SessionLimitReject, SessionMeta{UserAgent: "ua", IP: "10.0.0.1"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}

	// Resolve returns the owning subject and must touch last_seen_at.
	subj, err := sessions.ResolveSession(ctx, token)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if subj != "alice" {
		t.Fatalf("resolve subject = %q, want alice", subj)
	}
	var lastSeen1 time.Time
	if err := pool.QueryRow(ctx, `SELECT last_seen_at FROM sessions WHERE user_id = $1`, u.ID).Scan(&lastSeen1); err != nil {
		t.Fatalf("read last_seen: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := sessions.ResolveSession(ctx, token); err != nil {
		t.Fatalf("resolve again: %v", err)
	}
	var lastSeen2 time.Time
	if err := pool.QueryRow(ctx, `SELECT last_seen_at FROM sessions WHERE user_id = $1`, u.ID).Scan(&lastSeen2); err != nil {
		t.Fatalf("read last_seen 2: %v", err)
	}
	if !lastSeen2.After(lastSeen1) {
		t.Fatalf("last_seen not advanced: %v !> %v", lastSeen2, lastSeen1)
	}

	// A forged/unknown token resolves to ErrNotFound (fail-closed).
	if _, err := sessions.ResolveSession(ctx, "not-a-real-token"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resolve(unknown) = %v, want ErrNotFound", err)
	}

	// Count reflects the single active session.
	if n, _ := sessions.CountUserSessions(ctx, u.ID); n != 1 {
		t.Fatalf("CountUserSessions = %d, want 1", n)
	}
	if n, _ := sessions.CountActiveSessions(ctx); n != 1 {
		t.Fatalf("CountActiveSessions = %d, want 1", n)
	}

	// Logout deletes the row and is idempotent.
	if err := sessions.DeleteSession(ctx, token); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := sessions.ResolveSession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resolve after logout = %v, want ErrNotFound", err)
	}
	if err := sessions.DeleteSession(ctx, token); err != nil {
		t.Fatalf("second delete not idempotent: %v", err)
	}
}

// TestIntegrationSessionExpiry covers the sliding renew and the absolute maximum
// cap, plus the janitor.
func TestIntegrationSessionExpiry(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)
	sessions := NewSessionRepo(pool)

	ten, _ := tenants.Create(ctx, "acme", "ACME")
	u, _ := users.Create(ctx, ten.ID, "alice", nil)

	// An already-expired session does not resolve and is swept by the janitor.
	past := time.Now().Add(-time.Minute)
	expiredTok, err := sessions.CreateSession(ctx, u.ID, past.Add(-time.Hour), past, 0, SessionLimitReject, SessionMeta{})
	if err != nil {
		t.Fatalf("create expired: %v", err)
	}
	if _, err := sessions.ResolveSession(ctx, expiredTok); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resolve(expired) = %v, want ErrNotFound", err)
	}
	if n, _ := sessions.DeleteExpiredSessions(ctx); n != 1 {
		t.Fatalf("DeleteExpiredSessions = %d, want 1", n)
	}

	// Sliding renew: created an hour ago, extend by 30m with a 90m absolute cap →
	// the cap (created+90m) binds below now+30m only near the end; here now+30m is
	// the smaller, so the new expiry is ~30m out.
	created := time.Now().Add(-time.Hour)
	tok, err := sessions.CreateSession(ctx, u.ID, created, time.Now().Add(5*time.Minute), 0, SessionLimitReject, SessionMeta{})
	if err != nil {
		t.Fatalf("create sliding: %v", err)
	}
	exp, err := sessions.ExtendSession(ctx, tok, 30*time.Minute, 90*time.Minute)
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	// created+90m is ~30m from now, now+30m is ~30m from now — cap and slide are
	// within a minute of each other; assert the expiry lands in that window.
	if d := time.Until(exp); d < 28*time.Minute || d > 31*time.Minute {
		t.Fatalf("extended expiry %v out of expected ~30m window", d)
	}

	// Absolute cap reached: created 2h ago, 90m cap → extend returns an expiry not
	// after now, signalling "force fresh login".
	created2 := time.Now().Add(-2 * time.Hour)
	tok2, _ := sessions.CreateSession(ctx, u.ID, created2, time.Now().Add(5*time.Minute), 0, SessionLimitReject, SessionMeta{})
	exp2, err := sessions.ExtendSession(ctx, tok2, 30*time.Minute, 90*time.Minute)
	if err != nil {
		t.Fatalf("extend capped: %v", err)
	}
	if exp2.After(time.Now()) {
		t.Fatalf("capped extend expiry %v is after now, want <= now", exp2)
	}

	// Extending a deleted session fails closed.
	if err := sessions.DeleteSession(ctx, tok); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := sessions.ExtendSession(ctx, tok, 30*time.Minute, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("extend(deleted) = %v, want ErrNotFound", err)
	}
}

// TestIntegrationSessionLimit covers the login-time concurrent-session limit for
// both policies, including the per-access effective limit.
func TestIntegrationSessionLimit(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)
	sessions := NewSessionRepo(pool)

	ten, _ := tenants.Create(ctx, "acme", "ACME")
	u, _ := users.Create(ctx, ten.ID, "alice", nil)
	now := time.Now()
	mk := func(limit int, policy SessionLimitPolicy) (string, error) {
		return sessions.CreateSession(ctx, u.ID, now, now.Add(time.Hour), limit, policy, SessionMeta{})
	}

	// limit 0 == unlimited: three logins all succeed.
	for i := 0; i < 3; i++ {
		if _, err := mk(0, SessionLimitReject); err != nil {
			t.Fatalf("unlimited login %d: %v", i, err)
		}
	}
	if n, _ := sessions.CountUserSessions(ctx, u.ID); n != 3 {
		t.Fatalf("after unlimited, count = %d, want 3", n)
	}

	// Fresh user, limit 2, reject policy: the 3rd concurrent login is refused.
	u2, _ := users.Create(ctx, ten.ID, "bob", nil)
	mk2 := func(limit int, policy SessionLimitPolicy) (string, error) {
		return sessions.CreateSession(ctx, u2.ID, now, now.Add(time.Hour), limit, policy, SessionMeta{})
	}
	if _, err := mk2(2, SessionLimitReject); err != nil {
		t.Fatalf("login 1: %v", err)
	}
	if _, err := mk2(2, SessionLimitReject); err != nil {
		t.Fatalf("login 2: %v", err)
	}
	if _, err := mk2(2, SessionLimitReject); !errors.Is(err, ErrSessionLimit) {
		t.Fatalf("login 3 (reject) = %v, want ErrSessionLimit", err)
	}
	if n, _ := sessions.CountUserSessions(ctx, u2.ID); n != 2 {
		t.Fatalf("reject kept count = %d, want 2", n)
	}

	// evict_oldest: the oldest session is dropped and the new one admitted, so the
	// count stays at the limit and the evicted token no longer resolves. Reset to a
	// clean two-session state first for a deterministic eviction check.
	if _, err := sessions.DeleteUserSessions(ctx, u2.ID); err != nil {
		t.Fatalf("reset: %v", err)
	}
	oldTok, _ := sessions.CreateSession(ctx, u2.ID, now.Add(-2*time.Minute), now.Add(time.Hour), 2, SessionLimitReject, SessionMeta{})
	if _, err := sessions.CreateSession(ctx, u2.ID, now.Add(-time.Minute), now.Add(time.Hour), 2, SessionLimitReject, SessionMeta{}); err != nil {
		t.Fatalf("second: %v", err)
	}
	if _, err := sessions.CreateSession(ctx, u2.ID, now, now.Add(time.Hour), 2, SessionLimitEvictOldest, SessionMeta{}); err != nil {
		t.Fatalf("evict login: %v", err)
	}
	if n, _ := sessions.CountUserSessions(ctx, u2.ID); n != 2 {
		t.Fatalf("evict count = %d, want 2", n)
	}
	if _, err := sessions.ResolveSession(ctx, oldTok); !errors.Is(err, ErrNotFound) {
		t.Fatalf("evicted session still resolves: %v", err)
	}
}

// TestIntegrationSessionRevocation covers immediate revocation: a paused/deleted
// access and a paused tenant kill live sessions, both by the status join in
// ResolveSession and by the explicit cascade deletes.
func TestIntegrationSessionRevocation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)
	sessions := NewSessionRepo(pool)

	now := time.Now()

	// (1) Paused access: resolve fails via the status join even before any delete.
	ten, _ := tenants.Create(ctx, "acme", "ACME")
	u, _ := users.Create(ctx, ten.ID, "alice", nil)
	tok, _ := sessions.CreateSession(ctx, u.ID, now, now.Add(time.Hour), 0, SessionLimitReject, SessionMeta{})
	if _, err := sessions.ResolveSession(ctx, tok); err != nil {
		t.Fatalf("pre-pause resolve: %v", err)
	}
	if err := users.SetStatus(ctx, u.ID, StatusPaused); err != nil {
		t.Fatalf("pause user: %v", err)
	}
	if _, err := sessions.ResolveSession(ctx, tok); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resolve after user pause = %v, want ErrNotFound", err)
	}
	// The explicit revoke removes the now-dead row too.
	if n, _ := sessions.DeleteUserSessions(ctx, u.ID); n != 1 {
		t.Fatalf("DeleteUserSessions = %d, want 1", n)
	}

	// (2) Paused tenant cascades to all of its accesses.
	ten2, _ := tenants.Create(ctx, "beta", "Beta")
	a, _ := users.Create(ctx, ten2.ID, "amy", nil)
	b, _ := users.Create(ctx, ten2.ID, "ben", nil)
	tokA, _ := sessions.CreateSession(ctx, a.ID, now, now.Add(time.Hour), 0, SessionLimitReject, SessionMeta{})
	tokB, _ := sessions.CreateSession(ctx, b.ID, now, now.Add(time.Hour), 0, SessionLimitReject, SessionMeta{})
	if err := tenants.SetStatus(ctx, ten2.ID, StatusPaused); err != nil {
		t.Fatalf("pause tenant: %v", err)
	}
	if _, err := sessions.ResolveSession(ctx, tokA); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resolve A after tenant pause = %v, want ErrNotFound", err)
	}
	if _, err := sessions.ResolveSession(ctx, tokB); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resolve B after tenant pause = %v, want ErrNotFound", err)
	}
	if n, _ := sessions.DeleteTenantSessions(ctx, ten2.ID); n != 2 {
		t.Fatalf("DeleteTenantSessions = %d, want 2", n)
	}

	// (3) Deleting an access removes its sessions by ON DELETE CASCADE.
	ten3, _ := tenants.Create(ctx, "gamma", "Gamma")
	g, _ := users.Create(ctx, ten3.ID, "gil", nil)
	gtok, _ := sessions.CreateSession(ctx, g.ID, now, now.Add(time.Hour), 0, SessionLimitReject, SessionMeta{})
	if err := users.Delete(ctx, g.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := sessions.ResolveSession(ctx, gtok); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resolve after user delete = %v, want ErrNotFound", err)
	}
}
