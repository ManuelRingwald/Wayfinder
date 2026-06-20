package store

import (
	"context"

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
