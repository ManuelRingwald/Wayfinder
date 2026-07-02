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
	Role       store.Role // defaults to admin
	Password   string     // optional; only stored as a builtin credential when set
}

// validate checks the required fields and the role without touching the
// database, so callers fail fast (and the check is unit-testable DB-free).
// A tenant is required only for a user: a platform admin has no tenant (ONB-3,
// ADR 0011), so -tenant is not required (and is ignored) for role admin.
func (p bootstrapParams) validate() error {
	if p.Subject == "" {
		return errors.New("missing -subject")
	}
	if !p.Role.Valid() {
		return fmt.Errorf("invalid -role %q (user|admin)", p.Role)
	}
	if p.Role == store.RoleUser && p.TenantSlug == "" {
		return errors.New("missing -tenant (required for a user)")
	}
	return nil
}

// runBootstrap provisions the account and (optionally) credential. It is
// idempotent: an existing account is reused, not duplicated, so the command is
// safe to re-run (e.g. to (re)set a password). Progress is written to out.
//
// The account world depends on the role (ONB-3, ADR 0011): a user is homed under
// a tenant (get-or-create by slug), while a platform admin is global and has no
// tenant (the -tenant flag is ignored). An existing subject in the *wrong* world
// (a user re-bootstrapped as admin or vice versa, or a user under a different
// tenant) is treated as a conflict rather than silently re-homed/converted.
func runBootstrap(ctx context.Context, pool *pgxpool.Pool, p bootstrapParams, out io.Writer) error {
	if err := p.validate(); err != nil {
		return err
	}

	tenants := store.NewTenantRepo(pool)
	users := store.NewUserRepo(pool)
	creds := store.NewCredentialRepo(pool)

	u, err := provisionAccount(ctx, tenants, users, p, out)
	if err != nil {
		return err
	}

	// Credential: only when a password is supplied (builtin mode). Set is an
	// upsert, so this also (re)sets the password of an existing account.
	if p.Password != "" {
		hash, err := auth.HashPassword(p.Password)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		if err := creds.Set(ctx, u.ID, hash); err != nil {
			return fmt.Errorf("set credential: %w", err)
		}
		_, _ = fmt.Fprintf(out, "set builtin password for %q\n", u.Subject)
	} else {
		_, _ = fmt.Fprintf(out, "no -password given: account has no builtin credential "+
			"(fine for proxy mode; for builtin mode re-run with -password or WAYFINDER_BOOTSTRAP_PASSWORD)\n")
	}
	return nil
}

// provisionAccount get-or-creates the account for p, branching on the role: a
// user under its tenant, an admin globally. It returns the resolved account.
func provisionAccount(ctx context.Context, tenants *store.TenantRepo, users *store.UserRepo, p bootstrapParams, out io.Writer) (store.User, error) {
	var email *string
	if p.Email != "" {
		email = &p.Email
	}

	// Admin world: global, no tenant.
	if p.Role == store.RoleAdmin {
		u, err := users.GetBySubject(ctx, p.Subject)
		switch {
		case errors.Is(err, store.ErrNotFound):
			if u, err = users.CreateAdmin(ctx, p.Subject, email); err != nil {
				return store.User{}, fmt.Errorf("create admin: %w", err)
			}
			_, _ = fmt.Fprintf(out, "created admin %q (id=%d)\n", u.Subject, u.ID)
			return u, nil
		case err != nil:
			return store.User{}, fmt.Errorf("look up account: %w", err)
		default:
			if u.Role != store.RoleAdmin {
				return store.User{}, fmt.Errorf("subject %q already exists as a tenant user; refusing to convert to admin", p.Subject)
			}
			_, _ = fmt.Fprintf(out, "admin %q already exists (id=%d)\n", u.Subject, u.ID)
			return u, nil
		}
	}

	// User world: homed under a tenant (get-or-create by slug).
	name := p.TenantName
	if name == "" {
		name = p.TenantSlug
	}
	t, err := tenants.GetBySlug(ctx, p.TenantSlug)
	switch {
	case errors.Is(err, store.ErrNotFound):
		if t, err = tenants.Create(ctx, p.TenantSlug, name); err != nil {
			return store.User{}, fmt.Errorf("create tenant: %w", err)
		}
		_, _ = fmt.Fprintf(out, "created tenant %q (id=%d)\n", t.Slug, t.ID)
	case err != nil:
		return store.User{}, fmt.Errorf("look up tenant: %w", err)
	default:
		_, _ = fmt.Fprintf(out, "tenant %q already exists (id=%d)\n", t.Slug, t.ID)
	}

	u, err := users.GetBySubject(ctx, p.Subject)
	switch {
	case errors.Is(err, store.ErrNotFound):
		if u, err = users.Create(ctx, t.ID, p.Subject, email); err != nil {
			return store.User{}, fmt.Errorf("create user: %w", err)
		}
		_, _ = fmt.Fprintf(out, "created user %q (id=%d)\n", u.Subject, u.ID)
		return u, nil
	case err != nil:
		return store.User{}, fmt.Errorf("look up account: %w", err)
	default:
		if u.Role != store.RoleUser {
			return store.User{}, fmt.Errorf("subject %q already exists as a platform admin; refusing to convert to a tenant user", p.Subject)
		}
		if u.TenantID != t.ID {
			return store.User{}, fmt.Errorf("user %q already exists under a different tenant (id=%d); refusing to re-home", p.Subject, u.TenantID)
		}
		_, _ = fmt.Fprintf(out, "user %q already exists (id=%d)\n", u.Subject, u.ID)
		return u, nil
	}
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
	fs.StringVar(&role, "role", string(store.RoleAdmin), "role: user|admin")
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
