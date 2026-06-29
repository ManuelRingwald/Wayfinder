package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/store"
)

// Per-feed source credential management (ORCH-2c 3a, ADR 0012 §6, NFR-SEC-004).
// A feed's source configuration (ORCH-1) references credentials only by handle
// (cred_ref); this is where the actual value is set, rotated and cleared. The
// route is strictly write-only: a value goes in (sealed at rest before storage),
// but is never read back to the browser — the GET reports only which refs are
// configured. This mirrors the per-tenant OpenAIP key isolation (ONB-6) and keeps
// plaintext credentials off the browser edge entirely; they are decrypted only in
// the orchestrator control plane at launch (SecretResolver).
//
// The cipher and key never touch this layer: the handler depends on SecretService
// (an *orchestrator.SecretSealer in production), which seals on write. When no key
// is configured (WAYFINDER_SECRET_KEY unset) the service is nil and the routes
// return 503 — the capability is simply off, not silently insecure.

const (
	// maxSecretRefLen bounds an accepted cred_ref so a malformed path cannot store
	// an unbounded key. cred_refs are short handles; this is generous.
	maxSecretRefLen = 256
	// maxSecretValueLen bounds an accepted secret value. Source credentials
	// (API keys, tokens) are short; this is generous while capping the blob.
	maxSecretValueLen = 4096
)

// secretRefDTO reports a configured cred_ref without disclosing its value.
// Configured is always true for a ref returned here (it has a stored secret); the
// field is explicit so the shape stays forward-compatible and self-describing.
type secretRefDTO struct {
	Ref        string `json:"ref"`
	Configured bool   `json:"configured"`
}

// feedSecretsDTO is the wire shape of a feed's configured secret references.
type feedSecretsDTO struct {
	Secrets []secretRefDTO `json:"secrets"`
}

// getFeedSecrets lists the cred_refs that have a stored secret for the feed. It
// never returns a value — only which references are configured.
func (h *Handler) getFeedSecrets(w http.ResponseWriter, r *http.Request) {
	fid, err := pathInt(r, "feedID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feed id")
		return
	}
	if h.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "secret store not configured")
		return
	}
	if !h.feedExists(w, r, fid) {
		return
	}
	refs, err := h.secrets.ListSecretRefs(r.Context(), fid)
	if err != nil {
		h.internalError(w, "list feed secrets", err)
		return
	}
	out := feedSecretsDTO{Secrets: make([]secretRefDTO, 0, len(refs))}
	for _, ref := range refs {
		out.Secrets = append(out.Secrets, secretRefDTO{Ref: ref, Configured: true})
	}
	writeJSON(w, http.StatusOK, out)
}

// putFeedSecret sets (or replaces) the credential value for a feed's cred_ref. The
// value is sealed at rest by the service before storage and is never echoed back.
func (h *Handler) putFeedSecret(w http.ResponseWriter, r *http.Request) {
	fid, err := pathInt(r, "feedID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feed id")
		return
	}
	ref, ok := secretRef(w, r)
	if !ok {
		return
	}
	// value carries the plaintext credential. It is required: clearing a secret is
	// the DELETE route, so an empty value is a client error, not a silent clear.
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSecretValueLen+1024)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	value := body.Value
	if value == "" {
		writeError(w, http.StatusBadRequest, "value is required (use DELETE to clear a secret)")
		return
	}
	if len(value) > maxSecretValueLen {
		writeError(w, http.StatusBadRequest, "value too long")
		return
	}
	if h.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "secret store not configured")
		return
	}
	if !h.feedExists(w, r, fid) {
		return
	}
	if err := h.secrets.SetSecret(r.Context(), fid, ref, value); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "feed not found")
			return
		}
		h.internalError(w, "set feed secret", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteFeedSecret removes a feed's cred_ref secret. A missing ref yields 404.
func (h *Handler) deleteFeedSecret(w http.ResponseWriter, r *http.Request) {
	fid, err := pathInt(r, "feedID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feed id")
		return
	}
	ref, ok := secretRef(w, r)
	if !ok {
		return
	}
	if h.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "secret store not configured")
		return
	}
	if !h.feedExists(w, r, fid) {
		return
	}
	if err := h.secrets.DeleteSecret(r.Context(), fid, ref); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "secret not found")
			return
		}
		h.internalError(w, "delete feed secret", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// secretRef extracts and validates the {ref...} path value. It writes a 400 and
// returns ok=false when the ref is empty or too long.
func secretRef(w http.ResponseWriter, r *http.Request) (string, bool) {
	ref := strings.TrimSpace(r.PathValue("ref"))
	if ref == "" {
		writeError(w, http.StatusBadRequest, "cred_ref is required")
		return "", false
	}
	if len(ref) > maxSecretRefLen {
		writeError(w, http.StatusBadRequest, "cred_ref too long")
		return "", false
	}
	return ref, true
}

// feedExists writes a 404 (or 500) and returns false when the feed is unknown, so
// the secret handlers surface a clean "feed not found" before touching the store.
func (h *Handler) feedExists(w http.ResponseWriter, r *http.Request, feedID int64) bool {
	if _, err := h.feeds.GetByID(r.Context(), feedID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "feed not found")
			return false
		}
		h.internalError(w, "get feed", err)
		return false
	}
	return true
}
