package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// idTokenVerifier abstracts OIDC token verification so ProxyAuthenticator can be
// unit-tested against a local issuer. go-oidc's *oidc.IDTokenVerifier satisfies it.
type idTokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

// ProxyAuthenticator authenticates requests in proxy mode (ModeProxy). An OIDC
// reverse proxy authenticates the user and forwards the OIDC token as a bearer
// token; Wayfinder *validates* it — issuer, audience, signature (against the
// issuer's JWKS) and expiry — rather than blindly trusting a header
// (defense-in-depth, ADR 0006 §5). The token's subject claim is the identity.
type ProxyAuthenticator struct {
	verifier idTokenVerifier
}

// NewProxyAuthenticator builds a ProxyAuthenticator validating tokens issued by
// issuer for audience. It contacts the issuer's OIDC discovery endpoint to learn
// the JWKS, so it needs network access and a live context at startup; the JWKS
// is then cached and refreshed by go-oidc.
func NewProxyAuthenticator(ctx context.Context, issuer, audience string) (*ProxyAuthenticator, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: oidc discovery for %q: %w", issuer, err)
	}
	return &ProxyAuthenticator{verifier: provider.Verifier(&oidc.Config{ClientID: audience})}, nil
}

// Authenticate validates the bearer OIDC token and returns its subject. A
// missing token, failed validation or empty subject is ErrUnauthenticated
// (fail-closed) — the detailed cause is intentionally not leaked to the caller.
func (a *ProxyAuthenticator) Authenticate(r *http.Request) (string, error) {
	raw := bearerToken(r)
	if raw == "" {
		return "", ErrUnauthenticated
	}
	tok, err := a.verifier.Verify(r.Context(), raw)
	if err != nil {
		return "", ErrUnauthenticated
	}
	if tok.Subject == "" {
		return "", ErrUnauthenticated
	}
	return tok.Subject, nil
}

// bearerToken extracts a bearer token from the Authorization header, or "".
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
