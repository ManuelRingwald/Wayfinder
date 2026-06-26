package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// UserStore is the slice of the user repo the access-management routes need (AP6)
// plus the platform-admin management routes (ONB-3). *store.UserRepo satisfies it;
// tests use a fake. Create provisions a tenant user; CreateAdmin provisions a
// tenant-less platform admin — the two are strictly separated (ONB-3, ADR 0011).
type UserStore interface {
	ListByTenant(ctx context.Context, tenantID int64) ([]store.User, error)
	ListAdmins(ctx context.Context) ([]store.User, error)
	GetByID(ctx context.Context, id int64) (store.User, error)
	GetBySubject(ctx context.Context, subject string) (store.User, error)
	Create(ctx context.Context, tenantID int64, subject string, email *string) (store.User, error)
	CreateAdmin(ctx context.Context, subject string, email *string) (store.User, error)
	SetStatus(ctx context.Context, id int64, status store.Status) error
	SetMustChangePassword(ctx context.Context, id int64, must bool) error
	CountActiveAdmins(ctx context.Context) (int, error)
	Delete(ctx context.Context, id int64) error
}

// CredentialStore persists and retrieves a builtin-mode password hash for a user
// (AP6 password set/reset; ONB-1 self-service password change). *store.CredentialRepo
// satisfies it. GetHash returns store.ErrNotFound for a user without a local
// credential (e.g. an OIDC-only user).
type CredentialStore interface {
	Set(ctx context.Context, userID int64, passwordHash string) error
	GetHash(ctx context.Context, userID int64) (string, error)
}

// minPasswordLen is the server-side minimum for a builtin password set through
// the admin API. A weak password is a security regression the admin should not
// be able to introduce silently.
const minPasswordLen = 8

// userDTO is the admin-facing shape of an access account. The password hash and
// other secrets are never exposed; role is always "user" for accounts created
// here (admins are provisioned out-of-band via bootstrap).
type userDTO struct {
	ID      int64   `json:"id"`
	Subject string  `json:"subject"`
	Email   *string `json:"email,omitempty"`
	Role    string  `json:"role"`
	Status  string  `json:"status"`
}

func toUserDTO(u store.User) userDTO {
	return userDTO{ID: u.ID, Subject: u.Subject, Email: u.Email, Role: string(u.Role), Status: string(u.Status)}
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	us, err := h.users.ListByTenant(r.Context(), tid)
	if err != nil {
		h.internalError(w, "list users", err)
		return
	}
	out := make([]userDTO, len(us))
	for i, u := range us {
		out[i] = toUserDTO(u)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	var body struct {
		Subject  string `json:"subject"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	body.Subject = strings.TrimSpace(body.Subject)
	if body.Subject == "" {
		writeError(w, http.StatusBadRequest, "subject is required")
		return
	}
	// Strict separation (ONB-3, ADR 0011): this per-tenant route provisions tenant
	// users only. Platform admins are global (no tenant) and are managed through
	// /api/admin/admins. Reject any attempt to create an admin here so the two
	// worlds cannot be mixed via the tenant URL. An empty or "user" role is fine.
	if role := strings.TrimSpace(body.Role); role != "" && role != string(store.RoleUser) {
		writeError(w, http.StatusBadRequest, "this route creates tenant users only; manage platform admins via /api/admin/admins")
		return
	}
	if body.Password != "" && len(body.Password) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password too short (min 8 characters)")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	// Subjects are globally unique; pre-check for a clean 409 instead of surfacing
	// the UNIQUE-constraint violation as a 500. The DB constraint remains the
	// real guard against a race.
	if _, err := h.users.GetBySubject(r.Context(), body.Subject); err == nil {
		writeError(w, http.StatusConflict, "subject already exists")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		h.internalError(w, "check subject", err)
		return
	}
	var email *string
	if e := strings.TrimSpace(body.Email); e != "" {
		email = &e
	}
	// Accounts created here are always role "user" — this endpoint provisions
	// tenant access accounts, not platform admins (those go through
	// /api/admin/admins). The store's Create constructor encodes that invariant.
	u, err := h.users.Create(r.Context(), tid, body.Subject, email)
	if err != nil {
		h.internalError(w, "create user", err)
		return
	}
	if body.Password != "" {
		hash, herr := auth.HashPassword(body.Password)
		if herr != nil {
			h.internalError(w, "hash password", herr)
			return
		}
		if err := h.creds.Set(r.Context(), u.ID, hash); err != nil {
			h.internalError(w, "set credential", err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, toUserDTO(u))
}

func (h *Handler) setUserStatus(w http.ResponseWriter, r *http.Request) {
	tid, uid, ok := h.userPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	status := store.Status(body.Status)
	if !status.Valid() {
		writeError(w, http.StatusBadRequest, "invalid status (expected active|paused)")
		return
	}
	if _, ok := h.userInTenant(w, r, tid, uid); !ok {
		return
	}
	if err := h.users.SetStatus(r.Context(), uid, status); err != nil {
		h.internalError(w, "set user status", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	tid, uid, ok := h.userPath(w, r)
	if !ok {
		return
	}
	if _, ok := h.userInTenant(w, r, tid, uid); !ok {
		return
	}
	if err := h.users.Delete(r.Context(), uid); err != nil {
		h.internalError(w, "delete user", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setUserPassword(w http.ResponseWriter, r *http.Request) {
	tid, uid, ok := h.userPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(body.Password) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password too short (min 8 characters)")
		return
	}
	if _, ok := h.userInTenant(w, r, tid, uid); !ok {
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		h.internalError(w, "hash password", err)
		return
	}
	if err := h.creds.Set(r.Context(), uid, hash); err != nil {
		h.internalError(w, "set credential", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setTenantStatus pauses or reactivates a whole tenant (AP6). A paused tenant
// blocks login for all of its accounts (enforced at the login edge); existing
// sessions are not terminated here — immediate revocation is AP7.
func (h *Handler) setTenantStatus(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	status := store.Status(body.Status)
	if !status.Valid() {
		writeError(w, http.StatusBadRequest, "invalid status (expected active|paused)")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	if err := h.tenants.SetStatus(r.Context(), tid, status); err != nil {
		h.internalError(w, "set tenant status", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// userPath parses the {tenantID} and {userID} path values shared by the
// per-user routes, writing a 400 and returning ok=false on a malformed id.
func (h *Handler) userPath(w http.ResponseWriter, r *http.Request) (tid, uid int64, ok bool) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return 0, 0, false
	}
	uid, err = pathInt(r, "userID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return 0, 0, false
	}
	return tid, uid, true
}

// userInTenant resolves the user and verifies it belongs to the named tenant,
// keeping the resource hierarchy honest: a user id from another tenant yields a
// 404 (not found *under this tenant*), so an admin cannot mutate an account via
// the wrong tenant's URL.
func (h *Handler) userInTenant(w http.ResponseWriter, r *http.Request, tid, uid int64) (store.User, bool) {
	u, err := h.users.GetByID(r.Context(), uid)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return store.User{}, false
	}
	if err != nil {
		h.internalError(w, "get user", err)
		return store.User{}, false
	}
	if u.TenantID != tid {
		writeError(w, http.StatusNotFound, "user not found")
		return store.User{}, false
	}
	return u, true
}
