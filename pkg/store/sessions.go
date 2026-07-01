package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSessionLimit is returned by CreateSession when the access is already at its
// concurrent-session limit and the policy is reject (AP7, ADR 0009 §5). Callers
// translate it into a login denial distinct from a wrong password.
var ErrSessionLimit = errors.New("store: session limit reached")

// sessionTokenEnc encodes both the raw cookie token and its stored hash. Raw
// (dot-free) URL base64 keeps the token safe to carry in a cookie and to embed
// in the signed value without colliding with the '.' field separator.
var sessionTokenEnc = base64.RawURLEncoding

// SessionRepo is the server-side session registry (AP7, ADR 0009 §5): the source
// of truth for which sessions exist, so they can be counted (per-access limit)
// and revoked (immediate pause/delete/logout) — neither of which a stateless
// cookie allows.
type SessionRepo struct {
	db *pgxpool.Pool
}

// NewSessionRepo returns a SessionRepo backed by the given pool.
func NewSessionRepo(db *pgxpool.Pool) *SessionRepo { return &SessionRepo{db: db} }

// hashToken maps a raw cookie token to the id stored in the registry. Only the
// hash is persisted, so a database dump does not yield usable cookies (the raw
// token lives only in the browser).
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return sessionTokenEnc.EncodeToString(sum[:])
}

// newSessionToken returns a fresh, unguessable 256-bit token (the value the
// cookie carries) together with its stored hash.
func newSessionToken() (token, id string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("store: generate session token: %w", err)
	}
	token = sessionTokenEnc.EncodeToString(b)
	return token, hashToken(token), nil
}

