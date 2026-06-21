package broadcast

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
)

// Message is sent to all WebSocket clients. It carries either a track update
// (Tracks) or a feed-health update (FeedStatus, from the CAT065 heartbeat,
// Firefly ADR 0018) — the two are routed separately by the frontend.
type Message struct {
	Tracks     []TrackMessage     `json:"tracks"`
	TimeMs     int64              `json:"time_ms"`
	FeedStatus *FeedStatusMessage `json:"feed_status,omitempty"`
}

// FeedStatusMessage carries the CAT065 feed-health state to the browser.
type FeedStatusMessage struct {
	// State is "ok" (heartbeat fresh), "stale" (heartbeat lost) or "unknown"
	// (no heartbeat seen yet).
	State string `json:"state"`
	// ServiceID is the CAT065 I065/015 service identification, when known.
	ServiceID uint8 `json:"service_id,omitempty"`
}

// TrackBatch is a set of decoded tracks attributed to the feed they arrived on.
// FeedID is stamped by the receiver (WF2-20) and threaded onto every resulting
// TrackMessage, so the scoped fan-out (WF2-21) can filter per tenant
// subscription. FeedID 0 is the single-tenant / single-feed fallback.
type TrackBatch struct {
	FeedID int64
	Tracks []cat062.DecodedTrack
}

// Scope decides which tracks a client may receive (WF2-21 — the cross-tenant
// isolation boundary). In WF2-21.1 it is feed-level: a client sees a track only
// if the track's feed is in the allowed set, resolved from the tenant's
// subscriptions at connect time. A nil Scope is unscoped — single-tenant, sees
// every feed. View filters (AOI/FL/category) layer on in WF2-21.2.
type Scope struct {
	// TenantID is the tenant this scope belongs to, used only for per-tenant
	// metrics labelling (WF2-23.2). 0 means single-tenant / unattributed. It does
	// not affect isolation (that is the feed allow-set + view filter).
	TenantID int64
	feeds    map[int64]bool
	view     *ViewFilter // nil = no view restriction within the allowed feeds
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
// given feed. A nil scope (single-tenant) allows every feed.
func (s *Scope) AllowsFeed(feedID int64) bool {
	if s == nil {
		return true
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
	// FeedID is the catalogue feed this track arrived on (WF2-20). Omitted in the
	// single-tenant fallback (FeedID 0); set once feeds are catalogued (WF2-20.2).
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
	Ended  bool    `json:"ended,omitempty"`
	PSRAge float64 `json:"psr_age"`
	// AdsbAgeS is the time since the last ADS-B (Extended Squitter)
	// contribution, in seconds, from I062/290's ES subfield (ICD 2.4.0).
	// Present only for tracks with an ADS-B component (Firefly ADR 0019); its
	// presence is what the frontend uses to show the ADS-B badge.
	AdsbAgeS *float64 `json:"adsb_age_s,omitempty"`
	Accuracy float64  `json:"accuracy"`
	Mode3A   *uint16  `json:"mode_3a,omitempty"`
	ICAOAddr *uint32  `json:"icao_addr,omitempty"`
	// FlightLevelFt is the measured barometric flight level in feet (I062/136),
	// present only for tracks carrying a Mode C reply.
	FlightLevelFt *float64 `json:"flight_level_ft,omitempty"`
	// Callsign is the target identification / flight ID (I062/245), present
	// only for tracks carrying a Mode S identification reply.
	Callsign *string `json:"callsign,omitempty"`
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
// tracks it may receive and the view filter applied within them (nil = unscoped
// / single-tenant).
type Client struct {
	send  chan Message
	scope *Scope
}

// New creates a new Broadcaster.
func New(logger *slog.Logger) *Broadcaster {
	return &Broadcaster{
		trackChan:      make(chan TrackBatch, 10),
		logger:         logger,
		registerChan:   make(chan *Client, 10),
		unregisterChan: make(chan *Client, 10),
		messageChan:    make(chan Message, 10),
		tenant:         make(map[int64]*tenantCounters),
	}
}

// TracksChan returns the channel for receiving CAT062 track batches (each
// attributed to its feed, WF2-20).
func (b *Broadcaster) TracksChan() chan<- TrackBatch {
	return b.trackChan
}

// RegisterClient adds a new client to receive broadcasts, filtered by scope.
// A nil scope is unscoped (single-tenant: receives every feed).
func (b *Broadcaster) RegisterClient(sendChan chan Message, scope *Scope) *Client {
	c := &Client{send: sendChan, scope: scope}
	b.registerChan <- c
	return c
}

// UnregisterClient removes a client from the broadcast list.
func (b *Broadcaster) UnregisterClient(c *Client) {
	b.unregisterChan <- c
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
			b.clients.Delete(c)
			b.tenantConnectedDelta(c, -1)
			close(c.send)
			b.logger.Debug("client unregistered", slog.Int("clients", b.clientCount()))

		case batch := <-b.trackChan:
			b.broadcastTracks(batch)

		case msg := <-b.messageChan:
			b.broadcast(msg)
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

// tracksToMessage converts a feed's CAT062 decoded tracks to a broadcast
// message, stamping the batch's FeedID onto every track (WF2-20).
func (b *Broadcaster) tracksToMessage(batch TrackBatch) Message {
	msg := Message{
		Tracks: make([]TrackMessage, len(batch.Tracks)),
		TimeMs: timeNowMs(),
	}

	for i, track := range batch.Tracks {
		msg.Tracks[i] = TrackMessage{
			FeedID:        batch.FeedID,
			TrackNum:      track.TrackNum,
			SAC:           track.Source.SAC,
			SIC:           track.Source.SIC,
			Latitude:      track.WGS84.Latitude,
			Longitude:     track.WGS84.Longitude,
			Vx:            track.Velocity.Vx,
			Vy:            track.Velocity.Vy,
			CartX:         track.Cartesian.X,
			CartY:         track.Cartesian.Y,
			Confirmed:     track.Status.Confirmed,
			Coasting:      track.Status.Coasting,
			Ended:         track.Status.Ended,
			PSRAge:        track.UpdateAge.PSRAge,
			AdsbAgeS:      track.UpdateAge.ESAge,
			Accuracy:      track.Accuracy.APC,
			Mode3A:        track.Mode3A,
			ICAOAddr:      track.ICAOAddr,
			FlightLevelFt: track.FlightLevelFt,
			Callsign:      track.Callsign,
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
			b.UnregisterClient(c)
		}
		return true
	})
}

// tenantConnectedDelta adjusts a tenant's connected-client gauge (WF2-23.2).
// No-op for unscoped (single-tenant) clients.
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

// broadcast sends a message to all connected clients (used for global, non-track
// messages such as the feed-health status, which is not feed-scoped).
func (b *Broadcaster) broadcast(msg Message) {
	b.logger.Debug("broadcasting", slog.Int("tracks", len(msg.Tracks)), slog.Int("clients", b.clientCount()))

	b.clients.Range(func(key, value any) bool {
		c := key.(*Client)
		select {
		case c.send <- msg:
		default:
			// Client's send channel is full; unregister it.
			b.logger.Warn("client send channel full, evicting client")
			b.evicted.Add(1)
			b.UnregisterClient(c)
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

// timeNowMs returns current time in milliseconds since Unix epoch.
func timeNowMs() int64 {
	return 0 // TODO: Use CAT062 Time-of-Day instead
}
