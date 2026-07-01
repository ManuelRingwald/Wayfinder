package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// userColumns is the column list shared by every user query.
const userColumns = `id, tenant_id, subject, email, role, status, must_change_password, session_limit, created_at`

// UserRepo provides access to the users table.
type UserRepo struct {
	db *pgxpool.Pool
}

// NewUserRepo returns a UserRepo backed by the given pool.
func NewUserRepo(db *pgxpool.Pool) *UserRepo { return &UserRepo{db: db} }

// Create inserts a tenant user (role 'user') under the given tenant. Platform
// admins are created via CreateAdmin instead — the two are strictly separated
// (ONB-3, ADR 0011), so this constructor never sets role 'admin'. A nil email
// stores SQL NULL. A duplicate subject is rejected by the UNIQUE constraint; a
// missing/zero tenant is rejected by the FK and the role/tenant CHECK constraint.
func (r *UserRepo) Create(ctx context.Context, tenantID int64, subject string, email *string) (User, error) {
	const q = `INSERT INTO users (tenant_id, subject, email, role) VALUES ($1, $2, $3, 'user') RETURNING ` + userColumns
	u, err := scanUser(r.db.QueryRow(ctx, q, tenantID, subject, email))
	if err != nil {
		return User{}, wrap("create user", err)
	}
	return u, nil
}

// CreateAdmin inserts a platform admin (role 'admin', no tenant). Admins are
// global — they belong to no tenant (ONB-3, ADR 0011) — so tenant_id is stored as
// NULL and reads back as TenantID 0. A nil email stores SQL NULL. A duplicate
// subject is rejected by the UNIQUE constraint.
func (r *UserRepo) CreateAdmin(ctx context.Context, subject string, email *string) (User, error) {
	const q = `INSERT INTO users (tenant_id, subject, email, role) VALUES (NULL, $1, $2, 'admin') RETURNING ` + userColumns
	u, err := scanUser(r.db.QueryRow(ctx, q, subject, email))
	if err != nil {
		return User{}, wrap("create admin", err)
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

// ListAdmins returns every platform admin (role 'admin'), ordered by id. Admins
// have no tenant, so unlike ListByTenant this is a global list — it backs the
// /api/admin/admins management surface (ONB-3, ADR 0011).
func (r *UserRepo) ListAdmins(ctx context.Context) ([]User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE role = 'admin' ORDER BY id`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, wrap("list admins", err)
	}
	defer rows.Close()

	var admins []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, wrap("scan admin", err)
		}
		admins = append(admins, u)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("iterate admins", err)
	}
	return admins, nil
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

// SetMustChangePassword sets or clears a user's forced-password-change flag
// (ONB-1, ADR 0011). It is set on the auto-seeded default admin and cleared the
// moment that admin changes its own password. A missing user yields ErrNotFound.
func (r *UserRepo) SetMustChangePassword(ctx context.Context, id int64, must bool) error {
	const q = `UPDATE users SET must_change_password = $2 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, must)
	if err != nil {
		return wrap("set must_change_password", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetSessionLimit sets or clears a user's per-access concurrent-session limit
// (AP7, ADR 0009 §5). A nil limit stores SQL NULL (fall back to the deployment
// default); a non-negative value overrides it (0 == unlimited). A negative value
// is rejected before the query (fail-closed). A missing user yields ErrNotFound.
func (r *UserRepo) SetSessionLimit(ctx context.Context, id int64, limit *int) error {
	if limit != nil && *limit < 0 {
		return fmt.Errorf("store: set session limit: must be non-negative, got %d", *limit)
	}
	const q = `UPDATE users SET session_limit = $2 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, limit)
	if err != nil {
		return wrap("set session limit", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountActiveAdmins returns the number of users with role 'admin' and status
// 'active'. It backs two invariants (ONB-1, ADR 0011): the boot auto-seed only
// provisions the default admin when this is zero, and the "last active admin"
// guard refuses to delete/pause the final admin (no self-lockout).
func (r *UserRepo) CountActiveAdmins(ctx context.Context) (int, error) {
	const q = `SELECT count(*) FROM users WHERE role = 'admin' AND status = 'active'`
	var n int
	if err := r.db.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, wrap("count active admins", err)
	}
	return n, nil
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
// named types do not depend on pgx's type map. tenant_id is nullable (a platform
// admin has none, ONB-3): a SQL NULL maps to TenantID 0, the in-process sentinel
// for "no tenant".
func scanUser(row rowScanner) (User, error) {
	var (
		u        User
		tenantID *int64
		role     string
		status   string
	)
	// session_limit is nullable (nil == fall back to the deployment default),
	// scanned straight into the *int field like tenant_id above.
	if err := row.Scan(&u.ID, &tenantID, &u.Subject, &u.Email, &role, &status, &u.MustChangePassword, &u.SessionLimit, &u.CreatedAt); err != nil {
		return User{}, err
	}
	if tenantID != nil {
		u.TenantID = *tenantID
	}
	u.Role = Role(role)
	u.Status = Status(status)
	return u, nil
}
