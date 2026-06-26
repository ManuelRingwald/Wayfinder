package adminapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// handlerForAdmins wires a handler for the ONB-3 platform-admin routes. Admins
// have no tenant, so the tenant fake is empty.
func handlerForAdmins(us UserStore, cs CredentialStore) *Handler {
	return handlerForUsers(us, cs, fakeTenants{})
}

func TestListAdmins(t *testing.T) {
	us := &fakeUserStore{admins: []store.User{
		{ID: 1, Subject: "root", Role: store.RoleAdmin, Status: store.StatusActive},
		{ID: 2, Subject: "ops", Role: store.RoleAdmin, Status: store.StatusPaused},
	}}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, &fakeCredStore{}).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/admins", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got []adminDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 || got[0].Subject != "root" || got[1].Status != "paused" {
		t.Fatalf("admins = %+v", got)
	}
}

func TestCreateAdminWithPassword(t *testing.T) {
	us := &fakeUserStore{bySubject: map[string]store.User{}, nextID: 42}
	cs := &fakeCredStore{}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, cs).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/admins", `{"subject":"newadmin","password":"hunter2!!"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	// Created tenant-less, always role admin.
	if us.created.TenantID != 0 || us.created.Role != store.RoleAdmin || us.created.Subject != "newadmin" {
		t.Fatalf("created = %+v, want tenant-less admin", us.created)
	}
	hash, ok := cs.set[42]
	if !ok {
		t.Fatal("no credential stored for new admin")
	}
	if good, _ := auth.VerifyPassword(hash, "hunter2!!"); !good {
		t.Fatal("stored hash does not verify against the password")
	}
}

func TestCreateAdminNoPasswordSkipsCredential(t *testing.T) {
	us := &fakeUserStore{bySubject: map[string]store.User{}, nextID: 5}
	cs := &fakeCredStore{}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, cs).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/admins", `{"subject":"proxyadmin"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if len(cs.set) != 0 {
		t.Fatalf("a credential was stored for a password-less (proxy) admin: %v", cs.set)
	}
}

func TestCreateAdminValidation(t *testing.T) {
	cases := map[string]struct {
		body string
		want int
	}{
		"missing subject": {`{"subject":"  "}`, http.StatusBadRequest},
		"short password":  {`{"subject":"e","password":"short"}`, http.StatusBadRequest},
		"bad json":        {`not-json`, http.StatusBadRequest},
	}
	for name, tc := range cases {
		rec := httptest.NewRecorder()
		handlerForAdmins(&fakeUserStore{bySubject: map[string]store.User{}}, &fakeCredStore{}).
			ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/admins", tc.body, 99, store.RoleAdmin))
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
	}
}

func TestCreateAdminDuplicateSubject(t *testing.T) {
	us := &fakeUserStore{bySubject: map[string]store.User{
		"taken": {ID: 9, Subject: "taken", Role: store.RoleAdmin, Status: store.StatusActive},
	}}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, &fakeCredStore{}).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/admins", `{"subject":"taken","password":"longenough"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

// --- last-active-admin guard (the central ONB invariant) --------------------

func TestSetAdminStatusPauseNonLastSucceeds(t *testing.T) {
	us := &fakeUserStore{
		byID:         map[int64]store.User{2: {ID: 2, Subject: "ops", Role: store.RoleAdmin, Status: store.StatusActive}},
		activeAdmins: 2, // another active admin remains
	}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, &fakeCredStore{}).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/admins/2", `{"status":"paused"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if us.statusSet[2] != store.StatusPaused {
		t.Fatalf("status set = %q, want paused", us.statusSet[2])
	}
}

func TestSetAdminStatusPauseLastRefused(t *testing.T) {
	us := &fakeUserStore{
		byID:         map[int64]store.User{2: {ID: 2, Subject: "root", Role: store.RoleAdmin, Status: store.StatusActive}},
		activeAdmins: 1, // this is the only active admin
	}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, &fakeCredStore{}).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/admins/2", `{"status":"paused"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (last active admin)", rec.Code)
	}
	if _, ok := us.statusSet[2]; ok {
		t.Error("last active admin must not be paused")
	}
}

// Reactivating is never guarded — even the "only" admin can be set active.
func TestSetAdminStatusReactivateNotGuarded(t *testing.T) {
	us := &fakeUserStore{
		byID:         map[int64]store.User{2: {ID: 2, Subject: "root", Role: store.RoleAdmin, Status: store.StatusPaused}},
		activeAdmins: 0,
	}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, &fakeCredStore{}).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/admins/2", `{"status":"active"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if us.statusSet[2] != store.StatusActive {
		t.Fatalf("status set = %q, want active", us.statusSet[2])
	}
}

