package store

import (
	"context"
	"errors"
	"testing"
)

func TestMulticastPoolValidate(t *testing.T) {
	if err := DefaultMulticastPool.Validate(); err != nil {
		t.Fatalf("default pool invalid: %v", err)
	}
	bad := map[string]MulticastPool{
		"two octets":     {Base24: "239.255", OctetMin: 1, OctetMax: 254, Port: 8600},
		"bad octet":      {Base24: "239.255.300", OctetMin: 1, OctetMax: 254, Port: 8600},
		"not multicast":  {Base24: "10.0.0", OctetMin: 1, OctetMax: 254, Port: 8600},
		"min above max":  {Base24: "239.255.0", OctetMin: 200, OctetMax: 100, Port: 8600},
		"octet over 255": {Base24: "239.255.0", OctetMin: 1, OctetMax: 300, Port: 8600},
		"bad port":       {Base24: "239.255.0", OctetMin: 1, OctetMax: 254, Port: 0},
	}
	for name, p := range bad {
		if err := p.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		}
	}
}

func TestMulticastPoolGroup(t *testing.T) {
	p := MulticastPool{Base24: "239.255.0", OctetMin: 1, OctetMax: 254, Port: 8600}
	if g := p.group(42).String(); g != "239.255.0.42" {
		t.Fatalf("group(42) = %q, want 239.255.0.42", g)
	}
}

// TestIntegrationFeedAutoAllocate exercises the allocator against a real database:
// sequential allocation, skipping a manually taken endpoint, a duplicate manual
// endpoint → ErrEndpointTaken, and pool exhaustion. Requires WAYFINDER_TEST_DB_URL.
func TestIntegrationFeedAutoAllocate(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	// A tiny pool (octets 1..3) so exhaustion is reachable in the test.
	repo := NewFeedRepo(pool).WithMulticastPool(MulticastPool{Base24: "239.255.9", OctetMin: 1, OctetMax: 3, Port: 8600})

	// Sequential allocation hands out the lowest free group.
	f1, err := repo.CreateAutoAllocated(ctx, "auto-1", nil, nil)
	if err != nil || f1.MulticastGroup != "239.255.9.1" || f1.Port != 8600 {
		t.Fatalf("first auto = %+v, %v, want 239.255.9.1:8600", f1, err)
	}
	f2, err := repo.CreateAutoAllocated(ctx, "auto-2", nil, nil)
	if err != nil || f2.MulticastGroup != "239.255.9.2" {
		t.Fatalf("second auto = %+v, %v, want 239.255.9.2", f2, err)
	}

	// A manual create on a colliding endpoint is rejected by the unique constraint.
	if _, err := repo.Create(ctx, "manual-dup", "239.255.9.1", 8600, nil, nil); !errors.Is(err, ErrEndpointTaken) {
		t.Fatalf("manual dup = %v, want ErrEndpointTaken", err)
	}

	// The allocator skips the now-taken octets and assigns the last free one (.3).
	f3, err := repo.CreateAutoAllocated(ctx, "auto-3", nil, nil)
	if err != nil || f3.MulticastGroup != "239.255.9.3" {
		t.Fatalf("third auto = %+v, %v, want 239.255.9.3", f3, err)
	}

	// The pool (1..3) is now full → exhaustion.
	if _, err := repo.CreateAutoAllocated(ctx, "auto-4", nil, nil); !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("fourth auto = %v, want ErrPoolExhausted", err)
	}

	// A different group/port outside the pool is unaffected (manual endpoints on
	// other groups don't shrink the pool).
	if _, err := repo.Create(ctx, "other-group", "239.255.0.62", 8600, nil, nil); err != nil {
		t.Fatalf("manual outside pool: %v", err)
	}
}
