package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseMode(t *testing.T) {
	cases := map[string]struct {
		want Mode
		ok   bool
	}{
		"proxy":   {ModeProxy, true},
		"builtin": {ModeBuiltin, true},
		"PROXY":   {ModeProxy, true},    // case-insensitive
		" proxy ": {ModeProxy, true},    // trimmed
		"":        {ModeBuiltin, false}, // fallback to builtin (ADR 0014)
		"none":    {ModeBuiltin, false}, // removed mode -> fallback
		"banana":  {ModeBuiltin, false}, // fallback
	}
	for in, want := range cases {
		got, ok := ParseMode(in)
		if got != want.want || ok != want.ok {
			t.Errorf("ParseMode(%q) = %q,%v; want %q,%v", in, got, ok, want.want, want.ok)
		}
	}
}

func TestBuiltinAuthenticator(t *testing.T) {
	a := BuiltinAuthenticator{CookieName: "wf_session", Key: testKey}

	// Valid cookie -> subject.
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "wf_session", Value: MintSession("bob", time.Hour, testKey)})
	subject, err := a.Authenticate(r)
	if err != nil || subject != "bob" {
		t.Fatalf("valid cookie = %q, %v", subject, err)
	}

	// Missing cookie -> ErrUnauthenticated.
	if _, err := a.Authenticate(httptest.NewRequest("GET", "/", nil)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("missing cookie err = %v, want ErrUnauthenticated", err)
	}

	// Invalid cookie value -> ErrUnauthenticated.
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.AddCookie(&http.Cookie{Name: "wf_session", Value: "tampered"})
	if _, err := a.Authenticate(r2); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("invalid cookie err = %v, want ErrUnauthenticated", err)
	}
}
