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

// Platform-admin management (ONB-3, ADR 0011). Admins are global — they belong to
// no tenant — and are managed through these dedicated routes, strictly separated
// from the per-tenant user routes in adminapi_users.go. Every route is mounted
// behind requireAdmin (cross-cutting platform operation).
//
// The central safety invariant is the "last active admin" guard
// (wouldOrphanAdmins): deleting or pausing the final active admin is refused
// fail-closed (HTTP 409) so the platform can never be left with no way in.

// adminDTO is the management-facing shape of a platform admin. There is no tenant
// field (admins have none); the password hash is never exposed.
type adminDTO struct {
	ID                 int64   `json:"id"`
	Subject            string  `json:"subject"`
	Email              *string `json:"email,omitempty"`
	Status             string  `json:"status"`
	MustChangePassword bool    `json:"must_change_password"`
}

func toAdminDTO(u store.User) adminDTO {
	return adminDTO{
		ID:                 u.ID,
		Subject:            u.Subject,
		Email:              u.Email,
		Status:             string(u.Status),
		MustChangePassword: u.MustChangePassword,
	}
}

func (h *Handler) listAdmins(w http.ResponseWriter, r *http.Request) {
	admins, err := h.users.ListAdmins(r.Context())
	if err != nil {
		h.internalError(w, "list admins", err)
		return
	}
	out := make([]adminDTO, len(admins))
	for i, a := range admins {
		out[i] = toAdminDTO(a)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createAdmin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Subject  string `json:"subject"`
		Email    string `json:"email"`
		Password string `json:"password"`
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
	if body.Password != "" && len(body.Password) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password too short (min 8 characters)")
		return
	}
	// Subjects are globally unique; pre-check for a clean 409 instead of surfacing
	// the UNIQUE-constraint violation as a 500. The DB constraint remains the real
	// guard against a race.
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
	u, err := h.users.CreateAdmin(r.Context(), body.Subject, email)
	if err != nil {
		h.internalError(w, "create admin", err)
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
	writeJSON(w, http.StatusCreated, toAdminDTO(u))
}

func (h *Handler) setAdminStatus(w http.ResponseWriter, r *http.Request) {
	u, ok := h.adminByID(w, r)
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
	// Pausing the last active admin would lock everyone out — refuse fail-closed.
	if status == store.StatusPaused {
		orphan, err := h.wouldOrphanAdmins(r.Context(), u)
		if err != nil {
			h.internalError(w, "count active admins", err)
			return
		}
		if orphan {
			writeError(w, http.StatusConflict, "cannot pause the last active admin")
			return
		}
	}
	if err := h.users.SetStatus(r.Context(), u.ID, status); err != nil {
		h.internalError(w, "set admin status", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteAdmin(w http.ResponseWriter, r *http.Request) {
	u, ok := h.adminByID(w, r)
	if !ok {
		return
	}
	orphan, err := h.wouldOrphanAdmins(r.Context(), u)
	if err != nil {
		h.internalError(w, "count active admins", err)
		return
	}
	if orphan {
		writeError(w, http.StatusConflict, "cannot delete the last active admin")
		return
	}
	if err := h.users.Delete(r.Context(), u.ID); err != nil {
		h.internalError(w, "delete admin", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setAdminPassword(w http.ResponseWriter, r *http.Request) {
	u, ok := h.adminByID(w, r)
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
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		h.internalError(w, "hash password", err)
		return
	}
	if err := h.creds.Set(r.Context(), u.ID, hash); err != nil {
		h.internalError(w, "set credential", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// adminByID resolves the {adminID} path value to a user and verifies it is a
// platform admin. A non-admin id (a tenant user) is reported as 404 on this
// surface, so the admin routes cannot be used to reach or mutate tenant users.
func (h *Handler) adminByID(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	aid, err := pathInt(r, "adminID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid admin id")
		return store.User{}, false
	}
	u, err := h.users.GetByID(r.Context(), aid)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "admin not found")
		return store.User{}, false
	}
	if err != nil {
		h.internalError(w, "get admin", err)
		return store.User{}, false
	}
	if u.Role != store.RoleAdmin {
		writeError(w, http.StatusNotFound, "admin not found")
		return store.User{}, false
	}
	return u, true
}

// wouldOrphanAdmins reports whether deleting or pausing target would leave the
// platform with no active admin — the central ONB-3 invariant. Only an active
// admin can be the final one; a paused or non-admin account never triggers the
// guard. The same CountActiveAdmins backs the boot auto-seed and DELETE /me guard.
func (h *Handler) wouldOrphanAdmins(ctx context.Context, target store.User) (bool, error) {
	if target.Role != store.RoleAdmin || target.Status != store.StatusActive {
		return false, nil
	}
	n, err := h.users.CountActiveAdmins(ctx)
	if err != nil {
		return false, err
	}
	return n <= 1, nil
}
