// Package health — per-feed health registry (AP4).
//
// Registry tracks CAT065 heartbeat liveness and recent track activity for each
// feed ID, enabling the admin dashboard to show a per-feed health colour
// (green / yellow / red) without N separate FeedHealth instances wired into
// main. It also exposes aggregate Status/Observe methods that are drop-in
// replacements for the former single-feed FeedHealth used by the readiness
// probe and the browser feed-status banner.
package health

import (
	"sync"
	"time"
)

// SensorDetail is the per-sensor view within a feed (#237): the identity and
// state of one radar/sensor from the most recent CAT063 block, plus the
// registration bias currently applied to it. It lets the operator see WHICH
// sensor is degraded and how far it is being range/azimuth-corrected — a growing
// bias is an early warning of a miscalibrating sensor. The health package holds
// it as a plain domain type; the transport layers (broadcast, adminapi) attach
// their own JSON shape.
type SensorDetail struct {
	SAC         uint8
	SIC         uint8
	Operational bool
	// Reason is the per-source failure reason for a degraded sensor (Firefly
	// ADR 0033), "" when operational or unknown.
	Reason string
	// RangeBiasM / AzimuthBiasDeg are the applied registration correction
	// (I063/080 SRB in metres, I063/081 SAB in degrees), nil when no correction
	// is in force for this sensor (absence, never 0).
	RangeBiasM     *float64
	AzimuthBiasDeg *float64
}

// FeedSnapshot is a point-in-time health view for one feed (AP4).
type FeedSnapshot struct {
	EverSeen          bool
	Stale             bool
	LastHeartbeatAgoS float64 // seconds since last heartbeat; negative if never seen
	TrackCountRecent  int64   // size of the most recently received CAT062 block

	// SensorsTotal and SensorsActive are populated once CAT063 sensor-status
	// messages are decoded (Firefly issue #32 / WF-1). Until then both are zero
	// ("unknown") and Color() never returns "yellow".
	SensorsTotal  int
	SensorsActive int

	// SdpsDegraded is true when the most recent CAT065 heartbeat carried NOGO
	// (I065/040 != operational) — the SDPS/tracker reports itself degraded even
	// though it is still alive (Firefly SAFE.4, #261). Drives Color() "yellow".
	// False before the first heartbeat and for an operational SDPS.
	SdpsDegraded bool

	// DegradedReason is the per-source failure reason for a degraded feed
	// ("unreachable" / "auth" / "rate_limited"), decoded from the CAT063 I063/RE
	// SRC-REASON sub-field (Firefly ADR 0033). "" when the feed is healthy or the
	// degradation carries no known reason. Purely informational — it does not
	// affect Color().
	DegradedReason string

	// Sensors is the per-sensor breakdown from the most recent CAT063 block
	// (#237): identity, operational state and applied registration bias per
	// sensor. Empty ("nil") until CAT063 arrives. Drives the per-sensor detail on
	// the feed-health chip and the admin dashboard.
	Sensors []SensorDetail
}

// Color returns the display colour for this feed:
//   - "red":    no heartbeat (stale or never seen)
//   - "yellow": heartbeat fresh but degraded — either the SDPS itself reports
//     NOGO (CAT065 I065/040, Firefly SAFE.4: a stalled tracker still heartbeats
//     but flags itself degraded, #261) or the sensor fusion is degraded (at least
//     one configured sensor silent, 0 < SensorsActive < SensorsTotal, CAT063)
//   - "green":  heartbeat fresh and operational (empty sky counts as green)
//
// A NOGO heartbeat therefore drives "yellow" instead of "green": it still resets
// the staleness clock (the feed is alive), but it is no longer mistaken for a
// healthy one — it surfaces as a degraded feed rather than silently vanishing.
func (s FeedSnapshot) Color() string {
	if !s.EverSeen || s.Stale {
		return "red"
	}
	if s.SdpsDegraded || (s.SensorsTotal > 0 && s.SensorsActive < s.SensorsTotal) {
		return "yellow"
	}
	return "green"
}

// feedEntry holds per-feed heartbeat tracking and the most-recently-received
// block size (used as the "recent track count" proxy) and sensor counts.
type feedEntry struct {
	fh             *FeedHealth
	mu             sync.Mutex
	block          int64          // size of last received CAT062 block
	sensorsActive  int            // active sensors from last CAT063 block
	sensorsTotal   int            // total sensors from last CAT063 block
	degradedReason string         // per-source failure reason from last CAT063 block (ADR 0033)
	sensors        []SensorDetail // per-sensor breakdown from last CAT063 block (#237)
	sdpsDegraded   bool           // SDPS NOGO from the last CAT065 heartbeat (#261)
}

// Registry tracks health and recent track activity per feed ID. Feeds are
// registered lazily on the first heartbeat or track record. Safe for concurrent
// use.
type Registry struct {
	timeout time.Duration
	mu      sync.Mutex
	entries map[int64]*feedEntry
	// global aggregates all feeds into one FeedHealth for backward-compatible
	// Status/Observe calls (readiness probe, browser feed-status banner).
	global *FeedHealth
}

