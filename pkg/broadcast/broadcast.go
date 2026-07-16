package broadcast

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
)

// Message is sent to WebSocket clients. It carries either a track update
// (Tracks) or a per-feed health update (FeedStatus, Firefly ICD 2.5.0).
type Message struct {
	Tracks     []TrackMessage     `json:"tracks"`
	TimeMs     int64              `json:"time_ms"`
	FeedStatus *FeedStatusMessage `json:"feed_status,omitempty"`
}

// FeedStatusMessage carries the per-feed health state to the browser (AP4,
// Firefly ICD 2.5.0, ADR 0022). Derived from the FeedSnapshot in the health
// registry — combining CAT065 heartbeat liveness and CAT063 sensor counts.
// Routed only to clients subscribed to FeedID (WF2-21 isolation).
type FeedStatusMessage struct {
	// FeedID identifies which feed this status belongs to. 0 = the unassigned
	// ENV fallback feed, routed to all clients (no real catalogue feed uses 0).
	FeedID int64 `json:"feed_id"`
	// Color is the health indicator: "green" (operational), "yellow" (degraded:
	// heartbeat fresh but 0 < sensors_active < sensors_total), "red" (stale or
	// never seen).
	Color string `json:"color"`
	// SensorsActive is the number of operational sensors from the last CAT063
	// block. 0 when no CAT063 data has arrived yet (unknown).
	SensorsActive int `json:"sensors_active"`
	// SensorsTotal is the total number of sensors from the last CAT063 block.
	// 0 when no CAT063 data has arrived yet (unknown).
	SensorsTotal int `json:"sensors_total"`
	// DegradedReason is the per-source failure reason for a degraded feed
	// ("unreachable" / "auth" / "rate_limited"), from the CAT063 I063/RE
	// SRC-REASON sub-field (Firefly ADR 0033). Omitted when empty (healthy feed
	// or no known reason) so the healthy path stays byte-for-byte unchanged.
	DegradedReason string `json:"degraded_reason,omitempty"`
	// Sensors is the per-sensor breakdown from the last CAT063 block (#237):
	// identity, state and applied registration bias per sensor. Omitted until
	// CAT063 arrives, so a feed without sensor status stays byte-for-byte
	// unchanged.
	Sensors []FeedSensor `json:"sensors,omitempty"`
}

// FeedSensor is one sensor's status within a feed status message (#237). It
// carries the applied registration bias so the ASD can show, per sensor, how far
// the SDPS is range/azimuth-correcting it (a growing bias warns of a
// miscalibrating radar). Bias fields are omitted when no correction is applied.
type FeedSensor struct {
	SAC         uint8 `json:"sac"`
	SIC         uint8 `json:"sic"`
	Operational bool  `json:"operational"`
	// DegradedReason is this sensor's failure reason when degraded (ADR 0033).
	DegradedReason string `json:"degraded_reason,omitempty"`
	// RangeBiasM (I063/080 SRB, metres) and AzimuthBiasDeg (I063/081 SAB,
	// degrees) are the applied correction; omitted when none is in force.
	RangeBiasM     *float64 `json:"range_bias_m,omitempty"`
	AzimuthBiasDeg *float64 `json:"azimuth_bias_deg,omitempty"`
}

// TrackBatch is a set of decoded tracks attributed to the feed they arrived on.
// FeedID is stamped by the receiver (WF2-20) and threaded onto every resulting
// TrackMessage, so the scoped fan-out (WF2-21) can filter per tenant
// subscription. FeedID 0 is the unassigned ENV fallback feed (no catalogue feed).
type TrackBatch struct {
	FeedID int64
	Tracks []cat062.DecodedTrack
}

