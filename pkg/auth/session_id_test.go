package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionIDRoundTrip(t *testing.T) {
	tok := "abc123_DEF-456" // dot-free base64url, like a real token
	cookie := MintSessionID(tok, testKey)
	got, err := ParseSessionID(cookie, testKey)
	if err != nil {
		t.Fatalf("ParseSessionID: %v", err)
	}
	if got != tok {
		t.Fatalf("token = %q, want %q", got, tok)
	}
}

func TestSessionIDTamperAndWrongKey(t *testing.T) {
	cookie := MintSessionID("tok", testKey)

	// Flipped signature.
	if _, err := ParseSessionID(cookie+"x", testKey); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("tampered sig = %v, want ErrSessionInvalid", err)
	}
	// Different key must not verify.
	if _, err := ParseSessionID(cookie, []byte("other-key")); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("wrong key = %v, want ErrSessionInvalid", err)
	}
	// No dot at all.
	if _, err := ParseSessionID("nodot", testKey); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("no separator = %v, want ErrSessionInvalid", err)
	}
}

// A legacy stateless cookie (subject.iat.exp.sig) has a dotted payload, so
// ParseSessionID must reject it — this is how the authenticator distinguishes the
// two cookie shapes and falls back to the legacy path.
func TestSessionIDRejectsLegacyCookie(t *testing.T) {
	legacy := MintSession("bob", time.Hour, testKey)
	if _, err := ParseSessionID(legacy, testKey); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("ParseSessionID(legacy) = %v, want ErrSessionInvalid", err)
	}
}

// fakeResolver is a table-driven SessionResolver for the registry authenticator.
type fakeResolver struct {
	subjects map[string]string // token -> subject; missing => ErrUnauthenticated
}

func (f fakeResolver) ResolveSession(_ context.Context, token string) (string, error) {
	if s, ok := f.subjects[token]; ok {
		return s, nil
	}
	return "", errors.New("not found")
}

func TestRegistryAuthenticator(t *testing.T) {
	res := fakeResolver{subjects: map[string]string{"live-token": "alice"}}
	a := BuiltinAuthenticator{CookieName: "wf_session", Key: testKey, Sessions: res}

	// A registry session-id cookie whose token resolves -> subject.
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "wf_session", Value: MintSessionID("live-token", testKey)})
	if subj, err := a.Authenticate(r); err != nil || subj != "alice" {
		t.Fatalf("valid session id = %q, %v; want alice", subj, err)
	}

	// A well-signed session-id cookie the registry rejects (revoked/expired/paused)
	// -> ErrUnauthenticated. This is the immediate-revocation guarantee.
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.AddCookie(&http.Cookie{Name: "wf_session", Value: MintSessionID("dead-token", testKey)})
	if _, err := a.Authenticate(r2); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("revoked session = %v, want ErrUnauthenticated", err)
	}

	// A legacy stateless cookie still authenticates on its signature (sanfte
	// Übernahme): the registry authenticator falls back when the cookie is not a
	// session id.
	r3 := httptest.NewRequest("GET", "/", nil)
	r3.AddCookie(&http.Cookie{Name: "wf_session", Value: MintSession("carol", time.Hour, testKey)})
	if subj, err := a.Authenticate(r3); err != nil || subj != "carol" {
		t.Fatalf("legacy fallback = %q, %v; want carol", subj, err)
	}

	// An expired legacy cookie is rejected (the fallback still checks expiry).
	r4 := httptest.NewRequest("GET", "/", nil)
	r4.AddCookie(&http.Cookie{Name: "wf_session", Value: MintSessionAt("dave", time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour), testKey)})
	if _, err := a.Authenticate(r4); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired legacy = %v, want ErrUnauthenticated", err)
	}
}
