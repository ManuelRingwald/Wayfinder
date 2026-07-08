package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// TestIntegrationViewProfilesCRUD exercises the ADR 0023 per-user view profiles
// against a real database: list/create/update/delete/set-default, the
// three-per-user cap, the single-default invariant, and cross-user isolation.
// Requires WAYFINDER_TEST_DB_URL (skipped otherwise).
func TestIntegrationViewProfilesCRUD(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants := NewTenantRepo(pool)
	users := NewUserRepo(pool)
	repo := NewViewProfileRepo(pool)

	ten, err := tenants.Create(ctx, "acme", "ACME")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	u, err := users.Create(ctx, ten.ID, "alice", nil)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	other, err := users.Create(ctx, ten.ID, "bob", nil)
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	// Fresh user: no profiles, no default.
	if list, err := repo.ListByUser(ctx, u.ID); err != nil || len(list) != 0 {
		t.Fatalf("initial list = %v (err %v), want empty", list, err)
	}
	if _, err := repo.GetDefault(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetDefault(none) = %v, want ErrNotFound", err)
	}

	// Create three; the first is the login default.
	p1, err := repo.Create(ctx, u.ID, "Approach", json.RawMessage(`{"rangeRings":true}`), true)
	if err != nil {
		t.Fatalf("create p1: %v", err)
	}
	if !p1.IsDefault {
		t.Errorf("p1.IsDefault = false, want true")
	}
	var s map[string]any
	if err := json.Unmarshal(p1.Settings, &s); err != nil || s["rangeRings"] != true {
		t.Errorf("p1.Settings = %s (err %v), want the stored object", p1.Settings, err)
	}
	if _, err := repo.Create(ctx, u.ID, "Overview", json.RawMessage(`{}`), false); err != nil {
		t.Fatalf("create p2: %v", err)
	}
	p3, err := repo.Create(ctx, u.ID, "Tower", nil, false)
	if err != nil {
		t.Fatalf("create p3: %v", err)
	}
	// nil settings normalise to an empty object, never SQL NULL.
	if string(p3.Settings) != "{}" {
		t.Errorf("p3.Settings = %s, want {}", p3.Settings)
	}

	// The fourth exceeds the cap.
	if _, err := repo.Create(ctx, u.ID, "Extra", nil, false); !errors.Is(err, ErrProfileLimit) {
		t.Fatalf("4th create = %v, want ErrProfileLimit", err)
	}

	// List returns exactly three, in creation order.
	list, err := repo.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 || list[0].Name != "Approach" || list[2].Name != "Tower" {
		t.Fatalf("list order/size wrong: %+v", list)
	}

	// Exactly one default, and it is p1.
	if def, err := repo.GetDefault(ctx, u.ID); err != nil || def.ID != p1.ID {
		t.Fatalf("GetDefault = %+v (err %v), want p1", def, err)
	}

	// SetDefault moves the flag; the old default is cleared (single-default holds).
	if moved, err := repo.SetDefault(ctx, u.ID, p3.ID); err != nil || !moved.IsDefault {
		t.Fatalf("SetDefault = %+v (err %v)", moved, err)
	}
	if def, _ := repo.GetDefault(ctx, u.ID); def.ID != p3.ID {
		t.Errorf("after SetDefault, default = %d, want %d", def.ID, p3.ID)
	}
	list, _ = repo.ListByUser(ctx, u.ID)
	defaults := 0
	for _, p := range list {
		if p.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Errorf("default count = %d, want 1", defaults)
	}

	// Update renames and replaces settings.
	upd, err := repo.Update(ctx, u.ID, p1.ID, "Approach EDDH", json.RawMessage(`{"historyDots":false}`))
	if err != nil || upd.Name != "Approach EDDH" {
		t.Fatalf("update = %+v (err %v)", upd, err)
	}

	// Cross-user access is denied and never leaks another user's row.
	if _, err := repo.Update(ctx, other.ID, p1.ID, "hax", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-user update = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, other.ID, p1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-user delete = %v, want ErrNotFound", err)
	}
	if _, err := repo.SetDefault(ctx, other.ID, p1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-user setdefault = %v, want ErrNotFound", err)
	}

	// Deleting frees a slot; a subsequent create then succeeds.
	if err := repo.Delete(ctx, u.ID, p1.ID); err != nil {
		t.Fatalf("delete p1: %v", err)
	}
	if list, _ := repo.ListByUser(ctx, u.ID); len(list) != 2 {
		t.Fatalf("after delete, size = %d, want 2", len(list))
	}
	if _, err := repo.Create(ctx, u.ID, "Fourth OK", nil, false); err != nil {
		t.Fatalf("create after delete: %v", err)
	}

	// The other user is fully isolated — still no profiles.
	if ol, _ := repo.ListByUser(ctx, other.ID); len(ol) != 0 {
		t.Errorf("other user profiles = %d, want 0", len(ol))
	}
}
