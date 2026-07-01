package adminapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// fakeRevoker records the AP7 session-revocation calls the pause handlers make.
type fakeRevoker struct {
	users   []int64
	tenants []int64
}

func (f *fakeRevoker) DeleteUserSessions(_ context.Context, userID int64) (int64, error) {
	f.users = append(f.users, userID)
	return 1, nil
}

func (f *fakeRevoker) DeleteTenantSessions(_ context.Context, tenantID int64) (int64, error) {
	f.tenants = append(f.tenants, tenantID)
	return 2, nil
}

func handlerForUsersWithRevoker(us UserStore, ft fakeTenants, rev SessionRevoker) *Handler {
	return New(&fakeVS{}, &fakeVS{}, fakeFeeds{}, ft, us, &fakeCredStore{}, &fakeEntitlements{}, nil, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil).
		WithSessionRevoker(rev)
}

// TestPauseUserRevokesSessions: pausing an access revokes its live sessions (AP7).
func TestPauseUserRevokesSessions(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, TenantID: 7, Subject: "bob", Role: store.RoleUser, Status: store.StatusActive},
	}}
	rev := &fakeRevoker{}
	rec := httptest.NewRecorder()
	handlerForUsersWithRevoker(us, tenantsWith(7), rev).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/tenants/7/users/2", `{"status":"paused"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if len(rev.users) != 1 || rev.users[0] != 2 {
		t.Fatalf("revoked users = %v, want [2]", rev.users)
	}
}

// TestReactivateUserDoesNotRevoke: reactivating (paused→active) must NOT revoke —
// revocation is only for suspension.
func TestReactivateUserDoesNotRevoke(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, TenantID: 7, Subject: "bob", Role: store.RoleUser, Status: store.StatusPaused},
	}}
	rev := &fakeRevoker{}
	rec := httptest.NewRecorder()
	handlerForUsersWithRevoker(us, tenantsWith(7), rev).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/tenants/7/users/2", `{"status":"active"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if len(rev.users) != 0 {
		t.Fatalf("reactivation revoked sessions: %v", rev.users)
	}
}

// TestPauseTenantRevokesSessions: pausing a tenant revokes every session under it.
func TestPauseTenantRevokesSessions(t *testing.T) {
	rev := &fakeRevoker{}
	rec := httptest.NewRecorder()
	handlerForUsersWithRevoker(&fakeUserStore{}, tenantsWith(7), rev).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/tenants/7", `{"status":"paused"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if len(rev.tenants) != 1 || rev.tenants[0] != 7 {
		t.Fatalf("revoked tenants = %v, want [7]", rev.tenants)
	}
}

// TestNilRevokerPauseStillSucceeds: with no revoker wired, a pause still succeeds
// (the session resolver's status join is the fail-closed backstop).
func TestNilRevokerPauseStillSucceeds(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, TenantID: 7, Subject: "bob", Role: store.RoleUser, Status: store.StatusActive},
	}}
	rec := httptest.NewRecorder()
	// handlerForUsers wires no revoker.
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/tenants/7/users/2", `{"status":"paused"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (nil revoker must be a no-op, not a failure)", rec.Code)
	}
}
