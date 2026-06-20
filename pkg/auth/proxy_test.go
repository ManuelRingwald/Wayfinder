package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

const (
	testIssuer   = "https://issuer.example"
	testAudience = "wayfinder"
	testKID      = "k1"
)

// newProxyAuth builds a ProxyAuthenticator whose verifier trusts pub via a local
// JWKS endpoint (no network/discovery), exercising the real go-oidc path.
func newProxyAuth(t *testing.T, pub *rsa.PublicKey) *ProxyAuthenticator {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON(pub, testKID))
	}))
	t.Cleanup(srv.Close)
	ks := oidc.NewRemoteKeySet(context.Background(), srv.URL)
	v := oidc.NewVerifier(testIssuer, ks, &oidc.Config{ClientID: testAudience})
	return &ProxyAuthenticator{verifier: v}
}

func jwksJSON(pub *rsa.PublicKey, kid string) []byte {
	b, _ := json.Marshal(map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"kid": kid,
			"alg": "RS256",
			"use": "sig",
			"n":   b64.EncodeToString(pub.N.Bytes()),
			"e":   b64.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	})
	return b
}

func mintJWT(t *testing.T, priv *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid})
	payload, _ := json.Marshal(claims)
	signingInput := b64.EncodeToString(header) + "." + b64.EncodeToString(payload)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signingInput + "." + b64.EncodeToString(sig)
}

func bearerReq(token string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func validClaims() map[string]any {
	return map[string]any{
		"iss": testIssuer,
		"aud": testAudience,
		"sub": "oidc|carol",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
}

func TestProxyAuthenticatorValid(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := newProxyAuth(t, &priv.PublicKey)

	subject, err := a.Authenticate(bearerReq(mintJWT(t, priv, testKID, validClaims())))
	if err != nil || subject != "oidc|carol" {
		t.Fatalf("valid token = %q, %v, want oidc|carol", subject, err)
	}
}

func TestProxyAuthenticatorRejects(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048) // not in the JWKS
	a := newProxyAuth(t, &priv.PublicKey)

	expired := validClaims()
	expired["exp"] = time.Now().Add(-time.Minute).Unix()

	wrongAud := validClaims()
	wrongAud["aud"] = "someone-else"

	wrongIss := validClaims()
	wrongIss["iss"] = "https://evil.example"

	noSub := validClaims()
	delete(noSub, "sub")

	cases := map[string]string{
		"missing token":  "",
		"not a jwt":      "garbage",
		"expired":        mintJWT(t, priv, testKID, expired),
		"wrong audience": mintJWT(t, priv, testKID, wrongAud),
		"wrong issuer":   mintJWT(t, priv, testKID, wrongIss),
		"bad signature":  mintJWT(t, other, testKID, validClaims()), // signed by an untrusted key
		"empty subject":  mintJWT(t, priv, testKID, noSub),
	}
	for name, token := range cases {
		if _, err := a.Authenticate(bearerReq(token)); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("%s: err = %v, want ErrUnauthenticated", name, err)
		}
	}
}

func TestBearerToken(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer abc.def.ghi")
	if got := bearerToken(r); got != "abc.def.ghi" {
		t.Errorf("bearerToken = %q", got)
	}
	// Non-bearer / missing -> empty.
	for _, h := range []string{"", "Basic abc", "Bearer", "bearer"} {
		r := httptest.NewRequest("GET", "/", nil)
		if h != "" {
			r.Header.Set("Authorization", h)
		}
		if got := bearerToken(r); got != "" {
			t.Errorf("bearerToken(%q) = %q, want empty", h, got)
		}
	}
}
