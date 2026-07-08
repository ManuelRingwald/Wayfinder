package adminapi

import "net/http"

// AirspaceOption is one airspace from a tenant's cache, projected for the AoR
// editor picker (ASD-014): the stable OpenAIP id (the value the AoR list stores),
// the display name, and the numeric type / ICAO class (both optional — omitted
// when the source lacked them). Only airspaces carrying a stable id are useful as
// AoR entries, so the lister drops those without one.
type AirspaceOption struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      *int   `json:"type,omitempty"`
	ICAOClass *int   `json:"icao_class,omitempty"`
}

// AirspaceLister returns a tenant's cached airspaces so the admin AoR editor can
// pick by name instead of raw ids (ASD-014). nil disables the route (404). It is
// satisfied by an adapter over aeronautical.Registry in main.go, which keeps
// adminapi transport-agnostic (no import of the aeronautical/GeoJSON types). The
// list reflects whatever is currently cached for the tenant — empty when the
// tenant has no OpenAIP data yet.
type AirspaceLister interface {
	ListAirspaces(tenantID int64) []AirspaceOption
}

// WithAirspaceLister wires the AoR airspace picker (ASD-014). Nil-safe by
// omission — without it the route reports 404. Returns the handler for chaining.
func (h *Handler) WithAirspaceLister(l AirspaceLister) *Handler {
	h.aeroList = l
	return h
}

// getTenantAirspaces serves a tenant's airspaces (id + name + type) for the AoR
// editor. Cross-tenant, admin-only. The list is whatever is cached for the tenant
// (empty when no OpenAIP data) — the AoR selection is validated authoritatively on
// the view PUT, so this route is a convenience for the picker, not a gate.
func (h *Handler) getTenantAirspaces(w http.ResponseWriter, r *http.Request) {
	if h.aeroList == nil {
		writeError(w, http.StatusNotFound, "airspace list unavailable")
		return
	}
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	out := h.aeroList.ListAirspaces(tid)
	if out == nil {
		out = []AirspaceOption{}
	}
	writeJSON(w, http.StatusOK, out)
}