// Scope decides which tracks a client may receive (WF2-21 — the cross-tenant
// isolation boundary). In WF2-21.1 it is feed-level: a client sees a track only
// if the track's feed is in the allowed set, resolved from the tenant's
// subscriptions at connect time. Every client is scoped (ADR 0014): a nil Scope
// is fail-closed (sees nothing), never a passthrough. View filters
// (AOI/FL/category) layer on in WF2-21.2.
type Scope struct {
	// TenantID is the tenant this scope belongs to, used only for per-tenant
	// metrics labelling (WF2-23.2). 0 means unattributed. It does not affect
	// isolation (that is the feed allow-set + view filter).
	TenantID int64
	// UserID is the user behind the connection. Like TenantID it does not affect
	// isolation; it is carried so a live re-scope (WF2-33) can re-resolve this
	// connection's *effective* view, which may be a per-user override.
	UserID int64
	feeds  map[int64]bool
	view   *ViewFilter // nil = no view restriction within the allowed feeds
}

// NewScope builds a feed-level scope from the allowed feed ids. An empty list
// yields a scope that allows nothing — fail-closed: a tenant with no
// subscriptions sees no tracks.
func NewScope(feedIDs []int64) *Scope {
	s := &Scope{feeds: make(map[int64]bool, len(feedIDs))}
	for _, id := range feedIDs {
		s.feeds[id] = true
	}
	return s
}

// NewScopeWithView builds a scope with both the feed allow-set and a view filter
// (WF2-21.2). A nil view means no view restriction within the allowed feeds.
func NewScopeWithView(feedIDs []int64, view *ViewFilter) *Scope {
	s := NewScope(feedIDs)
	s.view = view
	return s
}

// filterView returns msg unchanged when the scope has no view filter (fast path);
// otherwise a copy whose Tracks contain only those the view admits.
func (s *Scope) filterView(msg Message) Message {
	if s == nil || s.view == nil {
		return msg
	}
	kept := make([]TrackMessage, 0, len(msg.Tracks))
	for _, t := range msg.Tracks {
		if s.view.admits(t) {
			kept = append(kept, t)
		}
	}
	msg.Tracks = kept // msg is a value copy; the caller's Message is unaffected
	return msg
}

// AllowsFeed reports whether a client with this scope may receive tracks from the
// given feed. Every client is scoped (ADR 0014): production never registers a nil
// scope. A nil scope is therefore treated fail-closed — it allows no feed — so a
// stray unscoped client never sees the whole picture rather than the reverse.
func (s *Scope) AllowsFeed(feedID int64) bool {
	if s == nil {
		return false
	}
	return s.feeds[feedID]
}

// BBox is a WGS84 bounding box in degrees. A track is inside if its position lies
// within [MinLat,MaxLat] × [MinLon,MaxLon] (inclusive).
type BBox struct {
	MinLat, MinLon, MaxLat, MaxLon float64
}

// ViewFilter is a tenant's view scope within its allowed feeds (WF2-21.2): an
// optional area of interest and flight-level band, enforced server-side as a
// data-minimisation boundary — a track outside it never leaves the server
// (bandwidth, billing, no F12 leak). It is **fail-open**: a track missing the
// attribute a filter needs (e.g. no measured flight level) is delivered, never
// dropped — never hide a real aircraft (NFR-SEC-003 safety). Bounds are inclusive.
type ViewFilter struct {
	AOI     *BBox    // nil = no area restriction
	FLMinFt *float64 // nil = no lower bound (feet)
	FLMaxFt *float64 // nil = no upper bound (feet)
}

// admits reports whether a track passes the view filter. Position is part of
// every track, so the AOI check is exact; the flight-level check fails open when
// the track carries no measured level.
func (v *ViewFilter) admits(t TrackMessage) bool {
	if v == nil {
		return true
	}
	if v.AOI != nil {
		if t.Latitude < v.AOI.MinLat || t.Latitude > v.AOI.MaxLat ||
			t.Longitude < v.AOI.MinLon || t.Longitude > v.AOI.MaxLon {
			return false // unambiguously outside the area of interest
		}
	}
	if t.FlightLevelFt != nil { // fail-open: no measured FL ⇒ deliver
		if v.FLMinFt != nil && *t.FlightLevelFt < *v.FLMinFt {
			return false
		}
		if v.FLMaxFt != nil && *t.FlightLevelFt > *v.FLMaxFt {
			return false
		}
	}
	return true
}

