package adminapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/manuelringwald/wayfinder/pkg/sensorclass"
	"github.com/manuelringwald/wayfinder/pkg/store"
)

// Feed lifecycle management (ONB-5, ADR 0011). Creating and deleting feeds from
// the UI — and joining/leaving their multicast groups live — replaces the
// CLI-only `wayfinder feed add` step and the restart it used to require.
//
// Create is atomic across the catalogue and the live receiver: the feed is
// catalogued first, then its receiver is started; if the group cannot be joined
// the catalogue row is rolled back, so a feed is never left half-created
// (catalogued but silent). Delete is the mirror: the receiver leaves the group,
// then the row (and its subscriptions, by cascade) is removed.

const maxFeedNameLen = 100

// createFeed adds a feed to the catalogue and joins its multicast group live.
func (h *Handler) createFeed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name           string   `json:"name"`
		MulticastGroup string   `json:"multicast_group"`
		Port           int      `json:"port"`
		Region         string   `json:"region"`
		SensorMix      []string `json:"sensor_mix"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	name := strings.TrimSpace(body.Name)
	group := strings.TrimSpace(body.MulticastGroup)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(name) > maxFeedNameLen {
		writeError(w, http.StatusBadRequest, "name too long")
		return
	}

	// Endpoint is optional (ORCH-4): omit both group and port to have the server
	// auto-allocate a collision-free multicast endpoint from the pool; supply both
	// for a manual override. Supplying only one is a client error.
	auto := group == "" && body.Port == 0
	if !auto {
		if group == "" || body.Port == 0 {
			writeError(w, http.StatusBadRequest, "provide both multicast_group and port, or neither (to auto-allocate)")
			return
		}
		if err := validateMulticast(group, body.Port); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// Duplicate-name pre-check for a clean 409 instead of surfacing the UNIQUE
	// violation as a 500; the DB constraint (migration 00008) remains the race
	// backstop.
	if _, err := h.feeds.GetByName(r.Context(), name); err == nil {
		writeError(w, http.StatusConflict, "a feed with this name already exists")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		h.internalError(w, "check feed name", err)
		return
	}

	var regionPtr *string
	if region := strings.TrimSpace(body.Region); region != "" {
		regionPtr = &region
	}
	// Normalise the sensor mix: drop blanks; the store validates it against the
	// controlled vocabulary and rejects unknown classes (surfaced as 400).
	var mix []string
	for _, s := range body.SensorMix {
		if s = strings.TrimSpace(s); s != "" {
			mix = append(mix, s)
		}
	}

	var f store.Feed
	var err error
	if auto {
		f, err = h.feeds.CreateAutoAllocated(r.Context(), name, regionPtr, mix)
	} else {
		f, err = h.feeds.Create(r.Context(), name, group, body.Port, regionPtr, mix)
	}
	if err != nil {
		var unknown *sensorclass.UnknownClassError
		switch {
		case errors.As(err, &unknown):
			writeError(w, http.StatusBadRequest, "invalid sensor_mix: "+err.Error())
		case errors.Is(err, store.ErrEndpointTaken):
			writeError(w, http.StatusConflict, "multicast endpoint already in use")
		case errors.Is(err, store.ErrPoolExhausted):
			writeError(w, http.StatusInsufficientStorage, "no free multicast endpoint available (pool exhausted)")
		default:
			h.internalError(w, "create feed", err)
		}
		return
	}

	// Join the multicast group live (no restart). If the join fails — e.g. the
	// group/port is already bound — roll the catalogue row back so the operator
	// sees a clean failure and no silent, non-receiving feed lingers.
	if h.feedLife != nil {
		if err := h.feedLife.Start(f.ID, f.Name, f.MulticastGroup, f.Port); err != nil {
			if derr := h.feeds.Delete(r.Context(), f.ID); derr != nil {
				h.internalError(w, "rollback feed after failed join", derr)
				return
			}
			h.internalError(w, "join feed multicast group", err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, toFeedDTO(f))
}

// deleteFeed removes a feed from the catalogue and leaves its multicast group.
// Subscriptions referencing the feed cascade away (migration 00001); a tenant
// whose subscription was the deleted feed simply stops receiving its tracks.
// Guard C (ADR 0011): deletion is NOT blocked when tenants are subscribed — the
// grants cascade — but the count is logged so the operator action is auditable.
func (h *Handler) deleteFeed(w http.ResponseWriter, r *http.Request) {
	fid, err := pathInt(r, "feedID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feed id")
		return
	}
	if _, err := h.feeds.GetByID(r.Context(), fid); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "feed not found")
		return
	} else if err != nil {
		h.internalError(w, "get feed", err)
		return
	}

	// Stop the live receiver first so no further tracks are decoded for a feed
	// about to vanish from the catalogue. Releasing the group before the row keeps
	// the observable order intuitive (silent, then gone).
	if h.feedLife != nil {
		h.feedLife.Stop(fid)
	}

	if err := h.feeds.Delete(r.Context(), fid); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "feed not found")
			return
		}
		h.internalError(w, "delete feed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validateMulticast checks the wire coordinates of a feed: the group must be a
// valid IPv4 multicast address (the 224.0.0.0–239.255.255.255 range the receiver
// can join), and the port a usable UDP port. Rejecting bad coordinates here keeps
// the catalogue from ever holding a feed the receiver cannot bind.
func validateMulticast(group string, port int) error {
	if group == "" {
		return errors.New("multicast_group is required")
	}
	ip := net.ParseIP(group)
	if ip == nil || ip.To4() == nil {
		return errors.New("multicast_group must be an IPv4 address")
	}
	if !ip.IsMulticast() {
		return errors.New("multicast_group must be in the multicast range (224.0.0.0–239.255.255.255)")
	}
	if port < 1 || port > 65535 {
		return errors.New("port must be in 1..65535")
	}
	return nil
}

// Feed source configuration (ORCH-1b, ADR 0012). A feed gains the generic,
// Firefly-agnostic source list the orchestrator will later turn into a dedicated
// Firefly instance, plus the coarse outer coverage bbox handed to that instance.
// These are platform-admin operations (requireAdmin); the config is internal
// orchestration metadata, not tenant-facing, and cred_ref is only a *reference*
// to a per-feed secret — never the secret value (ADR 0012 §6).

// defaultCoverageMarginKm pads the derived coverage bbox beyond the union of the
// source query windows, so the coarse outer bound comfortably contains the
// precise inner tenant AOI. The operator may override coverage explicitly.
const defaultCoverageMarginKm = 50.0

// feedSourcesDTO is the wire shape of a feed's source configuration. Sources
// reuses the store model's JSON tags (type/bbox/sac/sic/cred_ref); CoverageBBox
// is the derived coarse outer bound (null when no source carries a bbox).
type feedSourcesDTO struct {
	Sources      store.SourceConfig `json:"sources"`
	CoverageBBox *store.BBox        `json:"coverage_bbox"`
}

// getFeedSources returns a feed's source configuration and derived coverage.
func (h *Handler) getFeedSources(w http.ResponseWriter, r *http.Request) {
	fid, err := pathInt(r, "feedID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feed id")
		return
	}
	sources, coverage, err := h.feeds.GetSourceConfig(r.Context(), fid)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "feed not found")
		return
	}
	if err != nil {
		h.internalError(w, "get feed sources", err)
		return
	}
	if sources == nil {
		sources = store.SourceConfig{}
	}
	writeJSON(w, http.StatusOK, feedSourcesDTO{Sources: sources, CoverageBBox: coverage})
}

// putFeedSources replaces a feed's source configuration. The sources are
// validated server-side (closed vocabulary, per-kind rules) — a rejected config
// returns 400 with the offending entry's index, never a partial write. When the
// body omits coverage_bbox, the server derives the coarse outer bound from the
// source bboxes (+ a default margin); an explicit coverage_bbox lets the operator
// override. The stored config is read back so the response is canonical.
func (h *Handler) putFeedSources(w http.ResponseWriter, r *http.Request) {
	fid, err := pathInt(r, "feedID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feed id")
		return
	}
	var body feedSourcesDTO
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := body.Sources.Validate(); err != nil {
		var ise *store.InvalidSourceError
		if errors.As(err, &ise) {
			writeError(w, http.StatusBadRequest, "invalid sources: "+ise.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid sources")
		return
	}

	coverage := body.CoverageBBox
	if coverage == nil {
		coverage = body.Sources.CoverageBBox(defaultCoverageMarginKm) // may be nil
	} else if err := validateBBox(coverage); err != nil {
		writeError(w, http.StatusBadRequest, "invalid coverage_bbox: "+err.Error())
		return
	}

	if err := h.feeds.SetSourceConfig(r.Context(), fid, body.Sources, coverage); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "feed not found")
			return
		}
		var ise *store.InvalidSourceError
		if errors.As(err, &ise) {
			writeError(w, http.StatusBadRequest, "invalid sources: "+ise.Error())
			return
		}
		h.internalError(w, "set feed sources", err)
		return
	}

	sources, cov, err := h.feeds.GetSourceConfig(r.Context(), fid)
	if err != nil {
		h.internalError(w, "read back feed sources", err)
		return
	}
	if sources == nil {
		sources = store.SourceConfig{}
	}
	writeJSON(w, http.StatusOK, feedSourcesDTO{Sources: sources, CoverageBBox: cov})
}

// validateBBox enforces the WGS84 invariants on an operator-supplied coverage
// bbox so a 400 is returned for bad input rather than surfacing a store error as
// a 500. Mirrors the AOI check in validateView.
func validateBBox(b *store.BBox) error {
	if b.MinLat < -90 || b.MinLat > 90 || b.MaxLat < -90 || b.MaxLat > 90 {
		return errors.New("latitude out of range [-90,90]")
	}
	if b.MinLon < -180 || b.MinLon > 180 || b.MaxLon < -180 || b.MaxLon > 180 {
		return errors.New("longitude out of range [-180,180]")
	}
	if b.MinLat > b.MaxLat || b.MinLon > b.MaxLon {
		return errors.New("min must be <= max")
	}
	return nil
}
