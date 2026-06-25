// Package health tracks the liveness of the CAT065 SDPS heartbeat feed
// (Firefly ADR 0018). It lets Wayfinder distinguish an empty sky (a valid,
// track-less feed that still heartbeats) from a dead feed (no heartbeat for
// too long), which drives the staleness banner, the /metrics gauge and the
// readiness probe.
//
// The clock is passed in rather than read internally, so the staleness logic
// is deterministic and unit-testable (no sleeping in tests).
package health

import (
	"sync"
	"time"
)

// Status is a point-in-time view of the feed's liveness.
type Status struct {
	// EverSeen is true once at least one heartbeat has arrived. Before that we
	// cannot claim the feed is stale — it may simply not emit CAT065 yet.
	EverSeen bool
	// Stale is true when a heartbeat was seen but none has arrived within the
	// configured timeout.
	Stale bool
}

// FeedHealth tracks the time of the last received CAT065 heartbeat and reports
// whether the feed has gone stale. Safe for concurrent use.
type FeedHealth struct {
	mu            sync.Mutex
	timeout       time.Duration
	lastHeartbeat time.Time
	everSeen      bool

	reportInit   bool
	reportStatus Status
}

// New creates a FeedHealth that considers the feed stale when no heartbeat has
// arrived for longer than timeout.
func New(timeout time.Duration) *FeedHealth {
	return &FeedHealth{timeout: timeout}
}

// RecordHeartbeat marks that a heartbeat arrived at time now.
func (h *FeedHealth) RecordHeartbeat(now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastHeartbeat = now
	h.everSeen = true
}

// Status returns the feed liveness as of now.
func (h *FeedHealth) Status(now time.Time) Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.statusLocked(now)
}

func (h *FeedHealth) statusLocked(now time.Time) Status {
	return Status{
		EverSeen: h.everSeen,
		Stale:    h.everSeen && now.Sub(h.lastHeartbeat) > h.timeout,
	}
}

// LastHeartbeat returns the wall-clock time of the most recent heartbeat and
// whether any heartbeat has been seen. Used by the per-feed health registry
// to compute last_heartbeat_ago for the admin dashboard (AP4).
func (h *FeedHealth) LastHeartbeat() (t time.Time, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastHeartbeat, h.everSeen
}

// Observe returns the current status and whether it changed since the last call
// to Observe — used to broadcast a feed-status update only on a transition
// (first heartbeat, ok→stale, stale→ok) instead of on every tick.
func (h *FeedHealth) Observe(now time.Time) (status Status, changed bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	status = h.statusLocked(now)
	changed = !h.reportInit || status != h.reportStatus
	h.reportInit = true
	h.reportStatus = status
	return status, changed
}
