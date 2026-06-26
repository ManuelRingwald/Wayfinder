package store

import (
	"context"
	"errors"
	"testing"
)

// TestIntegrationMustChangePasswordAndAdminCount exercises the ONB-1 (ADR 0011)
// additions against a real database: the must_change_password flag defaults to
// false (non-breaking migration), can be toggled, and CountActiveAdmins counts
// only active admins (the basis for the boot auto-seed guard and the "last active
// admin" guard).
func TestIntegrationMustChangePasswordAndAdminCount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)

	ten, err := tenants.Create(ctx, "acme", "ACME")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	// No admins yet (the boot auto-seed keys on this being zero).
	if n, err := users.CountActiveAdmins(ctx); err != nil || n != 0 {
		t.Fatalf("CountActiveAdmins on empty = (%d, %v), want (0, nil)", n, err)
	}

	// Platform admins are tenant-less (ONB-3).
	admin, err := users.CreateAdmin(ctx, "admin", nil)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	// New rows default to must_change_password=false.
	if admin.MustChangePassword {
		t.Fatalf("new user must_change_password = true, want false (non-breaking default)")
	}

	if err := users.SetMustChangePassword(ctx, admin.ID, true); err != nil {
		t.Fatalf("set must_change_password: %v", err)
	}
	if got, _ := users.GetByID(ctx, admin.ID); !got.MustChangePassword {
		t.Fatalf("after set, must_change_password = false, want true")
	}
	if err := users.SetMustChangePassword(ctx, admin.ID, false); err != nil {
		t.Fatalf("clear must_change_password: %v", err)
	}
	if got, _ := users.GetBySubject(ctx, "admin"); got.MustChangePassword {
		t.Fatalf("after clear, must_change_password = true, want false")
	}
	if err := users.SetMustChangePassword(ctx, 999999, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetMustChangePassword(missing) = %v, want ErrNotFound", err)
	}

	// One active admin now.
	if n, _ := users.CountActiveAdmins(ctx); n != 1 {
		t.Fatalf("CountActiveAdmins = %d, want 1", n)
	}
	// A paused admin does not count (it cannot log in).
	if err := users.SetStatus(ctx, admin.ID, StatusPaused); err != nil {
		t.Fatalf("pause admin: %v", err)
	}
	if n, _ := users.CountActiveAdmins(ctx); n != 0 {
		t.Fatalf("CountActiveAdmins with paused admin = %d, want 0", n)
	}
	// A plain user never counts as an admin.
	if _, err := users.Create(ctx, ten.ID, "alice", nil); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if n, _ := users.CountActiveAdmins(ctx); n != 0 {
		t.Fatalf("CountActiveAdmins counting a plain user = %d, want 0", n)
	}
}
