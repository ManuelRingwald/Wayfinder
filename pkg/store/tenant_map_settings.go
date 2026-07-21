package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantMapSettingsRepo is a per-tenant key/value store for map-data overrides
// (Epic #307 hybrid, ADR 0035). It mirrors SettingsRepo but scopes every row to a
// tenant, so one tenant's override never touches another's. Values are opaque
// strings; the caller (pkg/mapconfig) owns the "tenant-override ?? global ?? env"
// resolution on top. Non-secret values only — sealed secrets keep the
// SettingsRepo + pkg/secret pattern.
type TenantMapSettingsRepo struct {
	db *pgxpool.Pool
}

// NewTenantMapSettingsRepo returns a repo backed by the given pool.
func NewTenantMapSettingsRepo(db *pgxpool.Pool) *TenantMapSettingsRepo {
	return &TenantMapSettingsRepo{db: db}
}

// Get returns the stored value for (tenantID, key), or ok=false when absent.
func (r *TenantMapSettingsRepo) Get(ctx context.Context, tenantID int64, key string) (string, bool, error) {
	const q = `SELECT value FROM tenant_map_settings WHERE tenant_id = $1 AND key = $2`
	var v string
	if err := r.db.QueryRow(ctx, q, tenantID, key).Scan(&v); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, wrap("get tenant map setting", err)
	}
	return v, true, nil
}

// Set upserts the value for (tenantID, key) (idempotent on the primary key).
func (r *TenantMapSettingsRepo) Set(ctx context.Context, tenantID int64, key, value string) error {
	const q = `INSERT INTO tenant_map_settings (tenant_id, key, value) VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`
	if _, err := r.db.Exec(ctx, q, tenantID, key, value); err != nil {
		return wrap("set tenant map setting", err)
	}
	return nil
}

// Delete removes an override. A missing row is not an error (idempotent clear →
// the setting falls back to the global/env value).
func (r *TenantMapSettingsRepo) Delete(ctx context.Context, tenantID int64, key string) error {
	const q = `DELETE FROM tenant_map_settings WHERE tenant_id = $1 AND key = $2`
	if _, err := r.db.Exec(ctx, q, tenantID, key); err != nil {
		return wrap("delete tenant map setting", err)
	}
	return nil
}
