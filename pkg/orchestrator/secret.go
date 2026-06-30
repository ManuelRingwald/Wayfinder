package orchestrator

import (
	"context"
	"strconv"

	"github.com/manuelringwald/wayfinder/pkg/secret"
)

// credAAD binds a sealed secret to its (feed_id, cred_ref) identity via AES-GCM
// Additional Authenticated Data (NFR-SEC-004, defense-in-depth). The encoding is
// unambiguous: the feed id is decimal (never contains the NUL separator), so no
// cred_ref can collide with a different (feed_id, cred_ref) pair. Seal and Open
// must derive it identically; a relocated/replayed blob under a different identity
// fails to decrypt.
func credAAD(feedID int64, credRef string) []byte {
	return []byte(strconv.FormatInt(feedID, 10) + "\x00" + credRef)
}

// SecretReader reads a feed's stored (encrypted) credential blob by reference
// (satisfied by *store.SecretRepo). It returns store.ErrNotFound when the ref has
// no secret configured.
type SecretReader interface {
	Get(ctx context.Context, feedID int64, credRef string) (ciphertext string, err error)
}

// SecretResolver turns a per-feed cred_ref into its plaintext credential by
// reading the stored ciphertext and decrypting it with the deployment key
// (ORCH-2c, ADR 0012 §6). It lives in the orchestrator control plane — the only
// component that both holds the key at launch and injects the value into a
// spawned tracker container. The plaintext is never persisted, never returned to
// the browser and never logged.
type SecretResolver struct {
	secrets SecretReader
	cipher  *secret.Cipher
}

// NewSecretResolver wires the resolver over the secret store and cipher.
func NewSecretResolver(secrets SecretReader, cipher *secret.Cipher) *SecretResolver {
	return &SecretResolver{secrets: secrets, cipher: cipher}
}

// Resolve returns the plaintext credential for a feed's cred_ref. A missing ref
// surfaces the store's ErrNotFound; a tampered/wrong-key blob surfaces
// secret.ErrDecrypt.
func (r *SecretResolver) Resolve(ctx context.Context, feedID int64, credRef string) (string, error) {
	ciphertext, err := r.secrets.Get(ctx, feedID, credRef)
	if err != nil {
		return "", err
	}
	return r.cipher.Open(ciphertext, credAAD(feedID, credRef))
}

// SecretWriter persists (and removes) a feed's encrypted credential blobs by
// reference, and lists which refs are configured (satisfied by *store.SecretRepo).
// It stores only opaque ciphertext — the key never reaches the persistence layer.
type SecretWriter interface {
	Set(ctx context.Context, feedID int64, credRef, ciphertext string) error
	Delete(ctx context.Context, feedID int64, credRef string) error
	ListRefs(ctx context.Context, feedID int64) ([]string, error)
}

// SecretSealer is the write-side counterpart of SecretResolver (ORCH-2c, ADR 0012
// §6). It seals a plaintext credential with the deployment key and stores the
// resulting ciphertext; it lives in the browser-facing server, the only component
// that accepts an operator-supplied value. The plaintext is sealed immediately and
// never persisted in the clear, never returned to the browser and never logged.
// Reads of the value happen only in the orchestrator control plane (SecretResolver).
type SecretSealer struct {
	secrets SecretWriter
	cipher  *secret.Cipher
}

// NewSecretSealer wires the sealer over the secret store and cipher.
func NewSecretSealer(secrets SecretWriter, cipher *secret.Cipher) *SecretSealer {
	return &SecretSealer{secrets: secrets, cipher: cipher}
}

// SetSecret seals the plaintext credential for a feed's cred_ref and stores it
// (idempotent upsert).
func (s *SecretSealer) SetSecret(ctx context.Context, feedID int64, credRef, plaintext string) error {
	blob, err := s.cipher.Seal(plaintext, credAAD(feedID, credRef))
	if err != nil {
		return err
	}
	return s.secrets.Set(ctx, feedID, credRef, blob)
}

// DeleteSecret removes a feed's cred_ref secret; a missing ref surfaces the
// store's ErrNotFound.
func (s *SecretSealer) DeleteSecret(ctx context.Context, feedID int64, credRef string) error {
	return s.secrets.Delete(ctx, feedID, credRef)
}

// ListSecretRefs returns the cred_refs that have a stored secret for the feed
// (the values are never exposed — only which refs are configured).
func (s *SecretSealer) ListSecretRefs(ctx context.Context, feedID int64) ([]string, error) {
	return s.secrets.ListRefs(ctx, feedID)
}
