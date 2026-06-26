package adminapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// meReq builds an /api/admin request whose Identity carries the
// must_change_password flag, exercising the ONB-1 gate and self-service routes.
func meReq(method, path, body string, role store.Role, mustChange bool) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	return r.WithContext(tenant.WithIdentity(r.Context(),
		tenant.Identity{TenantID: 7, UserID: 1, Subject: "admin", Role: role, MustChangePassword: mustChange}))
}

func handlerForMe(us UserStore, cs CredentialStore) *Handler {
	return New(&fakeVS{}, &fakeVS{}, fakeFeeds{}, fakeTenants{}, us, cs, &fakeEntitlements{},
		nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

// --- must_change_password gate (ONB-1) --------------------------------------

func TestGateBlocksNonAllowlistedWhenMustChange(t *testing.T) {
	h := handlerForMe(&fakeUserStore{}, &fakeCredStore{})
	// A normal admin route is refused while the flag is set.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/overview"},
		{http.MethodGet, "/api/admin/tenants"},
		{http.MethodPut, "/api/admin/view"},
		{http.MethodDelete, "/api/admin/me"}, // delete is NOT an unlock action
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, meReq(tc.method, tc.path, "", store.RoleAdmin, true))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "password_change_required") {
			t.Errorf("%s %s: body = %q, want password_change_required marker", tc.method, tc.path, rec.Body.String())
		}
	}
}

func TestGateAllowsUnlockRoutesWhenMustChange(t *testing.T) {
	h := handlerForMe(&fakeUserStore{}, &fakeCredStore{})
	// whoami and GET /me must pass through so the SPA can render the mask.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/whoami"},
		{http.MethodGet, "/api/admin/me"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, meReq(tc.method, tc.path, "", store.RoleAdmin, true))
		if rec.Code != http.StatusOK {
			t.Errorf("%s %s: status = %d, want 200 (allowlisted)", tc.method, tc.path, rec.Code)
		}
	}
}

func TestGateInactiveWhenFlagClear(t *testing.T) {
	h := handlerForMe(&fakeUserStore{}, &fakeCredStore{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, meReq(http.MethodGet, "/api/admin/overview", "", store.RoleAdmin, false))
	if rec.Code == http.StatusForbidden {
		t.Fatalf("status = 403, want the route to run normally when the flag is clear")
	}
}

// --- GET /api/admin/me -------------------------------------------------------

func TestGetMeReportsFlag(t *testing.T) {
	h := handlerForMe(&fakeUserStore{}, &fakeCredStore{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, meReq(http.MethodGet, "/api/admin/me", "", store.RoleAdmin, true))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got meDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.MustChangePassword || got.Subject != "admin" || got.Role != store.RoleAdmin {
		t.Errorf("me = %+v, want admin/admin with must_change_password=true", got)
	}
}

// --- PUT /api/admin/me/password ---------------------------------------------

func TestPutMePasswordSuccessClearsFlag(t *testing.T) {
	hash, err := auth.HashPassword("admin")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	us := &fakeUserStore{}
	cs := &fakeCredStore{getHash: map[int64]string{1: hash}}
	h := handlerForMe(us, cs)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, meReq(http.MethodPut, "/api/admin/me/password",
		`{"current_password":"admin","new_password":"newsecret123"}`, store.RoleAdmin, true))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if must, ok := us.mustChgSet[1]; !ok || must {
		t.Errorf("must_change_password not cleared: %v", us.mustChgSet)
	}
	if _, ok := cs.set[1]; !ok {
		t.Errorf("new password hash not stored")
	}
}

func TestPutMePasswordWrongCurrent(t *testing.T) {
	hash, _ := auth.HashPassword("admin")
	us := &fakeUserStore{}
	cs := &fakeCredStore{getHash: map[int64]string{1: hash}}
	h := handlerForMe(us, cs)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, meReq(http.MethodPut, "/api/admin/me/password",
		`{"current_password":"wrong","new_password":"newsecret123"}`, store.RoleAdmin, true))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if _, ok := us.mustChgSet[1]; ok {
		t.Errorf("flag must not change on a failed password change")
	}
}

func TestPutMePasswordTooShort(t *testing.T) {
	hash, _ := auth.HashPassword("admin")
	cs := &fakeCredStore{getHash: map[int64]string{1: hash}}
	h := handlerForMe(&fakeUserStore{}, cs)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, meReq(http.MethodPut, "/api/admin/me/password",
		`{"current_password":"admin","new_password":"short"}`, store.RoleAdmin, true))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- DELETE /api/admin/me (last-admin guard) --------------------------------

func TestDeleteMeLastAdminRefused(t *testing.T) {
	us := &fakeUserStore{activeAdmins: 1}
	h := handlerForMe(us, &fakeCredStore{})
	rec := httptest.NewRecorder()
	// Flag clear so the gate does not pre-empt the guard.
	h.ServeHTTP(rec, meReq(http.MethodDelete, "/api/admin/me", "", store.RoleAdmin, false))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (last admin)", rec.Code)
	}
	if us.deleted[1] {
		t.Errorf("last admin must not be deleted")
	}
}

func TestDeleteMeNonLastAdminSucceeds(t *testing.T) {
	us := &fakeUserStore{activeAdmins: 2}
	h := handlerForMe(us, &fakeCredStore{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, meReq(http.MethodDelete, "/api/admin/me", "", store.RoleAdmin, false))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if !us.deleted[1] {
		t.Errorf("account should be deleted when another admin remains")
	}
}
