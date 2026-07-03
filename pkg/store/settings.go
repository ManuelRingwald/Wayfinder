package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SettingsRepo is a small key/value store for platform-wide settings managed at
// runtime (AERO-2, ADR 0018). Values are opaque strings — the caller decides the
// encoding (e.g. a sealed secret blob for the global OpenAIP key). It stays free of
// crypto: sealing/opening happens in the caller (cmd/wayfinder), so the store never
// holds a plaintext secret or the cipher key.
type SettingsRepo struct {
	db *pgxpool.Pool
}

// NewSettingsRepo returns a SettingsRepo backed by the given pool.
func NewSettingsRepo(db *pgxpool.Pool) *SettingsRepo { return &SettingsRepo{db: db} }

// Get returns the stored value for key, or ok=false when there is no row.
func (r *SettingsRepo) Get(ctx context.Context, key string) (string, bool, error) {
	const q = `SELECT value FROM platform_settings WHERE key = $1`
	var v string
	if err := r.db.QueryRow(ctx, q, key).Scan(&v); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, wrap("get platform setting", err)
	}
	return v, true, nil
}

// Set upserts the value for key (idempotent on the primary key).
func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	const q = `INSERT INTO platform_settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`
	if _, err := r.db.Exec(ctx, q, key, value); err != nil {
		return wrap("set platform setting", err)
	}
	return nil
}

// Delete removes a setting. A missing key is not an error (idempotent clear).
func (r *SettingsRepo) Delete(ctx context.Context, key string) error {
	const q = `DELETE FROM platform_settings WHERE key = $1`
	if _, err := r.db.Exec(ctx, q, key); err != nil {
		return wrap("delete platform setting", err)
	}
	return nil
}
