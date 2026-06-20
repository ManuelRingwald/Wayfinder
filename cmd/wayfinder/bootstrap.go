package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// bootstrapParams are the inputs for provisioning the first tenant + admin user
// of a fresh multi-tenant deployment (WF2-13). Without this there is no way to
// obtain a first login: proxy mode needs a user row to map the OIDC subject to a
// tenant, builtin mode needs a user *and* a password.
type bootstrapParams struct {
	TenantSlug string
	TenantName string     // defaults to TenantSlug when empty
	Subject    string     // OIDC subject (proxy) or username (builtin)
	Email      string     // optional
	Role       store.Role // defaults to tenant_admin
	Password   string     // optional; only stored as a builtin credential when set
}

// validate checks the required fields and the role without touching the
// database, so callers fail fast (and the check is unit-testable DB-free).
func (p bootstrapParams) validate() error {
	if p.TenantSlug == "" {
		return errors.New("missing -tenant")
	}
	if p.Subject == "" {
		return errors.New("missing -subject")
	}
	if !p.Role.Valid() {
		return fmt.Errorf("invalid -role %q (operator|tenant_admin|super_admin)", p.Role)
	}
	return nil
}

// runBootstrap provisions the tenant, user and (optionally) credential. It is
// idempotent: an existing tenant/user is reused, not duplicated, so the command
// is safe to re-run (e.g. to (re)set the admin password). Progress is written to
// out. A user that already exists under a *different* tenant is treated as a
// conflict rather than silently re-homed.
func runBootstrap(ctx context.Context, pool *pgxpool.Pool, p bootstrapParams, out io.Writer) error {
	if err := p.validate(); err != nil {
		return err
	}
	name := p.TenantName
	if name == "" {
		name = p.TenantSlug
	}

	tenants := store.NewTenantRepo(pool)
	users := store.NewUserRepo(pool)
	creds := store.NewCredentialRepo(pool)

	// Tenant: get-or-create by slug.
	t, err := tenants.GetBySlug(ctx, p.TenantSlug)
	switch {
	case errors.Is(err, store.ErrNotFound):
		if t, err = tenants.Create(ctx, p.TenantSlug, name); err != nil {
			return fmt.Errorf("create tenant: %w", err)
		}
		fmt.Fprintf(out, "created tenant %q (id=%d)\n", t.Slug, t.ID)
	case err != nil:
		return fmt.Errorf("look up tenant: %w", err)
	default:
		fmt.Fprintf(out, "tenant %q already exists (id=%d)\n", t.Slug, t.ID)
	}

	// User: get-or-create by subject.
	u, err := users.GetBySubject(ctx, p.Subject)
	switch {
	case errors.Is(err, store.ErrNotFound):
		var email *string
		if p.Email != "" {
			email = &p.Email
		}
		if u, err = users.Create(ctx, t.ID, p.Subject, email, p.Role); err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		fmt.Fprintf(out, "created user %q (id=%d, role=%s)\n", u.Subject, u.ID, u.Role)
	case err != nil:
		return fmt.Errorf("look up user: %w", err)
	default:
		if u.TenantID != t.ID {
			return fmt.Errorf("user %q already exists under a different tenant (id=%d); refusing to re-home", p.Subject, u.TenantID)
		}
		fmt.Fprintf(out, "user %q already exists (id=%d, role=%s)\n", u.Subject, u.ID, u.Role)
	}

	// Credential: only when a password is supplied (builtin mode). Set is an
	// upsert, so this also (re)sets the password of an existing user.
	if p.Password != "" {
		hash, err := auth.HashPassword(p.Password)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		if err := creds.Set(ctx, u.ID, hash); err != nil {
			return fmt.Errorf("set credential: %w", err)
		}
		fmt.Fprintf(out, "set builtin password for user %q\n", u.Subject)
	} else {
		fmt.Fprintf(out, "no -password given: user has no builtin credential "+
			"(fine for proxy mode; for builtin mode re-run with -password or WAYFINDER_BOOTSTRAP_PASSWORD)\n")
	}
	return nil
}

// bootstrapCommand is the `wayfinder bootstrap` entry point: it parses flags,
// opens the database (WAYFINDER_DB_URL), ensures the schema is migrated and runs
// runBootstrap. The password is read from -password or, preferably,
// WAYFINDER_BOOTSTRAP_PASSWORD (a flag value is visible in the process list).
func bootstrapCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	fs.SetOutput(out)
	var (
		p    bootstrapParams
		role string
	)
	fs.StringVar(&p.TenantSlug, "tenant", "", "tenant slug (required)")
	fs.StringVar(&p.TenantName, "tenant-name", "", "tenant display name (default: slug)")
	fs.StringVar(&p.Subject, "subject", "", "admin subject / username (required)")
	fs.StringVar(&p.Email, "email", "", "admin email (optional)")
	fs.StringVar(&role, "role", string(store.RoleTenantAdmin), "role: operator|tenant_admin|super_admin")
	fs.StringVar(&p.Password, "password", "", "builtin-mode password (prefer WAYFINDER_BOOTSTRAP_PASSWORD)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p.Role = store.Role(role)
	if p.Password == "" {
		p.Password = os.Getenv("WAYFINDER_BOOTSTRAP_PASSWORD")
	}

	dsn := os.Getenv("WAYFINDER_DB_URL")
	if dsn == "" {
		return errors.New("WAYFINDER_DB_URL must be set to bootstrap a multi-tenant deployment")
	}
	// Validate before opening a connection so obvious mistakes fail instantly.
	if err := p.validate(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := store.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer pool.Close()
	// Ensure the schema (incl. the credentials table) exists; bootstrap may be the
	// very first thing run against a new database.
	if err := store.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return runBootstrap(ctx, pool, p, out)
}
