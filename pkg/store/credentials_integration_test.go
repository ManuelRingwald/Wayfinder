package store

import (
	"context"
	"errors"
	"testing"
)

func TestIntegrationCredentialRepo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)
	creds := NewCredentialRepo(pool)

	ten, _ := tenants.Create(ctx, "demo", "Demo")
	u, _ := users.Create(ctx, ten.ID, "bob", nil, RoleOperator)

	// A user without a local credential -> ErrNotFound (e.g. an OIDC user).
	if _, err := creds.GetHash(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing credential = %v, want ErrNotFound", err)
	}

	if err := creds.Set(ctx, u.ID, "$argon2id$hash1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := creds.GetHash(ctx, u.ID)
	if err != nil || got != "$argon2id$hash1" {
		t.Fatalf("get = %q, %v", got, err)
	}

	// Upsert replaces the hash.
	if err := creds.Set(ctx, u.ID, "$argon2id$hash2"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := creds.GetHash(ctx, u.ID); got != "$argon2id$hash2" {
		t.Fatalf("after upsert = %q", got)
	}
}
