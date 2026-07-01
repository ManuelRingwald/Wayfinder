package auth

import (
	"errors"
	"strconv"
	"strings"
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

func TestSessionClaimsRoundTrip(t *testing.T) {
	before := time.Now().Unix()
	c, err := ParseSessionClaims(MintSession("oidc|alice", time.Hour, testKey), testKey)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Subject != "oidc|alice" {
		t.Fatalf("subject = %q, want oidc|alice", c.Subject)
	}
	if c.IssuedAt < before || c.IssuedAt > time.Now().Unix() {
		t.Errorf("IssuedAt = %d, want ~now", c.IssuedAt)
	}
	if c.ExpiresAt <= c.IssuedAt {
		t.Errorf("ExpiresAt %d not after IssuedAt %d", c.ExpiresAt, c.IssuedAt)
	}
}

func TestSessionMintAtPreservesIssuedAt(t *testing.T) {
	iat := time.Now().Add(-25 * time.Minute)
	exp := time.Now().Add(time.Hour)
	c, err := ParseSessionClaims(MintSessionAt("bob", iat, exp, testKey), testKey)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.IssuedAt != iat.Unix() {
		t.Errorf("IssuedAt = %d, want %d", c.IssuedAt, iat.Unix())
	}
	if c.ExpiresAt != exp.Unix() {
		t.Errorf("ExpiresAt = %d, want %d", c.ExpiresAt, exp.Unix())
	}
}

// A cookie minted before the issued-at field existed (legacy `subject.exp`
// layout) must still verify and parse, with IssuedAt == 0 — so browsers holding
// an old cookie are not force-logged-out on the upgrade.
func TestSessionLegacyTokenParses(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	payload := b64.EncodeToString([]byte("bob")) + "." + strconv.FormatInt(exp, 10)
	legacy := payload + "." + sign(payload, testKey)

	c, err := ParseSessionClaims(legacy, testKey)
	if err != nil {
		t.Fatalf("legacy parse: %v", err)
	}
	if c.Subject != "bob" {
		t.Errorf("subject = %q, want bob", c.Subject)
	}
	if c.IssuedAt != 0 {
		t.Errorf("IssuedAt = %d, want 0 (legacy)", c.IssuedAt)
	}
	if c.ExpiresAt != exp {
		t.Errorf("ExpiresAt = %d, want %d", c.ExpiresAt, exp)
	}
	// The legacy path also works through the subject-only helper.
	if s, err := ParseSession(legacy, testKey); err != nil || s != "bob" {
		t.Errorf("ParseSession(legacy) = %q, %v", s, err)
	}
}

// The signature covers the issued-at field, so tampering with it is rejected.
func TestSessionIssuedAtTamperDetected(t *testing.T) {
	tok := MintSessionAt("bob", time.Now().Add(-time.Hour), time.Now().Add(time.Hour), testKey)
	parts := strings.SplitN(tok, ".", 4) // subject.iat.exp.sig
	if len(parts) != 4 {
		t.Fatalf("unexpected token layout: %q", tok)
	}
	parts[1] = strconv.FormatInt(time.Now().Unix(), 10) // pretend it was just issued
	if _, err := ParseSession(strings.Join(parts, "."), testKey); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("tampered iat err = %v, want ErrSessionInvalid", err)
	}
}
