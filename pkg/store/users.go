package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// userColumns is the column list shared by every user query.
const userColumns = `id, tenant_id, subject, email, role, status, created_at`

// UserRepo provides access to the users table.
type UserRepo struct {
	db *pgxpool.Pool
}

// NewUserRepo returns a UserRepo backed by the given pool.
func NewUserRepo(db *pgxpool.Pool) *UserRepo { return &UserRepo{db: db} }

// Create inserts a user under the given tenant. The role is validated before the
// query (fail-closed: an unknown role never reaches the database). A nil email
// stores SQL NULL. A duplicate subject is rejected by the UNIQUE constraint.
func (r *UserRepo) Create(ctx context.Context, tenantID int64, subject string, email *string, role Role) (User, error) {
	if !role.Valid() {
		return User{}, fmt.Errorf("store: create user: invalid role %q", role)
	}
	const q = `INSERT INTO users (tenant_id, subject, email, role) VALUES ($1, $2, $3, $4) RETURNING ` + userColumns
	u, err := scanUser(r.db.QueryRow(ctx, q, tenantID, subject, email, string(role)))
	if err != nil {
		return User{}, wrap("create user", err)
	}
	return u, nil
}

// GetBySubject resolves an authenticated identity (OIDC subject / username) to
// its user, or ErrNotFound. This is the lookup WF2-11/12 use to derive the
// tenant context from a request.
func (r *UserRepo) GetBySubject(ctx context.Context, subject string) (User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE subject = $1`
	u, err := scanUser(r.db.QueryRow(ctx, q, subject))
	if err != nil {
		return User{}, wrap("get user by subject", err)
	}
	return u, nil
}

// GetByID returns the user with the given id, or ErrNotFound.
func (r *UserRepo) GetByID(ctx context.Context, id int64) (User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	u, err := scanUser(r.db.QueryRow(ctx, q, id))
	if err != nil {
		return User{}, wrap("get user by id", err)
	}
	return u, nil
}

// ListByTenant returns all users of a tenant, ordered by id.
func (r *UserRepo) ListByTenant(ctx context.Context, tenantID int64) ([]User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE tenant_id = $1 ORDER BY id`
	rows, err := r.db.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrap("list users", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, wrap("scan user", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("iterate users", err)
	}
	return users, nil
}

// SetStatus updates a user's lifecycle status (AP6). The status is validated
// before the query (fail-closed: an unknown status never reaches the database).
// A missing user yields ErrNotFound so callers can return 404.
func (r *UserRepo) SetStatus(ctx context.Context, id int64, status Status) error {
	if !status.Valid() {
		return fmt.Errorf("store: set user status: invalid status %q", status)
	}
	const q = `UPDATE users SET status = $2 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, string(status))
	if err != nil {
		return wrap("set user status", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a user. Dependent rows (credentials, per-user view overrides)
// are cleared by ON DELETE CASCADE. A missing user yields ErrNotFound.
func (r *UserRepo) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM users WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return wrap("delete user", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// scanUser reads a user row. role and status are scanned through strings so the
// named types do not depend on pgx's type map.
func scanUser(row rowScanner) (User, error) {
	var (
		u      User
		role   string
		status string
	)
	if err := row.Scan(&u.ID, &u.TenantID, &u.Subject, &u.Email, &role, &status, &u.CreatedAt); err != nil {
		return User{}, err
	}
	u.Role = Role(role)
	u.Status = Status(status)
	return u, nil
}