// NewRegistry creates a Registry that marks a feed stale when no heartbeat
// has arrived for longer than timeout.
func NewRegistry(timeout time.Duration) *Registry {
	return &Registry{
		timeout: timeout,
		entries: make(map[int64]*feedEntry),
		global:  New(timeout),
	}
}

func (r *Registry) getOrCreate(feedID int64) *feedEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[feedID]
	if !ok {
		e = &feedEntry{fh: New(r.timeout)}
		r.entries[feedID] = e
	}
	return e
}

// Forget drops all per-feed health state for feedID. It is called when a feed is
// deleted from the catalogue (ONB-5, ADR 0011) so the dashboard stops reporting a
// phantom (forever-stale) feed and the entry's memory is reclaimed. The global
// aggregate is intentionally left untouched: it reflects whether *any* feed has
// ever been seen / is stale, and a removed feed should not retroactively rewrite
// that history. A subsequent heartbeat/track for the same id re-creates the entry.
func (r *Registry) Forget(feedID int64) {
	r.mu.Lock()
	delete(r.entries, feedID)
	r.mu.Unlock()
}

// RecordHeartbeat records a CAT065 heartbeat for feedID at wall-clock time now.
// operational is the SDPS status carried by the heartbeat (I065/040 NOGO, #261):
// false means the SDPS reports itself degraded even though it still heartbeats
// (Firefly SAFE.4). The heartbeat always resets the staleness clock (the feed is
// alive); the degraded flag drives the feed colour to "yellow" so a NOGO is not
// mistaken for a healthy feed.
func (r *Registry) RecordHeartbeat(feedID int64, now time.Time, operational bool) {
	e := r.getOrCreate(feedID)
	e.fh.RecordHeartbeat(now)
	e.mu.Lock()
	e.sdpsDegraded = !operational
	e.mu.Unlock()
	r.global.RecordHeartbeat(now)
}

// RecordTracks records that count tracks arrived in the most recent CAT062
// block for feedID.
func (r *Registry) RecordTracks(feedID int64, count int) {
	e := r.getOrCreate(feedID)
	e.mu.Lock()
	e.block = int64(count)
	e.mu.Unlock()
}

// RecordSensors records the per-sensor breakdown from the most recent CAT063
// block for feedID (Firefly ADR 0022 / #237). active is the number of
// operational sensors; total is the total number of sensors in the block; reason
// is the dominant per-source failure reason ("" when none, I063/RE SRC-REASON,
// Firefly ADR 0033); sensors is the full per-sensor detail (identity, state,
// applied bias). The counts are passed in (computed by the caller) so the
// aggregate colour stays consistent with the detail.
func (r *Registry) RecordSensors(feedID int64, active, total int, reason string, sensors []SensorDetail) {
	e := r.getOrCreate(feedID)
	e.mu.Lock()
	e.sensorsActive = active
	e.sensorsTotal = total
	e.degradedReason = reason
	e.sensors = sensors
	e.mu.Unlock()
}

// Snapshot returns the health snapshot for feedID as of now. If feedID has
// never been registered, it returns the zero value (EverSeen=false, Color "red").
func (r *Registry) Snapshot(feedID int64, now time.Time) FeedSnapshot {
	r.mu.Lock()
	e, ok := r.entries[feedID]
	r.mu.Unlock()
	if !ok {
		return FeedSnapshot{}
	}
	st := e.fh.Status(now)
	t, seen := e.fh.LastHeartbeat()
	agoS := -1.0
	if seen {
		agoS = now.Sub(t).Seconds()
	}
	e.mu.Lock()
	block := e.block
	sensorsActive := e.sensorsActive
	sensorsTotal := e.sensorsTotal
	degradedReason := e.degradedReason
	sensors := e.sensors
	sdpsDegraded := e.sdpsDegraded
	e.mu.Unlock()
	return FeedSnapshot{
		EverSeen:          st.EverSeen,
		Stale:             st.Stale,
		LastHeartbeatAgoS: agoS,
		TrackCountRecent:  block,
		SensorsActive:     sensorsActive,
		SensorsTotal:      sensorsTotal,
		DegradedReason:    degradedReason,
		Sensors:           sensors,
		SdpsDegraded:      sdpsDegraded,
	}
}

// Status returns the aggregate liveness across all feeds: EverSeen if any feed
// has ever heartbeated; Stale if any feed is stale. Drop-in for
// FeedHealth.Status.
func (r *Registry) Status(now time.Time) Status {
	return r.global.Status(now)
}

// Observe returns the aggregate status and whether it changed since the last
// Observe call. Drop-in for FeedHealth.Observe.
func (r *Registry) Observe(now time.Time) (Status, bool) {
	return r.global.Observe(now)
}
