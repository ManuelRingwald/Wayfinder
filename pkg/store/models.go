package store

import "time"

// Role is a user's authorisation role (ADR 0009 §1). It is a closed,
// two-value set: user (end user / pilot) and admin (platform operator).
// Valid guards writes so an unknown role never reaches the database.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// Valid reports whether r is a recognised role.
func (r Role) Valid() bool {
	switch r {
	case RoleUser, RoleAdmin:
		return true
	default:
		return false
	}
}

// Status is the lifecycle state of an access account or a tenant (AP6, ADR 0009).
// It is a closed set: active (may log in) and paused (suspended, data retained).
// Valid guards writes so an unknown status never reaches the database.
type Status string

const (
	StatusActive Status = "active"
	StatusPaused Status = "paused"
)

// Valid reports whether s is a recognised status.
func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusPaused:
		return true
	default:
		return false
	}
}

// SessionLimitPolicy decides what happens when an access is already at its
// concurrent-session limit and logs in again (AP7, ADR 0009 §5). It is a closed
// set; Valid guards configuration so an unknown policy never reaches enforcement.
type SessionLimitPolicy string

const (
	// SessionLimitReject refuses the N+1-th login (the ADR default): existing
	// sessions are kept, the new login is denied.
	SessionLimitReject SessionLimitPolicy = "reject"
	// SessionLimitEvictOldest drops the oldest session(s) to admit the new login.
	SessionLimitEvictOldest SessionLimitPolicy = "evict_oldest"
)

// Valid reports whether p is a recognised policy.
func (p SessionLimitPolicy) Valid() bool {
	switch p {
	case SessionLimitReject, SessionLimitEvictOldest:
		return true
	default:
		return false
	}
}

// Session is one server-side authentication session in the registry (AP7,
// ADR 0009 §5). ID is base64url(sha256(cookie token)) — the raw token is never
// stored. CreatedAt is the first-login time and anchors the absolute maximum
// lifetime (WF2-12.6); LastSeenAt is refreshed on each authenticated request;
// ExpiresAt is the sliding idle expiry. Meta carries best-effort client hints for
// an operator-facing "active sessions" view.
type Session struct {
	ID         string
	UserID     int64
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	Meta       SessionMeta
}

// SessionMeta is the optional client context recorded with a session. It is
// advisory only (never trusted for authorisation) and stored as JSONB.
type SessionMeta struct {
	UserAgent string `json:"user_agent,omitempty"`
	IP        string `json:"ip,omitempty"`
}

// Tenant is an isolated organisation (ADR 0005) — the unit of data isolation.
// Status gates login for all of the tenant's accounts (AP6): a paused tenant
// cascades to every access without touching the per-user status.
type Tenant struct {
	ID        int64
	Slug      string
	Name      string
	Status    Status
	CreatedAt time.Time
}

// User is either a platform admin or a tenant user, and the two are strictly
// separated (ONB-3, ADR 0011): a user (pilot/controller account) belongs to
// exactly one tenant; an admin (platform operator) belongs to none. TenantID
// carries that — a non-zero tenant for a user, and 0 (database NULL) for an
// admin. The invariant (admin XOR tenant) is enforced at the database by a CHECK
// constraint and in the store by separate Create/CreateAdmin constructors.
//
// Subject is the OIDC subject (proxy mode) or the username (builtin mode),
// ADR 0006 §5; it is the key by which an authenticated request is resolved
// (WF2-11/12). Email is optional. Status is the account lifecycle (AP6): a paused
// account cannot log in, but its row and configuration are retained.
// MustChangePassword (ONB-1, ADR 0011) marks an account whose password must be
// replaced before any other admin action is allowed — set on the auto-seeded
// default admin so the known default credential is valid for exactly one step:
// the one that replaces it.
// SessionLimit (AP7, ADR 0009 §5) is the per-access cap on concurrent sessions:
// nil falls back to the deployment default (WAYFINDER_SESSION_LIMIT_DEFAULT), a
// non-negative value overrides it, and 0 means unlimited.
type User struct {
	ID                 int64
	TenantID           int64 // 0 == no tenant (platform admin); non-zero for a tenant user
	Subject            string
	Email              *string
	Role               Role
	Status             Status
	MustChangePassword bool
	SessionLimit       *int
	CreatedAt          time.Time
}
