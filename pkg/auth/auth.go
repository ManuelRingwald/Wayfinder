// Package auth provides browser-edge authentication for Wayfinder 2.0 (ADR 0006
// §5). It establishes *who* a request is (the identity "subject"); mapping that
// subject to a user and tenant, and enforcing it as fail-closed middleware,
// lives in WF2-12.
//
// Two modes (WAYFINDER_AUTH_MODE), mirroring the proxy-primary pattern of
// ADR 0003. Multi-tenant is the only supported mode (ADR 0014): authentication
// is always on — there is no unauthenticated "none" mode.
//
//   - proxy   (primary): an OIDC reverse proxy authenticates and forwards a
//     trusted token; Wayfinder validates it (WF2-11.2).
//   - builtin (default): Wayfinder authenticates users itself via argon2id
//     password hashes and a signed session cookie (this file + password/session).
//     The default when WAYFINDER_AUTH_MODE is unset (zero-touch, ADR 0011).
package auth

import (
	"errors"
	"net/http"
	"strings"
)

// ErrUnauthenticated is returned by an Authenticator when a request carries no
// valid identity. Callers treat it as fail-closed: no identity, no access.
var ErrUnauthenticated = errors.New("auth: unauthenticated")

// Mode selects how requests are authenticated (WAYFINDER_AUTH_MODE).
type Mode string

const (
	ModeProxy   Mode = "proxy"
	ModeBuiltin Mode = "builtin"
)

// ParseMode parses WAYFINDER_AUTH_MODE. An empty or unrecognised value falls
// back to ModeBuiltin (the zero-touch multi-tenant default, ADR 0014); ok
// reports whether the input was a recognised mode, so the caller can warn on
// fallback. There is no unauthenticated mode — authentication is always on.
func ParseMode(s string) (mode Mode, ok bool) {
	switch Mode(strings.ToLower(strings.TrimSpace(s))) {
	case ModeProxy:
		return ModeProxy, true
	case ModeBuiltin:
		return ModeBuiltin, true
	default:
		return ModeBuiltin, false
	}
}

// Authenticator establishes the identity subject of a request. It returns
// ErrUnauthenticated when no valid identity is present.
type Authenticator interface {
	Authenticate(r *http.Request) (subject string, err error)
}