// TrackMessage represents a single track in JSON format.
type TrackMessage struct {
	// FeedID is the catalogue feed this track arrived on (WF2-20). Omitted for the
	// unassigned ENV fallback feed (FeedID 0); set for every catalogue feed (WF2-20.2).
	FeedID    int64   `json:"feed_id,omitempty"`
	TrackNum  uint16  `json:"track_num"`
	SAC       uint8   `json:"sac"`
	SIC       uint8   `json:"sic"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Vx        float64 `json:"vx"`
	Vy        float64 `json:"vy"`
	CartX     float64 `json:"cart_x"`
	CartY     float64 `json:"cart_y"`
	Confirmed bool    `json:"confirmed"`
	Coasting  bool    `json:"coasting"`
	// Ended is the I062/080 TSE flag: this is the final report for the track,
	// which is being deleted. The frontend removes the track on this. Only
	// serialized when true (a live track omits it).
	Ended bool `json:"ended,omitempty"`
	// Monosensor is the I062/080 MON flag: only one sensor contributed to the
	// track within the freshness window, so no second source cross-checks the
	// estimate (more prone to ghosts/bias). A quality hint the ASD shows
	// discreetly. Only serialized when true. Firefly ICD 3.2.0.
	Monosensor bool `json:"mono,omitempty"`
	// SPI is the I062/080 SPI flag: the last associated report carried the ident
	// (Special Position Identification) pulse — the pilot pressed "ident" on the
	// controller's request. The frontend highlights the track while it is set.
	// Transient; only serialized when true.
	SPI    bool    `json:"spi,omitempty"`
	PSRAge float64 `json:"psr_age"`
	// AdsbAgeS is the time since the last ADS-B (Extended Squitter)
	// contribution, in seconds, from I062/290's ES subfield (ICD 2.4.0).
	// Present only for tracks with an ADS-B component (Firefly ADR 0019); its
	// presence is what the frontend uses to show the ADS-B badge.
	AdsbAgeS *float64 `json:"adsb_age_s,omitempty"`
	// SSRAgeS, MDSAgeS and FlarmAgeS are the remaining per-technology update
	// ages from I062/290 (ICD 2.6.0, Firefly ADR 0027): SSR = Mode A/C, MDS =
	// Mode S, FLARM = Firefly's vendor subfield. Present only when the track
	// has been updated by that technology; together with AdsbAgeS they let the
	// frontend derive an authoritative provenance (A = ADS-B, F = FLARM, …).
	SSRAgeS   *float64 `json:"ssr_age_s,omitempty"`
	MDSAgeS   *float64 `json:"mds_age_s,omitempty"`
	FlarmAgeS *float64 `json:"flarm_age_s,omitempty"`
	Accuracy  float64  `json:"accuracy"`
	Mode3A    *uint16  `json:"mode_3a,omitempty"`
	ICAOAddr  *uint32  `json:"icao_addr,omitempty"`
	// FlightLevelFt is the measured barometric flight level in feet (I062/136),
	// present only for tracks carrying a Mode C reply.
	FlightLevelFt *float64 `json:"flight_level_ft,omitempty"`
	// Callsign is the target identification / flight ID (I062/245), present
	// only for tracks carrying a Mode S identification reply.
	Callsign *string `json:"callsign,omitempty"`
	// Mode-S Downlink Aircraft Parameters (I062/380, ICD 3.4.0), present only when
	// the aircraft reports them. SelectedAltitudeFt (SAL) is the autopilot's target
	// altitude — the frontend compares it to the flight level for level-bust
	// detection. MagneticHeadingDeg/IasKt/Mach feed the detail panel.
	SelectedAltitudeFt *float64 `json:"selected_altitude_ft,omitempty"`
	MagneticHeadingDeg *float64 `json:"magnetic_heading_deg,omitempty"`
	IasKt              *float64 `json:"ias_kt,omitempty"`
	Mach               *float64 `json:"mach,omitempty"`
	// Vertical chain (I062/130/135/220, ICD 3.5.0), present only when Firefly has
	// a fresh vertical estimate. BarometricAltitudeFt is the filtered altitude the
	// label prefers over the jumpier measured FlightLevelFt; QNHCorrected tells the
	// frontend whether it is a QNH altitude ("A") or a pressure flight level ("FL").
	// GeometricAltitudeFt (WGS-84) and RocdFtMin (rate, positive = climb) feed the
	// climb/descent arrow and the detail panel.
	GeometricAltitudeFt  *float64 `json:"geometric_altitude_ft,omitempty"`
	BarometricAltitudeFt *float64 `json:"barometric_altitude_ft,omitempty"`
	QNHCorrected         *bool    `json:"qnh_corrected,omitempty"`
	RocdFtMin            *float64 `json:"rocd_ft_min,omitempty"`
	// Kinematics chain (I062/200/210, ICD 3.6.0), present only per determined axis.
	// CourseTrend ("constant"/"right"/"left") drives the label turn indicator;
	// SpeedTrend and VerticalMotion feed the detail panel (VerticalMotion is the
	// qualitative I062/200 VERT axis — named distinctly from the rate-driven ▲/▼
	// tendency glyph the frontend derives; the quantitative RocdFtMin stays the
	// arrow's primary source). AccelAxMs2/AccelAyMs2 are the calculated horizontal
	// acceleration (East/North).
	CourseTrend    *string  `json:"course_trend,omitempty"`
	SpeedTrend     *string  `json:"speed_trend,omitempty"`
	VerticalMotion *string  `json:"vertical_motion,omitempty"`
	AccelAxMs2     *float64 `json:"accel_ax_ms2,omitempty"`
	AccelAyMs2     *float64 `json:"accel_ay_ms2,omitempty"`
	// Flight-plan correlation (I062/390, ICD 3.7.0), present only for a correlated
	// track. PlanCallsign is the filed callsign (may differ from the downlinked
	// Callsign — a mismatch the frontend surfaces); PlanDeparture/PlanDestination
	// are the ICAO route endpoints, present only when the plan carries them.
	PlanCallsign    *string `json:"plan_callsign,omitempty"`
	PlanDeparture   *string `json:"plan_departure,omitempty"`
	PlanDestination *string `json:"plan_destination,omitempty"`
}

// Sender can send messages to all connected clients.
type Sender interface {
	Send(msg Message) error
}

// Broadcaster listens for CAT062 tracks and broadcasts them to all connected clients.
type Broadcaster struct {
	trackChan chan TrackBatch
	clients   sync.Map // map[*Client]bool
	logger    *slog.Logger

	registerChan   chan *Client
	unregisterChan chan *Client
	messageChan    chan Message
	// rescopeChan carries live re-scope batches (WF2-33). Buffered so the admin
	// path enqueues without blocking; applied on the Run goroutine.
	rescopeChan chan map[*Client]*Scope

	evicted atomic.Int64

	// Per-tenant counters for /metrics (WF2-23.2). Written only from the Run
	// goroutine, read from the probe server, guarded by tenantMu.
	tenantMu sync.Mutex
	tenant   map[int64]*tenantCounters
}

type tenantCounters struct {
	connected int64
	delivered int64
}

// TenantMetric is a snapshot of one tenant's broadcast counters.
type TenantMetric struct {
	TenantID  int64
	Connected int64 // currently connected WebSocket clients
	Delivered int64 // total track messages delivered to this tenant's clients
}

// Client represents a connected WebSocket client. scope decides which feeds'
// tracks it may receive and the view filter applied within them (always non-nil
// in production, ADR 0014; a nil scope is fail-closed).
type Client struct {
	send chan Message
	// scope is the mutable isolation/view filter. It is read and written ONLY on
	// the Run goroutine (broadcastTracks reads it, applyScopes swaps it on a live
	// re-scope), so it needs no lock — the broadcaster is a single-goroutine
	// actor (WF2-33).
	scope *Scope
	// tenantID and userID are the immutable identity of the connection, copied
	// from the scope at registration. They never change for the life of the
	// client (a re-scope keeps the same tenant/user), so ClientsForTenant may read
	// them concurrently with Run without ever touching the mutable scope.
	tenantID int64
	userID   int64
}

// New creates a new Broadcaster.
func New(logger *slog.Logger) *Broadcaster {
	return &Broadcaster{
		trackChan:      make(chan TrackBatch, 10),
		logger:         logger,
		registerChan:   make(chan *Client, 10),
		unregisterChan: make(chan *Client, 10),
		messageChan:    make(chan Message, 10),
		rescopeChan:    make(chan map[*Client]*Scope, 16),
		tenant:         make(map[int64]*tenantCounters),
	}
}

// TracksChan returns the channel for receiving CAT062 track batches (each
// attributed to its feed, WF2-20).
func (b *Broadcaster) TracksChan() chan<- TrackBatch {
	return b.trackChan
}

// RegisterClient adds a new client to receive broadcasts, filtered by scope.
// Production always passes a non-nil scope (ADR 0014); a nil scope is fail-closed
// (AllowsFeed denies every feed), so such a client receives nothing.
func (b *Broadcaster) RegisterClient(sendChan chan Message, scope *Scope) *Client {
	c := &Client{send: sendChan, scope: scope}
	if scope != nil {
		// Pin the immutable identity so a later re-scope can find this client
		// without reading the mutable scope from another goroutine (WF2-33).
		c.tenantID = scope.TenantID
		c.userID = scope.UserID
	}
	b.registerChan <- c
	return c
}

// UnregisterClient asks the broadcaster to remove a client. It is the path for
// EXTERNAL goroutines (e.g. the WebSocket handler on disconnect): the removal is
// carried out on the Run goroutine, which drains unregisterChan. Code already
// running on the Run goroutine must NOT call this — sending to a channel only it
// receives from deadlocks once the buffer fills — and uses dropClient instead.
func (b *Broadcaster) UnregisterClient(c *Client) {
	b.unregisterChan <- c
}

// dropClient removes a client and closes its send channel. It MUST run on the
// Run goroutine — the sole owner of client teardown — so the drop-on-full-send
// paths (broadcastTracks/broadcast) and the unregisterChan case call it directly
// rather than UnregisterClient: routing those through unregisterChan would make
// the Run goroutine block sending to a channel only it receives from, deadlocking
// the broadcaster once the buffer fills. LoadAndDelete makes the close idempotent
// — a duplicate drop (e.g. an external UnregisterClient racing an eviction of the
// same client) is a harmless no-op, never a "close of closed channel" panic, and
// the per-tenant connected gauge is decremented exactly once.
func (b *Broadcaster) dropClient(c *Client) bool {
	if _, loaded := b.clients.LoadAndDelete(c); !loaded {
		return false
	}
	b.tenantConnectedDelta(c, -1)
	close(c.send)
	return true
}

// ClientRef identifies a connected client and the user behind it, for live
// re-scoping (WF2-33). The Client handle is opaque; UserID lets the caller
// re-resolve that connection's effective view (which may be a per-user override).
type ClientRef struct {
	Client *Client
	UserID int64
}

// ClientsForTenant snapshots every connected client of the given tenant. It is
// the first phase of a live re-scope (WF2-33): it reads only the immutable
// identity fields (never the mutable scope), so it is safe to call concurrently
// with the Run loop. The caller then resolves fresh scopes *off* the hot path and
// applies them via ApplyScopes. tenant 0 (unattributed) is never re-scoped.
func (b *Broadcaster) ClientsForTenant(tenantID int64) []ClientRef {
	if tenantID == 0 {
		return nil
	}
	var refs []ClientRef
	b.clients.Range(func(key, _ any) bool {
		c := key.(*Client)
		if c.tenantID == tenantID {
			refs = append(refs, ClientRef{Client: c, UserID: c.userID})
		}
		return true
	})
	return refs
}

// ApplyScopes hands a batch of freshly resolved scopes to the Run loop to swap
// onto the named clients (WF2-33, the apply phase). The swap happens *inside* Run
// — the same goroutine that evaluates tracks — so an active connection's scope is
// never mutated concurrently with its own track evaluation: no lock, no race. The
// Run loop is never blocked; this only enqueues onto a buffered channel, bounded
// by ctx so a shutdown cannot deadlock the caller. A nil/empty map is a no-op.
func (b *Broadcaster) ApplyScopes(ctx context.Context, scopes map[*Client]*Scope) error {
	if len(scopes) == 0 {
		return nil
	}
	select {
	case b.rescopeChan <- scopes:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// applyScopes swaps the new scope onto each still-connected client. It runs on
// the Run goroutine, preserving the invariant that c.scope is only ever written
// here and read in broadcastTracks (same goroutine → no synchronisation needed).
// A client that disconnected between snapshot and apply is skipped — its entry in
// b.clients is gone, so a stale handle can never resurrect it.
//
// When a feed is revoked (old scope allowed a feed that the new one does not), an
// empty-tracks frame is sent to the client immediately so the frontend clears any
// stale tracks from the revoked feed without waiting for the next batch — the
// next batch from that feed will not arrive because the scope is already updated
// (NFR-SEC-003).
func (b *Broadcaster) applyScopes(scopes map[*Client]*Scope) {
	n := 0
	purge := Message{Tracks: []TrackMessage{}}
	for c, s := range scopes {
		if _, ok := b.clients.Load(c); ok {
			if hasFeedRevoke(c.scope, s) {
				select {
				case c.send <- purge:
				default:
					// Client channel full; it will be evicted on the next broadcast.
				}
			}
			c.scope = s
			n++
		}
	}
	if n > 0 {
		b.logger.Debug("rescoped clients", slog.Int("count", n))
	}
}

// hasFeedRevoke reports whether transitioning from old to next removes any feed
// that the old scope permitted. A nil old scope has no feed allow-set, so
// nothing can be revoked.
func hasFeedRevoke(old, next *Scope) bool {
	if old == nil {
		return false
	}
	for feedID := range old.feeds {
		if !next.AllowsFeed(feedID) {
			return true
		}
	}
	return false
}

// Run starts the broadcaster loop (blocks until context is cancelled).
func (b *Broadcaster) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			b.closeAllClients()
			return ctx.Err()

		case c := <-b.registerChan:
			b.clients.Store(c, true)
			b.tenantConnectedDelta(c, +1)
			b.logger.Debug("client registered", slog.Int("clients", b.clientCount()))

		case c := <-b.unregisterChan:
			if b.dropClient(c) {
				b.logger.Debug("client unregistered", slog.Int("clients", b.clientCount()))
			}

		case batch := <-b.trackChan:
			b.broadcastTracks(batch)

		case msg := <-b.messageChan:
			b.broadcast(msg)

		case scopes := <-b.rescopeChan:
			b.applyScopes(scopes)
		}
	}
}

// Send allows external callers to send messages (for testing/integration).
func (b *Broadcaster) Send(msg Message) error {
	select {
	case b.messageChan <- msg:
		return nil
	default:
		return ErrBroadcasterFull
	}
}

// ClientCount returns the number of connected clients.
func (b *Broadcaster) ClientCount() int {
	return b.clientCount()
}

// EvictedCount returns the total number of clients evicted so far because
// their send channel was full (REQ NFR-OBS-002, exposed via /metrics).
func (b *Broadcaster) EvictedCount() int64 {
	return b.evicted.Load()
}

// enumStr converts a typed-string enum pointer (e.g. cat062.CourseTrend, the
// I062/200 axes) to a plain *string for the wire DTO, preserving nil so an
// undetermined/absent axis ships no field.
func enumStr[T ~string](p *T) *string {
	if p == nil {
		return nil
	}
	s := string(*p)
	return &s
}

// tracksToMessage converts a feed's CAT062 decoded tracks to a broadcast
// message, stamping the batch's FeedID onto every track (WF2-20).
func (b *Broadcaster) tracksToMessage(batch TrackBatch) Message {
	msg := Message{
		Tracks: make([]TrackMessage, len(batch.Tracks)),
		TimeMs: timeNowMs(),
	}

	for i, track := range batch.Tracks {
		// The QNH-correction flag only carries meaning alongside a barometric
		// altitude; emit it only when I062/135 is present, so an absent vertical
		// solution never ships a stray "qnh_corrected": false.
		var qnh *bool
		if track.BarometricAltitudeFt != nil {
			v := track.BaroQNHCorrected
			qnh = &v
		}
		msg.Tracks[i] = TrackMessage{
			FeedID:               batch.FeedID,
			TrackNum:             track.TrackNum,
			SAC:                  track.Source.SAC,
			SIC:                  track.Source.SIC,
			Latitude:             track.WGS84.Latitude,
			Longitude:            track.WGS84.Longitude,
			Vx:                   track.Velocity.Vx,
			Vy:                   track.Velocity.Vy,
			CartX:                track.Cartesian.X,
			CartY:                track.Cartesian.Y,
			Confirmed:            track.Status.Confirmed,
			Coasting:             track.Status.Coasting,
			Ended:                track.Status.Ended,
			Monosensor:           track.Status.Monosensor,
			SPI:                  track.Status.SPI,
			PSRAge:               track.UpdateAge.PSRAge,
			AdsbAgeS:             track.UpdateAge.ESAge,
			SSRAgeS:              track.UpdateAge.SSRAge,
			MDSAgeS:              track.UpdateAge.MDSAge,
			FlarmAgeS:            track.UpdateAge.FLARMAge,
			Accuracy:             track.Accuracy.APC,
			Mode3A:               track.Mode3A,
			ICAOAddr:             track.ICAOAddr,
			FlightLevelFt:        track.FlightLevelFt,
			Callsign:             track.Callsign,
			SelectedAltitudeFt:   track.SelectedAltitudeFt,
			MagneticHeadingDeg:   track.MagneticHeadingDeg,
			IasKt:                track.IndicatedAirspeedKt,
			Mach:                 track.MachNumber,
			GeometricAltitudeFt:  track.GeometricAltitudeFt,
			BarometricAltitudeFt: track.BarometricAltitudeFt,
			QNHCorrected:         qnh,
			RocdFtMin:            track.RateOfClimbDescentFtMin,
			CourseTrend:          enumStr(track.MotionCourse),
			SpeedTrend:           enumStr(track.MotionSpeed),
			VerticalMotion:       enumStr(track.MotionVertical),
			AccelAxMs2:           track.AccelAxMS2,
			AccelAyMs2:           track.AccelAyMS2,
			PlanCallsign:         track.PlanCallsign,
			PlanDeparture:        track.PlanDeparture,
			PlanDestination:      track.PlanDestination,
		}
	}

	return msg
}

// broadcastTracks delivers a feed's tracks only to clients whose scope allows
// that feed — the feed-level cross-tenant isolation boundary (WF2-21.1). A client
// not subscribed to the batch's feed receives nothing from it.
func (b *Broadcaster) broadcastTracks(batch TrackBatch) {
	msg := b.tracksToMessage(batch)
	b.logger.Debug("broadcasting tracks",
		slog.Int64("feed_id", batch.FeedID),
		slog.Int("tracks", len(msg.Tracks)),
		slog.Int("clients", b.clientCount()))

	b.clients.Range(func(key, value any) bool {
		c := key.(*Client)
		if !c.scope.AllowsFeed(batch.FeedID) {
			return true // not subscribed to this feed → isolation, skip
		}
		cmsg := c.scope.filterView(msg) // apply the client's AOI/FL view (WF2-21.2)
		if len(cmsg.Tracks) == 0 {
			return true // nothing in this client's view from this batch
		}
		select {
		case c.send <- cmsg:
			b.tenantDelivered(c, len(cmsg.Tracks))
		default:
			b.logger.Warn("client send channel full, evicting client")
			b.evicted.Add(1)
			b.dropClient(c)
		}
		return true
	})
}

// tenantConnectedDelta adjusts a tenant's connected-client gauge (WF2-23.2).
// No-op for an unattributed client (nil scope or TenantID 0).
func (b *Broadcaster) tenantConnectedDelta(c *Client, delta int64) {
	if c.scope == nil || c.scope.TenantID == 0 {
		return
	}
	b.tenantMu.Lock()
	b.tenantCounters(c.scope.TenantID).connected += delta
	b.tenantMu.Unlock()
}

// tenantDelivered adds to a tenant's delivered-track counter (WF2-23.2).
func (b *Broadcaster) tenantDelivered(c *Client, n int) {
	if c.scope == nil || c.scope.TenantID == 0 || n == 0 {
		return
	}
	b.tenantMu.Lock()
	b.tenantCounters(c.scope.TenantID).delivered += int64(n)
	b.tenantMu.Unlock()
}

// tenantCounters returns the (lazily created) counters for a tenant. Caller holds
// tenantMu.
func (b *Broadcaster) tenantCounters(tid int64) *tenantCounters {
	tc := b.tenant[tid]
	if tc == nil {
		tc = &tenantCounters{}
		b.tenant[tid] = tc
	}
	return tc
}

// TenantMetrics returns a snapshot of the per-tenant broadcast counters, sorted
// by tenant id for stable /metrics output (WF2-23.2).
func (b *Broadcaster) TenantMetrics() []TenantMetric {
	b.tenantMu.Lock()
	defer b.tenantMu.Unlock()
	out := make([]TenantMetric, 0, len(b.tenant))
	for tid, tc := range b.tenant {
		out = append(out, TenantMetric{TenantID: tid, Connected: tc.connected, Delivered: tc.delivered})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TenantID < out[j].TenantID })
	return out
}

// broadcast sends a message to connected clients. When the message carries a
// FeedStatusMessage with FeedID != 0, only clients subscribed to that feed
// receive it (per-feed status scoping, WF2-21 isolation). FeedID == 0 (the
// unassigned ENV fallback feed) goes to all clients.
func (b *Broadcaster) broadcast(msg Message) {
	b.logger.Debug("broadcasting", slog.Int("tracks", len(msg.Tracks)), slog.Int("clients", b.clientCount()))

	b.clients.Range(func(key, value any) bool {
		c := key.(*Client)
		if msg.FeedStatus != nil && msg.FeedStatus.FeedID != 0 && !c.scope.AllowsFeed(msg.FeedStatus.FeedID) {
			return true // not subscribed to this feed's status
		}
		select {
		case c.send <- msg:
		default:
			// Client's send channel is full; unregister it.
			b.logger.Warn("client send channel full, evicting client")
			b.evicted.Add(1)
			b.dropClient(c)
		}
		return true
	})
}

// closeAllClients closes all client send channels.
func (b *Broadcaster) closeAllClients() {
	b.clients.Range(func(key, value any) bool {
		c := key.(*Client)
		close(c.send)
		b.clients.Delete(c)
		return true
	})
}

// clientCount returns the current number of connected clients.
func (b *Broadcaster) clientCount() int {
	count := 0
	b.clients.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}

// timeNowMs is the wall-clock send time (Unix epoch milliseconds) stamped on a
// broadcast envelope as Message.TimeMs. It is the *envelope* time — when the server
// pushed this batch to clients, for client-side latency/diagnostics — deliberately
// distinct from each track's *data* time (CAT062 Time-of-Day), which travels in the
// per-track fields. Wall-clock is correct here precisely because this is not the
// deterministic data-time path (CLAUDE.md §7).
func timeNowMs() int64 {
	return time.Now().UnixMilli()
}
