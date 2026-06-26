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

// User belongs to exactly one tenant. Subject is the OIDC subject (proxy mode)
// or the username (builtin mode), ADR 0006 §5; it is the key by which an
// authenticated request is resolved to a tenant (WF2-11/12). Email is optional.
// Status is the account lifecycle (AP6): a paused account cannot log in, but its
// row and configuration are retained. MustChangePassword (ONB-1, ADR 0011) marks
// an account whose password must be replaced before any other admin action is
// allowed — set on the auto-seeded default admin so the known default credential
// is valid for exactly one step: the one that replaces it.
type User struct {
	ID                 int64
	TenantID           int64
	Subject            string
	Email              *string
	Role               Role
	Status             Status
	MustChangePassword bool
	CreatedAt          time.Time
}
