package orchestrator

import (
	"context"

	"github.com/manuelringwald/wayfinder/pkg/secret"
)

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
	return r.cipher.Open(ciphertext)
}
