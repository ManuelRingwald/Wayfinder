package adminapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
// disclosing the key itself, plus the persistent cache freshness (AERO-1, ADR
// 0018): when the tenant's OpenAIP data was last fetched and how many features are
// cached. The cache fields are omitted when nothing is cached yet or no cache
// reader is wired.
type openaipStatusDTO struct {
	Configured   bool       `json:"configured"`
	FetchedAt    *time.Time `json:"fetched_at,omitempty"`
	FeatureCount *int       `json:"feature_count,omitempty"`
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
	dto := openaipStatusDTO{Configured: key != nil}
	// Best-effort cache freshness (AERO-1): a read error just omits the fields —
	// it must never fail the status route.
	if h.aeroCache != nil {
		if fetchedAt, count, ok, cerr := h.aeroCache.AeroCacheStatus(r.Context(), tid); cerr != nil {
			h.logger.Warn("openaip status: read cache status failed", slog.Int64("tenant_id", tid), slog.String("error", cerr.Error()))
		} else if ok {
			c := count
			dto.FetchedAt = fetchedAt
			dto.FeatureCount = &c
		}
	}
	writeJSON(w, http.StatusOK, dto)
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
	// A key change is an explicit "fetch fresh data now" (AERO-1, ADR 0018): force a
	// refresh rather than the idempotent Apply, so a new/changed key re-fetches even
	// when stale data is still persisted from a previous key. Clearing the key
	// (key == nil) drops the per-tenant service back to the global cache.
	if key != nil {
		h.triggerAeroRefresh(r.Context(), tid)
	} else {
		h.triggerAeroApply(r.Context(), tid)
	}
	w.WriteHeader(http.StatusNoContent)
}

// refreshTenantOpenAIP forces a fresh OpenAIP fetch for one tenant (AERO-2, ADR
// 0018) — the per-tenant "refresh now" button (e.g. an AIRAC update). The fetch runs
// asynchronously in the registry; the route returns 202 once it is queued.
func (h *Handler) refreshTenantOpenAIP(w http.ResponseWriter, r *http.Request) {
	tid, err := pathInt(r, "tenantID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant id")
		return
	}
	if !h.tenantExists(w, r, tid) {
		return
	}
	h.triggerAeroRefresh(r.Context(), tid)
	w.WriteHeader(http.StatusAccepted)
}

// globalOpenAIPStatusDTO reports whether a UI-set global key is stored and whether
// encryption is available (WAYFINDER_SECRET_KEY). The key itself is never returned.
type globalOpenAIPStatusDTO struct {
	Configured          bool `json:"configured"`
	EncryptionAvailable bool `json:"encryption_available"`
}

func (h *Handler) getGlobalOpenAIP(w http.ResponseWriter, r *http.Request) {
	if h.globalAero == nil {
		writeJSON(w, http.StatusOK, globalOpenAIPStatusDTO{})
		return
	}
	configured, err := h.globalAero.Configured(r.Context())
	if err != nil {
		h.internalError(w, "get global openaip status", err)
		return
	}
	writeJSON(w, http.StatusOK, globalOpenAIPStatusDTO{
		Configured:          configured,
		EncryptionAvailable: h.globalAero.Available(),
	})
}

// setGlobalOpenAIP sets or clears the platform-wide OpenAIP key. Setting requires a
// configured cipher (503 without WAYFINDER_SECRET_KEY — Option A, no plaintext
// secret at rest). A successful set (or clear) triggers a fetch-all so every tenant
// picks up the new fallback immediately.
func (h *Handler) setGlobalOpenAIP(w http.ResponseWriter, r *http.Request) {
	if h.globalAero == nil {
		writeError(w, http.StatusNotFound, "global openaip key management unavailable")
		return
	}
	var body struct {
		APIKey *string `json:"api_key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	key := ""
	if body.APIKey != nil {
		key = strings.TrimSpace(*body.APIKey)
	}
	if len(key) > maxOpenAIPKeyLen {
		writeError(w, http.StatusBadRequest, "api_key too long")
		return
	}
	// Storing a (non-empty) key needs encryption; without it, fail closed rather than
	// persist a plaintext secret. Clearing (empty key) is always allowed.
	if key != "" && !h.globalAero.Available() {
		writeError(w, http.StatusServiceUnavailable, "encryption unavailable: set WAYFINDER_SECRET_KEY to store a global OpenAIP key")
		return
	}
	if err := h.globalAero.SetKey(r.Context(), key); err != nil {
		h.internalError(w, "set global openaip key", err)
		return
	}
	// The new (or cleared) global fallback must reach every tenant now.
	h.triggerAeroRefreshAll(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

// refreshAllOpenAIP forces a fresh OpenAIP fetch for every tenant (AERO-2) — the
// "refresh all" button. Asynchronous in the registry; returns 202 once queued.
func (h *Handler) refreshAllOpenAIP(w http.ResponseWriter, r *http.Request) {
	h.triggerAeroRefreshAll(r.Context())
	w.WriteHeader(http.StatusAccepted)
}

// aeroChangeDTO is the change-impact of the last refresh for one layer (AERO-3).
// prev_feature_count/added/removed are null on the first fetch (no prior to diff).
type aeroChangeDTO struct {
	Kind             string    `json:"kind"`
	FeatureCount     int       `json:"feature_count"`
	PrevFeatureCount *int      `json:"prev_feature_count"`
	Added            *int      `json:"added"`
	Removed          *int      `json:"removed"`
	FetchedAt        time.Time `json:"fetched_at"`
}

// getTenantOpenAIPChanges reports the per-layer change-impact of the tenant's last
// OpenAIP refresh (AERO-3). 404 when the feature is not wired.
func (h *Handler) getTenantOpenAIPChanges(w http.ResponseWriter, r *http.Request) {
	if h.aeroChanges == nil {
		writeError(w, http.StatusNotFound, "openaip change-impact unavailable")
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
	changes, err := h.aeroChanges.TenantAeroCacheChanges(r.Context(), tid)
	if err != nil {
		h.internalError(w, "list openaip changes", err)
		return
	}
	out := make([]aeroChangeDTO, 0, len(changes))
	for _, c := range changes {
		out = append(out, aeroChangeDTO{
			Kind:             c.Kind,
			FeatureCount:     c.FeatureCount,
			PrevFeatureCount: c.PrevFeatureCount,
			Added:            c.Added,
			Removed:          c.Removed,
			FetchedAt:        c.FetchedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
