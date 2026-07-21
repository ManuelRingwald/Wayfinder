package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/auth"
	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// emailPattern is a conservative, practical email shape (non-space/non-@ local
// part, an @, a dotted domain). It rejects obvious garbage while accepting
// normal addresses; it is deliberately NOT a full RFC-5322 parser (which would
// be both huge and permissive). maxEmailLen bounds the stored value (RFC 5321
// caps a forward path at 254 octets).
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

const maxEmailLen = 254

// meDTO is the logged-in principal's own account (ONB-1, ADR 0011). It is a
// read-only projection of the Identity plus the forced-change flag, so the SPA can
// decide whether to route the user to the password-change mask.
type meDTO struct {
	UserID             int64      `json:"user_id"`
	TenantID           int64      `json:"tenant_id"`
	Subject            string     `json:"subject"`
	Role               store.Role `json:"role"`
	MustChangePassword bool       `json:"must_change_password"`
}

// getMe returns the caller's own account. Reachable even while
// must_change_password is set (allowlisted), so the SPA can render the mask.
func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, meDTO{
		UserID:             id.UserID,
		TenantID:           id.TenantID,
		Subject:            id.Subject,
		Role:               id.Role,
		MustChangePassword: id.MustChangePassword,
	})
}

// putMePassword changes the caller's own builtin password (ONB-1, ADR 0011). It
// requires the current password (so a stolen, still-authenticated session cannot
// silently lock out the owner) and clears must_change_password on success — the
// single action that unlocks the rest of the admin surface for a freshly seeded
// admin. Reachable while the flag is set (allowlisted).
func (h *Handler) putMePassword(w http.ResponseWriter, r *http.Request) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(body.NewPassword) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password too short (min 8 characters)")
		return
	}

	// Verify the current password against the stored hash. A user without a local
	// credential (e.g. OIDC/proxy) cannot self-change a builtin password here.
	hash, err := h.creds.GetHash(r.Context(), id.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "no local password set for this account")
		return
	}
	if err != nil {
		h.internalError(w, "get credential", err)
		return
	}
	ok, err = auth.VerifyPassword(hash, body.CurrentPassword)
	if err != nil || !ok {
		// Constant-ish response: do not distinguish a malformed hash from a wrong
		// password to the client.
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		h.internalError(w, "hash password", err)
		return
	}
	if err := h.creds.Set(r.Context(), id.UserID, newHash); err != nil {
		h.internalError(w, "set credential", err)
		return
	}
	if err := h.users.SetMustChangePassword(r.Context(), id.UserID, false); err != nil {
		h.internalError(w, "clear must_change_password", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// putMeEmail sets or clears the caller's own contact email (#319, self-service
// under "Konto"). Self-scoped: the user id comes from the session Identity,
// never the request, so it is safe for any authenticated principal. Unlike a
// password change, an email change needs no local credential — so OIDC/proxy
// accounts may use it too. An empty value clears the email (store NULL); a
// non-empty value must be a plausible address. The stored value is exactly what
// an admin sees in the tenant's access table (userDTO.Email), so a self-service
// change is reflected there on the next load (the issue's "im Admin-Panel
// sichtbar/aktualisiert").
func (h *Handler) putMeEmail(w http.ResponseWriter, r *http.Request) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var email *string
	if e := strings.TrimSpace(body.Email); e != "" {
		if len(e) > maxEmailLen || !emailPattern.MatchString(e) {
			writeError(w, http.StatusBadRequest, "invalid email address")
			return
		}
		email = &e
	}
	if err := h.users.SetEmail(r.Context(), id.UserID, email); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		h.internalError(w, "set own email", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteMe deletes the caller's own account (ONB-1, ADR 0011). The "last active
// admin" guard refuses to remove the final admin (HTTP 409) so the platform can
// never be left with no way in. After deletion the session cookie is stale: the
// next request resolves to no user and is rejected fail-closed by the tenant
// middleware (immediate server-side session revocation is AP7, ADR 0009).
func (h *Handler) deleteMe(w http.ResponseWriter, r *http.Request) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if id.Role == store.RoleAdmin {
		n, err := h.users.CountActiveAdmins(r.Context())
		if err != nil {
			h.internalError(w, "count active admins", err)
			return
		}
		if n <= 1 {
			writeError(w, http.StatusConflict, "cannot delete the last active admin")
			return
		}
	}
	if err := h.users.Delete(r.Context(), id.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		h.internalError(w, "delete own account", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
