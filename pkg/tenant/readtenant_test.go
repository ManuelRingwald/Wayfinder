package tenant

import (
	"context"
	"testing"
)

// The read-tenant context marker (ADR 0008 Nachtrag) redirects READ endpoints to
// the impersonation target without ever touching the Identity: absent marker →
// fallback (the caller's own tenant), present marker → the target.
func TestReadTenantFallsBackWithoutMarker(t *testing.T) {
	ctx := context.Background()
	if got := ReadTenant(ctx, 7); got != 7 {
		t.Fatalf("ReadTenant without marker = %d, want fallback 7", got)
	}
	if _, ok := ImpersonatedTenant(ctx); ok {
		t.Fatal("ImpersonatedTenant without marker must report ok=false")
	}
}

func TestReadTenantReturnsMarkedTarget(t *testing.T) {
	ctx := WithReadTenant(context.Background(), 9)
	if got := ReadTenant(ctx, 7); got != 9 {
		t.Fatalf("ReadTenant with marker = %d, want target 9", got)
	}
	tid, ok := ImpersonatedTenant(ctx)
	if !ok || tid != 9 {
		t.Fatalf("ImpersonatedTenant = (%d, %v), want (9, true)", tid, ok)
	}
}

// The marker must not leak into or disturb the Identity: both live side by side
// under distinct context keys.
func TestReadTenantDoesNotTouchIdentity(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{TenantID: 7, UserID: 3})
	ctx = WithReadTenant(ctx, 9)
	id, ok := FromContext(ctx)
	if !ok || id.TenantID != 7 {
		t.Fatalf("Identity after WithReadTenant = (%+v, %v), want TenantID 7 intact", id, ok)
	}
}
