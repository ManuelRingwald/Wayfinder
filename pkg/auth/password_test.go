package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("unexpected hash format: %q", hash)
	}

	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatalf("verify correct = %v, %v, want true", ok, err)
	}

	ok, err = VerifyPassword(hash, "wrong password")
	if err != nil || ok {
		t.Fatalf("verify wrong = %v, %v, want false", ok, err)
	}
}

func TestHashIsSalted(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("two hashes of the same password are identical — salt not random")
	}
}

func TestVerifyInvalidHash(t *testing.T) {
	for _, bad := range []string{
		"",
		"not-a-hash",
		"$argon2id$v=19$m=65536,t=3,p=2$only-salt",
		"$bcrypt$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
	} {
		if _, err := VerifyPassword(bad, "x"); !errors.Is(err, ErrInvalidHash) {
			t.Errorf("VerifyPassword(%q) err = %v, want ErrInvalidHash", bad, err)
		}
	}
}
