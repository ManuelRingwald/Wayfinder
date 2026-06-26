package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// tenantColumns is the column list shared by every tenant query, kept in one
// place so the SELECTs and the scan helper stay in lock-step.
const tenantColumns = `id, slug, name, status, created_at`

// TenantRepo provides access to the tenants table.
type TenantRepo struct {
	db *pgxpool.Pool
}

// NewTenantRepo returns a TenantRepo backed by the given pool.
func NewTenantRepo(db *pgxpool.Pool) *TenantRepo { return &TenantRepo{db: db} }

// Create inserts a tenant and returns the stored row (id/status/created_at are
// filled by the database). A duplicate slug is rejected by the UNIQUE constraint.
func (r *TenantRepo) Create(ctx context.Context, slug, name string) (Tenant, error) {
	const q = `INSERT INTO tenants (slug, name) VALUES ($1, $2) RETURNING ` + tenantColumns
	t, err := scanTenant(r.db.QueryRow(ctx, q, slug, name))
	if err != nil {
		return Tenant{}, wrap("create tenant", err)
	}
	return t, nil
}

// GetByID returns the tenant with the given id, or ErrNotFound.
func (r *TenantRepo) GetByID(ctx context.Context, id int64) (Tenant, error) {
	const q = `SELECT ` + tenantColumns + ` FROM tenants WHERE id = $1`
	t, err := scanTenant(r.db.QueryRow(ctx, q, id))
	if err != nil {
		return Tenant{}, wrap("get tenant by id", err)
	}
	return t, nil
}

// GetBySlug returns the tenant with the given slug, or ErrNotFound.
func (r *TenantRepo) GetBySlug(ctx context.Context, slug string) (Tenant, error) {
	const q = `SELECT ` + tenantColumns + ` FROM tenants WHERE slug = $1`
	t, err := scanTenant(r.db.QueryRow(ctx, q, slug))
	if err != nil {
		return Tenant{}, wrap("get tenant by slug", err)
	}
	return t, nil
}

// SetStatus updates a tenant's lifecycle status (AP6). A paused tenant cascades
// to login for all of its accounts (enforced at the login edge). The status is
// validated before the query (fail-closed). A missing tenant yields ErrNotFound.
func (r *TenantRepo) SetStatus(ctx context.Context, id int64, status Status) error {
	if !status.Valid() {
		return fmt.Errorf("store: set tenant status: invalid status %q", status)
	}
	const q = `UPDATE tenants SET status = $2 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, string(status))
	if err != nil {
		return wrap("set tenant status", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a tenant and, by ON DELETE CASCADE, every row that references
// it: its users (and their credentials), feed subscriptions, entitlements and
// view configs (ONB-4, ADR 0011). The cascade is atomic (one DELETE). A missing
// tenant yields ErrNotFound. The caller is responsible for any higher-level guard
// (e.g. refusing to delete a tenant that still has accounts).
func (r *TenantRepo) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM tenants WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return wrap("delete tenant", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOpenAIPKey returns the tenant's per-tenant OpenAIP API key (ONB-6, ADR 0011),
// or nil when none is set — in which case the caller falls back to the global key.
// The key is read in isolation (not part of the shared Tenant row) so a secret is
// never carried through the general tenant DTOs. A missing tenant yields ErrNotFound.
func (r *TenantRepo) GetOpenAIPKey(ctx context.Context, id int64) (*string, error) {
	const q = `SELECT openaip_api_key FROM tenants WHERE id = $1`
	var key *string
	if err := r.db.QueryRow(ctx, q, id).Scan(&key); err != nil {
		return nil, wrap("get tenant openaip key", err)
	}
	return key, nil
}

// SetOpenAIPKey sets (non-nil) or clears (nil) the tenant's OpenAIP API key. A nil
// key restores the global-key fallback. A missing tenant yields ErrNotFound.
func (r *TenantRepo) SetOpenAIPKey(ctx context.Context, id int64, key *string) error {
	const q = `UPDATE tenants SET openaip_api_key = $2 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, key)
	if err != nil {
		return wrap("set tenant openaip key", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns all tenants ordered by id.
func (r *TenantRepo) List(ctx context.Context) ([]Tenant, error) {
	const q = `SELECT ` + tenantColumns + ` FROM tenants ORDER BY id`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, wrap("list tenants", err)
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, wrap("scan tenant", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("iterate tenants", err)
	}
	return tenants, nil
}

func scanTenant(row rowScanner) (Tenant, error) {
	var t Tenant
	err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.Status, &t.CreatedAt)
	return t, err
}
