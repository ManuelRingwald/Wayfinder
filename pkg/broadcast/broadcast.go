package broadcast

import (
	"context"
	"log/slog"
	"sync"

	"github.com/manuelringwald/wayfinder/pkg/cat062"
)

// Message is sent to all WebSocket clients.
type Message struct {
	Tracks []TrackMessage `json:"tracks"`
	TimeMs int64          `json:"time_ms"`
}

// TrackMessage represents a single track in JSON format.
type TrackMessage struct {
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
	Ended    bool    `json:"ended,omitempty"`
	PSRAge   float64 `json:"psr_age"`
	Accuracy float64 `json:"accuracy"`
	Mode3A   *uint16 `json:"mode_3a,omitempty"`
	ICAOAddr *uint32 `json:"icao_addr,omitempty"`
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
	trackChan chan []cat062.DecodedTrack
	clients   sync.Map // map[*Client]bool
	logger    *slog.Logger

	registerChan   chan *Client
	unregisterChan chan *Client
	messageChan    chan Message
}

// Client represents a connected WebSocket client.
type Client struct {
	send chan Message
}

// New creates a new Broadcaster.
func New(logger *slog.Logger) *Broadcaster {
	return &Broadcaster{
		trackChan:      make(chan []cat062.DecodedTrack, 10),
		logger:         logger,
		registerChan:   make(chan *Client, 10),
		unregisterChan: make(chan *Client, 10),
		messageChan:    make(chan Message, 10),
	}
}

// TracksChan returns the channel for receiving CAT062 tracks.
func (b *Broadcaster) TracksChan() chan<- []cat062.DecodedTrack {
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

		case tracks := <-b.trackChan:
			msg := b.tracksToMessage(tracks)
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

// tracksToMessage converts CAT062 decoded tracks to a broadcast message.
func (b *Broadcaster) tracksToMessage(tracks []cat062.DecodedTrack) Message {
	msg := Message{
		Tracks: make([]TrackMessage, len(tracks)),
		TimeMs: timeNowMs(),
	}

	for i, track := range tracks {
		msg.Tracks[i] = TrackMessage{
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
