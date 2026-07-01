package auth

import (
	"context"
	"net/http"
)

// SessionResolver validates a registry session token and returns the subject of
// its owner, or an error when the token is unknown, expired, revoked, or its
// access/tenant is paused (AP7, ADR 0009 §5). *store.SessionRepo satisfies it;
// tests use a fake. It is defined here (rather than importing store) to keep the
// auth package free of a persistence dependency, mirroring tenant.UserLookup.
type SessionResolver interface {
	ResolveSession(ctx context.Context, token string) (subject string, err error)
}

// BuiltinAuthenticator authenticates via a signed session cookie (ModeBuiltin).
// The cookie is minted at login (after verifying an argon2id password); here it
// is read and verified. A missing, invalid or expired cookie is
// ErrUnauthenticated (fail-closed).
//
// When Sessions is set (AP7), the authenticator is registry-backed: a session-id
// cookie is resolved against the registry, so a session can be revoked
// immediately (pause/delete/logout) rather than living until its signature
// expires. For the rollout it stays backward compatible — a still-valid legacy
// stateless cookie (subject.iat.exp, minted before AP7) is accepted on its
// signature alone (sanfte Übernahme). The next renew converts such a cookie into
// a registry session, so the stateless grace window closes within one TTL. With
// Sessions nil the authenticator is purely stateless (the pre-AP7 behaviour).
type BuiltinAuthenticator struct {
	CookieName string
	Key        []byte
	Sessions   SessionResolver
}

// Authenticate reads and verifies the session cookie, returning its subject. A
// registry session id is resolved against the registry (revocable); a legacy
// stateless cookie is accepted on its signature (transitional). Any failure is
// ErrUnauthenticated.
func (a BuiltinAuthenticator) Authenticate(r *http.Request) (string, error) {
	c, err := r.Cookie(a.CookieName)
	if err != nil {
		return "", ErrUnauthenticated
	}

	// Registry path (AP7): a signed session-id cookie is authoritative only if the
	// registry still holds the session and its access/tenant are active.
	if a.Sessions != nil {
		if token, perr := ParseSessionID(c.Value, a.Key); perr == nil {
			subject, rerr := a.Sessions.ResolveSession(r.Context(), token)
			if rerr != nil {
				return "", ErrUnauthenticated
			}
			return subject, nil
		}
		// Not a session-id cookie → fall through to the legacy stateless path so
		// browsers holding a pre-AP7 cookie stay logged in until their next renew.
	}

	subject, err := ParseSession(c.Value, a.Key)
	if err != nil {
		return "", ErrUnauthenticated
	}
	return subject, nil
}
