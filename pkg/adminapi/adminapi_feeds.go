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
	if err := validateMulticast(group, body.Port); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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

	f, err := h.feeds.Create(r.Context(), name, group, body.Port, regionPtr, mix)
	if err != nil {
		var unknown *sensorclass.UnknownClassError
		if errors.As(err, &unknown) {
			writeError(w, http.StatusBadRequest, "invalid sensor_mix: "+err.Error())
			return
		}
		h.internalError(w, "create feed", err)
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
