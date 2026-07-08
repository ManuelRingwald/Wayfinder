package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/manuelringwald/wayfinder/pkg/store"
	"github.com/manuelringwald/wayfinder/pkg/tenant"
)

// ViewProfileStore is the slice of the store the per-user view-profile API needs
// (small interface so the handlers are unit-testable with a fake). All methods
// are scoped to the acting user (VP-2, ADR 0023).
type ViewProfileStore interface {
	ListByUser(ctx context.Context, userID int64) ([]store.ViewProfile, error)
	Create(ctx context.Context, userID int64, name string, settings json.RawMessage, makeDefault bool) (store.ViewProfile, error)
	Update(ctx context.Context, userID, id int64, name string, settings json.RawMessage) (store.ViewProfile, error)
	Delete(ctx context.Context, userID, id int64) error
	SetDefault(ctx context.Context, userID, id int64) (store.ViewProfile, error)
}

// WithViewProfiles wires the per-user view-profile store (VP-2, ADR 0023) so the
// /api/view-profiles routes are active. Nil-safe by omission — without it those
// routes report 404 (feature unavailable), like the other optional stores.
func (h *Handler) WithViewProfiles(s ViewProfileStore) *Handler {
	h.profiles = s
	return h
}

// Limits for a view profile (VP-2). The name is a short human label; the settings
// blob is opaque display config (the backend never interprets it) but is bounded
// so a client cannot store an unbounded payload per profile.
const (
	maxViewProfileNameLen       = 60
	maxViewProfileSettingsBytes = 16 * 1024
)

// viewProfileReq is the create/update request body. Settings is passed through to
// the store verbatim (opaque). MakeDefault is honoured only on create; the
// default is otherwise moved via the dedicated /default route.
type viewProfileReq struct {
	Name        string          `json:"name"`
	Settings    json.RawMessage `json:"settings"`
	MakeDefault bool            `json:"make_default"`
}

// viewProfileDTO is the wire shape of a stored profile.
type viewProfileDTO struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Settings  json.RawMessage `json:"settings"`
	IsDefault bool            `json:"is_default"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func toViewProfileDTO(vp store.ViewProfile) viewProfileDTO {
	s := vp.Settings
	if len(s) == 0 {
		s = json.RawMessage(`{}`)
	}
	return viewProfileDTO{ID: vp.ID, Name: vp.Name, Settings: s, IsDefault: vp.IsDefault, UpdatedAt: vp.UpdatedAt}
}

// validateViewProfile normalises and checks a create/update payload: the name is
// trimmed and must be non-empty and within the length cap; the settings blob must
// be a JSON OBJECT (never an array/scalar — the frontend stores a toggle map) and
// within the size cap. An empty/omitted settings normalises to "{}". The backend
// deliberately does NOT validate the individual toggle keys (opaque, ADR 0023).
func validateViewProfile(name string, settings json.RawMessage) (string, json.RawMessage, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil, errors.New("name must not be empty")
	}
	if utf8.RuneCountInString(name) > maxViewProfileNameLen {
		return "", nil, errors.New("name too long")
	}
	if len(settings) == 0 {
		return name, json.RawMessage(`{}`), nil
	}
	if len(settings) > maxViewProfileSettingsBytes {
		return "", nil, errors.New("settings too large")
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(settings, &obj); err != nil {
		return "", nil, errors.New("settings must be a JSON object")
	}
	return name, settings, nil
}

// ViewProfilesHandler returns the HTTP handler for the per-user view-profile
// routes (VP-2, ADR 0023). It is mounted in main.go behind the tenant middleware
// (any authenticated user) — NOT the admin gate: a profile is strictly the acting
// user's own. Every operation reads the user id from the session Identity, never
// from the request, and the store scopes each query by user id.
func (h *Handler) ViewProfilesHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/view-profiles", h.listViewProfiles)
	mux.HandleFunc("POST /api/view-profiles", h.createViewProfile)
	mux.HandleFunc("PUT /api/view-profiles/{id}", h.updateViewProfile)
	mux.HandleFunc("DELETE /api/view-profiles/{id}", h.deleteViewProfile)
	mux.HandleFunc("POST /api/view-profiles/{id}/default", h.setDefaultViewProfile)
	return mux
}

// viewProfileActor resolves the acting user and guards the nil store. It writes
// the appropriate error and returns ok=false when the request cannot proceed.
func (h *Handler) viewProfileActor(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := tenant.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return 0, false
	}
	if h.profiles == nil {
		writeError(w, http.StatusNotFound, "view profiles unavailable")
		return 0, false
	}
	return id.UserID, true
}

func (h *Handler) listViewProfiles(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.viewProfileActor(w, r)
	if !ok {
		return
	}
	list, err := h.profiles.ListByUser(r.Context(), userID)
	if err != nil {
		h.internalError(w, "list view profiles", err)
		return
	}
	out := make([]viewProfileDTO, 0, len(list))
	for _, vp := range list {
		out = append(out, toViewProfileDTO(vp))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createViewProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.viewProfileActor(w, r)
	if !ok {
		return
	}
	var req viewProfileReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxViewProfileSettingsBytes+2048)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	name, settings, err := validateViewProfile(req.Name, req.Settings)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	vp, err := h.profiles.Create(r.Context(), userID, name, settings, req.MakeDefault)
	if errors.Is(err, store.ErrProfileLimit) {
		writeError(w, http.StatusConflict, "profile limit reached (max 3)")
		return
	}
	if err != nil {
		h.internalError(w, "create view profile", err)
		return
	}
	writeJSON(w, http.StatusCreated, toViewProfileDTO(vp))
}

func (h *Handler) updateViewProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.viewProfileActor(w, r)
	if !ok {
		return
	}
	pid, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req viewProfileReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxViewProfileSettingsBytes+2048)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	name, settings, err := validateViewProfile(req.Name, req.Settings)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	vp, err := h.profiles.Update(r.Context(), userID, pid, name, settings)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		h.internalError(w, "update view profile", err)
		return
	}
	writeJSON(w, http.StatusOK, toViewProfileDTO(vp))
}

func (h *Handler) deleteViewProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.viewProfileActor(w, r)
	if !ok {
		return
	}
	pid, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	err = h.profiles.Delete(r.Context(), userID, pid)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		h.internalError(w, "delete view profile", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setDefaultViewProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.viewProfileActor(w, r)
	if !ok {
		return
	}
	pid, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	vp, err := h.profiles.SetDefault(r.Context(), userID, pid)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		h.internalError(w, "set default view profile", err)
		return
	}
	writeJSON(w, http.StatusOK, toViewProfileDTO(vp))
}
