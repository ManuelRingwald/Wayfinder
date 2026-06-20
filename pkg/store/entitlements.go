package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EntitlementRepo manages per-tenant feature flags as data (ADR 0005 §4): the
// basis for tenant.HasFeature(...) without coupling the ASD core to billing
// (ADR 0005 §4; billing stays a separate plane, WF2-51 dormant).
type EntitlementRepo struct {
	db *pgxpool.Pool
}

// NewEntitlementRepo returns an EntitlementRepo backed by the given pool.
func NewEntitlementRepo(db *pgxpool.Pool) *EntitlementRepo { return &EntitlementRepo{db: db} }

// Set enables or disables a feature for a tenant (upsert).
func (r *EntitlementRepo) Set(ctx context.Context, tenantID int64, featureKey string, enabled bool) error {
	const q = `INSERT INTO entitlements (tenant_id, feature_key, enabled) VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, feature_key) DO UPDATE SET enabled = EXCLUDED.enabled`
	if _, err := r.db.Exec(ctx, q, tenantID, featureKey, enabled); err != nil {
		return wrap("set entitlement", err)
	}
	return nil
}

// IsEnabled reports whether a feature is enabled for a tenant. An absent row
// means "not enabled" (default-deny), so a missing entitlement is not an error.
func (r *EntitlementRepo) IsEnabled(ctx context.Context, tenantID int64, featureKey string) (bool, error) {
	const q = `SELECT enabled FROM entitlements WHERE tenant_id = $1 AND feature_key = $2`
	var enabled bool
	err := r.db.QueryRow(ctx, q, tenantID, featureKey).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, wrap("is enabled", err)
	}
	return enabled, nil
}

// ListByTenant returns all feature flags set for a tenant as a key->enabled map.
func (r *EntitlementRepo) ListByTenant(ctx context.Context, tenantID int64) (map[string]bool, error) {
	const q = `SELECT feature_key, enabled FROM entitlements WHERE tenant_id = $1`
	rows, err := r.db.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrap("list entitlements", err)
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var (
			key     string
			enabled bool
		)
		if err := rows.Scan(&key, &enabled); err != nil {
			return nil, wrap("scan entitlement", err)
		}
		out[key] = enabled
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("iterate entitlements", err)
	}
	return out, nil
}
