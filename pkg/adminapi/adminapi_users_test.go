package adminapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// tenantsWith returns a fakeTenants that knows tenant id (so tenantExists passes)
// and records SetStatus calls.
func tenantsWith(id int64) fakeTenants {
	return fakeTenants{
		byID:      map[int64]store.Tenant{id: {ID: id, Slug: "acme", Name: "ACME", Status: store.StatusActive}},
		statusSet: map[int64]store.Status{},
	}
}

func TestListUsers(t *testing.T) {
	us := &fakeUserStore{listByTen: map[int64][]store.User{
		7: {
			{ID: 1, TenantID: 7, Subject: "alice", Role: store.RoleUser, Status: store.StatusActive},
			{ID: 2, TenantID: 7, Subject: "bob", Role: store.RoleUser, Status: store.StatusPaused},
		},
	}}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/7/users", "", 7, store.RoleAdmin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got []userDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 || got[0].Subject != "alice" || got[1].Status != "paused" {
		t.Fatalf("users = %+v", got)
	}
}

func TestListUsersUnknownTenant(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerForUsers(&fakeUserStore{}, &fakeCredStore{}, fakeTenants{}).
		ServeHTTP(rec, adminReq(http.MethodGet, "/api/admin/tenants/9/users", "", 9, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestCreateUserWithPassword(t *testing.T) {
	us := &fakeUserStore{bySubject: map[string]store.User{}, nextID: 42}
	cs := &fakeCredStore{}
	rec := httptest.NewRecorder()
	handlerForUsers(us, cs, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/tenants/7/users", `{"subject":"carol","email":"c@x.de","password":"hunter2!!"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	// Created under the path tenant, always role user.
	if us.created.TenantID != 7 || us.created.Role != store.RoleUser || us.created.Subject != "carol" {
		t.Fatalf("created = %+v", us.created)
	}
	// The password round-trips through a real argon2id hash.
	hash, ok := cs.set[42]
	if !ok {
		t.Fatal("no credential stored for new user")
	}
	if good, _ := auth.VerifyPassword(hash, "hunter2!!"); !good {
		t.Fatal("stored hash does not verify against the password")
	}
}

func TestCreateUserNoPasswordSkipsCredential(t *testing.T) {
	us := &fakeUserStore{bySubject: map[string]store.User{}, nextID: 5}
	cs := &fakeCredStore{}
	rec := httptest.NewRecorder()
	handlerForUsers(us, cs, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/tenants/7/users", `{"subject":"dave"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if len(cs.set) != 0 {
		t.Fatalf("a credential was stored for a password-less (proxy) user: %v", cs.set)
	}
}

func TestCreateUserValidation(t *testing.T) {
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
		handlerForUsers(&fakeUserStore{bySubject: map[string]store.User{}}, &fakeCredStore{}, tenantsWith(7)).
			ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/tenants/7/users", tc.body, 7, store.RoleAdmin))
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
	}
}

// TestCreateUserRejectsAdminRole verifies the strict separation (ONB-3): the
// per-tenant user route refuses to create a platform admin — that is exclusively
// /api/admin/admins. No account is created.
func TestCreateUserRejectsAdminRole(t *testing.T) {
	us := &fakeUserStore{bySubject: map[string]store.User{}, nextID: 7}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/tenants/7/users", `{"subject":"sneaky","role":"admin","password":"longenough"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (admins are managed via /api/admin/admins)", rec.Code)
	}
	if us.created.ID != 0 {
		t.Error("no account must be created when an admin role is requested on the tenant route")
	}
}

// An explicit role:"user" is accepted (it matches the only role this route makes).
func TestCreateUserAcceptsExplicitUserRole(t *testing.T) {
	us := &fakeUserStore{bySubject: map[string]store.User{}, nextID: 8}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/tenants/7/users", `{"subject":"pilot","role":"user","password":"longenough"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if us.created.Role != store.RoleUser || us.created.TenantID != 7 {
		t.Fatalf("created = %+v, want tenant user", us.created)
	}
}

func TestCreateUserDuplicateSubject(t *testing.T) {
	us := &fakeUserStore{bySubject: map[string]store.User{
		"taken": {ID: 9, TenantID: 3, Subject: "taken", Role: store.RoleUser},
	}}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/tenants/7/users", `{"subject":"taken","password":"longenough"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestSetUserStatus(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, TenantID: 7, Subject: "bob", Role: store.RoleUser, Status: store.StatusActive},
	}}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/tenants/7/users/2", `{"status":"paused"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if us.statusSet[2] != store.StatusPaused {
		t.Fatalf("status set = %q, want paused", us.statusSet[2])
	}
}

