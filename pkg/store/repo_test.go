package store

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRoleValid(t *testing.T) {
	for _, r := range []Role{RoleUser, RoleAdmin} {
		if !r.Valid() {
			t.Errorf("Role(%q).Valid() = false, want true", r)
		}
	}
	for _, r := range []Role{"", "root", "Operator", "tenant-admin", "super_admin", "tenant_admin"} {
		if r.Valid() {
			t.Errorf("Role(%q).Valid() = true, want false", r)
		}
	}
}

func TestStatusValid(t *testing.T) {
	for _, s := range []Status{StatusActive, StatusPaused} {
		if !s.Valid() {
			t.Errorf("Status(%q).Valid() = false, want true", s)
		}
	}
	for _, s := range []Status{"", "Active", "disabled", "deleted", "suspended"} {
		if s.Valid() {
			t.Errorf("Status(%q).Valid() = true, want false", s)
		}
	}
}

func TestWrapMapsNoRowsToNotFound(t *testing.T) {
	if got := wrap("get tenant", pgx.ErrNoRows); !errors.Is(got, ErrNotFound) {
		t.Errorf("wrap(pgx.ErrNoRows) = %v, want errors.Is ErrNotFound", got)
	}

	sentinel := errors.New("boom")
	got := wrap("create tenant", sentinel)
	if errors.Is(got, ErrNotFound) {
		t.Errorf("wrap(other) should not be ErrNotFound: %v", got)
	}
	if !errors.Is(got, sentinel) {
		t.Errorf("wrap(other) lost the original error: %v", got)
	}
}
