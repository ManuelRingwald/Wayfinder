package auth

import "net/http"

// BuiltinAuthenticator authenticates via a signed session cookie (ModeBuiltin).
// The cookie is minted at login (after verifying an argon2id password); here it
// is read and verified. A missing, invalid or expired cookie is
// ErrUnauthenticated (fail-closed).
type BuiltinAuthenticator struct {
	CookieName string
	Key        []byte
}

// Authenticate reads and verifies the session cookie, returning its subject.
func (a BuiltinAuthenticator) Authenticate(r *http.Request) (string, error) {
	c, err := r.Cookie(a.CookieName)
	if err != nil {
		return "", ErrUnauthenticated
	}
	subject, err := ParseSession(c.Value, a.Key)
	if err != nil {
		return "", ErrUnauthenticated
	}
	return subject, nil
}