func TestSetUserStatusInvalid(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{2: {ID: 2, TenantID: 7}}}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/tenants/7/users/2", `{"status":"deleted"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestUserCrossTenantMismatch verifies a user id belonging to another tenant is
// 404 under the wrong tenant's URL (the resource hierarchy stays honest).
func TestUserCrossTenantMismatch(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, TenantID: 99, Subject: "elsewhere", Role: store.RoleUser},
	}}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/tenants/7/users/2", "", 7, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (user belongs to tenant 99)", rec.Code)
	}
	if us.deleted[2] {
		t.Fatal("user from another tenant must not be deleted")
	}
}

func TestDeleteUser(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, TenantID: 7, Subject: "bob", Role: store.RoleUser},
	}}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/tenants/7/users/2", "", 7, store.RoleAdmin))
	if rec.Code != http.StatusNoContent || !us.deleted[2] {
		t.Fatalf("status = %d, deleted = %v", rec.Code, us.deleted[2])
	}
}

func TestSetUserPassword(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{
		2: {ID: 2, TenantID: 7, Subject: "bob", Role: store.RoleUser},
	}}
	cs := &fakeCredStore{}
	rec := httptest.NewRecorder()
	handlerForUsers(us, cs, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/tenants/7/users/2/password", `{"password":"newsecret1"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if good, _ := auth.VerifyPassword(cs.set[2], "newsecret1"); !good {
		t.Fatal("reset password does not verify")
	}
}

func TestSetUserPasswordTooShort(t *testing.T) {
	us := &fakeUserStore{byID: map[int64]store.User{2: {ID: 2, TenantID: 7}}}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, tenantsWith(7)).ServeHTTP(rec,
		adminReq(http.MethodPut, "/api/admin/tenants/7/users/2/password", `{"password":"x"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSetTenantStatus(t *testing.T) {
	ft := tenantsWith(7)
	rec := httptest.NewRecorder()
	handlerForUsers(&fakeUserStore{}, &fakeCredStore{}, ft).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/tenants/7", `{"status":"paused"}`, 7, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if ft.statusSet[7] != store.StatusPaused {
		t.Fatalf("tenant status set = %q, want paused", ft.statusSet[7])
	}
}

func TestSetTenantStatusUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerForUsers(&fakeUserStore{}, &fakeCredStore{}, fakeTenants{}).ServeHTTP(rec,
		adminReq(http.MethodPatch, "/api/admin/tenants/9", `{"status":"paused"}`, 9, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// --- tenant lifecycle (ONB-4) -----------------------------------------------

func TestCreateTenant(t *testing.T) {
	ft := fakeTenants{bySlug: map[string]store.Tenant{}, created: map[string]store.Tenant{}, nextID: 5}
	rec := httptest.NewRecorder()
	handlerForUsers(&fakeUserStore{}, &fakeCredStore{}, ft).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/tenants", `{"slug":"acme","name":"ACME Air"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var got tenantDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Slug != "acme" || got.Name != "ACME Air" || got.Status != "active" {
		t.Fatalf("created tenant = %+v", got)
	}
	if _, ok := ft.created["acme"]; !ok {
		t.Error("Create was not called with the slug")
	}
}

func TestCreateTenantDefaultsNameToSlug(t *testing.T) {
	ft := fakeTenants{bySlug: map[string]store.Tenant{}, created: map[string]store.Tenant{}}
	rec := httptest.NewRecorder()
	handlerForUsers(&fakeUserStore{}, &fakeCredStore{}, ft).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/tenants", `{"slug":"acme"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if ft.created["acme"].Name != "acme" {
		t.Fatalf("name = %q, want default to slug", ft.created["acme"].Name)
	}
}

func TestCreateTenantValidation(t *testing.T) {
	cases := map[string]struct {
		body string
		want int
	}{
		"missing slug":    {`{"name":"x"}`, http.StatusBadRequest},
		"blank slug":      {`{"slug":"  "}`, http.StatusBadRequest},
		"uppercase slug":  {`{"slug":"ACME"}`, http.StatusBadRequest},
		"space in slug":   {`{"slug":"ac me"}`, http.StatusBadRequest},
		"leading hyphen":  {`{"slug":"-acme"}`, http.StatusBadRequest},
		"trailing hyphen": {`{"slug":"acme-"}`, http.StatusBadRequest},
		"bad json":        {`not-json`, http.StatusBadRequest},
	}
	for name, tc := range cases {
		ft := fakeTenants{bySlug: map[string]store.Tenant{}, created: map[string]store.Tenant{}}
		rec := httptest.NewRecorder()
		handlerForUsers(&fakeUserStore{}, &fakeCredStore{}, ft).
			ServeHTTP(rec, adminReq(http.MethodPost, "/api/admin/tenants", tc.body, 99, store.RoleAdmin))
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
		if len(ft.created) != 0 {
			t.Errorf("%s: an invalid tenant reached the store", name)
		}
	}
}

func TestCreateTenantDuplicateSlug(t *testing.T) {
	ft := fakeTenants{
		bySlug:  map[string]store.Tenant{"acme": {ID: 1, Slug: "acme"}},
		created: map[string]store.Tenant{},
	}
	rec := httptest.NewRecorder()
	handlerForUsers(&fakeUserStore{}, &fakeCredStore{}, ft).ServeHTTP(rec,
		adminReq(http.MethodPost, "/api/admin/tenants", `{"slug":"acme","name":"dup"}`, 99, store.RoleAdmin))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if len(ft.created) != 0 {
		t.Error("duplicate slug must not reach Create")
	}
}

func TestDeleteTenantEmptySucceeds(t *testing.T) {
	ft := fakeTenants{
		byID:    map[int64]store.Tenant{5: {ID: 5, Slug: "acme"}},
		deleted: map[int64]bool{},
	}
	us := &fakeUserStore{listByTen: map[int64][]store.User{}} // no accounts
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, ft).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/tenants/5", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if !ft.deleted[5] {
		t.Error("empty tenant should have been deleted")
	}
}

