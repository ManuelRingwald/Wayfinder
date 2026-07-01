package auth

import (
	"context"
	"fmt"
)

// Config selects and configures the Authenticator (from WAYFINDER_AUTH_* env).
type Config struct {
	Mode Mode

	// CookieName and SessionKey configure ModeBuiltin. SessionKey is required.
	CookieName string
	SessionKey []byte
	// Sessions is the server-side session registry (AP7, ADR 0009 §5). When set,
	// ModeBuiltin resolves session-id cookies against it (revocable sessions) and
	// still accepts legacy stateless cookies during the rollout. Nil keeps the
	// pre-AP7 stateless behaviour.
	Sessions SessionResolver

	// OIDCIssuer and OIDCAudience configure ModeProxy. Both are required.
	OIDCIssuer   string
	OIDCAudience string
}

// NewAuthenticator builds the Authenticator for cfg.Mode. Missing required
// settings are an error (fail-closed configuration): a half-configured secure
// mode must not silently degrade. ModeProxy contacts the OIDC issuer's discovery
// endpoint, so it needs network access and a live ctx.
func NewAuthenticator(ctx context.Context, cfg Config) (Authenticator, error) {
	switch cfg.Mode {
	case ModeBuiltin:
		if len(cfg.SessionKey) == 0 {
			return nil, fmt.Errorf("auth: builtin mode requires a session key")
		}
		name := cfg.CookieName
		if name == "" {
			name = "wf_session"
		}
		return BuiltinAuthenticator{CookieName: name, Key: cfg.SessionKey, Sessions: cfg.Sessions}, nil

	case ModeProxy:
		if cfg.OIDCIssuer == "" || cfg.OIDCAudience == "" {
			return nil, fmt.Errorf("auth: proxy mode requires OIDC issuer and audience")
		}
		return NewProxyAuthenticator(ctx, cfg.OIDCIssuer, cfg.OIDCAudience)

	default:
		return nil, fmt.Errorf("auth: unknown mode %q", cfg.Mode)
	}
}
