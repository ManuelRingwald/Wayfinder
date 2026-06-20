package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CredentialRepo stores builtin-mode password hashes (argon2id PHC strings),
// keyed by user. Only users who authenticate with a local password have a row;
// OIDC/proxy users have none (ADR 0006 §5). Hashing/verification live in
// pkg/auth — this repo only persists and retrieves the opaque hash.
type CredentialRepo struct {
	db *pgxpool.Pool
}

// NewCredentialRepo returns a CredentialRepo backed by the given pool.
func NewCredentialRepo(db *pgxpool.Pool) *CredentialRepo { return &CredentialRepo{db: db} }

// Set stores (or replaces) a user's password hash.
func (r *CredentialRepo) Set(ctx context.Context, userID int64, passwordHash string) error {
	const q = `INSERT INTO credentials (user_id, password_hash) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = now()`
	if _, err := r.db.Exec(ctx, q, userID, passwordHash); err != nil {
		return wrap("set credential", err)
	}
	return nil
}

// GetHash returns a user's stored password hash, or ErrNotFound if the user has
// no local credential (e.g. an OIDC-only user).
func (r *CredentialRepo) GetHash(ctx context.Context, userID int64) (string, error) {
	const q = `SELECT password_hash FROM credentials WHERE user_id = $1`
	var hash string
	err := r.db.QueryRow(ctx, q, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", wrap("get credential", err)
	}
	return hash, nil
}
