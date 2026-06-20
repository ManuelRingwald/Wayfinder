package auth

import (
	"errors"
	"testing"
	"time"
)

var testKey = []byte("test-signing-key-0123456789")

func TestSessionRoundTrip(t *testing.T) {
	tok := MintSession("oidc|alice", time.Hour, testKey)
	subject, err := ParseSession(tok, testKey)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if subject != "oidc|alice" {
		t.Fatalf("subject = %q, want oidc|alice", subject)
	}
}

func TestSessionTampered(t *testing.T) {
	tok := MintSession("alice", time.Hour, testKey)
	// Flip the last character of the signature.
	b := []byte(tok)
	if b[len(b)-1] == 'A' {
		b[len(b)-1] = 'B'
	} else {
		b[len(b)-1] = 'A'
	}
	if _, err := ParseSession(string(b), testKey); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("tampered token err = %v, want ErrSessionInvalid", err)
	}

	if _, err := ParseSession("garbage", testKey); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("garbage token err = %v, want ErrSessionInvalid", err)
	}
}

func TestSessionWrongKey(t *testing.T) {
	tok := MintSession("alice", time.Hour, testKey)
	if _, err := ParseSession(tok, []byte("a-different-key")); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("wrong-key err = %v, want ErrSessionInvalid", err)
	}
}

func TestSessionExpired(t *testing.T) {
	tok := MintSession("alice", -time.Minute, testKey) // already expired
	if _, err := ParseSession(tok, testKey); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expired err = %v, want ErrSessionExpired", err)
	}
}
