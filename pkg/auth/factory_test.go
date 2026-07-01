package auth

import (
	"context"
	"testing"
)

func TestNewAuthenticatorBuiltin(t *testing.T) {
	a, err := NewAuthenticator(context.Background(), Config{Mode: ModeBuiltin, SessionKey: []byte("k")})
	if err != nil {
		t.Fatalf("builtin: %v", err)
	}
	b, ok := a.(BuiltinAuthenticator)
	if !ok || b.CookieName != "wf_session" {
		t.Fatalf("builtin authenticator = %#v", a)
	}

	// Missing session key is a configuration error (fail-closed).
	if _, err := NewAuthenticator(context.Background(), Config{Mode: ModeBuiltin}); err == nil {
		t.Fatal("builtin without key should error")
	}
}

func TestNewAuthenticatorProxyValidation(t *testing.T) {
	// Missing issuer/audience errors before any network call.
	for _, cfg := range []Config{
		{Mode: ModeProxy},
		{Mode: ModeProxy, OIDCIssuer: "https://issuer.example"},
		{Mode: ModeProxy, OIDCAudience: "wayfinder"},
	} {
		if _, err := NewAuthenticator(context.Background(), cfg); err == nil {
			t.Errorf("proxy %#v should error on missing config", cfg)
		}
	}
}

func TestNewAuthenticatorUnknownMode(t *testing.T) {
	if _, err := NewAuthenticator(context.Background(), Config{Mode: Mode("bogus")}); err == nil {
		t.Fatal("unknown mode should error")
	}
}
