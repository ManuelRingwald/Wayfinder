package impersonation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

var testKey = []byte("test-signing-key-32-bytes-minimum-xx")

// fakeTenants is a DB-free TenantChecker. existing lists the tenant ids that are
// present; err, when set, simulates a database failure.
type fakeTenants struct {
	existing map[int64]bool
	err      error
}

func (f fakeTenants) Exists(_ context.Context, id int64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.existing[id], nil
}

func superAdmin() tenant.Identity {
	return tenant.Identity{TenantID: 1, UserID: 1, Subject: "root", Role: store.RoleSuperAdmin}
}

// --- grant crypto -----------------------------------------------------------

func TestMintParseRoundTrip(t *testing.T) {
	token := MintGrant(42, time.Hour, testKey)
	tid, err := parseGrant(token, testKey)
	if err != nil {
		t.Fatalf("parseGrant: unexpected error %v", err)
	}
	if tid != 42 {
		t.Fatalf("parseGrant: tid = %d, want 42", tid)
	}
}

func TestParseGrantRejectsTampered(t *testing.T) {
	token := MintGrant(7, time.Hour, testKey)
	// Flip the final signature character.
	tampered := token[:len(token)-1]
	if token[len(token)-1] == 'A' {
		tampered += "B"
	} else {
		tampered += "A"
	}
	if _, err := parseGrant(tampered, testKey); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("parseGrant(tampered): err = %v, want ErrInvalidGrant", err)
	}
}

func TestParseGrantRejectsWrongKey(t *testing.T) {
	token := MintGrant(7, time.Hour, testKey)
	if _, err := parseGrant(token, []byte("a-different-signing-key-entirely-yy")); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("parseGrant(wrong key): err = %v, want ErrInvalidGrant", err)
	}
}

func TestParseGrantRejectsExpired(t *testing.T) {
	token := MintGrant(7, -time.Minute, testKey) // already expired
	if _, err := parseGrant(token, testKey); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("parseGrant(expired): err = %v, want ErrInvalidGrant", err)
	}
}

// --- Resolve: the decision matrix -------------------------------------------

func TestResolveNoGrantIsDefaultPath(t *testing.T) {
	d, err := Resolve(context.Background(), "", superAdmin(), testKey, fakeTenants{})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if d.Active {
		t.Fatalf("expected inactive (default path), got Active with target %d", d.TargetTenantID)
	}
}

func TestResolveValidGrantSuperAdminActivates(t *testing.T) {
	token := MintGrant(42, time.Hour, testKey)
	tenants := fakeTenants{existing: map[int64]bool{42: true}}
	d, err := Resolve(context.Background(), token, superAdmin(), testKey, tenants)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !d.Active || d.TargetTenantID != 42 {
		t.Fatalf("expected Active target=42, got Active=%v target=%d", d.Active, d.TargetTenantID)
	}
}

func TestResolveValidGrantNonSuperAdminDenied(t *testing.T) {
	token := MintGrant(42, time.Hour, testKey)
	tenants := fakeTenants{existing: map[int64]bool{42: true}}
	for _, role := range []store.Role{store.RoleOperator, store.RoleTenantAdmin} {
		id := tenant.Identity{TenantID: 5, UserID: 9, Subject: "mallory", Role: role}
		d, err := Resolve(context.Background(), token, id, testKey, tenants)
		if !errors.Is(err, ErrDenied) {
			t.Fatalf("role=%s: err = %v, want ErrDenied", role, err)
		}
		if d.Active {
			t.Fatalf("role=%s: must not activate on denial", role)
		}
	}
}

func TestResolveValidGrantUnknownTenantRejected(t *testing.T) {
	token := MintGrant(99, time.Hour, testKey)
	tenants := fakeTenants{existing: map[int64]bool{42: true}} // 99 absent
	d, err := Resolve(context.Background(), token, superAdmin(), testKey, tenants)
	if !errors.Is(err, ErrUnknownTenant) {
		t.Fatalf("err = %v, want ErrUnknownTenant", err)
	}
	if d.Active {
		t.Fatalf("must not activate for a non-existent target tenant")
	}
}

func TestResolveExpiredGrantIsIgnored(t *testing.T) {
	token := MintGrant(42, -time.Minute, testKey) // expired
	tenants := fakeTenants{existing: map[int64]bool{42: true}}
	d, err := Resolve(context.Background(), token, superAdmin(), testKey, tenants)
	if err != nil {
		t.Fatalf("expired grant must not error (default path), got %v", err)
	}
	if d.Active {
		t.Fatalf("expired grant must not activate impersonation")
	}
}

func TestResolveTamperedGrantIsIgnored(t *testing.T) {
	token := MintGrant(42, time.Hour, testKey)
	tampered := "x" + token[1:] // corrupt the payload
	tenants := fakeTenants{existing: map[int64]bool{42: true}}
	d, err := Resolve(context.Background(), tampered, superAdmin(), testKey, tenants)
	if err != nil {
		t.Fatalf("tampered grant must not error (default path), got %v", err)
	}
	if d.Active {
		t.Fatalf("tampered grant must not activate impersonation")
	}
}

func TestResolveTenantCheckerErrorFailsClosed(t *testing.T) {
	token := MintGrant(42, time.Hour, testKey)
	tenants := fakeTenants{err: errors.New("database down")}
	d, err := Resolve(context.Background(), token, superAdmin(), testKey, tenants)
	if err == nil {
		t.Fatal("a tenant-checker error must propagate (fail closed), got nil")
	}
	if d.Active {
		t.Fatalf("must not activate when the tenant check fails")
	}
}