// CreateSession opens a session for userID and returns the raw token to place in
// the cookie. The concurrent-session limit is enforced atomically: limit == 0
// disables the check; otherwise, when the access is already at its limit, policy
// decides — SessionLimitReject returns ErrSessionLimit, SessionLimitEvictOldest
// deletes the oldest session(s) to make room.
//
// createdAt anchors the absolute maximum lifetime (WF2-12.6): pass now for a
// first login, or the original first-login time when adopting a still-valid
// legacy cookie (the sanfte-Übernahme migration path). expiresAt is the caller's
// already-capped sliding expiry, so even an unrenewed session self-expires at the
// absolute maximum via the ordinary expiry check.
func (r *SessionRepo) CreateSession(ctx context.Context, userID int64, createdAt, expiresAt time.Time, limit int, policy SessionLimitPolicy, meta SessionMeta) (string, error) {
	token, id, err := newSessionToken()
	if err != nil {
		return "", err
	}
	metaJSON, err := toJSONB(meta)
	if err != nil {
		return "", fmt.Errorf("store: marshal session meta: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", wrap("create session", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit

	// Serialize concurrent logins for the same access so the count-then-insert
	// below cannot race past the limit (TOCTOU), including across replicas — a
	// transaction-scoped advisory lock keyed on the user id, released at commit.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, userID); err != nil {
		return "", wrap("lock session", err)
	}
	// Expired rows must not count toward the limit; drop them first so a login is
	// never rejected against sessions that no longer exist.
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND expires_at <= now()`, userID); err != nil {
		return "", wrap("prune sessions", err)
	}

	if limit > 0 {
		var active int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id = $1`, userID).Scan(&active); err != nil {
			return "", wrap("count sessions", err)
		}
		if active >= limit {
			switch policy {
			case SessionLimitEvictOldest:
				// Free exactly enough room for the newcomer (usually one).
				evict := active - limit + 1
				if _, err := tx.Exec(ctx,
					`DELETE FROM sessions WHERE id IN (
						SELECT id FROM sessions WHERE user_id = $1 ORDER BY created_at ASC, id ASC LIMIT $2)`,
					userID, evict); err != nil {
					return "", wrap("evict sessions", err)
				}
			default: // SessionLimitReject (the ADR default)
				return "", ErrSessionLimit
			}
		}
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO sessions (id, user_id, created_at, last_seen_at, expires_at, client_meta)
		 VALUES ($1, $2, $3, $3, $4, $5::jsonb)`,
		id, userID, createdAt, expiresAt, metaJSON); err != nil {
		return "", wrap("insert session", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", wrap("commit session", err)
	}
	return token, nil
}

// ResolveSession validates a session token and returns the subject of its owner,
// refreshing last_seen_at. It enforces the full fail-closed chain in a single
// statement: the session must exist and be unexpired, the owning access must be
// active, and (for a tenant user) its tenant must be active. Any miss — unknown
// token, expired, revoked, paused access, paused tenant — yields ErrNotFound,
// which the authenticator treats as unauthenticated. This is the per-request
// registry lookup that gives immediate pause/revoke even without deleting rows.
func (r *SessionRepo) ResolveSession(ctx context.Context, token string) (string, error) {
	const q = `
		UPDATE sessions s
		   SET last_seen_at = now()
		  FROM users u
		  LEFT JOIN tenants t ON t.id = u.tenant_id
		 WHERE s.id = $1
		   AND s.user_id = u.id
		   AND s.expires_at > now()
		   AND u.status = 'active'
		   AND (u.tenant_id IS NULL OR t.status = 'active')
		RETURNING u.subject`
	var subject string
	if err := r.db.QueryRow(ctx, q, hashToken(token)).Scan(&subject); err != nil {
		return "", wrap("resolve session", err)
	}
	return subject, nil
}

// ExtendSession slides a session's expiry to now+ttl, capped at
// created_at+maxLife when maxLife > 0 (the absolute maximum, WF2-12.6). It
// refreshes last_seen_at and returns the new expiry. A revoked or already-expired
// session does not match and yields ErrNotFound, so renew forces a fresh login.
// When the absolute cap is already reached the returned expiry is not after now;
// the caller treats that as the maximum and refuses to renew.
func (r *SessionRepo) ExtendSession(ctx context.Context, token string, ttl, maxLife time.Duration) (time.Time, error) {
	const q = `
		UPDATE sessions SET
			last_seen_at = now(),
			expires_at = CASE
				WHEN $3::bigint > 0
					THEN LEAST(now() + make_interval(secs => $2), created_at + make_interval(secs => $3))
				ELSE now() + make_interval(secs => $2)
			END
		 WHERE id = $1 AND expires_at > now()
		RETURNING expires_at`
	var exp time.Time
	if err := r.db.QueryRow(ctx, q, hashToken(token), ttl.Seconds(), int64(maxLife.Seconds())).Scan(&exp); err != nil {
		return time.Time{}, wrap("extend session", err)
	}
	return exp, nil
}

// DeleteSession removes one session by its token — a real, server-side logout
// (AP7). It is idempotent: an unknown or already-removed token is not an error.
func (r *SessionRepo) DeleteSession(ctx context.Context, token string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, hashToken(token)); err != nil {
		return wrap("delete session", err)
	}
	return nil
}

// DeleteUserSessions revokes every session of one access (immediate pause/delete,
// AP7) and returns the number revoked. Idempotent.
func (r *SessionRepo) DeleteUserSessions(ctx context.Context, userID int64) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return 0, wrap("delete user sessions", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteTenantSessions revokes every session of every access under a tenant — the
// immediate tenant-pause cascade (AP7) — and returns the number revoked.
func (r *SessionRepo) DeleteTenantSessions(ctx context.Context, tenantID int64) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE tenant_id = $1)`, tenantID)
	if err != nil {
		return 0, wrap("delete tenant sessions", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteExpiredSessions removes all sessions past their expiry — the janitor
// sweep — and returns the number removed. Expiry is also enforced at resolve
// time; this only stops dead rows from accumulating.
func (r *SessionRepo) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, wrap("delete expired sessions", err)
	}
	return tag.RowsAffected(), nil
}

// CountUserSessions returns the number of unexpired sessions for one access.
func (r *SessionRepo) CountUserSessions(ctx context.Context, userID int64) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM sessions WHERE user_id = $1 AND expires_at > now()`, userID).Scan(&n); err != nil {
		return 0, wrap("count user sessions", err)
	}
	return n, nil
}

// CountActiveSessions returns the number of unexpired sessions across all
// accesses — the wayfinder_active_sessions gauge.
func (r *SessionRepo) CountActiveSessions(ctx context.Context) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE expires_at > now()`).Scan(&n); err != nil {
		return 0, wrap("count active sessions", err)
	}
	return n, nil
}
