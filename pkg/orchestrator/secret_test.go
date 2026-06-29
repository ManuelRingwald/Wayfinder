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

// fakeSecretWriter is an in-memory SecretWriter keyed by cred_ref (single feed).
type fakeSecretWriter struct {
	blobs map[string]string
}

func (f *fakeSecretWriter) Set(_ context.Context, _ int64, credRef, ciphertext string) error {
	f.blobs[credRef] = ciphertext
	return nil
}

func (f *fakeSecretWriter) Delete(_ context.Context, _ int64, credRef string) error {
	if _, ok := f.blobs[credRef]; !ok {
		return store.ErrNotFound
	}
	delete(f.blobs, credRef)
	return nil
}

func (f *fakeSecretWriter) ListRefs(_ context.Context, _ int64) ([]string, error) {
	refs := make([]string, 0, len(f.blobs))
	for ref := range f.blobs {
		refs = append(refs, ref)
	}
	return refs, nil
}

// The sealer stores ciphertext (never plaintext), and the resolver round-trips it
// back: the two halves of ORCH-2c share the same key and store.
func TestSecretSealerStoresCiphertextAndRoundTrips(t *testing.T) {
	ctx := context.Background()
	c := newCipher(t)
	w := &fakeSecretWriter{blobs: map[string]string{}}
	sealer := NewSecretSealer(w, c)

	if err := sealer.SetSecret(ctx, 1, "secret/sky", "opensky-secret"); err != nil {
		t.Fatalf("set secret: %v", err)
	}
	// What landed in the store must be ciphertext, not the plaintext.
	if blob := w.blobs["secret/sky"]; blob == "" || blob == "opensky-secret" {
		t.Fatalf("stored blob = %q, want sealed ciphertext", w.blobs["secret/sky"])
	}
	// The resolver (same key + store) recovers the plaintext.
	r := NewSecretResolver(fakeSecretStore{blobs: w.blobs}, c)
	got, err := r.Resolve(ctx, 1, "secret/sky")
	if err != nil || got != "opensky-secret" {
		t.Fatalf("resolve = %q, %v, want opensky-secret", got, err)
	}

	// ListSecretRefs reports the configured ref; Delete removes it.
	if refs, _ := sealer.ListSecretRefs(ctx, 1); len(refs) != 1 || refs[0] != "secret/sky" {
		t.Fatalf("list refs = %v, want [secret/sky]", refs)
	}
	if err := sealer.DeleteSecret(ctx, 1, "secret/sky"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := sealer.DeleteSecret(ctx, 1, "secret/sky"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("re-delete = %v, want ErrNotFound", err)
	}
}
