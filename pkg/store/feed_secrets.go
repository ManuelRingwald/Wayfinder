package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SecretRepo persists per-feed source credentials as opaque ciphertext (ORCH-2c,
// ADR 0012 §6). It is deliberately crypto-agnostic: callers seal/open the value
// with pkg/secret and store only the resulting blob here, so the key never
// touches the persistence layer. A secret is identified by (feed_id, cred_ref);
// the value is never exposed by the admin API (only its presence), and is read
// back only by the orchestrator control plane at launch.
type SecretRepo struct {
	db *pgxpool.Pool
}

// NewSecretRepo returns a SecretRepo backed by the given pool.
func NewSecretRepo(db *pgxpool.Pool) *SecretRepo { return &SecretRepo{db: db} }

// Set stores (or replaces) the ciphertext for a feed's cred_ref. Idempotent
// upsert on the (feed_id, cred_ref) key.
func (r *SecretRepo) Set(ctx context.Context, feedID int64, credRef, ciphertext string) error {
	const q = `INSERT INTO feed_secrets (feed_id, cred_ref, ciphertext)
		VALUES ($1, $2, $3)
		ON CONFLICT (feed_id, cred_ref)
		DO UPDATE SET ciphertext = EXCLUDED.ciphertext, updated_at = now()`
	if _, err := r.db.Exec(ctx, q, feedID, credRef, ciphertext); err != nil {
		return wrap("set feed secret", err)
	}
	return nil
}

// Get returns the stored ciphertext for a feed's cred_ref, or ErrNotFound.
func (r *SecretRepo) Get(ctx context.Context, feedID int64, credRef string) (string, error) {
	const q = `SELECT ciphertext FROM feed_secrets WHERE feed_id = $1 AND cred_ref = $2`
	var ct string
	if err := r.db.QueryRow(ctx, q, feedID, credRef).Scan(&ct); err != nil {
		return "", wrap("get feed secret", err)
	}
	return ct, nil
}

// Delete removes a feed's cred_ref secret. A missing row yields ErrNotFound.
func (r *SecretRepo) Delete(ctx context.Context, feedID int64, credRef string) error {
	const q = `DELETE FROM feed_secrets WHERE feed_id = $1 AND cred_ref = $2`
	tag, err := r.db.Exec(ctx, q, feedID, credRef)
	if err != nil {
		return wrap("delete feed secret", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListRefs returns the cred_refs that have a stored secret for the feed, ordered.
// Used by the admin API to report which references are configured (never the
// values).
func (r *SecretRepo) ListRefs(ctx context.Context, feedID int64) ([]string, error) {
	const q = `SELECT cred_ref FROM feed_secrets WHERE feed_id = $1 ORDER BY cred_ref`
	rows, err := r.db.Query(ctx, q, feedID)
	if err != nil {
		return nil, wrap("list feed secret refs", err)
	}
	defer rows.Close()

	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, wrap("scan secret ref", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("iterate secret refs", err)
	}
	return refs, nil
}
