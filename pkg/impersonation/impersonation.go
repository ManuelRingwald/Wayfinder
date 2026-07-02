// Package impersonation implements admin "View as Tenant X" (ADR 0008): a
// deliberate, audited, time-boxed and READ-ONLY break of cross-tenant isolation
// (NFR-SEC-003).
//
// The mechanism is a separate, explicit signal that never overwrites the
// authenticated tenant.Identity (the trust anchor) and never reaches a write
// path. A grant is a signed, short-lived token naming a target tenant; when an
// admin presents a valid grant, the WS read path (feed scope AND view)
// resolves against the target tenant instead of the caller's own — nothing else
// changes.
//
// The platform-wide role is admin (ADR 0009 collapsed the earlier
// super_admin/tenant_admin/operator model to admin/user); the ADR 0008 text and
// its "super_admin" wording are read as "admin".
//
// Security rules enforced here, all fail-closed:
//   - Only a cryptographically valid, unexpired grant carries any authority.
//   - Such a grant activates impersonation ONLY for an admin caller.
//   - A valid grant presented by a non-admin is DENIED loudly (ErrDenied)
//     so the caller can reject (403 / handshake reject) and audit it — never
//     silently honoured, never silently ignored (ADR 0008 §3, decision 4).
//   - An absent or invalid/expired grant yields the normal, non-impersonated
//     path with no error, keeping the default behaviour byte-identical so the
//     WF2-22 isolation tests stay valid.
package impersonation

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// CookieName is the HttpOnly cookie that carries the impersonation grant. A
// cookie rides natively on both REST requests and the WebSocket upgrade
// handshake, which a custom header cannot (ADR 0008 §2).
const CookieName = "wf_impersonation"

var (
	// ErrInvalidGrant means the grant token is malformed, its signature does not
	// verify, or it has expired. Resolve treats this as "no impersonation".
	ErrInvalidGrant = errors.New("impersonation: invalid or expired grant")
	// ErrDenied means a VALID grant was presented by a caller that is not an
	// admin. The caller MUST fail loud (403 / handshake reject) and audit
	// it — this is the spoofing/misuse signal (ADR 0008 §3).
	ErrDenied = errors.New("impersonation: caller is not admin")
	// ErrUnknownTenant means an admin presented a valid grant naming a
	// tenant that does not exist. The caller MUST reject (cannot impersonate a
	// non-existent tenant).
	ErrUnknownTenant = errors.New("impersonation: target tenant does not exist")
)

// MintGrant returns a signed, time-boxed grant naming targetTenantID. It reuses
// the audit-reviewed auth session primitive (HMAC-SHA256 over "<payload>.<exp>"
// with constant-time verification) so the signing path has a single, tested
// implementation; the signed payload is the target tenant id as a decimal
// string. The grant is signed, not encrypted — it carries no secret.
func MintGrant(targetTenantID int64, ttl time.Duration, key []byte) string {
	return auth.MintSession(strconv.FormatInt(targetTenantID, 10), ttl, key)
}

// parseGrant verifies a grant's signature and expiry and returns the target
// tenant id. Any failure — bad signature, expiry, or an unparsable/non-positive
// payload — is ErrInvalidGrant (fail-closed).
func parseGrant(token string, key []byte) (int64, error) {
	payload, err := auth.ParseSession(token, key)
	if err != nil {
		return 0, ErrInvalidGrant
	}
	tid, err := strconv.ParseInt(payload, 10, 64)
	if err != nil || tid <= 0 {
		return 0, ErrInvalidGrant
	}
	return tid, nil
}

// TenantChecker reports whether a tenant id exists. *store.TenantRepo is adapted
// to satisfy it during wiring (Häppchen 2); tests use a fake. A non-nil error
// (e.g. the database is unreachable) makes Resolve fail closed.
type TenantChecker interface {
	Exists(ctx context.Context, tenantID int64) (bool, error)
}

// Decision is the outcome of evaluating a grant against the caller's identity.
type Decision struct {
	// Active reports whether impersonation applies. When true, the read scope
	// and view MUST be resolved against TargetTenantID instead of id.TenantID.
	Active bool
	// TargetTenantID is the impersonated tenant (meaningful only when Active).
	TargetTenantID int64
}

// Resolve evaluates a raw grant (the cookie value, "" when absent) against the
// authenticated identity and decides whether impersonation applies. It is the
// single decision point for the read path.
//
// Outcomes:
//
//	rawGrant == ""                          → Decision{}, nil              (default path)
//	invalid / expired / tampered grant      → Decision{}, nil              (ignored; the
//	    caller may audit a note since rawGrant was non-empty yet !Active && err == nil)
//	valid grant, caller NOT admin            → Decision{}, ErrDenied        (fail loud)
//	valid grant, admin, target missing       → Decision{}, ErrUnknownTenant (reject)
//	valid grant, admin, target exists        → Decision{Active:true, …}, nil
func Resolve(ctx context.Context, rawGrant string, id tenant.Identity, key []byte, tenants TenantChecker) (Decision, error) {
	if rawGrant == "" {
		return Decision{}, nil // no signal → normal, non-impersonated path
	}
	tid, err := parseGrant(rawGrant, key)
	if err != nil {
		// A stale or garbled cookie carries no authority. Do not error — fall
		// back to the default path so behaviour stays byte-identical; the caller
		// can audit a note because rawGrant was non-empty.
		return Decision{}, nil
	}
	// A cryptographically valid, unexpired grant is honoured ONLY for an
	// admin. Anyone else presenting one is a misuse signal → fail loud,
	// before touching the database.
	if id.Role != store.RoleAdmin {
		return Decision{}, ErrDenied
	}
	ok, err := tenants.Exists(ctx, tid)
	if err != nil {
		return Decision{}, err // database error → fail closed
	}
	if !ok {
		return Decision{}, ErrUnknownTenant
	}
	return Decision{Active: true, TargetTenantID: tid}, nil
}
