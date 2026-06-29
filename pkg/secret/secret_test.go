package secret

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"testing"
)

func newKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("read key: %v", err)
	}
	return key
}

func TestSealOpenRoundTrip(t *testing.T) {
	c, err := NewCipher(newKey(t))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	plain := "opensky-client-secret-xyz"
	blob, err := c.Seal(plain)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if blob == plain {
		t.Fatal("sealed blob must not equal plaintext")
	}
	got, err := c.Open(blob)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got != plain {
		t.Fatalf("round-trip = %q, want %q", got, plain)
	}
}

func TestSealIsNonDeterministic(t *testing.T) {
	c, _ := NewCipher(newKey(t))
	a, _ := c.Seal("same")
	b, _ := c.Seal("same")
	if a == b {
		t.Fatal("two seals of the same plaintext must differ (random nonce)")
	}
}

func TestOpenWithWrongKeyFails(t *testing.T) {
	c1, _ := NewCipher(newKey(t))
	c2, _ := NewCipher(newKey(t))
	blob, _ := c1.Seal("secret")
	if _, err := c2.Open(blob); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("open with wrong key = %v, want ErrDecrypt", err)
	}
}

func TestOpenTamperedFails(t *testing.T) {
	c, _ := NewCipher(newKey(t))
	blob, _ := c.Seal("secret")
	raw, _ := base64.StdEncoding.DecodeString(blob)
	raw[len(raw)-1] ^= 0xFF // flip a tag bit
	tampered := base64.StdEncoding.EncodeToString(raw)
	if _, err := c.Open(tampered); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("open tampered = %v, want ErrDecrypt", err)
	}
}

func TestOpenGarbageFails(t *testing.T) {
	c, _ := NewCipher(newKey(t))
	for _, blob := range []string{"", "not-base64!!", "AAAA"} {
		if _, err := c.Open(blob); !errors.Is(err, ErrDecrypt) {
			t.Errorf("open(%q) = %v, want ErrDecrypt", blob, err)
		}
	}
}

func TestNewCipherRejectsBadKeyLength(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33} {
		if _, err := NewCipher(make([]byte, n)); err == nil {
			t.Errorf("NewCipher with %d-byte key should fail", n)
		}
	}
}

func TestKeyFromBase64(t *testing.T) {
	key := newKey(t)
	enc := base64.StdEncoding.EncodeToString(key)
	got, err := KeyFromBase64(enc)
	if err != nil {
		t.Fatalf("KeyFromBase64: %v", err)
	}
	if string(got) != string(key) {
		t.Fatal("key did not round-trip")
	}
	// Invalid base64 and wrong length are rejected.
	if _, err := KeyFromBase64("!!!"); err == nil {
		t.Error("invalid base64 should fail")
	}
	if _, err := KeyFromBase64(base64.StdEncoding.EncodeToString(make([]byte, 16))); err == nil {
		t.Error("16-byte key should fail")
	}
}
