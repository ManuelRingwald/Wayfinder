package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// OpenAIP per-tenant key management (ONB-6, ADR 0011). Each tenant may carry its
// own OpenAIP API key so it fetches airspace/navaid/waypoint data with its own
// account and quota, against its own area of interest. A tenant without a key
// falls back to the global key (WAYFINDER_OPENAIP_API_KEY).
//
// The key is a secret: it is set through this route but never read back to the
// browser — the GET reports only whether a key is configured. Setting or clearing
// the key (re)applies the tenant's per-tenant OpenAIP refresh live (no restart).

// maxOpenAIPKeyLen bounds an accepted key so a malformed body cannot store an
// unbounded blob. OpenAIP keys are short; this is generous.
const maxOpenAIPKeyLen = 512

// openaipStatusDTO reports whether a per-tenant key is configured, without ever
// disclosing the key itself.
type openaipStatusDTO struct {
	Configured bool `json:"configured"`
}

func (h *Handler) getTenantOpenAIP(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	key, err := h.tenants.GetOpenAIPKey(r.Context(), tid)
	if err != nil {
		h.internalError(w, "get tenant openaip key", err)
		return
	}
	writeJSON(w, http.StatusOK, openaipStatusDTO{Configured: key != nil})
}

func (h *Handler) setTenantOpenAIP(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	// api_key is a pointer so the client can distinguish "set to this" from
	// "clear" (null). An empty/whitespace string is treated as a clear, too, so the
	// UI can submit an empty field to remove the key.
	var body struct {
		APIKey *string `json:"api_key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var key *string
	if body.APIKey != nil {
		if trimmed := strings.TrimSpace(*body.APIKey); trimmed != "" {
			if len(trimmed) > maxOpenAIPKeyLen {
				writeError(w, http.StatusBadRequest, "api_key too long")
				return
			}
			key = &trimmed
		}
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	if err := h.tenants.SetOpenAIPKey(r.Context(), tid, key); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tenant not found")
			return
		}
		h.internalError(w, "set tenant openaip key", err)
		return
	}
	// Live-apply: (re)start the tenant's per-tenant OpenAIP refresh with the new
	// key (or fall back to the global one when cleared). Idempotent if unchanged.
	h.triggerAeroApply(r.Context(), tid)
	w.WriteHeader(http.StatusNoContent)
}
