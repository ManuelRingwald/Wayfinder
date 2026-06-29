package orchestrator

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/secret"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// fakeSecretStore is an in-memory SecretReader keyed by (feedID, credRef).
type fakeSecretStore struct {
	blobs map[string]string
}

func key(feedID int64, ref string) string { return ref } // single feed in tests

func (f fakeSecretStore) Get(_ context.Context, feedID int64, credRef string) (string, error) {
	if ct, ok := f.blobs[key(feedID, credRef)]; ok {
		return ct, nil
	}
	return "", store.ErrNotFound
}

func newCipher(t *testing.T) *secret.Cipher {
	t.Helper()
	k := make([]byte, secret.KeySize)
	_, _ = io.ReadFull(rand.Reader, k)
	c, err := secret.NewCipher(k)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return c
}

func TestSecretResolverResolves(t *testing.T) {
	ctx := context.Background()
	c := newCipher(t)
	blob, _ := c.Seal("opensky-secret")
	store := fakeSecretStore{blobs: map[string]string{"secret/sky": blob}}
	r := NewSecretResolver(store, c)

	got, err := r.Resolve(ctx, 1, "secret/sky")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "opensky-secret" {
		t.Fatalf("resolved = %q, want opensky-secret", got)
	}
}

func TestSecretResolverMissingRef(t *testing.T) {
	r := NewSecretResolver(fakeSecretStore{blobs: map[string]string{}}, newCipher(t))
	if _, err := r.Resolve(context.Background(), 1, "secret/nope"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("resolve(missing) = %v, want ErrNotFound", err)
	}
}

func TestSecretResolverWrongKeyFails(t *testing.T) {
	// A blob sealed with a different key cannot be opened → ErrDecrypt.
	other := newCipher(t)
	blob, _ := other.Seal("x")
	r := NewSecretResolver(fakeSecretStore{blobs: map[string]string{"r": blob}}, newCipher(t))
	if _, err := r.Resolve(context.Background(), 1, "r"); !errors.Is(err, secret.ErrDecrypt) {
		t.Fatalf("resolve(wrong key) = %v, want ErrDecrypt", err)
	}
}