func TestDeleteAdminLastRefused(t *testing.T) {
	us := &fakeUserStore{
		byID:         map[int64]store.User{2: {ID: 2, Subject: "root", Role: store.RoleAdmin, Status: store.StatusActive}},
		activeAdmins: 1,
	}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, &fakeCredStore{}).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/admins/2", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if us.deleted[2] {
		t.Error("last active admin must not be deleted")
	}
}

func TestDeleteAdminNonLastSucceeds(t *testing.T) {
	us := &fakeUserStore{
		byID:         map[int64]store.User{2: {ID: 2, Subject: "ops", Role: store.RoleAdmin, Status: store.StatusActive}},
		activeAdmins: 2,
	}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, &fakeCredStore{}).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/admins/2", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusNoContent || !us.deleted[2] {
		t.Fatalf("status = %d, deleted = %v; want 204 + deleted", rec.Code, us.deleted[2])
	}
}

// Deleting a *paused* admin is allowed even if it would be the only admin row:
// it is not active, so it can never be the last *active* admin.
func TestDeleteAdminPausedNotGuarded(t *testing.T) {
	us := &fakeUserStore{
		byID:         map[int64]store.User{2: {ID: 2, Subject: "ops", Role: store.RoleAdmin, Status: store.StatusPaused}},
		activeAdmins: 0,
	}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, &fakeCredStore{}).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/admins/2", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusNoContent || !us.deleted[2] {
		t.Fatalf("status = %d, deleted = %v; want 204 + deleted", rec.Code, us.deleted[2])
	}
}

// A tenant user id must not be reachable through the admin routes: 404, and no
// mutation happens.
func TestAdminRoutesRejectNonAdminID(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, TenantID: 7, Subject: "pilot", Role: store.RoleUser, Status: store.StatusActive},
	}}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPatch, "/api/admin/admins/2", `{"status":"paused"}`},
		{http.MethodDelete, "/api/admin/admins/2", ""},
		{http.MethodPut, "/api/admin/admins/2/password", `{"password":"longenough"}`},
	} {
		rec := httptest.NewRecorder()
		handlerForAdmins(us, &fakeCredStore{}).ServeHTTP(rec, adminReq(tc.method, tc.path, tc.body, 99, store.RoleAdmin))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404 (tenant user not on admin surface)", tc.method, tc.path, rec.Code)
		}
		if len(us.statusSet) != 0 || len(us.deleted) != 0 {
			t.Errorf("%s %s: a mutation touched a tenant user via the admin surface", tc.method, tc.path)
		}
	}
}

func TestSetAdminPassword(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, Subject: "ops", Role: store.RoleAdmin, Status: store.StatusActive},
	}}
	cs := &fakeCredStore{}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, cs).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/admins/2/password", `{"password":"newsecret1"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if good, _ := auth.VerifyPassword(cs.set[2], "newsecret1"); !good {
		t.Fatal("reset password does not verify")
	}
}

func TestSetAdminPasswordTooShort(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, Subject: "ops", Role: store.RoleAdmin, Status: store.StatusActive},
	}}
	rec := httptest.NewRecorder()
	handlerForAdmins(us, &fakeCredStore{}).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/admins/2/password", `{"password":"x"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestAdminRoutesForbidNonAdmin verifies every ONB-3 admin route is behind
// requireAdmin: a role user reaching them gets 403 and no mutation occurs.
func TestAdminRoutesForbidNonAdmin(t *testing.T) {
	routes := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/admin/admins", ""},
		{http.MethodPost, "/api/admin/admins", `{"subject":"x","password":"longenough"}`},
		{http.MethodPatch, "/api/admin/admins/2", `{"status":"paused"}`},
		{http.MethodDelete, "/api/admin/admins/2", ""},
		{http.MethodPut, "/api/admin/admins/2/password", `{"password":"longenough"}`},
	}
	for _, rt := range routes {
		us := &fakeUserStore{
			bySubject: map[string]store.User{},
			byID:      map[int64]store.User{2: {ID: 2, Role: store.RoleAdmin, Status: store.StatusActive}},
		}
		cs := &fakeCredStore{}
		rec := httptest.NewRecorder()
		handlerForAdmins(us, cs).ServeHTTP(rec, adminReq(rt.method, rt.path, rt.body, 7, store.RoleUser))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", rt.method, rt.path, rec.Code)
		}
		if us.created.ID != 0 || len(us.statusSet) != 0 || len(us.deleted) != 0 || len(cs.set) != 0 {
			t.Errorf("%s %s: a mutation occurred despite 403", rt.method, rt.path)
		}
	}
}
