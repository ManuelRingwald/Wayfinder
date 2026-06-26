package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// Tenant lifecycle management (ONB-4, ADR 0011). Creating and deleting tenants
// from the UI replaces the last CLI-only provisioning step. Both routes are
// mounted behind requireAdmin.
//
// Deleting a tenant cascades (ON DELETE CASCADE) to its dependents — but only
// after the "tenant not empty" guard passes: a tenant that still has accounts is
// refused (409), so a single click can never silently wipe a fleet of controller
// logins. The destructive cascade is a conscious two-step (remove accounts first).

// slugPattern constrains a tenant slug to a DNS-label-like form: lowercase
// letters/digits separated by single hyphens, no leading/trailing hyphen. It is
// the stable, URL-safe key a tenant is addressed by.
var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

const maxSlugLen = 63

func (h *Handler) createTenant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	slug := strings.TrimSpace(body.Slug)
	name := strings.TrimSpace(body.Name)
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}
	if len(slug) > maxSlugLen || !slugPattern.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "invalid slug (lowercase letters, digits and hyphens; no leading/trailing hyphen)")
		return
	}
	if name == "" {
		name = slug // mirror the bootstrap default
	}
	// Slugs are unique; pre-check for a clean 409 instead of surfacing the
	// UNIQUE-constraint violation as a 500. The DB constraint remains the real
	// guard against a race.
	if _, err := h.tenants.GetBySlug(r.Context(), slug); err == nil {
		writeError(w, http.StatusConflict, "slug already exists")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		h.internalError(w, "check slug", err)
		return
	}
	t, err := h.tenants.Create(r.Context(), slug, name)
	if err != nil {
		h.internalError(w, "create tenant", err)
		return
	}
	writeJSON(w, http.StatusCreated, tenantDTO{ID: t.ID, Slug: t.Slug, Name: t.Name, Status: string(t.Status)})
}

func (h *Handler) deleteTenant(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	// Guard B (ADR 0011): refuse to delete a tenant that still has accounts. The
	// operator must remove them first, so the cascading delete is never an
	// accidental one-click wipe of controller logins. (Admins are tenant-less since
	// ONB-3, so ListByTenant never returns one — deleting a tenant cannot affect
	// the platform-admin set.)
	us, err := h.users.ListByTenant(r.Context(), tid)
	if err != nil {
		h.internalError(w, "list tenant users", err)
		return
	}
	if len(us) > 0 {
		writeError(w, http.StatusConflict, "tenant still has accounts; remove them before deleting it")
		return
	}
	if err := h.tenants.Delete(r.Context(), tid); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tenant not found")
			return
		}
		h.internalError(w, "delete tenant", err)
		return
	}
	h.triggerAeroStop(tid) // drop the tenant's per-tenant OpenAIP refresh (ONB-6)
	w.WriteHeader(http.StatusNoContent)
}
