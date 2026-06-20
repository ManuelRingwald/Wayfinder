package store

import "time"

// Role is a user's authorisation role within their tenant (ADR 0006 §5). It is a
// closed set; Valid guards writes so an unknown role never reaches the database.
type Role string

const (
	RoleOperator    Role = "operator"
	RoleTenantAdmin Role = "tenant_admin"
	RoleSuperAdmin  Role = "super_admin"
)

// Valid reports whether r is a recognised role.
func (r Role) Valid() bool {
	switch r {
	case RoleOperator, RoleTenantAdmin, RoleSuperAdmin:
		return true
	default:
		return false
	}
}

// Tenant is an isolated organisation (ADR 0005) — the unit of data isolation.
type Tenant struct {
	ID        int64
	Slug      string
	Name      string
	Status    string
	CreatedAt time.Time
}

// User belongs to exactly one tenant. Subject is the OIDC subject (proxy mode)
// or the username (builtin mode), ADR 0006 §5; it is the key by which an
// authenticated request is resolved to a tenant (WF2-11/12). Email is optional.
type User struct {
	ID        int64
	TenantID  int64
	Subject   string
	Email     *string
	Role      Role
	CreatedAt time.Time
}