func TestDeleteTenantWithAccountsRefused(t *testing.T) {
	ft := fakeTenants{
		byID:    map[int64]store.Tenant{5: {ID: 5, Slug: "acme"}},
		deleted: map[int64]bool{},
	}
	us := &fakeUserStore{listByTen: map[int64][]store.User{
		5: {{ID: 1, TenantID: 5, Subject: "pilot", Role: store.RoleUser}},
	}}
	rec := httptest.NewRecorder()
	handlerForUsers(us, &fakeCredStore{}, ft).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/tenants/5", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (tenant not empty)", rec.Code)
	}
	if ft.deleted[5] {
		t.Error("a non-empty tenant must not be deleted (guard B)")
	}
}

func TestDeleteTenantUnknownIs404(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerForUsers(&fakeUserStore{}, &fakeCredStore{}, fakeTenants{}).ServeHTTP(rec,
		adminReq(http.MethodDelete, "/api/admin/tenants/9", "", 99, store.RoleAdmin))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestTenantLifecycleRoutesForbidNonAdmin verifies the ONB-4 routes are behind
// requireAdmin: a role user gets 403 and no mutation occurs.
func TestTenantLifecycleRoutesForbidNonAdmin(t *testing.T) {
	routes := []struct {
		method, path, body string
	}{
		{http.MethodPost, "/api/admin/tenants", `{"slug":"x","name":"X"}`},
		{http.MethodDelete, "/api/admin/tenants/5", ""},
	}
	for _, rt := range routes {
		ft := fakeTenants{
			bySlug:  map[string]store.Tenant{},
			byID:    map[int64]store.Tenant{5: {ID: 5}},
			created: map[string]store.Tenant{},
			deleted: map[int64]bool{},
		}
		rec := httptest.NewRecorder()
		handlerForUsers(&fakeUserStore{}, &fakeCredStore{}, ft).ServeHTTP(rec, adminReq(rt.method, rt.path, rt.body, 7, store.RoleUser))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", rt.method, rt.path, rec.Code)
		}
		if len(ft.created) != 0 || len(ft.deleted) != 0 {
			t.Errorf("%s %s: a mutation occurred despite 403", rt.method, rt.path)
		}
	}
}

// TestAccessRoutesForbidNonAdmin verifies every AP6 route is behind requireAdmin:
// a role user (non-admin) reaching them gets 403 and no mutation occurs.
func TestAccessRoutesForbidNonAdmin(t *testing.T) {
	routes := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/admin/tenants/7/users", ""},
		{http.MethodPost, "/api/admin/tenants/7/users", `{"subject":"x","password":"longenough"}`},
		{http.MethodPatch, "/api/admin/tenants/7/users/2", `{"status":"paused"}`},
		{http.MethodDelete, "/api/admin/tenants/7/users/2", ""},
		{http.MethodPut, "/api/admin/tenants/7/users/2/password", `{"password":"longenough"}`},
		{http.MethodPatch, "/api/admin/tenants/7", `{"status":"paused"}`},
	}
	for _, rt := range routes {
		us := &fakeUserStore{
			bySubject: map[string]store.User{},
			byID:      map[int64]store.User{2: {ID: 2, TenantID: 7}},
		}
		cs := &fakeCredStore{}
		ft := tenantsWith(7)
		rec := httptest.NewRecorder()
		handlerForUsers(us, cs, ft).ServeHTTP(rec, adminReq(rt.method, rt.path, rt.body, 7, store.RoleUser))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", rt.method, rt.path, rec.Code)
		}
		if us.created.ID != 0 || len(us.statusSet) != 0 || len(us.deleted) != 0 || len(cs.set) != 0 || len(ft.statusSet) != 0 {
			t.Errorf("%s %s: a mutation occurred despite 403", rt.method, rt.path)
		}
	}
}
