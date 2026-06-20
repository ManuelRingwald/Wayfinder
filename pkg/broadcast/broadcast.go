package broadcast

import (
	"context"
	"log/slog"
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
}

// Client represents a connected WebSocket client.
type Client struct {
	send chan Message
}

// New creates a new Broadcaster.
func New(logger *slog.Logger) *Broadcaster {
	return &Broadcaster{
		trackChan:      make(chan TrackBatch, 10),
		logger:         logger,
		registerChan:   make(chan *Client, 10),
		unregisterChan: make(chan *Client, 10),
		messageChan:    make(chan Message, 10),
	}
}

// TracksChan returns the channel for receiving CAT062 track batches (each
// attributed to its feed, WF2-20).
func (b *Broadcaster) TracksChan() chan<- TrackBatch {
	return b.trackChan
}

// RegisterClient adds a new client to receive broadcasts.
func (b *Broadcaster) RegisterClient(sendChan chan Message) *Client {
	c := &Client{send: sendChan}
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
			b.logger.Debug("client registered", slog.Int("clients", b.clientCount()))

		case c := <-b.unregisterChan:
			b.clients.Delete(c)
			close(c.send)
			b.logger.Debug("client unregistered", slog.Int("clients", b.clientCount()))

		case batch := <-b.trackChan:
			msg := b.tracksToMessage(batch)
			b.broadcast(msg)

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

// broadcast sends a message to all connected clients.
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
