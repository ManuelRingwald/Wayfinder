package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// Default identity provisioned by the boot auto-seed (ONB-1, ADR 0011). The
// password is intentionally a well-known default: zero-touch onboarding means the
// operator only starts the container, then logs in and is forced to replace it
// (must_change_password). The known credential is therefore valid for exactly one
// action — the one that changes it (enforced fail-closed in pkg/adminapi).
const (
	defaultAdminSubject  = "admin"
	defaultAdminPassword = "admin"
)

// autoSeedDefaultAdmin provisions the default platform admin on first boot so a
// fresh deployment is usable without any terminal step (ONB-1/ONB-3, ADR 0011).
// It is idempotent and fail-safe: it only seeds when there is no active admin
// yet, so a restart (or an operator who has already changed the password / added
// admins) never re-creates or resets anything. It is a no-op outside builtin
// mode — none/proxy modes mint no local password, so a seeded builtin credential
// is pointless.
//
// The admin is global (no tenant, ONB-3). Deliberately NO tenant is seeded
// (ADR 0011 Nachtrag): the earlier convenience tenant "default" only saved the
// single UI click of creating one (ONB-4) and confused operators — every real
// deployment names its own tenants, so a fresh instance starts with zero. The
// seed reuses runBootstrap (the same idempotent provisioning the CLI uses) and
// then marks the new admin must_change_password, so the known default credential
// must be rotated at first login.
func autoSeedDefaultAdmin(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	users := store.NewUserRepo(pool)

	n, err := users.CountActiveAdmins(ctx)
	if err != nil {
		return fmt.Errorf("auto-seed: count admins: %w", err)
	}
	if n > 0 {
		return nil // already provisioned (or operator-managed) — leave it alone
	}

	// Tenant-less default admin.
	p := bootstrapParams{
		Subject:  defaultAdminSubject,
		Role:     store.RoleAdmin,
		Password: defaultAdminPassword,
	}
	if err := runBootstrap(ctx, pool, p, out); err != nil {
		return fmt.Errorf("auto-seed: bootstrap default admin: %w", err)
	}

	// Force the password change at first login. Resolve the freshly seeded admin
	// by subject (runBootstrap does not return the row).
	u, err := users.GetBySubject(ctx, defaultAdminSubject)
	if err != nil {
		return fmt.Errorf("auto-seed: resolve seeded admin: %w", err)
	}
	if err := users.SetMustChangePassword(ctx, u.ID, true); err != nil {
		return fmt.Errorf("auto-seed: set must_change_password: %w", err)
	}
	_, _ = fmt.Fprintf(out, "auto-seeded default admin %q (must change password at first login)\n", defaultAdminSubject)
	return nil
}
